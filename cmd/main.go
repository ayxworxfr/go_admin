package main

import (
	"fmt"

	"github.com/ayxworxfr/go_admin/internal/bootstrap"
	"github.com/ayxworxfr/go_admin/internal/platform/config"
	"github.com/ayxworxfr/go_admin/pkg/logger"
	"github.com/ayxworxfr/go_admin/pkg/utils"
	_ "github.com/go-sql-driver/mysql"
)

func main() {
	cfg := loadConfig()
	initLogger(cfg.Logger)

	if err := bootstrap.Run(cfg); err != nil {
		panic(fmt.Sprintf("application stopped: %v", err))
	}
}

func loadConfig() *config.Config {
	configPath := utils.GetAbsPath("conf/config.yaml")
	cfg, err := config.Load(configPath)
	if err != nil {
		panic(fmt.Sprintf("Failed to load config: %v", err))
	}
	return cfg
}

func initLogger(cfg config.LoggerConfig) {
	logger.InitLogger(logger.Config{
		LogFile:    cfg.LogFile,
		Level:      cfg.Level,
		MaxSize:    cfg.MaxSize,
		MaxBackups: cfg.MaxBackups,
		MaxAge:     cfg.MaxAge,
		Compress:   cfg.Compress,
		Console:    cfg.Console,
	})
}
