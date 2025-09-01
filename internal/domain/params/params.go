package params

// ---------------------- 用户管理模块 ----------------------

// CreateUserRequest 创建用户请求
type CreateUserRequest struct {
	Username  string   `json:"username" vd:"len($)>0&&len($)<50"`
	Password  string   `json:"password" vd:"len($)>=6&&len($)<20"`
	Email     string   `json:"email" vd:"len($)>0&&len($)<100"`
	Phone     string   `json:"phone" vd:"len($)<20"`
	AvatarURL string   `json:"avatar_url" vd:"len($)<255"`
	RoleIDs   []uint64 `json:"role_ids" vd:"len($)>0"` // 至少关联一个角色
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
	RoleIDs   *[]uint64 `json:"role_ids"` // 指针区分未设置和空数组
	Status    int       `json:"status"`
}

// DeleteUserRequest 删除用户请求
type DeleteUserRequest struct {
	IDs []uint64 `json:"ids" vd:"len($)>0"`
}

// GetUserRequest 获取用户请求
type GetUserRequest struct {
	ID    uint64 `query:"id" vd:"$>0"`
	Flags int    `query:"flags"` // 控制响应内容
}

// GetUserListRequest 获取用户列表请求
type GetUserListRequest struct {
	Page
	Username string `query:"username" vd:"len($)>=0&&len($)<50" xorm:"username op=like"`
	Email    string `query:"email" vd:"len($)>=0&&len($)<100" xorm:"email op=like"`
	Phone    string `query:"phone" vd:"len($)>=0&&len($)<20" xorm:"phone op=like"`
	RoleID   uint64 `query:"role_id" xorm:"role_id op=in"`
	Status   int    `query:"status" xorm:"status op=eq"`
	Flags    int    `query:"flags"`
}

// ---------------------- 权限管理模块 ----------------------

// CreatePermissionRequest 创建权限请求
type CreatePermissionRequest struct {
	Name        string `json:"name" vd:"len($)>0&&len($)<50"`
	Code        string `json:"code" vd:"len($)>0&&len($)<50"`
	Description string `json:"description" vd:"len($)<255"`
	ParentID    uint64 `json:"parent_id"`
	Type        int    `json:"type"` // 1:菜单,2:按钮,3:接口
	Path        string `json:"path" vd:"len($)<255"`
	Method      string `json:"method" vd:"len($)<50"`
	Status      int    `json:"status"`
}

// CreatePermissionsRequest 批量创建权限请求
type CreatePermissionsRequest struct {
	Permissions []*CreatePermissionRequest `json:"permissions"`
}

// UpdatePermissionRequest 更新权限请求
type UpdatePermissionRequest struct {
	ID          uint64  `json:"id" vd:"$>0"`
	Name        string  `json:"name" vd:"len($)>=0&&len($)<50"`
	Code        string  `json:"code" vd:"len($)>=0&&len($)<50"`
	Description string  `json:"description" vd:"len($)<255"`
	ParentID    *uint64 `json:"parent_id"` // 允许清空父级
	Type        int     `json:"type"`
	Path        string  `json:"path" vd:"len($)<255"`
	Method      string  `json:"method" vd:"len($)<50"`
	Status      int     `json:"status"`
}

// DeletePermissionRequest 删除权限请求
type DeletePermissionRequest struct {
	IDs []uint64 `json:"ids" vd:"len($)>0"`
}

// GetPermissionRequest 获取权限请求
type GetPermissionRequest struct {
	ID    uint64 `query:"id" vd:"$>0"`
	Flags int    `query:"flags"` // 控制响应内容
}

// GetPermissionListRequest 获取权限列表请求
type GetPermissionListRequest struct {
	Page
	Name   string `query:"name" vd:"len($)>=0&&len($)<50" xorm:"name op=like"`
	Code   string `query:"code" vd:"len($)>=0&&len($)<50" xorm:"code op=startswith"`
	Type   int    `query:"type" xorm:"type op=eq"`
	Path   string `query:"path" vd:"len($)>=0&&len($)<255" xorm:"path op=like"`
	Method string `query:"method" xorm:"method op=eq"`
	Status int    `query:"status" xorm:"status op=eq"`
	Flags  int    `query:"flags"`
}

// ---------------------- 角色管理模块 ----------------------

// CreateRoleRequest 创建角色请求
type CreateRoleRequest struct {
	Name          string   `json:"name" vd:"len($)>0&&len($)<50"`
	Code          string   `json:"code" vd:"len($)>0&&len($)<50"`
	Description   string   `json:"description" vd:"len($)<255"`
	Status        int      `json:"status"`
	PermissionIDs []uint64 `json:"permission_ids"`
}

// CreateRolesRequest 批量创建角色请求
type CreateRolesRequest struct {
	Roles []*CreateRoleRequest `json:"roles"`
}

// UpdateRoleRequest 更新角色请求
type UpdateRoleRequest struct {
	ID            uint64    `json:"id" vd:"$>0"`
	Name          string    `json:"name" vd:"len($)>=0&&len($)<50"`
	Code          string    `json:"code" vd:"len($)>=0&&len($)<50"`
	Description   string    `json:"description" vd:"len($)<255"`
	Status        int       `json:"status"`
	PermissionIDs *[]uint64 `json:"permission_ids"` // 指针区分未设置和空数组
}

// DeleteRoleRequest 删除角色请求
type DeleteRoleRequest struct {
	IDs []uint64 `json:"ids" vd:"len($)>0"`
}

// GetRoleRequest 获取角色请求
type GetRoleRequest struct {
	ID    uint64 `query:"id" vd:"$>0"`
	Flags int    `query:"flags"`
}

// GetRoleListRequest 获取角色列表请求
type GetRoleListRequest struct {
	Page
	Name   string `query:"name" vd:"len($)>=0&&len($)<50" xorm:"name op=like"`
	Code   string `query:"code" vd:"len($)>=0&&len($)<50" xorm:"code op=startswith"`
	Status int    `query:"status" xorm:"status op=eq"`
	Flags  int    `query:"flags"`
}

// GetRolePermissionsRequest 获取角色权限请求
type GetRolePermissionsRequest struct {
	RoleID uint64 `query:"role_id" vd:"$>0"`
}

// AssignRolePermissionsRequest 分配角色权限请求
type AssignRolePermissionsRequest struct {
	RoleID        uint64   `json:"role_id" vd:"$>0"`
	PermissionIDs []uint64 `json:"permission_ids"`
}

// ---------------------- 系统设置管理模块 ----------------------

// CreateSystemSettingRequest 创建系统配置请求
type CreateSystemSettingRequest struct {
	Category    string `json:"category" vd:"len($)>0&&len($)<50"`
	Key         string `json:"key" vd:"len($)>0&&len($)<50"`
	Value       string `json:"value"`
	Type        uint8  `json:"type" vd:"$>0&&$<=4"`
	Description string `json:"description"`
}

// UpdateSystemSettingRequest 更新系统配置请求
type UpdateSystemSettingRequest struct {
	ID          uint64 `json:"id" vd:"$>0"`
	Category    string `json:"category" vd:"len($)>=0&&len($)<50"`
	Key         string `json:"key" vd:"len($)>=0&&len($)<50"`
	Value       string `json:"value"`
	Type        uint8  `json:"type" vd:"$>=0&&$<=4"`
	Description string `json:"description"`
}

// DeleteSystemSettingRequest 删除系统配置请求
type DeleteSystemSettingRequest struct {
	IDs []uint64 `json:"ids" vd:"len($)>0"`
}

// GetSystemSettingRequest 获取系统配置请求
type GetSystemSettingRequest struct {
	ID    uint64 `query:"id" vd:"$>0"`
	Flags int    `query:"flags"`
}

// GetSystemSettingListRequest 获取系统配置列表请求
type GetSystemSettingListRequest struct {
	Page
	Category string `query:"category" vd:"len($)>=0&&len($)<50" xorm:"category op=eq"`
	Key      string `query:"key" vd:"len($)>=0&&len($)<50" xorm:"key op=like"`
	Type     uint8  `query:"type" xorm:"type op=eq"`
	Flags    int    `query:"flags"`
}

// GetSystemSettingByCategoryRequest 根据分类获取系统配置请求
type GetSystemSettingByCategoryRequest struct {
	Category string `query:"category" vd:"len($)>0&&len($)<50"`
}
