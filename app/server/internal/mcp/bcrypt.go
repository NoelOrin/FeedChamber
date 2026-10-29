package mcp

import "golang.org/x/crypto/bcrypt"

// defaultVerify 是 API Key 的密码哈希校验。
//
// gateway/apikey.go 用同一个函数（bcrypt.CompareHashAndPassword）校验，
// 这里保持完全一致：两处若用了不同的比较方式，同一个 Key 会在一边通过、
// 另一边失败，症状极难排查。
func defaultVerify(hash, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}

// VerifyFunc 允许测试替换 bcrypt 校验，避免每个用例都付一次 bcrypt 成本。
type VerifyFunc = VerifyPassword
