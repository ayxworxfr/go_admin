package tests

import (
	"fmt"
	"sync"

	"github.com/ayxworxfr/go_admin/internal/config"
	"github.com/ayxworxfr/go_admin/pkg/jwtauth"
	"github.com/ayxworxfr/go_admin/pkg/logger"
	"github.com/ayxworxfr/go_admin/pkg/utils"
)

var (
	once sync.Once
)

// 模拟数据库连接
func init() {
	once.Do(func() {
		cfg := InitConfig()
		InitLogger(cfg.Logger)
	})
}

func InitConfig() *config.Config {
	// 加载配置
	configPath := utils.GetAbsPath("conf/config_test.yaml")
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

func InitLogger(cfg config.LoggerConfig) {
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
}
