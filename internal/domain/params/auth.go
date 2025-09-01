package params

type LoginRequest struct {
	Username string `json:"username" vd:"len($)>0"`
	Password string `json:"password" vd:"len($)>0"`
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" vd:"len($)>0"`
}

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

// GetPermissionListRequest 获取权限列表请求
type GetDataPermissionListRequest struct {
	Page
	Name  string `query:"name" vd:"len($)>=0&&len($)<50" xorm:"name op=like"`
	Code  string `query:"code" vd:"len($)>=0&&len($)<50" xorm:"code op=startswith"`
	Level int    `query:"level" vd:"$>=0"`
}

// GetUserPermissionsRequest 获取用户权限请求
type GetUserPermissionsRequest struct {
	UserID uint64 `query:"user_id" vd:"$>0"`
}

// SetRoleDataPermissionRequest 设置角色数据权限请求
type SetRoleDataPermissionRequest struct {
	RoleID           uint64 `json:"role_id" vd:"$>0"`
	ResourceType     string `json:"resource_type" vd:"len($)>0"`
	DataPermissionID uint64 `json:"data_permission_id" vd:"$>0"`
}

// GetRoleDataPermissionRequest 获取角色数据权限请求
type GetRoleDataPermissionRequest struct {
	RoleID       uint64 `query:"role_id" vd:"$>0"`
	ResourceType string `query:"resource_type"`
}

// LockResourceRequest 锁定资源请求
type LockResourceRequest struct {
	ResourceType string `json:"resource_type" vd:"len($)>0"`
	ResourceID   uint64 `json:"resource_id" vd:"$>0"`
	LockReason   string `json:"lock_reason"`
	ExpireHours  int    `json:"expire_hours"` // 锁定小时数，0表示永久
}

// UnlockResourceRequest 解锁资源请求
type UnlockResourceRequest struct {
	ResourceType string `json:"resource_type" vd:"len($)>0"`
	ResourceID   uint64 `json:"resource_id" vd:"$>0"`
}

// CheckResourcePermissionRequest 检查资源权限请求
type CheckResourcePermissionRequest struct {
	UserID       uint64 `json:"user_id" vd:"$>0"`
	ResourceType string `json:"resource_type" vd:"len($)>0"`
	ResourceID   uint64 `json:"resource_id"`
}

// GetMyLockedResourcesRequest 获取我锁定的资源请求
type GetMyLockedResourcesRequest struct {
	Page
	UserID       uint64 `query:"user_id"`
	ResourceType string `query:"resource_type"`
}
