package config

import (
	"os"
	"strconv"
)

// applyEnvOverrides 用环境变量覆盖 YAML。
// 规则：变量存在且非空才覆盖，避免把合法的空密码/空串误清掉。
//
// 密钥类（Docker / K8s 应以 .env 或 Secret 为唯一来源）：
//
//	DATABASE_PASSWORD、REDIS_PASSWORD、JWT_SECRET
//
// 连接类（编排里改 host 不必改镜像内配置文件）：
//
//	DATABASE_HOST、DATABASE_PORT、DATABASE_USER、DATABASE_NAME
//	REDIS_HOST、REDIS_PORT
//
// 运行类（已有约定，保持兼容）：
//
//	APP_PORT、INSTANCE_ID、OTEL_EXPORTER_OTLP_ENDPOINT、OTEL_EXPORTER_OTLP_PROTOCOL
func applyEnvOverrides(cfg *Config) {
	if cfg == nil {
		return
	}

	// 运行时
	overrideInt(&cfg.Server.Port, "APP_PORT")
	overrideString(&cfg.OpenTelemetry.Service, "INSTANCE_ID")
	overrideString(&cfg.OpenTelemetry.Endpoint, "OTEL_EXPORTER_OTLP_ENDPOINT")
	overrideString(&cfg.OpenTelemetry.Protocol, "OTEL_EXPORTER_OTLP_PROTOCOL")

	// 数据库
	overrideString(&cfg.Database.Host, "DATABASE_HOST")
	overrideInt(&cfg.Database.Port, "DATABASE_PORT")
	overrideString(&cfg.Database.User, "DATABASE_USER")
	overrideString(&cfg.Database.Password, "DATABASE_PASSWORD")
	overrideString(&cfg.Database.DBName, "DATABASE_NAME")

	// Redis
	overrideString(&cfg.Redis.Host, "REDIS_HOST")
	overrideInt(&cfg.Redis.Port, "REDIS_PORT")
	overrideString(&cfg.Redis.Password, "REDIS_PASSWORD")

	// JWT
	overrideString(&cfg.JWT.Secret, "JWT_SECRET")
}

func overrideString(dst *string, key string) {
	if v, ok := lookupNonEmpty(key); ok {
		*dst = v
	}
}

func overrideInt(dst *int, key string) {
	v, ok := lookupNonEmpty(key)
	if !ok {
		return
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return
	}
	*dst = n
}

func lookupNonEmpty(key string) (string, bool) {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return "", false
	}
	return v, true
}
