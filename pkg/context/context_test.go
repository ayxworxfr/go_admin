package context

import (
	"testing"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/stretchr/testify/assert"
)

func TestContext_SetHeader(t *testing.T) {
	c := createTestContext()
	c.SetHeader("Test-Header", "test-value")

	value := c.GetResponseHeader("Test-Header")
	assert.Equal(t, "test-value", value)
}

func TestContext_GetHeader(t *testing.T) {
	c := createTestContext()
	c.RequestContext.Request.Header.Set("Test-Header", "test-value")

	value := c.GetHeader("Test-Header")
	assert.Equal(t, "test-value", value)
}

func TestContext_SetCookie(t *testing.T) {
	c := createTestContext()
	c.SetCookie("test-cookie", "test-value", 3600, "/", "example.com", true, true)

	cookie := c.RequestContext.Response.Header.Get("Set-Cookie")
	assert.Contains(t, cookie, "test-cookie=test-value")
	assert.Contains(t, cookie, "max-age=3600")
	assert.Contains(t, cookie, "domain=example.com")
	assert.Contains(t, cookie, "path=/")
	assert.Contains(t, cookie, "HttpOnly")
	assert.Contains(t, cookie, "secure")
	// Note: SameSite is also included by default in Hertz
	assert.Contains(t, cookie, "SameSite")
}

func TestContext_Cookie(t *testing.T) {
	c := createTestContext()
	c.Request.Header.Set("Cookie", "test-cookie=test-value")

	value := c.Cookie("test-cookie")
	assert.Equal(t, "test-value", value)
}

func TestContext_GetUserValue(t *testing.T) {
	c := createTestContext()
	c.SetUserValue("test-key", "test-value")

	value, exists := c.GetUserValue("test-key")
	assert.True(t, exists)
	assert.Equal(t, "test-value", value)
}

func TestContext_IsWebsocket(t *testing.T) {
	c := createTestContext()
	c.Request.Header.Set("Connection", "Upgrade")
	c.Request.Header.Set("Upgrade", "websocket")

	assert.True(t, c.IsWebsocket())

	c.Request.Header.Del("Upgrade")
	assert.False(t, c.IsWebsocket())
}

func createTestContext() *Context {
	h := app.NewContext(0)
	h.Request.Header.SetMethod("GET")
	h.Request.SetRequestURI("/test")
	return &Context{RequestContext: h}
}
