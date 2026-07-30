package service

import (
	"context"

	"github.com/ayxworxfr/go_admin/internal/modules/user/model"
)

// UserFinder 是 user 模块对外暴露的最小接口（消费方视角）。
// iam 模块登录鉴权时只需要"按用户名/ID 查用户 + 校验密码"这三件事，
// 不应该也不需要知道 user 模块内部用什么 ORM、密码用什么算法哈希——
// 这正是依赖方按需定义窄接口、被依赖方隐式实现的组合优先做法。
//
// 依赖方向：iam -> user.UserFinder，user 不反向依赖 iam。
type UserFinder interface {
	FindByID(ctx context.Context, id uint64) (*model.User, error)
	FindByUsername(ctx context.Context, username string) (*model.User, error)
	VerifyPassword(user *model.User, plainPassword string) bool
}
