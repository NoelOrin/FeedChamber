package handler

import (
	"testing"

	"feedchamber/internal/model"
	"feedchamber/internal/pkg"
	"feedchamber/internal/service"

	"github.com/stretchr/testify/assert"
)

// 后台路由对三种凭证来源是同一条：会话（站长）、API Key、OAuth 客户端。
// actorFor 必须把后两者识别成机器凭证，否则「后台」就是绕过分级生效的后门：
// 持 customize:write 的 Key 调 /snippets/{id}/toggle 就能让 JS 上线。
func TestActorForMapsCredentialSource(t *testing.T) {
	t.Run("会话是人工操作，可发布", func(t *testing.T) {
		got := actorFor("", "", "", "u1")
		assert.Equal(t, service.WriteSourceUI, got.Source)
		assert.True(t, got.CanPublish)
		assert.Equal(t, "u1", got.UserID)
	})

	t.Run("API Key 是机器凭证，默认不可发布", func(t *testing.T) {
		got := actorFor(pkg.AuthMethodAPIKey, "read,write", "", "u1")
		assert.Equal(t, service.WriteSourceMachine, got.Source)
		assert.False(t, got.CanPublish, "scope 里没有 customize:publish 就不能发布")
	})

	t.Run("API Key 显式授予 publish 后可发布", func(t *testing.T) {
		got := actorFor(pkg.AuthMethodAPIKey, model.PermCustomizeWrite+","+model.PermCustomizePublish, "", "u1")
		assert.Equal(t, service.WriteSourceMachine, got.Source)
		assert.True(t, got.CanPublish)
	})

	t.Run("OAuth 客户端读自己的 scope", func(t *testing.T) {
		got := actorFor(pkg.AuthMethodOAuthClient, "", model.PermCustomizePublish, "u1")
		assert.Equal(t, service.WriteSourceMachine, got.Source)
		assert.True(t, got.CanPublish)

		// api_key_scopes 不得被当成 client 的 scope（取错键会变成凭空授权）。
		prefixed := actorFor(pkg.AuthMethodOAuthClient, model.PermCustomizePublish, "", "u1")
		assert.False(t, prefixed.CanPublish)
	})
}

// 机器凭证写高风险内容必须落草稿——这条由 service 的 decideHighRisk 兜底，
// 但前提是 handler 把 Source 标对；这里断言「标对」这一半。
func TestActorForMachineCannotBypassDraftGate(t *testing.T) {
	machine := actorFor(pkg.AuthMethodAPIKey, "customize:write", "", "u1")
	assert.Equal(t, service.WriteSourceMachine, machine.Source)
	assert.False(t, machine.CanPublish)
}
