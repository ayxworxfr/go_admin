package bootstrap

import (
	"fmt"
	"io"

	"github.com/ayxworxfr/go_admin/internal/modules/iam/tokenstore"
	"github.com/ayxworxfr/go_admin/internal/platform/config"
	pkgredis "github.com/ayxworxfr/go_admin/pkg/redis"
)

// newTokenStore 按 jwt.token_store.driver 选择策略。
// closer 仅 redis 驱动非 nil，供组合根在退出时关闭连接。
func newTokenStore(cfg *config.Config) (tokenstore.TokenStore, io.Closer, error) {
	tsCfg := cfg.JWT.TokenStore
	opts := tokenstore.Options{
		Driver:    tsCfg.Driver,
		KeyPrefix: tsCfg.KeyPrefix,
	}

	if tokenstore.NormalizeDriver(tsCfg.Driver) == tokenstore.DriverRedis {
		client, err := pkgredis.New(toRedisOptions(cfg.Redis))
		if err != nil {
			return nil, nil, fmt.Errorf("init redis for jwt.token_store: %w", err)
		}
		opts.Redis = client
	}

	store, err := tokenstore.New(opts)
	if err != nil {
		if opts.Redis != nil {
			_ = opts.Redis.Close()
		}
		return nil, nil, err
	}
	if opts.Redis == nil {
		return store, nil, nil
	}
	return store, opts.Redis, nil
}

func toRedisOptions(cfg config.RedisConfig) pkgredis.Options {
	return pkgredis.Options{
		Addr:         cfg.Addr(),
		Password:     cfg.Password,
		DB:           cfg.DB,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConns,
	}
}
