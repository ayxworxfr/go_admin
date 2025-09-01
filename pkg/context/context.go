package context

import (
	"context"
	"mime/multipart"
	"strings"

	"github.com/ayxworxfr/go_admin/pkg/jwtauth"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol"
)

// Context 是对 Hertz 的 app.RequestContext 的封装
type Context struct {
	ctx context.Context
	*app.RequestContext
}

// NewContext 创建一个新的 Context
func NewContext(ctx context.Context, c *app.RequestContext) *Context {
	return &Context{ctx: ctx, RequestContext: c}
}

// Context 返回原始的 context.Context
func (ctx *Context) Context() context.Context {
	return ctx.ctx
}

func (ctx *Context) GetUserID() uint64 {
	if userID, err := jwtauth.Instance.GetUserIDUint64(ctx.RequestContext); err == nil {
		return userID
	}
	return 0
}

// GetUserValue 从用户值中获取指定键的值
func (ctx *Context) GetUserValue(key string) (value any, exists bool) {
	return ctx.RequestContext.Get(key)
}

// SetUserValue 设置用户值
func (ctx *Context) SetUserValue(key string, value any) {
	ctx.RequestContext.Set(key, value)
}

// JSON 将给定的结构体序列化为 JSON 并写入响应体
func (ctx *Context) JSON(code int, obj any) {
	ctx.RequestContext.JSON(code, obj)
}

// String 将给定的字符串写入响应体
func (ctx *Context) String(code int, format string, values ...any) {
	ctx.RequestContext.String(code, format, values...)
}

// Redirect 重定向请求到指定的 URL
func (ctx *Context) Redirect(code int, location string) {
	ctx.RequestContext.Redirect(code, []byte(location))
}

// SetCookie 添加一个 Set-Cookie 头到响应头中
func (ctx *Context) SetCookie(name, value string, maxAge int, path, domain string, secure, httpOnly bool) {
	ctx.RequestContext.SetCookie(name, value, maxAge, path, domain, protocol.CookieSameSiteDefaultMode, secure, httpOnly)
}

// Cookie 返回请求中指定名称的 cookie 值
func (ctx *Context) Cookie(name string) string {
	return string(ctx.RequestContext.Cookie(name))
}

// SetHeader 设置响应头
func (ctx *Context) SetHeader(key, value string) {
	ctx.RequestContext.Response.Header.Set(key, value)
}

// GetHeader 获取请求头
func (ctx *Context) GetHeader(key string) string {
	return string(ctx.RequestContext.Request.Header.Peek(key))
}

// GetResponseHeader 获取响应头
func (ctx *Context) GetResponseHeader(key string) string {
	return string(ctx.RequestContext.Response.Header.Peek(key))
}

// Bind 将请求数据绑定到给定的结构体指针
func (ctx *Context) Bind(obj any) error {
	return ctx.RequestContext.Bind(obj)
}

// ClientIP 返回客户端的 IP 地址
func (ctx *Context) ClientIP() string {
	return ctx.RequestContext.ClientIP()
}

// Abort 阻止待处理的处理程序被调用
func (ctx *Context) Abort() {
	ctx.RequestContext.Abort()
}

// AbortWithStatus 调用 Abort 并写入指定的状态码
func (ctx *Context) AbortWithStatus(code int) {
	ctx.RequestContext.AbortWithStatus(code)
}

// IsAborted 返回当前上下文是否已经被终止
func (ctx *Context) IsAborted() bool {
	return ctx.RequestContext.IsAborted()
}

// Param 返回 URL 参数的值
func (ctx *Context) Param(key string) string {
	return ctx.RequestContext.Param(key)
}

// Query 返回 URL 查询参数的值
func (ctx *Context) Query(key string) string {
	return ctx.RequestContext.Query(key)
}

// PostForm 返回 POST 表单中指定键的值
func (ctx *Context) PostForm(key string) string {
	return ctx.RequestContext.PostForm(key)
}

// FormFile 返回指定表单键的第一个文件
func (ctx *Context) FormFile(name string) (*multipart.FileHeader, error) {
	return ctx.RequestContext.FormFile(name)
}

// SaveUploadedFile 将上传的文件保存到指定目标
func (ctx *Context) SaveUploadedFile(file *multipart.FileHeader, dst string) error {
	return ctx.RequestContext.SaveUploadedFile(file, dst)
}

// Status 设置 HTTP 响应状态码
func (ctx *Context) Status(code int) {
	ctx.RequestContext.SetStatusCode(code)
}

// Data 将一些数据写入响应体并更新 HTTP 状态码
func (ctx *Context) Data(code int, contentType string, data []byte) {
	ctx.RequestContext.Data(code, contentType, data)
}

// HTML 渲染 HTML 模板
func (ctx *Context) HTML(code int, name string, obj any) {
	ctx.RequestContext.HTML(code, name, obj)
}

// File 将指定文件写入响应体
func (ctx *Context) File(filepath string) {
	ctx.RequestContext.File(filepath)
}

// ContentType 返回请求的 Content-Type 头
func (ctx *Context) ContentType() string {
	return string(ctx.RequestContext.ContentType())
}

// IsWebsocket 如果请求头表明客户端正在发起 websocket 握手，则返回 true
func (ctx *Context) IsWebsocket() bool {
	if strings.Contains(strings.ToLower(string(ctx.GetHeader("Connection"))), "upgrade") &&
		strings.ToLower(string(ctx.GetHeader("Upgrade"))) == "websocket" {
		return true
	}
	return false
}

// FullPath 返回匹配的路由完整路径
func (ctx *Context) FullPath() string {
	return ctx.RequestContext.FullPath()
}

// Method 返回请求的 HTTP 方法
func (ctx *Context) Method() string {
	return string(ctx.RequestContext.Method())
}

// Path 返回请求的路径
func (ctx *Context) Path() string {
	return string(ctx.RequestContext.Path())
}

// RequestBody 返回请求体
func (ctx *Context) RequestBody() []byte {
	return ctx.RequestContext.Request.Body()
}

// SetStatusCode 设置响应状态码
func (ctx *Context) SetStatusCode(statusCode int) {
	ctx.RequestContext.SetStatusCode(statusCode)
}

// WriteString 将字符串写入响应
func (ctx *Context) WriteString(s string) (int, error) {
	return ctx.RequestContext.WriteString(s)
}

// Write 将字节切片写入响应
func (ctx *Context) Write(data []byte) (int, error) {
	return ctx.RequestContext.Write(data)
}

// Error 将错误附加到当前上下文
func (ctx *Context) Error(err error) {
	ctx.RequestContext.Error(err)
}

// GetRawData 获取原始请求体
func (ctx *Context) GetRawData() []byte {
	return ctx.RequestContext.Request.Body()
}

// SetCookieKV 设置 cookie（简化版）
func (ctx *Context) SetCookieKV(key, value string) {
	ctx.RequestContext.SetCookie(key, value, 0, "/", "", protocol.CookieSameSiteDefaultMode, false, true)
}
