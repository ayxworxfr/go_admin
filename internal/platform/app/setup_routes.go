package app

import (
	"github.com/ayxworxfr/go_admin/internal/platform/middleware"
	"github.com/ayxworxfr/go_admin/internal/platform/router"
)

// SetupRoutes 挂载全部路由。authHandler 负责登录/刷新令牌等公开接口，
// businessHandlers 是需要 JWT 鉴权保护、按方法名自动注册的业务 Handler
// （user/iam/systemsetting 各模块的 Handler 实例），全部由 bootstrap.Container
// 组装后传入，本函数只做路由挂载，不关心每个 Handler 依赖了什么 Service。
func (a *App) SetupRoutes(authHandler any, jwtMiddleware *middleware.JWTAuthMiddleware, businessHandlers ...any) {
	root := a.Group("/")
	root.GET("/health", HealthHandler)
	root.GET("/metrics", MetricsHandler())

	api := a.Group("/api")
	reg := router.NewRegister()

	// 公开路由：登录 / 刷新令牌
	reg.RegisterStruct(api, authHandler)

	// 使用JWT中间件保护的路由
	protected := api.Group("/protected")
	protected.Use(jwtMiddleware.Handle())

	// 各业务模块路由
	reg.RegisterStruct(protected, businessHandlers...)
}
