package handler

import (
	"fmt"
	"reflect"

	auth_handler "github.com/ayxworxfr/go_admin/internal/handler/auth"
)

var (
	AllHandlerInstance           []any
	LoginHandlerInstance         auth_handler.ILoginHandler
	UserHandlerInstance          auth_handler.IUserHandler
	RoleHandlerInstance          auth_handler.IRoleHandler
	PermissionHandlerInstance    auth_handler.IPermissionHandler
	UserRoleHandlerInstance      auth_handler.IUserRoleHandler
	SystemSettingHandlerInstance ISystemSettingHandler
)

func init() {
	LoginHandlerInstance = &auth_handler.LoginHandler{}

	// 下列实例使用包扫描自动注册路由
	// createAndRegister(&LoginHandlerInstance, &auth_handler.LoginHandler{})
	createAndRegister(&UserHandlerInstance, &auth_handler.UserHandler{})
	createAndRegister(&RoleHandlerInstance, &auth_handler.RoleHandler{})
	createAndRegister(&PermissionHandlerInstance, &auth_handler.PermissionHandler{})
	createAndRegister(&UserRoleHandlerInstance, &auth_handler.UserRoleHandler{})
	createAndRegister(&SystemSettingHandlerInstance, &SystemSettingHandler{})
}

func createAndRegister(addressPtr any, handler any) {
	// 获取addressPtr的反射值
	addressValue := reflect.ValueOf(addressPtr)

	// 确保addressPtr是指针
	if addressValue.Kind() != reflect.Ptr {
		panic("addressPtr must be a pointer")
	}

	// 获取指针指向的值
	addressElem := addressValue.Elem()

	// 确保可以设置值
	if !addressElem.CanSet() {
		panic("addressPtr value cannot be set")
	}

	// 验证handler类型是否可以赋值给addressElem
	handlerType := reflect.TypeOf(handler)
	if !handlerType.Implements(addressElem.Type()) {
		panic(fmt.Sprintf("handler type %v does not implement interface %v", handlerType, addressElem.Type()))
	}

	// 设置值
	addressElem.Set(reflect.ValueOf(handler))

	// 添加到AllHandlerInstance
	AllHandlerInstance = append(AllHandlerInstance, handler)
}
