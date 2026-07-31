package redis

import (
	"context"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

const defaultPingTimeout = 3 * time.Second

// Options 构造 Redis 客户端的参数。不依赖业务配置结构——由组合根从
// config.RedisConfig 映射进来，保持 pkg 对 internal 零依赖。
type Options struct {
	Addr         string
	Password     string
	DB           int
	PoolSize     int
	MinIdleConns int
	// PingTimeout 建连后探活超时；零值用默认 3s
	PingTimeout time.Duration
}

// Client 对 go-redis 的薄封装：只暴露本项目真正用到的命令，避免业务代码
// 直接散落依赖第三方 API，后续换驱动或加指标也只改这一处。
type Client struct {
	raw *goredis.Client
}

// New 创建客户端并 Ping 确认可达；失败时关闭连接后返回错误。
func New(opts Options) (*Client, error) {
	if opts.Addr == "" {
		return nil, fmt.Errorf("redis addr is empty")
	}
	if opts.PoolSize < 0 {
		return nil, fmt.Errorf("redis pool_size is invalid: %d", opts.PoolSize)
	}

	pingTimeout := opts.PingTimeout
	if pingTimeout <= 0 {
		pingTimeout = defaultPingTimeout
	}

	raw := goredis.NewClient(&goredis.Options{
		Addr:         opts.Addr,
		Password:     opts.Password,
		DB:           opts.DB,
		PoolSize:     opts.PoolSize,
		MinIdleConns: opts.MinIdleConns,
	})

	ctx, cancel := context.WithTimeout(context.Background(), pingTimeout)
	defer cancel()
	if err := raw.Ping(ctx).Err(); err != nil {
		_ = raw.Close()
		return nil, fmt.Errorf("redis ping failed: %w", err)
	}
	return &Client{raw: raw}, nil
}

// Set 写入带 TTL 的字符串键
func (c *Client) Set(ctx context.Context, key, value string, ttl time.Duration) error {
	if err := c.raw.Set(ctx, key, value, ttl).Err(); err != nil {
		return fmt.Errorf("redis set %q: %w", key, err)
	}
	return nil
}

// Exists 判断键是否存在
func (c *Client) Exists(ctx context.Context, key string) (bool, error) {
	n, err := c.raw.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("redis exists %q: %w", key, err)
	}
	return n > 0, nil
}

// Close 关闭底层连接池
func (c *Client) Close() error {
	if c == nil || c.raw == nil {
		return nil
	}
	return c.raw.Close()
}
