package jwtauth

import (
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/stretchr/testify/assert"
)

func TestParseDuration(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    time.Duration
		wantErr bool
		errMsg  string
	}{
		{
			name:    "60 seconds",
			input:   "60s",
			want:    60 * time.Second,
			wantErr: false,
		},
		{
			name:    "5 minutes",
			input:   "5m",
			want:    5 * time.Minute,
			wantErr: false,
		},
		{
			name:    "2 hours",
			input:   "2h",
			want:    2 * time.Hour,
			wantErr: false,
		},
		{
			name:    "1 day",
			input:   "1d",
			want:    24 * time.Hour,
			wantErr: false,
		},
		{
			name:    "3 weeks",
			input:   "3w",
			want:    3 * 7 * 24 * time.Hour,
			wantErr: false,
		},
		{
			name:    "empty string",
			input:   "",
			want:    0,
			wantErr: true,
			errMsg:  "empty duration string",
		},
		{
			name:    "invalid unit",
			input:   "10x",
			want:    0,
			wantErr: true,
			errMsg:  "unknown unit in duration",
		},
		{
			name:    "no unit",
			input:   "10",
			want:    0,
			wantErr: true,
			errMsg:  "invalid duration format",
		},
		{
			name:    "invalid number",
			input:   "abcdefh", // 全字母输入
			want:    0,
			wantErr: true,
			errMsg:  "invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseDuration(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseDuration() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errMsg != "" {
				assert.Contains(t, err.Error(), tt.errMsg)
			}
			if got != tt.want {
				t.Errorf("parseDuration() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestJWT_GenerateToken(t *testing.T) {
	jwtManager, err := NewJWT("test-secret-key", "24h", "30d")
	assert.NoError(t, err)

	userID := "123"
	username := "test-user"
	roleKey := "admin"

	tokenPair, err := jwtManager.GenerateToken(userID, username, roleKey)
	assert.NoError(t, err)
	assert.NotEmpty(t, tokenPair.AccessToken)
	assert.NotEmpty(t, tokenPair.RefreshToken)
}

func TestJWT_ParseToken(t *testing.T) {
	jwtManager, err := NewJWT("test-secret-key", "24h", "30d")
	assert.NoError(t, err)

	userID := "123"
	username := "test-user"
	roleKey := "admin"

	tokenPair, _ := jwtManager.GenerateToken(userID, username, roleKey)

	claims, err := jwtManager.ParseToken(tokenPair.AccessToken)
	assert.NoError(t, err)
	assert.Equal(t, userID, claims.Identity)
	assert.Equal(t, username, claims.Nice)
	assert.Equal(t, roleKey, claims.RoleKey)
	assert.Equal(t, "access", claims.Type)
}

func TestJWT_RefreshToken(t *testing.T) {
	jwtManager, err := NewJWT("test-secret-key", "1m", "30d")
	assert.NoError(t, err)

	userID := "123"
	username := "test-user"
	roleKey := "admin"

	// 生成初始token对
	initialPair, err := jwtManager.GenerateToken(userID, username, roleKey)
	assert.NoError(t, err)
	initialAccessToken := initialPair.AccessToken
	initialAccessClaims, err := jwtManager.ParseToken(initialAccessToken)
	assert.NoError(t, err)

	// 模拟一点时间流逝（确保过期时间不同）
	time.Sleep(1000 * time.Millisecond)

	// 刷新token
	refreshedPair, err := jwtManager.RefreshToken(initialPair.RefreshToken)
	assert.NoError(t, err)
	refreshedAccessToken := refreshedPair.AccessToken
	refreshedAccessClaims, err := jwtManager.ParseToken(refreshedAccessToken)
	assert.NoError(t, err)

	// 断言新token与原token不同
	assert.NotEqual(t, initialAccessToken, refreshedAccessToken)

	// 断言过期时间不同
	assert.NotEqual(t, initialAccessClaims.ExpiresAt, refreshedAccessClaims.ExpiresAt)

	// 断言用户信息相同
	assert.Equal(t, userID, refreshedAccessClaims.Identity)
	assert.Equal(t, username, refreshedAccessClaims.Nice)
	assert.Equal(t, roleKey, refreshedAccessClaims.RoleKey)
}

func TestJWT_ContextClaims(t *testing.T) {
	jwtManager, err := NewJWT("test-secret-key", "24h", "30d")
	assert.NoError(t, err)

	userID := "123"
	username := "test-user"
	roleKey := "admin"

	tokenPair, _ := jwtManager.GenerateToken(userID, username, roleKey)
	claims, _ := jwtManager.ParseToken(tokenPair.AccessToken)

	// 创建模拟上下文并设置claims
	ctx := app.NewContext(1)
	ctx.Set(ClaimsKey, claims)

	// 测试提取claims
	extractedClaims, err := jwtManager.ContextClaims(ctx)
	assert.NoError(t, err)
	assert.Equal(t, claims, extractedClaims)
}

func TestJWT_ExtractUserInfo(t *testing.T) {
	jwtManager, err := NewJWT("test-secret-key", "24h", "30d")
	assert.NoError(t, err)

	userID := "123"
	username := "test-user"
	roleKey := "admin"

	tokenPair, _ := jwtManager.GenerateToken(userID, username, roleKey)
	claims, _ := jwtManager.ParseToken(tokenPair.AccessToken)

	// 创建模拟上下文并设置claims
	ctx := app.NewContext(1)
	ctx.Set(ClaimsKey, claims)

	// 测试提取用户信息
	userInfo, err := jwtManager.ExtractUserInfo(ctx)
	assert.NoError(t, err)
	assert.Equal(t, userID, userInfo.UserID)
	assert.Equal(t, username, userInfo.Username)
	assert.Equal(t, roleKey, userInfo.RoleKey)
}

func TestJWT_InvalidDuration(t *testing.T) {
	// 测试无效的时间格式
	_, err := NewJWT("key", "invalid-duration", "30d")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid duration format")

	_, err = NewJWT("key", "10x", "30d")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown unit in duration")
}

func TestJWT_TokenExpiration(t *testing.T) {
	// 创建1分钟过期的token
	jwtManager, err := NewJWT("test-secret-key", "1m", "30d")
	assert.NoError(t, err)

	userID := "123"
	username := "test-user"
	roleKey := "admin"

	tokenPair, _ := jwtManager.GenerateToken(userID, username, roleKey)

	// 解析token并验证过期时间
	claims, err := jwtManager.ParseToken(tokenPair.AccessToken)
	assert.NoError(t, err)

	// 验证过期时间在1分钟内
	expiresAt := claims.ExpiresAt.Unix()
	now := time.Now().Unix()
	assert.True(t, expiresAt > now)
	assert.True(t, expiresAt < now+120) // 允许有少许误差
}
