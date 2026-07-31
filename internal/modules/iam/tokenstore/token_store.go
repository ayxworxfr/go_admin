package tokenstore

import (
	"context"
	"time"
)

// TokenStore 令牌撤销状态存储的策略接口，用于支撑登出后 token 立即失效。
// 旧版本 LoginOut 是一句 `// todo 让token失效` 的空实现——JWT 本身是无状态的，
// 要让签发过的 token 提前失效，必须有一个额外的撤销名单，这正是该接口的职责。
//
// 实现切换（策略模式）：
//   - InMemoryTokenStore：单机进程内 map，默认本地开发使用
//   - RedisTokenStore：多实例共享撤销名单，Docker/多副本部署使用
//
// 调用方（AuthService / JWT 中间件）只依赖本接口，不关心底层实现。
type TokenStore interface {
	// Revoke 撤销一个 token（记录其 jti），exp 用于清理时判断该记录何时可以丢弃
	Revoke(ctx context.Context, jti string, exp time.Time) error
	// IsRevoked 查询 jti 是否已被撤销。约定：jti 为空时由调用方自行判断是否跳过检查
	// （中间件层会对空 jti 直接放行，兼容签发时未带 jti 的旧 token）
	IsRevoked(ctx context.Context, jti string) (bool, error)
}
