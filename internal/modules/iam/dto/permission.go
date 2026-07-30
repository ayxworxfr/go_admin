package dto

import (
	"time"

	"github.com/ayxworxfr/go_admin/pkg/apiparam"
)

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
	ID uint64 `query:"id" vd:"$>0"`
}

// GetPermissionListRequest 获取权限列表请求
type GetPermissionListRequest struct {
	apiparam.Page
	Name   string `query:"name" vd:"len($)>=0&&len($)<50" xorm:"name op=like"`
	Code   string `query:"code" vd:"len($)>=0&&len($)<50" xorm:"code op=startswith"`
	Type   int    `query:"type" xorm:"type op=eq"`
	Path   string `query:"path" vd:"len($)>=0&&len($)<255" xorm:"path op=like"`
	Method string `query:"method" xorm:"method op=eq"`
	Status int    `query:"status" xorm:"status op=eq"`
}

// PermissionResponse 权限视图对象
type PermissionResponse struct {
	ID          uint64    `json:"id"`
	Name        string    `json:"name"`
	Code        string    `json:"code"`
	Description string    `json:"description"`
	ParentID    uint64    `json:"parent_id"`
	Type        int       `json:"type"`
	Path        string    `json:"path"`
	Method      string    `json:"method"`
	Status      int       `json:"status"`
	CreateTime  time.Time `json:"create_time"`
	UpdateTime  time.Time `json:"update_time"`
}
