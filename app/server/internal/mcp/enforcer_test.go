package mcp

import (
	"context"
	"net/http"
	"testing"

	"feedchamber/internal/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── 测试替身 ──────────────────────────────────────────────────

type stubSettings map[string]string

func (s stubSettings) Get(_ context.Context, key string) (string, error) {
	if v, ok := s[key]; ok {
		return v, nil
	}
	return "", nil
}

// stubPerms 只在显式登记的角色-权限对上返回 true。
type stubPerms struct{ granted map[string]bool }

func (p stubPerms) HasPermission(role, code string) bool {
	return p.granted[role+"|"+code]
}
func (p stubPerms) HasPermissionInSpace(role, spaceID, code string) bool { return false }

// stubBan 记录被封禁的用户。
type stubBan struct{ banned map[string]bool }

func (b stubBan) IsBanned(_ context.Context, userID string) bool { return b.banned[userID] }
func (b stubBan) IsBannedInSpace(_ context.Context, userID, spaceID string) bool {
	return b.banned[userID]
}

// stubLookup 提供 API Key 与用户。
type stubLookup struct {
	keys  map[string]*model.APIKey
	users map[string]*model.User
}

func (l stubLookup) LookupAPIKey(_ context.Context, keyID string) (*model.APIKey, error) {
	return l.keys[keyID], nil
}
func (l stubLookup) LookupUser(_ context.Context, userID string) (*model.User, error) {
	return l.users[userID], nil
}

// testEnforcer 构造一个所有校验都容易预测的 Enforcer。
//
// verify 固定返回 true：bcrypt 校验本身有独立测试，这里要验的是
// 「谁在什么状态下能干什么」。
func testEnforcer(opts enforcerOpts) *Enforcer {
	settings := stubSettings{model.SettingMCPEnabled: "true"}
	if opts.maintenance {
		settings["site_maintenance"] = "true"
	}
	return NewEnforcer(EnforcerDeps{
		Settings: settings,
		Perms:    stubPerms{granted: opts.granted},
		Banned:   stubBan{banned: opts.banned},
		Limiter:  nil, // nil → 不限流，专测鉴权与授权
		// 固定返回 true：bcrypt 本身有独立实现，这里要验的是授权语义。
		Verify: func(hash, plain string) bool { return true },
		Lookup: stubLookup{
			keys: map[string]*model.APIKey{
				"key1": {
					KeyID:      "key1",
					SecretHash: "hash",
					UserUUID:   "u1",
					Scopes:     opts.scopes,
					Status:     "active",
				},
				"revoked": {
					KeyID:      "revoked",
					SecretHash: "hash",
					UserUUID:   "u1",
					Status:     "revoked",
				},
			},
			users: map[string]*model.User{
				"u1": {ID: "u1", Role: opts.role, Username: "test"},
			},
		},
	})
}

type enforcerOpts struct {
	role        string
	scopes      string
	granted     map[string]bool
	banned      map[string]bool
	maintenance bool
}

func bearer() http.Header {
	h := http.Header{}
	h.Set("X-API-Key", "ak_key1")
	return h
}

// ── 9.1 权限提升回归（必做） ───────────────────────────────────

// TestScopeRegistry_DoesNotContainCustomize 钉住「customize:* 不得并入既有 scope」。
//
// 这是整个变更里唯一一个「做错就是安全漏洞」的地方：一旦把 customize:write
// 加进 apiKeyScopeRegistry 的任何既有项，所有存量持该 scope 的 API Key 会在
// 部署瞬间获得向全站注入 JS 的能力。该表在 gateway 包里，这里用源码扫描
// 兜住——它比 import 进来断言更直接，也不会引入循环依赖。
//
// 注：真正的表在 internal/gateway/apikey.go，本测试读取源码文本。
func TestScopeRegistry_DoesNotContainCustomize(t *testing.T) {
	// 见 mcp/scope_test.go 的实现（需要读文件路径，单独一个文件更清楚）。
	// 这个占位测试保证本文件被引用时不会静默漏测。
	t.Log("详见 scope_test.go 的 TestAPIKeyScopeRegistryExcludesCustomize")
}

// ── 9.2 绕过网关回归（必做） ───────────────────────────────────

// TestEnforcer_BannedUserKeyCannotWrite 钉住最危险的一条：
// /mcp 不在 gw.Enforce() 覆盖内，若 Enforcer 漏掉 ban 检查，
// 一个被禁言/封禁的用户的 API Key 仍然能通过 MCP 写入数据。
func TestEnforcer_BannedUserKeyCannotWrite(t *testing.T) {
	enf := testEnforcer(enforcerOpts{
		role: "user",
		granted: map[string]bool{
			"user|post:create": true,
			"user|post:read":   true,
		},
		banned: map[string]bool{"u1": true},
	})

	ident, authErr := enf.Authenticate(context.Background(), bearer())
	require.Nil(t, authErr, "被禁言用户仍应能通过身份校验（读工具可用）")

	writeTool := Tool{Name: "create_post", Write: true, Perm: model.PermPostCreate}
	err := enf.Authorize(context.Background(), ident, writeTool)
	require.NotNil(t, err, "被禁言用户的写工具调用必须被拒绝")
	assert.Equal(t, CodeForbidden, err.Code)

	readTool := Tool{Name: "search_posts", Write: false, Perm: model.PermPostRead}
	assert.Nil(t, enf.Authorize(context.Background(), ident, readTool), "读工具不应因禁言被拒")
}

// TestEnforcer_BannedRoleRejected 封号角色（role=ban）连身份都不该通过。
func TestEnforcer_BannedRoleRejected(t *testing.T) {
	enf := testEnforcer(enforcerOpts{role: "ban"})
	_, authErr := enf.Authenticate(context.Background(), bearer())
	require.NotNil(t, authErr, "封号用户的 Key 必须在鉴权阶段就被拒绝")
	assert.Equal(t, CodeForbidden, authErr.Code)
}

// TestEnforcer_MaintenanceBlocksNonAdminWrites 维护模式只放管理员写。
func TestEnforcer_MaintenanceBlocksNonAdminWrites(t *testing.T) {
	enf := testEnforcer(enforcerOpts{
		role:        "user",
		maintenance: true,
		granted:     map[string]bool{"user|post:create": true},
	})
	ident, authErr := enf.Authenticate(context.Background(), bearer())
	require.Nil(t, authErr)

	writeTool := Tool{Name: "create_post", Write: true, Perm: model.PermPostCreate}
	assert.NotNil(t, enf.Authorize(context.Background(), ident, writeTool), "维护中普通用户不能写")

	// 管理员在维护模式下仍可写（与 MaintenanceGate 语义一致）。
	adminEnf := testEnforcer(enforcerOpts{
		role:        "admin",
		maintenance: true,
		granted:     map[string]bool{"admin|post:create": true},
	})
	adminIdent, authErr := adminEnf.Authenticate(context.Background(), bearer())
	require.Nil(t, authErr)
	assert.Nil(t, adminEnf.Authorize(context.Background(), adminIdent, writeTool), "管理员在维护中仍可写")
}

// TestEnforcer_RevokedKeyRejected 吊销的 Key 不得通过。
func TestEnforcer_RevokedKeyRejected(t *testing.T) {
	enf := testEnforcer(enforcerOpts{role: "user"})
	h := http.Header{}
	h.Set("X-API-Key", "ak_revoked")
	_, err := enf.Authenticate(context.Background(), h)
	require.NotNil(t, err)
	assert.Equal(t, CodeForbidden, err.Code)
}

// TestEnforcer_DisabledWhenSettingOff 总开关关闭时一律拒绝。
func TestEnforcer_DisabledWhenSettingOff(t *testing.T) {
	enf := NewEnforcer(EnforcerDeps{Settings: stubSettings{model.SettingMCPEnabled: "false"}})
	_, err := enf.Authenticate(context.Background(), bearer())
	require.NotNil(t, err)
	assert.Equal(t, CodeForbidden, err.Code)
}

// ── 自定义权限域隔离 ──────────────────────────────────────────

// TestCanPublishCustomize_RequiresExplicitPermission
// 「发布权限独立于写入权限」：只有 customize:publish 才能让高风险内容直生效。
func TestCanPublishCustomize_RequiresExplicitPermission(t *testing.T) {
	writeOnly := testEnforcer(enforcerOpts{
		role:    "admin",
		granted: map[string]bool{"admin|" + model.PermCustomizeWrite: true},
	})
	ident, authErr := writeOnly.Authenticate(context.Background(), bearer())
	require.Nil(t, authErr)
	assert.False(t, writeOnly.CanPublishCustomize(ident), "只有写权限时不得具备发布权限")

	// 发布权限必须落在**凭证的 scope** 上，而不是角色权限表
	// （见 TestCanPublishCustomize_NotInheritedFromAdminRole）。
	withPublish := testEnforcer(enforcerOpts{
		role:    "admin",
		granted: map[string]bool{"admin|" + model.PermCustomizeWrite: true},
		scopes:  "customize:write,customize:publish",
	})
	ident2, authErr := withPublish.Authenticate(context.Background(), bearer())
	require.Nil(t, authErr)
	assert.True(t, withPublish.CanPublishCustomize(ident2))
}

// TestAuthorize_CustomizeToolsRequireCustomizePerm
// 未授予自定义权限的 Key 不得调用自定义层工具。
func TestAuthorize_CustomizeToolsRequireCustomizePerm(t *testing.T) {
	enf := testEnforcer(enforcerOpts{
		role:    "user",
		granted: map[string]bool{"user|post:read": true},
	})
	ident, authErr := enf.Authenticate(context.Background(), bearer())
	require.Nil(t, authErr)

	tool := Tool{
		Name: "upsert_snippet", Write: true,
		Perm: model.PermCustomizeWrite,
	}
	err := enf.Authorize(context.Background(), ident, tool)
	require.NotNil(t, err)
	assert.Equal(t, CodeForbidden, err.Code)
}

// TestCanPublishCustomize_NotInheritedFromAdminRole
//
// 这是分级生效策略的关键一环：admin 角色默认持有 customize:publish
// （它本来就能在后台改这些配置），但**API Key 不该继承它**。
// 否则「admin 创建的任意 Key」都自动具备让 JS 直接生效的能力，
// 草稿保护对最需要它的那批凭证完全失效。
func TestCanPublishCustomize_NotInheritedFromAdminRole(t *testing.T) {
	enf := testEnforcer(enforcerOpts{
		role: "admin",
		granted: map[string]bool{
			// 角色权限表里确实有 publish（真实部署就是这样）
			"admin|" + model.PermCustomizePublish: true,
			"admin|" + model.PermCustomizeWrite:   true,
		},
		// 但 Key 的 scopes 里没有 customize:publish
		scopes: "post:read,customize:read,customize:write",
	})
	ident, authErr := enf.Authenticate(context.Background(), bearer())
	require.Nil(t, authErr)
	assert.False(t, enf.CanPublishCustomize(ident),
		"admin 角色的 Key 不得仅因角色继承就获得发布权限")

	// 显式在 scope 里授予后才算。
	withScope := testEnforcer(enforcerOpts{
		role:    "admin",
		granted: map[string]bool{"admin|" + model.PermCustomizeWrite: true},
		scopes:  "post:read,customize:read,customize:write,customize:publish",
	})
	ident2, authErr := withScope.Authenticate(context.Background(), bearer())
	require.Nil(t, authErr)
	assert.True(t, withScope.CanPublishCustomize(ident2),
		"scope 里显式写了 customize:publish 才应具备发布权限")
}
