package service

import (
	"context"
	"strconv"
	"time"

	"github.com/ayxworxfr/go_admin/internal/modules/iam/dto"
	"github.com/ayxworxfr/go_admin/internal/modules/iam/tokenstore"
	usersvc "github.com/ayxworxfr/go_admin/internal/modules/user/service"
	"github.com/ayxworxfr/go_admin/pkg/jwtauth"
	"github.com/ayxworxfr/go_admin/pkg/logger"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

// AuthService 认证服务：登录、刷新令牌、登出。依赖的都是模块导出的最小接口
// （user.UserFinder）或同模块内的协作对象（UserRoleService），不再依赖
// pkg/jwtauth.Instance 这个全局变量。
type AuthService struct {
	userFinder  usersvc.UserFinder
	userRoleSvc *UserRoleService
	tokenStore  tokenstore.TokenStore
	jwt         *jwtauth.JWT
}

// NewAuthService 创建认证服务
func NewAuthService(userFinder usersvc.UserFinder, userRoleSvc *UserRoleService, tokenStore tokenstore.TokenStore, jwt *jwtauth.JWT) *AuthService {
	return &AuthService{
		userFinder:  userFinder,
		userRoleSvc: userRoleSvc,
		tokenStore:  tokenStore,
		jwt:         jwt,
	}
}

// Login 用户登录：校验密码 -> 取角色 -> 生成令牌
func (s *AuthService) Login(ctx context.Context, username, password string) (*dto.TokenResponse, error) {
	user, err := s.userFinder.FindByUsername(ctx, username)
	if err != nil {
		logger.Error(ctx, "Login failed", zap.Error(err), zap.String("username", username))
		return nil, errors.New("invalid credentials")
	}

	if !s.userFinder.VerifyPassword(user, password) {
		logger.Warn(ctx, "Invalid password", zap.String("username", username))
		return nil, errors.New("invalid credentials")
	}

	roleCode, err := s.resolveRoleCode(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	tokenPair, err := s.jwt.GenerateToken(strconv.FormatUint(user.ID, 10), user.Username, roleCode)
	if err != nil {
		logger.Error(ctx, "Failed to generate token", zap.Error(err), zap.Uint64("user_id", user.ID))
		return nil, errors.Wrap(err, "failed to generate token")
	}

	logger.Info(ctx, "Login successful", zap.String("username", user.Username))
	return &dto.TokenResponse{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
		ExpiresAt:    tokenPair.ExpiresAt,
	}, nil
}

// RefreshToken 使用 refresh token 换取新的令牌对
func (s *AuthService) RefreshToken(ctx context.Context, refreshToken string) (*dto.TokenResponse, error) {
	if refreshToken == "" {
		return nil, errors.New("refresh token is required")
	}

	claims, err := s.jwt.ParseToken(refreshToken)
	if err != nil {
		return nil, errors.Wrap(err, "invalid refresh token")
	}
	if claims.Type != jwtauth.RefreshTokenType {
		return nil, errors.New("not a refresh token")
	}

	userID, err := strconv.ParseUint(claims.Identity, 10, 64)
	if err != nil {
		return nil, errors.Wrap(err, "invalid user ID in token")
	}

	roleCode, err := s.resolveRoleCode(ctx, userID)
	if err != nil {
		logger.Warn(ctx, "Failed to resolve role for token refresh", zap.Error(err), zap.Uint64("user_id", userID))
		roleCode = "guest"
	}

	newToken, err := s.jwt.GenerateToken(claims.Identity, claims.Nice, roleCode)
	if err != nil {
		return nil, errors.Wrap(err, "could not generate new token")
	}

	return &dto.TokenResponse{
		AccessToken:  newToken.AccessToken,
		RefreshToken: newToken.RefreshToken,
		ExpiresAt:    newToken.ExpiresAt,
	}, nil
}

// Logout 撤销当前 access token，使其在过期前立即失效
func (s *AuthService) Logout(ctx context.Context, claims *jwtauth.Claims) error {
	if claims == nil || claims.ID == "" {
		return nil
	}
	var exp time.Time
	if claims.ExpiresAt != nil {
		exp = claims.ExpiresAt.Time
	} else {
		exp = time.Now().Add(24 * time.Hour)
	}
	return s.tokenStore.Revoke(ctx, claims.ID, exp)
}

// resolveRoleCode 取用户优先级最高角色的 code，无角色时退化为 guest
func (s *AuthService) resolveRoleCode(ctx context.Context, userID uint64) (string, error) {
	roles, err := s.userRoleSvc.RetrieveRoleResponsesByUserID(ctx, userID, 0)
	if err != nil {
		return "", errors.Wrap(err, "failed to retrieve user roles")
	}
	if role := fetchHighestPriorityRole(roles); role != nil {
		return role.Code, nil
	}
	return "guest", nil
}
