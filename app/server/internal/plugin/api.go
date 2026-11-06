package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/sirupsen/logrus"
)

// 能力常量。插件在 plugin.json 的 capabilities 里声明，运行时逐个校验。
//
// 每个常量都必须至少被一个 API 方法要求（TestEveryAPIMethodDeclaresACapability
// 与 apiSpecs 共同钉住）——一个没有任何 API 使用的能力，等于在安装确认页向
// 管理员展示一条它其实用不上的权限。
const (
	CapPostsRead     = "posts:read"
	CapPostsWrite    = "posts:write"
	CapCommentsRead  = "comments:read"
	CapCommentsWrite = "comments:write"
	CapUsersRead     = "users:read"
	CapNotifySend    = "notifications:send"
	CapKVRead        = "kv:read"
	CapKVWrite       = "kv:write"
	CapSlotsWrite    = "slots:write"
)

// knownCapabilities 是核心能力白名单。
//
// 新增能力必须同时出现在这里与某个 apiSpec 的 cap 字段上：只加在这里而不被
// 任何 API 使用，管理员会在安装确认页看到一条无效权限。
var knownCapabilities = map[string]struct{}{
	CapPostsRead:     {},
	CapPostsWrite:    {},
	CapCommentsRead:  {},
	CapCommentsWrite: {},
	CapUsersRead:     {},
	CapNotifySend:    {},
	CapKVRead:        {},
	CapKVWrite:       {},
	CapSlotsWrite:    {},
}

// capabilityWithoutAPI 是「已声明但暂时没有 API 使用」的能力。
//
// 存在本身就是缺陷：管理员在装机确认页批准它时，批准的是一条不存在的能力，
// 插件拿不到任何对应 API。此表只允许变短——新增条目必须同时修改
// TestCapabilityWithoutAPIListIsAccurate 里的 key-set 断言，从而进入 review 视野。
//
// 两个方向的约束来自**两条不同的测试**，别指望一条兜住全部：
//   - TestEveryCapabilityUsedByAnAPI：每个「已知但无人使用」的能力必须在这里，
//     否则变红。它管的是**漏登记**（某个空能力忘了写进白名单）。
//   - TestCapabilityWithoutAPIListIsAccurate 的 key-set 断言：这里不许**多**出条目，
//     否则变红。它管的是**白名单扩张**——这一半只能靠钉住 key 集合的上界，
//     因为「新增一个能力 + 把它写进白名单」在语义上与存量两条无法区分。
//
// 钉的是 key 集合而非整张 map：**缩短**（按 §5.3 删除某条能力，或给它接上真实
// API）只改本表即可，不必再动测试；**增长**则必须显式改断言。
//
// 待裁定：删除（会让已声明它的存量插件重装失败）还是接上真实 API（新能力）。
// 见 docs/superpowers/specs/2026-11-06-plugin-capability-config-design.md §5.3。
var capabilityWithoutAPI = map[string]string{
	CapPostsRead:    "从未有过宿主 API；待裁定（§5.3）",
	CapCommentsRead: "从未有过宿主 API；待裁定（§5.3）",
}

// KnownCapabilities 返回全部合法 capability（**已排序**，供 manifest 校验与后台展示）。
//
// 排序不是为了好看：这份清单会被写进安装确认页、审计与测试断言，
// map 迭代序会让同样的输入产生不同的输出，测试与日志都无法逐字节对照。
func KnownCapabilities() []string {
	out := make([]string, 0, len(knownCapabilities))
	for c := range knownCapabilities {
		out = append(out, c)
	}
	// sort.Strings 对本包的依赖是零成本（标准库），却换来确定性输出。
	sort.Strings(out)
	return out
}

// IsKnownCapability 报告 name 是否是核心白名单里的能力。
//
// Manifest.Validate 用它之外的白名单也行，但两个调用点必须看同一份表：
// 「安装时校验」与「运行时校验」各持一份是能力系统最常见的漂移。
func IsKnownCapability(name string) bool {
	_, ok := knownCapabilities[name]
	return ok
}

// CapabilitySet 是某个插件已授予的能力集合。
//
// 底层类型刻意就是 map[string]struct{}（命名类型，非独立结构）：
// Manifest.Validate(knownCaps map[string]struct{}) 可以直接接收它，
// 计划里 m.Validate(CapabilitySetFromNames(KnownCapabilities())) 因此无需转换。
//
// 三个必须避免的错法（api_test.go 各钉一条）：
//   - nil/空集合必须拒绝**每一个**写类 API（fail closed，见 grants）；
//   - manifest 里的未知能力由 Manifest.Validate 在安装期拒绝，运行期不猜；
//   - RegisterAPIs 只为已授予的能力注册真实 handler，未授予时注册的是
//     拒绝桩——它连 deps 都不持有，删掉桩里的检查也执行不到任何业务代码。
type CapabilitySet map[string]struct{}

// CapabilitySetFromNames 把名字列表转成能力集合。
//
// 未知名字被**丢弃**而不是保留：本函数只会收窄而不会放宽权限（没有任何 API
// 要求一个不存在的能力），丢弃因此是 fail-closed 的。真正的「未知能力必须
// 报错」发生在 Manifest.Validate，安装期就该拦住——不要指望运行期兜底，
// 也不要因为这里静默丢弃就以为安装校验可以省。
func CapabilitySetFromNames(names []string) CapabilitySet {
	out := make(CapabilitySet, len(names))
	for _, n := range names {
		if IsKnownCapability(n) {
			out[n] = struct{}{}
		}
	}
	return out
}

// grants 报告集合是否授予了 cap。
//
// 这是唯一的判定入口，且**空能力名永远不通过**：某个 apiSpec 若漏写 cap
// （值为 ""），它的每一次调用都会因为这一行被拒绝，而不是变成人人可调。
// nil 集合天然通过不了任何判定——这就是 fail closed，无需额外分支。
func (s CapabilitySet) grants(cap string) bool {
	if cap == "" {
		return false
	}
	_, ok := s[cap]
	return ok
}

// ---------------------------------------------------------------------------
// 接缝接口
// ---------------------------------------------------------------------------

// PostWriter 是插件可用的帖子写操作。
//
// 只暴露「业务动作」而不是通用 CRUD：插件需要改状态、加标签，
// 但不需要任意 UPDATE。收窄到枚举动作，审计才有意义。
type PostWriter interface {
	SetStatus(ctx context.Context, postID, status string) error
	AddTag(ctx context.Context, postID, tag string) error
}

// CommentWriter 是插件可用的评论写操作。
type CommentWriter interface {
	SetStatus(ctx context.Context, commentID, status string) error
}

// UserReader 是插件可用的用户读取。
type UserReader interface {
	GetLevel(ctx context.Context, userID string) (int, error)
	GetEmail(ctx context.Context, userID string) (string, error)
}

// Notifier 让插件发站内通知。
type Notifier interface {
	Send(ctx context.Context, userID, title, body string) error
}

// KVStore 是插件专属键值空间。
//
// 实现方拿到的 key **已经**被 api.go 限定在 plugin:<pluginID>: 之下
// （见 pluginKVKey）。不要在实现方再拼一次前缀——那会得到
// plugin:x:plugin:x:k，而且会把作用域逻辑复制成两份。
type KVStore interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string) error
}

// SlotWriter 让插件写入壳层插槽条目。
type SlotWriter interface {
	Set(ctx context.Context, slot, kind string, props map[string]any, sortOrder int) error
	Delete(ctx context.Context, slot string) error
}

// AuditRecorder 记录插件发起的写操作。
type AuditRecorder interface {
	RecordPluginAction(ctx context.Context, pluginID, action, target, detail string) error
}

// APIDeps 是宿主暴露给插件的全部能力实现。
//
// 每个字段都是**业务接缝**，由 Task 7/8 用真实 service 适配；本包因此
// 不 import repository、不持有 *gorm.DB（插件写入必须经 service 层，
// 审计/事件等副作用才与管理员操作一致）。
type APIDeps struct {
	Posts         PostWriter
	Comments      CommentWriter
	Users         UserReader
	Notifications Notifier
	KV            KVStore
	Slots         SlotWriter
	Audit         AuditRecorder
}

// ---------------------------------------------------------------------------
// 错误
// ---------------------------------------------------------------------------

// ErrCapabilityDenied 是「插件没有声明该能力」的可识别错误。
//
// 用 errors.Is 判定，不要匹配文案：spec 要求把权限错误返回给插件，
// 插件侧据此区分「我没申请这个权限」与「宿主出错了」。
var ErrCapabilityDenied = errors.New("capability denied")

// errAPINotWired 表示能力已授予，但宿主没有为该 API 接线（deps 字段为 nil）。
//
// 与 ErrCapabilityDenied 分开：前者该找管理员要权限，后者是宿主的装配问题。
// 两者都必须**明确报错而不是 panic**——handler 跑在 IPC 的 dispatch
// goroutine 里，一次 panic 就是整个后端进程崩溃。
var errAPINotWired = errors.New("host API not wired")

// errUnknownMethod 是未知方法的标准错误（与 ipc.go 的文案保持同形）。
func errUnknownMethod(method string) error {
	return fmt.Errorf("unknown method: %s", method)
}

// ---------------------------------------------------------------------------
// API 规格表
// ---------------------------------------------------------------------------

// Func 是解码并校验之后的处理函数。
//
// **不再是 ipc.Handler**：参数已经由 decodeArgs 解好并校验，handler 不再
// 直接解析整个 params 对象，也就不会出现「忘了校验必填字段」或
// 「在第二处重复声明字段名」（审计侧）的问题。
//
// 唯一例外是 FieldObject：它按设计透传原始 JSON，取到后仍需自己 Unmarshal
// （当前只有 api.slots.set 的 props）。
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
	// audit 零值表示无审计（读类）。写类必须非零——由
	// TestWriteSpecsDeclareAudit 与 TestAuditFieldReferencesExistInParams 钉住。
	// （TestWriteAPIsAreAudited 只数字节数，漏写 audit 时 adapt 会退化成方法名
	// 而不报错，因此那一条并不能充当本字段的约束。）
	audit AuditSpec
	build func(pluginID string, deps APIDeps) (Func, error)
}

// apiSpecs 是全部宿主 API。**顺序即注册顺序**（不使用 map，输出确定）。
//
// 方法名沿用计划约定的 api.<域>.<动作>，与插件侧 hook.<钩子名> 同一命名空间
// 家族但不同前缀——插件按前缀分派就不会把两者搞混。
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
			{Name: "body", Type: FieldString, Required: true, NoTrim: true, Desc: "通知正文（原样入库，不 trim）"},
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
			{Name: "key", Type: FieldString, Required: true, NoTrim: true, Desc: "键名（原样传递，自动加 plugin:<id>: 前缀）"},
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
			{Name: "key", Type: FieldString, Required: true, NoTrim: true, Desc: "键名（原样传递，自动加 plugin:<id>: 前缀）"},
			// value 是**唯一**允许为空串的字段："清空这个键"是合法语义。
			{Name: "value", Type: FieldString, Required: true, AllowEmpty: true, NoTrim: true, Desc: "值（原样存储，允许空串）"},
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

// apiCapabilities 是「方法 → 所需能力」的视图，供测试与后台展示使用。
func apiCapabilities() map[string]string {
	out := make(map[string]string, len(apiSpecs))
	for _, s := range apiSpecs {
		out[s.method] = s.cap
	}
	return out
}

// apiMethodNames 返回全部已实现的 API 方法名，顺序与 apiSpecs 一致。
//
// 它直接从规格表推导，不再单独维护一份字面量清单——brief 里的字面量清单
// 与 lookupAPI 的 case 列表是两份手写数据，少写一条的症状是「该方法永远
// unknown method」，而且不会编译报错。
func apiMethodNames() []string {
	out := make([]string, 0, len(apiSpecs))
	for _, s := range apiSpecs {
		out = append(out, s.method)
	}
	return out
}

// writeAPIMethods 返回全部**写类** API 方法名（顺序同 apiSpecs）。
//
// 写类 = 必须落审计的那些。测试用它逐一验证「未声明能力 → 拒绝」
// 与「成功调用 → 恰好一条审计」，新增写 API 时自动纳入覆盖。
func writeAPIMethods() []string {
	var out []string
	for _, s := range apiSpecs {
		if s.write {
			out = append(out, s.method)
		}
	}
	return out
}

// findAPI 按方法名查规格。
func findAPI(method string) (apiSpec, bool) {
	for _, s := range apiSpecs {
		if s.method == method {
			return s, true
		}
	}
	return apiSpec{}, false
}

// ---------------------------------------------------------------------------
// 查找 + capability 校验
// ---------------------------------------------------------------------------

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

// denyHandler 是「能力未授予」的拒绝桩。
//
// 它刻意**不收 deps**：没有 service、没有 KV、没有审计器可碰。留在这里的
// 唯一目的是给插件一个可识别的权限错误（spec 要求返回权限错误，而不是
// 让插件把一个权限问题误诊成方法名拼错或宿主超时）。
func denyHandler(pluginID, method, cap string) Handler {
	return func(context.Context, json.RawMessage) (any, error) {
		logrus.WithFields(logrus.Fields{
			"plugin":     pluginID,
			"api":        method,
			"capability": cap,
		}).Debug("plugin: 拒绝未声明能力的宿主 API 调用")
		return nil, fmt.Errorf("%s: 插件 %s 未声明 capability %q: %w",
			method, pluginID, cap, ErrCapabilityDenied)
	}
}

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

// ---------------------------------------------------------------------------
// 返回值
// ---------------------------------------------------------------------------

// okResult 是写类 API 成功时的固定返回。
//
// 参数校验/取值已由 args.go 的 decodeArgs 在 adapt 里统一完成，
// 本文件不再有 decodeParams / requireFields。
func okResult() any { return map[string]bool{"ok": true} }

// ---------------------------------------------------------------------------
// KV 命名空间
// ---------------------------------------------------------------------------

// kvNamespaceRoot 是所有插件 KV 键的公共前缀。
const kvNamespaceRoot = "plugin:"

// maxKVKeyBytes 是插件自填 key 部分的长度上限。
//
// 不是安全边界，是防呆：几百字节以上的键几乎肯定是把 blob 当 key 用了，
// 而 NATS KV 的键有长度约束，早一点报错比在存储层拿到一个晦涩错误好。
const maxKVKeyBytes = 200

// pluginKVKey 把一个插件自填的 key 变成 `plugin:<pluginID>:<key>`。
//
// # 为什么作用域必须在这里拼，而不是交给插件或存储实现
//
// 把前缀交给插件自觉，等于没有隔离；交给 KVStore 实现，则每个实现都要
// 重做一遍同一件事（Task 7 用真实 kvstore 适配时会再写第二次），两份
// 逻辑迟早分叉。api.go 是插件 key 进入系统的唯一入口，作用域拼在这里，
// 实现方拿到的就已经是限定好的全键。
//
// # 拒绝什么
//
// 三类逃逸各有对应断言（TestKVRejectsKeysEscapingPluginNamespace）：
//
//   - 路径穿越：`../x`、`a/../../b`、`sub\..\..\x`。KV 键不是文件路径，
//     但「键空间用 / 分层」的实现（含将来可能换的后端）会把它们解释成
//     越界，拒绝比赌实现行为可靠。
//   - 绝对化：`/abs/path`、Windows 盘符。同上。
//   - 冒充命名空间：`plugin:other:k` —— 直接点名别人的前缀，必须拒绝，
//     否则 beta 可以读 alpha 的键（前缀拼接后变成 plugin:beta:plugin:other:k
//     虽然不会真的读到，但这类输入没有任何合法用途，报错更好）。
//
// 另外拒绝空段（`a//b`）、`.` 段、含 `:` 的 key、控制字符与超长 key。
func pluginKVKey(pluginID, key string) (string, error) {
	prefix, err := pluginKVNamespace(pluginID)
	if err != nil {
		return "", err
	}

	trimmed := strings.TrimSpace(key)
	if trimmed == "" {
		return "", fmt.Errorf("kv: key 不能为空")
	}
	if trimmed != key {
		// 静默 trim 会让 "k" 与 " k" 落到同一个键上，插件很难自己发现。
		return "", fmt.Errorf("kv: key 不能以空白开头或结尾")
	}
	if len(key) > maxKVKeyBytes {
		return "", fmt.Errorf("kv: key 过长（%d 字节，上限 %d）", len(key), maxKVKeyBytes)
	}
	if strings.ContainsRune(key, ':') {
		return "", fmt.Errorf("kv: key 不能包含 %q（会与命名空间分隔符混淆）", ":")
	}
	if strings.ContainsRune(key, '\\') {
		return "", fmt.Errorf("kv: key 不能包含反斜杠: %q", key)
	}
	if strings.HasPrefix(key, "/") {
		return "", fmt.Errorf("kv: key 不能是绝对路径形态: %q", key)
	}
	// Windows 盘符形态（C:\... / C:/...）：单字母 + 冒号已在上面被 : 拦住，
	// 这里只防纯文本意义上的 "C" 与盘符混淆——不做多余判断。
	for _, r := range key {
		if r < 0x20 || r == 0x7f {
			return "", fmt.Errorf("kv: key 不能包含控制字符: %q", key)
		}
		if unicode.IsSpace(r) {
			return "", fmt.Errorf("kv: key 不能包含空白字符: %q", key)
		}
	}
	for _, seg := range strings.Split(key, "/") {
		switch seg {
		case "":
			return "", fmt.Errorf("kv: key 不能包含空路径段（%q）", key)
		case ".", "..":
			return "", fmt.Errorf("kv: key 不能包含 %q 路径段（%q）", seg, key)
		}
	}

	full := prefix + key
	// 最后一道：拼接结果必须真的在命名空间之下。上面的逐条校验已经足够，
	// 但作用域是本任务唯一的安全边界，多一次断言不贵。
	if !strings.HasPrefix(full, prefix) {
		return "", fmt.Errorf("kv: key 逃出插件命名空间: %q", key)
	}
	return full, nil
}

// pluginKVNamespace 是某个插件的 KV 前缀 `plugin:<pluginID>:`。
//
// pluginID 同样要校验：空 pluginID 会让所有插件共享 `plugin::`，而含冒号的
// pluginID（"a:b"）与另一个插件的键（"a" + "b:c"）会拼出同一个全键。
// 两种情况都等于命名空间隔离失效，因此直接拒绝整个 KV 操作。
func pluginKVNamespace(pluginID string) (string, error) {
	if strings.TrimSpace(pluginID) == "" {
		return "", fmt.Errorf("kv: 插件标识为空，无法确定命名空间")
	}
	if len(pluginID) > 64 {
		return "", fmt.Errorf("kv: 插件标识过长: %q", pluginID)
	}
	if strings.HasPrefix(pluginID, kvNamespaceRoot) {
		return "", fmt.Errorf("kv: 插件标识不能以 %q 开头: %q", kvNamespaceRoot, pluginID)
	}
	if strings.ContainsAny(pluginID, ":/\\") {
		return "", fmt.Errorf("kv: 插件标识不能包含 %q 或 %q: %q", ":", "/", pluginID)
	}
	for _, r := range pluginID {
		if r < 0x20 || r == 0x7f || unicode.IsSpace(r) {
			return "", fmt.Errorf("kv: 插件标识含非法字符: %q", pluginID)
		}
	}
	if pluginID == "." || pluginID == ".." {
		return "", fmt.Errorf("kv: 插件标识非法: %q", pluginID)
	}
	return kvNamespaceRoot + pluginID + ":", nil
}
