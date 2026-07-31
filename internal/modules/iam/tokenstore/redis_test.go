package tokenstore

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	pkgredis "github.com/ayxworxfr/go_admin/pkg/redis"
	"github.com/stretchr/testify/require"
)

func newTestRedisStore(t *testing.T) (*RedisTokenStore, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)

	client, err := pkgredis.New(pkgredis.Options{Addr: mr.Addr()})
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })
	return NewRedisTokenStore(client, "test:revoked:"), mr
}

func TestRedisTokenStore_RevokeAndCheck(t *testing.T) {
	s, _ := newTestRedisStore(t)
	ctx := context.Background()

	revoked, err := s.IsRevoked(ctx, "jti-1")
	require.NoError(t, err)
	require.False(t, revoked)

	require.NoError(t, s.Revoke(ctx, "jti-1", time.Now().Add(time.Hour)))
	revoked, err = s.IsRevoked(ctx, "jti-1")
	require.NoError(t, err)
	require.True(t, revoked)
}

func TestRedisTokenStore_SkipExpired(t *testing.T) {
	s, mr := newTestRedisStore(t)
	ctx := context.Background()

	require.NoError(t, s.Revoke(ctx, "jti-exp", time.Now().Add(-time.Second)))
	require.Empty(t, mr.Keys())
	revoked, err := s.IsRevoked(ctx, "jti-exp")
	require.NoError(t, err)
	require.False(t, revoked)
}

func TestRedisTokenStore_TTLAligned(t *testing.T) {
	s, mr := newTestRedisStore(t)
	ctx := context.Background()

	require.NoError(t, s.Revoke(ctx, "jti-ttl", time.Now().Add(2*time.Second)))
	require.True(t, mr.Exists("test:revoked:jti-ttl"))

	mr.FastForward(3 * time.Second)
	revoked, err := s.IsRevoked(ctx, "jti-ttl")
	require.NoError(t, err)
	require.False(t, revoked)
}
