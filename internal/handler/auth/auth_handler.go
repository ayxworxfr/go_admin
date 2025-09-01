package auth_handler

import (
	"errors"

	"github.com/ayxworxfr/go_admin/internal/domain/params"
	"github.com/ayxworxfr/go_admin/internal/domain/vo"
	"github.com/ayxworxfr/go_admin/internal/service"
	"github.com/ayxworxfr/go_admin/pkg/context"
	"github.com/ayxworxfr/go_admin/pkg/jwtauth"
)

type ILoginHandler interface {
	Login(c *context.Context) *context.Response
	RefreshToken(c *context.Context) *context.Response
	LoginOut(c *context.Context) *context.Response
	ProtectedHandler(c *context.Context) *context.Response
}

type LoginHandler struct{}

// @route POST /login
func (h *LoginHandler) Login(c *context.Context) *context.Response {
	var req params.LoginRequest
	if err := c.BindAndValidate(&req); err != nil {
		return context.ParamError(err)
	}

	token, err := service.AuthServiceInstance.Login(c.Context(), req.Username, req.Password)
	if err != nil {
		if errors.Is(err, errors.New("invalid credentials")) {
			return context.Unauthorized("Invalid credentials")
		}
		return context.InternalError(err)
	}

	claims, err := jwtauth.Instance.ParseToken(token.AccessToken)
	if err != nil {
		return context.Unauthorized("Invalid token")
	}
	result := vo.LoginResult{
		TokenResponse: vo.TokenResponse{
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
func (h *LoginHandler) RefreshToken(c *context.Context) *context.Response {
	var req params.RefreshTokenRequest
	if err := c.BindAndValidate(&req); err != nil {
		return context.ParamError(err)
	}

	token, err := service.AuthServiceInstance.RefreshToken(c.Context(), req.RefreshToken)
	if err != nil {
		return context.Unauthorized(err.Error())
	}

	return context.Success(token)
}

func (h *LoginHandler) LoginOut(c *context.Context) *context.Response {
	// todo 让token失效
	return context.Success("LoginOut")
}

// ProtectedHandler 受保护的路由示例
func (h *LoginHandler) ProtectedHandler(c *context.Context) *context.Response {
	claims, err := jwtauth.Instance.ContextClaims(c.RequestContext)
	if err != nil {
		return context.Unauthorized("Invalid token")
	}

	return context.Success(claims)
}
