package dto

// LoginRequest 登录请求
type LoginRequest struct {
	Username string `json:"username" vd:"len($)>0"`
	Password string `json:"password" vd:"len($)>0"`
}

// RefreshTokenRequest 刷新令牌请求
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" vd:"len($)>0"`
}

// TokenResponse 令牌响应
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresAt    int64  `json:"expires_at"`
}

// LoginResult 登录结果，在令牌之外附带前端需要的登录态展示信息
type LoginResult struct {
	TokenResponse
	Status           string `json:"status"`
	Type             string `json:"type"`
	CurrentAuthority string `json:"currentAuthority"`
}
