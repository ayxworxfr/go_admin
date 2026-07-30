package model

import "time"

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
