package cache

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestInMemoryCache_Basic(t *testing.T) {
	c := NewInMemoryCache(time.Minute)
	_, ok := c.Get(1)
	require.False(t, ok)

	c.Set(1, map[string]bool{"GET:/api/x": true})
	perms, ok := c.Get(1)
	require.True(t, ok)
	require.True(t, perms["GET:/api/x"])

	c.InvalidateUser(1)
	_, ok = c.Get(1)
	require.False(t, ok)
}

func TestInMemoryCache_InvalidateAll(t *testing.T) {
	c := NewInMemoryCache(time.Minute)
	c.Set(1, map[string]bool{"a": true})
	c.Set(2, map[string]bool{"b": true})
	c.InvalidateAll()
	_, ok1 := c.Get(1)
	_, ok2 := c.Get(2)
	require.False(t, ok1)
	require.False(t, ok2)
}

func TestInMemoryCache_TTL(t *testing.T) {
	c := NewInMemoryCache(20 * time.Millisecond)
	c.Set(9, map[string]bool{"x": true})
	time.Sleep(30 * time.Millisecond)
	_, ok := c.Get(9)
	require.False(t, ok)
}
