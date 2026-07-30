package crypter

// PasswordHasher 密码哈希策略接口（Strategy Pattern）。
// 不同的哈希算法通过实现该接口互相替换，调用方（如用户模块的
// Service）只依赖这个 2 方法的小接口，不关心具体算法。
type PasswordHasher interface {
	// Hash 对明文密码生成哈希，返回值需自描述（包含算法参数），
	// 以便未来更换默认参数时旧哈希仍可被 Verify 正确校验。
	Hash(plain string) (string, error)
	// Verify 校验明文密码是否匹配已存储的哈希。
	Verify(plain, hashed string) bool
}
