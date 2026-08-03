package bootstrap

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	myapp "github.com/ayxworxfr/go_admin/internal/platform/app"
	"github.com/ayxworxfr/go_admin/internal/platform/config"
	"github.com/ayxworxfr/go_admin/internal/platform/db"
	"github.com/ayxworxfr/go_admin/internal/platform/middleware"
	"github.com/ayxworxfr/go_admin/internal/platform/middleware/sentinel"
	"github.com/ayxworxfr/go_admin/pkg/crypter"
	"github.com/ayxworxfr/go_admin/pkg/jwtauth"
	"github.com/ayxworxfr/go_admin/pkg/logger"
)

const shutdownTimeout = 3 * time.Second

// Run 是进程级组合根：创建基础设施 → 装配 Container → 挂中间件与路由 →
// 阻塞监听信号并优雅退出。cmd/main 只负责加载配置与日志后调用本函数。
func Run(cfg *config.Config) error {
	jwt, err := jwtauth.NewJWT(cfg.JWT.Secret, cfg.JWT.AccessTokenExp, cfg.JWT.RefreshTokenExp)
	if err != nil {
		return fmt.Errorf("failed to initialize JWT: %w", err)
	}

	engine, err := db.NewEngine(cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize database engine: %w", err)
	}

	tokenStore, tokenStoreCloser, err := newTokenStore(cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize token store: %w", err)
	}

	// 必须在 NewApp（内部 NewServerTracer）之前安装 TracerProvider，
	// 否则 Hertz 会绑到 noop，日志里虽有 trace_id，Jaeger 却永远空。
	otelProvider, err := myapp.InitOpenTelemetry(cfg.OpenTelemetry)
	if err != nil {
		logger.Error(context.Background(), "Failed to initialize OpenTelemetry", logger.Err(err))
	}

	container := NewContainer(engine, crypter.NewArgon2Hasher(), jwt, tokenStore)
	app := myapp.NewApp(cfg)

	if otelProvider != nil {
		app.RegisterExit(func() error {
			if err := otelProvider.Shutdown(context.Background()); err != nil {
				logger.Error(context.Background(), "Failed to shutdown OpenTelemetry provider", logger.Err(err))
				return err
			}
			return nil
		})
	}
	if tokenStoreCloser != nil {
		app.RegisterExit(func() error {
			return tokenStoreCloser.Close()
		})
	}

	registerInfra(app)

	// 中间件（装饰器链，顺序即执行顺序）
	app.Use(middleware.CorsMiddleware())
	app.Use(sentinel.SentinelMiddleware())
	app.Use(middleware.MetricsMiddleware())
	app.Use(middleware.GlobalErrorHandlerMiddleware())
	app.Use(middleware.LogMiddleware())
	app.Use(middleware.TraceContextMiddleware())

	setupRoutes(app, container)

	errCh := make(chan error, 1)
	go func() {
		errCh <- app.Run()
	}()

	return awaitShutdown(app, errCh)
}

func awaitShutdown(app *myapp.App, errCh <-chan error) error {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("server stopped unexpectedly: %w", err)
		}
		return nil
	case sig := <-quit:
		logger.Info(context.Background(), "Shutting down server...", logger.String("signal", sig.String()))
		app.GracefulShutdown(shutdownTimeout)
		logger.Info(context.Background(), "Server exiting")
		return nil
	}
}
