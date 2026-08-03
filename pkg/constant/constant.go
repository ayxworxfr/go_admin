// Package constant 收纳跨模块共享的常量。
//
// 只在这里放"确实被 2 个以上包同时用到、且必须保持一致"的值；只属于单个模块内部的
// 常量应该留在对应模块自己的包里（例如 jwtauth.ClaimsKey、jwtauth.AccessTokenType
// 只服务于 jwtauth 自身的语义，搬到这里反而会制造一个所有模块都要认识的无意义公共依赖）。
//
// 当前收录的两个常量是从真实的重复代码里提炼出来的：Authorization 头名和 Bearer
// 前缀曾经分别在 internal/platform/middleware（JWT 中间件）和 pkg/api
// （Context.BearerToken）各写了一份，且大小写敏感策略不一致——中间件用大小写敏感的
// 切片比较，api 用 strings.EqualFold，等价于同一个协议规则有两份可能分叉的实现。
package constant

// HeaderAuthorization 是标准 HTTP Authorization 请求头名称。
const HeaderAuthorization = "Authorization"

// BearerPrefix 是 Authorization 头中 Bearer Token 方案的前缀（含末尾空格）。
//
// 按 RFC 7235，认证方案名不区分大小写，比较该前缀时应配合 strings.EqualFold，
// 不要直接做大小写敏感的字符串切片比较。
const BearerPrefix = "Bearer "
