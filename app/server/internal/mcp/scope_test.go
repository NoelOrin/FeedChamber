package mcp

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAPIKeyScopeRegistryExcludesCustomize 是任务 9.1 的「权限提升」回归测试，
// 也是本变更里唯一一条「做错就是安全漏洞」的断言。
//
// 背景：gateway/apiKeyScopeRegistry 把粗粒度 scope（read / write / admin）
// 映射到权限码集合。若把 customize:read / write / publish 中任意一个追加进
// 任何既有 scope，那么**所有存量持该 scope 的 API Key 会在部署瞬间获得
// 向全站注入 JS 的能力**——这是权限提升，且不会有任何报错提示。
//
// 用源码扫描而不是 import 断言：registry 是 gateway 包的包级私有变量，
// 从 mcp 包无法直接读到；而这条约束的本质是「那段源码里不该出现这些字符串」，
// 扫描文本恰好直接表达这个语义。
func TestAPIKeyScopeRegistryExcludesCustomize(t *testing.T) {
	path := filepath.Join("..", "gateway", "apikey.go")
	raw, err := os.ReadFile(path)
	require.NoError(t, err, "读取 gateway/apikey.go 失败：%s", path)

	src := string(raw)

	// 截取 apiKeyScopeRegistry 的声明块：从 `apiKeyScopeRegistry = map` 到
	// 它闭合的 `}`。用括号配平而不是正则贪婪匹配，避免跨过整份文件。
	start := strings.Index(src, "apiKeyScopeRegistry = map")
	require.Greater(t, start, -1, "没找到 apiKeyScopeRegistry 声明，测试需要同步更新")

	registry := extractBalancedBraces(src[start:])
	require.NotEmpty(t, registry, "提取 registry 块失败")

	for _, scope := range []string{"customize:read", "customize:write", "customize:publish"} {
		assert.NotContains(t, registry, scope,
			"apiKeyScopeRegistry 不得包含 %q：并入任何既有 scope 等于让所有存量 Key 获得注入 JS 的能力", scope)
	}

	// 同时确认三个既有 scope 仍在那张表里——否则说明这次改动把整张表改坏了，
	// 上面的 NotContains 会因为「什么都没匹配到」而假绿。
	for _, expected := range []string{`"read"`, `"write"`, `"admin"`} {
		assert.Contains(t, registry, expected, "registry 应仍包含 %s scope", expected)
	}
}

// extractBalancedBraces 从 s 中第一个 '{' 开始，返回配平到对应 '}' 的子串。
func extractBalancedBraces(s string) string {
	start := strings.Index(s, "{")
	if start < 0 {
		return ""
	}
	depth := 0
	for i := start; i < len(s); i++ {
		switch s[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}
	return ""
}

// TestCustomizePermissionsNotGrantedToNonAdminRoles 确认自定义权限只给 admin。
//
// 模型层里 customize:* 若被加进 moderator 或 user 角色，等于把「注入任意 JS」
// 的能力下放给非全权角色。
func TestCustomizePermissionsNotGrantedToNonAdminRoles(t *testing.T) {
	path := filepath.Join("..", "model", "permission.go")
	raw, err := os.ReadFile(path)
	require.NoError(t, err)

	src := string(raw)

	// 找到 DefaultRolePermissions 里每个角色的权限列表块。
	for _, role := range []string{"moderator", "user", "ban", "pending_user"} {
		block := roleBlock(src, role)
		if block == "" {
			continue // 该角色可能不存在于表里
		}
		assert.NotContains(t, block, "PermCustomize",
			"角色 %q 不应持有 customize:* 权限", role)
	}
}

// roleBlock 取出 `"role": {` 到配平 `}` 的内容。
func roleBlock(src, role string) string {
	marker := regexp.MustCompile(`"` + regexp.QuoteMeta(role) + `":\s*\{`)
	loc := marker.FindStringIndex(src)
	if loc == nil {
		return ""
	}
	return extractBalancedBraces(src[loc[0]:])
}
