package models

import (
	"time"
)

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

// SystemSetting 系统配置模型
type SystemSetting struct {
	ID          uint64    `xorm:"pk autoincr bigint unsigned 'id'" json:"id"`
	Category    string    `xorm:"varchar(50) notnull 'category'" json:"category"`
	Key         string    `xorm:"varchar(50) notnull unique 'key'" json:"key"`
	Value       string    `xorm:"text 'value'" json:"value"`
	Type        uint8     `xorm:"tinyint unsigned notnull 'type'" json:"type"`
	Description string    `xorm:"text 'description'" json:"description"`
	CreateBy    uint64    `xorm:"bigint unsigned notnull 'create_by'" json:"create_by"`
	CreateTime  time.Time `xorm:"created 'create_time'" json:"create_time"`
	UpdateTime  time.Time `xorm:"updated 'update_time'" json:"update_time"`
}

// DataPermissionCode 数据权限编码常量
const (
	DataPermissionOwnData  = "OWN"  // 本人数据
	DataPermissionDeptData = "DEPT" // 本部门数据
	DataPermissionAllData  = "ALL"  // 全公司数据
)

// DataPermissionLevel 数据权限级别常量
const (
	DataPermissionLevelOwn  = 1 // 本人级别
	DataPermissionLevelDept = 2 // 部门级别
	DataPermissionLevelAll  = 3 // 全公司级别
)

// DataPermission 数据权限模型
type DataPermission struct {
	ID          uint64    `xorm:"pk autoincr bigint unsigned 'id'" json:"id"`
	Code        string    `xorm:"varchar(20) notnull unique 'code'" json:"code"`
	Name        string    `xorm:"varchar(50) notnull 'name'" json:"name"`
	Level       int       `xorm:"int notnull 'level'" json:"level"`
	Description string    `xorm:"text 'description'" json:"description"`
	CreateTime  time.Time `xorm:"created" json:"create_time"`
	UpdateTime  time.Time `xorm:"updated" json:"update_time"`
}

// IsOwnerOnlyPermission 判断是否为仅自己数据权限
func (dp *DataPermission) IsOwnerOnlyPermission() bool {
	return dp.Code == DataPermissionOwnData
}

// IsDepartmentPermission 判断是否为部门级权限
func (dp *DataPermission) IsDepartmentPermission() bool {
	return dp.Code == DataPermissionDeptData
}

// IsAllDataPermission 判断是否为全公司数据权限
func (dp *DataPermission) IsAllDataPermission() bool {
	return dp.Code == DataPermissionAllData
}
