package crypter

import (
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"testing"
)

func TestSHA384Crypter_Encrypt(t *testing.T) {
	tests := []struct {
		name           string
		key            string
		password       string
		expectedLength int
	}{
		{
			name:           "正常密码加密",
			key:            CRYPTER_KEY,
			password:       "testPassword123",
			expectedLength: 96, // SHA-384 哈希值的十六进制表示长度为 96
		},
		{
			name:           "空密码加密",
			key:            CRYPTER_KEY,
			password:       "",
			expectedLength: 96,
		},
		{
			name:           "不同密钥相同密码",
			key:            "differentKey@2025",
			password:       "testPassword123",
			expectedLength: 96,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			crypter := NewSHA384Crypter(tt.key)
			encrypted := crypter.Encrypt(tt.password)

			// 验证长度是否符合 SHA-384 的十六进制输出
			if len(encrypted) != tt.expectedLength {
				t.Errorf("加密后的字符串长度应为 %d，但得到 %d", tt.expectedLength, len(encrypted))
			}

			// 验证加密是否使用 HMAC-SHA-384
			expectedMAC := hmac.New(sha512.New384, []byte(tt.key))
			expectedMAC.Write([]byte(tt.password))
			expectedEncrypted := hex.EncodeToString(expectedMAC.Sum(nil))

			if encrypted != expectedEncrypted {
				t.Errorf("加密结果不匹配\ngot:  %s\nwant: %s", encrypted, expectedEncrypted)
			}
		})
	}
}

func TestSHA384Crypter_Verify(t *testing.T) {
	tests := []struct {
		name        string
		key         string
		correctPwd  string
		testPwd     string
		expectValid bool
	}{
		{
			name:        "正确密码验证",
			key:         CRYPTER_KEY,
			correctPwd:  "testPassword123",
			testPwd:     "testPassword123",
			expectValid: true,
		},
		{
			name:        "错误密码验证",
			key:         CRYPTER_KEY,
			correctPwd:  "testPassword123",
			testPwd:     "wrongPassword",
			expectValid: false,
		},
		{
			name:        "空密码验证",
			key:         CRYPTER_KEY,
			correctPwd:  "",
			testPwd:     "",
			expectValid: true,
		},
		{
			name:        "不同密钥的验证",
			key:         "differentKey@2025",
			correctPwd:  "testPassword123",
			testPwd:     "testPassword123",
			expectValid: false, // 使用不同密钥加密的密码应该验证失败
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 创建使用 CRYPTER_KEY 的加密器
			crypter := NewSHA384Crypter(CRYPTER_KEY)
			// 使用正确的密码生成加密值
			encrypted := crypter.Encrypt(tt.correctPwd)
			if tt.key != CRYPTER_KEY {
				newCrypter := NewSHA384Crypter(tt.key)
				encrypted = newCrypter.Encrypt(tt.correctPwd)
			}

			// 验证测试密码
			isValid := crypter.Verify(tt.testPwd, encrypted)

			if isValid != tt.expectValid {
				t.Errorf("密码验证结果不符合预期：期望 %v，但得到 %v", tt.expectValid, isValid)
			}

			// 不同密钥的验证测试
			if tt.name == "不同密钥的验证" {
				// 创建使用不同密钥的加密器
				differentCrypter := NewSHA384Crypter(tt.key)
				// 使用不同密钥加密相同密码
				differentEncrypted := differentCrypter.Encrypt(tt.correctPwd)

				// 验证：使用原始密钥的加密器应该无法验证不同密钥加密的结果
				if crypter.Verify(tt.correctPwd, differentEncrypted) {
					t.Errorf("使用不同密钥加密的密码验证通过")
				}

				// 额外验证：使用正确密钥的加密器应该能验证自己加密的结果
				if !differentCrypter.Verify(tt.correctPwd, differentEncrypted) {
					t.Errorf("使用相同密钥加密的密码验证失败")
				}
			}
		})
	}
}

func TestHMACConsistency(t *testing.T) {
	// 测试同一密码多次加密结果是否一致
	crypter := NewSHA384Crypter(CRYPTER_KEY)
	password := "consistentTest123"

	encrypted1 := crypter.Encrypt(password)
	encrypted2 := crypter.Encrypt(password)

	if encrypted1 != encrypted2 {
		t.Errorf("同一密码两次加密结果不一致\ngot:  %s\nwant: %s", encrypted1, encrypted2)
	}
}

func TestHMACKeyImpact(t *testing.T) {
	// 测试不同密钥对同一密码的加密结果是否不同
	crypter1 := NewSHA384Crypter("key1")
	crypter2 := NewSHA384Crypter("key2")
	password := "samePassword"

	encrypted1 := crypter1.Encrypt(password)
	encrypted2 := crypter2.Encrypt(password)

	if encrypted1 == encrypted2 {
		t.Errorf("不同密钥对同一密码的加密结果相同")
	}
}
