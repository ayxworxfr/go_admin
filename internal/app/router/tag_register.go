package router

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"regexp"
	"runtime"
	"strings"
	"unicode"

	mycontext "context"

	"github.com/ayxworxfr/go_admin/pkg/context"
	"github.com/ayxworxfr/go_admin/pkg/logger"
	"github.com/ayxworxfr/go_admin/pkg/utils"
)

// TagRouterRegister 基于路由标签的路由注册器
type TagRouterRegister struct {
	routers            []*Router
	PathFormatStrategy PathFormatStrategy
	allHandlers        map[string]any
}

// NewTagRouterRegister 创建基于标签的路由注册器
func NewTagRouterRegister() *TagRouterRegister {
	return &TagRouterRegister{
		PathFormatStrategy: SlashCase,
		allHandlers:        make(map[string]any),
	}
}

func (r *TagRouterRegister) AddHandlerInstance(handlers ...any) {
	for _, handler := range handlers {
		structName := reflect.TypeOf(handler).Elem().Name()
		r.allHandlers[structName] = handler
	}
}

// RegisterRouters 注册多个路由
func (r *TagRouterRegister) RegisterRouters(group *RouterGroup, routers ...*Router) {
	for _, router := range routers {
		r.register(group, router)
	}
}

// RegisterRouterByFunc 基于标签解析并注册路由
func (r *TagRouterRegister) RegisterRouterByFunc(group *RouterGroup, handlerFuncList ...any) {
	if len(handlerFuncList) < 2 {
		return
	}
	obj := handlerFuncList[0]
	v := reflect.ValueOf(obj)
	tp := reflect.TypeOf(obj)
	for _, h := range handlerFuncList {
		// obj reflect.Value, method reflect.Method
		funcName := runtime.FuncForPC(reflect.ValueOf(h).Pointer()).Name()
		funcName = r.extractFunctionName(funcName)
		method, ok := tp.MethodByName(funcName)
		if !ok {
			continue
		}
		router := r.parseMethodRoute(v, method)
		if router == nil {
			// 标签解析失败时，回退到函数名称推断
			router = r.inferRouter(h)
		}
		r.register(group, router)
	}
}

// RegisterStruct 扫描结构体实例的方法并注册有标签的路由
func (r *TagRouterRegister) RegisterStruct(group *RouterGroup, instanceList ...any) {
	for _, instance := range instanceList {
		r.registerStruct(group, instance)
	}
}

func (r *TagRouterRegister) registerStruct(group *RouterGroup, instance any) {
	v := reflect.ValueOf(instance)
	t := reflect.TypeOf(instance)

	methodNum := v.NumMethod()
	logger.Debugf(mycontext.Background(), "Struct type: %s, Method count: %d", t.Elem().Name(), methodNum)
	// 遍历所有方法
	for i := 0; i < methodNum; i++ {
		method := t.Method(i)
		methodType := method.Type

		// 检查方法签名是否符合处理器要求
		if methodType.NumIn() >= 1 && methodType.In(1) == reflect.TypeOf(&context.Context{}) {
			router := r.parseMethodRoute(v, method)
			if router != nil && router.IsValid() {
				r.register(group, router)
			}
		}
	}
}

// GetRouters 获取已注册的路由
func (r *TagRouterRegister) GetRouters() []*Router {
	return r.routers
}

// register 执行实际的路由注册
func (r *TagRouterRegister) register(group *RouterGroup, router *Router) {
	if router != nil && !router.IsValid() {
		msg := "invalid router: " + router.path
		panic(msg)
	}
	if group == nil {
		panic("group is nil")
	}
	// 避免重复注册
	for _, r := range r.routers {
		if r.path == router.path && r.method == router.method {
			logger.Debugf(mycontext.Background(), "Router already registered: %s %s", r.method, r.path)
			return
		}
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

func (r *TagRouterRegister) RegisterByPackage(group *RouterGroup, pkgPath string) error {
	pkgPath = utils.GetAbsPath(pkgPath)
	routes, err := r.parsePackageRoutes(pkgPath)
	if err != nil {
		return err
	}
	for _, route := range routes {
		r.register(group, route)
	}
	return nil
}

func (r *TagRouterRegister) parsePackageRoutes(pkgPath string) ([]*Router, error) {
	var routes []*Router

	// 创建文件集
	fset := token.NewFileSet()

	// 解析包目录
	pkgs, err := parser.ParseDir(fset, pkgPath, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	// 遍历所有包（通常只有一个）
	for _, pkg := range pkgs {
		// 遍历包内所有文件
		for fileName, file := range pkg.Files {
			logger.Debugf(mycontext.Background(), "Parsing file: %s", fileName)
			// 解析文件中的所有类型和方法
			structTypes := make(map[string]*ast.StructType)

			// 首先收集所有结构体定义
			for _, decl := range file.Decls {
				if genDecl, ok := decl.(*ast.GenDecl); ok && genDecl.Tok == token.TYPE {
					for _, spec := range genDecl.Specs {
						if typeSpec, ok := spec.(*ast.TypeSpec); ok {
							if structType, ok := typeSpec.Type.(*ast.StructType); ok {
								structTypes[typeSpec.Name.Name] = structType
							}
						}
					}
				}
			}

			// 然后查找所有方法并提取路由信息
			for _, decl := range file.Decls {
				if funcDecl, ok := decl.(*ast.FuncDecl); ok && funcDecl.Recv != nil {
					// 确保这是一个方法（有接收器）
					recv := funcDecl.Recv.List[0]
					var structName string

					// 处理指针接收器 (*StructType)
					if ptrType, ok := recv.Type.(*ast.StarExpr); ok {
						if ident, ok := ptrType.X.(*ast.Ident); ok {
							structName = ident.Name
						}
					} else if ident, ok := recv.Type.(*ast.Ident); ok {
						// 处理值接收器 (StructType)
						structName = ident.Name
					}

					// 检查是否为已注册的结构体
					if _, exists := structTypes[structName]; exists {
						// 检查方法注释中是否有 @route 标签
						if funcDecl.Doc != nil {
							for _, comment := range funcDecl.Doc.List {
								text := strings.TrimSpace(comment.Text)
								tags := extractRouteTag(text)
								if tags != nil {
									if len(tags) >= 2 {
										router := r.createRouterFromMethod(structName, funcDecl.Name.Name, tags[0], tags[1])
										if router != nil {
											routes = append(routes, router)
										}
									}
									break
								}
							}
						}
					}
				}
			}
		}
	}

	return routes, nil
}

// 创建路由时，存储方法的反射值
func (r *TagRouterRegister) createRouterFromMethod(structName string, methodName string, httpMethod, path string) *Router {
	// 从注册表中获取结构体实例
	structPtr := r.allHandlers[structName]
	if structPtr == nil {
		logger.Debugf(mycontext.Background(), "Struct %s not found in handlers", structName)
		return nil
	}

	// 获取结构体的反射值
	structValue := reflect.ValueOf(structPtr)

	// 检查是否为指针
	if structValue.Kind() != reflect.Ptr {
		// 如果不是指针，获取其地址（如果是可寻址的）
		if structValue.CanAddr() {
			structValue = structValue.Addr()
		} else {
			panic(fmt.Sprintf("结构体 %s 不可寻址", structName))
		}
	}

	// 获取方法
	methodValue := structValue.MethodByName(methodName)
	if !methodValue.IsValid() {
		panic(fmt.Sprintf("方法 %s 不存在或未导出", methodName))
	}

	// 验证方法签名
	methodType := methodValue.Type()
	if !(methodType.NumIn() >= 1 && methodType.In(0) == reflect.TypeOf(&context.Context{})) {
		panic(fmt.Sprintf("方法 %s 签名不符合要求", methodName))
	}

	return &Router{
		method:      RouterMethod(httpMethod),
		path:        path,
		handlerFunc: methodValue.Interface(),
	}
}

// parseMethodRoute 解析方法上的路由标签
func (r *TagRouterRegister) parseMethodRoute(obj reflect.Value, method reflect.Method) *Router {
	tags := getRouteTag(method)

	if tags == nil {
		return nil
	}

	logger.Debugf(mycontext.Background(), "parseMethodRoute tag: %s", tags)
	methodStr := strings.ToUpper(tags[0])
	path := tags[1]

	var httpMethod RouterMethod
	switch methodStr {
	case "GET", "POST", "PUT", "DELETE":
		httpMethod = RouterMethod(methodStr)
	default:
		return nil
	}

	return &Router{
		path:        path,
		method:      httpMethod,
		handlerFunc: obj.Method(method.Index).Interface(),
	}
}

// inferRouter 基于函数名称推断路由（回退机制）
func (r *TagRouterRegister) inferRouter(handlerFunc any) *Router {
	funcName := runtime.FuncForPC(reflect.ValueOf(handlerFunc).Pointer()).Name()
	funcName = r.extractFunctionName(funcName)

	method, pathBase := r.inferMethodAndPathBase(funcName)
	path := r.formatPath(pathBase)

	return &Router{
		path:        path,
		method:      method,
		handlerFunc: handlerFunc,
	}
}

// extractFunctionName 提取并清理函数名
func (r *TagRouterRegister) extractFunctionName(fullName string) string {
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
func (r *TagRouterRegister) inferMethodAndPathBase(funcName string) (RouterMethod, string) {
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
func (r *TagRouterRegister) formatPath(name string) string {
	if strings.HasSuffix(name, "List") {
		base := strings.TrimSuffix(name, "List")
		return r.applyFormatStrategy(base) + "/list"
	}
	return r.applyFormatStrategy(name)
}

// applyFormatStrategy 应用路径格式策略
func (r *TagRouterRegister) applyFormatStrategy(name string) string {
	switch r.PathFormatStrategy {
	case SnakeCase:
		return "/" + toSnakeCase(name)
	case SlashCase:
		return "/" + toSlashCase(name)
	default:
		return "/" + toSnakeCase(name)
	}
}

// getRouteTag 获取方法上的路由标签
// getRouteTag 通过反射获取方法的 @route 标签
func getRouteTag(method reflect.Method) []string {
	// 获取方法的函数指针
	funcPtr := method.Func.Pointer()
	funcInfo := runtime.FuncForPC(funcPtr)
	if funcInfo == nil {
		return nil
	}

	// 获取方法所在的源文件和行号
	file, _ := funcInfo.FileLine(funcPtr)

	// 创建文件集并解析单个文件
	fset := token.NewFileSet()
	astFile, err := parser.ParseFile(fset, file, nil, parser.ParseComments)
	if err != nil {
		return nil
	}

	// 获取接收器类型名称
	receiverType := method.Type.In(0) // 方法的第一个参数是接收器
	if receiverType.Kind() == reflect.Ptr {
		receiverType = receiverType.Elem() // 解引用指针
	}
	receiverTypeName := receiverType.Name()

	// 查找匹配的方法声明
	for _, decl := range astFile.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok || funcDecl.Name.Name != method.Name {
			continue // 不是目标方法
		}

		// 检查接收器类型
		if funcDecl.Recv == nil || len(funcDecl.Recv.List) == 0 {
			continue // 不是方法（没有接收器）
		}

		recvType := funcDecl.Recv.List[0].Type
		var recvTypeName string

		// 处理指针接收器 (*T)
		if ptrType, ok := recvType.(*ast.StarExpr); ok {
			if ident, ok := ptrType.X.(*ast.Ident); ok {
				recvTypeName = ident.Name
			}
		} else if ident, ok := recvType.(*ast.Ident); ok {
			// 处理值接收器 (T)
			recvTypeName = ident.Name
		}

		if recvTypeName != receiverTypeName {
			continue // 接收器类型不匹配
		}

		// 找到匹配的方法，提取 @route 标签
		if funcDecl.Doc != nil {
			return extractRouteTag(funcDecl.Doc.Text())
		}
	}
	// 从注释中提取@route标签
	return nil
}

// extractRouteTag 从注释文本中提取路由标签
func extractRouteTag(comment string) []string {
	lines := strings.Split(comment, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// 获取@route后面的内容
		re := regexp.MustCompile(`@route\s+([\s\w/]*)`)
		if re.MatchString(line) {
			tag := re.FindStringSubmatch(line)[1]
			reg := regexp.MustCompile(`\s+`)
			tag = reg.ReplaceAllString(tag, " ")
			result := strings.Fields(tag)
			if len(result) >= 2 {
				// 第一个方法大写
				result[0] = strings.ToUpper(result[0])
				return result
			}
		}
	}
	return nil
}

// toSnakeCase 驼峰转下划线
func toSnakeCase(s string) string {
	var result strings.Builder
	var prevChar rune

	for i, c := range s {
		if i > 0 && unicode.IsUpper(c) && prevChar != '_' {
			result.WriteRune('_')
		}
		result.WriteRune(unicode.ToLower(c))
		prevChar = c
	}

	return result.String()
}

// toSlashCase 驼峰转斜杠
func toSlashCase(s string) string {
	var result strings.Builder
	var prevChar rune

	for i, c := range s {
		if i > 0 && unicode.IsUpper(c) && prevChar != '/' {
			result.WriteRune('/')
		}
		result.WriteRune(unicode.ToLower(c))
		prevChar = c
	}

	return result.String()
}
