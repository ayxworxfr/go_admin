package crypter

import (
	"strings"
	"testing"
)

func TestECCCrypter(t *testing.T) {
	// 创建一个新的ECCCrypter实例
	crypter, err := NewECCCrypter()
	if err != nil {
		t.Fatalf("Failed to create ECCCrypter: %v", err)
	}

	// 测试加密和解密
	t.Run("EncryptDecrypt", func(t *testing.T) {
		plaintext := "Hello, ECC encryption!"
		ciphertext, err := crypter.Encrypt(plaintext)
		if err != nil {
			t.Fatalf("Encryption failed: %v", err)
		}

		decrypted, err := crypter.Decrypt(ciphertext)
		if err != nil {
			t.Fatalf("Decryption failed: %v", err)
		}

		if decrypted != plaintext {
			t.Errorf("Decrypted text does not match original. Got %s, want %s", decrypted, plaintext)
		}
	})

	// 测试空字符串的加密和解密
	t.Run("EncryptDecryptEmptyString", func(t *testing.T) {
		plaintext := ""
		ciphertext, err := crypter.Encrypt(plaintext)
		if err != nil {
			t.Fatalf("Encryption of empty string failed: %v", err)
		}

		decrypted, err := crypter.Decrypt(ciphertext)
		if err != nil {
			t.Fatalf("Decryption of empty string failed: %v", err)
		}

		if decrypted != plaintext {
			t.Errorf("Decrypted empty string does not match. Got %s, want %s", decrypted, plaintext)
		}
	})

	// 测试公钥导出和导入
	t.Run("PublicKeyExportImport", func(t *testing.T) {
		publicKey := crypter.GetPublicKeyBase64()
		if publicKey == "" {
			t.Fatal("Failed to get public key")
		}

		newCrypter, err := NewECCCrypter()
		if err != nil {
			t.Fatalf("Failed to create new ECCCrypter: %v", err)
		}

		err = newCrypter.SetPublicKeyBase64(publicKey)
		if err != nil {
			t.Fatalf("Failed to set public key: %v", err)
		}

		// 使用新的crypter加密，原crypter解密
		plaintext := "Test public key import"
		ciphertext, err := newCrypter.Encrypt(plaintext)
		if err != nil {
			t.Fatalf("Encryption with imported public key failed: %v", err)
		}

		decrypted, err := crypter.Decrypt(ciphertext)
		if err != nil {
			t.Fatalf("Decryption after public key import failed: %v", err)
		}

		if decrypted != plaintext {
			t.Errorf("Decrypted text after public key import does not match. Got %s, want %s", decrypted, plaintext)
		}
	})

	// 测试长文本的加密和解密
	t.Run("EncryptDecryptLongText", func(t *testing.T) {
		plaintext := strings.Repeat("Long text for encryption test. ", 100)
		ciphertext, err := crypter.Encrypt(plaintext)
		if err != nil {
			t.Fatalf("Encryption of long text failed: %v", err)
		}

		decrypted, err := crypter.Decrypt(ciphertext)
		if err != nil {
			t.Fatalf("Decryption of long text failed: %v", err)
		}

		if decrypted != plaintext {
			t.Errorf("Decrypted long text does not match original. Got %d characters, want %d characters", len(decrypted), len(plaintext))
		}
	})

	// 测试无效的密文解密
	t.Run("DecryptInvalidCiphertext", func(t *testing.T) {
		invalidCiphertext := "ThisIsNotAValidCiphertext"
		_, err := crypter.Decrypt(invalidCiphertext)
		if err == nil {
			t.Error("Decryption of invalid ciphertext should fail, but it didn't")
		}
	})

	// 测试无效的公钥导入
	t.Run("ImportInvalidPublicKey", func(t *testing.T) {
		invalidPublicKey := "ThisIsNotAValidPublicKey"
		err := crypter.SetPublicKeyBase64(invalidPublicKey)
		if err == nil {
			t.Error("Setting invalid public key should fail, but it didn't")
		}
	})

	// 测试多次加密同一明文
	t.Run("MultipleEncryptions", func(t *testing.T) {
		plaintext := "Same plaintext"
		ciphertext1, err := crypter.Encrypt(plaintext)
		if err != nil {
			t.Fatalf("First encryption failed: %v", err)
		}

		ciphertext2, err := crypter.Encrypt(plaintext)
		if err != nil {
			t.Fatalf("Second encryption failed: %v", err)
		}

		if ciphertext1 == ciphertext2 {
			t.Error("Multiple encryptions of the same plaintext should produce different ciphertexts")
		}

		// 确保两个密文都能正确解密
		decrypted1, err := crypter.Decrypt(ciphertext1)
		if err != nil || decrypted1 != plaintext {
			t.Errorf("Decryption of first ciphertext failed or incorrect")
		}

		decrypted2, err := crypter.Decrypt(ciphertext2)
		if err != nil || decrypted2 != plaintext {
			t.Errorf("Decryption of second ciphertext failed or incorrect")
		}
	})
}
