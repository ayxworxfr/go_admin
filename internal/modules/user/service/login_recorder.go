package service

import "context"

// LoginRecorder 是 user 模块对外暴露的登录时间回写能力（消费方视角）。
//
// 单独声明为一个只有一个方法的窄接口，不并入 UserFinder：FindByUsername/
// VerifyPassword 是"查找"语义，UpdateLastLoginTime 是"写入"语义，两者关注点
// 不同——调用方（iam.AuthService）登录成功后只需要这一个写入能力，不应该
// 因为共用同一个接口就被迫连带感知查找方法的实现细节。
//
// 依赖方向：iam -> user.LoginRecorder，user 不反向依赖 iam。
type LoginRecorder interface {
	UpdateLastLoginTime(ctx context.Context, userID uint64) error
}
