package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"testing"
	"time"

	"feedchamber/internal/model"

	"github.com/stretchr/testify/require"
)

// ---- 测试替身 ----

type fakePostWriter struct {
	calls []string
	err   error
}

func (f *fakePostWriter) SetStatus(ctx context.Context, postID, status string) error {
	f.calls = append(f.calls, postID+":"+status)
	return f.err
}

func (f *fakePostWriter) AddTag(ctx context.Context, postID, tag string) error {
	f.calls = append(f.calls, postID+"+"+tag)
	return f.err
}

type fakeCommentWriter struct{ calls []string }

func (f *fakeCommentWriter) SetStatus(ctx context.Context, commentID, status string) error {
	f.calls = append(f.calls, commentID+":"+status)
	return nil
}

type fakeUserReader struct{ levels map[string]int }

func (f *fakeUserReader) GetLevel(ctx context.Context, userID string) (int, error) {
	return f.levels[userID], nil
}

func (f *fakeUserReader) GetEmail(ctx context.Context, userID string) (string, error) {
	return userID + "@example.test", nil
}

type fakeNotifier struct{ sent []string }

func (f *fakeNotifier) Send(ctx context.Context, userID, title, body string) error {
	f.sent = append(f.sent, userID+"|"+title)
	return nil
}

// fakeKV 是**不含命名空间逻辑**的裸存储：作用域必须由 api.go 自己拼，
// 这样测试才能看到它到底写进去的是什么 key。
type fakeKV struct {
	data map[string]string
	gets []string
	sets []string
}

func newFakeKV() *fakeKV { return &fakeKV{data: map[string]string{}} }

func (f *fakeKV) Get(ctx context.Context, key string) (string, error) {
	f.gets = append(f.gets, key)
	return f.data[key], nil
}

func (f *fakeKV) Set(ctx context.Context, key, value string) error {
	f.sets = append(f.sets, key)
	f.data[key] = value
	return nil
}

type fakeSlots struct{ calls []string }

func (f *fakeSlots) Set(ctx context.Context, slot, kind string, props map[string]any, sortOrder int) error {
	f.calls = append(f.calls, "set:"+slot)
	return nil
}

func (f *fakeSlots) Delete(ctx context.Context, slot string) error {
	f.calls = append(f.calls, "del:"+slot)
	return nil
}

type fakeAudit struct {
	records []string
	err     error
}

func (f *fakeAudit) RecordPluginAction(ctx context.Context, pluginID, action, target, detail string) error {
	// 记录里带上 detail：既有断言只看前缀 "test-plugin|" 与 target 子串，
	// 追加一段不改变任何已断言的语义；而 TestAuditTargetComesFromParamDeclarations
	// 需要看到 detail 才能钉住「审计字段来自同一份参数声明」。
	f.records = append(f.records, pluginID+"|"+action+"|"+target+"|"+detail)
	return f.err
}

// ---- 测试辅助 ----

// errAuditDown 是审计失败测试用的哨兵错误。
var errAuditDown = errors.New("audit store down")

// contextWithTimeout 返回一个 5s 上限的 ctx（IPC 往返测试用）。
func contextWithTimeout(t *testing.T) (context.Context, context.CancelFunc) {
	t.Helper()
	return context.WithTimeout(context.Background(), 5*time.Second)
}

// writeAPISampleParams 返回某个写类 API 的一组合法参数。
//
// 新增写类 API 时必须在这里补一条，否则 TestLookupAPIFailsClosedWithoutCapabilities
// 会以「参数非法」而不是「能力被拒」失败——这正是想要的提醒方式：
// 新 API 不被这两条测试覆盖是不允许的。
func writeAPISampleParams(method string) map[string]any {
	switch method {
	case "api.posts.setStatus":
		return map[string]any{"post_id": "p1", "status": "pending"}
	case "api.posts.addTag":
		return map[string]any{"post_id": "p1", "tag": "auto"}
	case "api.comments.setStatus":
		return map[string]any{"comment_id": "c1", "status": "pending"}
	case "api.notifications.send":
		return map[string]any{"user_id": "u1", "title": "t", "body": "b"}
	case "api.kv.set":
		return map[string]any{"key": "k", "value": "v"}
	case "api.slots.set":
		return map[string]any{"slot": "sidebar.top", "kind": "text", "props": map[string]any{"text": "hi"}}
	case "api.slots.delete":
		return map[string]any{"slot": "sidebar.top"}
	default:
		return nil
	}
}

// dedupe 返回去重后的副本（保持首次出现顺序）。
func dedupe(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if _, dup := seen[s]; dup {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

// sortedCopy 返回排序后的副本，用来断言「输出已排序」。
func sortedCopy(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}

// invokeAPI 是测试辅助：把一次 API 调用直接路由到 handler。
func invokeAPI(t *testing.T, caps CapabilitySet, deps APIDeps, method string, params any) error {
	t.Helper()
	return invokeAPIAs(t, "test-plugin", caps, deps, method, params)
}

func invokeAPIAs(t *testing.T, pluginID string, caps CapabilitySet, deps APIDeps, method string, params any) error {
	t.Helper()
	raw, err := json.Marshal(params)
	require.NoError(t, err)
	fn, ok := lookupAPI(method, pluginID, caps, deps)
	if !ok {
		return errUnknownMethod(method)
	}
	_, err = fn(context.Background(), raw)
	return err
}

// mustJSON 把参数对象编码成 json.RawMessage（喂 decodeArgs 用）。
func mustJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(v)
	require.NoError(t, err)
	return raw
}

// ---- capability 校验 ----

func TestRegisterAPIsRejectsUndeclaredCapability(t *testing.T) {
	posts := &fakePostWriter{}
	deps := APIDeps{Posts: posts, Audit: &fakeAudit{}}

	caps := CapabilitySet{"posts:read": {}} // 只有读
	err := invokeAPI(t, caps, deps, "api.posts.setStatus", map[string]string{
		"post_id": "p1", "status": "pending",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "capability")
	require.Empty(t, posts.calls, "未声明能力的调用不得触达 service")
}

func TestRegisterAPIsAllowsDeclaredCapability(t *testing.T) {
	posts := &fakePostWriter{}
	audit := &fakeAudit{}
	deps := APIDeps{Posts: posts, Audit: audit}

	caps := CapabilitySet{"posts:write": {}}
	require.NoError(t, invokeAPI(t, caps, deps, "api.posts.setStatus", map[string]string{
		"post_id": "p1", "status": "pending",
	}))
	require.Equal(t, []string{"p1:pending"}, posts.calls)
	require.Len(t, audit.records, 1, "写类 API 必须落审计")
}

// caps 为 nil/空必须拒绝**每一个**写类 API（fail closed）。
// 三条错法里的第一条：默认放行或「没声明就查一下」。
func TestLookupAPIFailsClosedWithoutCapabilities(t *testing.T) {
	posts := &fakePostWriter{}
	audit := &fakeAudit{}
	kv := newFakeKV()
	deps := APIDeps{
		Posts: posts, Comments: &fakeCommentWriter{}, Users: &fakeUserReader{},
		Notifications: &fakeNotifier{}, KV: kv, Slots: &fakeSlots{}, Audit: audit,
	}

	for _, caps := range []CapabilitySet{nil, {}} {
		require.NotEmpty(t, writeAPIMethods(), "写类方法清单不得为空（否则本测试空转）")
		for _, method := range writeAPIMethods() {
			err := invokeAPI(t, caps, deps, method, writeAPISampleParams(method))
			require.Error(t, err, "caps=%v 时 %s 必须被拒绝", caps, method)
			require.Contains(t, err.Error(), "capability")
		}
	}

	require.Empty(t, posts.calls, "无能力的插件不得触达任何写 service")
	require.Empty(t, audit.records, "被拒绝的调用不得产生审计")
	require.Empty(t, kv.sets, "无能力的插件不得写入 KV")
}

// 未注册的 handler 意味着插件拿到的是「unknown method」，而不是一个
// 「先检查再执行」的 handler。两者对插件的可观测行为不同：后者在能力
// 缺失时仍然走了一遍 handler 代码。
func TestRegisterAPIsDoesNotHandleUndeclaredCapability(t *testing.T) {
	conn, peer := newPipePairForTest(t)
	RegisterAPIs(conn, "test-plugin", CapabilitySet{"posts:read": {}}, APIDeps{
		Posts: &fakePostWriter{}, Audit: &fakeAudit{},
	})

	ctx, cancel := contextWithTimeout(t)
	defer cancel()
	_, err := peer.Call(ctx, "api.posts.setStatus", map[string]string{
		"post_id": "p1", "status": "pending",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "capability",
		"未声明能力必须是明确的拒绝，而不是 unknown method 之外的静默")
}

// 已授予能力的 API 必须真的注册上（防止「全都不注册」也能让上面那条变绿）。
func TestRegisterAPIHandlesDeclaredCapabilityOverIPC(t *testing.T) {
	conn, peer := newPipePairForTest(t)
	posts := &fakePostWriter{}
	RegisterAPIs(conn, "test-plugin", CapabilitySet{"posts:write": {}}, APIDeps{
		Posts: posts, Audit: &fakeAudit{},
	})

	ctx, cancel := contextWithTimeout(t)
	defer cancel()
	raw, err := peer.Call(ctx, "api.posts.setStatus", map[string]string{
		"post_id": "p1", "status": "pending",
	})
	require.NoError(t, err)
	require.JSONEq(t, `{"ok":true}`, string(raw))
	require.Equal(t, []string{"p1:pending"}, posts.calls)
}

// ---- 写 API 的审计 ----

func TestWriteAPIsAreAudited(t *testing.T) {
	audit := &fakeAudit{}
	deps := APIDeps{
		Posts: &fakePostWriter{}, Comments: &fakeCommentWriter{}, Users: &fakeUserReader{},
		Notifications: &fakeNotifier{}, KV: newFakeKV(), Slots: &fakeSlots{}, Audit: audit,
	}
	caps := CapabilitySetFromNames(KnownCapabilities())

	for _, method := range writeAPIMethods() {
		before := len(audit.records)
		require.NoError(t, invokeAPI(t, caps, deps, method, writeAPISampleParams(method)), method)
		require.Len(t, audit.records, before+1, "%s 必须落且只落一条审计", method)
		require.True(t, strings.HasPrefix(audit.records[before], "test-plugin|"),
			"审计必须带插件标识: %q", audit.records[before])
	}
}

// 审计失败不得把已经成功的业务动作翻成失败（业务已落库，插件只会重试）。
func TestWriteAPIAuditFailureDoesNotFailTheCall(t *testing.T) {
	deps := APIDeps{
		Posts: &fakePostWriter{}, Audit: &fakeAudit{err: errAuditDown},
	}
	require.NoError(t, invokeAPI(t, CapabilitySet{"posts:write": {}}, deps,
		"api.posts.setStatus", map[string]string{"post_id": "p1", "status": "pending"}))
}

// ---- 参数校验：缺字段必须报错，不能把空串交给 service ----

func TestWriteAPIsRejectMissingRequiredFields(t *testing.T) {
	caps := CapabilitySetFromNames(KnownCapabilities())

	cases := []struct {
		method string
		params any
	}{
		{"api.posts.setStatus", map[string]string{"status": "pending"}},
		{"api.posts.setStatus", map[string]string{"post_id": "p1"}},
		{"api.posts.setStatus", map[string]string{"post_id": "p1", "status": "   "}},
		{"api.posts.addTag", map[string]string{"tag": "auto"}},
		{"api.posts.addTag", map[string]string{"post_id": "p1"}},
		{"api.comments.setStatus", map[string]string{"post_id": "c1"}},
		{"api.notifications.send", map[string]string{"title": "t", "body": "b"}},
		{"api.notifications.send", map[string]string{"user_id": "u1", "body": "b"}},
		{"api.notifications.send", map[string]string{"user_id": "u1", "title": "t"}},
		{"api.kv.set", map[string]string{"value": "v"}},
		{"api.kv.get", map[string]string{}},
		{"api.slots.set", map[string]string{"kind": "text"}},
		{"api.slots.delete", map[string]string{}},
	}

	for _, tc := range cases {
		posts := &fakePostWriter{}
		comments := &fakeCommentWriter{}
		notify := &fakeNotifier{}
		kv := newFakeKV()
		slots := &fakeSlots{}
		audit := &fakeAudit{}
		deps := APIDeps{Posts: posts, Comments: comments, Users: &fakeUserReader{},
			Notifications: notify, KV: kv, Slots: slots, Audit: audit}

		err := invokeAPI(t, caps, deps, tc.method, tc.params)
		require.Error(t, err, "%s 缺少必填字段时必须报错（params=%v）", tc.method, tc.params)
		require.Empty(t, posts.calls, "%s 参数非法时不得触达 service", tc.method)
		require.Empty(t, comments.calls)
		require.Empty(t, notify.sent)
		require.Empty(t, kv.sets)
		require.Empty(t, slots.calls)
		require.Empty(t, audit.records, "%s 参数非法时不得落审计", tc.method)
	}
}

// ---- KV 命名空间 ----

// 插件 MUST NOT 逃出 plugin:<id>: 命名空间：逃逸的 key 一律拒绝，
// 绝对化/穿越形态都不得原样写进存储。
func TestKVRejectsKeysEscapingPluginNamespace(t *testing.T) {
	for _, method := range []string{"api.kv.get", "api.kv.set"} {
		for _, key := range []string{
			"../../other-plugin/secret",
			"../evil",
			"/abs/path",
			"plugin:other:key", // 冒充别的插件的命名空间
			"a/../../b",
			"sub\\..\\..\\x",
			"",
			"   ",
		} {
			kv := newFakeKV()
			audit := &fakeAudit{}
			deps := APIDeps{KV: kv, Audit: audit}
			caps := CapabilitySetFromNames(KnownCapabilities())

			params := map[string]string{"key": key}
			if method == "api.kv.set" {
				params["value"] = "v"
			}
			err := invokeAPI(t, caps, deps, method, params)
			require.Error(t, err, "%s key=%q 必须被拒绝", method, key)
			require.Empty(t, kv.sets, "%s key=%q 不得写进存储", method, key)
			require.Empty(t, kv.gets, "%s key=%q 不得读存储", method, key)
			require.Empty(t, audit.records, "被拒绝的 KV 写不得落审计")
		}
	}
}

// 合法的 key 必须被强制加上 plugin:<id>: 前缀——起作用的是 api.go，
// 不是插件自觉，也不是存储实现。
func TestKVScopesKeysUnderPluginPrefix(t *testing.T) {
	kv := newFakeKV()
	deps := APIDeps{KV: kv, Audit: &fakeAudit{}}
	caps := CapabilitySetFromNames(KnownCapabilities())

	require.NoError(t, invokeAPIAs(t, "alpha", caps, deps, "api.kv.set",
		map[string]string{"key": "counter", "value": "7"}))
	require.Equal(t, []string{"plugin:alpha:counter"}, kv.sets,
		"KV 实现收到的 key 必须已经被限定在插件命名空间内")

	require.NoError(t, invokeAPIAs(t, "alpha", caps, deps, "api.kv.get",
		map[string]string{"key": "counter"}))
	require.Equal(t, []string{"plugin:alpha:counter"}, kv.gets)
}

// 第二个插件读不到第一个插件的键：即便 key 完全相同，前缀也不同；
// 而直接点名别人的命名空间会被拒绝。
func TestKVIsolatesPluginsFromEachOther(t *testing.T) {
	store := newFakeKV()
	caps := CapabilitySetFromNames(KnownCapabilities())
	deps := APIDeps{KV: store, Audit: &fakeAudit{}}

	require.NoError(t, invokeAPIAs(t, "alpha", caps, deps, "api.kv.set",
		map[string]string{"key": "shared", "value": "alpha-secret"}))
	require.Equal(t, []string{"plugin:alpha:shared"}, store.sets)

	var got struct {
		Value string `json:"value"`
	}
	fn, ok := lookupAPI("api.kv.get", "beta", caps, deps)
	require.True(t, ok)
	raw, err := fn(context.Background(), json.RawMessage(`{"key":"shared"}`))
	require.NoError(t, err)
	encoded, err := json.Marshal(raw)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(encoded, &got))
	require.Empty(t, got.Value, "beta 不得读到 alpha 的键")

	// 直接点名 alpha 的命名空间必须被拒绝，而不是被当成合法 key。
	err = invokeAPIAs(t, "beta", caps, deps, "api.kv.get",
		map[string]string{"key": "plugin:alpha:shared"})
	require.Error(t, err)
}

// kv.get 不带写能力也能用；kv:write 不隐含 kv:read，反之亦然。
func TestKVCapabilitiesAreIndependent(t *testing.T) {
	kv := newFakeKV()
	deps := APIDeps{KV: kv, Audit: &fakeAudit{}}

	require.Error(t, invokeAPI(t, CapabilitySet{"kv:write": {}}, deps,
		"api.kv.get", map[string]string{"key": "k"}), "kv:write 不隐含 kv:read")
	require.Error(t, invokeAPI(t, CapabilitySet{"kv:read": {}}, deps,
		"api.kv.set", map[string]string{"key": "k", "value": "v"}), "kv:read 不隐含 kv:write")
}

// 省略 value 等价于空串（清空这个键），这是重构前的行为，不得收紧。
//
// 重构前 kv.set 从不调 requireFields，{"key":"c"} 会存下 ""。把 value 标成
// Required 后同一个调用变成「参数缺失: value」——清空一个键这个合法语义
// 被参数校验挡住了。
func TestKVSetWithoutValueStoresEmpty(t *testing.T) {
	kv := newFakeKV()
	deps := APIDeps{KV: kv, Audit: &fakeAudit{}}
	err := invokeAPIAs(t, "alpha", CapabilitySet{"kv:write": {}}, deps, "api.kv.set",
		map[string]any{"key": "c"})
	require.NoError(t, err, "省略 value 必须合法")
	require.Equal(t, []string{"plugin:alpha:c"}, kv.sets)
	require.Equal(t, "", kv.data["plugin:alpha:c"], "省略 value 必须存成空串")
}

// 端到端：带空白的 key 必须仍被 pluginKVKey 拒绝（证明 NoTrim 真的生效）。
//
// 这条与 TestNoTrimFlagsArePinned 是一对：那条钉声明，这条钉**行为**。
// 只钉声明时，把 decodeArgs 里 `if !f.NoTrim` 的判断删掉仍会全绿。
func TestKVKeyWhitespaceStillRejected(t *testing.T) {
	for _, method := range []string{"api.kv.get", "api.kv.set"} {
		kv := newFakeKV()
		deps := APIDeps{KV: kv, Audit: &fakeAudit{}}
		params := map[string]any{"key": " k"}
		if method == "api.kv.set" {
			params["value"] = "v"
		}
		err := invokeAPIAs(t, "alpha", CapabilitySet{"kv:write": {}, "kv:read": {}}, deps, method, params)
		require.Error(t, err, "%s 必须拒绝首尾空白的 key", method)
		require.Contains(t, err.Error(), "空白")
		require.Empty(t, kv.sets)
		require.Empty(t, kv.gets)
	}
}

// ---- 未知方法 ----

func TestLookupAPIUnknownMethod(t *testing.T) {
	_, ok := lookupAPI("api.posts.teleport", "test-plugin", CapabilitySetFromNames(KnownCapabilities()), APIDeps{})
	require.False(t, ok)
	require.Contains(t, errUnknownMethod("api.posts.teleport").Error(), "unknown method")
}

// apiMethodNames 与 lookupAPI 的 case 列表必须严格一致：
// 少一个 → 该 API 永远「unknown method」；多一个 → RegisterAPIs 注册空壳。
func TestAPIMethodNamesMatchLookup(t *testing.T) {
	caps := CapabilitySetFromNames(KnownCapabilities())
	deps := APIDeps{
		Posts: &fakePostWriter{}, Comments: &fakeCommentWriter{}, Users: &fakeUserReader{},
		Notifications: &fakeNotifier{}, KV: newFakeKV(), Slots: &fakeSlots{}, Audit: &fakeAudit{},
	}
	names := apiMethodNames()
	require.Len(t, names, len(dedupe(names)), "方法名不得重复")

	for _, name := range names {
		_, ok := lookupAPI(name, "test-plugin", caps, deps)
		require.True(t, ok, "%s 出现在 apiMethodNames() 里但 lookupAPI 不认识", name)
	}

	// 反向：writeAPIMethods() 必须是 apiMethodNames() 的子集。
	for _, name := range writeAPIMethods() {
		require.Contains(t, names, name, "%s 是写类却没登记在 apiMethodNames() 里", name)
	}
}

// ---- cap -> 方法映射 ----

// 每个 API 方法必须声明它需要的一个 capability；未列出的方法按「无能力要求」
// 处理会被下面的断言抓住。
func TestEveryAPIMethodDeclaresACapability(t *testing.T) {
	caps := apiCapabilities()
	for _, name := range apiMethodNames() {
		need, ok := caps[name]
		require.True(t, ok, "%s 没有登记 capability（漏登记＝可能放行未声明能力）", name)
		require.True(t, IsKnownCapability(need), "%s 声明的 capability %q 不在白名单里", name, need)
	}
}

// ---- CapabilitySet 与 manifest 的接缝 ----

func TestKnownCapabilitiesAreSortedAndKnown(t *testing.T) {
	// 反复取：map 迭代序每次不同，单次比对只是「可能」抓到未排序。
	// 20 次全绿才算真的排过序（未排序时几乎必红）。
	for i := 0; i < 20; i++ {
		caps := KnownCapabilities()
		require.NotEmpty(t, caps)
		require.Equal(t, sortedCopy(caps), caps, "KnownCapabilities 必须排序（确定性输出）")
		require.Equal(t, len(caps), len(dedupe(caps)))
	}

	// 内容与常量逐一对齐：漏登记一个常量，它就无法被任何 manifest 声明。
	require.Equal(t, []string{
		CapCommentsRead, CapCommentsWrite,
		CapKVRead, CapKVWrite,
		CapNotifySend,
		CapPostsRead, CapPostsWrite,
		CapSlotsWrite,
		CapUsersRead,
	}, KnownCapabilities())

	for _, c := range KnownCapabilities() {
		require.True(t, IsKnownCapability(c), "%q 必须被 IsKnownCapability 认可", c)
	}
	require.False(t, IsKnownCapability("posts:teleport"))
	require.False(t, IsKnownCapability(""))
}

// pluginID 参与 KV 命名空间拼接，它自己就是一条逃逸面：
// 空 ID 让所有插件共享 plugin::，含 ":" 的 ID 与另一个插件拼出同一个全键。
func TestKVRejectsUnsafePluginIDs(t *testing.T) {
	kv := newFakeKV()
	deps := APIDeps{KV: kv, Audit: &fakeAudit{}}
	caps := CapabilitySetFromNames(KnownCapabilities())

	for _, id := range []string{"", "   ", "a:b", "alpha/../beta", "../beta", "plugin:alpha", "a\\b", "a b"} {
		err := invokeAPIAs(t, id, caps, deps, "api.kv.set", map[string]string{"key": "k", "value": "v"})
		require.Error(t, err, "pluginID=%q 必须被拒绝（否则命名空间可能与其他插件重合）", id)
		require.Empty(t, kv.sets, "pluginID=%q 不得写进存储", id)
	}
}

// CapabilitySetFromNames 的输出必须能直接喂给 Manifest.Validate
// （计划 Task 8 的调用点：m.Validate(plugin.CapabilitySetFromNames(plugin.KnownCapabilities()))）。
func TestCapabilitySetFromNamesFeedsManifestValidate(t *testing.T) {
	known := CapabilitySetFromNames(KnownCapabilities())
	require.IsType(t, map[string]struct{}{}, map[string]struct{}(known),
		"CapabilitySet 的底层类型必须是 map[string]struct{}，才能直接传给 Validate")

	m, err := ParseManifest([]byte(`{
		"name":"x","version":"1","protocol_version":"1",
		"hooks":["post.created"],
		"capabilities":["posts:read","kv:write"]
	}`))
	require.NoError(t, err)
	require.NoError(t, m.Validate(known), "合法 manifest 必须通过")

	m2, err := ParseManifest([]byte(`{
		"name":"x","version":"1","protocol_version":"1",
		"hooks":["post.created"],
		"capabilities":["posts:teleport"]
	}`))
	require.NoError(t, err)
	require.Error(t, m2.Validate(known), "未知 capability 仍必须报错")

	// nil/空 knownCaps 必须拒绝任何声明的 capability（fail closed）。
	require.Error(t, m2.Validate(nil))
	require.Error(t, m2.Validate(CapabilitySet{}))

	// 未知名字不得混进集合。
	require.NotContains(t, CapabilitySetFromNames([]string{"posts:read", "posts:teleport"}), "posts:teleport",
		"CapabilitySetFromNames 不得静默放行未知名字")
	require.Contains(t, CapabilitySetFromNames([]string{"posts:read", "posts:teleport"}), "posts:read")
}

// 权限错误必须可被调用方识别（spec：返回权限错误给插件）。
func TestCapabilityErrorsAreIdentifiable(t *testing.T) {
	err := invokeAPI(t, CapabilitySet{"posts:read": {}}, APIDeps{Posts: &fakePostWriter{}, Audit: &fakeAudit{}},
		"api.posts.setStatus", map[string]string{"post_id": "p1", "status": "pending"})
	require.ErrorIs(t, err, ErrCapabilityDenied)
}

// 每个写类规格的审计三元组必须**逐字**符合预期。
//
// 只检查「audit 非零」与「字段名存在」抓不住这些错：Target/Detail 对调、
// Action 拼错、KV Scope 忘了加——它们都能让审计落一条格式正确但内容错位的
// 记录，而 TestWriteAPIsAreAudited 只数条数，全都照绿。
//
// 同样重要的是这里走**真实路径**：样例参数先过 decodeArgs（证明它自身合法），
// 再由 spec.audit.extract 取三元组，与 adapt 落审计用的是同一条代码路径。
func TestAuditTriplesAreExact(t *testing.T) {
	want := map[string]struct{ action, target, detail string }{
		"api.posts.setStatus":    {"post.setStatus", "p1", "pending"},
		"api.posts.addTag":       {"post.addTag", "p1", "auto"},
		"api.comments.setStatus": {"comment.setStatus", "c1", "pending"},
		"api.notifications.send": {"notification.send", "u1", "t"},
		// KV 的 target 是**加过前缀的全键**（AuditScopePluginKV），detail 为空。
		"api.kv.set":       {"kv.set", "plugin:test-plugin:k", ""},
		"api.slots.set":    {"slot.set", "sidebar.top", "text"},
		"api.slots.delete": {"slot.delete", "sidebar.top", ""},
	}

	// 表必须与写类清单**恰好**同集：新增写类规格时忘了补一行，
	// 它会在这里直接报错，而不是悄悄逃出审计口径的覆盖范围。
	writeMethods := writeAPIMethods()
	require.Len(t, want, len(writeMethods),
		"预期表必须与 writeAPIMethods() 一一对应：%v", writeMethods)
	for _, method := range writeMethods {
		_, ok := want[method]
		require.True(t, ok, "%s 是写类但预期表里没有它", method)
	}

	for method, exp := range want {
		spec, ok := findAPI(method)
		require.True(t, ok, "%s 必须存在", method)
		require.True(t, spec.write, "%s 必须是写类", method)

		a, err := decodeArgs(mustJSON(t, writeAPISampleParams(method)), spec.params)
		require.NoError(t, err, "%s 的样例参数必须能通过自身校验", method)

		action, target, detail := spec.audit.extract("test-plugin", a)
		require.Equal(t, exp.action, action, "%s 的审计 action", method)
		require.Equal(t, exp.target, target, "%s 的审计 target", method)
		require.Equal(t, exp.detail, detail, "%s 的审计 detail", method)
	}
}

// NoTrim 必须落在且只落在这四个字段上。
//
// 少了 kv 的 key，pluginKVKey 的 `trimmed != key` 空白拒绝就被静默绕过 ——
// " k" 会被 trim 成 "k" 从而被接受，等于放宽 KV 命名空间隔离。
func TestNoTrimFlagsArePinned(t *testing.T) {
	want := map[string][]string{
		"api.kv.get":             {"key"},
		"api.kv.set":             {"key", "value"},
		"api.notifications.send": {"body"},
	}
	for method, fields := range want {
		spec, ok := findAPI(method)
		require.True(t, ok)
		got := map[string]bool{}
		for _, f := range spec.params {
			if f.NoTrim {
				got[f.Name] = true
			}
		}
		for _, name := range fields {
			require.True(t, got[name], "%s 的 %s 必须 NoTrim", method, name)
		}
		require.Len(t, got, len(fields), "%s 不应有额外的 NoTrim 字段", method)
	}
}

// 写类规格必须声明审计，读类必须不声明——这是 adapt 里那个
// `if action == "" { action = s.method }` 兜底之外唯一的强制约束。
//
// 没有这条时，一个漏写 audit 的写类规格仍然会产出一条审计记录，
// 只是 action 退化成方法名、target 变成空串，而 TestWriteAPIsAreAudited
// 只数字节数，照样全绿——正是本重构要消灭的那种静默漂移。
func TestWriteSpecsDeclareAudit(t *testing.T) {
	for _, s := range apiSpecs {
		if s.write {
			require.False(t, s.audit.isZero(),
				"%s 是写类但 audit 为零值：审计会静默退化成方法名", s.method)
			require.NotEmpty(t, s.audit.Action, "%s 的 audit.Action 不能为空", s.method)
			continue
		}
		require.True(t, s.audit.isZero(),
			"%s 不是写类却声明了 audit（读类不应落审计）", s.method)
	}
}

// 审计引用的字段必须真实存在于该规格的 params 里。
//
// 改了 params 里的字段名却忘了同步 audit.Target/Detail 时，joinArgs 会
// 取到空串——不报错，只是审计里少了一个标识。这条把它变成编译期之后
// 第一个会红的东西。
func TestAuditFieldReferencesExistInParams(t *testing.T) {
	for _, s := range apiSpecs {
		declared := make(map[string]bool, len(s.params))
		for _, f := range s.params {
			declared[f.Name] = true
		}
		for _, name := range s.audit.Target {
			require.True(t, declared[name],
				"%s 的 audit.Target 引用了不存在的参数 %q", s.method, name)
		}
		for _, name := range s.audit.Detail {
			require.True(t, declared[name],
				"%s 的 audit.Detail 引用了不存在的参数 %q", s.method, name)
		}
	}
}

// 反向校验：每个已声明能力都必须被 ≥1 个 API 使用。
//
// 重构前只有正向（每个 API 都有已知能力）。缺少反向的后果是真实的：
// posts:read 与 comments:read 一直是空的——管理员在装机确认页批准它们时，
// 批准的是一条不存在的能力，插件拿不到任何对应 API。
//
// 白名单里的是已知的空能力，待裁定删除还是接上真实 API
// （见 docs/superpowers/specs/2026-11-06-plugin-capability-config-design.md §5.3）。
// **这张表只许变短**：新增条目等于给"先声明后实现"开口子。
func TestEveryCapabilityUsedByAnAPI(t *testing.T) {
	used := make(map[string]bool)
	for _, cap := range apiCapabilities() {
		used[cap] = true
	}

	for _, cap := range KnownCapabilities() {
		if used[cap] {
			continue
		}
		_, allowed := capabilityWithoutAPI[cap]
		require.True(t, allowed,
			"能力 %q 没有被任何 API 使用，且不在 capabilityWithoutAPI 白名单里。"+
				"要么给它接上 API，要么从 knownCapabilities 删除它", cap)
	}
}

// 白名单本身不得包含已被使用的能力——那会让上面那条测试失去意义。
func TestCapabilityWithoutAPIListIsAccurate(t *testing.T) {
	used := make(map[string]bool)
	for _, cap := range apiCapabilities() {
		used[cap] = true
	}
	for cap := range capabilityWithoutAPI {
		require.False(t, used[cap],
			"%q 已经在被 API 使用，应从 capabilityWithoutAPI 移除", cap)
		require.True(t, IsKnownCapability(cap),
			"%q 不在 knownCapabilities 里，白名单出现幽灵条目", cap)
	}

	// 白名单只许变短：keys 必须是 {CapPostsRead, CapCommentsRead} 的子集。
	//
	// 不钉住时白名单可以静默扩张——把某个已用能力指向别的 cap 再加进白名单，
	// 全套测试仍然全绿（TestEveryCapabilityUsedByAnAPI 只查「未被使用的能力
	// 是否在白名单里」，查不出「白名单里多了新条目」）。
	//
	// 用 Subset 而不是 ElementsMatch：收缩（修掉 §5.3）应当无需改测试，
	// 只有扩张才需要显式改这里并说明理由。
	keys := make([]string, 0, len(capabilityWithoutAPI))
	for cap := range capabilityWithoutAPI {
		keys = append(keys, cap)
	}
	require.Subset(t,
		[]string{CapPostsRead, CapCommentsRead}, keys,
		"capabilityWithoutAPI 只许变短：新增条目必须同步修改本断言并说明理由")
}

// 插件侧的状态枚举字面量与 service 层权威常量必须一致。
//
// 生产代码不能 import internal/model（会把 gorm 带进来），所以 Enum 只能是
// 字符串字面量。这条测试用**内部测试包**（package plugin）把它钉回去：
// service 层加了新状态而这里忘了同步时，测试变红。
//
// 实测：package plugin 的 _test.go import internal/model 后，
// go list -deps ./internal/plugin 的 gorm 计数仍为 0（它只算非测试构建图）。
func TestStatusEnumMatchesModelConstants(t *testing.T) {
	want := []string{
		model.PostStatusApproved,
		model.PostStatusPending,
		model.PostStatusRejected,
		model.PostStatusClosed,
	}

	for _, m := range []string{"api.posts.setStatus", "api.comments.setStatus"} {
		spec, ok := findAPI(m)
		require.True(t, ok, "%s 必须存在", m)

		var got []string
		for _, f := range spec.params {
			if f.Name == "status" {
				got = f.Enum
			}
		}
		require.NotEmpty(t, got, "%s 的 status 必须声明 Enum（供文档展示）", m)
		require.Equal(t, want, got, "%s 的 status 枚举与 model 常量不一致", m)
	}
}
