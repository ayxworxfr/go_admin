// Package logger 提供基于 zap 的结构化日志能力：统一的初始化入口、
// context 绑定字段，以及不依赖具体日志库类型的 Field 构造函数（见 field.go）。
package logger

import (
	"context"
	"os"
	"sync"
	"sync/atomic"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Config 日志配置
type Config struct {
	LogFile    string // 日志文件路径
	Level      string // 日志级别（debug/info/warn/error）
	MaxSize    int    // 单个日志文件最大大小（MB）
	MaxBackups int    // 最大保留日志文件数
	MaxAge     int    // 日志文件最大保存天数
	Compress   bool   // 是否压缩旧日志
	Console    bool   // 是否输出到控制台
}

var (
	rootLogger atomic.Value // *zap.Logger
	level      zap.AtomicLevel
	initOnce   sync.Once
)

// 默认配置
const (
	defaultLogFile    = "./logs/app.log"
	defaultLevel      = "info"
	defaultMaxSize    = 100
	defaultMaxBackups = 3
	defaultMaxAge     = 7
)

// ensureInit 保证在业务代码显式调用 InitLogger 之前也能安全使用全局日志函数
// （WithContext/Debug/Info/... 都会先走这里兜底）。
//
// 这里只做纯 stdout 输出、不落任何文件：兜底路径的调用方（典型场景是测试代码，
// 或者像 pkg/cron 那样把 nil Logger 转发到全局 logger 包的组件）往往不会关心、
// 也不该关心磁盘上多出一个 logs/ 目录。真正需要文件轮转能力的场景，必须在应用
// 启动路径显式调用 InitLogger(cfg)——两者共享同一个 sync.Once，谁先执行谁生效，
// 后来者是 no-op，从根本上避免"先到的兜底覆盖后到的真实配置"之外的并发初始化问题。
func ensureInit() {
	initOnce.Do(func() {
		build(zap.NewAtomicLevelAt(zapcore.InfoLevel), zapcore.AddSync(os.Stdout))
	})
}

// InitLogger 初始化日志系统。进程生命周期内只有第一次调用生效
// （无论是这里，还是被某次日志调用提前触发的 ensureInit 兜底），后续调用均为 no-op。
func InitLogger(cfg Config) {
	initOnce.Do(func() {
		cfg = applyDefaults(cfg)

		lvl := zap.NewAtomicLevel()
		setLevel(lvl, cfg.Level)

		build(lvl, buildWriteSyncer(cfg))
	})
}

// applyDefaults 为零值字段填充默认配置。
func applyDefaults(cfg Config) Config {
	if cfg.LogFile == "" {
		cfg.LogFile = defaultLogFile
	}
	if cfg.Level == "" {
		cfg.Level = defaultLevel
	}
	if cfg.MaxSize == 0 {
		cfg.MaxSize = defaultMaxSize
	}
	if cfg.MaxBackups == 0 {
		cfg.MaxBackups = defaultMaxBackups
	}
	if cfg.MaxAge == 0 {
		cfg.MaxAge = defaultMaxAge
	}
	return cfg
}

// buildWriteSyncer 按配置组装输出目标：始终写文件（带轮转），Console 为真时额外输出到 stdout。
func buildWriteSyncer(cfg Config) zapcore.WriteSyncer {
	fileSyncer := zapcore.AddSync(&lumberjack.Logger{
		Filename:   cfg.LogFile,
		MaxSize:    cfg.MaxSize,
		MaxBackups: cfg.MaxBackups,
		MaxAge:     cfg.MaxAge,
		Compress:   cfg.Compress,
	})
	if cfg.Console {
		return zapcore.NewMultiWriteSyncer(fileSyncer, zapcore.AddSync(os.Stdout))
	}
	return fileSyncer
}

// build 用给定的级别与输出目标组装根日志器，并发布为全局可见状态。
func build(lvl zap.AtomicLevel, writer zapcore.WriteSyncer) {
	level = lvl
	core := zapcore.NewCore(getEncoder(), writer, level)
	l := zap.New(core, zap.AddCaller(), zap.AddCallerSkip(2))
	rootLogger.Store(l)
}

func getEncoder() zapcore.Encoder {
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	return zapcore.NewConsoleEncoder(encoderConfig)
}

// parseLevel 把配置里的级别字符串映射为 zapcore.Level；未知取值一律按 info
// 处理，不返回错误——日志级别配错不应该导致启动失败。
//
// setLevel（初始化/动态调整）与 Enabled（第三方日志适配器的短路判断）曾经
// 各自维护一份相同的 switch，这里收敛成唯一实现，避免两处映射表逐渐分叉。
func parseLevel(levelStr string) zapcore.Level {
	switch levelStr {
	case "debug":
		return zapcore.DebugLevel
	case "info":
		return zapcore.InfoLevel
	case "warn":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	case "fatal":
		return zapcore.FatalLevel
	default:
		return zapcore.InfoLevel
	}
}

// setLevel 是 SetLevel/InitLogger 共用的级别设置逻辑，接收显式的 AtomicLevel
// 而不是直接操作包级变量，方便在未完成全局初始化的场景下复用。
func setLevel(lvl zap.AtomicLevel, levelStr string) {
	lvl.SetLevel(parseLevel(levelStr))
}

type loggerKey struct{}

// root 返回当前根日志器；调用前必须保证 ensureInit 已执行。
func root() *zap.Logger {
	return rootLogger.Load().(*zap.Logger)
}

// fromContext 从上下文中取出带字段的日志器；没有则回退到根日志器。
// 不导出：业务代码不应持有底层 *zap.Logger。
func fromContext(ctx context.Context) *zap.Logger {
	ensureInit()
	if l, ok := ctx.Value(loggerKey{}).(*zap.Logger); ok {
		return l
	}
	return root()
}

// WithContext 把一组结构化字段绑定到新的 context 上。
// 之后通过该 ctx 打出的日志都会自动带上这些字段（典型用途：请求级 trace_id）。
func WithContext(ctx context.Context, fields ...Field) context.Context {
	return context.WithValue(ctx, loggerKey{}, fromContext(ctx).With(toZap(fields)...))
}

func log(ctx context.Context, fn func(*zap.Logger, string, ...zap.Field), msg string, fields ...Field) {
	fn(fromContext(ctx), msg, toZap(fields)...)
}

func logf(ctx context.Context, fn func(*zap.SugaredLogger, string, ...any), template string, args ...any) {
	fn(fromContext(ctx).Sugar(), template, args...)
}

// 日志级别方法
func Debug(ctx context.Context, msg string, fields ...Field) {
	log(ctx, (*zap.Logger).Debug, msg, fields...)
}
func Info(ctx context.Context, msg string, fields ...Field) {
	log(ctx, (*zap.Logger).Info, msg, fields...)
}
func Warn(ctx context.Context, msg string, fields ...Field) {
	log(ctx, (*zap.Logger).Warn, msg, fields...)
}
func Error(ctx context.Context, msg string, fields ...Field) {
	log(ctx, (*zap.Logger).Error, msg, fields...)
}
func Fatal(ctx context.Context, msg string, fields ...Field) {
	log(ctx, (*zap.Logger).Fatal, msg, fields...)
}

// 格式化日志方法
func Debugf(ctx context.Context, template string, args ...any) {
	logf(ctx, (*zap.SugaredLogger).Debugf, template, args...)
}
func Infof(ctx context.Context, template string, args ...any) {
	logf(ctx, (*zap.SugaredLogger).Infof, template, args...)
}
func Warnf(ctx context.Context, template string, args ...any) {
	logf(ctx, (*zap.SugaredLogger).Warnf, template, args...)
}
func Errorf(ctx context.Context, template string, args ...any) {
	logf(ctx, (*zap.SugaredLogger).Errorf, template, args...)
}
func Fatalf(ctx context.Context, template string, args ...any) {
	logf(ctx, (*zap.SugaredLogger).Fatalf, template, args...)
}

// SetLevel 动态设置日志级别。日志系统尚未初始化时会先兜底初始化，
// 避免对零值 zap.AtomicLevel 调 SetLevel 导致的空指针 panic。
func SetLevel(levelStr string) {
	ensureInit()
	setLevel(level, levelStr)
}

// Enabled 查询指定级别当前是否会真正输出。供适配第三方日志接口时做短路判断。
// levelStr 取值与 Config.Level 一致：debug/info/warn/error/fatal；未知值按 info 处理。
func Enabled(levelStr string) bool {
	ensureInit()
	return level.Enabled(parseLevel(levelStr))
}

// Sync 同步日志到磁盘
func Sync() error {
	ensureInit()
	return root().Sync()
}
