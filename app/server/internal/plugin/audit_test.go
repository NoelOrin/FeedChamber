package plugin

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// Target/Detail 按字段名取值并按序拼接。
func TestAuditSpecExtractsDeclaredFields(t *testing.T) {
	a, err := decodeArgs(json.RawMessage(`{"post_id":"p1","status":"pending"}`), []Field{
		{Name: "post_id", Type: FieldString, Required: true},
		{Name: "status", Type: FieldString, Required: true},
	})
	require.NoError(t, err)

	spec := AuditSpec{Action: "post.setStatus", Target: []string{"post_id"}, Detail: []string{"status"}}
	action, target, detail := spec.extract("test-plugin", a)
	require.Equal(t, "post.setStatus", action)
	require.Equal(t, "p1", target)
	require.Equal(t, "pending", detail)
}

// 多个字段按声明顺序拼接，分隔符是单个空格。
//
// 两侧**都**要至少两个字段：此前两处都只有一个元素，" 拼接"这件事
// 完全没有被执行到——把 joinArgs 改成 strings.Join(parts, "") 或 "," 仍是全绿。
func TestAuditSpecJoinsMultipleFields(t *testing.T) {
	a, err := decodeArgs(json.RawMessage(`{"slot":"header","kind":"text","zone":"top","mode":"dark"}`), []Field{
		{Name: "slot", Type: FieldString, Required: true},
		{Name: "kind", Type: FieldString, Required: true},
		{Name: "zone", Type: FieldString, Required: true},
		{Name: "mode", Type: FieldString, Required: true},
	})
	require.NoError(t, err)

	spec := AuditSpec{Action: "slot.set", Target: []string{"slot", "zone"}, Detail: []string{"kind", "mode"}}
	_, target, detail := spec.extract("test-plugin", a)
	require.Equal(t, "header top", target, "多字段按声明顺序用空格拼接")
	require.Equal(t, "text dark", detail, "Detail 同样按序拼接")
}

// 单字段时拼接不引入任何多余字符（与多字段形成对照）。
func TestAuditSpecSingleFieldHasNoSeparator(t *testing.T) {
	a, err := decodeArgs(json.RawMessage(`{"slot":"header","kind":"text"}`), []Field{
		{Name: "slot", Type: FieldString, Required: true},
		{Name: "kind", Type: FieldString, Required: true},
	})
	require.NoError(t, err)

	spec := AuditSpec{Action: "slot.set", Target: []string{"slot"}, Detail: []string{"kind"}}
	_, target, detail := spec.extract("test-plugin", a)
	require.Equal(t, "header", target)
	require.Equal(t, "text", detail)
}

// 未声明 Detail 时为空串（api.slots.delete 就是这样）。
func TestAuditSpecEmptyDetail(t *testing.T) {
	a, err := decodeArgs(json.RawMessage(`{"slot":"header"}`), []Field{
		{Name: "slot", Type: FieldString, Required: true},
	})
	require.NoError(t, err)

	spec := AuditSpec{Action: "slot.delete", Target: []string{"slot"}}
	_, target, detail := spec.extract("test-plugin", a)
	require.Equal(t, "header", target)
	require.Equal(t, "", detail)
}

// AuditScopePluginKV 把 target 过 pluginKVKey，落**加过前缀的全键**。
// 审计里出现裸 "counter" 无法定位到某个插件。
func TestAuditSpecScopesPluginKVTarget(t *testing.T) {
	a, err := decodeArgs(json.RawMessage(`{"key":"counter","value":"1"}`), []Field{
		{Name: "key", Type: FieldString, Required: true},
		{Name: "value", Type: FieldString, Required: true, AllowEmpty: true},
	})
	require.NoError(t, err)

	spec := AuditSpec{Action: "kv.set", Target: []string{"key"}, Scope: AuditScopePluginKV}
	_, target, _ := spec.extract("alpha", a)
	require.Equal(t, "plugin:alpha:counter", target)
}

// 非法 key 让 pluginKVKey 报错时，回退到未加前缀的原值——
// 与重构前 audit 闭包的行为一致（它也是 err 时返回 in.Key）。
func TestAuditSpecFallsBackWhenScopeFails(t *testing.T) {
	a, err := decodeArgs(json.RawMessage(`{"key":"../escape"}`), []Field{
		{Name: "key", Type: FieldString, Required: true},
	})
	require.NoError(t, err, "参数校验不拦截 KV 键形态，那是 pluginKVKey 的职责")

	spec := AuditSpec{Action: "kv.set", Target: []string{"key"}, Scope: AuditScopePluginKV}
	_, target, _ := spec.extract("alpha", a)
	require.Equal(t, "../escape", target, "pluginKVKey 失败时回退原值")
}

// 零值 AuditSpec 表示「无审计」（读类 API）。
func TestAuditSpecIsZero(t *testing.T) {
	require.True(t, AuditSpec{}.isZero())
	require.False(t, AuditSpec{Action: "x"}.isZero())
	require.False(t, AuditSpec{Target: []string{"a"}}.isZero())
}
