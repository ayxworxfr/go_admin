package cache

import (
	"sync"
	"time"
)

// PermissionCache 用户权限路径缓存的策略接口。旧版本把 `permissionCache
// map[uint64]map[string]bool` 直接裸露在 PermissionService 里，`cacheExpiration`
// 字段声明了却从未被读取——缓存实际永不过期。接口化之后：
//   - InMemoryCache 是本次的默认实现，真正实现了 TTL；
//   - 以后要接 Redis 支持多实例一致性，只需新增一个实现，PermissionChecker 不用改。
type PermissionCache interface {
	// Get 返回用户的权限路径映射；ok=false 表示未命中（不存在或已过期）
	Get(userID uint64) (map[string]bool, bool)
	// Set 写入用户的权限路径映射，从写入时刻起计时 TTL
	Set(userID uint64, perms map[string]bool)
	// InvalidateUser 清除单个用户的缓存（分配角色/权限变更后调用）
	InvalidateUser(userID uint64)
	// InvalidateAll 清除所有用户的缓存（角色-权限关系整体变更后调用）
	InvalidateAll()
}

// entry 缓存条目，记录写入时间以支持 TTL 判断
type entry struct {
	data      map[string]bool
	expiresAt time.Time
}

// InMemoryCache 进程内权限缓存实现，带真实的 TTL 过期判断。
// 不解决多实例一致性问题（见重构文档 Non-Goals）——每个实例各自维护一份，
// 靠 TTL 兜底最终一致，而不是靠 ClearAll 广播。
type InMemoryCache struct {
	mu  sync.RWMutex
	ttl time.Duration
	m   map[uint64]entry
}

// NewInMemoryCache 创建带 TTL 的进程内缓存
func NewInMemoryCache(ttl time.Duration) *InMemoryCache {
	return &InMemoryCache{
		ttl: ttl,
		m:   make(map[uint64]entry),
	}
}

// Get 命中且未过期才返回 true；已过期的条目会被顺手清除，避免内存堆积
func (c *InMemoryCache) Get(userID uint64) (map[string]bool, bool) {
	c.mu.RLock()
	e, ok := c.m[userID]
	c.mu.RUnlock()
	if !ok {
		return nil, false
	}
	if time.Now().After(e.expiresAt) {
		c.InvalidateUser(userID)
		return nil, false
	}
	return e.data, true
}

// Set 写入缓存并重置过期时间
func (c *InMemoryCache) Set(userID uint64, perms map[string]bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.m[userID] = entry{data: perms, expiresAt: time.Now().Add(c.ttl)}
}

// InvalidateUser 清除单个用户的缓存
func (c *InMemoryCache) InvalidateUser(userID uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.m, userID)
}

// InvalidateAll 清除所有用户的缓存
func (c *InMemoryCache) InvalidateAll() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.m = make(map[uint64]entry)
}
