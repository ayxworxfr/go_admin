package config

import (
	"fmt"
	"os"
	"sync"

	"github.com/ayxworxfr/go_admin/pkg/cron"
	"gopkg.in/yaml.v3"
)

// Config 结构体用于存储所有配置
type Config struct {
	Server        ServerConfig        `yaml:"server"`
	Database      DatabaseConfig      `yaml:"database"`
	Redis         RedisConfig         `yaml:"redis"`
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

// RedisConfig 存储 Redis 连接配置（token_store、限流等共享）
type RedisConfig struct {
	Host         string `yaml:"host"`
	Port         int    `yaml:"port"`
	Password     string `yaml:"password"`
	DB           int    `yaml:"db"`
	PoolSize     int    `yaml:"pool_size"`
	MinIdleConns int    `yaml:"min_idle_conns"`
}

// NewRedisConfig 带默认值的 Redis 配置
func NewRedisConfig() RedisConfig {
	return RedisConfig{
		Host:         "127.0.0.1",
		Port:         6379,
		DB:           0,
		PoolSize:     10,
		MinIdleConns: 2,
	}
}

// Addr 返回 host:port
func (c RedisConfig) Addr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

// TokenStoreConfig 令牌撤销存储策略配置（挂在 jwt 下）
type TokenStoreConfig struct {
	// Driver: memory（单机）| redis（多实例共享），取值与 tokenstore.Driver* 对齐
	Driver string `yaml:"driver"`
	// KeyPrefix Redis 键前缀，仅 redis 驱动使用
	KeyPrefix string `yaml:"key_prefix"`
}

// NewTokenStoreConfig 默认使用进程内 memory 驱动
func NewTokenStoreConfig() TokenStoreConfig {
	return TokenStoreConfig{
		Driver:    "memory",
		KeyPrefix: "go_admin:jwt:revoked:",
	}
}

// JWTConfig 存储 JWT 相关配置（签发参数 + 登出撤销策略同属会话生命周期）
type JWTConfig struct {
	Secret          string           `yaml:"secret"`
	AccessTokenExp  string           `yaml:"access_token_exp"`
	RefreshTokenExp string           `yaml:"refresh_token_exp"`
	TokenStore      TokenStoreConfig `yaml:"token_store"`
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
			Redis:         NewRedisConfig(),
			JWT:           JWTConfig{TokenStore: NewTokenStoreConfig()},
			OpenTelemetry: NewOpenTelemetryConfig(),
		}
		err = loadFile(filename, config)
		if err != nil {
			return
		}
		// 环境变量覆盖 YAML（密钥与连接信息以编排/.env 为准）
		applyEnvOverrides(config)
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
