package plugin

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// 必填字段缺失时报错，且**点名**字段（与 requireFields 的既有文案一致）。
func TestDecodeArgsRejectsMissingRequired(t *testing.T) {
	fields := []Field{
		{Name: "post_id", Type: FieldString, Required: true},
		{Name: "status", Type: FieldString, Required: true},
	}
	_, err := decodeArgs(json.RawMessage(`{"post_id":"p1"}`), fields)
	require.Error(t, err)
	require.Contains(t, err.Error(), "status")
	require.NotContains(t, err.Error(), "post_id", "只要缺失的字段出现在错误里")
}

// 纯空白等同于缺失。
func TestDecodeArgsTreatsBlankAsMissing(t *testing.T) {
	fields := []Field{{Name: "post_id", Type: FieldString, Required: true}}
	_, err := decodeArgs(json.RawMessage(`{"post_id":"   "}`), fields)
	require.Error(t, err)
	require.Contains(t, err.Error(), "post_id")
}

// 多个字段缺失时按名字排序，错误文案可逐字断言。
func TestDecodeArgsSortsMissingNames(t *testing.T) {
	fields := []Field{
		{Name: "zeta", Type: FieldString, Required: true},
		{Name: "alpha", Type: FieldString, Required: true},
	}
	_, err := decodeArgs(json.RawMessage(`{}`), fields)
	require.Error(t, err)
	require.Equal(t, "参数缺失: alpha, zeta", err.Error())
}

// AllowEmpty 让空串合法（只有 api.kv.set 的 value 需要）。
func TestDecodeArgsAllowsEmptyWhenOptedIn(t *testing.T) {
	fields := []Field{
		{Name: "key", Type: FieldString, Required: true},
		{Name: "value", Type: FieldString, Required: true, AllowEmpty: true},
	}
	a, err := decodeArgs(json.RawMessage(`{"key":"k","value":""}`), fields)
	require.NoError(t, err)
	require.Equal(t, "k", a.String("key"))
	require.Equal(t, "", a.String("value"))
}

// String() 返回 TrimSpace 后的值——这是既有 handler 里 10 处 strings.TrimSpace 的替代。
func TestArgsStringTrims(t *testing.T) {
	fields := []Field{{Name: "tag", Type: FieldString, Required: true}}
	a, err := decodeArgs(json.RawMessage(`{"tag":"  auto  "}`), fields)
	require.NoError(t, err)
	require.Equal(t, "auto", a.String("tag"))
}

// 未知字段被容忍（不 DisallowUnknownFields），这是既有语义。
func TestDecodeArgsToleratesUnknownFields(t *testing.T) {
	fields := []Field{{Name: "post_id", Type: FieldString, Required: true}}
	a, err := decodeArgs(json.RawMessage(`{"post_id":"p1","extra":[1,2,3]}`), fields)
	require.NoError(t, err)
	require.Equal(t, "p1", a.String("post_id"))
}

// 空 params 且无必填字段是合法的。
func TestDecodeArgsAllowsEmptyParamsWhenNothingRequired(t *testing.T) {
	a, err := decodeArgs(nil, []Field{{Name: "x", Type: FieldString}})
	require.NoError(t, err)
	require.Equal(t, "", a.String("x"))
	require.False(t, a.Has("x"))
}

// 坏 JSON 报错，文案与既有 decodeParams 一致（"参数不是合法 JSON 对象"）。
func TestDecodeArgsRejectsBadJSON(t *testing.T) {
	_, err := decodeArgs(json.RawMessage(`not json`), []Field{{Name: "a", Type: FieldString}})
	require.Error(t, err)
	require.Contains(t, err.Error(), "参数不是合法 JSON 对象")
}

// 类型不符时报错而不是静默零值。
func TestDecodeArgsRejectsWrongType(t *testing.T) {
	fields := []Field{{Name: "sort", Type: FieldInt}}
	_, err := decodeArgs(json.RawMessage(`{"sort":"abc"}`), fields)
	require.Error(t, err)
	require.Contains(t, err.Error(), "必须是整数")
}

// NoTrim 字段保留原始值（首尾空白不丢），用于 KV key 与通知正文。
func TestDecodeArgsNoTrimPreservesValue(t *testing.T) {
	fields := []Field{{Name: "key", Type: FieldString, Required: true, NoTrim: true}}
	a, err := decodeArgs(json.RawMessage(`{"key":" k "}`), fields)
	require.NoError(t, err)
	require.Equal(t, " k ", a.String("key"), "NoTrim 必须原样保留")
}

// NoTrim 不影响「必填非空」判定：纯空白仍然算缺失。
func TestDecodeArgsNoTrimStillRejectsBlank(t *testing.T) {
	fields := []Field{{Name: "key", Type: FieldString, Required: true, NoTrim: true}}
	_, err := decodeArgs(json.RawMessage(`{"key":"   "}`), fields)
	require.Error(t, err)
	require.Contains(t, err.Error(), "key")
}

// Int 的 happy path：整数真的被存进 map。
func TestArgsIntStoresValue(t *testing.T) {
	fields := []Field{
		{Name: "sort", Type: FieldInt},
		{Name: "name", Type: FieldString},
	}
	a, err := decodeArgs(json.RawMessage(`{"sort":7,"name":"x"}`), fields)
	require.NoError(t, err)
	require.Equal(t, 7, a.Int("sort"))
	require.True(t, a.Has("sort"))
}

// Raw 的 happy path：对象字段的原始 JSON 被保留。
func TestArgsRawStoresObject(t *testing.T) {
	fields := []Field{{Name: "props", Type: FieldObject}}
	a, err := decodeArgs(json.RawMessage(`{"props":{"text":"hi"}}`), fields)
	require.NoError(t, err)

	var got map[string]string
	require.NoError(t, json.Unmarshal(a.Raw("props"), &got))
	require.Equal(t, map[string]string{"text": "hi"}, got)
	require.True(t, a.Has("props"))
}

// 缺省值：Type 为零值 FieldString 时按字符串处理（不 panic）。
func TestFieldZeroTypeIsString(t *testing.T) {
	require.Equal(t, FieldString, FieldType(0))
}
