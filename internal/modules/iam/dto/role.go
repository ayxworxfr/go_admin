package dto

import (
	"time"

	"github.com/ayxworxfr/go_admin/pkg/apiparam"
)

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
	ID uint64 `query:"id" vd:"$>0"`
}

// GetRoleListRequest 获取角色列表请求
type GetRoleListRequest struct {
	apiparam.Page
	Name   string `query:"name" vd:"len($)>=0&&len($)<50" xorm:"name op=like"`
	Code   string `query:"code" vd:"len($)>=0&&len($)<50" xorm:"code op=startswith"`
	Status int    `query:"status" xorm:"status op=eq"`
	Flags  int    `query:"flags"` // 是否附带角色的权限列表，见 dto.INCLUDE_PERMISSION
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

// RoleResponse 角色视图对象
type RoleResponse struct {
	ID          uint64                 `json:"id"`
	Name        string                 `json:"name"`
	Code        string                 `json:"code"`
	Description string                 `json:"description"`
	Status      int                    `json:"status"`
	CreateTime  time.Time              `json:"create_time"`
	UpdateTime  time.Time              `json:"update_time"`
	Permissions []*PermissionResponse  `json:"permissions,omitempty"`
}
