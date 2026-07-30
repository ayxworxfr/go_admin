package router

import (
	mycontext "context"
	"reflect"
	"runtime"
	"strings"

	"github.com/ayxworxfr/go_admin/pkg/context"
	"github.com/ayxworxfr/go_admin/pkg/logger"
)

// AutoRouterRegister 基于函数名称自动推断的路由注册器
type AutoRouterRegister struct {
	PathFormatStrategy PathFormatStrategy
	routers            []*Router
}

// NewAutoRouterRegister 创建基于函数名称的路由注册器
func NewAutoRouterRegister() *AutoRouterRegister {
	return &AutoRouterRegister{
		PathFormatStrategy: SlashCase,
	}
}

// RegisterRouters 注册多个路由
func (r *AutoRouterRegister) RegisterRouters(group *RouterGroup, routers ...*Router) {
	for _, router := range routers {
		r.register(group, router)
	}
}

// RegisterRouterByFunc 基于函数名称自动推断并注册路由
func (r *AutoRouterRegister) RegisterRouterByFunc(group *RouterGroup, handlerFuncList ...any) {
	for _, h := range handlerFuncList {
		router := r.inferRouter(h)
		r.register(group, router)
	}
}

// RegisterStruct 扫描结构体方法并基于函数名注册路由
func (r *AutoRouterRegister) RegisterStruct(group *RouterGroup, instanceList ...any) {
	logger.Debugf(mycontext.Background(), "RegisterStruct: %d", len(instanceList))
	for _, instance := range instanceList {
		r.registerStruct(group, instance)
	}
}

func (r *AutoRouterRegister) RegisterByPackage(group *RouterGroup, pkgPath string) error {
	// 暂时不支持
	return nil
}

func (r *AutoRouterRegister) registerStruct(group *RouterGroup, instance any) {
	// 获取实例的反射值和类型
	v := reflect.ValueOf(instance)
	t := reflect.TypeOf(instance)
	// 处理非指针情况（确保是指针类型）
	if v.Kind() != reflect.Ptr {
		if v.CanAddr() {
			v = v.Addr()
			t = v.Type()
		} else {
			logger.Warnf(mycontext.Background(), "实例不可寻址，无法注册为处理器: %v", t)
			return
		}
	}

	methodNum := v.NumMethod()
	logger.Debugf(mycontext.Background(), "Struct type: %s, Method count: %d", t.Elem().Name(), methodNum)
	// 遍历所有方法
	for i := 0; i < methodNum; i++ {
		method := t.Method(i)
		methodType := method.Type

		// 检查方法签名是否符合处理器要求
		if methodType.NumIn() >= 1 && methodType.In(1) == reflect.TypeOf(&context.Context{}) {
			// 获取方法值并调用
			methodValue := v.MethodByName(method.Name)
			if !methodValue.IsValid() {
				logger.Warnf(mycontext.Background(), "无法获取方法值: %s", method.Name)
				continue
			}

			// 创建处理函数
			handlerFunc := methodValue.Interface()
			// 根据函数名推断路由
			router := r.inferRouter(handlerFunc, method.Name)
			if router.IsValid() {
				r.register(group, router)
			}
		}
	}
}

// GetRouters 获取已注册的路由
func (r *AutoRouterRegister) GetRouters() []*Router {
	return r.routers
}

// register 执行实际的路由注册
func (r *AutoRouterRegister) register(group *RouterGroup, router *Router) {
	if !router.IsValid() {
		msg := "invalid router: " + router.path
		panic(msg)
	}
	if group == nil {
		panic("group is nil")
	}

	switch router.method {
	case GET:
		group.GET(router.path, router.handlerFunc)
	case POST:
		group.POST(router.path, router.handlerFunc)
	case PUT:
		group.PUT(router.path, router.handlerFunc)
	case DELETE:
		group.DELETE(router.path, router.handlerFunc)
	default:
		panic("unsupported router method")
	}
	r.routers = append(r.routers, router)
}

// inferRouter 基于函数名称推断路由
func (r *AutoRouterRegister) inferRouter(handlerFunc any, methodName ...string) *Router {
	var funcName string
	if len(methodName) > 0 {
		funcName = methodName[0]
	} else {
		funcName = runtime.FuncForPC(reflect.ValueOf(handlerFunc).Pointer()).Name()
		funcName = r.extractFunctionName(funcName)
	}

	method, pathBase := r.inferMethodAndPathBase(funcName)
	path := r.formatPath(pathBase)

	return &Router{
		path:        path,
		method:      method,
		handlerFunc: handlerFunc,
	}
}

// extractFunctionName 提取并清理函数名
func (r *AutoRouterRegister) extractFunctionName(fullName string) string {
	lastDot := strings.LastIndex(fullName, ".")
	var funcName string
	if lastDot > 0 {
		funcName = fullName[lastDot+1:]
	} else {
		funcName = fullName
	}

	if strings.HasSuffix(funcName, "-fm") {
		return funcName[:len(funcName)-3]
	}
	if strings.HasSuffix(funcName, "-m") {
		return funcName[:len(funcName)-2]
	}
	return funcName
}

// inferMethodAndPathBase 推断HTTP方法和基础路径
func (r *AutoRouterRegister) inferMethodAndPathBase(funcName string) (RouterMethod, string) {
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

// formatPath 格式化路径
func (r *AutoRouterRegister) formatPath(name string) string {
	if strings.HasSuffix(name, "List") {
		base := strings.TrimSuffix(name, "List")
		return r.applyFormatStrategy(base) + "/list"
	}
	return r.applyFormatStrategy(name)
}

// applyFormatStrategy 应用路径格式策略
func (r *AutoRouterRegister) applyFormatStrategy(name string) string {
	switch r.PathFormatStrategy {
	case SnakeCase:
		return "/" + toSnakeCase(name)
	case SlashCase:
		return "/" + toSlashCase(name)
	default:
		return "/" + toSnakeCase(name)
	}
}
