package tokenstore

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestInMemoryTokenStore_RevokeAndCheck(t *testing.T) {
	s := NewInMemoryTokenStore()
	ctx := context.Background()

	revoked, err := s.IsRevoked(ctx, "jti-1")
	require.NoError(t, err)
	require.False(t, revoked)

	require.NoError(t, s.Revoke(ctx, "jti-1", time.Now().Add(time.Hour)))
	revoked, err = s.IsRevoked(ctx, "jti-1")
	require.NoError(t, err)
	require.True(t, revoked)
}

func TestInMemoryTokenStore_ExpiredEntryIsNotRevoked(t *testing.T) {
	s := NewInMemoryTokenStore()
	ctx := context.Background()

	require.NoError(t, s.Revoke(ctx, "jti-exp", time.Now().Add(-time.Second)))
	revoked, err := s.IsRevoked(ctx, "jti-exp")
	require.NoError(t, err)
	require.False(t, revoked)
}

func TestInMemoryTokenStore_EmptyJTI(t *testing.T) {
	s := NewInMemoryTokenStore()
	ctx := context.Background()

	require.NoError(t, s.Revoke(ctx, "", time.Now().Add(time.Hour)))
	revoked, err := s.IsRevoked(ctx, "")
	require.NoError(t, err)
	require.False(t, revoked)
}
