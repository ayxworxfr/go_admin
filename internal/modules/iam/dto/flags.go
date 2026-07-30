package dto

// ResponseFlags 响应内容控制标志（位标志），用于按需展开关联数据，
// 避免"角色列表要不要带权限"这类可选展开逻辑写成一堆 if 参数。
type ResponseFlags struct {
	flags int
}

// 预定义响应标志常量。原版本还定义了 INCLUDE_USER/INCLUDE_CUSTOMER/INCLUDE_CONTACT，
// 但全项目没有一处读取过这几个标志——那是从别的项目抄过来的死代码，本次一并清理，
// 只保留 iam 模块实际会判断的两个标志。
const (
	INCLUDE_ROLE       = 1 << iota // 包含角色信息
	INCLUDE_PERMISSION             // 包含权限信息
)

// ALL_AUTH_FLAGS 角色+权限的默认展开组合
const ALL_AUTH_FLAGS = INCLUDE_ROLE | INCLUDE_PERMISSION

// NewResponseFlags 创建响应标志实例
func NewResponseFlags(initialFlags ...int) *ResponseFlags {
	flags := 0
	for _, flag := range initialFlags {
		flags |= flag
	}
	return &ResponseFlags{flags: flags}
}

// Has 检查是否包含指定标志
func (f *ResponseFlags) Has(flag int) bool {
	return f.flags&flag != 0
}
