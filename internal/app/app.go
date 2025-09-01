// internal/app/app.go

package app

import (
	"context"
	"fmt"
	"time"

	"github.com/ayxworxfr/go_admin/internal/app/router"
	"github.com/ayxworxfr/go_admin/internal/config"
	validator "github.com/ayxworxfr/go_admin/internal/domain/validate"
	"github.com/ayxworxfr/go_admin/pkg/logger"
	"github.com/cloudwego/hertz/pkg/app/server"
	hertztracing "github.com/hertz-contrib/obs-opentelemetry/tracing"
	"go.uber.org/zap"
)

type App struct {
	server    *server.Hertz
	config    *config.Config
	initFuncs []func() error
	exitFuncs []func() error
}

func NewApp(cfg *config.Config) *App {
	tracer, tcfg := hertztracing.NewServerTracer()
	h := server.Default(tracer,
		server.WithHostPorts(fmt.Sprintf(":%d", cfg.Server.Port)),
		server.WithCustomBinder(validator.NewDecimalBinder()),
	)
	h.Use(hertztracing.ServerMiddleware(tcfg))
	return &App{
		server: h,
		config: cfg,
	}
}

func (a *App) Run() error {
	ctx := context.Background()
	logger.Info(ctx, "Starting application...")
	if err := a.executeFuns(a.initFuncs...); err != nil {
		panic(fmt.Sprintf("failed to initialize application: %v", err))
	}
	logger.Info(ctx, "Starting server", zap.Int("port", a.config.Server.Port))
	return a.server.Run()
}

func (a *App) GracefulShutdown(timeout time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	a.executeFuns(a.exitFuncs...)

	if err := a.server.Shutdown(ctx); err != nil {
		logger.Error(context.Background(), "Server forced to shutdown", zap.Error(err))
	}
}

func (a *App) Group(path string) *router.RouterGroup {
	return router.NewRouterGroup(a.server.Group(path))
}

func (a *App) RegisterInit(initFuncs ...func() error) {
	a.initFuncs = append(a.initFuncs, initFuncs...)
}

func (a *App) RegisterExit(exitFuncs ...func() error) {
	a.exitFuncs = append(a.exitFuncs, exitFuncs...)
}

func (a *App) executeFuns(funs ...func() error) error {
	for _, fun := range funs {
		if err := fun(); err != nil {
			return err
		}
	}
	return nil
}
