package plugin

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// 每个已注册方法都有 schema，且键集合与 apiMethodNames() 完全一致——
// 不能多也不能少，否则插件作者会看到一个不存在的参数说明。
func TestAPISchemasCoverAllMethods(t *testing.T) {
	schemas := APISchemas()
	require.Len(t, schemas, len(apiMethodNames()))
	for _, m := range apiMethodNames() {
		_, ok := schemas[m]
		require.True(t, ok, "%s 缺少 schema", m)
	}
}

// 必填字段进 required，非必填不进。
func TestSchemaRequiredMatchesFieldDecl(t *testing.T) {
	m := schemaFor([]Field{
		{Name: "post_id", Type: FieldString, Required: true},
		{Name: "note", Type: FieldString},
	})
	req, ok := m["required"].([]string)
	require.True(t, ok, "required 必须是 []string")
	require.Equal(t, []string{"post_id"}, req)

	props := m["properties"].(map[string]any)
	require.Contains(t, props, "post_id")
	require.Contains(t, props, "note")
}

// 无必填字段时不产生 required 键（而不是空数组）。
func TestSchemaOmitsEmptyRequired(t *testing.T) {
	m := schemaFor([]Field{{Name: "a", Type: FieldString}})
	_, has := m["required"]
	require.False(t, has, "没有必填字段时不应出现 required")
}

// Enum 写进属性（供文档展示），这是它唯一的用途。
func TestSchemaCarriesEnum(t *testing.T) {
	m := schemaFor([]Field{
		{Name: "status", Type: FieldString, Required: true,
			Enum: []string{"approved", "pending"}},
	})
	props := m["properties"].(map[string]any)
	status := props["status"].(map[string]any)
	require.Equal(t, []string{"approved", "pending"}, status["enum"])
	require.Equal(t, "string", status["type"])
}

// 类型映射正确。
func TestSchemaFieldTypes(t *testing.T) {
	m := schemaFor([]Field{
		{Name: "n", Type: FieldInt},
		{Name: "o", Type: FieldObject},
		{Name: "s", Type: FieldString},
	})
	props := m["properties"].(map[string]any)
	require.Equal(t, "integer", props["n"].(map[string]any)["type"])
	require.Equal(t, "object", props["o"].(map[string]any)["type"])
	require.Equal(t, "string", props["s"].(map[string]any)["type"])
}

// schema 必须可序列化为合法 JSON（它会经 JSON-RPC 发给插件）。
func TestAPISchemasMarshalToValidJSON(t *testing.T) {
	for name, raw := range APISchemas() {
		var v any
		require.NoError(t, json.Unmarshal(raw, &v), "%s 的 schema 不是合法 JSON", name)
	}
}

// 非法字段名在注册期就该被拦住：schema 与取值器都按名索引，
// 空名会让两者同时静默失效。
func TestSchemaRejectsEmptyFieldName(t *testing.T) {
	require.Panics(t, func() {
		schemaFor([]Field{{Name: "", Type: FieldString}})
	})
}
