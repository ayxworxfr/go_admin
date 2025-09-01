package middleware

import (
	"context"
	"reflect"

	mycontext "github.com/ayxworxfr/go_admin/pkg/context"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

func BindAndValidateMiddleware() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		// 获取处理函数
		handler := c.Handler()
		if handler == nil {
			c.Next(ctx)
			return
		}

		// 获取处理函数的类型
		handlerType := reflect.TypeOf(handler)
		if handlerType.Kind() != reflect.Func {
			c.Next(ctx)
			return
		}

		// 检查处理函数是否有参数
		if handlerType.NumIn() < 2 {
			c.Next(ctx)
			return
		}

		// 获取第二个参数的类型（第一个是 context）
		paramType := handlerType.In(1)
		if paramType.Kind() != reflect.Ptr || paramType.Elem().Kind() != reflect.Struct {
			c.Next(ctx)
			return
		}

		// 创建参数实例
		param := reflect.New(paramType.Elem()).Interface()

		// 绑定和验证
		if err := c.BindAndValidate(param); err != nil {
			rsp := mycontext.ParamError(err)
			c.JSON(consts.StatusBadRequest, rsp)
			c.Abort()
			return
		}

		// 将绑定的参数存储在上下文中
		c.Set("bindedRequest", param)

		c.Next(ctx)
	}
}
