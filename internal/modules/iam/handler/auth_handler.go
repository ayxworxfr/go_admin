package handler

import (
	"github.com/ayxworxfr/go_admin/internal/modules/iam/dto"
	"github.com/ayxworxfr/go_admin/internal/modules/iam/service"
	"github.com/ayxworxfr/go_admin/pkg/context"
	"github.com/ayxworxfr/go_admin/pkg/jwtauth"
)

// AuthHandler 登录会话相关接口
type AuthHandler struct {
	authSvc *service.AuthService
	jwt     *jwtauth.JWT
}

// NewAuthHandler 创建登录处理器
func NewAuthHandler(authSvc *service.AuthService, jwt *jwtauth.JWT) *AuthHandler {
	return &AuthHandler{authSvc: authSvc, jwt: jwt}
}

// @route POST /login
func (h *AuthHandler) Login(c *context.Context) *context.Response {
	var req dto.LoginRequest
	if err := c.BindAndValidate(&req); err != nil {
		return context.ParamError(err)
	}

	token, err := h.authSvc.Login(c.Context(), req.Username, req.Password)
	if err != nil {
		return context.Unauthorized(err)
	}

	claims, err := h.jwt.ParseToken(token.AccessToken)
	if err != nil {
		return context.Unauthorized("Invalid token")
	}

	result := dto.LoginResult{
		TokenResponse: dto.TokenResponse{
			AccessToken:  token.AccessToken,
			RefreshToken: token.RefreshToken,
			ExpiresAt:    token.ExpiresAt,
		},
		Status:           "ok",
		Type:             "account",
		CurrentAuthority: claims.RoleKey,
	}
	return context.Success(result)
}

// @route POST /refresh/token
// RefreshToken 刷新令牌
func (h *AuthHandler) RefreshToken(c *context.Context) *context.Response {
	var req dto.RefreshTokenRequest
	if err := c.BindAndValidate(&req); err != nil {
		return context.ParamError(err)
	}

	token, err := h.authSvc.RefreshToken(c.Context(), req.RefreshToken)
	if err != nil {
		return context.Unauthorized(err.Error())
	}
	return context.Success(token)
}

// @route POST /logout
func (h *AuthHandler) LoginOut(c *context.Context) *context.Response {
	claims, err := h.jwt.ContextClaims(c.RequestContext)
	if err != nil {
		return context.Unauthorized("Invalid token")
	}
	if err := h.authSvc.Logout(c.Context(), claims); err != nil {
		return context.InternalError(err)
	}
	return context.Success("LoginOut")
}

// ProtectedHandler 受保护的路由示例
func (h *AuthHandler) ProtectedHandler(c *context.Context) *context.Response {
	claims, err := h.jwt.ContextClaims(c.RequestContext)
	if err != nil {
		return context.Unauthorized("Invalid token")
	}
	return context.Success(claims)
}
