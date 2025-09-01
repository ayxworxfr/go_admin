package crypter

import (
	"os"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	// 设置测试专用的密钥文件路径（例如临时文件或固定测试文件）
	os.Setenv("CRYPTER_KEY_PATH", "./test_crypter.key")

	// 运行测试
	code := m.Run()

	// 测试后清理（删除测试密钥文件）
	os.Remove("./test_crypter.key")

	// 退出测试
	os.Exit(code)
}

func TestHybridCrypter(t *testing.T) {
	testCases := []struct {
		name string
		data string
	}{
		{"Short text", "Hello, World!"},
		{"Empty string", ""},
		{"Long text", strings.Repeat("Long text for testing. ", 100)},
		{"Special characters", "!@#$%^&*()_+{}[]|\"':;?/>.<,~`"},
	}

	instance := InitHybridCrypter()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// 直接使用单例（依赖 test_main 中设置的环境变量）
			encrypted, err := instance.Encrypt(tc.data)
			if err != nil {
				t.Fatalf("Encryption failed: %v", err)
			}

			decrypted, err := instance.Decrypt(encrypted)
			if err != nil {
				t.Fatalf("Decryption failed: %v", err)
			}

			if decrypted != tc.data {
				t.Errorf("Decrypted data does not match original. Got %q, want %q", decrypted, tc.data)
			}
		})
	}
}

// 测试单例初始化
func TestSingletonInitialization(t *testing.T) {
	instance := InitHybridCrypter()

	// 测试单例加密解密功能
	testData := "Test singleton"
	encrypted, err := instance.Encrypt(testData)
	if err != nil {
		t.Fatalf("Encryption failed: %v", err)
	}

	decrypted, err := instance.Decrypt(encrypted)
	if err != nil {
		t.Fatalf("Decryption failed: %v", err)
	}

	if decrypted != testData {
		t.Errorf("Decrypted data does not match original. Got %q, want %q", decrypted, testData)
	}
}
