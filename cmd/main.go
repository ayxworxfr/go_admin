package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	myapp "github.com/ayxworxfr/go_admin/internal/app"
	"github.com/ayxworxfr/go_admin/internal/config"
	"github.com/ayxworxfr/go_admin/internal/cron"
	"github.com/ayxworxfr/go_admin/internal/dao"
	"github.com/ayxworxfr/go_admin/internal/middleware"
	"github.com/ayxworxfr/go_admin/internal/middleware/sentinel"
	"github.com/ayxworxfr/go_admin/internal/service"
	"github.com/ayxworxfr/go_admin/pkg/jwtauth"
	"github.com/ayxworxfr/go_admin/pkg/logger"
	"github.com/ayxworxfr/go_admin/pkg/utils"
	_ "github.com/go-sql-driver/mysql"
	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
)

func main() {
	cfg := InitConfig()
	app := myapp.NewApp(cfg)
	ctx := context.Background()
	// 初始化日志系统
	if err := InitLogger(cfg.Logger); err != nil {
		panic(fmt.Sprintf("Failed to initialize logger: %v", err))
	}

	app.RegisterInit(func() error {
		configPath := utils.GetAbsPath("conf/sentinel.yaml")
		if err := initService(app); err != nil {
			return errors.Wrap(err, "Failed to initialize service")
		}
		// 异步初始化，加快启动速度
		go func() error {
			if err := sentinel.InitSentinel(configPath); err != nil {
				logger.Errorf(ctx, "Failed to initialize sentinel: %v", err)
				return errors.Wrap(err, "Failed to initialize sentinel")
			}
			// 初始化OpenTelemetry
			if err := initOpenTelemetry(ctx, cfg.OpenTelemetry, app); err != nil {
				logger.Errorf(ctx, "Failed to initialize OpenTelemetry: %v", err)
				return errors.Wrap(err, "Failed to initialize OpenTelemetry")
			}
			return nil
		}()
		return nil
	})

	// 添加中间件
	app.Use(middleware.CorsMiddleware())
	app.Use(sentinel.SentinelMiddleware())
	app.Use(middleware.GlobalErrorHandlerMiddleware())
	app.Use(middleware.LogMiddleware())
	app.Use(middleware.TraceContextMiddleware())
	app.Use(middleware.BindAndValidateMiddleware())

	// 注册路由
	app.SetupRoutes()

	// 启动服务器
	go startServer(app)

	// 优雅关闭
	gracefulShutdown(app)
}

func initOpenTelemetry(ctx context.Context, cfg config.OpenTelemetryConfig, app *myapp.App) error {
	otelProvider, err := myapp.InitOpenTelemetry(cfg)
	if err != nil {
		logger.Errorf(ctx, "Failed to initialize OpenTelemetry: %v", err)
	}
	exitFun := func() error {
		if cfg.Enable {
			if err := otelProvider.Shutdown(ctx); err != nil {
				logger.Errorf(ctx, "Failed to shutdown OpenTelemetry provider: %v", err)
				return err
			}
		}
		return nil
	}
	app.RegisterExit(exitFun)
	return nil
}

func InitConfig() *config.Config {
	// 加载配置
	configPath := utils.GetAbsPath("conf/config.yaml")
	cfg, err := config.Load(configPath)
	if err != nil {
		panic(fmt.Sprintf("Failed to load config: %v", err))
	}

	// 初始化JWT
	if jwt, err := jwtauth.NewJWT(cfg.JWT.Secret, cfg.JWT.AccessTokenExp, cfg.JWT.RefreshTokenExp); err != nil {
		panic(fmt.Sprintf("Failed to initialize JWT: %v", err))
	} else {
		jwtauth.Init(jwt)
	}
	return cfg
}

func InitLogger(cfg config.LoggerConfig) error {
	// 初始化日志系统
	loggerConfig := logger.Config{
		LogFile:    cfg.LogFile,
		Level:      cfg.Level,
		MaxSize:    cfg.MaxSize,
		MaxBackups: cfg.MaxBackups,
		MaxAge:     cfg.MaxAge,
		Compress:   cfg.Compress,
		Console:    cfg.Console,
	}
	logger.InitLogger(loggerConfig)
	return nil
}

func initService(app *myapp.App) error {
	var result *multierror.Error

	// 初始化数据库
	if err := dao.InitRepo(); err != nil {
		result = multierror.Append(result, err)
	}
	// 初始Service层
	if err := service.Init(); err != nil {
		result = multierror.Append(result, err)
	}

	if taskManager, err := cron.InitCronTask(); err != nil {
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
	// 创建一个通道来接收操作系统的信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// 阻塞，直到接收到退出信号
	<-quit
	logger.Info(context.Background(), "Shutting down server...")

	// 设置关闭超时时间
	const shutdownTimeout = 3 * time.Second

	// 调用 GracefulShutdown
	app.GracefulShutdown(shutdownTimeout)

	logger.Info(context.Background(), "Server exiting")
}
