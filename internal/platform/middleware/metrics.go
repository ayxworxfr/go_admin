package middleware

import (
	"context"
	"strconv"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	httpRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total number of HTTP requests processed.",
	}, []string{"method", "path", "status"})

	httpRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "HTTP request latency in seconds.",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "path", "status"})
)

// MetricsMiddleware 记录请求计数与耗时。path 使用路由模板（FullPath），
// 避免带 ID 的原始路径把标签基数打爆。
func MetricsMiddleware() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		start := time.Now()
		c.Next(ctx)

		path := c.FullPath()
		if path == "" {
			path = "unmatched"
		}
		// 探活与指标端点本身不计入业务流量
		if path == "/health" || path == "/metrics" {
			return
		}

		status := strconv.Itoa(c.Response.StatusCode())
		method := string(c.Request.Method())
		labels := prometheus.Labels{
			"method": method,
			"path":   path,
			"status": status,
		}
		httpRequestsTotal.With(labels).Inc()
		httpRequestDuration.With(labels).Observe(time.Since(start).Seconds())
	}
}
