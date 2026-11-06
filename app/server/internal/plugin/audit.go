package plugin

import "strings"

// AuditScope 决定 target 是否需要二次变换。
//
// 目前只有一种：插件的 KV 键要在审计里落**加过前缀的全键**，
// 裸 "counter" 无法定位到某个插件。
type AuditScope int

const (
	AuditScopePlain AuditScope = iota
	AuditScopePluginKV
)

// AuditSpec 把审计落点声明成字段引用，而不是第二个 json.Unmarshal 闭包。
//
// 重构前每个写类 API 都有一份匿名 struct + Unmarshal 来提取审计字段，
// 与 build 里那份是两份数据：build 改了字段名、audit 仍按旧名解析，
// json.Unmarshal 静默返回零值——写操作生效、审计目标变空串，且不报错。
type AuditSpec struct {
	Action string
	// Target / Detail 是参数名，按序拼接。
	Target []string
	Detail []string
	// Scope 决定 target 是否要再做一次变换（零值 = 原样）。
	//
	// AuditScopePluginKV 会**覆盖**整个 joinArgs 的结果：target 变成
	// Target[0] 经 pluginKVKey 加过前缀的全键。因此它要求 len(Target) == 1——
	// 声明了两个以上字段时，除第一个以外的都会被静默丢弃。
	Scope AuditScope
}

// isZero 报告这是否是「无审计」。只有读类 API 应为 true。
func (s AuditSpec) isZero() bool {
	return s.Action == "" && len(s.Target) == 0 && len(s.Detail) == 0
}

// extract 按声明从已解码的参数里取审计三元组。
//
// a 已经在 decodeArgs 阶段校验过，因此这里不会因为缺字段而 panic；
// 但字段可能确实不存在（非必填且未传），此时取到空串——与重构前一致。
func (s AuditSpec) extract(pluginID string, a *Args) (action, target, detail string) {
	action = s.Action
	target = joinArgs(a, s.Target)
	detail = joinArgs(a, s.Detail)

	if s.Scope == AuditScopePluginKV && len(s.Target) > 0 {
		raw := a.String(s.Target[0])
		if scoped, err := pluginKVKey(pluginID, raw); err == nil {
			target = scoped
		}
		// pluginKVKey 失败时保留原值：业务调用大概率也已经失败，
		// 但审计仍应记录插件试图访问的键，便于排查。
	}
	return action, target, detail
}

// joinArgs 按字段名顺序取值，用空格拼接。
//
// 不做别的过滤：空名不是需要在这里兜住的输入——audit.Target/Detail 是
// Go 字面量，写错了由 TestAuditFieldReferencesExistInParams 直接报红，
// 而不是让这里静默跳过一段、产出一个少了一截的 target。
func joinArgs(a *Args, names []string) string {
	if len(names) == 0 {
		return ""
	}
	parts := make([]string, 0, len(names))
	for _, n := range names {
		parts = append(parts, a.String(n))
	}
	return strings.Join(parts, " ")
}
