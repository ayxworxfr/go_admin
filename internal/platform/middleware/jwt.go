package middleware

import (
	"context"
	"strconv"
	"strings"

	mycontext "github.com/ayxworxfr/go_admin/pkg/context"
	"github.com/ayxworxfr/go_admin/pkg/jwtauth"
	"github.com/ayxworxfr/go_admin/pkg/logger"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"go.uber.org/zap"
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
	ExcludePaths: []string{"/api/login", "/api/refresh", "/api/hello"},
	Enable:       true,
}

// JWTAuthMiddleware 承载 JWT 认证所需的依赖。相比旧版直连
// jwtauth.Instance / service.PermissionServiceInstance 两个全局单例，
// 这里改为构造注入，方便测试替换与后续更换实现（如 Redis TokenStore）。
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
		// 1. JWT验证
		tokenString := c.Request.Header.Get("Authorization")
		if tokenString == "" {
			rsp := mycontext.Unauthorized("No token provided")
			c.JSON(consts.StatusUnauthorized, rsp)
			c.Abort()
			return
		}

		// 移除 "Bearer " 前缀
		if len(tokenString) > 7 && tokenString[:7] == "Bearer " {
			tokenString = tokenString[7:]
		}

		claims, err := m.jwt.ParseToken(tokenString)
		if err != nil {
			rsp := mycontext.Unauthorized("Invalid token: " + err.Error())
			c.JSON(consts.StatusUnauthorized, rsp)
			c.Abort()
			return
		}

		// 2. Token撤销检查：jti 为空（异常场景）时直接放行，避免误判下线
		if claims.ID != "" {
			revoked, err := m.tokenStore.IsRevoked(ctx, claims.ID)
			if err != nil {
				logger.Error(ctx, "Failed to check token revocation", zap.Error(err))
				rsp := mycontext.Unauthorized("Token check error")
				c.JSON(consts.StatusUnauthorized, rsp)
				c.Abort()
				return
			}
			if revoked {
				rsp := mycontext.Unauthorized("Token has been revoked")
				c.JSON(consts.StatusUnauthorized, rsp)
				c.Abort()
				return
			}
		}

		// 3. 提取用户信息
		userIDStr := claims.Identity
		userID, err := strconv.ParseUint(userIDStr, 10, 64)
		if err != nil {
			rsp := mycontext.Unauthorized("Invalid user ID in token")
			c.JSON(consts.StatusUnauthorized, rsp)
			c.Abort()
			return
		}
		c.Set(jwtauth.ClaimsKey, claims)

		// 4. 权限验证（如果启用）
		if m.config.Enable {
			requestMethod := string(c.Request.Method())
			requestPath := string(c.Request.URI().Path())
			methodPath := requestMethod + ":" + requestPath

			// 检查是否在排除列表中
			if isExcludedPath(methodPath, m.config.ExcludePaths) {
				c.Next(ctx)
				return
			}

			// 检查用户是否有权限访问此路径
			hasPermission, err := m.checker.HasPermission(ctx, userID, requestMethod, requestPath)
			if err != nil {
				logger.Error(ctx, "Failed to check permission", zap.Error(err),
					zap.Uint64("user_id", userID), zap.String("method", requestMethod), zap.String("path", requestPath))
				rsp := mycontext.Unauthorized("Permission check error")
				c.JSON(consts.StatusUnauthorized, rsp)
				c.Abort()
				return
			}

			if !hasPermission {
				logger.Warn(ctx, "Permission denied",
					zap.Uint64("user_id", userID), zap.String("method", requestMethod), zap.String("path", requestPath))
				rsp := mycontext.Unauthorized("Permission denied")
				c.JSON(consts.StatusUnauthorized, rsp)
				c.Abort()
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
