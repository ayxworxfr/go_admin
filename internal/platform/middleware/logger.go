package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/ayxworxfr/go_admin/pkg/logger"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

const (
	defaultMaxBodyLogBytes = 2 << 10 // 2KiB：日志里够排查，不会拖垮吞吐
	defaultMaxJSONDepth    = 8
	defaultMaxFields       = 64 // 单个 query/body 最多保留的字段数，防止超大表单刷爆日志
	truncatedSuffix        = "...[truncated]"
)

// LogMiddleware 返回默认配置的访问日志中间件。
func LogMiddleware() app.HandlerFunc {
	return NewLogger().Handle()
}

// LoggerConfig 访问日志中间件参数。零值字段由 NewLogger 填默认值。
type LoggerConfig struct {
	// MaxBodyLogBytes 请求/响应体写入日志的最大字节数（截断前）
	MaxBodyLogBytes int
	// SensitiveFields 命中（子串、忽略大小写）的 JSON/表单字段名将被脱敏
	SensitiveFields []string
	// SkipPaths 完全跳过访问日志的路径（如健康检查）
	SkipPaths []string
}

// LoggerMiddleware 访问日志中间件。
type LoggerMiddleware struct {
	config          LoggerConfig
	sensitiveSubstr []string
	skipPaths       map[string]struct{}
}

// loggedRequest 结构化请求快照，写入访问日志的 request 字段。
type loggedRequest struct {
	Query map[string]string `json:"query,omitempty"`
	Body  any               `json:"body,omitempty"`
}

// NewLogger 创建日志中间件；未提供配置时使用生产向默认值。
func NewLogger(config ...LoggerConfig) *LoggerMiddleware {
	cfg := LoggerConfig{
		MaxBodyLogBytes: defaultMaxBodyLogBytes,
		SensitiveFields: []string{"password", "token", "secret", "authorization", "credit_card", "ssn"},
		SkipPaths:       []string{"/health", "/metrics"},
	}
	if len(config) > 0 {
		user := config[0]
		if user.MaxBodyLogBytes > 0 {
			cfg.MaxBodyLogBytes = user.MaxBodyLogBytes
		}
		if len(user.SensitiveFields) > 0 {
			cfg.SensitiveFields = user.SensitiveFields
		}
		if user.SkipPaths != nil {
			cfg.SkipPaths = user.SkipPaths
		}
	}

	skip := make(map[string]struct{}, len(cfg.SkipPaths))
	for _, p := range cfg.SkipPaths {
		skip[p] = struct{}{}
	}
	sensitive := make([]string, len(cfg.SensitiveFields))
	for i, f := range cfg.SensitiveFields {
		sensitive[i] = strings.ToLower(f)
	}

	return &LoggerMiddleware{
		config:          cfg,
		sensitiveSubstr: sensitive,
		skipPaths:       skip,
	}
}

// Handle 记录单次访问日志：Next 之前抓取请求参数，之后写一行（含 query/body）。
func (l *LoggerMiddleware) Handle() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		path := string(c.Request.URI().PathOriginal())
		if _, skip := l.skipPaths[path]; skip {
			c.Next(ctx)
			return
		}

		start := time.Now()
		method := string(c.Request.Method())
		// 必须在 Next 之前抓取：后续中间件/handler 可能消费或改写 body 视图
		reqSnapshot := l.captureRequest(c)

		c.Next(ctx)

		latency := time.Since(start)
		statusCode := c.Response.StatusCode()
		span := trace.SpanFromContext(ctx)
		spanCtx := span.SpanContext()

		fields := []zap.Field{
			zap.String("trace_id", spanCtx.TraceID().String()),
			zap.String("span_id", spanCtx.SpanID().String()),
			zap.String("method", method),
			zap.String("path", path),
			zap.Int("status", statusCode),
			zap.Duration("latency", latency),
			zap.String("client_ip", clientIP(c)),
			zap.String("user_agent", string(c.Request.Header.UserAgent())),
			zap.Int("request_size", len(c.Request.Body())),
			zap.Int("response_size", len(c.Response.Body())),
		}
		if reqSnapshot != nil {
			fields = append(fields, zap.Any("request", reqSnapshot))
		}
		// 错误响应才带 body，避免成功路径把大段 JSON 打进日志
		if statusCode >= consts.StatusBadRequest {
			if rsp := l.truncateBytes(c.Response.Body()); rsp != "" {
				fields = append(fields, zap.String("response_body", rsp))
			}
		}

		if statusCode >= consts.StatusBadRequest {
			span.SetStatus(codes.Error, http.StatusText(statusCode))
			logger.Warn(ctx, "HTTP request", fields...)
		} else {
			span.SetStatus(codes.Ok, "")
			logger.Info(ctx, "HTTP request", fields...)
		}

		span.SetAttributes(
			attribute.Int("http.status_code", statusCode),
			attribute.Int64("http.latency_ms", latency.Milliseconds()),
			attribute.Int("http.request_size", len(c.Request.Body())),
			attribute.Int("http.response_size", len(c.Response.Body())),
		)
	}
}

// captureRequest 提取可观测的请求快照：query + body（已脱敏/截断）。
func (l *LoggerMiddleware) captureRequest(c *app.RequestContext) *loggedRequest {
	query := l.captureQuery(c)
	body := l.captureBody(c)
	if query == nil && body == nil {
		return nil
	}
	return &loggedRequest{Query: query, Body: body}
}

func (l *LoggerMiddleware) captureQuery(c *app.RequestContext) map[string]string {
	if c.QueryArgs().Len() == 0 {
		return nil
	}
	out := make(map[string]string)
	count := 0
	c.QueryArgs().VisitAll(func(key, value []byte) {
		if count >= defaultMaxFields {
			return
		}
		k := string(key)
		if l.isSensitive(k) {
			out[k] = "****"
		} else {
			out[k] = l.truncateString(string(value))
		}
		count++
	})
	if len(out) == 0 {
		return nil
	}
	return out
}

func (l *LoggerMiddleware) captureBody(c *app.RequestContext) any {
	body := c.Request.Body()
	if len(body) == 0 {
		return nil
	}

	contentType := string(c.Request.Header.ContentType())
	switch {
	case strings.Contains(contentType, "application/json"):
		return l.captureJSONBody(body)
	case strings.Contains(contentType, "application/x-www-form-urlencoded"):
		return l.captureFormBody(body)
	default:
		// 无 Content-Type 时也尝试按 JSON 解析——常见客户端漏带头
		if trimmed := bytes.TrimSpace(body); len(trimmed) > 0 && (trimmed[0] == '{' || trimmed[0] == '[') {
			if parsed := l.captureJSONBody(body); parsed != nil {
				return parsed
			}
		}
		return l.truncateBytes(body)
	}
}

func (l *LoggerMiddleware) captureJSONBody(body []byte) any {
	body = bytes.TrimSpace(body)
	if len(body) == 0 {
		return nil
	}

	var data any
	if err := json.Unmarshal(body, &data); err != nil {
		return l.truncateBytes(body)
	}

	redacted := l.redactValue(data, 0)
	if l.exceedsBudget(redacted) {
		raw, err := json.Marshal(redacted)
		if err != nil {
			return l.truncateBytes(body)
		}
		return l.truncateBytes(raw)
	}
	return redacted
}

func (l *LoggerMiddleware) captureFormBody(body []byte) map[string]string {
	raw := string(body)
	if raw == "" {
		return nil
	}
	out := make(map[string]string)
	pairs := strings.Split(raw, "&")
	for i, pair := range pairs {
		if i >= defaultMaxFields {
			out["_truncated_fields"] = truncatedSuffix
			break
		}
		key, val, ok := strings.Cut(pair, "=")
		if !ok {
			continue
		}
		if key == "" {
			continue
		}
		if l.isSensitive(key) {
			out[key] = "****"
			continue
		}
		out[key] = l.truncateString(val)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func (l *LoggerMiddleware) redactValue(v any, depth int) any {
	if depth > defaultMaxJSONDepth {
		return "[max_depth]"
	}
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		count := 0
		for k, val := range x {
			if count >= defaultMaxFields {
				out["_truncated_fields"] = truncatedSuffix
				break
			}
			if l.isSensitive(k) {
				out[k] = "****"
			} else {
				out[k] = l.redactValue(val, depth+1)
			}
			count++
		}
		return out
	case []any:
		limit := len(x)
		if limit > defaultMaxFields {
			limit = defaultMaxFields
		}
		out := make([]any, limit)
		for i := 0; i < limit; i++ {
			out[i] = l.redactValue(x[i], depth+1)
		}
		return out
	case string:
		return l.truncateString(x)
	default:
		return v
	}
}

func (l *LoggerMiddleware) isSensitive(key string) bool {
	lower := strings.ToLower(key)
	for _, s := range l.sensitiveSubstr {
		if strings.Contains(lower, s) {
			return true
		}
	}
	return false
}

func (l *LoggerMiddleware) exceedsBudget(v any) bool {
	raw, err := json.Marshal(v)
	if err != nil {
		return true
	}
	return len(raw) > l.config.MaxBodyLogBytes
}

func (l *LoggerMiddleware) truncateBytes(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	if len(b) <= l.config.MaxBodyLogBytes {
		return string(b)
	}
	return string(b[:l.config.MaxBodyLogBytes]) + truncatedSuffix
}

func (l *LoggerMiddleware) truncateString(s string) string {
	if len(s) <= l.config.MaxBodyLogBytes {
		return s
	}
	return s[:l.config.MaxBodyLogBytes] + truncatedSuffix
}

func clientIP(c *app.RequestContext) string {
	if xff := c.Request.Header.Get("X-Forwarded-For"); xff != "" {
		if i := strings.IndexByte(xff, ','); i >= 0 {
			return strings.TrimSpace(xff[:i])
		}
		return strings.TrimSpace(xff)
	}
	if xri := c.Request.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	remoteAddr := c.RemoteAddr().String()
	if host, _, err := net.SplitHostPort(remoteAddr); err == nil {
		return host
	}
	return remoteAddr
}
