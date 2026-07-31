package tokenstore

import (
	"context"
	"strings"
	"time"

	pkgredis "github.com/ayxworxfr/go_admin/pkg/redis"
)

const defaultKeyPrefix = "go_admin:jwt:revoked:"

// RedisTokenStore 基于 Redis 的撤销名单，多实例共享同一份 jti 黑名单。
// 键值用 token 剩余 TTL 自动过期，无需定时扫库——与 JWT 自然失效对齐。
type RedisTokenStore struct {
	client    *pkgredis.Client
	keyPrefix string
}

// NewRedisTokenStore 创建 Redis 撤销名单。client 必须非 nil；prefix 为空时用默认前缀。
func NewRedisTokenStore(client *pkgredis.Client, keyPrefix string) *RedisTokenStore {
	if keyPrefix == "" {
		keyPrefix = defaultKeyPrefix
	}
	if !strings.HasSuffix(keyPrefix, ":") {
		keyPrefix += ":"
	}
	return &RedisTokenStore{client: client, keyPrefix: keyPrefix}
}

func (s *RedisTokenStore) key(jti string) string {
	return s.keyPrefix + jti
}

// Revoke 将 jti 写入 Redis，TTL = 直到原 token 过期的剩余时间。
// 已过期或空 jti 直接跳过，避免无意义写入。
func (s *RedisTokenStore) Revoke(ctx context.Context, jti string, exp time.Time) error {
	if jti == "" {
		return nil
	}
	ttl := time.Until(exp)
	if ttl <= 0 {
		return nil
	}
	return s.client.Set(ctx, s.key(jti), "1", ttl)
}

// IsRevoked 查询 Redis 中是否存在该 jti 的撤销键
func (s *RedisTokenStore) IsRevoked(ctx context.Context, jti string) (bool, error) {
	if jti == "" {
		return false, nil
	}
	return s.client.Exists(ctx, s.key(jti))
}
