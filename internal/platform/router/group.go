package router

import (
	"context"
	"fmt"
	"reflect"

	"github.com/ayxworxfr/go_admin/pkg/logger"
	"github.com/ayxworxfr/go_admin/pkg/reqctx"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/cloudwego/hertz/pkg/route"
)

// RouterGroup 对 Hertz RouterGroup 的薄封装：挂中间件、注册路由、记录本 group 路由表。
type RouterGroup struct {
	group       *route.RouterGroup
	routers     []*Router
	middlewares []any
}

// NewRouterGroup 创建路由组
func NewRouterGroup(group *route.RouterGroup) *RouterGroup {
	return &RouterGroup{
		group:       group,
		routers:     make([]*Router, 0),
		middlewares: make([]any, 0),
	}
}

// Group 创建一个新的路由组
func (rg *RouterGroup) Group(path string) *RouterGroup {
	return &RouterGroup{
		group:       rg.group.Group(path),
		middlewares: append([]any{}, rg.middlewares...),
	}
}

// Use 添加中间件
func (rg *RouterGroup) Use(middleware ...any) {
	rg.middlewares = append(rg.middlewares, middleware...)
}

// Handle 是一个通用的方法，用于处理所有 HTTP 方法。
// 中间件与 handler 在注册时预编译，请求热路径不再做 TypeOf/签名分支。
func (rg *RouterGroup) Handle(method, path string, handler any) {
	rg.routers = append(rg.routers, NewRouter(method, path, handler))
	if rg.group == nil {
		return
	}
	logger.Debug(context.Background(), fmt.Sprintf("register route: %s %s%s", method, rg.group.BasePath(), path))
	rg.group.Handle(method, path, compileChain(rg.middlewares, handler))
}

// Routers 返回本 group 上注册的路由
func (rg *RouterGroup) Routers() []*Router {
	return rg.routers
}

// FindRouter 按 method+path 查找本 group 上的路由
func (rg *RouterGroup) FindRouter(method, path string) (*Router, bool) {
	for _, r := range rg.routers {
		if string(r.method) == method && r.path == path {
			return r, true
		}
	}
	return nil, false
}

func (rg *RouterGroup) GET(path string, handler any) {
	rg.Handle("GET", path, handler)
}

func (rg *RouterGroup) POST(path string, handler any) {
	rg.Handle("POST", path, handler)
}

func (rg *RouterGroup) PUT(path string, handler any) {
	rg.Handle("PUT", path, handler)
}

func (rg *RouterGroup) DELETE(path string, handler any) {
	rg.Handle("DELETE", path, handler)
}

// --- 请求适配：启动期预编译，热路径只跑闭包 ---

// step 是启动期预编译后的处理步骤。
// 返回 false 表示链路终止（已写响应或被 abort）。
type step func(stdCtx context.Context, c *app.RequestContext, myCtx *reqctx.Context) bool

var contextPtrType = reflect.TypeOf((*reqctx.Context)(nil))

func compileChain(middlewares []any, handler any) app.HandlerFunc {
	steps := make([]step, 0, len(middlewares)+1)
	for _, mw := range middlewares {
		steps = append(steps, compileStep(mw))
	}
	steps = append(steps, compileStep(handler))

	return func(stdCtx context.Context, c *app.RequestContext) {
		myCtx := reqctx.New(stdCtx, c)
		for _, s := range steps {
			if !s(stdCtx, c, myCtx) {
				return
			}
		}
	}
}

func compileStep(handler any) step {
	if handler == nil {
		return func(context.Context, *app.RequestContext, *reqctx.Context) bool { return true }
	}

	// 原生 Hertz HandlerFunc（JWT / Logger / Metrics 等）
	if hf, ok := handler.(app.HandlerFunc); ok {
		return func(stdCtx context.Context, c *app.RequestContext, myCtx *reqctx.Context) bool {
			hf(stdCtx, c)
			return !myCtx.IsAborted()
		}
	}

	v := reflect.ValueOf(handler)
	t := v.Type()
	if t.Kind() != reflect.Func {
		return func(context.Context, *app.RequestContext, *reqctx.Context) bool { return true }
	}

	switch t.NumIn() {
	case 1:
		if t.In(0) == contextPtrType {
			return compileCtxOnly(v, t)
		}
	case 2:
		if t.In(0) == contextPtrType && t.In(1).Kind() == reflect.Ptr {
			return compileCtxReq(v, t)
		}
	}

	return func(_ context.Context, _ *app.RequestContext, myCtx *reqctx.Context) bool {
		myCtx.String(consts.StatusInternalServerError, "Invalid handler function")
		return false
	}
}

func compileCtxOnly(v reflect.Value, t reflect.Type) step {
	hasOut := t.NumOut() > 0
	return func(_ context.Context, _ *app.RequestContext, myCtx *reqctx.Context) bool {
		outs := v.Call([]reflect.Value{reflect.ValueOf(myCtx)})
		if !hasOut {
			return true
		}
		return writeOutputs(myCtx, outs)
	}
}

func compileCtxReq(v reflect.Value, t reflect.Type) step {
	reqType := t.In(1).Elem()
	hasOut := t.NumOut() > 0
	return func(_ context.Context, c *app.RequestContext, myCtx *reqctx.Context) bool {
		param := reflect.New(reqType)
		if err := c.BindAndValidate(param.Interface()); err != nil {
			reqctx.ParamError(err).Write(myCtx)
			return false
		}
		outs := v.Call([]reflect.Value{reflect.ValueOf(myCtx), param})
		if !hasOut {
			return true
		}
		return writeOutputs(myCtx, outs)
	}
}

// writeOutputs 处理 handler 返回值：*Response / error 终止链路，其它继续。
func writeOutputs(c *reqctx.Context, outs []reflect.Value) bool {
	if len(outs) == 0 {
		return true
	}
	out := outs[0]
	if (out.Kind() == reflect.Ptr || out.Kind() == reflect.Interface) && out.IsNil() {
		return true
	}
	switch val := out.Interface().(type) {
	case *reqctx.Response:
		val.Write(c)
		return false
	case error:
		c.String(consts.StatusInternalServerError, val.Error())
		return false
	default:
		return true
	}
}
