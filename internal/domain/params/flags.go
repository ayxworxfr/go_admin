package params

import "errors"

// 响应内容控制标志
type ResponseFlags struct {
	flags int
}

// 预定义响应标志常量
const (
	INCLUDE_ROLE       = 1 << iota // 包含角色信息 (0001)
	INCLUDE_PERMISSION             // 包含权限信息 (0010)
	INCLUDE_DETAIL                 // 包含详细信息 (0100)
	INCLUDE_USER                   // 包含用户信息 (1000)
	INCLUDE_CUSTOMER               // 包含客户信息 (10000)
	INCLUDE_CONTACT                // 包含联系人信息 (100000)
)

// 所有权限相关标志的集合
const ALL_AUTH_FLAGS = INCLUDE_ROLE | INCLUDE_PERMISSION | INCLUDE_DETAIL

// 所有用户相关标志的集合
const ALL_USER_FLAGS = ALL_AUTH_FLAGS | INCLUDE_USER

// 所有客户相关标志的集合
const ALL_CUSTOMER_FLAGS = ALL_USER_FLAGS | INCLUDE_CUSTOMER | INCLUDE_CONTACT

// NewResponseFlags 创建响应标志实例
func NewResponseFlags(initialFlags ...int) *ResponseFlags {
	flags := 0
	for _, flag := range initialFlags {
		flags |= flag
	}
	return &ResponseFlags{flags: flags}
}

// Add 添加标志
func (f *ResponseFlags) Add(flag int) {
	f.flags |= flag
}

// Remove 移除标志
func (f *ResponseFlags) Remove(flag int) {
	f.flags &^= flag
}

// Has 检查是否包含指定标志
func (f *ResponseFlags) Has(flag int) bool {
	return f.flags&flag != 0
}

// Get 获取当前标志值
func (f *ResponseFlags) Get() int {
	return f.flags
}

// Set 设置标志值
func (f *ResponseFlags) Set(flags int) {
	f.flags = flags
}

// Validate 验证标志是否有效
func (f *ResponseFlags) Validate(allowedFlags int) error {
	if f.flags&^allowedFlags != 0 {
		return errors.New("invalid flags detected")
	}
	return nil
}
