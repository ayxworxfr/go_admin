package vo

import (
	"time"

	"github.com/ayxworxfr/go_admin/pkg/jwtauth"
)

// User 用户视图对象
type User struct {
	ID            uint64    `json:"id"`
	Username      string    `json:"username"`
	Email         string    `json:"email"`
	Phone         string    `json:"phone"`
	AvatarURL     string    `json:"avatar_url"`
	Roles         []*Role   `json:"roles"`
	Status        int       `json:"status"`
	CreateTime    time.Time `json:"create_time"`
	UpdateTime    time.Time `json:"update_time"`
	LastLoginTime time.Time `json:"last_login_time"`
}

type UserRoutes struct {
	Username string   `json:"username"`
	Role     string   `json:"role"`
	Routes   []string `json:"routes"`
}

func NewUserRoutes(claims *jwtauth.Claims, routers []string) *UserRoutes {
	return &UserRoutes{
		Username: claims.Nice,
		Role:     claims.RoleKey,
		Routes:   routers,
	}
}
