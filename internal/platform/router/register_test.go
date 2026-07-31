package router

import (
	"net/http"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"github.com/ayxworxfr/go_admin/internal/platform/routegen"
	"github.com/ayxworxfr/go_admin/pkg/reqctx"
	_ "github.com/ayxworxfr/go_admin/pkg/tests"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testRouterPkg = "github.com/ayxworxfr/go_admin/internal/platform/router"

// MockHandler 模拟的 Handler；@route 仅作文档，测试路由由 init 写入编译表。
type MockHandler struct{}

// @route POST /login
func (h *MockHandler) Login(ctx *reqctx.Context) {}

// @route GET /refresh
func (h *MockHandler) RefreshToken(ctx *reqctx.Context) {}

// InternalMethod 无编译表条目，应按函数名推断注册。
func (h *MockHandler) InternalMethod(ctx *reqctx.Context) {}

func init() {
	registerCompiledRoute(testRouterPkg, "MockHandler", "Login", POST, "/login")
	registerCompiledRoute(testRouterPkg, "MockHandler", "RefreshToken", GET, "/refresh")
}

func TestLookupMethodTag(t *testing.T) {
	handler := &MockHandler{}
	tp := reflect.TypeOf(handler)

	tests := []struct {
		methodName string
		ok         bool
		method     Method
		path       string
	}{
		{"Login", true, POST, "/login"},
		{"RefreshToken", true, GET, "/refresh"},
		{"InternalMethod", false, "", ""},
	}

	for _, test := range tests {
		t.Run(test.methodName, func(t *testing.T) {
			method, ok := tp.MethodByName(test.methodName)
			assert.True(t, ok)
			tag, found := lookupMethodTag(method)
			assert.Equal(t, test.ok, found)
			if test.ok {
				assert.Equal(t, test.method, tag.method)
				assert.Equal(t, test.path, tag.path)
			}
		})
	}
}

func TestRegisterStruct(t *testing.T) {
	mockGroup := NewRouterGroup(nil)
	reg := NewRegister()
	reg.RegisterStruct(mockGroup, &MockHandler{})

	tests := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/login"},
		{http.MethodGet, "/refresh"},
	}

	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			rt, ok := mockGroup.FindRouter(test.method, test.path)
			assert.True(t, ok)
			assert.Equal(t, test.path, rt.Path())
		})
	}

	// InternalMethod 无编译表条目，按名推断为 POST /internal/method
	_, ok := mockGroup.FindRouter(http.MethodPost, "/internal/method")
	assert.True(t, ok, "无标签方法应按函数名推断注册")
}

func TestInferFromName(t *testing.T) {
	method, path := inferFromName("GetUserList", SlashCase)
	assert.Equal(t, GET, method)
	assert.Equal(t, "/user/list", path)

	method, path = inferFromName("CreateUser", SlashCase)
	assert.Equal(t, POST, method)
	assert.Equal(t, "/user", path)
}

func TestCompiledRoutesFresh(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))

	scanned, err := routegen.ScanModule(root)
	require.NoError(t, err)
	require.NotEmpty(t, scanned)

	for _, r := range scanned {
		tag, ok := compiledRoutes[r.Key()]
		if !ok {
			t.Errorf("编译表缺失 %s，请执行: go generate ./internal/platform/router/...", r.Key())
			continue
		}
		assert.Equal(t, Method(r.HTTPMethod), tag.method, r.Key())
		assert.Equal(t, r.Path, tag.path, r.Key())
	}
}
