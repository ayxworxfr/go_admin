package app

import (
	"github.com/ayxworxfr/go_admin/internal/app/router"
	"github.com/ayxworxfr/go_admin/internal/handler"
	"github.com/ayxworxfr/go_admin/internal/middleware"
)

func (a *App) SetupRoutes() {
	root := a.Group("/")
	root.GET("/health", handler.HelloHandler)
	root.GET("/metrics", handler.HelloHandler)

	api := a.Group("/api")
	api.GET("/hello", handler.HelloHandler)

	// 公开路由
	router.AutoRegister.RegisterStruct(
		api,
		handler.LoginHandlerInstance,
	)

	// 使用JWT中间件保护的路由
	protected := api.Group("/protected")
	protected.Use(middleware.JWTMiddleware())

	protected.GET("/test", handler.LoginHandlerInstance.ProtectedHandler)

	// auth模块路由
	router.AutoRegister.RegisterStruct(protected, handler.AllHandlerInstance...)
	// router.TagRegister.RegisterByPackage(protected, "internal/handler/auth")
}
