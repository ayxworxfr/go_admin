package sentinel

import (
	"context"

	"github.com/alibaba/sentinel-golang/logging"
	"github.com/ayxworxfr/go_admin/pkg/logger"
)

// SentinelLogger 把 sentinel-golang 的 logging.Logger 适配到本项目的 logger 包。
// 不持有任何底层日志库实例，换库时本适配器无需改动。
type SentinelLogger struct{}

// NewSentinelLogger 创建 sentinel 日志适配器。
func NewSentinelLogger() logging.Logger {
	return &SentinelLogger{}
}

func (l *SentinelLogger) Info(msg string, keysAndValues ...any) {
	logger.Info(context.Background(), msg, kvFields(keysAndValues...)...)
}

func (l *SentinelLogger) Warn(msg string, keysAndValues ...any) {
	logger.Warn(context.Background(), msg, kvFields(keysAndValues...)...)
}

func (l *SentinelLogger) Error(err error, msg string, keysAndValues ...any) {
	fields := kvFields(keysAndValues...)
	if err != nil {
		fields = append(fields, logger.Err(err))
	}
	logger.Error(context.Background(), msg, fields...)
}

func (l *SentinelLogger) Fatal(msg string, keysAndValues ...any) {
	logger.Fatal(context.Background(), msg, kvFields(keysAndValues...)...)
}

func (l *SentinelLogger) Debug(msg string, keysAndValues ...any) {
	if l.DebugEnabled() {
		logger.Debug(context.Background(), msg, kvFields(keysAndValues...)...)
	}
}

func (l *SentinelLogger) DebugEnabled() bool {
	return logger.Enabled("debug")
}

func (l *SentinelLogger) IsInfoEnabled() bool {
	return logger.Enabled("info")
}

func (l *SentinelLogger) InfoEnabled() bool {
	return logger.Enabled("info")
}

func (l *SentinelLogger) WarnEnabled() bool {
	return logger.Enabled("warn")
}

func (l *SentinelLogger) ErrorEnabled() bool {
	return logger.Enabled("error")
}

// kvFields 把 sentinel 的 keysAndValues 展平为 logger.Field。
func kvFields(keysAndValues ...any) []logger.Field {
	if len(keysAndValues) == 0 {
		return nil
	}
	if len(keysAndValues)%2 != 0 {
		keysAndValues = append(keysAndValues, nil)
	}

	fields := make([]logger.Field, 0, len(keysAndValues)/2)
	for i := 0; i < len(keysAndValues); i += 2 {
		key, ok := keysAndValues[i].(string)
		if !ok {
			continue
		}
		var value any
		if i+1 < len(keysAndValues) {
			value = keysAndValues[i+1]
		}
		fields = append(fields, logger.Any(key, value))
	}
	return fields
}
