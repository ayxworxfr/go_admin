package dto

// AssignRolesRequest 分配角色请求
type AssignRolesRequest struct {
	UserID  uint64   `json:"user_id" vd:"$>0"`
	RoleIDs []uint64 `json:"role_ids"`
}

// GetUserRolesRequest 获取用户角色请求
type GetUserRolesRequest struct {
	UserID uint64 `query:"user_id" vd:"$>0"`
	Flags  int    `query:"flags"`
}

// GetUserPermissionsRequest 获取用户权限请求
type GetUserPermissionsRequest struct {
	UserID uint64 `query:"user_id" vd:"$>0"`
}

// UserRolesResponse 用户及其角色的组合视图。
//
// 这是 iam 模块唯一需要"借用"user 基本信息的地方（分配角色后要把用户名一起
// 返回给前端）。没有把它做成 user.dto.UserResponse 再加一个 Roles 字段，
// 是因为那样会让 user 模块的响应结构反过来感知 iam 的存在，方向就反了。
// iam 通过 user.UserFinder 拿到基础字段后，用自己的类型重新组装。
type UserRolesResponse struct {
	ID       uint64          `json:"id"`
	Username string          `json:"username"`
	Email    string          `json:"email"`
	Roles    []*RoleResponse `json:"roles"`
}
