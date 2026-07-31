package handler

import (
	"github.com/ayxworxfr/go_admin/internal/modules/iam/dto"
	"github.com/ayxworxfr/go_admin/internal/modules/iam/service"
	"github.com/ayxworxfr/go_admin/pkg/jwtauth"
	"github.com/ayxworxfr/go_admin/pkg/reqctx"
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
func (h *AuthHandler) Login(c *reqctx.Context) *reqctx.Response {
	var req dto.LoginRequest
	if err := c.BindAndValidate(&req); err != nil {
		return reqctx.ParamError(err)
	}

	token, err := h.authSvc.Login(c.Context(), req.Username, req.Password)
	if err != nil {
		return reqctx.Unauthorized(err)
	}

	claims, err := h.jwt.ParseToken(token.AccessToken)
	if err != nil {
		return reqctx.Unauthorized("Invalid token")
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
	return reqctx.Success(result)
}

// @route POST /refresh/token
// RefreshToken 刷新令牌
func (h *AuthHandler) RefreshToken(c *reqctx.Context) *reqctx.Response {
	var req dto.RefreshTokenRequest
	if err := c.BindAndValidate(&req); err != nil {
		return reqctx.ParamError(err)
	}

	token, err := h.authSvc.RefreshToken(c.Context(), req.RefreshToken)
	if err != nil {
		return reqctx.Unauthorized(err.Error())
	}
	return reqctx.Success(token)
}

// @route POST /logout
// 该接口在公开路由组，不走 JWT 中间件，因此在此自行解析 Bearer access token。
func (h *AuthHandler) Logout(c *reqctx.Context) *reqctx.Response {
	tokenString := c.BearerToken()
	if tokenString == "" {
		return reqctx.Unauthorized("No token provided")
	}

	claims, err := h.jwt.ParseToken(tokenString)
	if err != nil {
		return reqctx.Unauthorized("Invalid token")
	}
	if claims.Type != "" && claims.Type != jwtauth.AccessTokenType {
		return reqctx.Unauthorized("Access token required")
	}
	if err := h.authSvc.Logout(c.Context(), claims); err != nil {
		return reqctx.InternalError(err)
	}
	return reqctx.Success("Logout")
}
