package plugin

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// FieldType 是参数的类型。零值必须是 FieldString——新增 Field 时漏写 Type
// 应当退化成最常见的字符串，而不是某个不可用的零值。
type FieldType int

const (
	FieldString FieldType = iota
	FieldInt
	FieldObject
	FieldBool
)

// Field 声明一个宿主 API 参数。
//
// 同一份声明同时产出三样东西（见 schema.go 与 decodeArgs）：
//   - 运行期校验（必填 / 空串 / 类型）
//   - 取值器（Args.String 的合法字段名集合）
//   - JSON Schema（给插件作者与安装确认页）
//
// 这三样此前是三份手写数据，漂移是静默的：build 改了字段名、audit 仍按旧名
// json.Unmarshal，得到零值——写操作照常生效，审计目标变成空串。
type Field struct {
	Name     string
	Type     FieldType
	Required bool
	// AllowEmpty 放行空串。只有 api.kv.set 的 value 需要（"清空这个键"是合法语义）。
	AllowEmpty bool
	// Enum 只写进 JSON Schema 供文档展示，**运行期不校验**。
	//
	// 取值域的权威定义在 service 层（post.go / content.go 的 isValidPostStatus）。
	// 在这里再校验一遍会产生第二个事实来源：加枚举值时改 service 忘了改这里，
	// 症状是「文档说支持、实际被拒」。不要"顺手补上"。
	Enum []string
	Desc string
}

// Args 是一次调用解码并校验后的参数集合。
//
// 取值一律走方法而不是导出字段：String() 统一 TrimSpace，与重构前 10 处
// strings.TrimSpace 的行为保持一致，调用点不会再漏 trim。
type Args struct {
	strs map[string]string
	ints map[string]int
	raws map[string]json.RawMessage
}

// decodeArgs 把原始 JSON 解码成 Args 并按 fields 校验。
//
// 坏 JSON 的文案与重构前的 decodeParams 一致（"参数不是合法 JSON 对象"），
// 缺失字段的文案与 requireFields 一致（"参数缺失: a, b"，按名排序）——
// 插件侧对这两条文案有依赖，重构不得改变。
func decodeArgs(params json.RawMessage, fields []Field) (*Args, error) {
	// 先解成通用 map：这样可以按 fields 逐个取值并做类型校验，
	// 又不引入 DisallowUnknownFields（插件多传字段必须被容忍）。
	var raw map[string]json.RawMessage
	if len(params) > 0 {
		if err := json.Unmarshal(params, &raw); err != nil {
			return nil, fmt.Errorf("参数不是合法 JSON 对象: %w", err)
		}
	}
	if raw == nil {
		raw = map[string]json.RawMessage{}
	}

	a := &Args{
		strs: make(map[string]string, len(fields)),
		ints: make(map[string]int, len(fields)),
		raws: make(map[string]json.RawMessage, len(fields)),
	}

	var missing []string
	for _, f := range fields {
		val, ok := raw[f.Name]
		if !ok {
			if f.Required {
				missing = append(missing, f.Name)
			}
			continue
		}
		switch f.Type {
		case FieldInt:
			var n int
			if err := json.Unmarshal(val, &n); err != nil {
				return nil, fmt.Errorf("参数类型错误: %s 必须是整数", f.Name)
			}
			a.ints[f.Name] = n
		case FieldObject:
			a.raws[f.Name] = val
		case FieldBool:
			var b bool
			if err := json.Unmarshal(val, &b); err != nil {
				return nil, fmt.Errorf("参数类型错误: %s 必须是布尔值", f.Name)
			}
			if b {
				a.ints[f.Name] = 1
			}
		default: // FieldString
			var s string
			if err := json.Unmarshal(val, &s); err != nil {
				return nil, fmt.Errorf("参数类型错误: %s 必须是字符串", f.Name)
			}
			s = strings.TrimSpace(s)
			if s == "" && f.Required && !f.AllowEmpty {
				missing = append(missing, f.Name)
				continue
			}
			a.strs[f.Name] = s
		}
	}

	if len(missing) > 0 {
		// 排序后报错：同一个请求永远得到同一条文案，测试可逐字断言。
		sort.Strings(missing)
		return nil, fmt.Errorf("参数缺失: %s", strings.Join(missing, ", "))
	}
	return a, nil
}

// String 返回 TrimSpace 后的字符串参数。字段不存在时返回空串。
func (a *Args) String(name string) string {
	if a == nil {
		return ""
	}
	return a.strs[name]
}

// Int 返回整数参数。字段不存在时返回零值。
func (a *Args) Int(name string) int {
	if a == nil {
		return 0
	}
	return a.ints[name]
}

// Raw 返回对象的原始 JSON（供 props 这类透传字段）。
func (a *Args) Raw(name string) json.RawMessage {
	if a == nil {
		return nil
	}
	return a.raws[name]
}

// Has 报告参数字段是否实际存在。
func (a *Args) Has(name string) bool {
	if a == nil {
		return false
	}
	if _, ok := a.strs[name]; ok {
		return true
	}
	if _, ok := a.ints[name]; ok {
		return true
	}
	_, ok := a.raws[name]
	return ok
}
