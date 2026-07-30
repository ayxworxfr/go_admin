package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ayxworxfr/go_admin/internal/bootstrap"
	iamhandler "github.com/ayxworxfr/go_admin/internal/modules/iam/handler"
	sshandler "github.com/ayxworxfr/go_admin/internal/modules/systemsetting/handler"
	userhandler "github.com/ayxworxfr/go_admin/internal/modules/user/handler"
	myapp "github.com/ayxworxfr/go_admin/internal/platform/app"
	"github.com/ayxworxfr/go_admin/internal/platform/config"
	platformcron "github.com/ayxworxfr/go_admin/internal/platform/cron"
	"github.com/ayxworxfr/go_admin/internal/platform/db"
	"github.com/ayxworxfr/go_admin/internal/platform/middleware"
	"github.com/ayxworxfr/go_admin/internal/platform/middleware/sentinel"
	"github.com/ayxworxfr/go_admin/pkg/crypter"
	"github.com/ayxworxfr/go_admin/pkg/jwtauth"
	"github.com/ayxworxfr/go_admin/pkg/logger"
	"github.com/ayxworxfr/go_admin/pkg/utils"
	_ "github.com/go-sql-driver/mysql"
	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
)

func main() {
	cfg := InitConfig()
	if err := InitLogger(cfg.Logger); err != nil {
		panic(fmt.Sprintf("Failed to initialize logger: %v", err))
	}

	jwt, err := jwtauth.NewJWT(cfg.JWT.Secret, cfg.JWT.AccessTokenExp, cfg.JWT.RefreshTokenExp)
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize JWT: %v", err))
	}
	jwtauth.Init(jwt) // pkg/context.Context.GetUserID() 仍依赖这个全局实例

	engine, err := db.NewEngine(cfg)
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize database engine: %v", err))
	}

	container := bootstrap.NewContainer(engine, crypter.NewArgon2Hasher(), jwt)

	app := myapp.NewApp(cfg)
	ctx := context.Background()

	app.RegisterInit(func() error {
		configPath := utils.GetAbsPath("conf/sentinel.yaml")
		if err := initCronTask(app); err != nil {
			return errors.Wrap(err, "Failed to initialize cron task")
		}
		// 异步初始化，加快启动速度
		go func() error {
			if err := sentinel.InitSentinel(configPath); err != nil {
				logger.Errorf(ctx, "Failed to initialize sentinel: %v", err)
				return errors.Wrap(err, "Failed to initialize sentinel")
			}
			if err := initOpenTelemetry(ctx, cfg.OpenTelemetry, app); err != nil {
				logger.Errorf(ctx, "Failed to initialize OpenTelemetry: %v", err)
				return errors.Wrap(err, "Failed to initialize OpenTelemetry")
			}
			return nil
		}()
		return nil
	})

	// 中间件（装饰器链，顺序即执行顺序，本次不改动）
	app.Use(middleware.CorsMiddleware())
	app.Use(sentinel.SentinelMiddleware())
	app.Use(middleware.GlobalErrorHandlerMiddleware())
	app.Use(middleware.LogMiddleware())
	app.Use(middleware.TraceContextMiddleware())
	app.Use(middleware.BindAndValidateMiddleware())

	setupRoutes(app, container)

	go startServer(app)
	gracefulShutdown(app)
}

// setupRoutes 用 Container 里装配好的服务构造各模块 Handler，再一次性交给
// App.SetupRoutes 挂载路由。这是本次重构里唯一"知道所有模块存在"的地方之一
// （另一个是 Container 本身），符合组合根的定义。
func setupRoutes(app *myapp.App, c *bootstrap.Container) {
	authHandler := iamhandler.NewAuthHandler(c.Auth, c.JWT)
	jwtMiddleware := middleware.NewJWTMiddleware(c.JWT, c.Checker, c.TokenStore)

	userHandler := userhandler.NewHandler(c.User, c.UserRole, c.Checker, c.JWT)
	roleHandler := iamhandler.NewRoleHandler(c.Role, c.Checker)
	permissionHandler := iamhandler.NewPermissionHandler(c.Permission, c.Checker)
	userRoleHandler := iamhandler.NewUserRoleHandler(c.UserRole, c.Checker)
	systemSettingHandler := sshandler.NewHandler(c.SystemSetting)

	app.SetupRoutes(authHandler, jwtMiddleware,
		userHandler, roleHandler, permissionHandler, userRoleHandler, systemSettingHandler)
}

func initOpenTelemetry(ctx context.Context, cfg config.OpenTelemetryConfig, app *myapp.App) error {
	otelProvider, err := myapp.InitOpenTelemetry(cfg)
	if err != nil {
		logger.Errorf(ctx, "Failed to initialize OpenTelemetry: %v", err)
	}
	app.RegisterExit(func() error {
		if cfg.Enable && otelProvider != nil {
			if err := otelProvider.Shutdown(ctx); err != nil {
				logger.Errorf(ctx, "Failed to shutdown OpenTelemetry provider: %v", err)
				return err
			}
		}
		return nil
	})
	return nil
}

func InitConfig() *config.Config {
	configPath := utils.GetAbsPath("conf/config.yaml")
	cfg, err := config.Load(configPath)
	if err != nil {
		panic(fmt.Sprintf("Failed to load config: %v", err))
	}
	return cfg
}

func InitLogger(cfg config.LoggerConfig) error {
	logger.InitLogger(logger.Config{
		LogFile:    cfg.LogFile,
		Level:      cfg.Level,
		MaxSize:    cfg.MaxSize,
		MaxBackups: cfg.MaxBackups,
		MaxAge:     cfg.MaxAge,
		Compress:   cfg.Compress,
		Console:    cfg.Console,
	})
	return nil
}

func initCronTask(app *myapp.App) error {
	var result *multierror.Error
	if taskManager, err := platformcron.InitCronTask(); err != nil {
		result = multierror.Append(result, err)
	} else {
		app.RegisterExit(func() error {
			taskManager.Stop()
			return nil
		})
	}
	return result.ErrorOrNil()
}

func startServer(app *myapp.App) {
	if err := app.Run(); err != nil {
		panic(fmt.Sprintf("Failed to start server: %v", err))
	}
}

func gracefulShutdown(app *myapp.App) {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	<-quit
	logger.Info(context.Background(), "Shutting down server...")

	const shutdownTimeout = 3 * time.Second
	app.GracefulShutdown(shutdownTimeout)

	logger.Info(context.Background(), "Server exiting")
}
