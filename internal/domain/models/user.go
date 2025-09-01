package models

import (
	"time"

	"github.com/ayxworxfr/go_admin/pkg/crypter"
)

// User 用户模型
type User struct {
	ID            uint64    `xorm:"pk autoincr bigint unsigned 'id'" json:"id"`
	Username      string    `xorm:"varchar(50) notnull unique 'username'" json:"username"`
	Password      string    `xorm:"varchar(100) notnull 'password'" json:"password"`
	Email         string    `xorm:"varchar(100) notnull unique 'email'" json:"email"`
	Phone         string    `xorm:"varchar(20) 'phone'" json:"phone"`
	AvatarURL     string    `xorm:"varchar(255) 'avatar_url'" json:"avatar_url"`
	DepartmentID  uint64    `xorm:"int 'department_id'" json:"department_id"`
	Status        int       `xorm:"int 'status'" json:"status"`
	CreateTime    time.Time `xorm:"created" json:"create_time"`
	UpdateTime    time.Time `xorm:"updated" json:"update_time"`
	LastLoginTime time.Time `xorm:"datetime 'last_login_time'" json:"last_login_time"`
}

func (u *User) Verify(password string) bool {
	return crypter.Instance.Verify(password, u.Password)
}

func (u *User) EncryptPassword() {
	u.Password = EncryptPassword(u.Password)
}

func EncryptPassword(password string) string {
	return crypter.Instance.Encrypt(password)
}
