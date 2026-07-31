package repository

import (
	"context"

	"xorm.io/xorm"
)

// Repository 泛型仓储：面向单一模型 T 的 CRUD / 查询入口。
// 事务与 session 生命周期由内部 *DB 统一管理。
type Repository[T any] struct {
	db *DB
}

// NewRepository 创建仓储实例
func NewRepository[T any](db *DB) *Repository[T] {
	return &Repository[T]{db: mustDB(db)}
}

// Transaction 代理到 *DB，使同一事务可跨多个 Repository。
//
// 使用示例：
//
//	err := repo.Transaction(ctx, func(txCtx context.Context) error {
//	    if err := repo.Create(txCtx, &user); err != nil {
//	        return err
//	    }
//	    return otherRepo.Update(txCtx, &order)
//	})
func (r *Repository[T]) Transaction(ctx context.Context, fn func(ctx context.Context) error) error {
	return r.db.Transaction(ctx, fn)
}

// Create 插入单条记录
func (r *Repository[T]) Create(ctx context.Context, model *T) error {
	if model == nil {
		return ErrInvalidModel
	}
	err := r.db.withSession(ctx, func(session *xorm.Session) error {
		_, err := session.Insert(model)
		return err
	})
	return wrapDBErr("Create", err)
}

// Update 按主键更新非空字段
func (r *Repository[T]) Update(ctx context.Context, model *T) error {
	if model == nil {
		return ErrInvalidModel
	}
	pk, err := primaryKeyValue(model)
	if err != nil {
		return err
	}
	err = r.db.withSession(ctx, func(session *xorm.Session) error {
		_, err := session.ID(pk).Update(model)
		return err
	})
	return wrapDBErr("Update", err)
}

// Delete 按模型主键（或非零字段条件）删除
func (r *Repository[T]) Delete(ctx context.Context, model *T) error {
	if model == nil {
		return ErrInvalidModel
	}
	err := r.db.withSession(ctx, func(session *xorm.Session) error {
		_, err := session.Delete(model)
		return err
	})
	return wrapDBErr("Delete", err)
}

// DeleteByID 根据主键删除
func (r *Repository[T]) DeleteByID(ctx context.Context, id any) error {
	model := new(T)
	if err := setPrimaryKey(model, id); err != nil {
		return err
	}
	return r.Delete(ctx, model)
}

// Find 根据模型非零字段查询单条；0 条 -> ErrNotFound，>1 条 -> ErrMultiple
func (r *Repository[T]) Find(ctx context.Context, model *T) (*T, error) {
	if model == nil {
		return nil, ErrInvalidModel
	}
	return r.findOne(ctx, buildFiltersFromModel(model))
}

// FindByID 根据主键查询
func (r *Repository[T]) FindByID(ctx context.Context, id any) (*T, error) {
	meta, err := metaOf[T]()
	if err != nil {
		return nil, err
	}
	return r.findOne(ctx, []Condition{{Field: meta.pkColumn, Op: OpEq, Value: id}})
}

// FindAll 根据模型非零字段查询全部匹配记录
func (r *Repository[T]) FindAll(ctx context.Context, model *T) ([]T, error) {
	if model == nil {
		return nil, ErrInvalidModel
	}
	return r.QueryBuilder().applyFilters(buildFiltersFromModel(model)).Find(ctx)
}

// FindPage 根据 query 非零字段分页查询，返回列表与总数。
// list 与 count 使用独立 session，互不污染 Limit。
func (r *Repository[T]) FindPage(ctx context.Context, query any, limit, offset int) ([]T, int64, error) {
	if query == nil {
		return nil, 0, ErrInvalidModel
	}
	filters := buildFiltersFromModel(query)
	rows, err := r.QueryBuilder().applyFilters(filters).Limit(limit).Offset(offset).Find(ctx)
	if err != nil {
		return nil, 0, err
	}
	total, err := r.QueryBuilder().applyFilters(filters).Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// BatchCreate 批量插入（单条 SQL / 一次会话，而非逐条假批量）
func (r *Repository[T]) BatchCreate(ctx context.Context, models []T) error {
	if len(models) == 0 {
		return nil
	}
	err := r.db.withSession(ctx, func(session *xorm.Session) error {
		_, err := session.Insert(&models)
		return err
	})
	return wrapDBErr("BatchCreate", err)
}

// QueryBuilder 获取链式查询构建器
func (r *Repository[T]) QueryBuilder() *QueryBuilder[T] {
	return NewQueryBuilder[T](r.db)
}

// Query 执行自定义 SQL 并将结果扫描进 []T（用于 CTE 等复杂查询）
func (r *Repository[T]) Query(ctx context.Context, sql string, args ...any) ([]T, error) {
	var rows []T
	err := r.db.withSession(ctx, func(session *xorm.Session) error {
		return session.SQL(sql, args...).Find(&rows)
	})
	return rows, wrapDBErr("Query", err)
}

func (r *Repository[T]) findOne(ctx context.Context, filters []Condition) (*T, error) {
	rows, err := r.QueryBuilder().applyFilters(filters).Limit(2).Find(ctx)
	if err != nil {
		return nil, err
	}
	switch len(rows) {
	case 0:
		return nil, ErrNotFound
	case 1:
		return &rows[0], nil
	default:
		return nil, ErrMultiple
	}
}
