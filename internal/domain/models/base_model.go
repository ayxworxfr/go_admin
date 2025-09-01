package models

import "time"

// Role 角色模型
type BaseModel struct {
	ID         uint64    `xorm:"pk autoincr 'id'" json:"id"`
	CreateTime time.Time `xorm:"created" json:"create_time"`
	UpdateTime time.Time `xorm:"updated" json:"update_time"`
}
