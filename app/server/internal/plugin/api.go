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

// apiSpec 是一个宿主 API 的定义：方法名、所需能力、是否为写类、构造器。
//
// 把这三件事放在**同一行**上，是这套设计的核心：
//
//	方法 → 能力 的映射分散在 switch 的 case 里时，「新增 API 忘了写校验」
//	是一个纯手工纪律问题；放进结构体字段后，缺字段会被
//	TestEveryAPIMethodDeclaresACapability 直接抓住，而 lookupAPI 只有
//	一条路径能构造出 handler——它必然先查能力。
type apiSpec struct {
	method string
	cap    string
	// write 为 true 时，调用成功后**统一**由 wrap 落审计；handler 自己
	// 不写审计，也就不可能漏写（handler 里根本没有审计调用点）。
	write bool
	build func(pluginID string, deps APIDeps) (Handler, error)
	// audit 在 write=true 时必填：从参数里提取 (action, target, detail)。
	// 拿不到就返回空串——目标对象缺失的写请求在 handler 里已经被拒了，
	// 走不到这里。
	audit func(pluginID string, p json.RawMessage) (action, target, detail string)
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
		build: func(_ string, deps APIDeps) (Handler, error) {
			if deps.Posts == nil {
				return nil, errAPINotWired
			}
			return func(ctx context.Context, p json.RawMessage) (any, error) {
				var in struct {
					PostID string `json:"post_id"`
					Status string `json:"status"`
				}
				if err := decodeParams(p, &in); err != nil {
					return nil, err
				}
				if err := requireFields(map[string]string{"post_id": in.PostID, "status": in.Status}); err != nil {
					return nil, err
				}
				if err := deps.Posts.SetStatus(ctx, strings.TrimSpace(in.PostID), strings.TrimSpace(in.Status)); err != nil {
					return nil, err
				}
				return okResult(), nil
			}, nil
		},
		audit: func(_ string, p json.RawMessage) (string, string, string) {
			var in struct {
				PostID string `json:"post_id"`
				Status string `json:"status"`
			}
			_ = json.Unmarshal(p, &in)
			return "post.setStatus", strings.TrimSpace(in.PostID), strings.TrimSpace(in.Status)
		},
	},
	{
		method: "api.posts.addTag",
		cap:    CapPostsWrite,
		write:  true,
		build: func(_ string, deps APIDeps) (Handler, error) {
			if deps.Posts == nil {
				return nil, errAPINotWired
			}
			return func(ctx context.Context, p json.RawMessage) (any, error) {
				var in struct {
					PostID string `json:"post_id"`
					Tag    string `json:"tag"`
				}
				if err := decodeParams(p, &in); err != nil {
					return nil, err
				}
				if err := requireFields(map[string]string{"post_id": in.PostID, "tag": in.Tag}); err != nil {
					return nil, err
				}
				if err := deps.Posts.AddTag(ctx, strings.TrimSpace(in.PostID), strings.TrimSpace(in.Tag)); err != nil {
					return nil, err
				}
				return okResult(), nil
			}, nil
		},
		audit: func(_ string, p json.RawMessage) (string, string, string) {
			var in struct {
				PostID string `json:"post_id"`
				Tag    string `json:"tag"`
			}
			_ = json.Unmarshal(p, &in)
			return "post.addTag", strings.TrimSpace(in.PostID), strings.TrimSpace(in.Tag)
		},
	},
	{
		method: "api.comments.setStatus",
		cap:    CapCommentsWrite,
		write:  true,
		build: func(_ string, deps APIDeps) (Handler, error) {
			if deps.Comments == nil {
				return nil, errAPINotWired
			}
			return func(ctx context.Context, p json.RawMessage) (any, error) {
				var in struct {
					CommentID string `json:"comment_id"`
					Status    string `json:"status"`
				}
				if err := decodeParams(p, &in); err != nil {
					return nil, err
				}
				if err := requireFields(map[string]string{"comment_id": in.CommentID, "status": in.Status}); err != nil {
					return nil, err
				}
				if err := deps.Comments.SetStatus(ctx, strings.TrimSpace(in.CommentID), strings.TrimSpace(in.Status)); err != nil {
					return nil, err
				}
				return okResult(), nil
			}, nil
		},
		audit: func(_ string, p json.RawMessage) (string, string, string) {
			var in struct {
				CommentID string `json:"comment_id"`
				Status    string `json:"status"`
			}
			_ = json.Unmarshal(p, &in)
			return "comment.setStatus", strings.TrimSpace(in.CommentID), strings.TrimSpace(in.Status)
		},
	},
	{
		method: "api.posts.getLevel",
		cap:    CapUsersRead,
		build: func(_ string, deps APIDeps) (Handler, error) {
			if deps.Users == nil {
				return nil, errAPINotWired
			}
			return func(ctx context.Context, p json.RawMessage) (any, error) {
				var in struct {
					UserID string `json:"user_id"`
				}
				if err := decodeParams(p, &in); err != nil {
					return nil, err
				}
				if err := requireFields(map[string]string{"user_id": in.UserID}); err != nil {
					return nil, err
				}
				lv, err := deps.Users.GetLevel(ctx, strings.TrimSpace(in.UserID))
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
		build: func(_ string, deps APIDeps) (Handler, error) {
			if deps.Users == nil {
				return nil, errAPINotWired
			}
			return func(ctx context.Context, p json.RawMessage) (any, error) {
				var in struct {
					UserID string `json:"user_id"`
				}
				if err := decodeParams(p, &in); err != nil {
					return nil, err
				}
				if err := requireFields(map[string]string{"user_id": in.UserID}); err != nil {
					return nil, err
				}
				email, err := deps.Users.GetEmail(ctx, strings.TrimSpace(in.UserID))
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
		build: func(_ string, deps APIDeps) (Handler, error) {
			if deps.Notifications == nil {
				return nil, errAPINotWired
			}
			return func(ctx context.Context, p json.RawMessage) (any, error) {
				var in struct {
					UserID string `json:"user_id"`
					Title  string `json:"title"`
					Body   string `json:"body"`
				}
				if err := decodeParams(p, &in); err != nil {
					return nil, err
				}
				if err := requireFields(map[string]string{
					"user_id": in.UserID, "title": in.Title, "body": in.Body,
				}); err != nil {
					return nil, err
				}
				if err := deps.Notifications.Send(ctx, strings.TrimSpace(in.UserID),
					strings.TrimSpace(in.Title), in.Body); err != nil {
					return nil, err
				}
				return okResult(), nil
			}, nil
		},
		audit: func(_ string, p json.RawMessage) (string, string, string) {
			var in struct {
				UserID string `json:"user_id"`
				Title  string `json:"title"`
			}
			_ = json.Unmarshal(p, &in)
			return "notification.send", strings.TrimSpace(in.UserID), strings.TrimSpace(in.Title)
		},
	},
	{
		method: "api.kv.get",
		cap:    CapKVRead,
		build: func(pluginID string, deps APIDeps) (Handler, error) {
			if deps.KV == nil {
				return nil, errAPINotWired
			}
			return func(ctx context.Context, p json.RawMessage) (any, error) {
				var in struct {
					Key string `json:"key"`
				}
				if err := decodeParams(p, &in); err != nil {
					return nil, err
				}
				scoped, err := pluginKVKey(pluginID, in.Key)
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
		build: func(pluginID string, deps APIDeps) (Handler, error) {
			if deps.KV == nil {
				return nil, errAPINotWired
			}
			return func(ctx context.Context, p json.RawMessage) (any, error) {
				var in struct {
					Key   string `json:"key"`
					Value string `json:"value"`
				}
				if err := decodeParams(p, &in); err != nil {
					return nil, err
				}
				// value 允许为空串（"清空这个键"是合法语义），key 不允许。
				scoped, err := pluginKVKey(pluginID, in.Key)
				if err != nil {
					return nil, err
				}
				if err := deps.KV.Set(ctx, scoped, in.Value); err != nil {
					return nil, err
				}
				return okResult(), nil
			}, nil
		},
		audit: func(pluginID string, p json.RawMessage) (string, string, string) {
			var in struct {
				Key string `json:"key"`
			}
			_ = json.Unmarshal(p, &in)
			// target 落**加过前缀的全键**：审计里出现 "counter" 无法定位到某个插件。
			scoped, err := pluginKVKey(pluginID, in.Key)
			if err != nil {
				return "kv.set", in.Key, ""
			}
			return "kv.set", scoped, ""
		},
	},
	{
		method: "api.slots.set",
		cap:    CapSlotsWrite,
		write:  true,
		build: func(_ string, deps APIDeps) (Handler, error) {
			if deps.Slots == nil {
				return nil, errAPINotWired
			}
			return func(ctx context.Context, p json.RawMessage) (any, error) {
				var in struct {
					Slot  string         `json:"slot"`
					Kind  string         `json:"kind"`
					Props map[string]any `json:"props"`
					Sort  int            `json:"sort"`
				}
				if err := decodeParams(p, &in); err != nil {
					return nil, err
				}
				if err := requireFields(map[string]string{"slot": in.Slot, "kind": in.Kind}); err != nil {
					return nil, err
				}
				if err := deps.Slots.Set(ctx, strings.TrimSpace(in.Slot), strings.TrimSpace(in.Kind),
					in.Props, in.Sort); err != nil {
					return nil, err
				}
				return okResult(), nil
			}, nil
		},
		audit: func(_ string, p json.RawMessage) (string, string, string) {
			var in struct {
				Slot string `json:"slot"`
				Kind string `json:"kind"`
			}
			_ = json.Unmarshal(p, &in)
			return "slot.set", strings.TrimSpace(in.Slot), strings.TrimSpace(in.Kind)
		},
	},
	{
		method: "api.slots.delete",
		cap:    CapSlotsWrite,
		write:  true,
		build: func(_ string, deps APIDeps) (Handler, error) {
			if deps.Slots == nil {
				return nil, errAPINotWired
			}
			return func(ctx context.Context, p json.RawMessage) (any, error) {
				var in struct {
					Slot string `json:"slot"`
				}
				if err := decodeParams(p, &in); err != nil {
					return nil, err
				}
				if err := requireFields(map[string]string{"slot": in.Slot}); err != nil {
					return nil, err
				}
				if err := deps.Slots.Delete(ctx, strings.TrimSpace(in.Slot)); err != nil {
					return nil, err
				}
				return okResult(), nil
			}, nil
		},
		audit: func(_ string, p json.RawMessage) (string, string, string) {
			var in struct {
				Slot string `json:"slot"`
			}
			_ = json.Unmarshal(p, &in)
			return "slot.delete", strings.TrimSpace(in.Slot), ""
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
// # 校验放在查找阶段，而不是 handler 内部
//
// 这是刻意选择的、**更难被后来者弄错**的形态：
//
//   - handler 里写 `if !grants(cap) { return err }` 是「每个 handler 各自记得
//     检查」，新增 API 时漏写一行就变成越权，且没有任何东西会提醒你；
//   - 放在 lookupAPI 里之后，handler 的构造**只有这一条路径**，而这条路径
//     的第一个动作就是查表。漏写校验不再是一个可能的错误——没有地方可漏。
//
// 未授予时返回的不是业务 handler，而是 denyHandler：它**不持有 deps**，
// 内部没有任何 service 调用。因此即便有人把 denyHandler 里的判定删掉，
// 也不可能执行到业务代码——它附近根本没有可执行的东西。
func lookupAPI(method, pluginID string, caps CapabilitySet, deps APIDeps) (Handler, bool) {
	spec, ok := findAPI(method)
	if !ok {
		return nil, false
	}
	if !caps.grants(spec.cap) {
		return denyHandler(pluginID, spec.method, spec.cap), true
	}
	build, err := spec.build(pluginID, deps)
	if err != nil {
		// 能力有、接线没有：明确报错。绝不能返回 nil handler——那会让
		// 插件等到 IPC 超时，日志里什么都看不到。
		return func(context.Context, json.RawMessage) (any, error) {
			return nil, fmt.Errorf("%s: %w（插件 %s）", spec.method, err, pluginID)
		}, true
	}
	return spec.wrap(pluginID, deps, build), true
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

// wrap 给业务 handler 套上三件事：panic 兜底、审计（写类）、原样透传结果。
//
// 审计放在这里而不是每个 handler 里，是「所有写类 API 必须落审计」这条
// 约束唯一的可靠实现：handler 内部没有审计调用点，漏写是不可能的。
func (s apiSpec) wrap(pluginID string, deps APIDeps, h Handler) Handler {
	return func(ctx context.Context, p json.RawMessage) (res any, err error) {
		// handler 跑在 ipc.go 的 dispatch goroutine 里，一次 panic 就是整个
		// 后端进程崩溃。插件能控制的输入（任意 JSON、任意 key）必须在这里兜住。
		defer func() {
			if r := recover(); r != nil {
				logrus.Errorf("plugin: %s 调用 %s 时 panic: %v", pluginID, s.method, r)
				res, err = nil, fmt.Errorf("%s: 宿主内部错误", s.method)
			}
		}()

		res, err = h(ctx, p)
		if err != nil {
			return res, err
		}
		if !s.write {
			return res, nil
		}

		action, target, detail := "", "", ""
		if s.audit != nil {
			action, target, detail = s.audit(pluginID, p)
		}
		if s.audit == nil || action == "" {
			// 不该发生（TestEveryAPIMethodDeclaresACapability 保证 write 规格
			// 都带 audit，解析不出 action 的输入已在 handler 里被拒）。
			// 仍然落一条可辨认的记录，而不是静默跳过——「一次写没有审计」
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
			}).Error("plugin: 写操作审计记录失败（业务动作已生效）")
		}
		return res, nil
	}
}

// RegisterAPIs 把宿主 API 挂到插件连接上。
//
// 注册规则：
//   - 能力已授予 → 注册真实 handler（内部仍会再查一次能力，纵深防御）；
//   - 能力未授予 → 注册 denyHandler（只有拒绝逻辑，不持有任何依赖）；
//   - 未知方法名 → 根本不注册，插件的调用会得到 ipc.go 的 unknown method。
//
// 因此注册进 Conn 的 handler 与「该插件被允许做什么」严格一一对应，
// 不存在「注册了但靠 handler 自己记得检查」的中间态。
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
// 参数校验
// ---------------------------------------------------------------------------

// decodeParams 解出参数对象，并把 JSON 错误包成读得懂的中文错误。
//
// 不做 DisallowUnknownFields：插件多传字段（比如把整个 post 对象透传回来）
// 应当被容忍，收紧它只会在协议演进时制造兼容性事故。
func decodeParams(p json.RawMessage, dst any) error {
	if len(p) == 0 {
		return fmt.Errorf("参数缺失：需要 JSON 对象")
	}
	if err := json.Unmarshal(p, dst); err != nil {
		return fmt.Errorf("参数不是合法 JSON 对象: %w", err)
	}
	return nil
}

// requireFields 校验必填字符串字段非空。
//
// 为什么必须有：插件传 `{}` 时，struct 字段全是零值，直接交给 service 就是
// 「用空 postID 执行 SetStatus」——那可能命中一条空主键记录，也可能在
// service 层变成一个看不懂的 SQL 错误。校验在这里，错误信息才能指名
// **哪个字段**缺了，插件作者一眼能改。
//
// 只校验「非空」，不校验取值域（例如 status 是否属于 approved/pending…）：
// 合法取值定义在 internal/model 里，而本包 MUST NOT 引入 model（它依赖
// gorm）。取值域的校验属于 Task 7 的 service 适配层。
func requireFields(fields map[string]string) error {
	// 排序后报错：同一个请求永远得到同一条错误文案，测试可逐字断言。
	names := make([]string, 0, len(fields))
	for name := range fields {
		names = append(names, name)
	}
	sort.Strings(names)

	var missing []string
	for _, name := range names {
		if strings.TrimSpace(fields[name]) == "" {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("参数缺失: %s", strings.Join(missing, ", "))
	}
	return nil
}

// okResult 是写类 API 成功时的固定返回。
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
