package router

import (
	"context"
	"fmt"
	"log"
	"reflect"

	mycontext "github.com/ayxworxfr/go_admin/pkg/context"
	"github.com/ayxworxfr/go_admin/pkg/logger"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/cloudwego/hertz/pkg/route"
	"github.com/samber/lo"
)

type RouterGroup struct {
	group       *route.RouterGroup
	routers     []*Router
	middlewares []any
}

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

// Handle 是一个通用的方法，用于处理所有 HTTP 方法
func (rg *RouterGroup) Handle(method, path string, handler any) {
	rg.pushRouter(method, path, handler)
	handlers := append(rg.middlewares, handler)
	if rg.group == nil {
		log.Print("rg.group is nil")
		return
	}
	basePath := rg.group.BasePath()
	logger.Debug(context.Background(), fmt.Sprintf("register route: %s %s%s", method, basePath, path))
	rg.group.Handle(method, path, adapt(handlers...))
}

func (rg *RouterGroup) pushRouter(method, path string, handler any) {
	rg.routers = append(rg.routers, NewRouter(method, path, handler))
}

func (rg *RouterGroup) GetRouter() []*Router {
	return rg.routers
}

func (rg *RouterGroup) FindRouter(method, path string) (*Router, bool) {
	router := lo.Filter(rg.routers, func(r *Router, index int) bool {
		return r.GetMethod().Value() == method && r.GetPath() == path
	})
	if len(router) != 1 {
		return nil, false
	}
	return router[0], true
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

// adapt 函数用于适配不同类型的处理函数和中间件
// 如果handle有返回值，将返回值作为响应返回
func adapt(handlers ...any) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		myCtx := mycontext.NewContext(ctx, c)

		for _, handler := range handlers {
			handlerType := reflect.TypeOf(handler)
			handlerValue := reflect.ValueOf(handler)

			// 处理app.MiddlewareFunc类型的中间件
			if middlewareFunc, ok := handlerValue.Interface().(app.HandlerFunc); ok {
				middlewareFunc(ctx, c)
				if !myCtx.IsAborted() {
					continue // 如果没有被中止，继续下一个处理函数
				}
				return // 如果被中止，直接返回
			}

			switch handlerType.NumIn() {
			case 1:
				// 处理不需要请求参数的情况
				results := handlerValue.Call([]reflect.Value{reflect.ValueOf(myCtx)})
				if !handleResults(myCtx, results) {
					return
				}
			case 2:
				// 处理需要请求参数的情况
				paramType := handlerType.In(1)
				param := reflect.New(paramType.Elem()).Interface()

				if err := c.BindAndValidate(param); err != nil {
					myCtx.JSON(consts.StatusBadRequest, map[string]any{
						"error": err.Error(),
					})
					return
				}

				results := handlerValue.Call([]reflect.Value{reflect.ValueOf(myCtx), reflect.ValueOf(param)})
				if !handleResults(myCtx, results) {
					return
				}
			default:
				myCtx.String(consts.StatusInternalServerError, "Invalid handler function")
				return
			}
		}
	}
}

// handleResults 处理处理函数的返回值
func handleResults(c *mycontext.Context, results []reflect.Value) bool {
	// 判断是否*Response类型
	if len(results) > 0 && !results[0].IsNil() {
		if response, ok := results[0].Interface().(*mycontext.Response); ok {
			response.Write(c)
			return false
		}
		if err, ok := results[0].Interface().(error); ok {
			c.String(consts.StatusInternalServerError, err.Error())
			return false
		}
	}
	return true
}
