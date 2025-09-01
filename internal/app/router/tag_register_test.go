package router

import (
	"fmt"
	"net/http"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/ayxworxfr/go_admin/pkg/context"
	_ "github.com/ayxworxfr/go_admin/pkg/tests"

	"github.com/stretchr/testify/assert"
)

// 模拟的Handler结构体
type MockHandler struct{}

// @route POST  /login
func (h *MockHandler) Login(ctx *context.Context) {}

// @route GET /refresh
func (h *MockHandler) RefreshToken(ctx *context.Context) {}

// 没有标签的方法，不应被注册
func (h *MockHandler) InternalMethod(ctx *context.Context) {}

// 测试从方法注释提取路由标签
func TestExtractRouteTag(t *testing.T) {
	tests := []struct {
		name     string
		comment  string
		expected string
	}{
		{
			name:     "POST /login",
			comment:  "// @route POST /login",
			expected: "POST /login",
		},
		{
			name:     "GET /refresh",
			comment:  "// @route GET /refresh",
			expected: "GET /refresh",
		},
		{
			name:     "普通注释",
			comment:  "这是普通注释",
			expected: "",
		},
		{
			name:     "行首@route",
			comment:  "@route POST /logout",
			expected: "POST /logout",
		},
		{
			name:     "格式不正确",
			comment:  "// @routePOST/logout",
			expected: "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tags := extractRouteTag(test.comment)
			tag := strings.Join(tags, " ")
			assert.Equal(t, test.expected, tag, "标签提取不正确")
		})
	}
}

// 测试标签解析和路由注册
func TestTagRouterRegister_ParseMethodRoute(t *testing.T) {
	register := NewTagRouterRegister()
	handler := &MockHandler{}

	// 使用反射获取方法
	tp := reflect.TypeOf(handler)
	methodLogin, _ := tp.MethodByName("Login")
	methodRefresh, _ := tp.MethodByName("RefreshToken")

	tests := []struct {
		methodName string
		method     reflect.Method
		expected   *Router
	}{
		{
			"Login",
			methodLogin,
			&Router{path: "/login", method: POST, handlerFunc: handler.Login},
		},
		{
			"RefreshToken",
			methodRefresh,
			&Router{path: "/refresh", method: GET, handlerFunc: handler.RefreshToken},
		},
	}

	for _, test := range tests {
		t.Run(test.methodName, func(t *testing.T) {
			router := register.parseMethodRoute(reflect.ValueOf(handler), test.method)
			assert.NotNil(t, router, "路由解析失败")
			assert.Equal(t, test.expected.path, router.path, "路径不匹配")
			assert.Equal(t, test.expected.method, router.method, "方法不匹配")
		})
	}
}

// 测试从方法注释获取路由标签
func TestGetRouteTag(t *testing.T) {
	// 使用反射获取方法
	handler := &MockHandler{}
	tp := reflect.TypeOf(handler)

	tests := []struct {
		methodName string
		expected   string
	}{
		{"Login", "POST /login"},
		{"RefreshToken", "GET /refresh"},
		{"InternalMethod", ""}, // 没有标签
	}

	for _, test := range tests {
		t.Run(test.methodName, func(t *testing.T) {
			method, ok := tp.MethodByName(test.methodName)
			assert.True(t, ok, "方法 %s 不存在", test.methodName)

			// 注意：getRouteTag需要访问源代码，这里使用模拟实现
			// 实际测试中可能需要提供一个测试文件或重构getRouteTag以接受参数
			tags := getRouteTag(method)
			tag := strings.Join(tags, " ")
			assert.Equal(t, test.expected, tag, "标签不匹配")
		})
	}
}

// 测试TagRouterRegister的RegisterStruct方法
func TestTagRouterRegister_RegisterStruct(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	mockGroup := NewRouterGroup(nil)
	register := NewTagRouterRegister()
	// handler := &auth_handler.LoginHandler{}
	handler := &MockHandler{}
	// 注册结构体
	register.RegisterStruct(mockGroup, handler)

	// 验证路由注册结果
	tests := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/login"},
		{http.MethodGet, "/refresh"},
	}

	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			router, _ := mockGroup.FindRouter(test.method, test.path)
			assert.NotNil(t, router, "路径 %s 未被注册", test.path)
		})
	}

	// 验证未标记的方法未被注册
	_, ok := mockGroup.FindRouter(http.MethodPost, "/internal_method")
	assert.False(t, ok, "未标记的方法被错误注册")
}

func TestTagRouterRegister_RegisterRouterByFunc(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	mockGroup := NewRouterGroup(nil)
	handler := &MockHandler{}
	// 获取方法表达式的反射值
	methodValue := reflect.ValueOf(handler.Login)

	// 获取方法类型
	methodType := methodValue.Type()

	// 获取方法名称
	methodName := runtime.FuncForPC(methodValue.Pointer()).Name()

	fmt.Printf("方法类型: %v\n", methodType)
	fmt.Printf("方法名称: %s\n", methodName) // 输出: main.(*MockHandler).Login
	register := NewTagRouterRegister()
	// 传递结构体获取tag，然后再传递函数
	register.RegisterRouterByFunc(mockGroup, handler, handler.Login, handler.RefreshToken)

	// 验证路由注册结果
	tests := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/login"},
		{http.MethodGet, "/refresh"},
	}

	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			router, _ := mockGroup.FindRouter(test.method, test.path)
			assert.Equal(t, test.path, router.GetPath(), "路径 %s 未被注册", test.path)
		})
	}
}

func TestTagRouterRegister_RegisterByPackage(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	mockGroup := NewRouterGroup(nil)
	register := TagRegister
	handlerPath := "internal/handler/auth"
	// 注册结构体
	register.RegisterByPackage(mockGroup, handlerPath)

	// 验证路由注册结果
	tests := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/login"},
		{http.MethodPost, "/refresh"},
	}

	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			router, _ := mockGroup.FindRouter(test.method, test.path)
			assert.NotNil(t, router, "路径 %s 未被注册", test.path)
		})
	}
}
