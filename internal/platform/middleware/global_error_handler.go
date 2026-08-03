package middleware

import (
	"context"
	"runtime/debug"

	"github.com/ayxworxfr/go_admin/pkg/api"
	"github.com/ayxworxfr/go_admin/pkg/logger"
	"github.com/cloudwego/hertz/pkg/app"
)

// GlobalErrorMiddleware 是一个中间件，用于捕获 panic 并统一处理错误
func GlobalErrorHandlerMiddleware() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		defer func() {
			if err := recover(); err != nil {
				// 获取堆栈跟踪
				stack := debug.Stack()

				// 使用结构化日志记录错误和堆栈跟踪
				logger.Error(ctx, "Panic occurred",
					logger.Any("error", err),
					logger.String("url", string(c.Request.URI().FullURI())),
					logger.String("method", string(c.Request.Method())),
					logger.String("stack", string(stack)),
				)

				api.Abort(c, api.InternalError())
			}
		}()

		c.Next(ctx)
	}
}
