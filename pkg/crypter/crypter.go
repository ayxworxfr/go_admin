package crypter

import (
	"fmt"
	"os"
)

var Instance *SHA384Crypter

func init() {
	Instance = NewSHA384Crypter(CRYPTER_KEY)
}

func InitHybridCrypter() *HybridCrypter {
	// 从环境变量获取密钥文件路径，默认为当前目录下的crypter.key
	keyPath := os.Getenv("CRYPTER_KEY_PATH")
	if keyPath == "" {
		keyPath = "crypter.key"
	}

	var err error
	instance, err := NewHybridCrypter(keyPath)
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize crypter: %v", err))
	}
	return instance
}
