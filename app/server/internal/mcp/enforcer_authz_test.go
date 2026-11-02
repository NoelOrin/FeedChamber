package mcp

import (
	"context"
	"errors"
	"testing"

	"feedchamber/internal/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 回归：API Key 的 scope 是**上限**，不是「与角色权限二选一」。
//
// 旧实现只要角色权限表授予就放行，于是 admin 的 read-only Key 能调
// upsert_snippet 往全站注入 CSS/JS，普通用户的 read-only Key 能发帖。
// 网关对 api_key 的判据是「scope 与角色同时覆盖」，MCP 必须一致。
func TestAuthorizeAPIKeyCannotExceedScope(t *testing.T) {
	enf := testEnforcer(enforcerOpts{
		role:   "admin",
		scopes: "read",
		granted: map[string]bool{
			"admin|" + model.PermCustomizeWrite: true,
			"admin|" + model.PermPostCreate:     true,
			"admin|" + model.PermCustomizeRead:  true,
			"admin|" + model.PermPostRead:       true,
		},
	})
	ident, errResp := enf.Authenticate(context.Background(), bearer())
	require.Nil(t, errResp)

	assert.NotNil(t, enf.Authorize(context.Background(), ident,
		Tool{Name: "upsert_snippet", Perm: model.PermCustomizeWrite, Write: true}),
		"read-only scope 不得调用写工具，即便角色是 admin")

	assert.NotNil(t, enf.Authorize(context.Background(), ident,
		Tool{Name: "create_post", Perm: model.PermPostCreate, Write: true}),
		"read-only scope 不得发帖")

	assert.Nil(t, enf.Authorize(context.Background(), ident,
		Tool{Name: "get_post", Perm: model.PermPostRead}),
		"scope 覆盖的读工具应放行")
}

// 精确码可以当 scope 用：这类 Key 的写权限必须真的生效。
func TestAuthorizeAPIKeyWithExactWriteScopePasses(t *testing.T) {
	enf := testEnforcer(enforcerOpts{
		role:   "admin",
		scopes: model.PermCustomizeWrite,
		granted: map[string]bool{
			"admin|" + model.PermCustomizeWrite: true,
		},
	})
	ident, errResp := enf.Authenticate(context.Background(), bearer())
	require.Nil(t, errResp)
	assert.Nil(t, enf.Authorize(context.Background(), ident,
		Tool{Name: "upsert_snippet", Perm: model.PermCustomizeWrite, Write: true}))
}

// 站长关掉 API Key 认证后，/mcp 也必须一起关——否则全局止血开关对这条通道失效。
func TestAuthenticateRejectsWhenAPIKeyAuthDisabled(t *testing.T) {
	enf := NewEnforcer(EnforcerDeps{
		Settings: stubSettings{
			model.SettingMCPEnabled:    "true",
			model.SettingAPIKeyEnabled: "false",
		},
		Perms:  stubPerms{granted: map[string]bool{}},
		Banned: stubBan{banned: map[string]bool{}},
		Verify: func(hash, plain string) bool { return true },
		Lookup: stubLookup{
			keys: map[string]*model.APIKey{
				"key1": {KeyID: "key1", SecretHash: "hash", UserUUID: "u1", Scopes: "read", Status: "active"},
			},
			users: map[string]*model.User{"u1": {ID: "u1", Role: "admin"}},
		},
	})

	_, errResp := enf.Authenticate(context.Background(), bearer())
	require.NotNil(t, errResp, "api_key_enabled=false 时 /mcp 必须拒绝")
	assert.Equal(t, CodeForbidden, errResp.Code)
}

// ── 工具启停必须落库 ──────────────────────────────────────────

type stubToolStateStore struct {
	raw   string
	err   error
	saves int
}

func (s *stubToolStateStore) SaveToolState(_ context.Context, raw string) error {
	s.saves++
	if s.err != nil {
		return s.err
	}
	s.raw = raw
	return nil
}

// 后台启停必须写回设置：否则重启或多实例下工具状态各自漂移。
func TestHandlerSetToolEnabledPersists(t *testing.T) {
	srv, reg := newTestServer(t)
	store := &stubToolStateStore{}
	h := NewHandler(srv)
	h.SetToolStateStore(store)

	require.NoError(t, h.SetToolEnabled(context.Background(), "write_tool", true))
	assert.True(t, reg.Enabled("write_tool"))
	assert.Equal(t, 1, store.saves)
	assert.Contains(t, store.raw, "-write_tool")

	// 回读：新实例按持久化值还原后状态一致。
	fresh := NewRegistry()
	fresh.Register(Tool{Name: "read_tool"})
	fresh.Register(Tool{Name: "write_tool", Write: true})
	fresh.ApplyDisabledList(store.raw)
	assert.True(t, fresh.Enabled("write_tool"))
}

// 持久化失败时不得留下「内存改了、库里没改」的不一致。
func TestHandlerSetToolEnabledRollsBackOnSaveFailure(t *testing.T) {
	srv, reg := newTestServer(t)
	store := &stubToolStateStore{err: errors.New("kv down")}
	h := NewHandler(srv)
	h.SetToolStateStore(store)

	require.Error(t, h.SetToolEnabled(context.Background(), "write_tool", true))
	assert.False(t, reg.Enabled("write_tool"), "保存失败必须回滚内存状态")
}

// 不存在的工具要能区分于「保存失败」。
func TestHandlerSetToolEnabledUnknownTool(t *testing.T) {
	srv, _ := newTestServer(t)
	h := NewHandler(srv)
	h.SetToolStateStore(&stubToolStateStore{})

	require.ErrorIs(t, h.SetToolEnabled(context.Background(), "nope", true), ErrToolNotFound)
}
