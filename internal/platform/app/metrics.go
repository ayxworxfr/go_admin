package app

import (
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/adaptor"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// metricsHTTPHandler 将 Prometheus 默认 Registry 以标准 exposition 格式写出。
// 使用包级单例，避免每次请求重建 gatherer/handler。
var metricsHTTPHandler = adaptor.HertzHandler(promhttp.Handler())

// MetricsHandler 暴露 GET /metrics，供 Prometheus scrape。
// 返回 text/plain 指标文本，不是 JSON 探活——与 HealthHandler 职责不同。
func MetricsHandler() app.HandlerFunc {
	return metricsHTTPHandler
}
