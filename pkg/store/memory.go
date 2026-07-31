package store

import (
	"sync"
	"time"
)

// entry 缓存条目。expiresAt 为零值表示永不过期。
type entry[V any] struct {
	value     V
	expiresAt time.Time
}

// Memory 进程内泛型存储，支持可选 TTL 与惰性过期清理。
// 与 pkg/redis 对称：业务侧（权限缓存、token 撤销名单等）只组合本类型，
// 不再各自维护 map + Mutex + 过期逻辑。
//
// 并发安全；过期条目在 Get / Set* 时顺手清理，不启定时器 goroutine。
type Memory[K comparable, V any] struct {
	mu         sync.RWMutex
	defaultTTL time.Duration // Set 使用的默认 TTL；<=0 表示 Set 永不过期
	m          map[K]entry[V]
}

// NewMemory 创建内存存储。defaultTTL 作用于 Set；按条目覆盖 TTL 请用 SetWithTTL / SetUntil。
func NewMemory[K comparable, V any](defaultTTL time.Duration) *Memory[K, V] {
	return &Memory[K, V]{
		defaultTTL: defaultTTL,
		m:          make(map[K]entry[V]),
	}
}

// Get 读取未过期的值；不存在或已过期返回零值与 false（过期键会被删除）
func (s *Memory[K, V]) Get(key K) (V, bool) {
	s.mu.RLock()
	e, ok := s.m[key]
	if !ok {
		s.mu.RUnlock()
		var zero V
		return zero, false
	}
	if isExpired(e.expiresAt) {
		s.mu.RUnlock()
		s.Delete(key)
		var zero V
		return zero, false
	}
	s.mu.RUnlock()
	return e.value, true
}

// Has 判断键是否存在且未过期
func (s *Memory[K, V]) Has(key K) bool {
	_, ok := s.Get(key)
	return ok
}

// Set 写入值，TTL 使用构造时的 defaultTTL；defaultTTL<=0 则永不过期
func (s *Memory[K, V]) Set(key K, value V) {
	if s.defaultTTL > 0 {
		s.SetWithTTL(key, value, s.defaultTTL)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[key] = entry[V]{value: value}
	s.cleanupLocked()
}

// SetWithTTL 写入值并指定存活时长。ttl<=0 时不写入（若键已存在则删除），避免无意义条目。
func (s *Memory[K, V]) SetWithTTL(key K, value V, ttl time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ttl <= 0 {
		delete(s.m, key)
		s.cleanupLocked()
		return
	}
	s.m[key] = entry[V]{value: value, expiresAt: time.Now().Add(ttl)}
	s.cleanupLocked()
}

// SetUntil 写入值并指定绝对过期时间。已过期则等同删除。
func (s *Memory[K, V]) SetUntil(key K, value V, expiresAt time.Time) {
	s.SetWithTTL(key, value, time.Until(expiresAt))
}

// Delete 删除指定键
func (s *Memory[K, V]) Delete(key K) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, key)
}

// Clear 清空全部键
func (s *Memory[K, V]) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m = make(map[K]entry[V])
}

// Len 返回当前条目数（含可能已过期但尚未被访问清理的键，近似值）
func (s *Memory[K, V]) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.m)
}

func (s *Memory[K, V]) cleanupLocked() {
	now := time.Now()
	for k, e := range s.m {
		if !e.expiresAt.IsZero() && now.After(e.expiresAt) {
			delete(s.m, k)
		}
	}
}

func isExpired(expiresAt time.Time) bool {
	return !expiresAt.IsZero() && time.Now().After(expiresAt)
}
