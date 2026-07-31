package tokenstore

import (
	"fmt"
	"strings"

	pkgredis "github.com/ayxworxfr/go_admin/pkg/redis"
)

// 驱动名（与 jwt.token_store.driver 配置值对齐）
const (
	DriverMemory = "memory"
	DriverRedis  = "redis"
)

// Options 构造 TokenStore 的运行时参数。Redis 仅在 Driver=redis 时需要。
type Options struct {
	Driver    string
	KeyPrefix string
	Redis     *pkgredis.Client
}

// New 按驱动名创建 TokenStore（工厂 / 策略选择）。driver 为空时回落 memory。
func New(opts Options) (TokenStore, error) {
	switch NormalizeDriver(opts.Driver) {
	case DriverMemory:
		return NewInMemoryTokenStore(), nil
	case DriverRedis:
		if opts.Redis == nil {
			return nil, fmt.Errorf("token store driver %q requires redis client", DriverRedis)
		}
		return NewRedisTokenStore(opts.Redis, opts.KeyPrefix), nil
	default:
		return nil, fmt.Errorf("unknown token store driver %q (want %q or %q)",
			opts.Driver, DriverMemory, DriverRedis)
	}
}

// NormalizeDriver 归一化驱动名，供组合根判断是否需要建 Redis 连接。
func NormalizeDriver(driver string) string {
	switch strings.ToLower(strings.TrimSpace(driver)) {
	case "", DriverMemory, "inmemory", "in-memory":
		return DriverMemory
	case DriverRedis:
		return DriverRedis
	default:
		return strings.ToLower(strings.TrimSpace(driver))
	}
}
