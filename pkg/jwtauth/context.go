package jwtauth

import (
	"errors"
	"strconv"

	"github.com/cloudwego/hertz/pkg/app"
)

// ClaimsFromContext 从请求上下文读取中间件已注入的 JWT 载荷。
// 不依赖 *JWT 实例——签名校验在中间件完成，这里只做类型安全的取值。
func ClaimsFromContext(c *app.RequestContext) (*Claims, error) {
	claims, exists := c.Get(ClaimsKey)
	if !exists {
		return nil, errors.New("jwt claims not found in context")
	}
	typed, ok := claims.(*Claims)
	if !ok || typed == nil {
		return nil, errors.New("invalid jwt claims in context")
	}
	return typed, nil
}

// UserIDUint64FromContext 从请求上下文获取用户 ID（uint64）。
func UserIDUint64FromContext(c *app.RequestContext) (uint64, error) {
	claims, err := ClaimsFromContext(c)
	if err != nil {
		return 0, err
	}
	return strconv.ParseUint(claims.Identity, 10, 64)
}
