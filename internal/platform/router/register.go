package router

import (
	"context"
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"unicode"

	"github.com/ayxworxfr/go_admin/pkg/api"
	"github.com/ayxworxfr/go_admin/pkg/logger"
)

// Method HTTP 方法
type Method string

const (
	GET    Method = "GET"
	POST   Method = "POST"
	PUT    Method = "PUT"
	DELETE Method = "DELETE"
)

func (m Method) String() string { return string(m) }

// PathFormat 方法名推断路径时的格式策略
type PathFormat int

const (
	// SnakeCase 驼峰转下划线：UserList -> /user_list
	SnakeCase PathFormat = iota
	// SlashCase 驼峰转斜杠：UserList -> /user/list
	SlashCase
)

// Router 一条已解析的路由定义
type Router struct {
	path        string
	method      Method
	handlerFunc any
}

// NewRouter 创建路由定义
func NewRouter(method, path string, handlerFunc any) *Router {
	return &Router{
		path:        path,
		method:      Method(method),
		handlerFunc: handlerFunc,
	}
}

func (r *Router) Path() string   { return r.path }
func (r *Router) Method() Method { return r.method }
func (r *Router) Handler() any   { return r.handlerFunc }

func (r *Router) Valid() bool {
	return r != nil && r.path != "" && r.method != "" && r.handlerFunc != nil
}

type routeKey struct {
	method Method
	path   string
}

// Register 统一路由注册器：优先查编译期路由表（routes_gen.go，来自 // @route），
// 否则按函数名推断。替代旧的运行时 ParseFile 方案（Docker / -trimpath 下不可用）。
type Register struct {
	format PathFormat
	seen   map[routeKey]struct{}
	routes []*Router
}

// NewRegister 创建注册器，默认 SlashCase（UserList -> /user/list）。
func NewRegister() *Register {
	return &Register{
		format: SlashCase,
		seen:   make(map[routeKey]struct{}),
	}
}

// WithPathFormat 设置函数名推断时的路径格式（不影响编译表中的显式路径）。
func (r *Register) WithPathFormat(format PathFormat) *Register {
	r.format = format
	return r
}

// Routes 返回本注册器已挂载的路由快照。
func (r *Register) Routes() []*Router {
	return r.routes
}

// RegisterRouters 注册显式构造的路由。
func (r *Register) RegisterRouters(group *RouterGroup, routers ...*Router) {
	for _, rt := range routers {
		r.mount(group, rt)
	}
}

// RegisterStruct 扫描结构体导出方法并注册处理器。
// 方法须以 *api.Context 为第一个参数（接收器之后）。
func (r *Register) RegisterStruct(group *RouterGroup, instances ...any) {
	for _, instance := range instances {
		r.registerStruct(group, instance)
	}
}

// RegisterFuncs 按函数值注册；无源码标签时按函数名推断。
func (r *Register) RegisterFuncs(group *RouterGroup, handlers ...any) {
	for _, h := range handlers {
		name := functionName(h)
		method, path := inferFromName(name, r.format)
		r.mount(group, &Router{path: path, method: method, handlerFunc: h})
	}
}

func (r *Register) registerStruct(group *RouterGroup, instance any) {
	v := reflect.ValueOf(instance)
	t := reflect.TypeOf(instance)
	if v.Kind() != reflect.Ptr {
		if !v.CanAddr() {
			logger.Warnf(context.Background(), "实例不可寻址，无法注册为处理器: %v", t)
			return
		}
		v = v.Addr()
		t = v.Type()
	}

	logger.Debugf(context.Background(), "RegisterStruct: %s methods=%d", t.Elem().Name(), v.NumMethod())

	for i := 0; i < t.NumMethod(); i++ {
		method := t.Method(i)
		if !isHandlerMethod(method) {
			continue
		}
		handler := v.Method(method.Index).Interface()
		rt := r.resolveRoute(method, handler)
		if rt.Valid() {
			r.mount(group, rt)
		}
	}
}

func (r *Register) resolveRoute(method reflect.Method, handler any) *Router {
	if tag, ok := lookupMethodTag(method); ok {
		return &Router{path: tag.path, method: tag.method, handlerFunc: handler}
	}
	m, path := inferFromName(method.Name, r.format)
	return &Router{path: path, method: m, handlerFunc: handler}
}

func (r *Register) mount(group *RouterGroup, rt *Router) {
	if !rt.Valid() {
		panic("invalid router: " + rt.path)
	}
	if group == nil {
		panic("group is nil")
	}

	key := routeKey{method: rt.method, path: rt.path}
	if _, dup := r.seen[key]; dup {
		logger.Debugf(context.Background(), "Router already registered: %s %s", rt.method, rt.path)
		return
	}

	switch rt.method {
	case GET:
		group.GET(rt.path, rt.handlerFunc)
	case POST:
		group.POST(rt.path, rt.handlerFunc)
	case PUT:
		group.PUT(rt.path, rt.handlerFunc)
	case DELETE:
		group.DELETE(rt.path, rt.handlerFunc)
	default:
		panic(fmt.Sprintf("unsupported router method: %s", rt.method))
	}

	r.seen[key] = struct{}{}
	r.routes = append(r.routes, rt)
}

// isHandlerMethod 判断是否为业务处理器：
// func (h *T) Xxx(c *api.Context, ...) —— In(0)=recv, In(1)=*Context
func isHandlerMethod(method reflect.Method) bool {
	t := method.Type
	if t.NumIn() < 2 {
		return false
	}
	return t.In(1) == reflect.TypeOf((*api.Context)(nil))
}

func inferFromName(funcName string, strategy PathFormat) (Method, string) {
	method, base := splitMethodPrefix(funcName)
	return method, formatPath(base, strategy)
}

func splitMethodPrefix(funcName string) (Method, string) {
	switch {
	case strings.HasPrefix(funcName, "Get"):
		return GET, strings.TrimPrefix(funcName, "Get")
	case strings.HasPrefix(funcName, "Post"):
		return POST, strings.TrimPrefix(funcName, "Post")
	case strings.HasPrefix(funcName, "Create"):
		return POST, strings.TrimPrefix(funcName, "Create")
	case strings.HasPrefix(funcName, "Put"):
		return PUT, strings.TrimPrefix(funcName, "Put")
	case strings.HasPrefix(funcName, "Update"):
		return PUT, strings.TrimPrefix(funcName, "Update")
	case strings.HasPrefix(funcName, "Delete"):
		return DELETE, strings.TrimPrefix(funcName, "Delete")
	default:
		return POST, funcName
	}
}

func functionName(handlerFunc any) string {
	full := runtime.FuncForPC(reflect.ValueOf(handlerFunc).Pointer()).Name()
	return cleanFuncName(full)
}

func cleanFuncName(fullName string) string {
	if i := strings.LastIndex(fullName, "."); i >= 0 {
		fullName = fullName[i+1:]
	}
	switch {
	case strings.HasSuffix(fullName, "-fm"):
		return fullName[:len(fullName)-3]
	case strings.HasSuffix(fullName, "-m"):
		return fullName[:len(fullName)-2]
	default:
		return fullName
	}
}

// formatPath 将方法名去掉 HTTP 前缀后的剩余部分格式化为 URL 路径。
// List 后缀约定为资源的列表接口：UserList -> /user/list。
func formatPath(name string, strategy PathFormat) string {
	if strings.HasSuffix(name, "List") {
		base := strings.TrimSuffix(name, "List")
		return applyPathFormat(base, strategy) + "/list"
	}
	return applyPathFormat(name, strategy)
}

func applyPathFormat(name string, strategy PathFormat) string {
	switch strategy {
	case SlashCase:
		return "/" + toSlashCase(name)
	default:
		return "/" + toSnakeCase(name)
	}
}

func toSnakeCase(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 4)
	var prev rune
	for i, c := range s {
		if i > 0 && unicode.IsUpper(c) && prev != '_' {
			b.WriteByte('_')
		}
		b.WriteRune(unicode.ToLower(c))
		prev = c
	}
	return b.String()
}

func toSlashCase(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 4)
	var prev rune
	for i, c := range s {
		if i > 0 && unicode.IsUpper(c) && prev != '/' {
			b.WriteByte('/')
		}
		b.WriteRune(unicode.ToLower(c))
		prev = c
	}
	return b.String()
}
