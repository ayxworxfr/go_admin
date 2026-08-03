package jwtauth

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	// AccessTokenType 表示 Access Token 类型
	AccessTokenType = "access"
	// RefreshTokenType 表示 Refresh Token 类型
	RefreshTokenType = "refresh"
	// ClaimsKey 表示 JWT 载荷的键名，由鉴权中间件写入 RequestContext
	ClaimsKey = "jwt_claims"
)

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

// JWT 管理 Access/Refresh Token 的签发、解析与刷新。
//
// 签名密钥与过期时长只在 NewJWT 构造期设置，实例构造完成后不再变化，
// 因此可以安全地在多个 goroutine 间共享同一个 *JWT（本项目就是这样用：
// 由 Container 构造一次，注入给 AuthHandler/AuthService/JWTAuthMiddleware）。
// 字段全部不导出：签名密钥属于敏感信息，不应该作为公开字段暴露给持有者随意读取。
type JWT struct {
	signingKey             []byte
	tokenExpiration        time.Duration
	refreshTokenExpiration time.Duration
}

// NewJWT 创建 JWT 管理器实例。tokenExp/refreshTokenExp 支持 s/m/h/d/w 单位
// （如 "24h"、"30d"），解析失败会返回错误，不会构造出一个状态不完整的实例。
func NewJWT(signingKey, tokenExp, refreshTokenExp string) (*JWT, error) {
	tokenExpDur, err := parseDuration(tokenExp)
	if err != nil {
		return nil, fmt.Errorf("invalid token expiration: %w", err)
	}

	refreshTokenExpDur, err := parseDuration(refreshTokenExp)
	if err != nil {
		return nil, fmt.Errorf("invalid refresh token expiration: %w", err)
	}

	return &JWT{
		signingKey:             []byte(signingKey),
		tokenExpiration:        tokenExpDur,
		refreshTokenExpiration: refreshTokenExpDur,
	}, nil
}

// durationUnits 是 parseDuration 支持的单位，d/w 是对 time.ParseDuration 的补充
// （标准库不认识"天"“周”，但配置文件里这样写最直观）。
var durationUnits = map[string]time.Duration{
	"s": time.Second,
	"m": time.Minute,
	"h": time.Hour,
	"d": time.Hour * 24,
	"w": time.Hour * 24 * 7,
}

// durationPattern 匹配「数字（可带小数）+ 单位字母」，如 "24h"、"1.5d"。
var durationPattern = regexp.MustCompile(`^(\d+(?:\.\d+)?)([a-zA-Z]+)$`)

// parseDuration 解析时间格式字符串为 time.Duration。
func parseDuration(s string) (time.Duration, error) {
	if s == "" {
		return 0, errors.New("empty duration string")
	}

	matches := durationPattern.FindStringSubmatch(s)
	if matches == nil {
		return 0, fmt.Errorf("invalid duration format: %s", s)
	}

	num, err := strconv.ParseFloat(matches[1], 64)
	if err != nil {
		return 0, fmt.Errorf("invalid number in duration: %s", s)
	}

	unit, ok := durationUnits[strings.ToLower(matches[2])]
	if !ok {
		return 0, fmt.Errorf("unknown unit in duration: %s", matches[2])
	}

	// 先转成 float64 再相乘、最后才转回 Duration：避免 time.Duration(num) 提前把
	// 小数部分截断（例如 "1.5h" 曾经被错误地算成 1h 而不是 1h30m）。
	return time.Duration(num * float64(unit)), nil
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
			ID:        uuid.NewString(), // jti，供 TokenStore 按需撤销
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(j.tokenExpiration)),
		},
	}
	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	accessTokenStr, err := accessToken.SignedString(j.signingKey)
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
			ID:        uuid.NewString(),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(j.refreshTokenExpiration)),
		},
	}
	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refreshTokenStr, err := refreshToken.SignedString(j.signingKey)
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
		return j.signingKey, nil
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
