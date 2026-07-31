package redis

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/require"
)

func newTestClient(t *testing.T) *Client {
	t.Helper()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)

	c, err := New(Options{Addr: mr.Addr()})
	require.NoError(t, err)
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func TestLock_TryLockExclusive(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()

	a := c.NewLock("res", WithTTL(time.Minute))
	b := c.NewLock("res", WithTTL(time.Minute))

	require.NoError(t, a.TryLock(ctx))
	require.ErrorIs(t, b.TryLock(ctx), ErrLockNotObtained)

	require.NoError(t, a.Unlock(ctx))
	require.NoError(t, b.TryLock(ctx))
	require.NoError(t, b.Unlock(ctx))
}

func TestLock_UnlockOthersTokenFails(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()

	a := c.NewLock("res", WithTTL(time.Minute))
	require.NoError(t, a.TryLock(ctx))

	// 伪造另一把同 key 的锁对象（不同 token），不能解开 a 的锁
	fake := c.NewLock("res", WithTTL(time.Minute))
	require.ErrorIs(t, fake.Unlock(ctx), ErrLockNotHeld)
	require.NoError(t, a.Unlock(ctx))
}

func TestLock_Refresh(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)

	c, err := New(Options{Addr: mr.Addr()})
	require.NoError(t, err)
	t.Cleanup(func() { _ = c.Close() })

	ctx := context.Background()
	lock := c.NewLock("res", WithTTL(2*time.Second))
	require.NoError(t, lock.TryLock(ctx))

	mr.FastForward(1500 * time.Millisecond)
	require.NoError(t, lock.Refresh(ctx))
	mr.FastForward(1500 * time.Millisecond)
	// 若未续期此时应已过期；续期后仍持有
	require.True(t, mr.Exists(lock.Key()))
	require.NoError(t, lock.Unlock(ctx))
}

func TestLock_LockRespectsContext(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()

	holder := c.NewLock("res", WithTTL(time.Minute))
	require.NoError(t, holder.TryLock(ctx))

	waitCtx, cancel := context.WithTimeout(ctx, 80*time.Millisecond)
	defer cancel()
	waiter := c.NewLock("res", WithTTL(time.Minute), WithRetryInterval(10*time.Millisecond))
	err := waiter.Lock(waitCtx)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrLockNotObtained)
}

func TestClient_WithLock(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()

	var ran atomic.Bool
	err := c.WithLock(ctx, "job", func(context.Context) error {
		ran.Store(true)
		return nil
	}, WithTTL(time.Minute), WithMaxRetries(3))
	require.NoError(t, err)
	require.True(t, ran.Load())

	// 锁应已释放
	require.NoError(t, c.NewLock("job", WithTTL(time.Minute)).TryLock(ctx))
}

func TestClient_WithLockPropagatesFnError(t *testing.T) {
	c := newTestClient(t)
	want := errors.New("boom")
	err := c.WithLock(context.Background(), "job", func(context.Context) error {
		return want
	}, WithTTL(time.Minute))
	require.ErrorIs(t, err, want)
}

func TestLock_ConcurrentMutualExclusion(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()

	const workers = 8
	var (
		counter atomic.Int64
		wg      sync.WaitGroup
	)
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			err := c.WithLock(ctx, "counter", func(context.Context) error {
				// 读改写中间故意 sleep：无分布式锁时极易丢更新
				n := counter.Load()
				time.Sleep(2 * time.Millisecond)
				counter.Store(n + 1)
				return nil
			}, WithTTL(time.Second), WithRetryInterval(5*time.Millisecond))
			require.NoError(t, err)
		}()
	}
	wg.Wait()
	require.Equal(t, int64(workers), counter.Load())
}
