package cron

import (
	"context"
	"fmt"
	"time"

	"github.com/ayxworxfr/go_admin/internal/config"
	"github.com/ayxworxfr/go_admin/pkg/httpclient"
	"github.com/ayxworxfr/go_admin/pkg/logger"
)

var HttpClient *httpclient.Client

const (
	Timeout = 5 * time.Second        // 请求超时时间
	Retries = 2                      // 失败时重试次数
	Backoff = 200 * time.Millisecond // 退避时间
)

func init() {
	// 创建HTTP客户端
	client := httpclient.NewClient(
		"",
		httpclient.WithTimeout(Timeout),
		httpclient.WithRetries(Retries),
		httpclient.WithBackoff(Backoff),
	)
	HttpClient = client
}

// 健康检查任务
func healthCheck() {
	ctx := context.Background()
	baseUrl := fmt.Sprintf("http://localhost:%d", config.GetAppPort())
	logger.Info(ctx, "[TASK] Performing health check...")

	// 发送请求并解析响应
	url := baseUrl + "/health"
	rsp, err := HttpClient.Get(ctx, url, nil)
	if err != nil || rsp.StatusCode/100 != 2 {
		logger.Errorf(ctx, "[TASK] Health check failed: %v", err)
		return
	}

	logger.Info(ctx, "[TASK] Health check successful")
}
