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
// 同一份声明同时产出两样东西（见 decodeArgs；Task 4 会基于同一份 Field 生成
// JSON Schema，那是第三样，目前还不存在）：
//   - 运行期校验（必填 / 空串 / 类型）
//   - 取值器（Args.String 的合法字段名集合）
//
// 此前同一份参数形状被写了两遍（build 一处、audit 一处，共 17 个匿名 struct），
// 而 JSON Schema 则完全不存在。改一处不报错：audit 按旧名解析得到零值——
// 写操作照常生效，审计目标变成空串。
type Field struct {
	Name     string
	Type     FieldType
	Required bool
	// AllowEmpty 放行空串。只有 api.kv.set 的 value 需要（"清空这个键"是合法语义）。
	AllowEmpty bool
	// NoTrim 表示这个字段的**值**必须原样传递，不做 TrimSpace。
	//
	// 只用于两个字段：KV 的 key 与通知的 body。
	//   - key：pluginKVKey 用 `trimmed != key` 拒绝首尾空白，预先把 " k" trim 成 "k"
	//     会让它静默接受，等于削弱 KV 命名空间隔离（该函数自己的注释就是这么说的）。
	//   - body：通知正文今天是不 trim 原样入库的，trim 会改变已存储内容。
	//
	// 注意：**必填非空的判定仍然用 trim 后的值**（与重构前 requireFields 一致）——
	// 一个纯空白的 key 依然算缺失。NoTrim 只影响存储与传递，不影响校验。
	NoTrim bool
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
// 取值一律走方法而不是导出字段：String() 统一 TrimSpace（NoTrim 字段除外），
// 与重构前 apiSpecs 里逐处 strings.TrimSpace 的行为一致，调用点不会再漏 trim。
// 既有代码里有 24 处 trim（13 处 build + 11 处 audit），但并非处处都 trim——
// KV 的 key 与通知的 body 原本就是原样传递的，那两个字段靠 Field.NoTrim 保留原值。
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
			// 注意：这里与 schema.go 的 schemaType 是一对必须同步的 switch。
			// schemaType 对未映射的类型 panic（构建期可控，且只在 APISchemas 时触发）；
			// 这里**不** panic —— decodeArgs 跑在每次插件调用的热路径上，
			// 把编程错误变成每个调用都返回「宿主内部错误」，比在构建期炸掉难排查得多。
			// （panic 本身不会打崩进程：adapt 有 defer recover，ipc 还有 callHandlerSafely。）
			var s string
			if err := json.Unmarshal(val, &s); err != nil {
				return nil, fmt.Errorf("参数类型错误: %s 必须是字符串", f.Name)
			}
			// 必填非空用 trim 后的值判定（与重构前 requireFields 一致）；
			// 但 NoTrim 字段存储原值，避免把 " k" 静默规整成 "k"。
			if strings.TrimSpace(s) == "" && f.Required && !f.AllowEmpty {
				missing = append(missing, f.Name)
				continue
			}
			if !f.NoTrim {
				s = strings.TrimSpace(s)
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
