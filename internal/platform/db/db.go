package db

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ayxworxfr/go_admin/internal/platform/config"
	"github.com/ayxworxfr/go_admin/pkg/logger"
	"github.com/hashicorp/go-multierror"
	"go.uber.org/zap"
	"xorm.io/xorm"
	"xorm.io/xorm/log"
)

// NewEngine 使用配置创建 XORM 引擎。相比旧版 dao.InitDB 藏在 dao 包里靠
// sync.Once 变成隐式单例，这里改成一个普通的构造函数，引擎实例由
// bootstrap.Container 持有并显式传给各模块的仓储，谁用谁拿，不再靠全局变量找。
func NewEngine(cfg *config.Config) (*xorm.Engine, error) {
	dbConfig := cfg.Database
	dataSourceName := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		dbConfig.User, dbConfig.Password, dbConfig.Host, dbConfig.Port, dbConfig.DBName)

	engine, err := xorm.NewEngine("mysql", dataSourceName)
	if err != nil {
		return nil, fmt.Errorf("failed to create XORM engine: %w", err)
	}

	engine.SetMaxIdleConns(dbConfig.MaxIdleConns)
	engine.SetMaxOpenConns(dbConfig.MaxOpenConns)
	engine.SetConnMaxLifetime(time.Duration(dbConfig.ConnMaxLifetime) * time.Second)
	engine.AddHook(NewXormLogger(dbConfig.ShowSQL))

	switch cfg.Logger.Level {
	case "debug":
		engine.Logger().SetLevel(log.LOG_DEBUG)
	case "info":
		engine.Logger().SetLevel(log.LOG_INFO)
	case "warn":
		engine.Logger().SetLevel(log.LOG_WARNING)
	case "error":
		engine.Logger().SetLevel(log.LOG_ERR)
	default:
		engine.Logger().SetLevel(log.LOG_INFO)
	}

	return engine, nil
}

// SyncModels 同步数据库表结构。models 由 bootstrap.Container 收集各模块的
// model 列表后传入——db 包本身不知道有哪些模块，避免反向依赖 modules/*。
//
// dropTables: 是否删除现有表（危险操作，生产环境慎用）
// interactive: 是否启用交互式确认（仅在 dropTables 为 true 时生效）
func SyncModels(engine *xorm.Engine, models []any, dropTables, interactive bool) error {
	ctx := context.Background()
	var result *multierror.Error

	if dropTables {
		if interactive {
			fmt.Print("WARNING: This will DROP all tables and recreate them. Continue? (y/N): ")
			var response string
			fmt.Scanln(&response)
			if !strings.EqualFold(response, "y") && !strings.EqualFold(response, "yes") {
				logger.Info(ctx, "Schema sync cancelled")
				return nil
			}
		}

		for i := len(models) - 1; i >= 0; i-- {
			tableName := engine.TableName(models[i])
			logger.Info(ctx, "Dropping table", zap.String("table", tableName))
			if _, err := engine.Exec(fmt.Sprintf("DROP TABLE IF EXISTS `%s`", tableName)); err != nil {
				result = multierror.Append(result, fmt.Errorf("failed to drop table %s: %w", tableName, err))
			}
		}
	}

	for _, model := range models {
		tableName := engine.TableName(model)
		logger.Info(ctx, "Syncing table schema", zap.String("table", tableName))
		if err := engine.Sync2(model); err != nil {
			result = multierror.Append(result, fmt.Errorf("failed to sync table %s: %w", tableName, err))
		}
	}

	if result != nil {
		logger.Error(ctx, "Schema sync finished with errors", zap.Error(result))
		return result
	}

	logger.Info(ctx, "Schema sync completed successfully")
	return nil
}
