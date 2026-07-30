package middleware

import (
	"context"
	"runtime/debug"

	mycontext "github.com/ayxworxfr/go_admin/pkg/context"
	"github.com/ayxworxfr/go_admin/pkg/logger"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"go.uber.org/zap"
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
					zap.Any("error", err),
					zap.String("url", string(c.Request.URI().FullURI())),
					zap.String("method", string(c.Request.Method())),
					zap.String("stack", string(stack)),
				)

				// 返回统一的错误响应
				rsp := mycontext.InternalError()
				c.JSON(consts.StatusInternalServerError, rsp)
				c.Abort()
			}
		}()

		c.Next(ctx)
	}
}
