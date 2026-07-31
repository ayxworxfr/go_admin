package reqctx

// 业务码前缀规则：
// SUCCESS_*    : 成功类（100000-199999）
// CLIENT_*     : 客户端错误（200000-299999）
// SERVER_*     : 服务端错误（300000-399999）
// THIRD_PARTY_*: 第三方服务错误（400000-499999）
// SYSTEM_*     : 系统错误（500000-599999）

// 成功类
const (
	SUCCESS_OK         = 100000 // 操作成功
	SUCCESS_NO_CONTENT = 100001 // 成功但无返回内容
)

// 客户端错误类
const (
	CLIENT_PARAM_ERROR   = 200001 // 参数错误
	CLIENT_NOT_FOUND     = 200002 // 资源不存在
	CLIENT_UNAUTHORIZED  = 200003 // 未认证
	CLIENT_FORBIDDEN     = 200004 // 禁止访问
	CLIENT_CONFLICT      = 200005 // 资源冲突
	CLIENT_INVALID_TOKEN = 200007 // 无效令牌
	CLIENT_TOKEN_EXPIRED = 200008 // 令牌过期
)

// 服务端错误类
const (
	SERVER_INTERNAL_ERROR = 300001 // 服务端内部错误
	SERVER_DATABASE_ERROR = 300002 // 数据库操作失败
	SERVER_REDIS_ERROR    = 300003 // Redis 操作失败
	SERVER_RATE_LIMIT     = 300004 // 接口限流
	BUSINESS_ERROR        = 310000 // 业务错误
)

// 第三方 / 系统
const (
	THIRD_PARTY_ERROR = 400001 // 第三方服务错误
	SYSTEM_ERROR      = 500001 // 系统错误
)
