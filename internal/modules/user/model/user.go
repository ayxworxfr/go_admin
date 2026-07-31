package model

import "time"

// User 用户模型。不再持有密码哈希/校验方法——加密算法是可替换的策略，
// 由 Service 层持有 crypter.PasswordHasher 依赖，模型只保留纯数据结构，
// 避免"数据结构与具体加密实现耦合"导致以后换算法要改模型定义。
type User struct {
	ID       uint64 `xorm:"pk autoincr bigint unsigned 'id'" json:"id"`
	Username string `xorm:"varchar(50) notnull unique 'username'" json:"username"`
	// PasswordHash 持久化的 Argon2id 自描述哈希串，不是明文密码。
	// json:"-"：即使误把 model 直接序列化进响应，也不会泄露凭证材料。
	PasswordHash  string    `xorm:"varchar(255) notnull 'password_hash'" json:"-"`
	Email         string    `xorm:"varchar(100) notnull unique 'email'" json:"email"`
	Phone         string    `xorm:"varchar(20) 'phone'" json:"phone"`
	AvatarURL     string    `xorm:"varchar(255) 'avatar_url'" json:"avatar_url"`
	Status        int       `xorm:"int 'status'" json:"status"`
	CreateTime    time.Time `xorm:"created" json:"create_time"`
	UpdateTime    time.Time `xorm:"updated" json:"update_time"`
	LastLoginTime time.Time `xorm:"datetime 'last_login_time'" json:"last_login_time"`
}
