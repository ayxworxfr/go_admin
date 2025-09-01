package middleware

import (
	"context"
	"strconv"
	"strings"

	"github.com/ayxworxfr/go_admin/internal/service"
	mycontext "github.com/ayxworxfr/go_admin/pkg/context"
	"github.com/ayxworxfr/go_admin/pkg/jwtauth"
	"github.com/ayxworxfr/go_admin/pkg/logger"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"go.uber.org/zap"
)

// 权限验证配置
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

// JWTMiddleware JWT认证中间件
func JWTMiddleware(config ...PermissionConfig) app.HandlerFunc {
	// 设置配置
	cfg := defaultPermissionConfig
	if len(config) > 0 {
		cfg = config[0]
	}

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

		claims, err := jwtauth.Instance.ParseToken(tokenString)
		if err != nil {
			rsp := mycontext.Unauthorized("Invalid token: " + err.Error())
			c.JSON(consts.StatusUnauthorized, rsp)
			c.Abort()
			return
		}

		// 2. 提取用户信息
		userIDStr := claims.Identity
		userID, err := strconv.ParseUint(userIDStr, 10, 64)
		if err != nil {
			rsp := mycontext.Unauthorized("Invalid user ID in token")
			c.JSON(consts.StatusUnauthorized, rsp)
			c.Abort()
			return
		}
		c.Set(jwtauth.ClaimsKey, claims)

		// 3. 权限验证（如果启用）
		if cfg.Enable {
			requestMethod := string(c.Request.Method())
			requestPath := string(c.Request.URI().Path())
			methodPath := requestMethod + ":" + requestPath

			// 检查是否在排除列表中
			if isExcludedPath(methodPath, cfg.ExcludePaths) {
				c.Next(ctx)
				return
			}

			// 检查用户是否有权限访问此路径
			hasPermission, err := service.PermissionServiceInstance.HasPermission(ctx, userID, requestMethod, requestPath)
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
