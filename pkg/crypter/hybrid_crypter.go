package crypter

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

// HybridCrypter 混合加密器
type HybridCrypter struct {
	eccCrypter *ECCCrypter
	aesCrypter *AESCrypter
	keyPath    string // 存储密钥的路径
}

// KeyFile 密钥文件结构
type KeyFile struct {
	PrivateKey string `json:"private_key"`
	AESKey     string `json:"aes_key"`
}

// NewHybridCrypter 创建混合加密器
func NewHybridCrypter(keyPath string) (*HybridCrypter, error) {
	h := &HybridCrypter{keyPath: keyPath}

	// 检查密钥文件是否存在
	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		// 创建新密钥
		return h.createNewKeys()
	}

	// 加载现有密钥
	return h.loadExistingKeys()
}

// createNewKeys 创建新的密钥对
func (h *HybridCrypter) createNewKeys() (*HybridCrypter, error) {
	// 创建ECC加密器
	ecc, err := NewECCCrypter()
	if err != nil {
		return nil, err
	}

	// 生成随机AES密钥
	aesKey := make([]byte, 32) // 256位密钥
	if _, err := rand.Read(aesKey); err != nil {
		return nil, err
	}

	aes := NewAESCrypter(aesKey)

	h.eccCrypter = ecc
	h.aesCrypter = aes

	// 保存密钥
	if err := h.saveKeys(); err != nil {
		return nil, err
	}

	return h, nil
}

// loadExistingKeys 从文件加载现有密钥
func (h *HybridCrypter) loadExistingKeys() (*HybridCrypter, error) {
	data, err := os.ReadFile(h.keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read key file: %v", err)
	}

	var keyFile KeyFile
	if err := json.Unmarshal(data, &keyFile); err != nil {
		return nil, fmt.Errorf("failed to parse key file: %v", err)
	}

	// 恢复ECC私钥
	privateKeyBytes, err := base64.StdEncoding.DecodeString(keyFile.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decode ECC private key: %v", err)
	}

	curve := ecdh.P256()
	privateKey, err := curve.NewPrivateKey(privateKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to create ECC private key: %v", err)
	}

	publicKey := privateKey.PublicKey()

	ecc := &ECCCrypter{
		curve:      curve,
		privateKey: privateKey,
		publicKey:  publicKey,
	}

	// 恢复AES密钥
	aesKey, err := base64.StdEncoding.DecodeString(keyFile.AESKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decode AES key: %v", err)
	}

	aes := NewAESCrypter(aesKey)

	h.eccCrypter = ecc
	h.aesCrypter = aes

	return h, nil
}

// saveKeys 保存密钥到文件
func (h *HybridCrypter) saveKeys() error {
	// 保存ECC私钥
	privateKeyBytes := h.eccCrypter.privateKey.Bytes()
	privateKeyBase64 := base64.StdEncoding.EncodeToString(privateKeyBytes)

	// 保存AES密钥
	aesKeyBase64 := base64.StdEncoding.EncodeToString(h.aesCrypter.key)

	keyFile := KeyFile{
		PrivateKey: privateKeyBase64,
		AESKey:     aesKeyBase64,
	}

	// 序列化为JSON
	data, err := json.Marshal(keyFile)
	if err != nil {
		return fmt.Errorf("failed to serialize keys: %v", err)
	}

	// 写入文件
	if err := os.WriteFile(h.keyPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write key file: %v", err)
	}

	return nil
}

// Encrypt 加密明文
func (h *HybridCrypter) Encrypt(plaintext string) (string, error) {
	// 使用AES加密数据
	ciphertext, err := h.aesCrypter.Encrypt([]byte(plaintext))
	if err != nil {
		return "", err
	}

	// 使用ECC加密AES密钥
	encryptedKey, err := h.eccCrypter.Encrypt(string(h.aesCrypter.key))
	if err != nil {
		return "", err
	}

	// 组合加密后的密钥和数据
	result := encryptedKey + ":" + base64.StdEncoding.EncodeToString(ciphertext)
	return result, nil
}

// Decrypt 解密密文
func (h *HybridCrypter) Decrypt(ciphertext string) (string, error) {
	// 分离加密后的密钥和数据
	parts := strings.SplitN(ciphertext, ":", 2)
	if len(parts) != 2 {
		return "", errors.New("invalid ciphertext format")
	}

	// 解密AES密钥
	decryptedKey, err := h.eccCrypter.Decrypt(parts[0])
	if err != nil {
		return "", err
	}

	// 创建新的AES加密器
	tempAES := NewAESCrypter([]byte(decryptedKey))

	// 解码并解密数据
	decodedCiphertext, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return "", err
	}

	decryptedData, err := tempAES.Decrypt(decodedCiphertext)
	if err != nil {
		return "", err
	}

	return string(decryptedData), nil
}
