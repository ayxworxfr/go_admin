package repository

import (
	"context"
	"errors"
)

// QueryBuilder 链式查询构建器
type QueryBuilder[T any] struct {
	processor  ORMProcessor
	conditions []Condition
	orderBy    string
	limit      int
	offset     int
	lock       string
}

// Condition 查询条件
type Condition struct {
	Field string
	Op    Op
	Value any
}

// NewQueryBuilder 创建链式查询构建器
func NewQueryBuilder[T any](processor ORMProcessor) *QueryBuilder[T] {
	return &QueryBuilder[T]{
		processor: processor,
	}
}

// OrderBy 添加排序
func (qb *QueryBuilder[T]) OrderBy(fields string) *QueryBuilder[T] {
	qb.orderBy = fields
	return qb
}

// Limit 添加分页限制
func (qb *QueryBuilder[T]) Limit(limit int) *QueryBuilder[T] {
	qb.limit = limit
	return qb
}

// Offset 添加分页偏移
func (qb *QueryBuilder[T]) Offset(offset int) *QueryBuilder[T] {
	qb.offset = offset
	return qb
}

// ForUpdate 添加行锁
func (qb *QueryBuilder[T]) ForUpdate() *QueryBuilder[T] {
	qb.lock = "FOR UPDATE"
	return qb
}

// Find 执行查询并返回列表
func (qb *QueryBuilder[T]) Find(ctx context.Context) ([]T, error) {
	queryOpts := &QueryOption{
		OrderBy: qb.orderBy,
		Limit:   qb.limit,
		Offset:  qb.offset,
		Lock:    qb.lock,
		Filters: qb.conditions,
	}

	result, err := qb.processor.Query(ctx, new(T), queryOpts)
	if err != nil {
		return nil, err
	}

	data, ok := result.Data.([]T)
	if !ok {
		return nil, errors.New("invalid result type")
	}

	return data, nil
}

// First 返回第一条记录
func (qb *QueryBuilder[T]) First(ctx context.Context) (*T, error) {
	qb.Limit(1)
	result, err := qb.Find(ctx)
	if err != nil {
		return nil, err
	}

	if len(result) == 0 {
		return nil, errors.New("record not found")
	}

	return &result[0], nil
}

// Count 返回记录总数
func (qb *QueryBuilder[T]) Count(ctx context.Context) (int64, error) {
	queryOpts := &QueryOption{
		Filters: qb.conditions,
	}

	result, err := qb.processor.Query(ctx, new(T), queryOpts)
	if err != nil {
		return 0, err
	}

	return result.Total, nil
}

// Delete 执行删除操作
func (qb *QueryBuilder[T]) Delete(ctx context.Context) error {
	queryOpts := &QueryOption{
		Filters: qb.conditions,
		OrderBy: qb.orderBy,
		Limit:   qb.limit,
		Offset:  qb.offset,
		Lock:    qb.lock,
	}

	// 创建空模型实例
	model := new(T)
	err := qb.processor.DeleteByOption(ctx, model, queryOpts)
	return err
}

// Where 添加WHERE条件
func (qb *QueryBuilder[T]) Where() *QueryBuilder[T] {
	return qb
}

// where 添加WHERE条件
func (qb *QueryBuilder[T]) where(field string, op Op, value any) *QueryBuilder[T] {
	qb.conditions = append(qb.conditions, Condition{
		Field: field,
		Op:    op,
		Value: value,
	})
	return qb
}

// Or 添加OR WHERE条件
func (qb *QueryBuilder[T]) Or(field string, value any) *QueryBuilder[T] {
	return qb.where(field, OpOr, value)
}

// And 添加AND WHERE条件
func (qb *QueryBuilder[T]) And(field string, value any) *QueryBuilder[T] {
	return qb.where(field, OpAnd, value)
}

// Like 添加LIKE WHERE条件
func (qb *QueryBuilder[T]) Like(field string, value any) *QueryBuilder[T] {
	return qb.where(field, OpLike, value)
}

// Eq 添加等于WHERE条件
func (qb *QueryBuilder[T]) Eq(field string, value any) *QueryBuilder[T] {
	return qb.where(field, OpEq, value)
}

// Ne 添加不等于WHERE条件
func (qb *QueryBuilder[T]) Ne(field string, value any) *QueryBuilder[T] {
	return qb.where(field, OpNe, value)
}

// Gt 添加大于WHERE条件
func (qb *QueryBuilder[T]) Gt(field string, value any) *QueryBuilder[T] {
	return qb.where(field, OpGt, value)
}

// Lt 添加小于WHERE条件
func (qb *QueryBuilder[T]) Lt(field string, value any) *QueryBuilder[T] {
	return qb.where(field, OpLt, value)
}

// Gte 添加大于等于WHERE条件
func (qb *QueryBuilder[T]) Gte(field string, value any) *QueryBuilder[T] {
	return qb.where(field, OpGe, value)
}

// Lte 添加小于等于WHERE条件
func (qb *QueryBuilder[T]) Lte(field string, value any) *QueryBuilder[T] {
	return qb.where(field, OpLe, value)
}

// In 添加IN WHERE条件
func (qb *QueryBuilder[T]) In(field string, value any) *QueryBuilder[T] {
	return qb.where(field, OpIn, value)
}

// NotIn 添加NOT IN WHERE条件
func (qb *QueryBuilder[T]) NotIn(field string, value any) *QueryBuilder[T] {
	return qb.where(field, OpNotIn, value)
}

// IsNull 添加IS NULL WHERE条件
func (qb *QueryBuilder[T]) IsNull(field string) *QueryBuilder[T] {
	return qb.where(field, OpNull, nil)
}

// IsNotNull 添加IS NOT NULL WHERE条件
func (qb *QueryBuilder[T]) IsNotNull(field string) *QueryBuilder[T] {
	return qb.where(field, OpNotNull, nil)
}

func (qb *QueryBuilder[T]) GetOptions() *QueryOption {
	return &QueryOption{
		OrderBy: qb.orderBy,
		Limit:   qb.limit,
		Offset:  qb.offset,
		Lock:    qb.lock,
		Filters: qb.conditions,
	}
}

type Op string

func (op Op) String() string {
	return string(op)
}

const (
	OpAnd        Op = "and"
	OpOr         Op = "or"
	OpLike       Op = "like"
	OpStartsWith Op = "startswith" // 匹配以指定字符串开头的字段
	OpEndsWith   Op = "endswith"   // 匹配以指定字符串结尾的字段
	OpEq         Op = "eq"
	OpNe         Op = "ne"
	OpGt         Op = "gt"
	OpLt         Op = "lt"
	OpGe         Op = "ge"
	OpLe         Op = "le"
	OpIn         Op = "in"
	OpNotIn      Op = "notin"
	OpNull       Op = "null"
	OpNotNull    Op = "notnull"
)
