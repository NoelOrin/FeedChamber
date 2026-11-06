package plugin

import (
	"encoding/json"
	"fmt"

	"github.com/sirupsen/logrus"
)

// schemaType 把 FieldType 映射成 JSON Schema 的 type。
func schemaType(t FieldType) string {
	switch t {
	case FieldString:
		return "string"
	case FieldInt:
		return "integer"
	case FieldObject:
		return "object"
	case FieldBool:
		return "boolean"
	default:
		// 新增 FieldType 而忘了同步这里时直接 panic，而不是静默当成 string。
		// decodeArgs 的 default 分支会把它当字符串校验，于是「文档写 string、
		// 实际也拒绝非字符串」错得一致且无声——那正是这套重构要消灭的漂移。
		//
		// 这条 panic 由 TestSchemaTypePanicsOnUnmappedFieldType 钉住，
		// 而不是 TestAPISchemasCoverAllMethods —— 后者只比对键集合，
		// 而 apiSpecs 里的每个 FieldType 都有显式 case，永远走不到这里。
		panic(fmt.Sprintf("plugin: 未映射的 FieldType %d（schema 与 decodeArgs 必须同步）", int(t)))
	}
}

// schemaFor 由参数声明生成一条 JSON Schema。
//
// 同一个 []Field 既驱动运行期校验（decodeArgs），也驱动这份文档——
// 因此文档不会与实际校验漂移。这是把参数从 Go 代码变成数据的直接收益：
// 重构前插件作者要调 api.posts.setStatus，只能读 Go 源码。
//
// 字段名为空时 panic：schema 与取值器都按名索引，空名会让两者同时静默失效。
// 这是构建 schema 时（由 APISchemas 触发）就该暴露的编程错误，不是运行期输入问题。
func schemaFor(fields []Field) map[string]any {
	props := make(map[string]any, len(fields))
	required := make([]string, 0, len(fields))

	for _, f := range fields {
		if f.Name == "" {
			panic("plugin: Field.Name 不能为空（schema 与取值器都按名索引）")
		}
		prop := map[string]any{"type": schemaType(f.Type)}
		if f.Desc != "" {
			prop["description"] = f.Desc
		}
		if len(f.Enum) > 0 {
			// 只进文档，运行期不校验：取值域的权威定义在 service 层。
			prop["enum"] = f.Enum
		}
		props[f.Name] = prop
		if f.Required {
			required = append(required, f.Name)
		}
	}

	out := map[string]any{
		"type":       "object",
		"properties": props,
	}
	// 无必填字段时不写 required（空数组在部分客户端会被当成"有 required 但为空"）。
	if len(required) > 0 {
		out["required"] = required
	}
	return out
}

// APISchemas 返回「方法名 → JSON Schema」。
//
// 供插件作者的 IDE 提示、安装确认页与后台展示使用。序列化失败时跳过该条
// 并记日志而不是让整个调用失败——schema 是说明性产物，不该拖住安装流程。
func APISchemas() map[string]json.RawMessage {
	out := make(map[string]json.RawMessage, len(apiSpecs))
	for _, s := range apiSpecs {
		raw, err := json.Marshal(schemaFor(s.params))
		if err != nil {
			logrus.WithError(err).Warnf("plugin: 生成 %s 的 schema 失败", s.method)
			continue
		}
		out[s.method] = raw
	}
	return out
}
