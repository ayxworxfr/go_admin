package config

import (
	"os"
	"sync"

	"github.com/ayxworxfr/go_admin/pkg/cron"
	"gopkg.in/yaml.v3"
)

// Config 结构体用于存储所有配置
type Config struct {
	Server        ServerConfig        `yaml:"server"`
	Database      DatabaseConfig      `yaml:"database"`
	JWT           JWTConfig           `yaml:"jwt"`
	Logger        LoggerConfig        `yaml:"logger"`
	OpenTelemetry OpenTelemetryConfig `yaml:"opentelemetry"`
	Tasks         []cron.TaskConfig   `yaml:"tasks"`
}

// ServerConfig 存储服务器相关配置
type ServerConfig struct {
	Port int `yaml:"port"`
}

// DatabaseConfig 存储数据库相关配置
type DatabaseConfig struct {
	Host            string `yaml:"host"`
	Port            int    `yaml:"port"`
	User            string `yaml:"user"`
	Password        string `yaml:"password"`
	DBName          string `yaml:"dbname"`
	MaxIdleConns    int    `yaml:"max_idle_conns"`
	MaxOpenConns    int    `yaml:"max_open_conns"`
	ConnMaxLifetime int    `yaml:"conn_max_lifetime"` // 以秒为单位
	ShowSQL         bool   `yaml:"show_sql"`
}

// NewDatabaseConfig 创建一个带有默认值的 DatabaseConfig
func NewDatabaseConfig() DatabaseConfig {
	return DatabaseConfig{
		MaxIdleConns:    10,
		MaxOpenConns:    100,
		ConnMaxLifetime: 3600, // 默认1小时
		ShowSQL:         true,
	}
}

// JWTConfig 存储 JWT 相关配置
type JWTConfig struct {
	Secret          string `yaml:"secret"`
	AccessTokenExp  string `yaml:"access_token_exp"`
	RefreshTokenExp string `yaml:"refresh_token_exp"`
}

// LoggerConfig 存储日志相关配置
type LoggerConfig struct {
	LogFile    string `yaml:"log_file"`
	Level      string `yaml:"level"`
	MaxSize    int    `yaml:"max_size"`
	MaxBackups int    `yaml:"max_backups"`
	MaxAge     int    `yaml:"max_age"`
	Compress   bool   `yaml:"compress"`
	Console    bool   `yaml:"console"`
}

var (
	config *Config
	once   sync.Once
)

// Load 加载并解析 YAML 配置文件
func Load(filename string) (*Config, error) {
	var err error
	once.Do(func() {
		config = &Config{
			Database:      NewDatabaseConfig(), // 使用带有默认值的 DatabaseConfig
			OpenTelemetry: NewOpenTelemetryConfig(),
		}
		err = loadFile(filename, config)

		// 优先使用环境变量的值
		if instanceID := os.Getenv("INSTANCE_ID"); instanceID != "" {
			config.OpenTelemetry.Service = instanceID
		}
		if endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"); endpoint != "" {
			config.OpenTelemetry.Endpoint = endpoint
		}
		if protocol := os.Getenv("OTEL_EXPORTER_OTLP_PROTOCOL"); protocol != "" {
			config.OpenTelemetry.Protocol = protocol
		}
	})
	return config, err
}

// loadFile 读取并解析 YAML 文件
func loadFile(filename string, cfg *Config) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, cfg)
}

// Get 返回已加载的配置
func Get() *Config {
	return config
}

func GetCronTasks() []cron.TaskConfig {
	if config != nil {
		return config.Tasks
	}

	return nil
}

func GetAppPort() int {
	if config != nil {
		return config.Server.Port
	}

	return 0
}
