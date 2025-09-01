package router

import "github.com/ayxworxfr/go_admin/internal/handler"

var (
	AutoRegister IRouterRegister
	TagRegister  *TagRouterRegister
)

func init() {
	AutoRegister = NewAutoRouterRegister()
	TagRegister = NewTagRouterRegister()
	// 如果需要扫描包，需要先在这里注册实例
	TagRegister.AddHandlerInstance(handler.AllHandlerInstance...)
}

// IRouterRegister 路由注册接口
type IRouterRegister interface {
	RegisterRouters(group *RouterGroup, routers ...*Router)
	RegisterRouterByFunc(group *RouterGroup, handlerFuncList ...any)
	RegisterStruct(group *RouterGroup, instanceList ...any)
	RegisterByPackage(group *RouterGroup, pkgPath string) error
}

// 路径格式策略
type PathFormatStrategy int

const (
	// 驼峰转下划线：UserList -> /user_list
	SnakeCase PathFormatStrategy = iota
	// 驼峰转斜杠：UserList -> /user/list
	SlashCase
)

// Router 路由定义
type Router struct {
	path        string
	method      RouterMethod
	handlerFunc any
}

func NewRouter(method string, path string, handlerFunc any) *Router {
	return &Router{
		path:        path,
		method:      RouterMethod(method),
		handlerFunc: handlerFunc,
	}
}

func (r *Router) GetPath() string {
	return r.path
}

func (r *Router) GetMethod() RouterMethod {
	return r.method
}

func (r *Router) GetHandlerFunc() any {
	return r.handlerFunc
}

func (r *Router) IsValid() bool {
	if r.path == "" || r.method == "" || r.handlerFunc == nil {
		return false
	}
	return true
}

// RouterMethod HTTP方法
type RouterMethod string

const (
	GET    RouterMethod = "GET"
	POST   RouterMethod = "POST"
	PUT    RouterMethod = "PUT"
	DELETE RouterMethod = "DELETE"
)

func (r RouterMethod) Value() string {
	return string(r)
}
