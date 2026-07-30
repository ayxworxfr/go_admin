package dto

import (
	"time"

	"github.com/ayxworxfr/go_admin/pkg/apiparam"
)

// CreateUserRequest 创建用户请求
type CreateUserRequest struct {
	Username  string   `json:"username" vd:"len($)>0&&len($)<50"`
	Password  string   `json:"password" vd:"len($)>=6&&len($)<20"`
	Email     string   `json:"email" vd:"len($)>0&&len($)<100"`
	Phone     string   `json:"phone" vd:"len($)<20"`
	AvatarURL string   `json:"avatar_url" vd:"len($)<255"`
	RoleIDs   []uint64 `json:"role_ids" vd:"len($)>0"` // 至少关联一个角色，创建后由 handler 编排调用 iam 分配
	Status    int      `json:"status"`
}

// UpdateUserRequest 更新用户请求
type UpdateUserRequest struct {
	ID        uint64    `json:"id" vd:"$>0"`
	Username  string    `json:"username" vd:"len($)>=0&&len($)<50"`
	Password  string    `json:"password" vd:"len($)>=0||(len($)>=6&&len($)<20)"` // 允许不修改密码
	Email     string    `json:"email" vd:"len($)>=0&&len($)<100"`
	Phone     string    `json:"phone" vd:"len($)<20"`
	AvatarURL string    `json:"avatar_url" vd:"len($)<255"`
	RoleIDs   *[]uint64 `json:"role_ids"` // 指针区分"未设置"和"清空角色"
	Status    int       `json:"status"`
}

// DeleteUserRequest 删除用户请求
type DeleteUserRequest struct {
	IDs []uint64 `json:"ids" vd:"len($)>0"`
}

// GetUserRequest 获取用户请求
type GetUserRequest struct {
	ID uint64 `query:"id" vd:"$>0"`
}

// GetUserListRequest 获取用户列表请求
type GetUserListRequest struct {
	apiparam.Page
	Username string `query:"username" vd:"len($)>=0&&len($)<50" xorm:"username op=like"`
	Email    string `query:"email" vd:"len($)>=0&&len($)<100" xorm:"email op=like"`
	Phone    string `query:"phone" vd:"len($)>=0&&len($)<20" xorm:"phone op=like"`
	Status   int    `query:"status" xorm:"status op=eq"`
}

// UserResponse 用户视图对象
type UserResponse struct {
	ID            uint64    `json:"id"`
	Username      string    `json:"username"`
	Email         string    `json:"email"`
	Phone         string    `json:"phone"`
	AvatarURL     string    `json:"avatar_url"`
	Status        int       `json:"status"`
	CreateTime    time.Time `json:"create_time"`
	UpdateTime    time.Time `json:"update_time"`
	LastLoginTime time.Time `json:"last_login_time"`
}

// UserRoutes 登录用户可访问的前端路由/菜单权限
type UserRoutes struct {
	Username string   `json:"username"`
	Role     string   `json:"role"`
	Routes   []string `json:"routes"`
}
