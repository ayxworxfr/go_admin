package tokenstore

import (
	"context"
	"sync"
	"time"
)

// TokenStore 令牌撤销状态存储的策略接口，用于支撑登出后 token 立即失效。
// 旧版本 LoginOut 是一句 `// todo 让token失效` 的空实现——JWT 本身是无状态的，
// 要让签发过的 token 提前失效，必须有一个额外的撤销名单，这正是该接口的职责。
type TokenStore interface {
	// Revoke 撤销一个 token（记录其 jti），exp 用于清理时判断该记录何时可以丢弃
	Revoke(ctx context.Context, jti string, exp time.Time) error
	// IsRevoked 查询 jti 是否已被撤销。约定：jti 为空时由调用方自行判断是否跳过检查
	// （中间件层会对空 jti 直接放行，兼容签发时未带 jti 的旧 token）
	IsRevoked(ctx context.Context, jti string) (bool, error)
}

// InMemoryTokenStore 进程内的撤销名单实现，惰性清理已过期的撤销记录
// （查询/写入时顺手清理，不引入额外的定时器 goroutine）。
type InMemoryTokenStore struct {
	mu       sync.Mutex
	revoked  map[string]time.Time // jti -> 原 token 的过期时间
}

// NewInMemoryTokenStore 创建进程内撤销名单
func NewInMemoryTokenStore() *InMemoryTokenStore {
	return &InMemoryTokenStore{revoked: make(map[string]time.Time)}
}

// Revoke 记录一次撤销
func (s *InMemoryTokenStore) Revoke(_ context.Context, jti string, exp time.Time) error {
	if jti == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.revoked[jti] = exp
	s.cleanupLocked()
	return nil
}

// IsRevoked 查询是否已撤销；一个已经过了原始过期时间的记录本身就该被视为
// "token 已经自然失效"，不再需要撤销名单，顺手清理掉
func (s *InMemoryTokenStore) IsRevoked(_ context.Context, jti string) (bool, error) {
	if jti == "" {
		return false, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	exp, ok := s.revoked[jti]
	if !ok {
		return false, nil
	}
	if time.Now().After(exp) {
		delete(s.revoked, jti)
		return false, nil
	}
	return true, nil
}

// cleanupLocked 清除已经过期的撤销记录，调用方需持有 mu
func (s *InMemoryTokenStore) cleanupLocked() {
	now := time.Now()
	for jti, exp := range s.revoked {
		if now.After(exp) {
			delete(s.revoked, jti)
		}
	}
}
