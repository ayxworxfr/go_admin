package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/ayxworxfr/go_admin/pkg/logger"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

func LogMiddleware() app.HandlerFunc {
	middleware := NewLogger()
	return middleware.Logger()
}

// LoggerConfig 配置结构体，用于设置日志中间件参数
type LoggerConfig struct {
	MaxInlineSize   int    // 直接内联记录的最大字节数
	MaxTotalSize    int    // 最大解析大小
	MaxValueSize    int    // 单个值的最大大小
	MaxFields       int    // 最大字段数量
	MaxDepth        int    // 最大嵌套深度
	TruncatedSuffix string // 截断值的后缀标识
	SensitiveFields []string
}

// LoggerMiddleware 日志中间件结构体
type LoggerMiddleware struct {
	config LoggerConfig
}

// NewLogger 创建一个新的日志中间件实例
func NewLogger(config ...LoggerConfig) *LoggerMiddleware {
	// 设置默认配置
	cfg := LoggerConfig{
		MaxInlineSize:   1024 * 100,
		MaxTotalSize:    1024 * 1024,
		MaxValueSize:    1024 * 32,
		MaxFields:       1000,
		MaxDepth:        64,
		TruncatedSuffix: "[TRUNCATED]",
		SensitiveFields: []string{"password", "token", "secret", "credit_card", "ssn"},
	}

	// 如果提供了配置，则覆盖默认值
	if len(config) > 0 {
		userCfg := config[0]
		if userCfg.MaxInlineSize > 0 {
			cfg.MaxInlineSize = userCfg.MaxInlineSize
		}
		if userCfg.MaxTotalSize > 0 {
			cfg.MaxTotalSize = userCfg.MaxTotalSize
		}
		if userCfg.MaxValueSize > 0 {
			cfg.MaxValueSize = userCfg.MaxValueSize
		}
		if userCfg.MaxFields > 0 {
			cfg.MaxFields = userCfg.MaxFields
		}
		if userCfg.MaxDepth > 0 {
			cfg.MaxDepth = userCfg.MaxDepth
		}
		if userCfg.TruncatedSuffix != "" {
			cfg.TruncatedSuffix = userCfg.TruncatedSuffix
		}
		if len(userCfg.SensitiveFields) > 0 {
			cfg.SensitiveFields = userCfg.SensitiveFields
		}
	}

	return &LoggerMiddleware{config: cfg}
}

// Logger 实现中间件接口，返回Hertz处理函数
func (l *LoggerMiddleware) Logger() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		start := time.Now()
		path := string(c.Request.URI().Path())
		method := string(c.Request.Method())

		// 获取当前span
		span := trace.SpanFromContext(ctx)
		spanContext := span.SpanContext()

		// 提取并记录请求参数（根据需要调整，敏感参数可过滤）
		requestParams := l.extractRequestParams(c)
		if len(requestParams) > 0 {
			span.SetAttributes(attribute.String("http.request.params", fmt.Sprintf("%v", requestParams)))
		}

		// 基础日志字段
		logFields := []zap.Field{
			zap.String("trace_id", spanContext.TraceID().String()),
			zap.String("span_id", spanContext.SpanID().String()),
			zap.String("method", method),
			zap.String("path", path),
			zap.String("client_ip", l.getClientIP(c)),
			zap.String("user_agent", l.getUserAgent(c)),
			zap.Int64("request_size_bytes", l.getRequestSize(c)),
		}
		requestFields := append(logFields, zap.Any("request_body", requestParams))
		// 记录请求开始
		logger.Info(ctx, "Request started", requestFields...)

		// 创建管道(有彩蛋)
		pr, pw := io.Pipe()
		buf := &bytes.Buffer{}

		// 启动goroutine读取管道数据
		go func() {
			// 延迟关闭pr（确保读取完成）
			defer pr.Close()
			_, err := io.Copy(buf, pr)
			if err != nil && err != io.ErrClosedPipe {
				logger.Error(ctx, "Failed to copy response body", zap.Error(err))
			}
		}()
		// 设置Hertz将响应体写入pr
		c.Response.SetBodyStream(pr, -1)

		// 处理请求
		c.Next(ctx)
		// 请求处理完成后关闭pw（触发管道关闭）
		pw.Close()

		end := time.Now()
		latency := end.Sub(start)
		statusCode := c.Response.StatusCode()

		// 响应体记录
		rsp := l.parseResponse(c)
		// 更新日志字段
		logFields = append(logFields,
			zap.Int("status", statusCode),
			zap.Duration("latency", latency),
			zap.Int64("response_size_bytes", l.getResponseSize(c)),
			zap.String("response_body", rsp),
		)

		// 设置span状态
		if statusCode >= consts.StatusBadRequest {
			span.SetStatus(codes.Error, http.StatusText(statusCode))
			logger.Warn(ctx, "Request completed with error", logFields...)
		} else {
			span.SetStatus(codes.Ok, "")
			logger.Info(ctx, "Request completed successfully", logFields...)
		}

		span.SetAttributes(attribute.String("http.response.body", rsp))
		// 添加详细的span事件
		span.AddEvent("request_completed", trace.WithAttributes(
			attribute.Int("http.status_code", statusCode),
			attribute.Int64("http.latency_ms", latency.Milliseconds()),
			attribute.Int64("http.request.size", l.getRequestSize(c)),
			attribute.Int64("http.response.size", l.getResponseSize(c)),
			attribute.String("http.client_ip", l.getClientIP(c)),
			attribute.String("http.user_agent", l.getUserAgent(c)),
		))
	}
}

// 从请求头获取客户端IP
func (l *LoggerMiddleware) getClientIP(c *app.RequestContext) string {
	// 尝试从常见的代理头获取客户端IP
	if xff := c.Request.Header.Get("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	// 尝试从X-Real-IP获取
	if xri := c.Request.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// 从RemoteAddr解析
	remoteAddr := c.RemoteAddr().String()
	if host, _, err := net.SplitHostPort(remoteAddr); err == nil {
		return host
	}
	return remoteAddr
}

// 获取请求大小（字节）
func (l *LoggerMiddleware) getRequestSize(c *app.RequestContext) int64 {
	if c.Request.Body() == nil {
		return 0
	}
	return int64(len(c.Request.Body()))
}

// 获取响应大小（字节）
func (l *LoggerMiddleware) getResponseSize(c *app.RequestContext) int64 {
	if c.Response.Body() == nil {
		return 0
	}
	return int64(len(c.Response.Body()))
}

// 获取用户代理
func (l *LoggerMiddleware) getUserAgent(c *app.RequestContext) string {
	return string(c.Request.Header.UserAgent())
}

func (l *LoggerMiddleware) parseResponse(c *app.RequestContext) string {
	if c.Response.Body() == nil {
		return ""
	}

	if len(c.Response.Body()) <= l.config.MaxInlineSize {
		return string(c.Response.Body())
	}

	// 如果响应体较大，截断并记录
	return string(c.Response.Body()[:l.config.MaxInlineSize]) + l.config.TruncatedSuffix
}

// 提取请求参数（支持大对象截断）
func (l *LoggerMiddleware) extractRequestParams(c *app.RequestContext) map[string]string {
	params := make(map[string]string)

	// 敏感字段检查函数
	isSensitiveField := func(key string) bool {
		for _, field := range l.config.SensitiveFields {
			if strings.Contains(strings.ToLower(key), field) {
				return true
			}
		}
		return false
	}

	// 检查并设置参数（支持截断）
	checkAndSetParam := func(key string, value []byte, truncated bool) {
		if _, ok := params[key]; !ok {
			if isSensitiveField(key) {
				params[key] = "****"
				return
			}

			if truncated {
				params[key] = string(value) + l.config.TruncatedSuffix
			} else {
				params[key] = string(value)
			}
		}
	}

	// 1. 获取查询参数
	c.QueryArgs().VisitAll(func(key, value []byte) {
		keyStr := string(key)
		checkAndSetParam(keyStr, value, false)
	})

	contentType := string(c.Request.Header.Get("Content-Type"))

	// 2. 处理JSON请求体
	if strings.Contains(contentType, "application/json") && len(c.Request.Body()) > 0 {
		body := c.Request.Body()

		// 小型请求体直接解析
		if len(body) <= l.config.MaxInlineSize {
			var jsonData map[string]any
			if err := json.Unmarshal(body, &jsonData); err != nil {
				hlog.Warnf("Failed to parse small JSON body: %v", err)
				params["_json_parse_error"] = err.Error()
			} else {
				for k, v := range jsonData {
					// 将值转换为字符串并处理截断
					var valStr string
					var truncated bool

					switch v := v.(type) {
					case string:
						if len(v) > l.config.MaxValueSize {
							valStr = v[:l.config.MaxValueSize]
							truncated = true
						} else {
							valStr = v
						}
					default:
						// 非字符串类型使用默认格式
						valStr = fmt.Sprintf("%v", v)
					}

					checkAndSetParam(k, []byte(valStr), truncated)
				}
			}
			return params
		}

		// 大型请求体使用流式解析
		bodyReader := bytes.NewReader(body)
		limitedReader := io.LimitReader(bodyReader, int64(l.config.MaxTotalSize+1))
		decoder := json.NewDecoder(limitedReader)
		decoder.UseNumber() // 避免大数字精度丢失

		// 验证是否为对象
		token, err := decoder.Token()
		if err != nil {
			hlog.Warnf("Failed to parse JSON token: %v", err)
			params["_json_parse_error"] = err.Error()
			return params
		}

		if delim, ok := token.(json.Delim); !ok || delim != '{' {
			hlog.Warnf("JSON is not an object (got %T: %v)", token, token)
			params["_json_not_object"] = fmt.Sprintf("%v", token)
			return params
		}

		// 逐字段解析（使用栈跟踪嵌套深度）
		fieldCount := 0
		depth := 0

		for decoder.More() {
			if depth > l.config.MaxDepth {
				hlog.Warnf("JSON nesting too deep: %d (max %d)", depth, l.config.MaxDepth)
				params["_nesting_too_deep"] = fmt.Sprintf("%d", depth)
				break
			}

			// 读取键
			keyToken, err := decoder.Token()
			if err != nil {
				hlog.Warnf("Failed to read JSON key: %v", err)
				break
			}

			keyStr, ok := keyToken.(string)
			if !ok {
				hlog.Warnf("JSON key is not a string: %T", keyToken)
				continue
			}

			// 增加字段计数
			fieldCount++
			if fieldCount > l.config.MaxFields {
				hlog.Warnf("JSON has too many fields: %d (max %d)", fieldCount, l.config.MaxFields)
				params["_too_many_fields"] = fmt.Sprintf("%d", fieldCount)
				break
			}

			// 处理值（支持嵌套结构）
			var valueBuf bytes.Buffer
			encoder := json.NewEncoder(&valueBuf)
			encoder.SetEscapeHTML(false) // 保留原始特殊字符

			// 跟踪嵌套结构
			if err := l.decodeValueWithTruncation(decoder, &valueBuf, l.config.MaxValueSize, &depth); err != nil {
				hlog.Warnf("Failed to decode value for key %s: %v", keyStr, err)
				params[keyStr] = "[DECODE_ERROR]"
				continue
			}

			// 检查是否超过最大大小
			if bodyReader.Len() > l.config.MaxTotalSize {
				hlog.Warnf("JSON body exceeded max size: %d bytes", l.config.MaxTotalSize)
				params["_body_too_large"] = fmt.Sprintf("%d bytes", l.config.MaxTotalSize)
				break
			}

			// 检查值是否被截断
			truncated := valueBuf.Len() >= l.config.MaxValueSize
			checkAndSetParam(keyStr, valueBuf.Bytes(), truncated)
		}

		// 检查是否有未读取的尾部数据
		if _, err := decoder.Token(); err != io.EOF {
			hlog.Warnf("JSON parsing incomplete: %v", err)
			params["_json_incomplete"] = "true"
		}
	}

	return params
}

// 递归解码JSON值并在超过大小时截断
func (l *LoggerMiddleware) decodeValueWithTruncation(decoder *json.Decoder, buf *bytes.Buffer, maxSize int, depth *int) error {
	// 增加深度计数
	(*depth)++
	defer func() { (*depth)-- }() // 函数结束时减少深度

	token, err := decoder.Token()
	if err != nil {
		return err
	}

	// 处理不同类型的token
	switch token := token.(type) {
	case json.Delim:
		// 对象或数组开始
		if token == '{' {
			buf.WriteByte('{')
			first := true

			for decoder.More() {
				// 检查是否超过最大大小
				if buf.Len() >= maxSize {
					buf.WriteString("...}")
					return nil
				}

				if !first {
					buf.WriteByte(',')
				}
				first = false

				// 写入键
				key, err := decoder.Token()
				if err != nil {
					return err
				}

				// 键需要加引号
				json.NewEncoder(buf).Encode(key)
				buf.WriteByte(':')

				// 递归处理值
				if err := l.decodeValueWithTruncation(decoder, buf, maxSize, depth); err != nil {
					return err
				}
			}

			// 读取结束符 '}'
			if _, err := decoder.Token(); err != nil {
				return err
			}
			buf.WriteByte('}')

		} else if token == '[' {
			buf.WriteByte('[')
			first := true

			for decoder.More() {
				// 检查是否超过最大大小
				if buf.Len() >= maxSize {
					buf.WriteString("...]")
					return nil
				}

				if !first {
					buf.WriteByte(',')
				}
				first = false

				// 递归处理值
				if err := l.decodeValueWithTruncation(decoder, buf, maxSize, depth); err != nil {
					return err
				}
			}

			// 读取结束符 ']'
			if _, err := decoder.Token(); err != nil {
				return err
			}
			buf.WriteByte(']')
		}

	case string:
		// 字符串值需要加引号
		json.NewEncoder(buf).Encode(token)

	default:
		// 其他类型（数字、布尔、null）直接写入
		buf.WriteString(fmt.Sprintf("%v", token))
	}

	return nil
}
