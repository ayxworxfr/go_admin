package vo

import (
	"time"
)

// Role 角色视图对象
type Role struct {
	ID          uint64        `json:"id"`
	Name        string        `json:"name"`
	Code        string        `json:"code"`
	Description string        `json:"description"`
	Status      int           `json:"status"`
	CreateTime  time.Time     `json:"create_time"`
	UpdateTime  time.Time     `json:"update_time"`
	Permissions []*Permission `json:"permissions"`
}

// Permission 权限视图对象
type Permission struct {
	ID          uint64        `json:"id"`
	Name        string        `json:"name"`
	Code        string        `json:"code"`
	Description string        `json:"description"`
	ParentID    uint64        `json:"parent_id"`
	Type        int           `json:"type"`
	Path        string        `json:"path"`
	Method      string        `json:"method"`
	Status      int           `json:"status"`
	CreateTime  time.Time     `json:"create_time"`
	UpdateTime  time.Time     `json:"update_time"`
	Children    []*Permission `json:"children,omitempty"`
}

// DataPermission 数据权限视图对象
type DataPermission struct {
	ID          uint64    `json:"id"`
	Code        string    `json:"code"`
	Name        string    `json:"name"`
	Level       int       `json:"level"`
	Description string    `json:"description"`
	CreateTime  time.Time `json:"create_time"`
	UpdateTime  time.Time `json:"update_time"`
}

// SystemSetting 系统配置视图对象
type SystemSetting struct {
	ID          uint64    `json:"id"`
	Category    string    `json:"category"`
	Key         string    `json:"key"`
	Value       string    `json:"value"`
	Type        uint8     `json:"type"`
	TypeDisplay string    `json:"type_display"`
	Description string    `json:"description"`
	CreateBy    *User     `json:"create_by,omitempty"`
	CreateTime  time.Time `json:"create_time"`
	UpdateTime  time.Time `json:"update_time"`
}

// UserPermissionSummary 用户权限汇总视图
type UserPermissionSummary struct {
	UserID              uint64                     `json:"user_id"`
	Username            string                     `json:"username"`
	Roles               []*Role                    `json:"roles"`
	ResourcePermissions map[string]*DataPermission `json:"resource_permissions"` // 资源类型 -> 数据权限
}
