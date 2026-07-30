package app

import (
	"github.com/ayxworxfr/go_admin/pkg/context"
)

// HelloHandler 健康检查/存活探针的最小实现，被 /health、/metrics、/api/hello
// 共用——这三个路径目前语义等价，都只是"进程活着"的探针，尚未接入真正的
// 健康检查逻辑（数据库连通性等），保留现状，不在本次模块化重构中扩展它。
func HelloHandler(c *context.Context) *context.Response {
	return context.Success("Hello, World!")
}
