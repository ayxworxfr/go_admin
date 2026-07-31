package redis

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/require"
)

func TestClient_SetExistsClose(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)

	c, err := New(Options{Addr: mr.Addr()})
	require.NoError(t, err)
	t.Cleanup(func() { _ = c.Close() })

	ctx := context.Background()
	require.NoError(t, c.Set(ctx, "k", "1", time.Minute))
	ok, err := c.Exists(ctx, "k")
	require.NoError(t, err)
	require.True(t, ok)

	ok, err = c.Exists(ctx, "missing")
	require.NoError(t, err)
	require.False(t, ok)
}

func TestNew_EmptyAddr(t *testing.T) {
	_, err := New(Options{})
	require.Error(t, err)
}
