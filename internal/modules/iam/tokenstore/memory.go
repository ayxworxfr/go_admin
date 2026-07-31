package tokenstore

import (
	"context"
	"time"

	"github.com/ayxworxfr/go_admin/pkg/store"
)

// InMemoryTokenStore 进程内的撤销名单实现，惰性清理已过期的撤销记录
// （查询/写入时顺手清理，不引入额外的定时器 goroutine）。
// 不支持多实例：实例 A 登出后，实例 B 看不到撤销记录。
//
// 底层复用 pkg/store.Memory：键为 jti，值仅为占位；存活到原 token 过期时刻。
type InMemoryTokenStore struct {
	store *store.Memory[string, struct{}]
}

// NewInMemoryTokenStore 创建进程内撤销名单
func NewInMemoryTokenStore() *InMemoryTokenStore {
	return &InMemoryTokenStore{store: store.NewMemory[string, struct{}](0)}
}

// Revoke 记录一次撤销
func (s *InMemoryTokenStore) Revoke(_ context.Context, jti string, exp time.Time) error {
	if jti == "" {
		return nil
	}
	// SetUntil：已过期则不写入；未过期则以绝对时间作为 TTL 终点
	s.store.SetUntil(jti, struct{}{}, exp)
	return nil
}

// IsRevoked 查询是否已撤销；一个已经过了原始过期时间的记录本身就该被视为
// "token 已经自然失效"，不再需要撤销名单，顺手清理掉
func (s *InMemoryTokenStore) IsRevoked(_ context.Context, jti string) (bool, error) {
	if jti == "" {
		return false, nil
	}
	return s.store.Has(jti), nil
}
