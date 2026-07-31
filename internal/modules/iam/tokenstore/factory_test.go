package tokenstore

import (
	"testing"

	"github.com/alicebob/miniredis/v2"
	pkgredis "github.com/ayxworxfr/go_admin/pkg/redis"
	"github.com/stretchr/testify/require"
)

func TestNew_Memory(t *testing.T) {
	store, err := New(Options{Driver: DriverMemory})
	require.NoError(t, err)
	_, ok := store.(*InMemoryTokenStore)
	require.True(t, ok)
}

func TestNew_DefaultMemory(t *testing.T) {
	store, err := New(Options{})
	require.NoError(t, err)
	_, ok := store.(*InMemoryTokenStore)
	require.True(t, ok)
}

func TestNew_RedisRequiresClient(t *testing.T) {
	_, err := New(Options{Driver: DriverRedis})
	require.Error(t, err)
}

func TestNew_Redis(t *testing.T) {
	client := newTestRedisClient(t)
	store, err := New(Options{
		Driver:    DriverRedis,
		KeyPrefix: "x:",
		Redis:     client,
	})
	require.NoError(t, err)
	_, ok := store.(*RedisTokenStore)
	require.True(t, ok)
}

func TestNew_UnknownDriver(t *testing.T) {
	_, err := New(Options{Driver: "mongo"})
	require.Error(t, err)
}

func newTestRedisClient(t *testing.T) *pkgredis.Client {
	t.Helper()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)

	client, err := pkgredis.New(pkgredis.Options{Addr: mr.Addr()})
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })
	return client
}
