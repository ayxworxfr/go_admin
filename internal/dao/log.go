package dao

import (
	"context"
	"fmt"
	"time"

	"github.com/ayxworxfr/go_admin/pkg/logger"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"xorm.io/xorm/contexts"
)

type XormLogger struct {
	showSQL       bool
	recordEvent   bool
	slowThreshold time.Duration
}

func NewXormLogger(showSQL bool) *XormLogger {
	return &XormLogger{
		showSQL:       showSQL,
		recordEvent:   true,
		slowThreshold: 100 * time.Millisecond, // slow query threshold
	}
}
func (s *XormLogger) BeforeProcess(c *contexts.ContextHook) (context.Context, error) {
	return c.Ctx, nil
}

func (s *XormLogger) AfterProcess(c *contexts.ContextHook) error {
	if !s.showSQL {
		return nil
	}

	if c.ExecuteTime > s.slowThreshold {
		logger.Warnf(c.Ctx, "Slow SQL: %s, Args: %v, ExecTime: %v", c.SQL, c.Args, c.ExecuteTime)
	} else if c.ExecuteTime > 0 {
		logger.Infof(c.Ctx, "SQL: %s, Args: %v, ExecTime: %v", c.SQL, c.Args, c.ExecuteTime)
	}
	s.recordExecuteInfo(c.Ctx, c.SQL, c.Args, c.ExecuteTime)
	return nil
}

func (s *XormLogger) recordExecuteInfo(ctx context.Context, sql string, args []any, duration time.Duration) {
	info := map[string]any{
		"sql":      sql,
		"args":     args,
		"duration": fmt.Sprintf("%v", duration),
	}
	if len(args) == 0 {
		delete(info, "args")
	}
	if !s.recordEvent {
		return
	}
	RecordDbEvent(ctx, info)
}

func RecordDbEvent(ctx context.Context, info map[string]any) {
	span := trace.SpanFromContext(ctx)
	attributes := make([]attribute.KeyValue, 0, len(info))
	for k, v := range info {
		attributes = append(attributes, attribute.String(k, fmt.Sprintf("%v", v)))
	}
	span.AddEvent("db_execute_info", trace.WithAttributes(
		attributes...,
	))
}
