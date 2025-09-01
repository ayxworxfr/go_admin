package context

// 业务码前缀规则：
// SUCCESS_*    : 成功类（100000-199999）
// CLIENT_*     : 客户端错误（200000-299999）
// SERVER_*     : 服务端错误（300000-399999）
// THIRD_PARTY_*: 第三方服务错误（400000-499999）
// SYSTEM_*     : 系统错误（500000-599999）

// 成功类
const (
	SUCCESS_OK              = 100000 // 操作成功
	SUCCESS_NO_CONTENT      = 100001 // 成功但无返回内容
	SUCCESS_ACCEPTED        = 100002 // 请求已接受
	SUCCESS_PARTIAL_CONTENT = 100003 // 部分内容
)

// 客户端错误类
const (
	CLIENT_PARAM_ERROR       = 200001 // 参数错误
	CLIENT_NOT_FOUND         = 200002 // 资源不存在
	CLIENT_UNAUTHORIZED      = 200003 // 未认证
	CLIENT_FORBIDDEN         = 200004 // 禁止访问
	CLIENT_CONFLICT          = 200005 // 资源冲突
	CLIENT_TOO_MANY_REQUESTS = 200006 // 请求频率过高
	CLIENT_INVALID_TOKEN     = 200007 // 无效令牌
	CLIENT_TOKEN_EXPIRED     = 200008 // 令牌过期
	CLIENT_UNSUPPORTED_MEDIA = 200009 // 不支持的媒体类型
	CLIENT_VALIDATION_FAILED = 200010 // 数据验证失败
	CLIENT_MISSING_HEADER    = 200011 // 缺少必要请求头
	CLIENT_INVALID_FORMAT    = 200012 // 格式错误
)

// 服务端错误类
const (
	SERVER_INTERNAL_ERROR      = 300001 // 服务端内部错误
	SERVER_DATABASE_ERROR      = 300002 // 数据库操作失败
	SERVER_REDIS_ERROR         = 300003 // Redis 操作失败
	SERVER_RATE_LIMIT          = 300004 // 接口限流
	SERVER_SERVICE_UNAVAILABLE = 300005 // 服务不可用
	SERVER_TIMEOUT             = 300006 // 操作超时
	SERVER_CONFIG_ERROR        = 300007 // 配置错误
	SERVER_INIT_FAILED         = 300008 // 初始化失败
	BUSINESS_ERROR             = 310000 // 业务错误
)

// 第三方服务错误类
const (
	THIRD_PARTY_ERROR         = 400001 // 第三方服务错误
	THIRD_PARTY_PAYMENT_ERROR = 400002 // 支付服务错误
	THIRD_PARTY_SMS_ERROR     = 400003 // 短信服务错误
	THIRD_PARTY_EMAIL_ERROR   = 400004 // 邮件服务错误
	THIRD_PARTY_STORAGE_ERROR = 400005 // 存储服务错误
	THIRD_PARTY_API_ERROR     = 400006 // API调用错误
)

// 系统错误类
const (
	SYSTEM_ERROR              = 500001 // 系统错误
	SYSTEM_RESOURCE_EXHAUSTED = 500002 // 资源耗尽
	SYSTEM_FILE_NOT_FOUND     = 500003 // 文件不存在
	SYSTEM_PERMISSION_DENIED  = 500004 // 权限不足
)
