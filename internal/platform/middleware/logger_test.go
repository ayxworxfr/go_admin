package middleware

import (
	"strings"
	"testing"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCaptureRequest_JSONBodyAndQuery(t *testing.T) {
	l := NewLogger(LoggerConfig{
		MaxBodyLogBytes: 4096,
		SensitiveFields: []string{"password", "token"},
		SkipPaths:       []string{},
	})

	c := app.NewContext(0)
	c.Request.SetRequestURI("/api/protected/user?username=admin&access_token=xyz")
	c.Request.Header.SetContentTypeBytes([]byte("application/json"))
	c.Request.SetBodyString(`{"username":"admin","password":"secret","nested":{"refresh_token":"abc"}}`)

	got := l.captureRequest(c)
	require.NotNil(t, got)
	require.NotNil(t, got.Query)
	assert.Equal(t, "admin", got.Query["username"])
	assert.Equal(t, "****", got.Query["access_token"])

	body, ok := got.Body.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "admin", body["username"])
	assert.Equal(t, "****", body["password"])
	nested, ok := body["nested"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "****", nested["refresh_token"])
}

func TestCaptureRequest_FormBody(t *testing.T) {
	l := NewLogger(LoggerConfig{
		SensitiveFields: []string{"password"},
		SkipPaths:       []string{},
	})

	c := app.NewContext(0)
	c.Request.Header.SetMethod("POST")
	c.Request.Header.SetContentTypeBytes([]byte("application/x-www-form-urlencoded"))
	c.Request.SetBodyString("username=admin&password=secret")

	got := l.captureRequest(c)
	require.NotNil(t, got)
	body, ok := got.Body.(map[string]string)
	require.True(t, ok)
	assert.Equal(t, "admin", body["username"])
	assert.Equal(t, "****", body["password"])
}

func TestCaptureRequest_GuessJSONWithoutContentType(t *testing.T) {
	l := NewLogger(LoggerConfig{SkipPaths: []string{}})

	c := app.NewContext(0)
	c.Request.SetBodyString(`{"id":1}`)

	got := l.captureRequest(c)
	require.NotNil(t, got)
	body, ok := got.Body.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(1), body["id"])
}

func TestCaptureRequest_EmptyReturnsNil(t *testing.T) {
	l := NewLogger(LoggerConfig{SkipPaths: []string{}})
	c := app.NewContext(0)
	assert.Nil(t, l.captureRequest(c))
}

func TestTruncate_RespectsLimit(t *testing.T) {
	l := NewLogger(LoggerConfig{
		MaxBodyLogBytes: 8,
		SkipPaths:       []string{},
	})

	got := l.truncateBytes([]byte("0123456789abcdef"))
	assert.True(t, strings.HasPrefix(got, "01234567"))
	assert.True(t, strings.HasSuffix(got, truncatedSuffix))
	assert.Equal(t, 8+len(truncatedSuffix), len(got))
}

func TestSkipPathsDefaultIncludesHealth(t *testing.T) {
	l := NewLogger()
	_, ok := l.skipPaths["/health"]
	assert.True(t, ok)
	_, ok = l.skipPaths["/metrics"]
	assert.True(t, ok)
}
