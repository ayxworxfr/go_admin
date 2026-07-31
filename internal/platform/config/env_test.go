package config

import (
	"testing"
)

func TestApplyEnvOverrides_SecretsAndConnection(t *testing.T) {
	cfg := &Config{
		Server:   ServerConfig{Port: 8888},
		Database: DatabaseConfig{Host: "yaml-host", Port: 3306, User: "yaml-user", Password: "yaml-db-pass", DBName: "yaml-db"},
		Redis:    RedisConfig{Host: "yaml-redis", Port: 6379, Password: "yaml-redis-pass"},
		JWT:      JWTConfig{Secret: "yaml-jwt"},
		OpenTelemetry: OpenTelemetryConfig{
			Service:  "yaml-svc",
			Endpoint: "yaml-endpoint",
			Protocol: "http",
		},
	}

	t.Setenv("APP_PORT", "9999")
	t.Setenv("INSTANCE_ID", "app1")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "otel:4317")
	t.Setenv("OTEL_EXPORTER_OTLP_PROTOCOL", "grpc")
	t.Setenv("DATABASE_HOST", "mysql")
	t.Setenv("DATABASE_PORT", "3307")
	t.Setenv("DATABASE_USER", "go_user")
	t.Setenv("DATABASE_PASSWORD", "from-env-db")
	t.Setenv("DATABASE_NAME", "go_admin")
	t.Setenv("REDIS_HOST", "redis")
	t.Setenv("REDIS_PORT", "6380")
	t.Setenv("REDIS_PASSWORD", "from-env-redis")
	t.Setenv("JWT_SECRET", "from-env-jwt")

	applyEnvOverrides(cfg)

	if cfg.Server.Port != 9999 {
		t.Fatalf("APP_PORT: got %d", cfg.Server.Port)
	}
	if cfg.OpenTelemetry.Service != "app1" || cfg.OpenTelemetry.Endpoint != "otel:4317" || cfg.OpenTelemetry.Protocol != "grpc" {
		t.Fatalf("otel overrides: %+v", cfg.OpenTelemetry)
	}
	if cfg.Database.Host != "mysql" || cfg.Database.Port != 3307 || cfg.Database.User != "go_user" ||
		cfg.Database.Password != "from-env-db" || cfg.Database.DBName != "go_admin" {
		t.Fatalf("database overrides: %+v", cfg.Database)
	}
	if cfg.Redis.Host != "redis" || cfg.Redis.Port != 6380 || cfg.Redis.Password != "from-env-redis" {
		t.Fatalf("redis overrides: %+v", cfg.Redis)
	}
	if cfg.JWT.Secret != "from-env-jwt" {
		t.Fatalf("JWT_SECRET: got %q", cfg.JWT.Secret)
	}
}

func TestApplyEnvOverrides_EmptyEnvKeepsYAML(t *testing.T) {
	cfg := &Config{
		Database: DatabaseConfig{Password: "keep-me"},
		JWT:      JWTConfig{Secret: "keep-jwt"},
		Redis:    RedisConfig{Password: "keep-redis"},
	}

	t.Setenv("DATABASE_PASSWORD", "")
	t.Setenv("JWT_SECRET", "")
	t.Setenv("REDIS_PASSWORD", "")
	t.Setenv("APP_PORT", "not-a-number")

	applyEnvOverrides(cfg)

	if cfg.Database.Password != "keep-me" {
		t.Fatalf("empty DATABASE_PASSWORD should not override, got %q", cfg.Database.Password)
	}
	if cfg.JWT.Secret != "keep-jwt" {
		t.Fatalf("empty JWT_SECRET should not override, got %q", cfg.JWT.Secret)
	}
	if cfg.Redis.Password != "keep-redis" {
		t.Fatalf("empty REDIS_PASSWORD should not override, got %q", cfg.Redis.Password)
	}
}

func TestApplyEnvOverrides_NilSafe(t *testing.T) {
	applyEnvOverrides(nil)
}
