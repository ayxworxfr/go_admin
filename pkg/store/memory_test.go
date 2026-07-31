package store

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestMemory_SetGetDelete(t *testing.T) {
	s := NewMemory[string, int](time.Minute)
	s.Set("a", 1)
	v, ok := s.Get("a")
	require.True(t, ok)
	require.Equal(t, 1, v)

	s.Delete("a")
	_, ok = s.Get("a")
	require.False(t, ok)
}

func TestMemory_DefaultTTLExpiry(t *testing.T) {
	s := NewMemory[string, string](20 * time.Millisecond)
	s.Set("k", "v")
	_, ok := s.Get("k")
	require.True(t, ok)

	time.Sleep(30 * time.Millisecond)
	_, ok = s.Get("k")
	require.False(t, ok)
}

func TestMemory_SetWithTTLNonPositiveDeletes(t *testing.T) {
	s := NewMemory[string, struct{}](0)
	s.SetWithTTL("k", struct{}{}, time.Hour)
	require.True(t, s.Has("k"))

	s.SetWithTTL("k", struct{}{}, 0)
	require.False(t, s.Has("k"))
}

func TestMemory_SetUntil(t *testing.T) {
	s := NewMemory[string, bool](0)
	s.SetUntil("alive", true, time.Now().Add(time.Hour))
	require.True(t, s.Has("alive"))

	s.SetUntil("dead", true, time.Now().Add(-time.Second))
	require.False(t, s.Has("dead"))
}

func TestMemory_SetForeverWhenDefaultTTLZero(t *testing.T) {
	s := NewMemory[int, string](0)
	s.Set(1, "x")
	time.Sleep(15 * time.Millisecond)
	v, ok := s.Get(1)
	require.True(t, ok)
	require.Equal(t, "x", v)
}

func TestMemory_Clear(t *testing.T) {
	s := NewMemory[string, int](time.Minute)
	s.Set("a", 1)
	s.Set("b", 2)
	s.Clear()
	require.Equal(t, 0, s.Len())
}

func TestMemory_CleanupOnWrite(t *testing.T) {
	s := NewMemory[string, int](0)
	s.SetWithTTL("old", 1, 5*time.Millisecond)
	time.Sleep(15 * time.Millisecond)
	s.SetWithTTL("new", 2, time.Minute)
	require.False(t, s.Has("old"))
	require.True(t, s.Has("new"))
}
