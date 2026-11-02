package authz

import (
	"testing"

	"feedchamber/internal/model"
	"feedchamber/internal/pkg"

	"github.com/stretchr/testify/assert"
)

// fakePerms 是角色 → 权限码的内存替身，与 service.PermissionService 同语义。
type fakePerms map[string]map[string]bool

func (f fakePerms) HasPermission(role, code string) bool { return f[role][code] }

func permsWith(role, code string) fakePerms {
	return fakePerms{role: {code: true}}
}

// 会话凭证：只看角色权限表。
func TestAllowedSessionUsesRoleTable(t *testing.T) {
	perms := permsWith("admin", model.PermCustomizeWrite)

	assert.True(t, Allowed(Credential{Method: "", Role: "admin"}, perms, model.PermCustomizeWrite))
	assert.False(t, Allowed(Credential{Method: "", Role: "user"}, perms, model.PermCustomizeWrite))
}

// API Key：scope 与角色必须同时覆盖。这是与 MCP 旧实现（scope 或角色）的关键差异。
func TestAllowedAPIKeyRequiresScopeAndRole(t *testing.T) {
	perms := permsWith("admin", model.PermPostCreate)

	t.Run("角色有但 API Key scope 没有 → 拒绝", func(t *testing.T) {
		assert.False(t, Allowed(Credential{
			Method: pkg.AuthMethodAPIKey,
			Scopes: "read",
			Role:   "admin",
		}, perms, model.PermPostCreate))
	})

	t.Run("scope 与角色都有 → 放行", func(t *testing.T) {
		assert.True(t, Allowed(Credential{
			Method: pkg.AuthMethodAPIKey,
			Scopes: "write",
			Role:   "admin",
		}, perms, model.PermPostCreate))
	})
}

// 粗粒度 scope 必须按注册表展开（read/write/admin → 一组精确码）。
func TestAllowedAPIKeyExpandsCoarseScopes(t *testing.T) {
	perms := permsWith("user", model.PermPostRead)

	assert.True(t, Allowed(Credential{
		Method: pkg.AuthMethodAPIKey,
		Scopes: "read",
		Role:   "user",
	}, perms, model.PermPostRead))

	assert.False(t, Allowed(Credential{
		Method: pkg.AuthMethodAPIKey,
		Scopes: "read",
		Role:   "user",
	}, perms, model.PermPostCreate))
}

// 精确码可以直接当 scope 用（与 gateway 既有约定一致）。
func TestAllowedAPIKeyAcceptsExactCodeAsScope(t *testing.T) {
	perms := permsWith("user", model.PermCustomizeWrite)

	assert.True(t, Allowed(Credential{
		Method: pkg.AuthMethodAPIKey,
		Scopes: "post:read," + model.PermCustomizeWrite,
		Role:   "user",
	}, perms, model.PermCustomizeWrite))
}

// OAuth 客户端凭证不是用户：只按 scope 判定，不查角色权限表。
func TestAllowedOAuthClientUsesScopeOnly(t *testing.T) {
	empty := fakePerms{}

	assert.True(t, Allowed(Credential{
		Method: pkg.AuthMethodOAuthClient,
		Scopes: "read",
	}, empty, model.PermPostRead))

	assert.False(t, Allowed(Credential{
		Method: pkg.AuthMethodOAuthClient,
		Scopes: "read",
	}, empty, model.PermPostCreate))
}

// 空权限码表示「登录即可」，任何凭证来源都放行。
func TestAllowedEmptyCodeAlwaysTrue(t *testing.T) {
	assert.True(t, Allowed(Credential{}, fakePerms{}, ""))
}

// 未知凭证来源必须拒绝，避免新增来源时静默放行。
func TestAllowedUnknownMethodDenies(t *testing.T) {
	assert.False(t, Allowed(Credential{Method: "magic", Role: "admin"},
		permsWith("admin", model.PermCustomizeWrite), model.PermCustomizeWrite))
}

// ScopeHas 供「只认凭证 scope」的场景使用（如 customize:publish 的机器判定）。
func TestScopeHasIgnoresRole(t *testing.T) {
	assert.True(t, ScopeHas("post:read,"+model.PermCustomizePublish, model.PermCustomizePublish))
	assert.False(t, ScopeHas("write", model.PermCustomizePublish))
}
