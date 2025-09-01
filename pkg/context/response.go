package context

import (
	"fmt"

	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

type Response struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

func (rsp *Response) Write(ctx *Context) {
	// 正常请求，返回200
	ctx.JSON(consts.StatusOK, rsp)
}

// 成功响应函数
func Success(data any) *Response {
	return &Response{
		Code:    SUCCESS_OK,
		Message: "Success",
		Data:    data,
	}
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
	return &Response{
		Code:    SUCCESS_NO_CONTENT,
		Message: "No content",
		Data:    nil,
	}
}

// 客户端错误响应函数（支持string/error类型）
func ParamError(message any) *Response {
	return &Response{
		Code:    CLIENT_PARAM_ERROR,
		Message: formatMessage("Parameter error", message),
		Data:    nil,
	}
}

func NotFound(message any) *Response {
	return &Response{
		Code:    CLIENT_NOT_FOUND,
		Message: formatMessage("Resource not found", message),
		Data:    nil,
	}
}

func Unauthorized(message any) *Response {
	return &Response{
		Code:    CLIENT_UNAUTHORIZED,
		Message: formatMessage("Unauthorized", message),
		Data:    nil,
	}
}

// 服务端错误响应函数
func InternalError(message ...any) *Response {
	return &Response{
		Code:    SERVER_INTERNAL_ERROR,
		Message: formatOptionalMessage("Internal server error", message...),
		Data:    nil,
	}
}

func BusinessError(message ...any) *Response {
	return &Response{
		Code:    BUSINESS_ERROR,
		Message: formatOptionalMessage("Business error", message...),
		Data:    nil,
	}
}

// 接口限流响应函数
func RateLimit(message any) *Response {
	return &Response{
		Code:    SERVER_RATE_LIMIT,
		Message: formatMessage("Rate limit", message),
		Data:    nil,
	}
}

// Conflict 响应资源冲突错误（如重复创建）
func Conflict(message any) *Response {
	return &Response{
		Code:    CLIENT_CONFLICT,
		Message: formatMessage("Conflict", message),
		Data:    nil,
	}
}

func DatabaseError(message any) *Response {
	return &Response{
		Code:    SERVER_DATABASE_ERROR,
		Message: formatMessage("Database error", message),
		Data:    nil,
	}
}

// 第三方服务错误响应函数
func ThirdPartyError(serviceName string, message any) *Response {
	return &Response{
		Code:    THIRD_PARTY_ERROR,
		Message: formatServiceMessage(serviceName, "service error", message),
		Data:    nil,
	}
}

func PaymentError(message any) *Response {
	return &Response{
		Code:    THIRD_PARTY_PAYMENT_ERROR,
		Message: formatServiceMessage("Payment", "failed", message),
		Data:    nil,
	}
}

// 系统错误响应函数
func SystemError(message any) *Response {
	return &Response{
		Code:    SYSTEM_ERROR,
		Message: formatMessage("System error", message),
		Data:    nil,
	}
}

// 格式化消息（支持string/error类型）
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

// 格式化服务错误消息
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

// 格式化可选消息（用于支持变参）
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
