package bootstrap

import (
	iamhandler "github.com/ayxworxfr/go_admin/internal/modules/iam/handler"
	sshandler "github.com/ayxworxfr/go_admin/internal/modules/systemsetting/handler"
	userhandler "github.com/ayxworxfr/go_admin/internal/modules/user/handler"
	myapp "github.com/ayxworxfr/go_admin/internal/platform/app"
	"github.com/ayxworxfr/go_admin/internal/platform/middleware"
)

// setupRoutes 用 Container 里装配好的服务构造各模块 Handler，再一次性交给
// App.SetupRoutes 挂载路由。这是组合根里唯一"知道所有模块存在"的地方之一
// （另一个是 Container 本身）。
func setupRoutes(app *myapp.App, c *Container) {
	authHandler := iamhandler.NewAuthHandler(c.Auth, c.JWT)
	jwtMiddleware := middleware.NewJWTMiddleware(c.JWT, c.Checker, c.TokenStore)

	userHandler := userhandler.NewHandler(c.User, c.UserRole, c.Checker)
	roleHandler := iamhandler.NewRoleHandler(c.Role, c.Checker)
	permissionHandler := iamhandler.NewPermissionHandler(c.Permission, c.Checker)
	userRoleHandler := iamhandler.NewUserRoleHandler(c.UserRole, c.Checker)
	systemSettingHandler := sshandler.NewHandler(c.SystemSetting)

	app.SetupRoutes(authHandler, jwtMiddleware,
		userHandler, roleHandler, permissionHandler, userRoleHandler, systemSettingHandler)
}
