package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// R-04 的两半都必须成立：
//  1. 升级时新增的权限码要补到内置角色上（否则功能不出现且无报错）；
//  2. 管理员撤销过的默认权限**不得**在重启时复活。
func TestRolePermissionTopUpOnlyAddsNewCodes(t *testing.T) {
	defaults := map[string][]string{
		"admin": {"post:read", "customize:read", "customize:write"},
		"user":  {"post:read", "post:create"},
	}

	t.Run("已应用过的码不再补（撤销不复活）", func(t *testing.T) {
		applied := ParseAppliedCodes("post:read,post:create,customize:read,customize:write")
		perRole, newly := RolePermissionTopUp(defaults, applied)
		assert.Empty(t, perRole, "全部码都已应用过，不应再补任何东西")
		assert.Empty(t, newly)
	})

	t.Run("新增码只补到缺它的角色上", func(t *testing.T) {
		applied := ParseAppliedCodes("post:read,post:create,customize:read")
		perRole, newly := RolePermissionTopUp(defaults, applied)
		assert.Equal(t, []string{"customize:write"}, perRole["admin"])
		assert.NotContains(t, perRole, "user", "user 的码都应用过，不该被改动")
		assert.Equal(t, []string{"customize:write"}, newly)
	})

	t.Run("多角色共享的新码只记一次", func(t *testing.T) {
		shared := map[string][]string{
			"admin": {"post:read", "new:code"},
			"user":  {"post:read", "new:code"},
		}
		perRole, newly := RolePermissionTopUp(shared, ParseAppliedCodes("post:read"))
		assert.Equal(t, []string{"new:code"}, perRole["admin"])
		assert.Equal(t, []string{"new:code"}, perRole["user"])
		require.Len(t, newly, 1, "同一个码不应被记录两次")
	})
}

// 合并后的标记串必须稳定、可回读、无重复。
func TestMergeAppliedCodesIsStableAndDeduped(t *testing.T) {
	applied := ParseAppliedCodes("a,b")
	merged := MergeAppliedCodes(applied, []string{"b", "c", "a"})
	assert.Equal(t, "a,b,c", merged)
	assert.Equal(t, map[string]struct{}{"a": {}, "b": {}, "c": {}}, ParseAppliedCodes(merged))
}

func TestParseAppliedCodesTolerant(t *testing.T) {
	got := ParseAppliedCodes(" , post:read ,, user:update ,")
	assert.Len(t, got, 2)
	assert.Contains(t, got, "post:read")
	assert.Contains(t, got, "user:update")
}
