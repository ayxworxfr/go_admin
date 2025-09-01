package jwtauth

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/golang-jwt/jwt/v5"
)

var Instance *JWT

const (
	// AccessTokenType 表示 Access Token 类型
	AccessTokenType = "access"
	// RefreshTokenType 表示 Refresh Token 类型
	RefreshTokenType = "refresh"
	// ClaimsKey 表示 JWT 载荷的键名
	ClaimsKey = "jwt_claims"
)

func Init(jwt *JWT) {
	Instance = jwt
}

// Claims 定义 JWT 载荷结构
type Claims struct {
	Identity string `json:"identity"` // 用户ID
	Nice     string `json:"nice"`     // 用户名
	RoleKey  string `json:"rolekey"`  // 角色标识
	Type     string `json:"type"`     // token类型：access/refresh
	jwt.RegisteredClaims
}

// TokenPair 包含 Access Token 和 Refresh Token
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresAt    int64  `json:"expires_at"`
}

// JWT 管理结构体
type JWT struct {
	SigningKey             []byte // 签名密钥
	TokenExpirationStr     string // Access Token 有效期字符串
	RefreshTokenExpiration string // Refresh Token 有效期字符串
	tokenExpiration        time.Duration
	refreshTokenExpiration time.Duration
}

// NewJWT 创建 JWT 管理器实例
func NewJWT(signingKey, tokenExp, refreshTokenExp string) (*JWT, error) {
	jwtManager := &JWT{
		SigningKey:             []byte(signingKey),
		TokenExpirationStr:     tokenExp,
		RefreshTokenExpiration: refreshTokenExp,
	}

	// 解析时间字符串
	tokenExpDur, err := parseDuration(tokenExp)
	if err != nil {
		return nil, fmt.Errorf("invalid token expiration: %w", err)
	}

	refreshTokenExpDur, err := parseDuration(refreshTokenExp)
	if err != nil {
		return nil, fmt.Errorf("invalid refresh token expiration: %w", err)
	}

	jwtManager.tokenExpiration = tokenExpDur
	jwtManager.refreshTokenExpiration = refreshTokenExpDur
	return jwtManager, nil
}

// DefaultJWT 创建默认配置的 JWT 管理器
func DefaultJWT() (*JWT, error) {
	return NewJWT("your-secret-key", "24h", "30d")
}

// parseDuration 解析时间格式字符串为time.Duration
func parseDuration(s string) (time.Duration, error) {
	if s == "" {
		return 0, errors.New("empty duration string")
	}

	// 支持的时间单位
	units := map[string]time.Duration{
		"s": time.Second,
		"m": time.Minute,
		"h": time.Hour,
		"d": time.Hour * 24,
		"w": time.Hour * 24 * 7,
	}

	// 提取数字和单位
	numStr := ""
	unit := ""
	for _, char := range s {
		if char >= '0' && char <= '9' || char == '.' {
			numStr += string(char)
		} else {
			unit += string(char)
		}
	}

	if numStr == "" || unit == "" {
		return 0, fmt.Errorf("invalid duration format: %s", s)
	}

	// 解析数字
	num, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid number in duration: %s", s)
	}

	// 解析单位
	dur, ok := units[strings.ToLower(unit)]
	if !ok {
		return 0, fmt.Errorf("unknown unit in duration: %s", unit)
	}

	return time.Duration(num) * dur, nil
}

// GenerateToken 生成 JWT token 和 refresh token
func (j *JWT) GenerateToken(userID, username, roleKey string) (*TokenPair, error) {
	// 生成 Access Token
	accessClaims := Claims{
		Identity: userID,
		Nice:     username,
		RoleKey:  roleKey,
		Type:     AccessTokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(j.tokenExpiration)),
		},
	}
	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	accessTokenStr, err := accessToken.SignedString(j.SigningKey)
	if err != nil {
		return nil, fmt.Errorf("generate access token failed: %w", err)
	}

	// 生成 Refresh Token
	refreshClaims := Claims{
		Identity: userID,
		Nice:     username,
		RoleKey:  roleKey,
		Type:     RefreshTokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(j.refreshTokenExpiration)),
		},
	}
	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refreshTokenStr, err := refreshToken.SignedString(j.SigningKey)
	if err != nil {
		return nil, fmt.Errorf("generate refresh token failed: %w", err)
	}

	return &TokenPair{
		AccessToken:  accessTokenStr,
		RefreshToken: refreshTokenStr,
		ExpiresAt:    time.Now().Add(j.tokenExpiration).Unix(),
	}, nil
}

// ParseToken 解析 JWT token
func (j *JWT) ParseToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return j.SigningKey, nil
	})

	if err != nil {
		return nil, fmt.Errorf("parse token failed: %w", err)
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid token")
}

// RefreshToken 使用 refresh token 刷新 access token
func (j *JWT) RefreshToken(refreshTokenStr string) (*TokenPair, error) {
	// 解析 Refresh Token
	claims, err := j.ParseToken(refreshTokenStr)
	if err != nil {
		return nil, fmt.Errorf("invalid refresh token: %w", err)
	}

	// 验证 token 类型
	if claims.Type != RefreshTokenType {
		return nil, errors.New("not a refresh token")
	}

	// 生成新的 Token 对
	return j.GenerateToken(claims.Identity, claims.Nice, claims.RoleKey)
}

// ContextClaims 从上下文中提取 JWT 声明
func (j *JWT) ContextClaims(c *app.RequestContext) (*Claims, error) {
	claims, exists := c.Get(ClaimsKey)
	if !exists {
		return nil, errors.New("jwt claims not found in context")
	}

	return claims.(*Claims), nil
}

// UserInfo 从上下文中获取用户信息
type UserInfo struct {
	UserID   string
	Username string
	RoleKey  string
}

// ExtractUserInfo 从上下文中提取用户信息
func (j *JWT) ExtractUserInfo(c *app.RequestContext) (*UserInfo, error) {
	claims, err := j.ContextClaims(c)
	if err != nil {
		return nil, err
	}

	return &UserInfo{
		UserID:   claims.Identity,
		Username: claims.Nice,
		RoleKey:  claims.RoleKey,
	}, nil
}

// GetUserID 获取用户ID
func (j *JWT) GetUserID(c *app.RequestContext) (string, error) {
	claims, err := j.ContextClaims(c)
	if err != nil {
		return "", err
	}
	return claims.Identity, nil
}

// GetUserIDInt64 获取用户ID（int64类型）
func (j *JWT) GetUserIDInt64(c *app.RequestContext) (int64, error) {
	userID, err := j.GetUserID(c)
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(userID, 10, 64)
}

// GetUserIDUint64 获取用户ID（uint64类型）
func (j *JWT) GetUserIDUint64(c *app.RequestContext) (uint64, error) {
	userID, err := j.GetUserID(c)
	if err != nil {
		return 0, err
	}
	return strconv.ParseUint(userID, 10, 64)
}

// GetUsername 获取用户名
func (j *JWT) GetUsername(c *app.RequestContext) (string, error) {
	claims, err := j.ContextClaims(c)
	if err != nil {
		return "", err
	}
	return claims.Nice, nil
}

// GetRoleKey 获取角色标识
func (j *JWT) GetRoleKey(c *app.RequestContext) (string, error) {
	claims, err := j.ContextClaims(c)
	if err != nil {
		return "", err
	}
	return claims.RoleKey, nil
}
