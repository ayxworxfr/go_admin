package middleware

import (
	"context"
	"strconv"
	"strings"

	"github.com/ayxworxfr/go_admin/pkg/api"
	"github.com/ayxworxfr/go_admin/pkg/constant"
	"github.com/ayxworxfr/go_admin/pkg/jwtauth"
	"github.com/ayxworxfr/go_admin/pkg/logger"
	"github.com/cloudwego/hertz/pkg/app"
)

// PermissionChecker 是 JWTMiddleware 鉴权所需的最小接口，由 iam 模块的
// PermissionChecker 实现。中间件只依赖这一个方法，不感知权限数据的存储
// 与缓存细节（策略模式：具体校验策略由调用方注入）。
type PermissionChecker interface {
	HasPermission(ctx context.Context, userID uint64, method, path string) (bool, error)
}

// TokenStore 是令牌撤销状态查询的最小接口，由 iam 模块的 TokenStore 实现，
// 用于支撑登出后的 token 失效。
type TokenStore interface {
	IsRevoked(ctx context.Context, jti string) (bool, error)
}

// PermissionConfig 权限验证配置
type PermissionConfig struct {
	// 不需要验证权限的路径
	ExcludePaths []string
	// 是否启用权限验证
	Enable bool
}

// 默认配置
var defaultPermissionConfig = PermissionConfig{
	ExcludePaths: []string{"/api/login", "/api/refresh"},
	Enable:       true,
}

// JWTAuthMiddleware 承载 JWT 认证所需的依赖。JWT / PermissionChecker /
// TokenStore 均由组合根构造注入，方便测试替换与后续更换实现（如 Redis TokenStore）。
type JWTAuthMiddleware struct {
	jwt        *jwtauth.JWT
	checker    PermissionChecker
	tokenStore TokenStore
	config     PermissionConfig
}

// NewJWTMiddleware 创建 JWT 认证中间件，checker 与 tokenStore 由 Container 组装时注入。
func NewJWTMiddleware(jwt *jwtauth.JWT, checker PermissionChecker, tokenStore TokenStore, config ...PermissionConfig) *JWTAuthMiddleware {
	cfg := defaultPermissionConfig
	if len(config) > 0 {
		cfg = config[0]
	}
	return &JWTAuthMiddleware{jwt: jwt, checker: checker, tokenStore: tokenStore, config: cfg}
}

// Handle 返回 Hertz 处理函数
func (m *JWTAuthMiddleware) Handle() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		rawAuth := c.Request.Header.Get(constant.HeaderAuthorization)
		if rawAuth == "" {
			api.Abort(c, api.Unauthorized("No token provided"))
			return
		}
		tokenString := api.StripBearerPrefix(rawAuth)

		claims, err := m.jwt.ParseToken(tokenString)
		if err != nil {
			api.Abort(c, api.Unauthorized("Invalid token: "+err.Error()))
			return
		}

		// Token撤销检查：jti 为空（异常场景）时直接放行，避免误判下线
		if claims.ID != "" {
			revoked, err := m.tokenStore.IsRevoked(ctx, claims.ID)
			if err != nil {
				logger.Error(ctx, "Failed to check token revocation", logger.Err(err))
				api.Abort(c, api.Unauthorized("Token check error"))
				return
			}
			if revoked {
				api.Abort(c, api.Unauthorized("Token has been revoked"))
				return
			}
		}

		userID, err := strconv.ParseUint(claims.Identity, 10, 64)
		if err != nil {
			api.Abort(c, api.Unauthorized("Invalid user ID in token"))
			return
		}
		c.Set(jwtauth.ClaimsKey, claims)

		if m.config.Enable {
			requestMethod := string(c.Request.Method())
			requestPath := string(c.Request.URI().Path())
			methodPath := requestMethod + ":" + requestPath

			if isExcludedPath(methodPath, m.config.ExcludePaths) {
				c.Next(ctx)
				return
			}

			hasPermission, err := m.checker.HasPermission(ctx, userID, requestMethod, requestPath)
			if err != nil {
				logger.Error(ctx, "Failed to check permission", logger.Err(err),
					logger.Uint64("user_id", userID), logger.String("method", requestMethod), logger.String("path", requestPath))
				api.Abort(c, api.InternalError("Permission check error"))
				return
			}
			if !hasPermission {
				logger.Warn(ctx, "Permission denied",
					logger.Uint64("user_id", userID), logger.String("method", requestMethod), logger.String("path", requestPath))
				api.Abort(c, api.Forbidden("Permission denied"))
				return
			}
		}

		c.Next(ctx)
	}
}

// isExcludedPath 检查路径是否在排除列表中
func isExcludedPath(methodPath string, excludePaths []string) bool {
	for _, excludePath := range excludePaths {
		// 支持直接匹配和通配符匹配
		// 例如：排除 GET:/api/login 或 */api/health

		// 如果排除路径包含冒号，说明指定了方法
		if strings.Contains(excludePath, ":") {
			if methodPath == excludePath {
				return true
			}

			// 支持方法通配符，如 *:/api/health
			excludeParts := strings.SplitN(excludePath, ":", 2)
			if excludeParts[0] == "*" && strings.HasSuffix(methodPath, ":"+excludeParts[1]) {
				return true
			}
		} else {
			// 如果不包含冒号，检查路径部分是否匹配
			// 例如：排除 /api/login 则匹配所有方法的 /api/login
			if strings.HasSuffix(methodPath, ":"+excludePath) {
				return true
			}
		}
	}
	return false
}
