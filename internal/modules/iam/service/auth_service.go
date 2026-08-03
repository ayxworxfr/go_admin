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
)

// AuthService 认证服务：登录、刷新令牌、登出。依赖的都是模块导出的最小接口
// （user.UserFinder/user.LoginRecorder）或同模块内的协作对象（UserRoleService），
// JWT 管理器通过构造函数注入。
type AuthService struct {
	userFinder    usersvc.UserFinder
	loginRecorder usersvc.LoginRecorder
	userRoleSvc   *UserRoleService
	tokenStore    tokenstore.TokenStore
	jwt           *jwtauth.JWT
}

// NewAuthService 创建认证服务
func NewAuthService(userFinder usersvc.UserFinder, loginRecorder usersvc.LoginRecorder, userRoleSvc *UserRoleService, tokenStore tokenstore.TokenStore, jwt *jwtauth.JWT) *AuthService {
	return &AuthService{
		userFinder:    userFinder,
		loginRecorder: loginRecorder,
		userRoleSvc:   userRoleSvc,
		tokenStore:    tokenStore,
		jwt:           jwt,
	}
}

// Login 用户登录：校验密码 -> 取角色 -> 生成令牌
func (s *AuthService) Login(ctx context.Context, username, password string) (*dto.TokenResponse, error) {
	user, err := s.userFinder.FindByUsername(ctx, username)
	if err != nil {
		logger.Error(ctx, "Login failed", logger.Err(err), logger.String("username", username))
		return nil, errors.New("invalid credentials")
	}

	if !s.userFinder.VerifyPassword(user, password) {
		logger.Warn(ctx, "Invalid password", logger.String("username", username))
		return nil, errors.New("invalid credentials")
	}

	roleCode, err := s.resolveRoleCode(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	tokenPair, err := s.jwt.GenerateToken(strconv.FormatUint(user.ID, 10), user.Username, roleCode)
	if err != nil {
		logger.Error(ctx, "Failed to generate token", logger.Err(err), logger.Uint64("user_id", user.ID))
		return nil, errors.Wrap(err, "failed to generate token")
	}

	// 回写登录时间不应该让整个登录流程失败：这是一次审计性质的旁路写入，
	// 失败只记录日志，用户拿到的令牌依旧有效。
	if err := s.loginRecorder.UpdateLastLoginTime(ctx, user.ID); err != nil {
		logger.Warn(ctx, "Failed to update last login time", logger.Err(err), logger.Uint64("user_id", user.ID))
	}

	logger.Info(ctx, "Login successful", logger.String("username", user.Username))
	return &dto.TokenResponse{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
		ExpiresAt:    tokenPair.ExpiresAt,
	}, nil
}

// RefreshToken 使用 refresh token 换取新的令牌对。
//
// 刻意不在这里回写 last_login_time：该字段语义是"用户上一次真正输入凭证登录
// 的时间"，用于安全审计（如"这个账号最近有没有人登录过"），而刷新令牌是
// 前端静默发起的、用户无感知的动作，可能在会话期间触发很多次。如果这里也
// 更新，这一列就退化成了"最后一次活跃时间"，会掩盖真实的登录行为，且给
// 高频路径多引入一次不必要的写放大。如果需要"最后活跃时间"这类语义，
// 应该新增单独字段，不要复用 last_login_time。
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
		logger.Warn(ctx, "Failed to resolve role for token refresh", logger.Err(err), logger.Uint64("user_id", userID))
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
