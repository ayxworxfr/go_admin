package app

import (
	"github.com/cloudwego/hertz/pkg/app"
)

// MiddlewareFunc 定义了中间件函数的签名，与 Hertz 的 HandlerFunc 格式一致
type MiddlewareFunc = app.HandlerFunc

// Use 方法用于添加中间件
func (a *App) Use(middlewares ...MiddlewareFunc) {
	for _, m := range middlewares {
		a.server.Use(m)
	}
}
