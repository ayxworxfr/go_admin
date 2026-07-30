package model

import "time"

// Role 角色模型
type Role struct {
	ID          uint64    `xorm:"pk autoincr bigint unsigned 'id'" json:"id"`
	Name        string    `xorm:"varchar(50) notnull unique 'name'" json:"name"`
	Code        string    `xorm:"varchar(50) notnull unique 'code'" json:"code"`
	Description string    `xorm:"varchar(255) 'description'" json:"description"`
	Status      int       `xorm:"int 'status'" json:"status"` // 1=启用，0=禁用
	CreateTime  time.Time `xorm:"created" json:"create_time"`
	UpdateTime  time.Time `xorm:"updated" json:"update_time"`
}

// Permission 权限模型
type Permission struct {
	ID          uint64    `xorm:"pk autoincr bigint unsigned 'id'" json:"id"`
	Name        string    `xorm:"varchar(50) notnull unique 'name'" json:"name"`
	Code        string    `xorm:"varchar(50) notnull unique 'code'" json:"code"`
	Description string    `xorm:"varchar(255) 'description'" json:"description"`
	ParentID    uint64    `xorm:"int 'parent_id'" json:"parent_id"`
	Type        int       `xorm:"int 'type'" json:"type"` // 1: 菜单, 2: 按钮, 3: 接口
	Path        string    `xorm:"varchar(255) 'path'" json:"path"`
	Method      string    `xorm:"varchar(50) 'method'" json:"method"`
	Status      int       `xorm:"int 'status'" json:"status"` // 1=启用，0=禁用
	CreateTime  time.Time `xorm:"created" json:"create_time"`
	UpdateTime  time.Time `xorm:"updated" json:"update_time"`
}

// RolePermission 角色权限关联模型
type RolePermission struct {
	ID           uint64 `xorm:"pk autoincr bigint unsigned 'id'" json:"id"`
	RoleID       uint64 `xorm:"bigint unsigned notnull index 'role_id'" json:"role_id"`
	PermissionID uint64 `xorm:"bigint unsigned notnull index 'permission_id'" json:"permission_id"`
}

// UserRole 用户角色关联模型
type UserRole struct {
	ID     uint64 `xorm:"pk autoincr bigint unsigned 'id'" json:"id"`
	UserID uint64 `xorm:"bigint unsigned notnull index 'user_id'" json:"user_id"`
	RoleID uint64 `xorm:"bigint unsigned notnull index 'role_id'" json:"role_id"`
}
