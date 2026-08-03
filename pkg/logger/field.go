package logger

import (
	"time"

	"go.uber.org/zap"
)

// Field 是结构化日志的一个键值对。
//
// 调用方只能通过本包提供的构造函数（String / Int / Err 等）创建 Field，
// 绝不能直接依赖底层日志库的 Field 类型。换日志库时，只需改本包内部实现，
// 业务代码的 logger.Info(ctx, "msg", logger.String("k", "v")) 调用形态保持不变。
type Field struct {
	zapField zap.Field
}

// String 字符串字段。
func String(key, val string) Field {
	return Field{zapField: zap.String(key, val)}
}

// Strings 字符串切片字段。
func Strings(key string, val []string) Field {
	return Field{zapField: zap.Strings(key, val)}
}

// Int int 字段。
func Int(key string, val int) Field {
	return Field{zapField: zap.Int(key, val)}
}

// Int64 int64 字段。
func Int64(key string, val int64) Field {
	return Field{zapField: zap.Int64(key, val)}
}

// Uint64 uint64 字段。
func Uint64(key string, val uint64) Field {
	return Field{zapField: zap.Uint64(key, val)}
}

// Uint64s uint64 切片字段。
func Uint64s(key string, val []uint64) Field {
	return Field{zapField: zap.Uint64s(key, val)}
}

// Bool bool 字段。
func Bool(key string, val bool) Field {
	return Field{zapField: zap.Bool(key, val)}
}

// Float64 float64 字段。
func Float64(key string, val float64) Field {
	return Field{zapField: zap.Float64(key, val)}
}

// Duration time.Duration 字段。
func Duration(key string, val time.Duration) Field {
	return Field{zapField: zap.Duration(key, val)}
}

// Any 任意类型字段；优先使用更具体的构造函数（String / Int / Err 等），
// 只有确实无法归类时才退到 Any。
func Any(key string, val any) Field {
	return Field{zapField: zap.Any(key, val)}
}

// Err 错误字段。键名固定为 "error"（与主流结构化日志约定一致）。
// 命名刻意避开 Error，以免与日志级别方法 Error(ctx, msg, ...) 冲突。
func Err(err error) Field {
	return Field{zapField: zap.Error(err)}
}

// toZap 把本包 Field 切片转成底层实现所需的类型。仅限包内使用。
func toZap(fields []Field) []zap.Field {
	if len(fields) == 0 {
		return nil
	}
	out := make([]zap.Field, len(fields))
	for i := range fields {
		out[i] = fields[i].zapField
	}
	return out
}
