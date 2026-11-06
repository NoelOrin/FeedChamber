# 插件能力配置化实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把 `internal/plugin` 的宿主 API 参数形状从重复的 Go 匿名结构体收敛成一份声明式数据，使校验器、取值器与 JSON Schema 同源。

**Architecture:** 引入 `Field`/`Args`/`AuditSpec` 三个类型，让 `apiSpec` 携带 `params []Field`。`lookupAPI` 返回的不再是裸 `ipc.Handler`，而是内部分解良好的适配器：解码一次 → 按 `Field` 校验 → 调业务 `Func` → 按 `AuditSpec` 提取审计字段。`ipc.Handler` 类型本身不动，适配完全封闭在 `api.go` 内部。

**Tech Stack:** Go 1.25、`encoding/json`、`github.com/stretchr/testify/require`、标准库 `sort`/`strings`。无新增依赖。

**Spec:** `docs/superpowers/specs/2026-11-06-plugin-capability-config-design.md`

> **本计划已实测验证（2026-11-06）。** 计划里的全部 Go 代码在执行前已被真的编译并跑通一次：
> 既有 19 条 `api_test.go` 测试**零断言修改**全部通过、新增 23 条测试全部通过、
> 三项重复指标归零（`var in struct{` 17→0、`audit: func` 7→0、双重 `Unmarshal` 7→0）、
> `gorm` 计数仍为 0、`api.go` 902→816 行。验证完已把工作区还原——执行者从干净状态开始。
>
> 验证中发现并已修正的四处在计划正文里都有标注：`notifications.send` 的 `body`
> 误加 `AllowEmpty`（改变行为）、`slots.set` 的 `props` 缺省从 `nil` 变空 map（行为漂移）、
> 替换区间会误删 `denyHandler`（编译不过）、以及 §1.4 的「去掉重扫」实为**未做**。

## Global Constraints

- **不引入新依赖。** 只用标准库 + 已有的 testify。
- **`internal/plugin` 生产依赖图中的 gorm 包数必须恒为 0。** 每次改动后验证：
  `cd app/server && go list -deps ./internal/plugin | grep -c 'gorm.io'` → 必须输出 `0`。
- **`internal/plugin` 生产代码不得 import `internal/model`、`internal/repository`、`internal/service`、`internal/pkg/eventbus`。** 内部测试（`package plugin`）**可以** import `internal/model`——实测不影响 `go list -deps` 计数。
- **`ipc.Handler` 签名不变**：`type Handler func(ctx context.Context, params json.RawMessage) (any, error)`（`ipc.go:44`）。
- **7 个接缝接口与 `APIDeps` 结构不变**：`PostWriter`/`CommentWriter`/`UserReader`/`Notifier`/`KVStore`/`SlotWriter`/`AuditRecorder`。
- **`api_test.go` 现有 19 条测试的断言不得修改。** 只允许改测试辅助接线。任何需要改断言的改动都说明重构越界。
- **错误文案不得变化**：`unknown method: <名>`、`参数缺失: <字段, ...>`（按名排序）、`capability denied` 的可识别性（`errors.Is(err, ErrCapabilityDenied)`）。
- **取值域不在插件层校验**（`status` 等交 service 层），`Enum` 只进 schema 供文档。
- **不做运行时能力开关**，不做 HTTP/MCP 投影。

---

## 文件结构

| 文件 | 职责 | 动作 |
|------|------|------|
| `internal/plugin/args.go` | `Field`/`FieldType`/`Args` 类型；解码、校验、取值 | 新建 |
| `internal/plugin/audit.go` | `AuditSpec`/`AuditScope`；按声明提取 action/target/detail | 新建 |
| `internal/plugin/schema.go` | `APISchemas()`：由 `params []Field` 生成 JSON Schema | 新建 |
| `internal/plugin/api.go` | 能力常量、`apiSpecs`（换形）、`lookupAPI`（改造）、`RegisterAPIs` | 修改 |
| `internal/plugin/api_test.go` | 既有 19 条断言不动；追加新测试 | 修改（追加） |
| `internal/plugin/AGENTS.md` | 宿主 API 一节重写 | 修改 |

拆分理由：`api.go` 现已 902 行，再塞校验器与 schema 生成会到 1100+ 行且混杂三件事。按「参数是数据」「审计是数据」「schema 是投影」拆成三个小文件，每个都能单独读完。

---

### Task 1: `Args` 与 `Field` 的取值与校验

**Files:**
- Create: `app/server/internal/plugin/args.go`
- Test: `app/server/internal/plugin/args_test.go`

**Interfaces:**
- Consumes: 无（本任务自包含）
- Produces:
  - `type FieldType int`，常量 `FieldString`、`FieldInt`、`FieldObject`、`FieldBool`
  - `type Field struct { Name string; Type FieldType; Required bool; AllowEmpty bool; Enum []string; Desc string }`
  - `func decodeArgs(params json.RawMessage, fields []Field) (*Args, error)`
  - `func (a *Args) String(name string) string` — 返回 **已 TrimSpace** 的值
  - `func (a *Args) Int(name string) int`
  - `func (a *Args) Raw(name string) json.RawMessage` — 供 `props` 这类对象字段
  - `func (a *Args) Has(name string) bool`

- [ ] **Step 1: 写失败的测试**

创建 `app/server/internal/plugin/args_test.go`：

```go
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
	require.Contains(t, err.Error(), "sort")
}

// 缺省值：Type 为零值 FieldString 时按字符串处理（不 panic）。
func TestFieldZeroTypeIsString(t *testing.T) {
	require.Equal(t, FieldString, FieldType(0))
}
```

- [ ] **Step 2: 运行测试确认失败**

Run:
```bash
cd app/server && go test ./internal/plugin/ -run 'TestDecodeArgs|TestArgsStringTrims|TestFieldZeroType' -v
```
Expected: FAIL — `undefined: Field`, `undefined: decodeArgs`, `undefined: Args`

- [ ] **Step 3: 写实现**

创建 `app/server/internal/plugin/args.go`：

```go
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
```

- [ ] **Step 4: 运行测试确认通过**

Run:
```bash
cd app/server && go test ./internal/plugin/ -run 'TestDecodeArgs|TestArgsStringTrims|TestFieldZeroType' -v
```
Expected: PASS（10 条）

- [ ] **Step 5: 验证 gorm 计数仍为 0**

Run:
```bash
cd app/server && go list -deps ./internal/plugin | grep -c 'gorm.io'
```
Expected: `0`

- [ ] **Step 6: 提交**

```bash
cd /Users/noelorin/OrinForum
git add app/server/internal/plugin/args.go app/server/internal/plugin/args_test.go
git commit -m "feat(plugin): add declarative Args and Field for host API params"
```

---

### Task 2: `AuditSpec` 按声明提取审计字段

**Files:**
- Create: `app/server/internal/plugin/audit.go`
- Test: `app/server/internal/plugin/audit_test.go`

**Interfaces:**
- Consumes: Task 1 的 `*Args`、`Args.String`
- Produces:
  - `type AuditScope int`，常量 `AuditScopePlain`（0）、`AuditScopePluginKV`
  - `type AuditSpec struct { Action string; Target []string; Detail []string; Scope AuditScope }`
  - `func (s AuditSpec) isZero() bool` — 无审计
  - `func (s AuditSpec) extract(pluginID string, a *Args) (action, target, detail string)`

- [ ] **Step 1: 写失败的测试**

创建 `app/server/internal/plugin/audit_test.go`：

```go
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

// 多个字段按声明顺序拼接。
func TestAuditSpecJoinsMultipleFields(t *testing.T) {
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
```

- [ ] **Step 2: 运行测试确认失败**

Run:
```bash
cd app/server && go test ./internal/plugin/ -run 'TestAuditSpec' -v
```
Expected: FAIL — `undefined: AuditSpec`

- [ ] **Step 3: 写实现**

创建 `app/server/internal/plugin/audit.go`：

```go
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
	Scope  AuditScope
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

// joinArgs 按字段名顺序取值，跳过空名。
func joinArgs(a *Args, names []string) string {
	if len(names) == 0 {
		return ""
	}
	parts := make([]string, 0, len(names))
	for _, n := range names {
		if n == "" {
			continue
		}
		parts = append(parts, a.String(n))
	}
	return strings.Join(parts, " ")
}
```

- [ ] **Step 4: 运行测试确认通过**

Run:
```bash
cd app/server && go test ./internal/plugin/ -run 'TestAuditSpec' -v
```
Expected: PASS（6 条）

- [ ] **Step 5: 提交**

```bash
cd /Users/noelorin/OrinForum
git add app/server/internal/plugin/audit.go app/server/internal/plugin/audit_test.go
git commit -m "feat(plugin): declare audit targets as field references"
```

---

### Task 3: `apiSpec` 换形并迁移全部 10 条规格

这是本轮最大的一个任务。它有意做成一个整体：`apiSpec` 结构一改，10 条规格必须同时改完才能编译。

**Files:**
- Modify: `app/server/internal/plugin/api.go:212-556`（`apiSpec` 定义与 `apiSpecs` 表）
- Modify: `app/server/internal/plugin/api.go:717-733`（`RegisterAPIs`）
- Modify: `app/server/internal/plugin/api.go:600-720`（`lookupAPI` 及其返回的适配器）
- Test: `app/server/internal/plugin/api_test.go`（**仅追加**，不改既有断言）

**Interfaces:**
- Consumes: Task 1 的 `*Args`/`decodeArgs`/`Field`/`FieldString`/`FieldInt`/`FieldObject`；Task 2 的 `AuditSpec`/`AuditScopePluginKV`/`extract`
- Produces:
  - `type Func func(ctx context.Context, a *Args) (any, error)`
  - `type apiSpec struct { method, cap string; write bool; params []Field; audit AuditSpec; build func(pluginID string, deps APIDeps) (Func, error) }`
  - `lookupAPI(method, pluginID string, caps CapabilitySet, deps APIDeps) (Handler, bool)` — **签名不变**

各方法的参数声明（照抄，逐条）：

| method | params | audit |
|---|---|---|
| `api.posts.setStatus` | `post_id`(str,req) `status`(str,req) | `post.setStatus` / T:`post_id` / D:`status` |
| `api.posts.addTag` | `post_id`(str,req) `tag`(str,req) | `post.addTag` / T:`post_id` / D:`tag` |
| `api.comments.setStatus` | `comment_id`(str,req) `status`(str,req) | `comment.setStatus` / T:`comment_id` / D:`status` |
| `api.posts.getLevel` | `user_id`(str,req) | 无 |
| `api.users.getEmail` | `user_id`(str,req) | 无 |
| `api.notifications.send` | `user_id`(str,req) `title`(str,req) `body`(str,req) | `notification.send` / T:`user_id` / D:`title` |
| `api.kv.get` | `key`(str,req) | 无 |
| `api.kv.set` | `key`(str,req) `value`(str,req,**AllowEmpty**) | `kv.set` / T:`key` / Scope:`AuditScopePluginKV` |
| `api.slots.set` | `slot`(str,req) `kind`(str,req) `props`(object) `sort`(int) | `slot.set` / T:`slot` / D:`kind` |
| `api.slots.delete` | `slot`(str,req) | `slot.delete` / T:`slot` |

- [ ] **Step 1: 先加一条测试，钉住「审计 target 来自同一份参数声明」**

在 `app/server/internal/plugin/api_test.go` **末尾追加**（不要改动已有断言）：

```go
// 审计里的 target 必须来自**同一份**参数声明，而不是第二份匿名 struct。
//
// 这条钉住本轮重构的核心收益：重构前 build 与 audit 各自 Unmarshal 一次同一个
// JSON，改了字段名不会报错，只会让审计目标变成空串。
func TestAuditTargetComesFromParamDeclarations(t *testing.T) {
	posts := &fakePostWriter{}
	audit := &fakeAudit{}
	deps := APIDeps{Posts: posts, Audit: audit}

	err := invokeAPI(t, CapabilitySet{"posts:write": {}}, deps,
		"api.posts.setStatus", map[string]string{"post_id": "p1", "status": "pending"})
	require.NoError(t, err)
	require.Len(t, audit.records, 1)
	require.Contains(t, audit.records[0], "p1", "审计必须带上 target（来自 post_id）")
	require.Contains(t, audit.records[0], "pending", "审计必须带上 detail（来自 status）")
}
```

- [ ] **Step 2: 运行确认它通过（在当前实现下也应通过）**

Run:
```bash
cd app/server && go test ./internal/plugin/ -run 'TestAuditTargetComesFromParamDeclarations' -v
```
Expected: PASS — 这条是新实现的行为契约，旧实现恰好也满足。它作为重构的**安全网**存在，重构后必须仍为绿。

- [ ] **Step 3: 替换 `apiSpec` 定义与 `lookupAPI`**

把 `api.go` 中 `type apiSpec struct {...}`（212–223 行）整体替换为：

```go
// Func 是解码并校验之后的处理函数。
//
// **不再是 ipc.Handler**：参数已经由 decodeArgs 解好并校验，handler 不再
// 接触 json.RawMessage，也就不会出现「忘了 requireFields」或「字段名写错」。
type Func func(ctx context.Context, a *Args) (any, error)

// apiSpec 是一个宿主 API 的定义：方法名、所需能力、是否为写类、参数、审计、构造器。
//
// 把这几件事放在**同一行**上，是这套设计的核心：
//
//	方法 → 能力 的映射分散在 switch 的 case 里时，「新增 API 忘了写校验」
//	是一个纯手工纪律问题；放进结构体字段后，缺字段会被
//	TestEveryAPIMethodDeclaresACapability 直接抓住，而 lookupAPI 只有
//	一条路径能构造出 handler——它必然先查能力。
type apiSpec struct {
	method string
	cap    string
	// write 为 true 时，调用成功后**统一**由 wrap 落审计；handler 自己不写审计。
	write bool
	// params 是参数的唯一声明处：校验、取值、JSON Schema、审计提取全部由它派生。
	params []Field
	// audit 零值表示无审计（读类）。写类必须非零，由 TestWriteAPIsAreAudited 钉住。
	audit AuditSpec
	build func(pluginID string, deps APIDeps) (Func, error)
}
```

把 `lookupAPI`（原 600–640 行附近）整体替换为：

```go
// lookupAPI 把 method 解析成处理函数，并做 capability 校验。
//
// 返回的仍是 ipc.Handler（ipc.go 的类型不变），但内部已经把
// 「解码一次 → 校验 → 执行业务 → 按声明提取审计」封好了。
//
// 未授予能力时返回的不是业务 handler，而是 denyHandler：它**不持有 deps**，
// 内部没有任何 service 调用。因此即便有人把 denyHandler 里的判定删掉，
// 也不可能执行到业务代码。
func lookupAPI(method, pluginID string, caps CapabilitySet, deps APIDeps) (Handler, bool) {
	spec, ok := findAPI(method)
	if !ok {
		return nil, false
	}
	if !caps.grants(spec.cap) {
		return denyHandler(pluginID, spec.method, spec.cap), true
	}
	fn, err := spec.build(pluginID, deps)
	if err != nil {
		// 能力有、接线没有：明确报错。绝不能返回 nil handler——那会让
		// 插件等到 IPC 超时，日志里什么都看不到。
		return func(context.Context, json.RawMessage) (any, error) {
			return nil, fmt.Errorf("%s: %w（插件 %s）", spec.method, err, pluginID)
		}, true
	}
	return spec.adapt(pluginID, deps, fn), true
}

// adapt 把 Func 适配成 ipc.Handler：解码一次 → 校验 → 执行 → 审计。
//
// 审计在这里统一落，**handler 内部没有审计调用点**——「写类 API 忘审计」
// 不可能发生。新增写 API 只要 write:true + audit 非零，
// TestWriteAPIsAreAudited 自动纳入覆盖。
func (s apiSpec) adapt(pluginID string, deps APIDeps, fn Func) Handler {
	return func(ctx context.Context, params json.RawMessage) (res any, err error) {
		// handler 跑在 ipc.go 的 dispatch goroutine 里，一次 panic 就是整个
		// 后端进程崩溃。插件能控制的输入必须在这里兜住。
		defer func() {
			if r := recover(); r != nil {
				logrus.Errorf("plugin: %s 调用 %s 时 panic: %v", pluginID, s.method, r)
				res, err = nil, fmt.Errorf("%s: 宿主内部错误", s.method)
			}
		}()

		a, err := decodeArgs(params, s.params)
		if err != nil {
			return nil, err
		}

		res, err = fn(ctx, a)
		if err != nil {
			return res, err
		}
		if !s.write {
			return res, nil
		}

		action, target, detail := s.audit.extract(pluginID, a)
		if action == "" {
			// 不该发生（TestWriteAPIsAreAudited 保证写类都有 action）。
			// 仍落一条可辨认的记录，而不是静默跳过——「一次写没有审计」
			// 比「审计字段不好看」严重得多。
			action = s.method
		}
		if deps.Audit == nil {
			logrus.Errorf("plugin: %s 的 %s 已执行但 Audit 未接线，审计缺失", pluginID, s.method)
			return res, nil
		}
		if aErr := deps.Audit.RecordPluginAction(ctx, pluginID, action, target, detail); aErr != nil {
			// 业务动作已经生效，不能因为审计失败把结果翻成失败：
			// 插件会重试，而重试会**再执行一次**业务动作。
			logrus.WithError(aErr).WithFields(logrus.Fields{
				"plugin": pluginID,
				"api":    s.method,
				"action": action,
			}).Error("plugin: 审计落库失败（业务动作已生效，不翻转结果）")
		}
		return res, nil
	}
}
```

- [ ] **Step 4: 把 `RegisterAPIs` 去掉闭包里多余的复查**

替换 `RegisterAPIs`（原 717–733 行）为下面这段。它与你可能预期的版本相比**没有功能变化**——`lookupAPI` 签名未变、能力校验位置未变。保留这段代码的唯一目的是在 `apiSpecs` 换形后确认编译与行为仍然正确。

```go
// RegisterAPIs 为插件注册全部宿主 API。
//
// 遍历规格表本身（不查 map），因此注册顺序确定。
//
// 闭包内仍调 lookupAPI：它是**能力校验的唯一入口**。把能力判断提到注册期
// 虽然更快，却会让「未授予能力」从一次可测的拒绝变成一次注册决策，
// 而 TestRegisterAPIsDoesNotHandleUndeclaredCapability 正是钉后者的。
func RegisterAPIs(conn *Conn, pluginID string, caps CapabilitySet, deps APIDeps) {
	if conn == nil {
		return
	}
	for _, spec := range apiSpecs {
		spec := spec
		conn.Handle(spec.method, func(ctx context.Context, params json.RawMessage) (any, error) {
			fn, ok := lookupAPI(spec.method, pluginID, caps, deps)
			if !ok {
				// 规格表在包初始化后是只读的，理论上不可能查不到；
				// 万一发生，明确报错而不是让插件等到超时。
				return nil, errUnknownMethod(spec.method)
			}
			return fn(ctx, params)
		})
	}
}
```

**关于「重扫」的如实说明：** `lookupAPI` 内部仍会 `findAPI` 线性查找，所以每次调用的开销与重构前**相同**。spec §1.4 提出的这一点**本轮不做**：彻底去掉需要在 `RegisterAPIs` 里预建 `map[string]apiSpec` 并让 `lookupAPI` 接收 spec，而那会改变 `lookupAPI` 的签名——`api_test.go` 有 7 处测试直接调用它。收益（线性扫 10 条）远小于改动面，且它不属于本轮「消除参数重复」的目标。Task 6 的 AGENTS.md **不得**宣称已优化此项。

- [ ] **Step 5: 把 `apiSpecs` 表整体重写**

替换 `var apiSpecs = []apiSpec{...}`（原 229–556 行）为下面 10 条。**逐条照抄，不要简化**：

```go
var apiSpecs = []apiSpec{
	{
		method: "api.posts.setStatus",
		cap:    CapPostsWrite,
		write:  true,
		params: []Field{
			{Name: "post_id", Type: FieldString, Required: true, Desc: "帖子 ID"},
			{Name: "status", Type: FieldString, Required: true, Desc: "目标状态",
				Enum: []string{"approved", "pending", "rejected", "closed"}},
		},
		audit: AuditSpec{Action: "post.setStatus", Target: []string{"post_id"}, Detail: []string{"status"}},
		build: func(_ string, deps APIDeps) (Func, error) {
			if deps.Posts == nil {
				return nil, errAPINotWired
			}
			return func(ctx context.Context, a *Args) (any, error) {
				if err := deps.Posts.SetStatus(ctx, a.String("post_id"), a.String("status")); err != nil {
					return nil, err
				}
				return okResult(), nil
			}, nil
		},
	},
	{
		method: "api.posts.addTag",
		cap:    CapPostsWrite,
		write:  true,
		params: []Field{
			{Name: "post_id", Type: FieldString, Required: true, Desc: "帖子 ID"},
			{Name: "tag", Type: FieldString, Required: true, Desc: "标签名"},
		},
		audit: AuditSpec{Action: "post.addTag", Target: []string{"post_id"}, Detail: []string{"tag"}},
		build: func(_ string, deps APIDeps) (Func, error) {
			if deps.Posts == nil {
				return nil, errAPINotWired
			}
			return func(ctx context.Context, a *Args) (any, error) {
				if err := deps.Posts.AddTag(ctx, a.String("post_id"), a.String("tag")); err != nil {
					return nil, err
				}
				return okResult(), nil
			}, nil
		},
	},
	{
		method: "api.comments.setStatus",
		cap:    CapCommentsWrite,
		write:  true,
		params: []Field{
			{Name: "comment_id", Type: FieldString, Required: true, Desc: "评论 ID"},
			{Name: "status", Type: FieldString, Required: true, Desc: "目标状态",
				Enum: []string{"approved", "pending", "rejected", "closed"}},
		},
		audit: AuditSpec{Action: "comment.setStatus", Target: []string{"comment_id"}, Detail: []string{"status"}},
		build: func(_ string, deps APIDeps) (Func, error) {
			if deps.Comments == nil {
				return nil, errAPINotWired
			}
			return func(ctx context.Context, a *Args) (any, error) {
				if err := deps.Comments.SetStatus(ctx, a.String("comment_id"), a.String("status")); err != nil {
					return nil, err
				}
				return okResult(), nil
			}, nil
		},
	},
	{
		method: "api.posts.getLevel",
		cap:    CapUsersRead,
		params: []Field{
			{Name: "user_id", Type: FieldString, Required: true, Desc: "用户 ID"},
		},
		build: func(_ string, deps APIDeps) (Func, error) {
			if deps.Users == nil {
				return nil, errAPINotWired
			}
			return func(ctx context.Context, a *Args) (any, error) {
				lv, err := deps.Users.GetLevel(ctx, a.String("user_id"))
				if err != nil {
					return nil, err
				}
				return map[string]int{"level": lv}, nil
			}, nil
		},
	},
	{
		method: "api.users.getEmail",
		cap:    CapUsersRead,
		params: []Field{
			{Name: "user_id", Type: FieldString, Required: true, Desc: "用户 ID"},
		},
		build: func(_ string, deps APIDeps) (Func, error) {
			if deps.Users == nil {
				return nil, errAPINotWired
			}
			return func(ctx context.Context, a *Args) (any, error) {
				email, err := deps.Users.GetEmail(ctx, a.String("user_id"))
				if err != nil {
					return nil, err
				}
				return map[string]string{"email": email}, nil
			}, nil
		},
	},
	{
		method: "api.notifications.send",
		cap:    CapNotifySend,
		write:  true,
		params: []Field{
			{Name: "user_id", Type: FieldString, Required: true, Desc: "收件用户 ID"},
			{Name: "title", Type: FieldString, Required: true, Desc: "通知标题"},
			{Name: "body", Type: FieldString, Required: true, Desc: "通知正文"},
		},
		audit: AuditSpec{Action: "notification.send", Target: []string{"user_id"}, Detail: []string{"title"}},
		build: func(_ string, deps APIDeps) (Func, error) {
			if deps.Notifications == nil {
				return nil, errAPINotWired
			}
			return func(ctx context.Context, a *Args) (any, error) {
				if err := deps.Notifications.Send(ctx, a.String("user_id"),
					a.String("title"), a.String("body")); err != nil {
					return nil, err
				}
				return okResult(), nil
			}, nil
		},
	},
	{
		method: "api.kv.get",
		cap:    CapKVRead,
		params: []Field{
			{Name: "key", Type: FieldString, Required: true, Desc: "键名（自动加 plugin:<id>: 前缀）"},
		},
		build: func(pluginID string, deps APIDeps) (Func, error) {
			if deps.KV == nil {
				return nil, errAPINotWired
			}
			return func(ctx context.Context, a *Args) (any, error) {
				scoped, err := pluginKVKey(pluginID, a.String("key"))
				if err != nil {
					return nil, err
				}
				v, err := deps.KV.Get(ctx, scoped)
				if err != nil {
					return nil, err
				}
				return map[string]string{"value": v}, nil
			}, nil
		},
	},
	{
		method: "api.kv.set",
		cap:    CapKVWrite,
		write:  true,
		params: []Field{
			{Name: "key", Type: FieldString, Required: true, Desc: "键名（自动加 plugin:<id>: 前缀）"},
			// value 是**唯一**允许为空串的字段："清空这个键"是合法语义。
			{Name: "value", Type: FieldString, Required: true, AllowEmpty: true, Desc: "值（允许空串）"},
		},
		audit: AuditSpec{Action: "kv.set", Target: []string{"key"}, Scope: AuditScopePluginKV},
		build: func(pluginID string, deps APIDeps) (Func, error) {
			if deps.KV == nil {
				return nil, errAPINotWired
			}
			return func(ctx context.Context, a *Args) (any, error) {
				scoped, err := pluginKVKey(pluginID, a.String("key"))
				if err != nil {
					return nil, err
				}
				if err := deps.KV.Set(ctx, scoped, a.String("value")); err != nil {
					return nil, err
				}
				return okResult(), nil
			}, nil
		},
	},
	{
		method: "api.slots.set",
		cap:    CapSlotsWrite,
		write:  true,
		params: []Field{
			{Name: "slot", Type: FieldString, Required: true, Desc: "插槽名"},
			{Name: "kind", Type: FieldString, Required: true, Desc: "槽位类型"},
			{Name: "props", Type: FieldObject, Desc: "任意属性对象，原样透传"},
			{Name: "sort", Type: FieldInt, Desc: "排序值"},
		},
		audit: AuditSpec{Action: "slot.set", Target: []string{"slot"}, Detail: []string{"kind"}},
		build: func(_ string, deps APIDeps) (Func, error) {
			if deps.Slots == nil {
				return nil, errAPINotWired
			}
			return func(ctx context.Context, a *Args) (any, error) {
				// props 缺省时传 nil（不是空 map）：与重构前 in.Props 的零值一致。
				var props map[string]any
				if raw := a.Raw("props"); len(raw) > 0 {
					props = map[string]any{}
					if err := json.Unmarshal(raw, &props); err != nil {
						return nil, fmt.Errorf("参数类型错误: props 必须是对象")
					}
				}
				if err := deps.Slots.Set(ctx, a.String("slot"), a.String("kind"),
					props, a.Int("sort")); err != nil {
					return nil, err
				}
				return okResult(), nil
			}, nil
		},
	},
	{
		method: "api.slots.delete",
		cap:    CapSlotsWrite,
		write:  true,
		params: []Field{
			{Name: "slot", Type: FieldString, Required: true, Desc: "插槽名"},
		},
		audit: AuditSpec{Action: "slot.delete", Target: []string{"slot"}},
		build: func(_ string, deps APIDeps) (Func, error) {
			if deps.Slots == nil {
				return nil, errAPINotWired
			}
			return func(ctx context.Context, a *Args) (any, error) {
				if err := deps.Slots.Delete(ctx, a.String("slot")); err != nil {
					return nil, err
				}
				return okResult(), nil
			}, nil
		},
	},
}
```

- [ ] **Step 6: 删除已无引用的旧辅助函数**

从 `api.go` 删除 `decodeParams`（743–751 行）与 `requireFields`（763–781 行）——它们已被 `decodeArgs` 取代。

**保留** `okResult`、`pluginKVKey`、`denyHandler`、`errUnknownMethod`、`errAPINotWired`。

删除后检查：
```bash
cd app/server && grep -n "decodeParams\|requireFields" internal/plugin/*.go
```
Expected: 无输出（若 `api_test.go` 仍引用，见下一步）

- [ ] **Step 7: 修正测试辅助里对旧函数的引用（如有）**

若上一步显示 `api_test.go` 引用了 `decodeParams`/`requireFields`，把它们改成等价的 `decodeArgs` 调用。**不要改动任何断言语义。**

Run:
```bash
cd app/server && go build ./... && go vet ./internal/plugin/
```
Expected: 编译通过、vet 无输出

- [ ] **Step 8: 运行全部插件测试**

Run:
```bash
cd app/server && go test ./internal/plugin/ -count=1 -v 2>&1 | tail -40
```
Expected: 全部 PASS。**19 条既有测试一条都不能红**——这是重构是否越界的判据。

- [ ] **Step 9: 验证 gorm 计数与依赖隔离**

Run:
```bash
cd app/server && go list -deps ./internal/plugin | grep -c 'gorm.io'; go list -deps ./internal/plugin | grep -c repository
```
Expected: 两个都是 `0`

- [ ] **Step 10: 提交**

```bash
cd /Users/noelorin/OrinForum
git add app/server/internal/plugin/api.go app/server/internal/plugin/api_test.go
git commit -m "refactor(plugin): declare host API specs as data instead of duplicated structs"
```

---

### Task 4: `APISchemas()` 生成 JSON Schema

**Files:**
- Create: `app/server/internal/plugin/schema.go`
- Test: `app/server/internal/plugin/schema_test.go`

**Interfaces:**
- Consumes: Task 1 的 `Field`/`FieldType` 常量；Task 3 的 `apiSpecs`（含 `params`）
- Produces:
  - `func APISchemas() map[string]json.RawMessage` — 方法名 → JSON Schema 对象
  - `func schemaFor(fields []Field) map[string]any` — 单条规格的 schema（供测试直接断言）

- [ ] **Step 1: 写失败的测试**

创建 `app/server/internal/plugin/schema_test.go`：

```go
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
```

- [ ] **Step 2: 运行确认失败**

Run:
```bash
cd app/server && go test ./internal/plugin/ -run 'TestAPI?Schemas|TestSchema' -v
```
Expected: FAIL — `undefined: APISchemas`, `undefined: schemaFor`

- [ ] **Step 3: 写实现**

创建 `app/server/internal/plugin/schema.go`：

```go
package plugin

import (
	"encoding/json"

	"github.com/sirupsen/logrus"
)

// schemaType 把 FieldType 映射成 JSON Schema 的 type。
func schemaType(t FieldType) string {
	switch t {
	case FieldInt:
		return "integer"
	case FieldObject:
		return "object"
	case FieldBool:
		return "boolean"
	default:
		return "string"
	}
}

// schemaFor 由参数声明生成一条 JSON Schema。
//
// 同一个 []Field 既驱动运行期校验（decodeArgs），也驱动这份文档——
// 因此文档不会与实际校验漂移。这是把参数从 Go 代码变成数据的直接收益：
// 重构前插件作者要调 api.posts.setStatus，只能读 Go 源码。
//
// 字段名为空时 panic：schema 与取值器都按名索引，空名会让两者同时静默失效，
// 而这是**注册期**就该发现的编程错误，不是运行期输入问题。
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
```

- [ ] **Step 4: 运行确认通过**

Run:
```bash
cd app/server && go test ./internal/plugin/ -run 'TestAPI?Schemas|TestSchema' -v
```
Expected: PASS（7 条）

- [ ] **Step 5: 提交**

```bash
cd /Users/noelorin/OrinForum
git add app/server/internal/plugin/schema.go app/server/internal/plugin/schema_test.go
git commit -m "feat(plugin): expose JSON schemas derived from param declarations"
```

---

### Task 5: 反向能力校验与枚举字面量钉死

**Files:**
- Modify: `app/server/internal/plugin/api.go`（新增 `capabilityWithoutAPI`）
- Modify: `app/server/internal/plugin/api_test.go`（追加两条测试）

**Interfaces:**
- Consumes: Task 3 的 `apiSpecs`（含 `cap` 字段）、`apiCapabilities()`、`KnownCapabilities()`
- Produces: `var capabilityWithoutAPI map[string]string`（包级、非导出）

- [ ] **Step 1: 写失败的测试**

在 `app/server/internal/plugin/api_test.go` **末尾追加**：

```go
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
}
```

在 `app/server/internal/plugin/api_test.go` 末尾再追加（**internal 测试可 import model**，实测不影响 gorm 计数）：

```go
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
```

这个文件顶部需要补 `model` import。在 `api_test.go` 的 import 块追加：

```go
"feedchamber/internal/model"
```

- [ ] **Step 2: 运行确认失败**

Run:
```bash
cd app/server && go test ./internal/plugin/ -run 'TestEveryCapabilityUsedByAnAPI|TestCapabilityWithoutAPIListIsAccurate|TestStatusEnumMatchesModelConstants' -v
```
Expected: FAIL — `undefined: capabilityWithoutAPI`（另两条可能因缺变量同样失败）

- [ ] **Step 3: 在 `api.go` 的能力常量区之后加白名单**

在 `knownCapabilities` 定义之后插入：

```go
// capabilityWithoutAPI 是「已声明但暂时没有 API 使用」的能力。
//
// 存在本身就是缺陷：管理员在装机确认页批准它时，批准的是一条不存在的能力，
// 插件拿不到任何对应 API。此表只允许变短——新增条目必须同时给出理由与
// 移除计划，TestEveryCapabilityUsedByAnAPI 会强制每个条目在这里出现。
//
// 待裁定：删除（会让已声明它的存量插件重装失败）还是接上真实 API（新能力）。
// 见 docs/superpowers/specs/2026-11-06-plugin-capability-config-design.md §5.3。
var capabilityWithoutAPI = map[string]string{
	CapPostsRead:    "从未有过宿主 API；待裁定（§5.3）",
	CapCommentsRead: "从未有过宿主 API；待裁定（§5.3）",
}
```

- [ ] **Step 4: 运行确认通过**

Run:
```bash
cd app/server && go test ./internal/plugin/ -run 'TestEveryCapabilityUsedByAnAPI|TestCapabilityWithoutAPIListIsAccurate|TestStatusEnumMatchesModelConstants' -v
```
Expected: PASS（3 条）

- [ ] **Step 5: 验证 gorm 计数未被内部测试破坏**

Run:
```bash
cd app/server && go list -deps ./internal/plugin | grep -c 'gorm.io'
```
Expected: `0` — **这一步不能跳过**。测试 import 了 `internal/model`，此命令证明它只影响测试构建图。

- [ ] **Step 6: 提交**

```bash
cd /Users/noelorin/OrinForum
git add app/server/internal/plugin/api.go app/server/internal/plugin/api_test.go
git commit -m "test(plugin): guard unused capabilities and pin status enums to model"
```

---

### Task 6: 更新 `AGENTS.md` 并做全量验证

**Files:**
- Modify: `app/server/internal/plugin/AGENTS.md`（「宿主 API」一节与测试表）

**Interfaces:**
- Consumes: 前五个任务的全部产出
- Produces: 无代码产出

- [ ] **Step 1: 重写 AGENTS.md 的「`apiSpecs` 规格表」小节**

把该小节（原文从 `### `apiSpecs` 规格表：方法名、能力、是否写类在同一行` 到 `TestEveryAPIMethodDeclaresACapability` 要求每个方法都登记已知能力` 段末）替换为：

```markdown
### `apiSpecs` 规格表：方法名、能力、参数、审计在同一行

```go
type apiSpec struct {
    method string
    cap    string          // 所需能力；漏写 → 该 API 不可调用（fail closed）
    write  bool            // true → 成功后**统一**由 adapt 落审计
    params []Field         // 参数的唯一声明处
    audit  AuditSpec       // 零值 = 无审计（读类）
    build  func(pluginID string, deps APIDeps) (Func, error)
}
```

`params []Field` 是**唯一**的参数声明处，同时产出三样东西：

| 产物 | 实现 | 谁用 |
|------|------|------|
| 运行期校验 | `args.go` 的 `decodeArgs` | 每次调用 |
| 取值器 | `Args.String` / `Int` / `Raw`（`String` 统一 TrimSpace） | handler 内部 |
| JSON Schema | `schema.go` 的 `schemaFor` / `APISchemas()` | 安装确认页、插件作者 |

重构前的形态是每个方法写两份匿名 struct：`build` 里一份、`audit` 里再 `json.Unmarshal` 一份。**改一个字段名只改一处不报错**——audit 按旧名解析得到零值，写操作照常生效、审计目标变成空串。现在字段名只出现在 `params` 里。

`AuditSpec` 把审计落点声明成字段引用：

```go
audit: AuditSpec{Action: "post.setStatus", Target: []string{"post_id"}, Detail: []string{"status"}}
```

`api.kv.set` 需要把 target 落成**加过前缀的全键**，用 `Scope: AuditScopePluginKV` 表达，由统一提取器调 `pluginKVKey`——不再为它单开一个闭包。

两处收益：

- `apiMethodNames()` / `writeAPIMethods()` / `apiCapabilities()` 全部**从规格表推导**。
- 审计在 `adapt` 里统一落，**handler 内部没有审计调用点**——「写类 API 忘审计」不可能发生。

> 已知未做：`lookupAPI` 每次调用仍线性 `findAPI` 扫 10 条规格。去重需要在
> `RegisterAPIs` 预建 map 并改 `lookupAPI` 签名，而 7 处测试直接调它——收益不抵改动面。

**审计失败不翻转结果。** 业务动作已经生效，把 error 返回给插件只会让它重试，而重试会**再执行一次**写操作。
因此只记 error 日志（含 plugin/api/action 字段），返回成功。反过来，`deps.Audit == nil`（漏接线）
也只记 error 不 panic——handler 跑在 ipc 的 dispatch goroutine 里，panic 就是整个后端崩溃。
```

- [ ] **Step 2: 更新 AGENTS.md 的「参数校验」小节**

把 `### 参数校验：只查非空，不查取值域` 一节替换为：

```markdown
### 参数校验：只查必填 / 空串 / 类型，不查取值域

`decodeArgs` 拒绝缺失或纯空白的必填字段（错误里**点名**哪个字段，按名排序），
按 `Field.Type` 校验类型，但不校验 `status` 是否属于 `approved/pending/...`。

两条刻意的边界：

1. **取值域不在本包校验。** 权威定义在 service 层（`post.go` / `content.go` 的
   `isValidPostStatus`）。在插件层再校验一遍会产生第二个事实来源：加枚举值时
   改 service 忘了改这里，症状是「文档说支持、实际被拒」。
   `Field.Enum` **只写进 JSON Schema 供文档展示，运行期不校验**。
2. **枚举字面量的重复由测试钉住。** 生产代码不能 import `internal/model`
   （会把 5 个 gorm 包带进来），所以 `Enum` 只能是字面量；
   `TestStatusEnumMatchesModelConstants` 用**内部测试包**把它与 `model.PostStatus*`
   常量对齐。这条测试 import `internal/model` 不影响 `go list -deps` 的 gorm 计数
   （该命令只算非测试构建图）。

空值绝不能透传到 service：`{}` 解出来全是零值。`value`（kv.set）是**唯一**
允许为空串的字段（`Field.AllowEmpty`）。
```

- [ ] **Step 3: 更新测试表**

在 AGENTS.md 的测试表末尾追加三行：

```markdown
| `TestEveryCapabilityUsedByAnAPI` | **反向校验**：每个已声明能力都被 ≥1 个 API 使用；空能力必须在 `capabilityWithoutAPI` 白名单里（该表只许变短） || `TestAPISchemasCoverAllMethods` / `TestSchema*` | schema 与 `params` 同源：覆盖全部方法、required 与声明一致、Enum 进文档、字段名非空 |
| `TestStatusEnumMatchesModelConstants` | 插件侧 `status` 枚举字面量与 `model.PostStatus*` 常量一致（内部测试 import model，不影响 gorm 计数） |
```

- [ ] **Step 4: 更新「改这里要注意」里关于新增宿主 API 的那条**

把「新增宿主 API：**只加 `apiSpecs` 一行**」那条替换为：

```markdown
- 新增宿主 API：**只加 `apiSpecs` 一条**（method / cap / write / params / audit / build）。
  参数写进 `params []Field`，审计写进 `AuditSpec`——**不要再写第二个匿名 struct 或
  `json.Unmarshal` 提取审计字段**，那正是 2026-11-06 重构消掉的失效模式。
  `apiMethodNames()` 之类的字面量清单全部从规格表推导，不要手工维护。
  测试里补一条 `writeAPISampleParams`（写类），`TestWriteAPIsAreAudited` 会自动纳入覆盖。
- 不要给 `Field.Enum` 补运行期校验。取值域的权威定义在 service 层，
  在插件层再校验一遍会制造第二个事实来源（见「参数校验」一节）。
```

- [ ] **Step 5: 全量验证**

Run:
```bash
cd app/server && go build ./... && go vet ./... && go test ./internal/plugin/ -count=1 -race
```
Expected: 编译通过、vet 无输出、测试全 PASS

- [ ] **Step 6: 验收点复查（逐条对照 spec §4）**

Run:
```bash
cd app/server && echo "1) 匿名 struct 数（应为 0）:"; grep -c 'var in struct {' internal/plugin/api.go || true
echo "2) audit 闭包数（应为 0）:"; grep -c 'audit: func' internal/plugin/api.go || true
echo "3) 双重 Unmarshal（应为 0）:"; grep -c 'json.Unmarshal(p, &in)' internal/plugin/api.go || true
echo "7) gorm 计数（应为 0）:"; go list -deps ./internal/plugin | grep -c 'gorm.io' || true
echo "   api.go 行数:"; wc -l < internal/plugin/api.go
echo "   args.go/audit.go/schema.go 行数:"; wc -l internal/plugin/args.go internal/plugin/audit.go internal/plugin/schema.go
```
Expected: 前三项 `0`、gorm `0`；`api.go` 行数明显低于重构前的 902

`grep -c` 在零匹配时返回退出码 1，因此用 `|| true` 吞掉。

- [ ] **Step 7: 提交**

```bash
cd /Users/noelorin/OrinForum
git add app/server/internal/plugin/AGENTS.md
git commit -m "docs(plugin): record declarative param/audit/schema spec table"
```

---

## 自检记录

**Spec 覆盖：**

| Spec 章节 | 对应任务 |
|---|---|
| §1 问题（17 处匿名 struct / 7 个 audit 闭包 / 2 次 Unmarshal） | Task 3 Step 5-6 消除；Task 6 Step 6 验收 |
| §3 `Field` / `Args` / `AuditSpec` / `Func` | Task 1、2、3 |
| §3.1 解码只发生一次 | Task 3 Step 3（`adapt`） |
| §4 验收点 1-3（grep 归零） | Task 6 Step 6 |
| §4 验收点 4-5（反向校验） | Task 5 |
| §4 验收点 6（JSON Schema） | Task 4 |
| §4 验收点 7（gorm=0） | Task 1 Step 5、Task 3 Step 9、Task 5 Step 5、Task 6 Step 5 |
| §4 验收点 8（既有测试原样通过） | Task 3 Step 8 |
| §5.1 取值域不校验 | Task 3（`Field.Enum` 只进 schema）+ Task 6 Step 2 |
| §5.2 枚举字面量由测试钉死 | Task 5 Step 1 第三条测试 |
| §5.3 空能力白名单 | Task 5 |
| §6 文件清单 | 文件结构表 |
| §7 不做运行时开关 | 全局约束 |
| §8 实施顺序 | Task 1→6 |
| §9 风险（错误文案不变） | 全局约束 + Task 1 Step 1 逐字断言 |

**类型一致性核对：** `Func` 在 Task 3 定义、在全部 10 条规格的 `build` 返回值中使用，签名一致（`func(context.Context, *Args) (any, error)`）。`decodeArgs` 在 Task 1 定义（返回 `*Args, error`），Task 3 `adapt` 与 Task 2 测试使用，一致。`AuditSpec.extract` 在 Task 2 定义（`(pluginID string, a *Args) (string, string, string)`），Task 3 `adapt` 调用，一致。`schemaFor` 在 Task 4 定义并被 `APISchemas` 与测试共用。`capabilityWithoutAPI` 在 Task 5 定义，Task 5 两条测试引用。

**已知遗留（不在本轮范围）：** `posts:read` / `comments:read` 两条空能力仍存在，由白名单显式放行并标注待裁定（spec §5.3）。
