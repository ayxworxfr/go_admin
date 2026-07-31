package bootstrap

import (
	"context"

	myapp "github.com/ayxworxfr/go_admin/internal/platform/app"
	platformcron "github.com/ayxworxfr/go_admin/internal/platform/cron"
	"github.com/ayxworxfr/go_admin/internal/platform/middleware/sentinel"
	"github.com/ayxworxfr/go_admin/pkg/logger"
	"github.com/ayxworxfr/go_admin/pkg/utils"
	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

// registerInfra 注册启动/退出钩子：cron 失败即阻断启动；
// Sentinel 失败只记日志（可观测性/防护不应拖垮业务端口）。
// OpenTelemetry 必须在 NewApp 之前初始化，见 Run。
func registerInfra(app *myapp.App) {
	app.RegisterInit(func() error {
		if err := initCronTask(app); err != nil {
			return errors.Wrap(err, "failed to initialize cron task")
		}
		initSentinel(context.Background())
		return nil
	})
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

func initSentinel(ctx context.Context) {
	configPath := utils.GetAbsPath("conf/sentinel.yaml")
	if err := sentinel.InitSentinel(configPath); err != nil {
		logger.Error(ctx, "Failed to initialize sentinel", zap.Error(err))
	}
}
