package app

import (
	"github.com/ayxworxfr/go_admin/internal/platform/middleware"
	"github.com/ayxworxfr/go_admin/internal/platform/router"
	"github.com/ayxworxfr/go_admin/pkg/context"
)

// protectedProbe 是 /api/protected/test 演示路由所需的最小接口。用类型断言
// 而不是直接依赖 iam 包的具体 Handler 类型，platform/app 才能保持对业务模块
// 一无所知——路由挂载是组合根的职责，不应该反过来耦合某个具体模块。
type protectedProbe interface {
	ProtectedHandler(c *context.Context) *context.Response
}

// SetupRoutes 挂载全部路由。authHandler 负责登录/刷新令牌等公开接口，
// businessHandlers 是需要 JWT 鉴权保护、按方法名自动注册的业务 Handler
// （user/iam/systemsetting 各模块的 Handler 实例），全部由 bootstrap.Container
// 组装后传入，本函数只做路由挂载，不关心每个 Handler 依赖了什么 Service。
func (a *App) SetupRoutes(authHandler any, jwtMiddleware *middleware.JWTAuthMiddleware, businessHandlers ...any) {
	root := a.Group("/")
	root.GET("/health", HelloHandler)
	root.GET("/metrics", HelloHandler)

	api := a.Group("/api")
	api.GET("/hello", HelloHandler)

	// 公开路由：登录 / 刷新令牌
	router.AutoRegister.RegisterStruct(api, authHandler)

	// 使用JWT中间件保护的路由
	protected := api.Group("/protected")
	protected.Use(jwtMiddleware.Handle())

	if probe, ok := authHandler.(protectedProbe); ok {
		protected.GET("/test", probe.ProtectedHandler)
	}

	// 各业务模块路由
	router.AutoRegister.RegisterStruct(protected, businessHandlers...)
}
