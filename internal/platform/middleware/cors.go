package middleware

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
)

const DefaultAllowOrigin = "*"

// CorsMiddleware 跨域中间件
func CorsMiddleware(allowOrigin ...string) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		realAllowOrigin := DefaultAllowOrigin
		if len(allowOrigin) > 0 {
			realAllowOrigin = allowOrigin[0]
		}
		// 允许的源，* 表示允许所有源，生产环境应指定具体域名
		c.Response.Header.Set("Access-Control-Allow-Origin", realAllowOrigin)

		// 允许的请求方法
		c.Response.Header.Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")

		// 允许的请求头
		c.Response.Header.Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Token, X-Requested-With, Sec-Ch-Ua")

		// 允许携带认证信息（如Cookie）
		c.Response.Header.Set("Access-Control-Allow-Credentials", "true")

		// 预检请求缓存时间（秒）
		c.Response.Header.Set("Access-Control-Max-Age", "86400")

		// 处理预检请求（OPTIONS）
		if string(c.Request.Method()) == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		// 继续处理正常请求
		c.Next(ctx)
	}
}
