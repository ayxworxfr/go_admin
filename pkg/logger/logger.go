package logger

import (
	"context"
	"os"
	"sync/atomic"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	rootLogger atomic.Value
	level      zap.AtomicLevel
	initOnce   atomic.Bool
	Instance   *zap.Logger
)

// 默认配置
const (
	defaultLogFile    = "./logs/app.log"
	defaultLevel      = "info"
	defaultMaxSize    = 100
	defaultMaxBackups = 3
	defaultMaxAge     = 7
	defaultCompress   = true
	defaultConsole    = true
)

type loggerKey struct{}

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

// 确保初始化的内部函数
func ensureInit() {
	if initOnce.Load() {
		return
	}
	defer initOnce.Store(true)

	// 使用默认配置初始化
	InitLogger(Config{
		LogFile:    defaultLogFile,
		Level:      defaultLevel,
		MaxSize:    defaultMaxSize,
		MaxBackups: defaultMaxBackups,
		MaxAge:     defaultMaxAge,
		Compress:   defaultCompress,
		Console:    defaultConsole,
	})
}

// InitLogger 初始化日志系统
func InitLogger(cfg Config) {
	// 检查是否已初始化
	if initOnce.Load() {
		return // 已初始化，直接返回
	}
	defer initOnce.Store(true)

	// 设置默认值
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

	level = zap.NewAtomicLevel()
	setLogLevel(cfg.Level)

	// 配置日志输出
	var writeSyncer zapcore.WriteSyncer
	if cfg.Console {
		writeSyncer = zapcore.NewMultiWriteSyncer(
			zapcore.AddSync(&lumberjack.Logger{
				Filename:   cfg.LogFile,
				MaxSize:    cfg.MaxSize,
				MaxBackups: cfg.MaxBackups,
				MaxAge:     cfg.MaxAge,
				Compress:   cfg.Compress,
			}),
			zapcore.AddSync(os.Stdout),
		)
	} else {
		writeSyncer = zapcore.AddSync(&lumberjack.Logger{
			Filename:   cfg.LogFile,
			MaxSize:    cfg.MaxSize,
			MaxBackups: cfg.MaxBackups,
			MaxAge:     cfg.MaxAge,
			Compress:   cfg.Compress,
		})
	}

	// 配置日志编码器
	encoder := getEncoder()
	core := zapcore.NewCore(encoder, writeSyncer, level)

	// 创建根日志器
	logger := zap.New(core, zap.AddCaller(), zap.AddCallerSkip(2))
	rootLogger.Store(logger)
	Instance = logger
}

func setLogLevel(levelStr string) {
	switch levelStr {
	case "debug":
		level.SetLevel(zapcore.DebugLevel)
	case "info":
		level.SetLevel(zapcore.InfoLevel)
	case "warn":
		level.SetLevel(zapcore.WarnLevel)
	case "error":
		level.SetLevel(zapcore.ErrorLevel)
	case "fatal":
		level.SetLevel(zapcore.FatalLevel)
	default:
		level.SetLevel(zapcore.InfoLevel)
	}
}

func getEncoder() zapcore.Encoder {
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	return zapcore.NewConsoleEncoder(encoderConfig)
}

// FromContext 从上下文中获取日志器
func FromContext(ctx context.Context) *zap.Logger {
	ensureInit()
	if logger, ok := ctx.Value(loggerKey{}).(*zap.Logger); ok {
		return logger
	}
	return rootLogger.Load().(*zap.Logger)
}

// WithContext 创建带字段的上下文日志器
func WithContext(ctx context.Context, fields ...zap.Field) context.Context {
	return context.WithValue(ctx, loggerKey{}, FromContext(ctx).With(fields...))
}

// 泛型日志方法
func log(ctx context.Context, level func(*zap.Logger, string, ...zap.Field), msg string, fields ...zap.Field) {
	level(FromContext(ctx), msg, fields...)
}

// 泛型格式化日志方法
func logf(ctx context.Context, level func(*zap.SugaredLogger, string, ...any), template string, args ...any) {
	level(FromContext(ctx).Sugar(), template, args...)
}

// 日志级别方法
func Debug(ctx context.Context, msg string, fields ...zap.Field) {
	log(ctx, (*zap.Logger).Debug, msg, fields...)
}
func Info(ctx context.Context, msg string, fields ...zap.Field) {
	log(ctx, (*zap.Logger).Info, msg, fields...)
}
func Warn(ctx context.Context, msg string, fields ...zap.Field) {
	log(ctx, (*zap.Logger).Warn, msg, fields...)
}
func Error(ctx context.Context, msg string, fields ...zap.Field) {
	log(ctx, (*zap.Logger).Error, msg, fields...)
}
func Fatal(ctx context.Context, msg string, fields ...zap.Field) {
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

// SetLevel 动态设置日志级别
func SetLevel(levelStr string) {
	switch levelStr {
	case "debug":
		level.SetLevel(zapcore.DebugLevel)
	case "info":
		level.SetLevel(zapcore.InfoLevel)
	case "warn":
		level.SetLevel(zapcore.WarnLevel)
	case "error":
		level.SetLevel(zapcore.ErrorLevel)
	case "fatal":
		level.SetLevel(zapcore.FatalLevel)
	default:
		level.SetLevel(zapcore.InfoLevel)
	}
}

// Sync 同步日志到磁盘
func Sync() error {
	return rootLogger.Load().(*zap.Logger).Sync()
}
