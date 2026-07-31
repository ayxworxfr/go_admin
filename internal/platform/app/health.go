package app

import (
	"github.com/ayxworxfr/go_admin/pkg/reqctx"
)

// HealthHandler 存活探针（GET /health）。
// 目前只表示"进程活着"，尚未接入数据库连通性等深度检查。
// Prometheus 指标在 /metrics，由 MetricsHandler 单独暴露。
func HealthHandler(c *reqctx.Context) *reqctx.Response {
	return reqctx.Success("ok")
}
