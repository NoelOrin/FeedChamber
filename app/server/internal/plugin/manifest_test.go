package plugin_test

import (
	"testing"

	"feedchamber/internal/pkg/eventbus"
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

// ---- 钩子 → 事件映射（I1）----

// 每个「事件驱动」的钩子都必须映射到 eventbus 里真实存在的事件常量。
// 这条是 I1 的核心：先前 plan 假设钩子名与事件名是恒等映射，结果
// comment.created 找不到任何事件（真实事件是 content.created），
// 插件能装、能加载，却永远收不到调用。
func TestHookEventsAllExistInEventbus(t *testing.T) {
	known := make(map[string]struct{}, len(eventbus.AllEventTypes()))
	for _, e := range eventbus.AllEventTypes() {
		known[e] = struct{}{}
	}

	for hook, event := range plugin.HookEvents() {
		require.Contains(t, known, event,
			"钩子 %q 映射到的事件 %q 不在 eventbus.AllEventTypes() 中", hook, event)
		require.Contains(t, plugin.KnownHooks(), hook,
			"钩子 %q 出现在事件映射里，却不在 KnownHooks() 白名单中", hook)
	}
}

// 事件映射必须覆盖除 cron.daily 外的全部钩子：漏一个等于该钩子的插件静默失效。
func TestHookEventsCoversEveryHookExceptCron(t *testing.T) {
	events := plugin.HookEvents()

	require.Contains(t, events, plugin.HookCommentCreated)
	require.NotContains(t, events, plugin.HookCronDaily,
		"cron.daily 由 ticker 驱动，不得出现在事件映射里")
	require.False(t, plugin.HookIsEventDriven(plugin.HookCronDaily))
	require.True(t, plugin.HookIsEventDriven(plugin.HookCommentCreated))

	for _, hook := range plugin.KnownHooks() {
		if hook == plugin.HookCronDaily {
			continue
		}
		require.Contains(t, events, hook, "钩子 %q 缺少事件映射", hook)
		require.True(t, plugin.HookIsEventDriven(hook))
	}
}

// comment.created 绑定的是 content.created，而不是同名事件。
func TestHookCommentCreatedBindsToContentCreated(t *testing.T) {
	require.Equal(t, eventbus.EventContentCreated, plugin.HookEvents()[plugin.HookCommentCreated])
}

// user.registered 映射到同名的历史别名事件；Auth 同时双发
// user.created 与 user.registered，Task 5 不得因此双投递。
func TestHookUserRegisteredBindsToAliasEvent(t *testing.T) {
	require.Equal(t, eventbus.EventUserRegistered, plugin.HookEvents()[plugin.HookUserRegistered])
	require.NotEqual(t, eventbus.EventUserCreated, plugin.HookEvents()[plugin.HookUserRegistered],
		"必须绑定别名事件本身，否则双发的两条路径会重复投递")
}

// post.updated 没有任何事件源，必须从包里彻底消失。
func TestHookPostUpdatedIsGone(t *testing.T) {
	require.NotContains(t, plugin.KnownHooks(), "post.updated",
		"post.updated 没有对应事件，必须删除")
	require.NotContains(t, plugin.HookEvents(), "post.updated")
	require.False(t, plugin.HookIsEventDriven("post.updated"))
}

// ---- entry 路径校验（I2）----

// entry 在 Task 4 会变成文件系统路径，绝对路径与 .. 逃逸必须在解析期拒绝。
func TestParseManifestRejectsEscapingEntry(t *testing.T) {
	for _, entry := range []string{
		"../../../etc/passwd",
		"/etc/passwd",
		"sub/../ok.js",
		"   ",
	} {
		raw := []byte(`{"name":"x","version":"1","protocol_version":"1","entry":"` + entry + `"}`)
		_, err := plugin.ParseManifest(raw)
		require.Error(t, err, "entry=%q 必须被拒绝", entry)
	}
}

// 合法的 entry 不受影响，且缺省仍是 index.js。
func TestParseManifestAcceptsPlainEntryAndDefaults(t *testing.T) {
	m, err := plugin.ParseManifest([]byte(
		`{"name":"x","version":"1","protocol_version":"1","entry":"dist/main.js"}`))
	require.NoError(t, err)
	require.Equal(t, "dist/main.js", m.Entry)

	m2, err := plugin.ParseManifest([]byte(`{"name":"x","version":"1","protocol_version":"1"}`))
	require.NoError(t, err)
	require.Equal(t, "index.js", m2.Entry)
}
