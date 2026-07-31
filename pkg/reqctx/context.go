package reqctx

import (
	"context"
	"strings"

	"github.com/ayxworxfr/go_admin/pkg/jwtauth"
	"github.com/cloudwego/hertz/pkg/app"
)

// Context 是 handler 层的请求上下文：组合 stdlib context 与 Hertz RequestContext，
// 只暴露本项目真正用到的能力，不再把整个 Hertz API 面泄漏给业务。
//
// 需要底层能力时用 Request() 逃生，而不是在本包堆薄包装方法。
type Context struct {
	std context.Context
	rc  *app.RequestContext
}

// New 创建请求上下文（由 router 在每个请求入口调用）
func New(std context.Context, rc *app.RequestContext) *Context {
	return &Context{std: std, rc: rc}
}

// Context 返回 stdlib context.Context，供下沉到 service / repository
func (c *Context) Context() context.Context {
	return c.std
}

// Request 返回底层 Hertz RequestContext（逃生舱，少用）
func (c *Context) Request() *app.RequestContext {
	return c.rc
}

// JSON 写入 JSON 响应
func (c *Context) JSON(code int, obj any) {
	c.rc.JSON(code, obj)
}

// String 写入字符串响应
func (c *Context) String(code int, format string, values ...any) {
	c.rc.String(code, format, values...)
}

// IsAborted 当前链路是否已中断
func (c *Context) IsAborted() bool {
	return c.rc.IsAborted()
}

// Abort 中断后续处理
func (c *Context) Abort() {
	c.rc.Abort()
}

// Header 读取请求头
func (c *Context) Header(key string) string {
	return string(c.rc.Request.Header.Peek(key))
}

// BindAndValidate 绑定并校验请求体/查询参数（用于无 DTO 第二参的 handler，如 Login）
func (c *Context) BindAndValidate(obj any) error {
	return c.rc.BindAndValidate(obj)
}

// BearerToken 提取 Authorization: Bearer <token>；无 Bearer 前缀时原样返回，空则 ""
func (c *Context) BearerToken() string {
	raw := strings.TrimSpace(c.Header("Authorization"))
	if raw == "" {
		return ""
	}
	if len(raw) > 7 && strings.EqualFold(raw[:7], "Bearer ") {
		return strings.TrimSpace(raw[7:])
	}
	return raw
}

// UserID 从鉴权中间件注入的 JWT 载荷读取用户 ID；未登录返回 error，不再静默给 0
func (c *Context) UserID() (uint64, error) {
	return jwtauth.UserIDUint64FromContext(c.rc)
}

// Claims 从鉴权中间件注入的 JWT 载荷读取完整 Claims
func (c *Context) Claims() (*jwtauth.Claims, error) {
	return jwtauth.ClaimsFromContext(c.rc)
}
