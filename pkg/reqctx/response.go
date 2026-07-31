package reqctx

import (
	"fmt"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

// Response 统一 API 响应体。HTTP 状态由业务码映射，前端同时可读 code / HTTP status。
type Response struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

// HTTPStatus 按业务码区间映射标准 HTTP 状态，供 Write / Abort 共用。
func (rsp *Response) HTTPStatus() int {
	switch {
	case rsp.Code >= SUCCESS_OK && rsp.Code < 200000:
		return consts.StatusOK
	case rsp.Code == CLIENT_UNAUTHORIZED || rsp.Code == CLIENT_INVALID_TOKEN || rsp.Code == CLIENT_TOKEN_EXPIRED:
		return consts.StatusUnauthorized
	case rsp.Code == CLIENT_FORBIDDEN:
		return consts.StatusForbidden
	case rsp.Code == CLIENT_NOT_FOUND:
		return consts.StatusNotFound
	case rsp.Code == CLIENT_CONFLICT:
		return consts.StatusConflict
	case rsp.Code == SERVER_RATE_LIMIT:
		return consts.StatusTooManyRequests
	case rsp.Code >= 200000 && rsp.Code < 300000:
		return consts.StatusBadRequest
	case rsp.Code >= 300000:
		return consts.StatusInternalServerError
	default:
		return consts.StatusOK
	}
}

// Write 将响应写入 reqctx.Context（handler 返回路径）
func (rsp *Response) Write(ctx *Context) {
	ctx.JSON(rsp.HTTPStatus(), rsp)
}

// Abort 将响应写入原生 RequestContext 并中断链路（中间件路径）
func Abort(c *app.RequestContext, rsp *Response) {
	c.JSON(rsp.HTTPStatus(), rsp)
	c.Abort()
}

// --- 成功 ---

func Success(data any) *Response {
	return &Response{Code: SUCCESS_OK, Message: "Success", Data: data}
}

func PageSuccess(data any, total int64) *Response {
	return &Response{
		Code:    SUCCESS_OK,
		Message: "Success",
		Data: map[string]any{
			"records": data,
			"total":   total,
		},
	}
}

// NoContent 响应成功但无内容（如 DELETE 请求）
func NoContent() *Response {
	return &Response{Code: SUCCESS_NO_CONTENT, Message: "No content", Data: nil}
}

// --- 客户端错误 ---

func ParamError(message any) *Response {
	return &Response{Code: CLIENT_PARAM_ERROR, Message: formatMessage("Parameter error", message)}
}

func NotFound(message any) *Response {
	return &Response{Code: CLIENT_NOT_FOUND, Message: formatMessage("Resource not found", message)}
}

func Unauthorized(message any) *Response {
	return &Response{Code: CLIENT_UNAUTHORIZED, Message: formatMessage("Unauthorized", message)}
}

func Forbidden(message any) *Response {
	return &Response{Code: CLIENT_FORBIDDEN, Message: formatMessage("Forbidden", message)}
}

func Conflict(message any) *Response {
	return &Response{Code: CLIENT_CONFLICT, Message: formatMessage("Conflict", message)}
}

// --- 服务端 / 业务 ---

func InternalError(message ...any) *Response {
	return &Response{Code: SERVER_INTERNAL_ERROR, Message: formatOptionalMessage("Internal server error", message...)}
}

func BusinessError(message ...any) *Response {
	return &Response{Code: BUSINESS_ERROR, Message: formatOptionalMessage("Business error", message...)}
}

func RateLimit(message any) *Response {
	return &Response{Code: SERVER_RATE_LIMIT, Message: formatMessage("Rate limit", message)}
}

func DatabaseError(message any) *Response {
	return &Response{Code: SERVER_DATABASE_ERROR, Message: formatMessage("Database error", message)}
}

func ThirdPartyError(serviceName string, message any) *Response {
	return &Response{Code: THIRD_PARTY_ERROR, Message: formatServiceMessage(serviceName, "service error", message)}
}

func SystemError(message any) *Response {
	return &Response{Code: SYSTEM_ERROR, Message: formatMessage("System error", message)}
}

func formatMessage(prefix string, message any) string {
	switch v := message.(type) {
	case string:
		return fmt.Sprintf("%s: %s", prefix, v)
	case error:
		return fmt.Sprintf("%s: %s", prefix, v.Error())
	default:
		return prefix
	}
}

func formatServiceMessage(service, action string, message any) string {
	switch v := message.(type) {
	case string:
		return fmt.Sprintf("%s %s: %s", service, action, v)
	case error:
		return fmt.Sprintf("%s %s: %s", service, action, v.Error())
	default:
		return fmt.Sprintf("%s %s", service, action)
	}
}

func formatOptionalMessage(prefix string, message ...any) string {
	if len(message) == 0 {
		return prefix
	}
	if err, ok := message[0].(error); ok {
		return fmt.Sprintf("%s: %s", prefix, err.Error())
	}
	if str, ok := message[0].(string); ok {
		return fmt.Sprintf("%s: %s", prefix, str)
	}
	return prefix
}
