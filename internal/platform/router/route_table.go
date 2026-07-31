package router

import (
	"fmt"
	"reflect"
)

// routeTag 一条已解析的路由元数据（HTTP 方法 + 路径）。
type routeTag struct {
	method Method
	path   string
}

// compiledRoutes 由 routes_gen.go 的 init 填充，也可在测试中追加。
// 键格式：PkgPath.TypeName.MethodName，与 reflect 对齐。
var compiledRoutes = map[string]routeTag{}

// registerCompiledRoute 供生成代码与测试注册编译期路由。
func registerCompiledRoute(pkgPath, typeName, methodName string, method Method, path string) {
	key := pkgPath + "." + typeName + "." + methodName
	if _, dup := compiledRoutes[key]; dup {
		panic(fmt.Sprintf("router: duplicate compiled route: %s", key))
	}
	compiledRoutes[key] = routeTag{method: method, path: path}
}

// lookupMethodTag 从编译进二进制的路由表查找 // @route 元数据。
// 找不到时返回 ok=false，由调用方回退到函数名推断。
func lookupMethodTag(method reflect.Method) (routeTag, bool) {
	tag, ok := compiledRoutes[methodRouteKey(method)]
	return tag, ok
}

func methodRouteKey(method reflect.Method) string {
	recv := method.Type.In(0)
	if recv.Kind() == reflect.Ptr {
		recv = recv.Elem()
	}
	return recv.PkgPath() + "." + recv.Name() + "." + method.Name
}
