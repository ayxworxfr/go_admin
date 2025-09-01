package repository

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

var IsRecordSQLEvent = true

// QueryOption 查询选项
type QueryOption struct {
	OrderBy string
	Limit   int
	Offset  int
	Lock    string
	Filters []Condition
}

// Transaction函数定义
type TransactionFunc func(ctx context.Context) (any, error)

// TransactionExecutor 定义事务执行接口
type TransactionExecutor interface {
	// Begin 开始一个数据库事务，返回事务对象。
	// 事务对象类型由具体实现决定，需传递给 Commit 或 Rollback。
	Begin(ctx context.Context) (context.Context, error)

	// Commit 提交一个事务。
	// 参数 tx 必须是通过 Begin 方法获取的事务对象。
	Commit(tx context.Context) error

	// Rollback 回滚一个事务。
	// 参数 tx 必须是通过 Begin 方法获取的事务对象。通常在发生错误时调用。
	Rollback(tx context.Context) error

	// Transaction 以事务方式执行一组操作，确保操作的原子性。
	// 该方法会自动管理事务的生命周期（开始、提交或回滚），并处理 panic 情况。
	//
	// 参数 fn 是一个包含需要在事务中执行的所有操作的函数，
	// 这些操作应使用传入的 ctx 上下文（该上下文包含事务对象）。
	//
	// 使用示例：
	// err := repo.Transaction(ctx, func(ctx context.Context) error {
	//     if err := repo.Create(ctx, &user); err != nil {
	//         return err
	//     }
	//     if err := repo.Update(ctx, &order); err != nil {
	//         return err
	//     }
	//     return nil
	// })
	Transaction(ctx context.Context, fn TransactionFunc) (any, error)
}

// ORMProcessor 定义了通用的 ORM 操作接口，用于与数据库交互。
// 实现该接口的具体处理器（如 XormProcessor、GORMProcessor）负责将这些操作映射到底层 ORM 实现。
type ORMProcessor interface {
	// Create 插入单条记录到数据库。
	// 参数 model 必须是指向结构体的指针，结构体字段需通过标签指定数据库列名。
	Create(ctx context.Context, model any) error

	// Update 更新数据库中的记录。
	// 默认根据 model 中的 ID 字段定位记录，更新非零值字段。
	Update(ctx context.Context, model any) error

	// UpdateByOption 根据查询选项更新单条记录。
	// 参数 model 必须是指向结构体的指针，结构体字段需通过标签指定数据库列名。
	UpdateByOption(ctx context.Context, model any, opts *QueryOption) error

	// Delete 根据 model 中的主键删除记录。
	// model 必须包含有效的主键值。
	Delete(ctx context.Context, model any) error

	// DeleteByOption 根据查询选项删除多条记录。
	// 参数 opts 可指定过滤条件、排序等，实现批量删除。
	DeleteByOption(ctx context.Context, model any, opts *QueryOption) error

	// Query 根据查询选项执行查询，返回多条记录。
	// model 用于指定查询的表结构，opts 包含过滤、排序、分页等条件。
	// 返回结果中的 Data 字段需转换为对应的切片类型。
	Query(ctx context.Context, model any, opts *QueryOption) (*QueryResult, error)

	// BatchCreate 批量插入多条记录。
	// models 应为结构体切片，每个结构体代表一条记录。
	BatchCreate(ctx context.Context, models []any) error

	// BatchUpdate 批量更新多条记录。
	// models 中的每条记录需包含主键值，用于定位要更新的记录。
	BatchUpdate(ctx context.Context, models []any) error

	// BatchDelete 批量删除多条记录。
	// models 中的每条记录需包含主键值，用于定位要删除的记录。
	BatchDelete(ctx context.Context, models []any) error

	// Exec 执行 SQL 语句（如 INSERT、UPDATE、DELETE），返回受影响的行数。
	// 用于执行自定义 SQL，需确保 SQL 语句的安全性。
	Exec(ctx context.Context, sql string, args ...any) (int64, error)

	// QueryRow 执行查询并返回单行结果。
	// 返回 map[string]any 类型，键为列名，值为列值。
	QueryRow(ctx context.Context, sql string, args ...any) (map[string]any, error)

	// QueryRows 执行查询并返回多行结果。
	// 返回 []map[string]any 类型，每个 map 代表一行记录。
	QueryRows(ctx context.Context, sql string, args ...any) ([]map[string]any, error)

	// BuildFiltersFromModel 从模型对象中提取非零值字段，生成查询条件。
	// 用于自动构建基于模型字段的查询过滤器。
	BuildFiltersFromModel(model any) []Condition

	TransactionExecutor
}

// QueryResult 查询结果
type QueryResult struct {
	Data  any
	Total int64
}

// 自定义上下文键类型，避免与其他包的键冲突
type transactionKey struct{}

// 事务键实例
var TransactionKeyInstance = transactionKey{}

func RecordDbEvent(ctx context.Context, info map[string]any) {
	if !IsRecordSQLEvent {
		return
	}
	span := trace.SpanFromContext(ctx)
	attributes := make([]attribute.KeyValue, 0, len(info))
	for k, v := range info {
		attributes = append(attributes, attribute.String(k, fmt.Sprintf("%v", v)))
	}
	span.AddEvent("db_execute_info", trace.WithAttributes(
		attributes...,
	))
}
