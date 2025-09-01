package crypter

import (
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
)

// 服务名称+salt
const CRYPTER_KEY = "ServerName" + "@2025"

type SHA384Crypter struct {
	key []byte
}

// NewSHA384Crypter 创建新的 SHA-384 加密器
func NewSHA384Crypter(key string) *SHA384Crypter {
	return &SHA384Crypter{key: []byte(key)}
}

// Encrypt 使用 HMAC-SHA-384 加密密码
func (c *SHA384Crypter) Encrypt(password string) string {
	h := hmac.New(sha512.New384, c.key)
	h.Write([]byte(password))
	return hex.EncodeToString(h.Sum(nil))
}

// Verify 验证密码是否匹配加密值
func (c *SHA384Crypter) Verify(password, encryptedPassword string) bool {
	return c.Encrypt(password) == encryptedPassword
}
