package crypter

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Argon2Hasher 基于 Argon2id 的密码哈希实现（PasswordHasher 的默认策略）。
// 相比 HMAC-SHA384 + 固定 salt 的旧方案，Argon2id 为每次哈希生成独立的
// 随机 salt，并通过内存/迭代成本参数提高离线暴力破解的代价。
type Argon2Hasher struct {
	// memory 单次哈希占用的内存大小（KiB）
	memory uint32
	// iterations 迭代次数（时间成本）
	iterations uint32
	// parallelism 并行计算的线程数
	parallelism uint8
	// saltLength 随机 salt 长度（字节）
	saltLength uint32
	// keyLength 输出哈希长度（字节）
	keyLength uint32
}

// NewArgon2Hasher 创建默认参数的 Argon2id 哈希器（64MB 内存、3 次迭代、2 线程）。
// 这组参数是 OWASP 推荐的最低基线，可在压测后按机器资源调整。
func NewArgon2Hasher() *Argon2Hasher {
	return &Argon2Hasher{
		memory:      64 * 1024,
		iterations:  3,
		parallelism: 2,
		saltLength:  16,
		keyLength:   32,
	}
}

// Hash 生成形如 $argon2id$v=19$m=65536,t=3,p=2$<salt>$<hash> 的自描述哈希串。
// 自描述格式意味着即使今后调整参数，旧哈希依然可以被正确 Verify。
func (h *Argon2Hasher) Hash(plain string) (string, error) {
	salt := make([]byte, h.saltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt failed: %w", err)
	}

	sum := argon2.IDKey([]byte(plain), salt, h.iterations, h.memory, h.parallelism, h.keyLength)

	encodedSalt := base64.RawStdEncoding.EncodeToString(salt)
	encodedSum := base64.RawStdEncoding.EncodeToString(sum)

	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, h.memory, h.iterations, h.parallelism, encodedSalt, encodedSum), nil
}

// Verify 校验明文密码与已存储的 Argon2id 哈希是否匹配。
func (h *Argon2Hasher) Verify(plain, hashed string) bool {
	params, salt, sum, err := decodeArgon2Hash(hashed)
	if err != nil {
		return false
	}

	candidate := argon2.IDKey([]byte(plain), salt, params.iterations, params.memory, params.parallelism, uint32(len(sum)))
	return subtle.ConstantTimeCompare(candidate, sum) == 1
}

type argon2Params struct {
	memory      uint32
	iterations  uint32
	parallelism uint8
}

// decodeArgon2Hash 解析自描述的 Argon2id 哈希串，还原出参数、salt 与哈希值。
func decodeArgon2Hash(encoded string) (*argon2Params, []byte, []byte, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return nil, nil, nil, fmt.Errorf("invalid argon2id hash format")
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return nil, nil, nil, fmt.Errorf("invalid argon2id version segment: %w", err)
	}
	if version != argon2.Version {
		return nil, nil, nil, fmt.Errorf("unsupported argon2id version: %d", version)
	}

	params := &argon2Params{}
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &params.memory, &params.iterations, &params.parallelism); err != nil {
		return nil, nil, nil, fmt.Errorf("invalid argon2id params segment: %w", err)
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return nil, nil, nil, fmt.Errorf("invalid argon2id salt segment: %w", err)
	}

	sum, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return nil, nil, nil, fmt.Errorf("invalid argon2id hash segment: %w", err)
	}

	return params, salt, sum, nil
}
