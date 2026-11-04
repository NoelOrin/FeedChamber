package plugin_test

import (
	"testing"

	"feedchamber/internal/plugin"

	"github.com/stretchr/testify/require"
)

func TestParseManifestValid(t *testing.T) {
	raw := []byte(`{
		"name": "hello",
		"version": "1.0.0",
		"protocol_version": "1",
		"hooks": ["post.created", "comment.created"],
		"capabilities": ["posts:read", "kv:write"]
	}`)
	m, err := plugin.ParseManifest(raw)
	require.NoError(t, err)
	require.Equal(t, "hello", m.Name)
	require.Equal(t, plugin.ProtocolVersion1, m.ProtocolVersion)
	require.Equal(t, []string{"post.created", "comment.created"}, m.Hooks)
	require.Equal(t, []string{"posts:read", "kv:write"}, m.Capabilities)
}

func TestParseManifestRejectsUnknownProtocol(t *testing.T) {
	raw := []byte(`{"name":"x","version":"1","protocol_version":"99"}`)
	_, err := plugin.ParseManifest(raw)
	require.Error(t, err)
	require.Contains(t, err.Error(), "protocol_version")
}

func TestParseManifestRequiresNameAndVersion(t *testing.T) {
	_, err := plugin.ParseManifest([]byte(`{"name":"","version":"1"}`))
	require.Error(t, err)

	_, err = plugin.ParseManifest([]byte(`{"name":"x","version":""}`))
	require.Error(t, err)
}

// 未定义的钩子名被忽略，不导致解析失败。
func TestParseManifestDropsUnknownHooks(t *testing.T) {
	raw := []byte(`{
		"name": "x", "version": "1", "protocol_version": "1",
		"hooks": ["post.created", "not.a.real.hook"]
	}`)
	m, err := plugin.ParseManifest(raw)
	require.NoError(t, err)
	require.Equal(t, []string{"post.created"}, m.Hooks,
		"未知钩子必须被丢弃而非报错")
}

func TestParseManifestRejectsBadJSON(t *testing.T) {
	_, err := plugin.ParseManifest([]byte(`{`))
	require.Error(t, err)
}

// 与「未知钩子被丢弃」相反的半边：capability 决定权限，
// 未知 capability 必须报错而不是被悄悄忽略。
func TestManifestValidateRejectsUnknownCapabilities(t *testing.T) {
	m, err := plugin.ParseManifest([]byte(`{
		"name": "x", "version": "1", "protocol_version": "1",
		"hooks": ["post.created"],
		"capabilities": ["posts:read", "posts:teleport"]
	}`))
	require.NoError(t, err, "解析阶段不校验 capability，交给 Validate")

	known := map[string]struct{}{"posts:read": {}, "kv:write": {}}
	err = m.Validate(known)
	require.Error(t, err)
	require.Contains(t, err.Error(), "posts:teleport")
}

// Validate 的另外两条边界：能力全已知时通过；无钩子时即使能力合法也要报错。
func TestManifestValidateAcceptsKnownCapabilitiesAndRequiresHook(t *testing.T) {
	m, err := plugin.ParseManifest([]byte(`{
		"name": "x", "version": "1", "protocol_version": "1",
		"hooks": ["post.created"],
		"capabilities": ["posts:read", "kv:write"]
	}`))
	require.NoError(t, err)
	known := map[string]struct{}{"posts:read": {}, "kv:write": {}}
	require.NoError(t, m.Validate(known))

	// 钩子全为未知 → 被丢弃后为空 → 插件没有任何入口，必须拒绝。
	m2, err := plugin.ParseManifest([]byte(`{
		"name": "x", "version": "1", "protocol_version": "1",
		"hooks": ["not.a.real.hook"]
	}`))
	require.NoError(t, err)
	require.Empty(t, m2.Hooks)
	require.Error(t, m2.Validate(known))
}
