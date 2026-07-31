package reqctx

import (
	"context"
	"testing"

	"github.com/ayxworxfr/go_admin/pkg/jwtauth"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/stretchr/testify/require"
)

func newTestContext(t *testing.T) *Context {
	t.Helper()
	rc := app.NewContext(0)
	rc.Request.Header.SetMethod("GET")
	rc.Request.SetRequestURI("/test")
	return New(context.Background(), rc)
}

func TestContext_HeaderAndBearerToken(t *testing.T) {
	c := newTestContext(t)
	c.Request().Request.Header.Set("Authorization", "Bearer abc.def")
	require.Equal(t, "Bearer abc.def", c.Header("Authorization"))
	require.Equal(t, "abc.def", c.BearerToken())

	c.Request().Request.Header.Set("Authorization", "raw-token")
	require.Equal(t, "raw-token", c.BearerToken())
}

func TestContext_UserIDAndClaims(t *testing.T) {
	c := newTestContext(t)
	_, err := c.UserID()
	require.Error(t, err)

	c.Request().Set(jwtauth.ClaimsKey, &jwtauth.Claims{
		Identity: "42",
		Nice:     "admin",
		RoleKey:  "go_admin",
	})
	id, err := c.UserID()
	require.NoError(t, err)
	require.Equal(t, uint64(42), id)

	claims, err := c.Claims()
	require.NoError(t, err)
	require.Equal(t, "admin", claims.Nice)
}

func TestResponse_HTTPStatus(t *testing.T) {
	require.Equal(t, 200, Success(nil).HTTPStatus())
	require.Equal(t, 400, ParamError("x").HTTPStatus())
	require.Equal(t, 401, Unauthorized("x").HTTPStatus())
	require.Equal(t, 403, Forbidden("x").HTTPStatus())
	require.Equal(t, 404, NotFound("x").HTTPStatus())
	require.Equal(t, 409, Conflict("x").HTTPStatus())
	require.Equal(t, 429, RateLimit("x").HTTPStatus())
	require.Equal(t, 500, DatabaseError("x").HTTPStatus())
}
