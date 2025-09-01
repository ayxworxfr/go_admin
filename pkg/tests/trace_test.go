package tests

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	// 配置环境变量
	os.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4318")

	// 打印默认配置
	// cfg := GetConfig()
	// jsonStr, _ := json.Marshal(cfg)
	// fmt.Println("Default Config:", string(jsonStr))
	// fmt.Printf("OTLP Endpoint: %s, Service Name: %s\n", cfg.OTLPEndpoint, cfg.ServiceName)

	// 运行测试
	code := m.Run()

	// 退出测试
	os.Exit(code)
}

func TestJaegerIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Jaeger integration test in short mode")
	}

	// 从环境变量获取配置，没有则使用默认值
	config := GetConfig()

	// 执行测试
	TestTracingIntegration(t, config)
}

// 可选：添加更多测试用例
func TestJaegerIntegrationWithCustomSpans(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping custom span test in short mode")
	}

	config := GetConfig()
	config.TestSpanCount = 10 // 增加测试 span 数量
	config.TestEventCount = 5 // 增加每个 span 的事件数量

	TestTracingIntegration(t, config)
}

func TestOTLPHTTPRequest(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping OTLP HTTP request test in short mode")
	}

	t.Parallel()

	OTLPEndpoint := "localhost:4318"
	// OTLPEndpoint = "localhost:43180"
	ServiceName := "ut-test-service2"

	fmt.Printf("OTLP Endpoint: %s, Service Name: %s\n", OTLPEndpoint, ServiceName)
	// 生成合法的 traceId 和 spanId（32/16 位十六进制）
	traceID := generateHexID(16) // 32 字符
	spanID := generateHexID(8)   // 16 字符

	// 时间戳（纳秒）
	start := time.Now().UnixNano()
	end := start + 1*time.Second.Nanoseconds()

	// 构建最小合法的 OTLP JSON payload
	payload, err := json.Marshal(map[string]any{
		"resourceSpans": []map[string]any{
			{
				"resource": map[string]any{
					"attributes": []map[string]any{
						{
							"key": "service.name",
							"value": map[string]string{
								"stringValue": ServiceName, // 服务名（必填）
							},
						},
					},
				},
				"scopeSpans": []map[string]any{
					{
						"spans": []map[string]any{
							{
								"name":              "ut-span",         // Span 名称
								"traceId":           traceID,           // 合法 Trace ID
								"spanId":            spanID,            // 合法 Span ID
								"startTimeUnixNano": fmt.Sprint(start), // 开始时间
								"endTimeUnixNano":   fmt.Sprint(end),   // 结束时间（需 > 开始时间）
							},
						},
					},
				},
			},
		},
	})
	assert.NoError(t, err, "Failed to marshal JSON")

	// 发送 POST 请求到 Jaeger 的 OTLP HTTP 端口
	url := fmt.Sprintf("http://%s/v1/traces", OTLPEndpoint)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payload))
	assert.NoError(t, err, "Failed to create request")
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	assert.NoError(t, err, "Failed to send request")
	defer resp.Body.Close()

	// 验证 HTTP 响应状态码为 200 OK
	assert.Equal(t, http.StatusOK, resp.StatusCode, "Expected 200 OK response")

	// 可选：打印响应内容（用于调试）
	body, _ := io.ReadAll(resp.Body)
	t.Logf("Response: %s", body)
}

// generateHexID 生成指定字节数的十六进制字符串（用于 traceId/spanId）
func generateHexID(bytes int) string {
	// 使用 crypto/rand 生成真正的随机数
	buf := make([]byte, bytes)
	if _, err := rand.Read(buf); err != nil {
		panic(err) // 测试环境中简单处理，生产环境应更好地处理错误
	}
	return hex.EncodeToString(buf)
}
