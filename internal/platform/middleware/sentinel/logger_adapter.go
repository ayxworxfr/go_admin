package sentinel

import (
	"go.uber.org/zap"

	"github.com/alibaba/sentinel-golang/logging"
	"github.com/ayxworxfr/go_admin/pkg/logger"
)

// SentinelLogger 实现sentinel的日志接口
type SentinelLogger struct {
	logger *zap.Logger
}

// NewSentinelLogger 创建sentinel日志适配器
func NewSentinelLogger() logging.Logger {
	return &SentinelLogger{
		logger: logger.Instance,
	}
}

// Info 实现Info接口
func (l *SentinelLogger) Info(msg string, keysAndValues ...any) {
	l.logger.Info(msg, convertToZapFields(keysAndValues...)...)
}

// Warn 实现Warn接口
func (l *SentinelLogger) Warn(msg string, keysAndValues ...any) {
	l.logger.Warn(msg, convertToZapFields(keysAndValues...)...)
}

// Error 实现Error接口
func (l *SentinelLogger) Error(err error, msg string, keysAndValues ...any) {
	fields := convertToZapFields(keysAndValues...)
	fields = append(fields, zap.Error(err))
	l.logger.Error(msg, fields...)
}

// Fatal 实现Fatal接口
func (l *SentinelLogger) Fatal(msg string, keysAndValues ...any) {
	l.logger.Fatal(msg, convertToZapFields(keysAndValues...)...)
}

// Debug 实现Debug接口
func (l *SentinelLogger) Debug(msg string, keysAndValues ...any) {
	if l.DebugEnabled() {
		l.logger.Debug(msg, convertToZapFields(keysAndValues...)...)
	}
}

// DebugEnabled 实现DebugEnabled接口
func (l *SentinelLogger) DebugEnabled() bool {
	return l.logger.Core().Enabled(zap.DebugLevel)
}

// IsInfoEnabled 实现IsInfoEnabled接口
func (l *SentinelLogger) IsInfoEnabled() bool {
	return l.logger.Core().Enabled(zap.InfoLevel)
}

func (l *SentinelLogger) InfoEnabled() bool {
	return l.logger.Core().Enabled(zap.InfoLevel)
}

func (l *SentinelLogger) WarnEnabled() bool {
	return l.logger.Core().Enabled(zap.WarnLevel)
}

func (l *SentinelLogger) ErrorEnabled() bool {
	return l.logger.Core().Enabled(zap.ErrorLevel)
}

// 将键值对转换为zap.Field数组
func convertToZapFields(keysAndValues ...any) []zap.Field {
	if len(keysAndValues) == 0 {
		return nil
	}

	// 确保键值对是偶数个
	if len(keysAndValues)%2 != 0 {
		keysAndValues = append(keysAndValues, nil)
	}

	fields := make([]zap.Field, 0, len(keysAndValues)/2)
	for i := 0; i < len(keysAndValues); i += 2 {
		key, ok := keysAndValues[i].(string)
		if !ok {
			continue
		}

		var value any
		if i+1 < len(keysAndValues) {
			value = keysAndValues[i+1]
		}

		fields = append(fields, zap.Any(key, value))
	}

	return fields
}
