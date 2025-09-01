package middleware

import (
	"context"

	"github.com/ayxworxfr/go_admin/pkg/logger"
	"github.com/cloudwego/hertz/pkg/app"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// TraceContextMiddleware 提取追踪信息并注入到context
func TraceContextMiddleware() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		span := trace.SpanFromContext(ctx)
		spanContext := span.SpanContext()

		// 基础日志字段
		logFields := []zap.Field{
			zap.String("trace_id", spanContext.TraceID().String()),
			zap.String("span_id", spanContext.SpanID().String()),
			zap.String("method", string(c.Method())),
			zap.String("path", string(c.Path())),
			zap.String("client_ip", c.ClientIP()),
			zap.String("user_agent", string(c.UserAgent())),
		}
		newCtx := logger.WithContext(ctx, logFields...)

		c.Next(newCtx)
	}
}
