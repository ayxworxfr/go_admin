package service

import (
	"context"
	"strconv"

	"github.com/ayxworxfr/go_admin/internal/dao"
	"github.com/ayxworxfr/go_admin/internal/domain/models"
	"github.com/ayxworxfr/go_admin/internal/domain/vo"
	"github.com/ayxworxfr/go_admin/pkg/jwtauth"
	"github.com/ayxworxfr/go_admin/pkg/logger"
	"github.com/ayxworxfr/go_admin/pkg/repository"
	"github.com/pkg/errors"
	"github.com/samber/lo"
	"go.uber.org/zap"
)

// 权限响应控制标志（位标志）
const (
	RESPONSE_INCLUDE_ROLE       = 1 << iota // 包含用户角色信息
	RESPONSE_INCLUDE_PERMISSION             // 包含用户权限信息
	RESPONSE_INCLUDE_DETAIL                 // 包含详细信息
)

// AuthService 认证服务 - 负责用户认证和令牌管理
type AuthService struct {
	userRepo    repository.Repository[models.User]
	permService *PermissionService // 依赖权限服务
}

// NewAuthService 创建认证服务实例
func NewAuthService(permService *PermissionService) *AuthService {
	return &AuthService{
		userRepo:    dao.UserRepo,
		permService: permService,
	}
}

// Login 用户登录
func (s *AuthService) Login(ctx context.Context, username, password string) (*vo.TokenResponse, error) {
	query := models.User{Username: username}
	user, err := s.userRepo.Find(ctx, &query)
	if err != nil {
		logger.Error(ctx, "Login failed", zap.Error(err), zap.String("username", username))
		return nil, errors.New("invalid credentials")
	}

	// 校验密码
	if !user.Verify(password) {
		logger.Warn(ctx, "Invalid password", zap.String("username", username))
		return nil, errors.New("invalid credentials")
	}

	// 获取用户角色
	roles, err := s.permService.RetrieveRoleVosByUserID(ctx, user.ID)
	if err != nil {
		logger.Error(ctx, "Failed to retrieve user roles", zap.Error(err), zap.Uint64("user_id", user.ID))
		return nil, errors.Wrap(err, "failed to retrieve user roles")
	}

	// 获取最高优先级角色
	role := s.permService.fetchHighestPriorityRole(roles)
	roleCode := "guest" // 默认角色
	if role != nil {
		roleCode = role.Code
	}

	// 生成令牌（包含用户权限标志）
	tokenInfo, err := jwtauth.Instance.GenerateToken(
		strconv.FormatUint(user.ID, 10),
		user.Username,
		roleCode,
	)
	if err != nil {
		logger.Error(ctx, "Failed to generate token", zap.Error(err), zap.Uint64("user_id", user.ID))
		return nil, errors.Wrap(err, "failed to generate token")
	}

	logger.Info(ctx, "Login successful", zap.String("username", user.Username))
	return &vo.TokenResponse{
		AccessToken:  tokenInfo.AccessToken,
		RefreshToken: tokenInfo.RefreshToken,
		ExpiresAt:    tokenInfo.ExpiresAt,
	}, nil
}

// RefreshToken 刷新令牌
func (s *AuthService) RefreshToken(ctx context.Context, refreshToken string) (*vo.TokenResponse, error) {
	if refreshToken == "" {
		return nil, errors.New("refresh token is required")
	}

	claims, err := jwtauth.Instance.ParseToken(refreshToken)
	if err != nil {
		return nil, errors.Wrap(err, "invalid refresh token")
	}

	userID, err := strconv.ParseUint(claims.Identity, 10, 64)
	if err != nil {
		return nil, errors.Wrap(err, "invalid user ID in token")
	}

	// 根据用户ID获取角色信息以更新令牌
	roles, err := s.permService.RetrieveRoleVosByUserID(ctx, userID)
	if err != nil {
		logger.Warn(ctx, "Failed to retrieve user roles for token refresh",
			zap.Error(err), zap.Uint64("user_id", userID))
	}

	roleCode := "guest"
	if len(roles) > 0 {
		role := s.permService.fetchHighestPriorityRole(roles)
		if role != nil {
			roleCode = role.Code
		}
	}

	newToken, err := jwtauth.Instance.GenerateToken(
		claims.Identity,
		claims.Nice,
		roleCode,
	)
	if err != nil {
		return nil, errors.Wrap(err, "could not generate new token")
	}

	return &vo.TokenResponse{
		AccessToken:  newToken.AccessToken,
		RefreshToken: newToken.RefreshToken,
		ExpiresAt:    newToken.ExpiresAt,
	}, nil
}

// BuildUserResponse 构建用户响应（使用位标志控制返回内容）
func (s *AuthService) BuildUserResponse(ctx context.Context, userID uint64, flags int) (map[string]any, error) {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		logger.Error(ctx, "Failed to retrieve user", zap.Error(err), zap.Uint64("user_id", userID))
		return nil, errors.Wrap(err, "failed to retrieve user")
	}

	response := map[string]any{
		"id":   user.ID,
		"name": user.Username,
	}

	// 根据位标志动态添加响应字段
	if flags&RESPONSE_INCLUDE_ROLE != 0 {
		roles, err := s.permService.RetrieveRoleVosByUserID(ctx, userID)
		if err == nil {
			response["roles"] = lo.Map(roles, func(r *vo.Role, _ int) string { return r.Name })
		}
	}

	if flags&RESPONSE_INCLUDE_PERMISSION != 0 {
		permissions, err := s.permService.GetUserPermissions(ctx, userID)
		if err == nil {
			response["permissions"] = lo.Map(permissions, func(p vo.Permission, _ int) string {
				return p.Method + ":" + p.Path
			})
		}
	}

	if flags&RESPONSE_INCLUDE_DETAIL != 0 {
		response["create_time"] = user.CreateTime
		response["update_time"] = user.UpdateTime
	}

	return response, nil
}
