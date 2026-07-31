package repository

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"xorm.io/xorm"
)

var (
	// ErrNotFound 查询期望单条记录但结果为空
	ErrNotFound = errors.New("record not found")
	// ErrMultiple 查询期望单条记录但匹配到多条
	ErrMultiple = errors.New("multiple records found")
	// ErrNoPrimaryKey 模型缺少可识别的主键（xorm pk 标签）
	ErrNoPrimaryKey = errors.New("model has no primary key field")
	// ErrInvalidModel 传入的模型不是可用的结构体指针
	ErrInvalidModel = errors.New("model must be a non-nil pointer to struct")
)

// 自定义上下文键类型，避免与其他包的键冲突
type transactionKey struct{}

// DB 是仓储层的会话与事务门面。所有 Repository 共享同一个 *DB，
// 事务通过 context 传递 session，跨多个 Repository 的写操作才能落在同一事务里。
type DB struct {
	engine *xorm.Engine
}

// New 基于 xorm.Engine 创建仓储入口，替代原先的 NewXormProcessor。
func New(engine *xorm.Engine) *DB {
	if engine == nil {
		panic("repository: engine is nil")
	}
	return &DB{engine: engine}
}

// Engine 暴露底层引擎，仅供迁移/测试等基础设施使用。
func (db *DB) Engine() *xorm.Engine {
	return db.engine
}

// Transaction 以事务方式执行一组操作，自动管理生命周期并处理 panic。
//
// 若 ctx 已处于事务中，则复用当前事务（不开启嵌套事务），保证
// CreateRole 内再调 AssignRolePermissions 这类组合调用仍在同一事务里。
//
// 使用示例：
//
//	err := repo.Transaction(ctx, func(txCtx context.Context) error {
//	    if err := repo.Create(txCtx, &user); err != nil {
//	        return err
//	    }
//	    return otherRepo.Update(txCtx, &order)
//	})
func (db *DB) Transaction(ctx context.Context, fn func(ctx context.Context) error) error {
	if _, ok := sessionFrom(ctx); ok {
		return fn(ctx)
	}

	session := db.engine.NewSession()
	defer session.Close()

	if err := session.Begin(); err != nil {
		return err
	}
	txCtx := context.WithValue(ctx, transactionKey{}, session)

	committed := false
	defer func() {
		if p := recover(); p != nil {
			_ = session.Rollback()
			panic(p)
		}
		if !committed {
			_ = session.Rollback()
		}
	}()

	if err := fn(txCtx); err != nil {
		return err
	}
	if err := session.Commit(); err != nil {
		return err
	}
	committed = true
	return nil
}

func sessionFrom(ctx context.Context) (*xorm.Session, bool) {
	if ctx == nil {
		return nil, false
	}
	session, ok := ctx.Value(transactionKey{}).(*xorm.Session)
	if !ok || session == nil {
		return nil, false
	}
	return session, true
}

// withSession 在事务 session（若有）或短生命周期 session 上执行 fn。
// 非事务路径创建的 session 会在结束后 Close，杜绝泄漏。
func (db *DB) withSession(ctx context.Context, fn func(*xorm.Session) error) error {
	if session, ok := sessionFrom(ctx); ok {
		return fn(session.Context(ctx))
	}
	session := db.engine.NewSession().Context(ctx)
	defer session.Close()
	return fn(session)
}

func mustDB(db *DB) *DB {
	if db == nil {
		panic("repository: DB is nil")
	}
	return db
}

func wrapDBErr(op string, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrNotFound) || errors.Is(err, ErrMultiple) ||
		errors.Is(err, ErrNoPrimaryKey) || errors.Is(err, ErrInvalidModel) {
		return err
	}
	return fmt.Errorf("repository %s: %w", op, err)
}

// recordSQLEvent 控制是否把 SQL 执行信息写入当前 span。
// 默认开启；压测或批量导入时可关闭以避免事件风暴。
var recordSQLEvent atomic.Bool

func init() {
	recordSQLEvent.Store(true)
}

// SetRecordSQLEvent 开关 SQL span event 记录。
func SetRecordSQLEvent(enabled bool) {
	recordSQLEvent.Store(enabled)
}

// RecordDbEvent 将数据库执行信息写入当前 OpenTelemetry span。
// 供仓储内部与 platform/db 的 xorm 钩子共用。
func RecordDbEvent(ctx context.Context, info map[string]any) {
	if !recordSQLEvent.Load() {
		return
	}
	span := trace.SpanFromContext(ctx)
	if !span.IsRecording() {
		return
	}
	attributes := make([]attribute.KeyValue, 0, len(info))
	for k, v := range info {
		attributes = append(attributes, attribute.String(k, fmt.Sprintf("%v", v)))
	}
	span.AddEvent("db_execute_info", trace.WithAttributes(attributes...))
}
