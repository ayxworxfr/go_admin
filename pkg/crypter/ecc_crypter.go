package crypter

import (
	"crypto/ecdh"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
)

// ECCCrypter 使用ECDH进行密钥交换和加密
type ECCCrypter struct {
	curve      ecdh.Curve
	privateKey *ecdh.PrivateKey
	publicKey  *ecdh.PublicKey
}

// NewECCCrypter 创建新的ECC加密器
func NewECCCrypter() (*ECCCrypter, error) {
	curve := ecdh.P256()
	privateKey, err := curve.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}

	publicKey := privateKey.PublicKey()

	return &ECCCrypter{
		curve:      curve,
		privateKey: privateKey,
		publicKey:  publicKey,
	}, nil
}

// Encrypt 使用ECC公钥加密数据
func (c *ECCCrypter) Encrypt(plaintext string) (string, error) {
	message := []byte(plaintext)

	ephemeralPrivateKey, err := c.curve.GenerateKey(rand.Reader)
	if err != nil {
		return "", err
	}

	sharedSecret, err := ephemeralPrivateKey.ECDH(c.publicKey)
	if err != nil {
		return "", err
	}

	sharedKey := sha256.Sum256(sharedSecret)

	ciphertext := make([]byte, len(message))
	for i := 0; i < len(message); i++ {
		ciphertext[i] = message[i] ^ sharedKey[i%32]
	}

	ephemeralPublicKey := ephemeralPrivateKey.PublicKey()
	result := ephemeralPublicKey.Bytes()
	result = append(result, ciphertext...)

	return base64.StdEncoding.EncodeToString(result), nil
}

// Decrypt 使用ECC私钥解密数据
func (c *ECCCrypter) Decrypt(cryptoText string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(cryptoText)
	if err != nil {
		return "", err
	}

	keySize := len(c.publicKey.Bytes())

	if len(data) < keySize {
		return "", errors.New("invalid ciphertext")
	}

	ephemeralPublicKey, err := c.curve.NewPublicKey(data[:keySize])
	if err != nil {
		return "", err
	}

	sharedSecret, err := c.privateKey.ECDH(ephemeralPublicKey)
	if err != nil {
		return "", err
	}

	sharedKey := sha256.Sum256(sharedSecret)

	ciphertext := data[keySize:]
	plaintext := make([]byte, len(ciphertext))
	for i := 0; i < len(ciphertext); i++ {
		plaintext[i] = ciphertext[i] ^ sharedKey[i%32]
	}

	return string(plaintext), nil
}

// GetPublicKeyBase64 返回公钥的Base64编码
func (c *ECCCrypter) GetPublicKeyBase64() string {
	publicKeyBytes := c.publicKey.Bytes()
	return base64.StdEncoding.EncodeToString(publicKeyBytes)
}

// SetPublicKeyBase64 设置公钥
func (c *ECCCrypter) SetPublicKeyBase64(publicKeyBase64 string) error {
	publicKeyBytes, err := base64.StdEncoding.DecodeString(publicKeyBase64)
	if err != nil {
		return err
	}

	publicKey, err := c.curve.NewPublicKey(publicKeyBytes)
	if err != nil {
		return err
	}

	c.publicKey = publicKey
	return nil
}
