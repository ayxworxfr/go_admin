package dto

import (
	"time"

	"github.com/ayxworxfr/go_admin/pkg/apiparam"
)

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
	ID uint64 `query:"id" vd:"$>0"`
}

// GetSystemSettingListRequest 获取系统配置列表请求
type GetSystemSettingListRequest struct {
	apiparam.Page
	Category string `query:"category" vd:"len($)>=0&&len($)<50" xorm:"category op=eq"`
	Key      string `query:"key" vd:"len($)>=0&&len($)<50" xorm:"key op=like"`
	Type     uint8  `query:"type" xorm:"type op=eq"`
}

// GetSystemSettingByCategoryRequest 根据分类获取系统配置请求
type GetSystemSettingByCategoryRequest struct {
	Category string `query:"category" vd:"len($)>0&&len($)<50"`
}

// CreatorResponse 系统配置创建人的极简视图，只承载展示所需的两个字段，
// 不引入 user 模块的 UserResponse——避免 systemsetting 的响应结构反过来
// 感知 user 模块的字段全集。
type CreatorResponse struct {
	ID       uint64 `json:"id"`
	Username string `json:"username"`
}

// SystemSettingResponse 系统配置视图对象
type SystemSettingResponse struct {
	ID          uint64           `json:"id"`
	Category    string           `json:"category"`
	Key         string           `json:"key"`
	Value       string           `json:"value"`
	Type        uint8            `json:"type"`
	TypeDisplay string           `json:"type_display"`
	Description string           `json:"description"`
	CreateBy    *CreatorResponse `json:"create_by,omitempty"`
	CreateTime  time.Time        `json:"create_time"`
	UpdateTime  time.Time        `json:"update_time"`
}
