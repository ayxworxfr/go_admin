package dao

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ayxworxfr/go_admin/internal/config"
	"github.com/ayxworxfr/go_admin/internal/domain/models"
	"github.com/ayxworxfr/go_admin/pkg/logger"
	"github.com/ayxworxfr/go_admin/pkg/repository"
	"github.com/hashicorp/go-multierror"
	"go.uber.org/zap"
	"xorm.io/xorm"
	"xorm.io/xorm/log"
)

var (
	isSyncDB           bool = false
	engine             *xorm.Engine
	initOnce           sync.Once
	initError          error
	UserRepo           repository.Repository[models.User]
	RoleRepo           repository.Repository[models.Role]
	PermissionRepo     repository.Repository[models.Permission]
	UserRoleRepo       repository.Repository[models.UserRole]
	RolePermissionRepo repository.Repository[models.RolePermission]
	SystemSettingRepo  repository.Repository[models.SystemSetting]
	DataPermissionRepo repository.Repository[models.DataPermission]
)

func InitRepo() error {
	initOnce.Do(func() {
		engine = InitDB()
		if engine == nil {
			initError = fmt.Errorf("failed to initialize database")
			return
		}

		// 使用新的仓储初始化方式
		processor := repository.NewXormProcessor(engine)
		UserRepo = repository.NewRepository[models.User](processor)
		RoleRepo = repository.NewRepository[models.Role](processor)
		PermissionRepo = repository.NewRepository[models.Permission](processor)
		UserRoleRepo = repository.NewRepository[models.UserRole](processor)
		RolePermissionRepo = repository.NewRepository[models.RolePermission](processor)
		SystemSettingRepo = repository.NewRepository[models.SystemSetting](processor)
		DataPermissionRepo = repository.NewRepository[models.DataPermission](processor)
	})

	return initError
}

// InitDB 函数使用配置类初始化 XORM 引擎
func InitDB() *xorm.Engine {
	dbConfig := config.Get().Database

	dataSourceName := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		dbConfig.User,
		dbConfig.Password,
		dbConfig.Host,
		dbConfig.Port,
		dbConfig.DBName)

	engine, err := xorm.NewEngine("mysql", dataSourceName)
	if err != nil {
		logger.Fatalf(context.Background(), "Failed to create XORM engine: %v", err)
		return nil
	}

	// 设置数据库连接池
	engine.SetMaxIdleConns(dbConfig.MaxIdleConns)
	engine.SetMaxOpenConns(dbConfig.MaxOpenConns)
	engine.SetConnMaxLifetime(time.Duration(dbConfig.ConnMaxLifetime) * time.Second)

	// 配置 XORM 日志记录器
	engine.AddHook(NewXormLogger(dbConfig.ShowSQL))

	// 根据配置设置日志级别
	switch config.Get().Logger.Level {
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

	// 同步结构体与数据库表
	if isSyncDB {
		result := SyncDB(engine, false, false)
		if result != nil {
			logger.Fatalf(context.Background(), "Failed to sync database: %v", result)
			return nil
		}
	}

	return engine
}

// SyncDB 同步数据库结构
// dropTables: 是否删除现有表（危险操作，生产环境慎用）
// interactive: 是否启用交互式确认（仅在dropTables为true时生效）
func SyncDB(engine *xorm.Engine, dropTables, interactive bool) error {
	ctx := context.Background()
	var result *multierror.Error

	// 定义需要同步的模型
	modelList := []any{
		new(models.User),
		new(models.Role),
		new(models.Permission),
		new(models.UserRole),
		new(models.RolePermission),
		new(models.SystemSetting),
		new(models.DataPermission),
	}

	// 处理DROP TABLE逻辑
	if dropTables {
		if interactive {
			// 交互式确认
			fmt.Print("警告：即将删除所有表并重新创建！是否继续？(y/N): ")
			var response string
			fmt.Scanln(&response)
			if !strings.EqualFold(response, "y") && !strings.EqualFold(response, "yes") {
				logger.Info(ctx, "同步操作已取消")
				return nil
			}
		}

		// 按逆序删除表（避免外键约束问题）
		for i := len(modelList) - 1; i >= 0; i-- {
			model := modelList[i]
			tableName := engine.TableName(model)
			logger.Info(ctx, "删除表", zap.String("table", tableName))

			if _, err := engine.Exec(fmt.Sprintf("DROP TABLE IF EXISTS `%s`", tableName)); err != nil {
				result = multierror.Append(result, fmt.Errorf("删除表 %s 失败: %w", tableName, err))
			}
		}
	}

	// 同步表结构
	for _, model := range modelList {
		tableName := engine.TableName(model)
		logger.Info(ctx, "同步表结构", zap.String("table", tableName))

		if err := engine.Sync2(model); err != nil {
			result = multierror.Append(result, fmt.Errorf("同步表 %s 失败: %w", tableName, err))
		}
	}

	if result != nil {
		logger.Error(ctx, "数据库同步完成，但存在错误", zap.Error(result))
		return result
	}

	logger.Info(ctx, "数据库同步成功")
	return nil
}
