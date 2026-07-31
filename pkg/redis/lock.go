package redis

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"
)

const (
	defaultLockTTL           = 10 * time.Second
	defaultLockRetryInterval = 50 * time.Millisecond
	defaultLockKeyPrefix     = "go_admin:lock:"
)

var (
	// ErrLockNotObtained 在 TTL 内未能拿到锁（TryLock 立即失败，或 Lock 重试耗尽 / ctx 取消）
	ErrLockNotObtained = errors.New("redis: lock not obtained")
	// ErrLockNotHeld 解锁或续期时发现当前实例并不持有该锁（已过期被他人抢走，或从未加锁）
	ErrLockNotHeld = errors.New("redis: lock not held")
)

// 仅在 value 仍是自己的 token 时删除，避免误删别人的锁
var unlockScript = goredis.NewScript(`
if redis.call("get", KEYS[1]) == ARGV[1] then
  return redis.call("del", KEYS[1])
end
return 0
`)

// 仅持有者可续期
var refreshScript = goredis.NewScript(`
if redis.call("get", KEYS[1]) == ARGV[1] then
  return redis.call("pexpire", KEYS[1], ARGV[2])
end
return 0
`)

type lockOptions struct {
	ttl           time.Duration
	retryInterval time.Duration
	maxRetries    int // Lock 时使用；0 表示仅受 ctx 约束
	keyPrefix     string
}

// LockOption 配置分布式锁行为
type LockOption func(*lockOptions)

// WithTTL 锁自动过期时间，防止持有者崩溃导致死锁；必须 > 0
func WithTTL(ttl time.Duration) LockOption {
	return func(o *lockOptions) { o.ttl = ttl }
}

// WithRetryInterval Lock 自旋重试间隔
func WithRetryInterval(d time.Duration) LockOption {
	return func(o *lockOptions) { o.retryInterval = d }
}

// WithMaxRetries Lock 最大重试次数（不含首次尝试）。0 表示一直重试直到 ctx 结束
func WithMaxRetries(n int) LockOption {
	return func(o *lockOptions) { o.maxRetries = n }
}

// WithKeyPrefix 覆盖默认键前缀 go_admin:lock:
func WithKeyPrefix(prefix string) LockOption {
	return func(o *lockOptions) { o.keyPrefix = prefix }
}

// Lock 单 Redis 节点上的互斥锁（非 Redlock）。
// 适用本项目单 Redis / 主从（锁写在主库）场景；多主集群请另选 Redlock 实现。
type Lock struct {
	client *Client
	key    string
	token  string
	opts   lockOptions
}

// NewLock 创建一个尚未持有的锁对象。key 为业务资源名，实际 Redis 键 = prefix + key。
func (c *Client) NewLock(key string, opts ...LockOption) *Lock {
	o := lockOptions{
		ttl:           defaultLockTTL,
		retryInterval: defaultLockRetryInterval,
		keyPrefix:     defaultLockKeyPrefix,
	}
	for _, fn := range opts {
		fn(&o)
	}
	if o.ttl <= 0 {
		o.ttl = defaultLockTTL
	}
	if o.retryInterval <= 0 {
		o.retryInterval = defaultLockRetryInterval
	}
	if o.keyPrefix == "" {
		o.keyPrefix = defaultLockKeyPrefix
	}
	return &Lock{
		client: c,
		key:    o.keyPrefix + key,
		token:  uuid.NewString(),
		opts:   o,
	}
}

// Key 返回完整 Redis 键
func (l *Lock) Key() string { return l.key }

// TryLock 尝试加锁一次，失败立即返回 ErrLockNotObtained
func (l *Lock) TryLock(ctx context.Context) error {
	// SetNX 已弃用；SET key value PX ttl NX 语义不变，条件不满足时返回 redis.Nil
	err := l.client.raw.SetArgs(ctx, l.key, l.token, goredis.SetArgs{
		Mode: "NX",
		TTL:  l.opts.ttl,
	}).Err()
	if errors.Is(err, goredis.Nil) {
		return ErrLockNotObtained
	}
	if err != nil {
		return fmt.Errorf("redis try lock %q: %w", l.key, err)
	}
	return nil
}

// Lock 阻塞重试直到拿到锁、超过 MaxRetries，或 ctx 取消
func (l *Lock) Lock(ctx context.Context) error {
	attempts := 0
	for {
		err := l.TryLock(ctx)
		if err == nil {
			return nil
		}
		if !errors.Is(err, ErrLockNotObtained) {
			return err
		}

		attempts++
		if l.opts.maxRetries > 0 && attempts > l.opts.maxRetries {
			return ErrLockNotObtained
		}

		timer := time.NewTimer(l.opts.retryInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return fmt.Errorf("%w: %v", ErrLockNotObtained, ctx.Err())
		case <-timer.C:
		}
	}
}

// Unlock 安全释放：仅删除自己持有的锁
func (l *Lock) Unlock(ctx context.Context) error {
	n, err := unlockScript.Run(ctx, l.client.raw, []string{l.key}, l.token).Int()
	if err != nil {
		return fmt.Errorf("redis unlock %q: %w", l.key, err)
	}
	if n == 0 {
		return ErrLockNotHeld
	}
	return nil
}

// Refresh 在仍持有锁时把 TTL 重置为创建时配置的时长，适合长任务心跳续期
func (l *Lock) Refresh(ctx context.Context) error {
	ms := l.opts.ttl.Milliseconds()
	if ms <= 0 {
		return fmt.Errorf("redis refresh %q: invalid ttl", l.key)
	}
	n, err := refreshScript.Run(ctx, l.client.raw, []string{l.key}, l.token, ms).Int()
	if err != nil {
		return fmt.Errorf("redis refresh %q: %w", l.key, err)
	}
	if n == 0 {
		return ErrLockNotHeld
	}
	return nil
}

// WithLock 获取锁 → 执行 fn → 释放锁。优先返回 fn 的错误；fn 成功时返回解锁错误。
func (c *Client) WithLock(ctx context.Context, key string, fn func(context.Context) error, opts ...LockOption) error {
	lock := c.NewLock(key, opts...)
	if err := lock.Lock(ctx); err != nil {
		return err
	}

	fnErr := fn(ctx)
	unlockErr := lock.Unlock(context.Background())
	if fnErr != nil {
		return fnErr
	}
	return unlockErr
}
