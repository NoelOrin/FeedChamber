# 插件钩子系统实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让第三方 JS 插件以 Node 子进程形式接入站点，通过核心预定义的语义化钩子扩展功能，并能在壳层插槽中展示插件内容。

**Architecture:** 插件是 `node plugins/<name>/index.js` 子进程，宿主通过 stdin/stdout 上的 JSON-RPC 双向通信。宿主在业务动作**完成之后**广播事件（`SubscribeBroadcast`），分发给本机订阅了该钩子的插件；插件通过宿主暴露的结构化 API 读写数据，宿主在 `service` 层执行并落审计。壳层插槽只读已持久化数据，渲染路径不调用插件。

**Tech Stack:** Go 1.25（`os/exec` + `encoding/json`）、Node.js（插件侧，无框架依赖）、NATS（广播订阅）、React 19（插槽组件）

**Spec:** `openspec/changes/2026-11-04-plugin-hooks/`（proposal.md + specs/plugin-hooks + specs/shell-slots + specs/site-customization + specs/layered-architecture）

## Global Constraints

- **插件与后端同用户运行**（v1 决策）。"插件不直连数据库"是**约定而非强制**，文档不得声称强制隔离。
- **子进程环境变量 MUST 由白名单重建**，MUST NOT 基于 `os.Environ()` 构造。禁止下发 `DB_DSN` / `ADMIN_PASSWORD` / `*_SECRET` / `*_KEY`。
- **v1 不提供任何 `*.before_*` 同步钩子**。钩子在业务动作完成后触发，失败不回滚。
- **`internal/plugin` 不得引用 `internal/repository`**，不得持有 `*gorm.DB`。
- **广播订阅为纯增量**：不得改变现有 14 条队列订阅路径的语义。
- **插件 API 的写操作 MUST 与管理员操作走同一 service 入口**。
- 响应信封固定 `{ "code": 0, "msg": "success", "data": ... }`；写操作 `POST`，只读 `MAY GET`。
- 所有新后台路由 MUST 同时在 `internal/router/routes_admin.go` 挂 `RequirePermission` **且** 在 `internal/gateway/permission.go` 登记 path→permission 映射。
- 每个含有代码的模块目录变更 MUST 同步更新其 `AGENTS.md`（CI 会阻断）。

---

## 文件结构

### 后端新增

| 文件 | 职责 |
|---|---|
| `app/server/internal/plugin/host.go` | `Host` 接口与装配；插件生命周期编排 |
| `app/server/internal/plugin/supervisor.go` | `os/exec` 拉起 node、进程组、退避重启 |
| `app/server/internal/plugin/ipc.go` | stdio JSON-RPC 编解码（request / response / notification） |
| `app/server/internal/plugin/hooks.go` | 钩子名常量表、注册、分发 |
| `app/server/internal/plugin/api.go` | 宿主 API 注册、capability 校验、上下文失效 |
| `app/server/internal/plugin/manifest.go` | `plugin.json` 解析与校验 |
| `app/server/internal/plugin/slotapi.go` | `ctx.slots.*` 的实现（写插槽条目） |
| `app/server/internal/model/plugin_install.go` | 安装记录 + 状态机常量 |
| `app/server/internal/model/plugin_source.go` | 来源白名单 + 审批状态 |
| `app/server/internal/model/slot_entry.go` | 插槽条目 + 枚举 + Kind 常量 |
| `app/server/internal/repository/slot_entry.go` | 插槽仓储接口 + SQL 实现 |
| `app/server/internal/service/slot.go` | 插槽 CRUD、可见性过滤、审计 |
| `app/server/internal/handler/slot.go` | 插槽管理与公开读取端点 |

### 后端修改

| 文件 | 变更 |
|---|---|
| `app/server/internal/pkg/eventbus/eventbus.go` | 新增 `SubscribeBroadcast` |
| `app/server/internal/model/plugin.go` | 扩 `Manifest`/`Capabilities`/`ProtocolVersion`/`SourceID`/`Status` |
| `app/server/internal/model/permission.go` | 新增 `PermPluginRead` + seed 列表 |
| `app/server/internal/gateway/permission.go` | `plugin:read` 精确映射（读接口） |
| `app/server/internal/service/plugin.go` | 重写：安装、启停、来源审批 |
| `app/server/internal/handler/plugin.go` | 扩充端点 |
| `app/server/internal/router/routes_admin.go` | 挂权限码；新增插槽路由 |
| `app/server/internal/router/routes_public.go` | 新增公开插槽读取 |
| `app/server/internal/container/container.go` | 装配 `Host` |
| `app/server/internal/container/runtime.go` | `Start` 恢复插件；`Shutdown` 优雅停止 |
| `app/server/server/iris.go` | AutoMigrate 登记新模型 |
| `app/server/server/wiring.go` | 映射新 service 到 `handler.Deps` |

### 前端

| 文件 | 职责 |
|---|---|
| `app/web/components/slot/slot-host.tsx` | 读数据 + 可见性过滤 + 排序 + 渲染 |
| `app/web/components/slot/renderers/index.ts` | 渲染器注册表 |
| `app/web/components/slot/renderers/list.tsx` | 列表渲染 |
| `app/web/components/slot/renderers/stats.tsx` | 数字卡片 |
| `app/web/components/slot/renderers/markdown.tsx` | 受限 markdown |
| `packages/api-client/src/services/slot.ts` | 插槽 API |

---

## Task 1: 事件广播订阅（纯增量）

**Files:**
- Modify: `app/server/internal/pkg/eventbus/eventbus.go`
- Test: `app/server/internal/pkg/eventbus/broadcast_test.go`（新建）

**Interfaces:**
- Consumes: 现有 `EventBus` 结构体（`handlers map[string][]Handler`、`subs map[string]*nats.Subscription`、`nc *nats.Conn`）
- Produces: `func (eb *EventBus) SubscribeBroadcast(eventType string, handler Handler)`；`func (eb *EventBus) SubscribeBroadcastAsync(eventType string) Handler`（可选，本任务只做前者）

**背景：** 现有 `Subscribe` 用 `QueueSubscribe`（队列组），集群内每事件只被一个实例消费。插件需要广播，必须用不带 queue group 的 `Subscribe`。新增方法必须独立于现有 `handlers`/`subs` 两个 map，否则会与队列订阅互相污染。

- [ ] **Step 1: 写失败的测试**

创建 `app/server/internal/pkg/eventbus/broadcast_test.go`：

```go
package eventbus_test

import (
	"sync"
	"testing"
	"time"

	"feedchamber/internal/pkg/eventbus"
	"feedchamber/internal/pkg/testnats"

	"github.com/stretchr/testify/require"
)

// TestSubscribeBroadcastDeliversToAllHandlers 钉住广播语义：
// 同一事件类型注册多个广播 handler 时，每个都要收到。
func TestSubscribeBroadcastDeliversToAllHandlers(t *testing.T) {
	bus := testnats.NewEventBus(t)
	defer bus.Shutdown()

	var mu sync.Mutex
	got := map[string]int{}
	for _, name := range []string{"a", "b", "c"} {
		name := name
		bus.SubscribeBroadcast(eventbus.EventPostCreated, func(e eventbus.Event) {
			mu.Lock()
			got[name]++
			mu.Unlock()
		})
	}

	bus.Publish(eventbus.Event{Type: eventbus.EventPostCreated, Payload: map[string]any{"post_id": "p1"}})

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return got["a"] == 1 && got["b"] == 1 && got["c"] == 1
	}, 3*time.Second, 20*time.Millisecond)
}

// TestSubscribeBroadcastDoesNotChangeQueueSemantics 钉住「不破坏现有语义」：
// 队列订阅仍然是每事件只消费一次。
func TestSubscribeBroadcastDoesNotChangeQueueSemantics(t *testing.T) {
	bus := testnats.NewEventBus(t)
	defer bus.Shutdown()

	var mu sync.Mutex
	count := 0
	bus.Subscribe(eventbus.EventPostCreated, func(e eventbus.Event) {
		mu.Lock()
		count++
		mu.Unlock()
	})

	bus.Publish(eventbus.Event{Type: eventbus.EventPostCreated, Payload: map[string]any{}})

	// 等足够久确认没有重复消费。
	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return count == 1
	}, 3*time.Second, 20*time.Millisecond)

	time.Sleep(300 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, 1, count, "队列订阅必须只消费一次")
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
cd app/server && go test ./internal/pkg/eventbus/ -run TestSubscribeBroadcast -v
```
Expected: FAIL — `bus.SubscribeBroadcast undefined`

- [ ] **Step 3: 实现 SubscribeBroadcast**

在 `eventbus.go` 的 `Subscribe` 方法之后追加：

```go
// SubscribeBroadcast 订阅事件并保证广播语义：
// 同一事件会送达所有订阅方（含集群内其他实例的订阅方）。
//
// 与 Subscribe 的区别只有一个：不指定 queue group。NATS 的 core pub/sub 在
// 没有 queue group 时把消息分发给每个订阅者；有 queue group 时组内只选一个。
//
// 广播 handler 单独存一个 map：与队列订阅共用 handlers 会导致
// handleNATSMessage 把队列 handler 也跑一遍，等于改了现有语义。
func (eb *EventBus) SubscribeBroadcast(eventType string, handler Handler) {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	if eb.nc == nil {
		logrus.Errorf("eventbus: broadcast subscribe %q after shutdown", eventType)
		return
	}
	eb.broadcastHandlers[eventType] = append(eb.broadcastHandlers[eventType], handler)

	if len(eb.broadcastHandlers[eventType]) != 1 || eb.broadcastSubs[eventType] != nil {
		return
	}
	subject := eventSubjectPrefix + eventType
	sub, err := eb.nc.Subscribe(subject, func(msg *nats.Msg) {
		eb.handleBroadcastMessage(msg)
	})
	if err != nil {
		logrus.Errorf("eventbus: broadcast subscribe %q: %v", subject, err)
		return
	}
	eb.broadcastSubs[eventType] = sub
	_ = eb.nc.Flush()
}
```

在 `EventBus` 结构体中新增两个字段（与 `handlers`/`subs` 并列）：

```go
	// broadcastHandlers / broadcastSubs 与上面两组分开：
	// 广播订阅不带 queue group，集群内每个实例都会收到，交给本机 fan-out。
	broadcastHandlers map[string][]Handler
	broadcastSubs     map[string]*nats.Subscription
```

在 `New` 的构造处初始化：

```go
		broadcastHandlers: make(map[string][]Handler),
		broadcastSubs:     make(map[string]*nats.Subscription),
```

新增 `handleBroadcastMessage`（照 `handleNATSMessage` 的结构写，但走 broadcastHandlers）：

```go
// handleBroadcastMessage 把广播消息解包后分发给本机的广播 handler。
// 与 handleNATSMessage 一样在独立 context 上重建 trace。
func (eb *EventBus) handleBroadcastMessage(msg *nats.Msg) {
	var we wireEvent
	if err := json.Unmarshal(msg.Data, &we); err != nil {
		logrus.Errorf("eventbus: decode broadcast %q: %v", msg.Subject, err)
		return
	}
	ctx := consumerContext(msg.Header)
	eb.mu.RLock()
	hs := append([]Handler(nil), eb.broadcastHandlers[we.Type]...)
	eb.mu.RUnlock()
	for _, h := range hs {
		func() {
			defer func() {
				if r := recover(); r != nil {
					logrus.Errorf("eventbus: broadcast handler panic for %q: %v", we.Type, r)
				}
			}()
			h(Event{Type: we.Type, Payload: we.Payload, Ctx: ctx})
		}()
	}
}
```

在 `Shutdown` 中一并 `Unsubscribe` 广播订阅（照现有 `subs` 的清理方式写）。

- [ ] **Step 4: 运行测试确认通过**

```bash
cd app/server && go test ./internal/pkg/eventbus/ -run TestSubscribeBroadcast -v
```
Expected: PASS（两个测试都过）

- [ ] **Step 5: 跑该包全量测试确认无回归**

```bash
cd app/server && go test ./internal/pkg/eventbus/ -race
```
Expected: 全部 PASS

- [ ] **Step 6: 更新文档**

在 `docs/design/extension-points.md` 的「事件目录」表之后追加一段：

```markdown
### 两种订阅语义

- `Subscribe` — 队列订阅（NATS queue group）。**集群内每个事件只被消费一次**，用于通知、Webhook 这类「一次就够」的消费者。
- `SubscribeBroadcast` — 广播订阅（无 queue group）。**集群内每个订阅方都收到**，用于插件钩子这类「多个消费者都要知道」的场景。

两者互不影响：广播订阅不写入 `handlers`/`subs`，队列订阅的语义保持不变。
```

- [ ] **Step 7: 提交**

```bash
cd /Users/noelorin/OrinForum
git add app/server/internal/pkg/eventbus/ docs/design/extension-points.md
git commit -m "feat(eventbus): add SubscribeBroadcast for fan-out subscribers"
```

---

## Task 2: 插件 manifest 解析与校验

**Files:**
- Create: `app/server/internal/plugin/manifest.go`
- Test: `app/server/internal/plugin/manifest_test.go`

**Interfaces:**
- Produces: `type Manifest struct`、`func ParseManifest(raw []byte) (*Manifest, error)`、`func (m *Manifest) Validate() error`、常量 `ProtocolVersion1 = "1"`、`HookPostCreated = "post.created"` 等钩子常量。
- 后续任务依赖：`Manifest.Hooks []string`、`Manifest.Capabilities []string`、`Manifest.ProtocolVersion string`

**背景：** `plugin.json` 是插件的唯一契约文件。钩子名必须是核心预定义的，未知名必须被忽略而不是报错（spec：`未定义的钩子名 MUST 被忽略`）。

- [ ] **Step 1: 写失败的测试**

创建 `internal/plugin/manifest_test.go`：

```go
package plugin_test

import (
	"testing"

	"feedchamber/internal/plugin"

	"github.com/stretchr/testify/require"
)

func TestParseManifestValid(t *testing.T) {
	raw := []byte(`{
		"name": "hello",
		"version": "1.0.0",
		"protocol_version": "1",
		"hooks": ["post.created", "comment.created"],
		"capabilities": ["posts:read", "kv:write"]
	}`)
	m, err := plugin.ParseManifest(raw)
	require.NoError(t, err)
	require.Equal(t, "hello", m.Name)
	require.Equal(t, plugin.ProtocolVersion1, m.ProtocolVersion)
	require.Equal(t, []string{"post.created", "comment.created"}, m.Hooks)
	require.Equal(t, []string{"posts:read", "kv:write"}, m.Capabilities)
}

func TestParseManifestRejectsUnknownProtocol(t *testing.T) {
	raw := []byte(`{"name":"x","version":"1","protocol_version":"99"}`)
	_, err := plugin.ParseManifest(raw)
	require.Error(t, err)
	require.Contains(t, err.Error(), "protocol_version")
}

func TestParseManifestRequiresNameAndVersion(t *testing.T) {
	_, err := plugin.ParseManifest([]byte(`{"name":"","version":"1"}`))
	require.Error(t, err)

	_, err = plugin.ParseManifest([]byte(`{"name":"x","version":""}`))
	require.Error(t, err)
}

// 未定义的钩子名被忽略，不导致解析失败。
func TestParseManifestDropsUnknownHooks(t *testing.T) {
	raw := []byte(`{
		"name": "x", "version": "1", "protocol_version": "1",
		"hooks": ["post.created", "not.a.real.hook"]
	}`)
	m, err := plugin.ParseManifest(raw)
	require.NoError(t, err)
	require.Equal(t, []string{"post.created"}, m.Hooks,
		"未知钩子必须被丢弃而非报错")
}

func TestParseManifestRejectsBadJSON(t *testing.T) {
	_, err := plugin.ParseManifest([]byte(`{`))
	require.Error(t, err)
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
cd app/server && go test ./internal/plugin/ -run TestParseManifest -v
```
Expected: FAIL — package `feedchamber/internal/plugin` 不存在

- [ ] **Step 3: 实现 manifest.go**

创建 `internal/plugin/manifest.go`：

```go
// Package plugin 托管以 Node 子进程运行的第三方插件。
//
// 设计约束（见 openspec/changes/2026-11-04-plugin-hooks）：
//   - 插件进程由本包拉起，通过 stdio JSON-RPC 通信；
//   - 插件不加载进宿主进程，崩溃不影响宿主；
//   - 本包不引用 internal/repository，不持有 *gorm.DB。
package plugin

import (
	"encoding/json"
	"fmt"
	"sort"
)

// ProtocolVersion1 是当前唯一支持的插件协议版本。
// manifest 的 protocol_version 不在此集合内的一律拒绝加载。
const ProtocolVersion1 = "1"

// 钩子名常量。插件只能实现这里列出的钩子；未知名在解析时被丢弃。
const (
	HookPostCreated      = "post.created"
	HookPostUpdated      = "post.updated"
	HookPostClosed       = "post.closed"
	HookCommentCreated   = "comment.created"
	HookUserRegistered   = "user.registered"
	HookUserFollowed     = "user.followed"
	HookReportCreated    = "report.created"
	HookModerationDecided = "moderation.decided"
	HookCronDaily        = "cron.daily"
)

// knownHooks 是核心钩子白名单。
var knownHooks = map[string]struct{}{
	HookPostCreated:       {},
	HookPostUpdated:       {},
	HookPostClosed:        {},
	HookCommentCreated:    {},
	HookUserRegistered:    {},
	HookUserFollowed:      {},
	HookReportCreated:     {},
	HookModerationDecided: {},
	HookCronDaily:         {},
}

// KnownHooks 返回全部核心钩子名（排序后），供后台与文档使用。
func KnownHooks() []string {
	out := make([]string, 0, len(knownHooks))
	for h := range knownHooks {
		out = append(out, h)
	}
	sort.Strings(out)
	return out
}

// Manifest 是 plugin.json 的解析结果。
type Manifest struct {
	Name            string   `json:"name"`
	Version         string   `json:"version"`
	ProtocolVersion string   `json:"protocol_version"`
	Description     string   `json:"description,omitempty"`
	Author          string   `json:"author,omitempty"`
	// Entry 是入口文件名，默认 index.js。
	Entry        string   `json:"entry,omitempty"`
	Hooks        []string `json:"hooks"`
	Capabilities []string `json:"capabilities"`
}

// ParseManifest 解析并校验 plugin.json。
//
// 未知钩子被丢弃而不是报错：核心新增钩子是常事，旧插件不该因此被拒。
// 反过来，未知 capability 不丢弃——capability 决定权限，写错就该报错。
func ParseManifest(raw []byte) (*Manifest, error) {
	var m Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("plugin manifest 不是合法 JSON: %w", err)
	}
	if m.Name == "" {
		return nil, fmt.Errorf("plugin manifest 缺少 name")
	}
	if m.Version == "" {
		return nil, fmt.Errorf("plugin manifest 缺少 version")
	}
	if m.ProtocolVersion != ProtocolVersion1 {
		return nil, fmt.Errorf("plugin protocol_version 不支持: %q（需要 %q）", m.ProtocolVersion, ProtocolVersion1)
	}
	if m.Entry == "" {
		m.Entry = "index.js"
	}

	filtered := make([]string, 0, len(m.Hooks))
	for _, h := range m.Hooks {
		if _, ok := knownHooks[h]; ok {
			filtered = append(filtered, h)
		}
	}
	m.Hooks = filtered
	return &m, nil
}

// Validate 校验 capability 是否合法。单独一个方法是因为 capability
// 需要与站点已知的能力集比对，而能力集由 api.go 提供（避免本文件反向依赖）。
func (m *Manifest) Validate(knownCaps map[string]struct{}) error {
	for _, c := range m.Capabilities {
		if _, ok := knownCaps[c]; !ok {
			return fmt.Errorf("未知 capability: %q", c)
		}
	}
	if len(m.Hooks) == 0 {
		return fmt.Errorf("插件至少需要实现一个钩子")
	}
	return nil
}
```

- [ ] **Step 4: 运行测试确认通过**

```bash
cd app/server && go test ./internal/plugin/ -run TestParseManifest -v
```
Expected: PASS

- [ ] **Step 5: 提交**

```bash
cd /Users/noelorin/OrinForum
git add app/server/internal/plugin/
git commit -m "feat(plugin): parse and validate plugin.json manifest"
```

---

## Task 3: stdio JSON-RPC 编解码

**Files:**
- Create: `app/server/internal/plugin/ipc.go`
- Test: `app/server/internal/plugin/ipc_test.go`

**Interfaces:**
- Produces:
  - `type Frame struct { ID int64; Method string; Params json.RawMessage; Result json.RawMessage; Error *FrameError }`
  - `type Conn struct{...}`、`func NewConn(r io.Reader, w io.Writer) *Conn`
  - `func (c *Conn) Call(ctx context.Context, method string, params any) (json.RawMessage, error)`
  - `func (c *Conn) Notify(method string, params any) error`
  - `func (c *Conn) Handle(method string, fn func(context.Context, json.RawMessage) (any, error))`
  - `func (c *Conn) Start() error` / `func (c *Conn) Close() error`
- 后续任务依赖：`Conn.Call` 用于钩子分发；`Conn.Handle` 用于接收插件的 API 调用。

**背景：** 双向 RPC。宿主调用插件（钩子、cron）与插件调用宿主（`ctx.posts.*`）走同一对管道，靠 `ID` 关联请求与响应，靠 `Method` 区分方向。帧是 NDJSON（每行一个 JSON）。

- [ ] **Step 1: 写失败的测试**

创建 `internal/plugin/ipc_test.go`：

```go
package plugin_test

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"testing"
	"time"

	"feedchamber/internal/plugin"

	"github.com/stretchr/testify/require"
)

// newPipePair 返回一对接好的 Conn，用于双向通信测试。
func newPipePair(t *testing.T) (*plugin.Conn, *plugin.Conn) {
	t.Helper()
	c1, c2 := net.Pipe()
	a := plugin.NewConn(c1, c1)
	b := plugin.NewConn(c2, c2)
	require.NoError(t, a.Start())
	require.NoError(t, b.Start())
	t.Cleanup(func() {
		_ = a.Close()
		_ = b.Close()
	})
	return a, b
}

func TestConnCallReturnsResult(t *testing.T) {
	a, b := newPipePair(t)

	b.Handle("hook.post.created", func(ctx context.Context, params json.RawMessage) (any, error) {
		var in struct{ ID string `json:"id"` }
		require.NoError(t, json.Unmarshal(params, &in))
		return map[string]string{"seen": in.ID}, nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	raw, err := a.Call(ctx, "hook.post.created", map[string]string{"id": "p1"})
	require.NoError(t, err)

	var out struct{ Seen string `json:"seen"` }
	require.NoError(t, json.Unmarshal(raw, &out))
	require.Equal(t, "p1", out.Seen)
}

func TestConnCallPropagatesHandlerError(t *testing.T) {
	a, b := newPipePair(t)

	b.Handle("boom", func(ctx context.Context, params json.RawMessage) (any, error) {
		return nil, io.ErrUnexpectedEOF
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := a.Call(ctx, "boom", nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unexpected EOF")
}

func TestConnCallTimesOut(t *testing.T) {
	a, _ := newPipePair(t) // 对端不注册任何 handler

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	_, err := a.Call(ctx, "nobody.home", nil)
	require.Error(t, err)
	require.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestConnUnknownMethodReturnsError(t *testing.T) {
	a, _ := newPipePair(t)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := a.Call(ctx, "no.such.method", nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown method")
}

func TestConnNotifyDoesNotWaitForResponse(t *testing.T) {
	a, b := newPipePair(t)

	got := make(chan struct{}, 1)
	b.Handle("event", func(ctx context.Context, params json.RawMessage) (any, error) {
		got <- struct{}{}
		return nil, nil
	})

	require.NoError(t, a.Notify("event", map[string]string{"k": "v"}))
	select {
	case <-got:
	case <-time.After(2 * time.Second):
		t.Fatal("notify 未被处理")
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
cd app/server && go test ./internal/plugin/ -run TestConn -v
```
Expected: FAIL — `plugin.NewConn undefined`

- [ ] **Step 3: 实现 ipc.go**

创建 `internal/plugin/ipc.go`：

```go
package plugin

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
)

// Frame 是 IPC 的一帧（NDJSON：一行一个 JSON 对象）。
//
// 请求与响应共用这个结构：有 Method 的是请求，有 Result/Error 且无 Method 的是响应。
type Frame struct {
	ID     int64           `json:"id,omitempty"`
	Method string          `json:"method,omitempty"`
	Params json.RawMessage `json:"params,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *FrameError     `json:"error,omitempty"`
}

// FrameError 是远端返回的错误。
type FrameError struct {
	Message string `json:"message"`
}

func (e *FrameError) Error() string { return e.Message }

// Handler 处理一条来自对端的请求。
type Handler func(ctx context.Context, params json.RawMessage) (any, error)

// Conn 是一条双向 JSON-RPC 连接（通常挂在子进程的 stdin/stdout 上）。
//
// 读写各占一个 goroutine：读循环只把响应塞回等待者、把请求交给 handler；
// 调用方用 Call 阻塞等结果。所有写操作串行化，避免半行交错的帧。
type Conn struct {
	r io.Reader
	w io.Writer

	mu       sync.Mutex
	writeMu  sync.Mutex
	handlers map[string]Handler
	pending  map[int64]chan Frame
	nextID   atomic.Int64

	closed  chan struct{}
	closeOn sync.Once
}

// NewConn 用一对读写流构造连接。子进程场景下 r 与 w 是同一个管道。
func NewConn(r io.Reader, w io.Writer) *Conn {
	return &Conn{
		r:        r,
		w:        w,
		handlers: make(map[string]Handler),
		pending:  make(map[int64]chan Frame),
		closed:   make(chan struct{}),
	}
}

// Handle 注册对端可调用的方法。
func (c *Conn) Handle(method string, fn Handler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.handlers[method] = fn
}

// Start 启动读循环。
func (c *Conn) Start() error {
	go c.readLoop()
	return nil
}

// Close 关闭连接；在途调用会收到错误。
func (c *Conn) Close() error {
	c.closeOn.Do(func() { close(c.closed) })
	return nil
}

// Call 发一条请求并等待响应。ctx 取消时返回 ctx.Err()。
func (c *Conn) Call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	id := c.nextID.Add(1)
	ch := make(chan Frame, 1)

	c.mu.Lock()
	c.pending[id] = ch
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
	}()

	raw, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("encode params: %w", err)
	}
	if err := c.write(Frame{ID: id, Method: method, Params: raw}); err != nil {
		return nil, err
	}

	select {
	case f := <-ch:
		if f.Error != nil {
			return nil, f.Error
		}
		return f.Result, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-c.closed:
		return nil, fmt.Errorf("ipc closed")
	}
}

// Notify 发一条不需要响应的请求。
func (c *Conn) Notify(method string, params any) error {
	raw, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("encode params: %w", err)
	}
	return c.write(Frame{Method: method, Params: raw})
}

func (c *Conn) write(f Frame) error {
	data, err := json.Marshal(f)
	if err != nil {
		return fmt.Errorf("encode frame: %w", err)
	}
	data = append(data, '\n')
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	_, err = c.w.Write(data)
	return err
}

func (c *Conn) readLoop() {
	sc := bufio.NewScanner(c.r)
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	for sc.Scan() {
		var f Frame
		if err := json.Unmarshal(sc.Bytes(), &f); err != nil {
			continue // 坏帧跳过，不让单个脏行拖垮连接
		}
		if f.Method == "" {
			// 响应
			c.mu.Lock()
			ch := c.pending[f.ID]
			c.mu.Unlock()
			if ch != nil {
				ch <- f
			}
			continue
		}
		go c.dispatch(f) // 插件侧同时处理多个入站请求
	}
	_ = c.Close()
}

func (c *Conn) dispatch(f Frame) {
	c.mu.Lock()
	fn, ok := c.handlers[f.Method]
	c.mu.Unlock()

	if !ok {
		_ = c.write(Frame{ID: f.ID, Error: &FrameError{Message: "unknown method: " + f.Method}})
		return
	}
	res, err := fn(context.Background(), f.Params)
	if err != nil {
		_ = c.write(Frame{ID: f.ID, Error: &FrameError{Message: err.Error()}})
		return
	}
	raw, err := json.Marshal(res)
	if err != nil {
		_ = c.write(Frame{ID: f.ID, Error: &FrameError{Message: "encode result: " + err.Error()}})
		return
	}
	_ = c.write(Frame{ID: f.ID, Result: raw})
}
```

- [ ] **Step 4: 运行测试确认通过**

```bash
cd app/server && go test ./internal/plugin/ -run TestConn -race -v
```
Expected: 全部 PASS

- [ ] **Step 5: 提交**

```bash
cd /Users/noelorin/OrinForum
git add app/server/internal/plugin/
git commit -m "feat(plugin): bidirectional JSON-RPC over stdio"
```

---

（后续任务：Task 4 子进程 supervisor；Task 5 钩子分发；Task 6 宿主 API；Task 7 生命周期与装配；Task 8 安装与来源审批；Task 9 插槽后端；Task 10 插槽前端；Task 11 CLI。见续篇。）

---

## Task 4: 子进程 supervisor（拉起、环境白名单、退避重启）

**Files:**
- Create: `app/server/internal/plugin/supervisor.go`
- Test: `app/server/internal/plugin/supervisor_test.go`

**Interfaces:**
- Consumes: `NewConn(r, w)`、`Conn.Start()`、`Conn.Close()`（Task 3）
- Produces:
  - `type Supervisor struct{...}`
  - `func NewSupervisor(opts SupervisorOptions) *Supervisor`
  - `type SupervisorOptions struct { NodePath string; PluginDir string; Entry string; Env []string; MaxRestarts int; OnCrash func(reason string) }`
  - `func (s *Supervisor) Start(ctx context.Context) (*Conn, error)`
  - `func (s *Supervisor) Stop() error`
  - `func buildPluginEnv(whitelist []string, extra map[string]string) []string`（包内函数，测试直接调）
- 后续任务依赖：`Supervisor.Start` 返回已握手的 `*Conn`。

**背景：** 这是 spec 里唯一可执行的安全防线所在——`cmd.Env` 必须由白名单**重建**，不得基于 `os.Environ()`。测试要正面钉住这一点。

- [ ] **Step 1: 写失败的测试**

创建 `internal/plugin/supervisor_test.go`：

```go
package plugin

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// buildPluginEnv 必须白名单重建，不得继承宿主环境。
func TestBuildPluginEnvDoesNotInheritHost(t *testing.T) {
	t.Setenv("DB_DSN", "postgres://secret")
	t.Setenv("ADMIN_PASSWORD", "hunter2")

	env := buildPluginEnv([]string{"PATH", "NODE_ENV"}, map[string]string{"PLUGIN_SECRET": "ok"})

	forbidden := []string{"DB_DSN=", "ADMIN_PASSWORD="}
	for _, f := range forbidden {
		for _, e := range env {
			require.NotContains(t, e, f, "子进程环境不得包含 %s", f)
		}
	}
	require.Contains(t, env, "PLUGIN_SECRET=ok")
}

func TestBuildPluginEnvOmitsUnsetWhitelistEntries(t *testing.T) {
	os.Unsetenv("FC_TEST_ABSENT")
	env := buildPluginEnv([]string{"FC_TEST_ABSENT", "PATH"}, nil)
	for _, e := range env {
		require.NotContains(t, e, "FC_TEST_ABSENT=")
	}
}

func writeTestPlugin(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "index.js"), []byte(body), 0o644))
	return dir
}

func TestSupervisorStartsAndHandshakes(t *testing.T) {
	if _, err := os.Stat("/usr/bin/node"); err != nil {
		if _, err2 := os.Stat("/usr/local/bin/node"); err2 != nil {
			t.Skip("node 不可用，跳过子进程测试")
		}
	}
	dir := writeTestPlugin(t, `
process.stdin.setEncoding('utf8');
let buf = '';
process.stdin.on('data', (d) => {
  buf += d;
  let i;
  while ((i = buf.indexOf('\n')) >= 0) {
    const line = buf.slice(0, i); buf = buf.slice(i + 1);
    if (!line.trim()) continue;
    const f = JSON.parse(line);
    if (f.method === 'hook.cron.daily') {
      process.stdout.write(JSON.stringify({ id: f.id, result: { ok: true } }) + '\n');
    }
  }
});
`)

	s := NewSupervisor(SupervisorOptions{
		NodePath: "node",
		PluginDir: dir,
		Entry:     "index.js",
		MaxRestarts: 2,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := s.Start(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Stop() })

	raw, err := conn.Call(ctx, "hook.cron.daily", nil)
	require.NoError(t, err)
	require.Contains(t, string(raw), "ok")
}

func TestSupervisorRestartsWithBackoff(t *testing.T) {
	if _, err := os.Stat("/usr/bin/node"); err != nil {
		if _, err2 := os.Stat("/usr/local/bin/node"); err2 != nil {
			t.Skip("node 不可用")
		}
	}
	// 插件启动后立刻退出，触发重启并最终达到上限。
	dir := writeTestPlugin(t, `process.exit(1);`)

	crashes := make(chan int, 8)
	s := NewSupervisor(SupervisorOptions{
		NodePath: "node", PluginDir: dir, Entry: "index.js",
		MaxRestarts: 2,
		OnCrash: func(string) { crashes <- 1 },
	})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, _ = s.Start(ctx)

	select {
	case <-crashes:
	case <-time.After(8 * time.Second):
		t.Fatal("未观测到崩溃回调")
	}
	_ = s.Stop()
	require.False(t, s.Running(), "超过重启上限后必须停止")
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
cd app/server && go test ./internal/plugin/ -run 'TestBuildPluginEnv|TestSupervisor' -v
```
Expected: FAIL — `buildPluginEnv` / `NewSupervisor` 未定义

- [ ] **Step 3: 实现 supervisor.go**

创建 `internal/plugin/supervisor.go`：

```go
package plugin

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// envWhitelist 是允许下发给插件子进程的宿主变量。
//
// 这是本方案唯一可执行的数据访问防线：插件与后端同用户，能读文件系统、
// 也能自己连数据库，但至少不该从 process.env 里直接拿到 DB_DSN。
// 因此启动子进程时 Env 必须由白名单重建（见 buildPluginEnv），
// 绝不能 os.Environ() 起底再删几个——那是黑名单，漏一个就全漏。
var envWhitelist = []string{
	"PATH",
	"NODE_ENV",
	"NODE_OPTIONS",
	"TZ",
	"LANG",
}

// buildPluginEnv 按白名单重建子进程环境，并追加插件专属变量。
//
// 白名单里名字存在但宿主未设置时跳过（不写空值），
// 否则 node 会把空 PATH 当成有效值。
func buildPluginEnv(whitelist []string, extra map[string]string) []string {
	out := make([]string, 0, len(whitelist)+len(extra))
	for _, k := range whitelist {
		if v, ok := os.LookupEnv(k); ok {
			out = append(out, k+"="+v)
		}
	}
	for k, v := range extra {
		out = append(out, k+"="+v)
	}
	return out
}

// SupervisorOptions 是拉起一个插件所需的配置。
type SupervisorOptions struct {
	NodePath    string
	PluginDir   string
	Entry       string
	ExtraEnv    map[string]string
	MaxRestarts int
	// OnCrash 在每次崩溃时回调，用于更新落库状态。
	OnCrash func(reason string)
}

// Supervisor 托管单个插件的子进程生命周期。
type Supervisor struct {
	opts SupervisorOptions

	mu       sync.Mutex
	cmd      *exec.Cmd
	conn     *Conn
	restarts int
	running  bool
	stopped  bool
	cancel   context.CancelFunc
}

func NewSupervisor(opts SupervisorOptions) *Supervisor {
	if opts.MaxRestarts == 0 {
		opts.MaxRestarts = 5
	}
	if opts.NodePath == "" {
		opts.NodePath = "node"
	}
	return &Supervisor{opts: opts}
}

// Running 报告插件当前是否在运行。
func (s *Supervisor) Running() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

// Start 拉起子进程并建立 IPC。
func (s *Supervisor) Start(ctx context.Context) (*Conn, error) {
	childCtx, cancel := context.WithCancel(ctx)
	s.mu.Lock()
	s.cancel = cancel
	s.mu.Unlock()
	return s.spawn(childCtx)
}

func (s *Supervisor) spawn(ctx context.Context) (*Conn, error) {
	entry := filepath.Join(s.opts.PluginDir, s.opts.Entry)
	cmd := exec.CommandContext(ctx, s.opts.NodePath, entry)
	cmd.Dir = s.opts.PluginDir
	cmd.Env = buildPluginEnv(envWhitelist, s.opts.ExtraEnv)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("plugin stdin: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("plugin stdout: %w", err)
	}
	// stderr 直接继到宿主 stderr，插件日志才能在 docker logs 里看见。
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start plugin %s: %w", s.opts.PluginDir, err)
	}

	conn := NewConn(stdout, stdin)
	if err := conn.Start(); err != nil {
		_ = cmd.Process.Kill()
		return nil, err
	}

	s.mu.Lock()
	s.cmd = cmd
	s.conn = conn
	s.running = true
	s.mu.Unlock()

	go s.watch(ctx, cmd)
	return conn, nil
}

// watch 等子进程退出，按退避重启，超过上限后停止。
func (s *Supervisor) watch(ctx context.Context, cmd *exec.Cmd) {
	err := cmd.Wait()

	s.mu.Lock()
	s.running = false
	stopped := s.stopped
	restarts := s.restarts
	conn := s.conn
	s.mu.Unlock()

	if stopped {
		return
	}
	reason := "exited"
	if err != nil {
		reason = err.Error()
	}
	if s.opts.OnCrash != nil {
		s.opts.OnCrash(reason)
	}
	if conn != nil {
		_ = conn.Close()
	}

	if restarts >= s.opts.MaxRestarts {
		logrus.Errorf("plugin: %s 连续崩溃 %d 次，停止重启", s.opts.PluginDir, restarts)
		return
	}

	// 退避：1s → 2s → 4s → 8s → 16s
	backoff := time.Second << uint(restarts)
	logrus.Warnf("plugin: %s 退出（%s），%s 后重启", s.opts.PluginDir, reason, backoff)

	select {
	case <-time.After(backoff):
	case <-ctx.Done():
		return
	}

	s.mu.Lock()
	s.restarts++
	s.mu.Unlock()

	if _, err := s.spawn(ctx); err != nil {
		logrus.Errorf("plugin: %s 重启失败: %v", s.opts.PluginDir, err)
	}
}

// Stop 停止子进程且不再重启。
func (s *Supervisor) Stop() error {
	s.mu.Lock()
	s.stopped = true
	conn := s.conn
	cancel := s.cancel
	cmd := s.cmd
	s.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if conn != nil {
		_ = conn.Close()
	}
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
	return nil
}
```

- [ ] **Step 4: 运行测试确认通过**

```bash
cd app/server && go test ./internal/plugin/ -run 'TestBuildPluginEnv|TestSupervisor' -race -v
```
Expected: PASS（无 node 时后两个测试自动 skip，需在 CI 上确认不 skip）

- [ ] **Step 5: 提交**

```bash
cd /Users/noelorin/OrinForum
git add app/server/internal/plugin/
git commit -m "feat(plugin): supervise node subprocess with env whitelist and backoff restarts"
```

---

## Task 5: 钩子注册与分发

**Files:**
- Create: `app/server/internal/plugin/hooks.go`
- Test: `app/server/internal/plugin/hooks_test.go`

**Interfaces:**
- Consumes: `Conn.Call`（Task 3）、`Manifest.Hooks`（Task 2）、`Supervisor`（Task 4）
- Produces:
  - `type Host struct{...}`（本任务先只做钩子分发所需的部分）
  - `func (h *Host) RegisterPlugin(id string, m *Manifest, s *Supervisor)`
  - `func (h *Host) Dispatch(ctx context.Context, hook string, payload map[string]any)`
  - `func (h *Host) Subscribe(bus Broadcaster)`，其中 `type Broadcaster interface { SubscribeBroadcast(eventType string, handler func(eventType string, payload map[string]any)) }`
- 后续任务依赖：`Host.Dispatch` 由事件桥接调用。

**背景：** 一条事件可能对应多个插件。分发改成：按事件类型取订阅了该钩子的插件 → 并发调用各自 `Conn.Call` → 任一失败只记日志，不影响其他插件。

- [ ] **Step 1: 写失败的测试**

创建 `internal/plugin/hooks_test.go`：

```go
package plugin

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// fakeConn 记录被调用的钩子名。
type fakeConn struct{ calls *[]string }

func TestHostDispatchFansOutToAllSubscribers(t *testing.T) {
	var mu sync.Mutex
	seen := map[string][]string{}

	h := NewHost()
	for _, id := range []string{"a", "b"} {
		id := id
		m := &Manifest{Name: id, Hooks: []string{HookPostCreated}}
		h.RegisterPluginWithCaller(id, m, func(ctx context.Context, method string, params any) ([]byte, error) {
			mu.Lock()
			seen[id] = append(seen[id], method)
			mu.Unlock()
			return []byte(`{}`), nil
		})
	}

	h.DispatchHook(context.Background(), HookPostCreated, map[string]any{"post_id": "p1"})

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(seen["a"]) == 1 && len(seen["b"]) == 1
	}, 2*time.Second, 10*time.Millisecond, "两个插件都必须收到")
}

func TestHostDispatchIgnoresNonSubscribers(t *testing.T) {
	var mu sync.Mutex
	called := 0
	h := NewHost()
	h.RegisterPluginWithCaller("a", &Manifest{Name: "a", Hooks: []string{HookCommentCreated}},
		func(ctx context.Context, method string, params any) ([]byte, error) {
			mu.Lock()
			called++
			mu.Unlock()
			return []byte(`{}`), nil
		})

	h.DispatchHook(context.Background(), HookPostCreated, nil)

	time.Sleep(300 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, 0, called, "未订阅该钩子的插件不得被调用")
}

func TestHostDispatchFailureDoesNotBlockOthers(t *testing.T) {
	var mu sync.Mutex
	okCalled := 0
	h := NewHost()

	h.RegisterPluginWithCaller("bad", &Manifest{Name: "bad", Hooks: []string{HookPostCreated}},
		func(ctx context.Context, method string, params any) ([]byte, error) {
			return nil, errors.New("boom")
		})
	h.RegisterPluginWithCaller("good", &Manifest{Name: "good", Hooks: []string{HookPostCreated}},
		func(ctx context.Context, method string, params any) ([]byte, error) {
			mu.Lock()
			okCalled++
			mu.Unlock()
			return []byte(`{}`), nil
		})

	h.DispatchHook(context.Background(), HookPostCreated, nil)

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return okCalled == 1
	}, 2*time.Second, 10*time.Millisecond, "一个插件失败不得影响其他插件")
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
cd app/server && go test ./internal/plugin/ -run TestHostDispatch -v
```
Expected: FAIL — `NewHost` 未定义

- [ ] **Step 3: 实现 hooks.go**

创建 `internal/plugin/hooks.go`：

```go
package plugin

import (
	"context"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// PluginCaller 是宿主持有的、对一个插件发起调用的最小能力。
// 抽成函数类型而不是接口：测试里塞一个闭包就够了，不必造完整 Conn。
type PluginCaller func(ctx context.Context, method string, params any) ([]byte, error)

// hookTimeout 是单次钩子调用的上限。
//
// 钩子是异步的（在业务动作完成之后触发），所以超时不会影响用户请求；
// 但插件卡死会一直占着 goroutine，仍需兜底。
const hookTimeout = 5 * time.Second

// Host 是插件注册表与钩子分发器。
type Host struct {
	mu      sync.RWMutex
	plugins map[string]*registeredPlugin
}

type registeredPlugin struct {
	manifest *Manifest
	call     PluginCaller
}

func NewHost() *Host {
	return &Host{plugins: make(map[string]*registeredPlugin)}
}

// RegisterPlugin 登记一个已启动的插件。
func (h *Host) RegisterPlugin(id string, m *Manifest, conn *Conn) {
	h.RegisterPluginWithCaller(id, m, func(ctx context.Context, method string, params any) ([]byte, error) {
		return conn.Call(ctx, method, params)
	})
}

// RegisterPluginWithCaller 是 RegisterPlugin 的接缝版本，测试用。
func (h *Host) RegisterPluginWithCaller(id string, m *Manifest, call PluginCaller) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.plugins[id] = &registeredPlugin{manifest: m, call: call}
}

// UnregisterPlugin 摘除插件。
func (h *Host) UnregisterPlugin(id string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.plugins, id)
}

// DispatchHook 把一次钩子事件分发给所有订阅它的插件。
//
// 并发调用：一个插件慢不该拖住其他插件。任一失败只记日志——
// 钩子是事后通知，失败不能反过来影响已经完成的业务动作。
func (h *Host) DispatchHook(ctx context.Context, hook string, payload map[string]any) {
	targets := h.subscribers(hook)
	if len(targets) == 0 {
		return
	}

	var wg sync.WaitGroup
	for id, c := range targets {
		wg.Add(1)
		go func(id string, call PluginCaller) {
			defer wg.Done()
			callCtx, cancel := context.WithTimeout(ctx, hookTimeout)
			defer cancel()
			if _, err := call(callCtx, "hook."+hook, payload); err != nil {
				logrus.WithError(err).Warnf("plugin: %s 处理钩子 %s 失败", id, hook)
			}
		}(id, c)
	}
	wg.Wait()
}

// subscribers 返回订阅了指定钩子的插件调用入口。
func (h *Host) subscribers(hook string) map[string]PluginCaller {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make(map[string]PluginCaller)
	for id, p := range h.plugins {
		for _, hr := range p.manifest.Hooks {
			if hr == hook {
				out[id] = p.call
				break
			}
		}
	}
	return out
}

// Broadcaster 是事件总线的接缝，避免本包直接依赖 eventbus 的具体类型。
type Broadcaster interface {
	SubscribeBroadcast(eventType string, handler func(eventType string, payload map[string]any))
}

// SubscribeEvents 把事件总线的广播事件桥接到钩子分发。
//
// 事件类型与钩子名刻意保持同名（post.created 既是事件也是钩子），
// 这样桥接是一张恒等映射，不需要维护对照表。
func (h *Host) SubscribeEvents(b Broadcaster) {
	for _, hook := range KnownHooks() {
		hook := hook
		b.SubscribeBroadcast(hook, func(eventType string, payload map[string]any) {
			h.DispatchHook(context.Background(), hook, payload)
		})
	}
}
```

- [ ] **Step 4: 运行测试确认通过**

```bash
cd app/server && go test ./internal/plugin/ -run TestHostDispatch -race -v
```
Expected: PASS

- [ ] **Step 5: 提交**

```bash
cd /Users/noelorin/OrinForum
git add app/server/internal/plugin/
git commit -m "feat(plugin): dispatch semantic hooks to subscribed plugins"
```

---

## Task 6: 宿主 API（capability 校验 + 上下文失效）

**Files:**
- Create: `app/server/internal/plugin/api.go`
- Test: `app/server/internal/plugin/api_test.go`

**Interfaces:**
- Consumes: `Conn.Handle`（Task 3）
- Produces:
  - `type CapabilitySet map[string]struct{}`
  - `func KnownCapabilities() []string`
  - `type APIDeps struct{ Posts PostWriter; Comments CommentWriter; Users UserReader; Notifications Notifier; KV KVStore; Slots SlotWriter; Audit AuditRecorder }`
  - `func RegisterAPIs(conn *Conn, pluginID string, caps CapabilitySet, deps APIDeps)`
  - 各接缝接口（`PostWriter` 等）由 Task 7 用真实 service 适配
- 后续任务依赖：`RegisterAPIs` 在插件启动后调用。

**背景：** 插件调用宿主 API 走同一对管道，`method` 形如 `api.posts.setStatus`，`params` 是参数对象。capability 未声明的调用在运行时拒绝（spec 要求）。

- [ ] **Step 1: 写失败的测试**

创建 `internal/plugin/api_test.go`：

```go
package plugin

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

type fakePostWriter struct{ calls []string; err error }

func (f *fakePostWriter) SetStatus(ctx context.Context, postID, status string) error {
	f.calls = append(f.calls, postID+":"+status)
	return f.err
}

type fakeAudit struct{ records []string }

func (f *fakeAudit) RecordPluginAction(ctx context.Context, pluginID, action, target, detail string) error {
	f.records = append(f.records, pluginID+"|"+action+"|"+target)
	return nil
}

func TestRegisterAPIsRejectsUndeclaredCapability(t *testing.T) {
	posts := &fakePostWriter{}
	deps := APIDeps{Posts: posts, Audit: &fakeAudit{}}

	caps := CapabilitySet{"posts:read": {}} // 只有读
	err := invokeAPI(t, caps, deps, "api.posts.setStatus", map[string]string{
		"post_id": "p1", "status": "pending",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "capability")
	require.Empty(t, posts.calls, "未声明能力的调用不得触达 service")
}

func TestRegisterAPIsAllowsDeclaredCapability(t *testing.T) {
	posts := &fakePostWriter{}
	audit := &fakeAudit{}
	deps := APIDeps{Posts: posts, Audit: audit}

	caps := CapabilitySet{"posts:write": {}}
	require.NoError(t, invokeAPI(t, caps, deps, "api.posts.setStatus", map[string]string{
		"post_id": "p1", "status": "pending",
	}))
	require.Equal(t, []string{"p1:pending"}, posts.calls)
	require.Len(t, audit.records, 1, "写类 API 必须落审计")
}

// invokeAPI 是测试辅助：把一次 API 调用直接路由到 handler。
func invokeAPI(t *testing.T, caps CapabilitySet, deps APIDeps, method string, params any) error {
	t.Helper()
	raw, err := json.Marshal(params)
	require.NoError(t, err)
	fn, ok := lookupAPI(method, "test-plugin", caps, deps)
	if !ok {
		return errUnknownMethod(method)
	}
	_, err = fn(context.Background(), raw)
	return err
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
cd app/server && go test ./internal/plugin/ -run TestRegisterAPIs -v
```
Expected: FAIL — `CapabilitySet` 等未定义

- [ ] **Step 3: 实现 api.go**

创建 `internal/plugin/api.go`：

```go
package plugin

import (
	"context"
	"encoding/json"
	"fmt"
)

// 能力常量。插件在 plugin.json 的 capabilities 里声明，运行时逐个校验。
const (
	CapPostsRead       = "posts:read"
	CapPostsWrite      = "posts:write"
	CapCommentsRead    = "comments:read"
	CapCommentsWrite   = "comments:write"
	CapUsersRead       = "users:read"
	CapNotifySend      = "notifications:send"
	CapKVRead          = "kv:read"
	CapKVWrite         = "kv:write"
	CapSlotsWrite      = "slots:write"
)

var knownCapabilities = map[string]struct{}{
	CapPostsRead: {}, CapPostsWrite: {},
	CapCommentsRead: {}, CapCommentsWrite: {},
	CapUsersRead: {}, CapNotifySend: {},
	CapKVRead: {}, CapKVWrite: {}, CapSlotsWrite: {},
}

// KnownCapabilities 返回全部合法 capability（供 manifest 校验与后台展示）。
func KnownCapabilities() []string {
	out := make([]string, 0, len(knownCapabilities))
	for c := range knownCapabilities {
		out = append(out, c)
	}
	return out
}

// CapabilitySet 是某个插件已授予的能力集合。
type CapabilitySet map[string]struct{}

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

// KVStore 是插件专属键值空间（前缀由实现方拼接）。
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
type APIDeps struct {
	Posts         PostWriter
	Comments      CommentWriter
	Users         UserReader
	Notifications Notifier
	KV            KVStore
	Slots         SlotWriter
	Audit         AuditRecorder
}

// lookupAPI 把 method 解析成处理函数，并做 capability 校验。
//
// 校验放在查找阶段而不是 handler 内部：漏写校验的 handler 根本注册不进去。
func lookupAPI(method, pluginID string, caps CapabilitySet, deps APIDeps) (Handler, bool) {
	need := func(cap string) error {
		if _, ok := caps[cap]; !ok {
			return fmt.Errorf("插件 %s 未声明 capability %q", pluginID, cap)
		}
		return nil
	}

	switch method {
	case "api.posts.setStatus":
		return func(ctx context.Context, p json.RawMessage) (any, error) {
			if err := need(CapPostsWrite); err != nil {
				return nil, err
			}
			var in struct {
				PostID string `json:"post_id"`
				Status string `json:"status"`
			}
			if err := json.Unmarshal(p, &in); err != nil {
				return nil, err
			}
			if err := deps.Posts.SetStatus(ctx, in.PostID, in.Status); err != nil {
				return nil, err
			}
			_ = deps.Audit.RecordPluginAction(ctx, pluginID, "post.setStatus", in.PostID, in.Status)
			return map[string]bool{"ok": true}, nil
		}, true

	case "api.posts.addTag":
		return func(ctx context.Context, p json.RawMessage) (any, error) {
			if err := need(CapPostsWrite); err != nil {
				return nil, err
			}
			var in struct {
				PostID string `json:"post_id"`
				Tag    string `json:"tag"`
			}
			if err := json.Unmarshal(p, &in); err != nil {
				return nil, err
			}
			if err := deps.Posts.AddTag(ctx, in.PostID, in.Tag); err != nil {
				return nil, err
			}
			_ = deps.Audit.RecordPluginAction(ctx, pluginID, "post.addTag", in.PostID, in.Tag)
			return map[string]bool{"ok": true}, nil
		}, true

	case "api.posts.getLevel":
		return func(ctx context.Context, p json.RawMessage) (any, error) {
			if err := need(CapUsersRead); err != nil {
				return nil, err
			}
			var in struct {
				UserID string `json:"user_id"`
			}
			if err := json.Unmarshal(p, &in); err != nil {
				return nil, err
			}
			lv, err := deps.Users.GetLevel(ctx, in.UserID)
			if err != nil {
				return nil, err
			}
			return map[string]int{"level": lv}, nil
		}, true

	case "api.notifications.send":
		return func(ctx context.Context, p json.RawMessage) (any, error) {
			if err := need(CapNotifySend); err != nil {
				return nil, err
			}
			var in struct {
				UserID string `json:"user_id"`
				Title  string `json:"title"`
				Body   string `json:"body"`
			}
			if err := json.Unmarshal(p, &in); err != nil {
				return nil, err
			}
			if err := deps.Notifications.Send(ctx, in.UserID, in.Title, in.Body); err != nil {
				return nil, err
			}
			_ = deps.Audit.RecordPluginAction(ctx, pluginID, "notification.send", in.UserID, in.Title)
			return map[string]bool{"ok": true}, nil
		}, true

	case "api.kv.get":
		return func(ctx context.Context, p json.RawMessage) (any, error) {
			if err := need(CapKVRead); err != nil {
				return nil, err
			}
			var in struct {
				Key string `json:"key"`
			}
			if err := json.Unmarshal(p, &in); err != nil {
				return nil, err
			}
			v, err := deps.KV.Get(ctx, in.Key)
			if err != nil {
				return nil, err
			}
			return map[string]string{"value": v}, nil
		}, true

	case "api.kv.set":
		return func(ctx context.Context, p json.RawMessage) (any, error) {
			if err := need(CapKVWrite); err != nil {
				return nil, err
			}
			var in struct {
				Key   string `json:"key"`
				Value string `json:"value"`
			}
			if err := json.Unmarshal(p, &in); err != nil {
				return nil, err
			}
			if err := deps.KV.Set(ctx, in.Key, in.Value); err != nil {
				return nil, err
			}
			return map[string]bool{"ok": true}, nil
		}, true

	case "api.slots.set":
		return func(ctx context.Context, p json.RawMessage) (any, error) {
			if err := need(CapSlotsWrite); err != nil {
				return nil, err
			}
			var in struct {
				Slot  string         `json:"slot"`
				Kind  string         `json:"kind"`
				Props map[string]any `json:"props"`
				Sort  int            `json:"sort"`
			}
			if err := json.Unmarshal(p, &in); err != nil {
				return nil, err
			}
			if err := deps.Slots.Set(ctx, in.Slot, in.Kind, in.Props, in.Sort); err != nil {
				return nil, err
			}
			_ = deps.Audit.RecordPluginAction(ctx, pluginID, "slot.set", in.Slot, in.Kind)
			return map[string]bool{"ok": true}, nil
		}, true

	case "api.slots.delete":
		return func(ctx context.Context, p json.RawMessage) (any, error) {
			if err := need(CapSlotsWrite); err != nil {
				return nil, err
			}
			var in struct {
				Slot string `json:"slot"`
			}
			if err := json.Unmarshal(p, &in); err != nil {
				return nil, err
			}
			if err := deps.Slots.Delete(ctx, in.Slot); err != nil {
				return nil, err
			}
			_ = deps.Audit.RecordPluginAction(ctx, pluginID, "slot.delete", in.Slot, "")
			return map[string]bool{"ok": true}, nil
		}, true
	}
	return nil, false
}

func errUnknownMethod(method string) error {
	return fmt.Errorf("unknown method: %s", method)
}

// RegisterAPIs 把宿主 API 挂到插件连接上。
//
// 每个 method 都包一层：找不到对应 API 时返回明确错误，
// 而不是让插件在超时里空等。
func RegisterAPIs(conn *Conn, pluginID string, caps CapabilitySet, deps APIDeps) {
	for _, m := range apiMethodNames() {
		m := m
		conn.Handle(m, func(ctx context.Context, params json.RawMessage) (any, error) {
			fn, ok := lookupAPI(m, pluginID, caps, deps)
			if !ok {
				return nil, errUnknownMethod(m)
			}
			return fn(ctx, params)
		})
	}
}

// apiMethodNames 返回全部已实现的 API 方法名。
// 与 lookupAPI 的 case 列表保持一致（测试钉住）。
func apiMethodNames() []string {
	return []string{
		"api.posts.setStatus",
		"api.posts.addTag",
		"api.posts.getLevel",
		"api.notifications.send",
		"api.kv.get",
		"api.kv.set",
		"api.slots.set",
		"api.slots.delete",
	}
}
```

- [ ] **Step 4: 运行测试确认通过**

```bash
cd app/server && go test ./internal/plugin/ -run TestRegisterAPIs -race -v
```
Expected: PASS

- [ ] **Step 5: 提交**

```bash
cd /Users/noelorin/OrinForum
git add app/server/internal/plugin/
git commit -m "feat(plugin): host API surface with runtime capability checks"
```

---

（Task 7–11 见续篇：生命周期装配、安装与来源审批、插槽后端、插槽前端、CLI。）

---

## Task 7: 模型、生命周期装配与权限

**Files:**
- Create: `app/server/internal/model/plugin_install.go`
- Create: `app/server/internal/model/plugin_source.go`
- Modify: `app/server/internal/model/plugin.go`
- Modify: `app/server/internal/model/permission.go`
- Modify: `app/server/internal/plugin/host.go`（新建，聚合前面三个组件）
- Modify: `app/server/internal/container/container.go`、`runtime.go`
- Modify: `app/server/server/iris.go`、`server/wiring.go`
- Test: `app/server/internal/model/plugin_install_test.go`

**Interfaces:**
- Consumes: `Supervisor`（Task 4）、`Host.DispatchHook`（Task 5）、`RegisterAPIs`（Task 6）
- Produces: `model.PluginInstall`、`model.PluginStatus*` 常量、`container.Container.PluginHost`

**背景（重要）：** `plugin:manage` **已经存在**（`internal/model/permission.go:85`，已 seed，已在 `internal/gateway/permission.go:352` 登记前缀映射）。本任务**只需新增 `plugin:read`**，用于读接口不被写权限覆盖——这与 `customize:read` / `customize:write` 的既有模式一致。

- [ ] **Step 1: 写失败的测试**

创建 `internal/model/plugin_install_test.go`：

```go
package model_test

import (
	"testing"

	"feedchamber/internal/model"

	"github.com/stretchr/testify/require"
)

func TestPluginStatusValid(t *testing.T) {
	for _, s := range []string{
		model.PluginStatusInstalled,
		model.PluginStatusStarting,
		model.PluginStatusRunning,
		model.PluginStatusCrashed,
		model.PluginStatusStopped,
	} {
		require.True(t, model.ValidPluginStatus(s), s)
	}
	require.False(t, model.ValidPluginStatus("nonsense"))
}

// 重启策略依赖：从 running 恢复，crashed 不自动拉起。
func TestPluginStatusShouldResume(t *testing.T) {
	require.True(t, model.PluginStatusShouldResume(model.PluginStatusRunning))
	require.False(t, model.PluginStatusShouldResume(model.PluginStatusStopped))
	require.False(t, model.PluginStatusShouldResume(model.PluginStatusCrashed))
	require.False(t, model.PluginStatusShouldResume(model.PluginStatusInstalled))
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
cd app/server && go test ./internal/model/ -run 'TestPluginStatus' -v
```
Expected: FAIL — `undefined: model.PluginStatusInstalled`

- [ ] **Step 3: 实现模型**

创建 `internal/model/plugin_install.go`：

```go
package model

import "time"

// 插件状态机。
//
//   installed → starting → running
//                    ↓        ↓
//                 crashed ← (崩溃/超限)
//                    ↓
//                 stopped
//
// 宿主重启只恢复 running 的插件；crashed 与 stopped 都不自动拉起。
const (
	PluginStatusInstalled = "installed"
	PluginStatusStarting  = "starting"
	PluginStatusRunning   = "running"
	PluginStatusCrashed   = "crashed"
	PluginStatusStopped   = "stopped"
)

var pluginStatuses = []string{
	PluginStatusInstalled, PluginStatusStarting, PluginStatusRunning,
	PluginStatusCrashed, PluginStatusStopped,
}

func ValidPluginStatus(s string) bool {
	for _, v := range pluginStatuses {
		if v == s {
			return true
		}
	}
	return false
}

// PluginStatusShouldResume 决定宿主重启后是否拉起该插件。
func PluginStatusShouldResume(s string) bool {
	return s == PluginStatusRunning
}

// PluginInstall 是一次插件安装的运行时记录。
//
// 与 model.Plugin 的分工：Plugin 是「登记了什么」（清单、能力、来源），
// PluginInstall 是「这一份装在哪个目录、当前什么状态」。
// 分开是因为将来可能同一条 Plugin 多实例（多站点托管），
// 而 v1 只是一对一。
type PluginInstall struct {
	ID       string `gorm:"primaryKey;size:36" json:"id"`
	PluginID string `gorm:"size:64;not null;uniqueIndex" json:"plugin_id"`
	SourceID string `gorm:"size:36;index" json:"source_id"`
	// Dir 是插件代码在磁盘上的绝对路径。
	Dir    string `gorm:"size:512;not null" json:"dir"`
	Entry  string `gorm:"size:128;not null;default:'index.js'" json:"entry"`
	Status string `gorm:"size:16;not null;default:'installed';index" json:"status"`
	// Caps 是已授予的 capability（JSON 数组）。以授予时为准，
	// 之后 manifest 变化不自动扩权——改权需重新审批。
	Caps            string    `gorm:"type:text" json:"caps"`
	Restarts        int       `gorm:"not null;default:0" json:"restarts"`
	LastError       string    `gorm:"type:text" json:"last_error"`
	LastStartedAt   *time.Time `json:"last_started_at,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func (PluginInstall) TableName() string { return "plugin_installs" }
```

创建 `internal/model/plugin_source.go`：

```go
package model

import "time"

// 插件来源审批状态。
const (
	PluginSourcePending  = "pending"
	PluginSourceApproved = "approved"
	PluginSourceRejected = "rejected"
)

// 插件来源类型。v1 只支持从受信任代码仓库拉取。
const (
	PluginSourceGit = "git"
)

// PluginSource 是一条插件来源登记。
//
// 「来源审批」是信任模型的第一层：未审批的来源不能安装。
// 注意它挡的是「谁把代码放进来」，不是「代码是否安全」——
// 同用户子进程的边界问题见 proposal 的风险节。
type PluginSource struct {
	ID     string `gorm:"primaryKey;size:36" json:"id"`
	Name   string `gorm:"size:128;not null" json:"name"`
	Kind   string `gorm:"size:16;not null;default:'git'" json:"kind"`
	URL    string `gorm:"size:512;not null" json:"url"`
	Status string `gorm:"size:16;not null;default:'pending';index" json:"status"`
	// ApprovedBy 记录审批人；同一个人新增+审批时是同一个 id，
	// 这一列存在的意义是让审计能查出「自批」的情形。
	ApprovedBy string    `gorm:"size:36" json:"approved_by"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (PluginSource) TableName() string { return "plugin_sources" }

func ValidPluginSourceStatus(s string) bool {
	return s == PluginSourcePending || s == PluginSourceApproved || s == PluginSourceRejected
}
```

修改 `internal/model/plugin.go`，把结构体扩为：

```go
type Plugin struct {
	ID          string `gorm:"primaryKey;size:64" json:"id"`
	Name        string `gorm:"size:128;not null" json:"name"`
	Version     string `gorm:"size:32" json:"version"`
	Author      string `gorm:"size:128" json:"author"`
	Description string `gorm:"type:text" json:"description"`
	Homepage    string `gorm:"size:512" json:"homepage"`
	Settings    string `gorm:"type:text" json:"settings"`
	Enabled     bool   `gorm:"default:true" json:"enabled"`

	// v2：插件从「声明式元数据」变为可执行的子进程插件。
	// Manifest 存 plugin.json 原文，改动留痕可查。
	Manifest        string `gorm:"type:text" json:"manifest,omitempty"`
	ProtocolVersion string `gorm:"size:8" json:"protocol_version,omitempty"`
	Hooks           string `gorm:"type:text" json:"hooks,omitempty"`        // JSON 数组
	Capabilities    string `gorm:"type:text" json:"capabilities,omitempty"` // JSON 数组
	SourceID        string `gorm:"size:36;index" json:"source_id,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
```

修改 `internal/model/permission.go`：
- 在 `PermPluginManage = "plugin:manage"` 之后加一行 `PermPluginRead = "plugin:read"`
- 在权限定义表（`PermPluginManage` 那条之后）加：

```go
	{Code: PermPluginRead, Name: "查看插件", Description: "查看插件清单、运行状态与调用日志"},
```

- 在 admin 角色的权限列表里，`PermPluginManage` 之后加 `PermPluginRead`

- [ ] **Step 4: 运行模型测试确认通过**

```bash
cd app/server && go test ./internal/model/ -run 'TestPluginStatus' -v
```
Expected: PASS

- [ ] **Step 5: 网关映射**

在 `internal/gateway/permission.go` 的 `initRouteMap` 里，`p.addPrefix("/api/v1/admin/plugins/", "plugin:manage")` 之前插入读接口精确映射：

```go
	// 读接口必须是 plugin:read，不能被写前缀覆盖——
	// 否则「只想看看装了哪些插件」的 Key 会拿到安装/卸载能力。
	p.addExact("/api/v1/admin/plugins/list", "plugin:read")
	p.addExact("/api/v1/admin/plugins/{id}/status", "plugin:read")
```

（若 `addExact` 不存在，用该文件已有的精确映射方法，名字以源码为准。）

- [ ] **Step 6: 装配到 container**

创建 `internal/plugin/host.go`：

```go
package plugin

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"

	"github.com/sirupsen/logrus"
)

// Host 是插件子系统对外的唯一入口：管理生命周期 + 分发钩子。
//
// 它刻意不持有 repository：插件的持久化由调用方（container）在
// 外面做完，再通过 Options 注入回调。这样本包不依赖 internal/repository。
type Host struct {
	*dispatcher

	mu    sync.RWMutex
	supers map[string]*Supervisor
	opts   HostOptions
}

type HostOptions struct {
	// PluginRoot 是插件代码根目录（如 ./plugins）。
	PluginRoot string
	// NodePath 是 node 可执行文件路径，默认取 PATH 里的 node。
	NodePath string
	// OnStatus 在状态变化时回调，供调用方落库。
	OnStatus func(pluginID, status, lastError string)
	// APIDepsFor 为某个插件构造宿主 API 依赖（含 capability 集合）。
	APIDepsFor func(pluginID string, caps CapabilitySet) APIDeps
}

func NewHostWithOptions(opts HostOptions) *Host {
	return &Host{
		dispatcher: newDispatcher(),
		supers:     make(map[string]*Supervisor),
		opts:       opts,
	}
}

// StartPlugin 拉起一个插件并注册其钩子。
func (h *Host) StartPlugin(ctx context.Context, pluginID string, m *Manifest, caps CapabilitySet) error {
	h.mu.Lock()
	if _, exists := h.supers[pluginID]; exists {
		h.mu.Unlock()
		return fmt.Errorf("插件 %s 已在运行", pluginID)
	}
	sup := NewSupervisor(SupervisorOptions{
		NodePath:    h.opts.NodePath,
		PluginDir:   filepath.Join(h.opts.PluginRoot, pluginID),
		Entry:       m.Entry,
		MaxRestarts: 5,
		OnCrash: func(reason string) {
			if h.opts.OnStatus != nil {
				h.opts.OnStatus(pluginID, "crashed", reason)
			}
		},
	})
	h.supers[pluginID] = sup
	h.mu.Unlock()

	conn, err := sup.Start(ctx)
	if err != nil {
		h.mu.Lock()
		delete(h.supers, pluginID)
		h.mu.Unlock()
		return err
	}

	if h.opts.APIDepsFor != nil {
		RegisterAPIs(conn, pluginID, caps, h.opts.APIDepsFor(pluginID, caps))
	}
	h.RegisterPlugin(pluginID, m, conn)
	if h.opts.OnStatus != nil {
		h.opts.OnStatus(pluginID, "running", "")
	}
	logrus.Infof("plugin: %s 已启动（钩子 %v）", pluginID, m.Hooks)
	return nil
}

// StopPlugin 停止插件并摘除钩子。
func (h *Host) StopPlugin(pluginID string) error {
	h.mu.Lock()
	sup := h.supers[pluginID]
	delete(h.supers, pluginID)
	h.mu.Unlock()

	h.UnregisterPlugin(pluginID)
	if sup != nil {
		if err := sup.Stop(); err != nil {
			return err
		}
	}
	if h.opts.OnStatus != nil {
		h.opts.OnStatus(pluginID, "stopped", "")
	}
	return nil
}

// StopAll 在宿主关停时调用。
func (h *Host) StopAll() {
	h.mu.Lock()
	ids := make([]string, 0, len(h.supers))
	for id := range h.supers {
		ids = append(ids, id)
	}
	h.mu.Unlock()
	for _, id := range ids {
		_ = h.StopPlugin(id)
	}
}
```

同时把 Task 5 的 `Host` 重命名为 `dispatcher`（`type dispatcher struct`，构造函数 `newDispatcher()`），方法不变。这是为了避免两个 `Host` 冲突——Task 5 那层实际是「分发器」，Task 7 这层才是对外的 `Host`。

- [ ] **Step 7: 接入 container 与 runtime**

在 `internal/container/container.go` 的 `Container` 结构体中加字段：

```go
	// PluginHost 托管 Node 子进程插件。Start 之前为空。
	PluginHost *plugin.Host
```

在 `New` 末尾构造（此时不拉起任何进程）：

```go
	c.PluginHost = plugin.NewHostWithOptions(plugin.HostOptions{
		PluginRoot: "./plugins",
		OnStatus: func(pluginID, status, lastError string) {
			// 异步落库：状态回调在 supervisor 的 goroutine 里，
			// 不能因为一次 DB 抖动阻塞重启流程。
			go func() { _ = c.PluginSvc.SetRuntimeStatus(context.Background(), pluginID, status, lastError) }()
		},
	})
```

在 `runtime.go` 的 `Start` 里恢复插件（放在 worker 启动之后）：

```go
	// 恢复重启前处于 running 的插件。crashed / stopped 不自动拉起。
	if installs, err := c.PluginSvc.ListResumable(ctx); err == nil {
		for _, in := range installs {
			m, mErr := plugin.ParseManifest([]byte(in.Manifest))
			if mErr != nil {
				logrus.Errorf("plugin: %s manifest 解析失败，跳过恢复: %v", in.PluginID, mErr)
				continue
			}
			caps := plugin.CapabilitySet{}
			for _, cap := range in.Capabilities {
				caps[cap] = struct{}{}
			}
			if err := c.PluginHost.StartPlugin(ctx, in.PluginID, m, caps); err != nil {
				logrus.Errorf("plugin: 恢复 %s 失败: %v", in.PluginID, err)
			}
		}
	}
```

在 `Shutdown` 的最前面加：

```go
	r.c.PluginHost.StopAll()
```

在 `server/iris.go` 的 AutoMigrate 列表加：

```go
		&model.Plugin{},
		&model.PluginInstall{},
		&model.PluginSource{},
```

- [ ] **Step 8: 构建与测试**

```bash
cd app/server && go build ./... && go test ./internal/plugin/ ./internal/model/ ./internal/container/ -race
```
Expected: 全部 PASS

- [ ] **Step 9: 提交**

```bash
cd /Users/noelorin/OrinForum
git add app/server/internal/model/ app/server/internal/plugin/ app/server/internal/container/ app/server/internal/gateway/ app/server/server/
git commit -m "feat(plugin): wire subprocess host into container lifecycle"
```

---

## Task 8: 插件安装与来源审批

**Files:**
- Modify: `app/server/internal/service/plugin.go`
- Modify: `app/server/internal/service/plugin_iface.go`
- Modify: `app/server/internal/handler/plugin.go`
- Modify: `app/server/internal/router/routes_admin.go`
- Test: `app/server/internal/service/plugin_test.go`

**Interfaces:**
- Consumes: `model.PluginSource`、`model.PluginInstall`、`plugin.ParseManifest`
- Produces: `PluginService.CreateSource` / `ApproveSource` / `Install` / `SetRuntimeStatus` / `ListResumable`

- [ ] **Step 1: 写失败的测试**

创建 `internal/service/plugin_test.go`（若已存在则追加）：

```go
package service_test

import (
	"context"
	"testing"

	"feedchamber/internal/config"
	"feedchamber/internal/db"
	"feedchamber/internal/repository"
	"feedchamber/internal/service"

	"github.com/stretchr/testify/require"
)

func newPluginTestSvc(t *testing.T) service.PluginService {
	t.Helper()
	store, err := db.New(config.DBConfig{Type: "sqlite", DSN: ":memory:"})
	require.NoError(t, err)
	require.NoError(t, store.AutoMigrate(
		&modelPluginSource{}, // 见下方说明：用真实模型
	))
	return service.NewPluginService(
		repository.NewPluginRepository(store),
		repository.NewPluginSourceRepository(store),
		repository.NewPluginInstallRepository(store),
		nil, // audit
	)
}

// 未审批来源不可安装。
func TestInstallRejectsUnapprovedSource(t *testing.T) {
	svc := newPluginTestSvc(t)
	ctx := context.Background()

	src, err := svc.CreateSource(ctx, "test", "git", "https://example.com/x.git", "admin")
	require.NoError(t, err)
	require.Equal(t, "pending", src.Status)

	_, err = svc.Install(ctx, service.PluginInstallInput{
		PluginID: "hello", SourceID: src.ID, Manifest: validManifestJSON,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "审批")
}

// 审批后可以安装。
func TestInstallAcceptsApprovedSource(t *testing.T) {
	svc := newPluginTestSvc(t)
	ctx := context.Background()

	src, err := svc.CreateSource(ctx, "test", "git", "https://example.com/x.git", "admin")
	require.NoError(t, err)
	require.NoError(t, svc.ApproveSource(ctx, src.ID, "admin2"))

	inst, err := svc.Install(ctx, service.PluginInstallInput{
		PluginID: "hello", SourceID: src.ID, Manifest: validManifestJSON,
	})
	require.NoError(t, err)
	require.Equal(t, "installed", inst.Status)
}

const validManifestJSON = `{
	"name":"hello","version":"1.0.0","protocol_version":"1",
	"hooks":["post.created"],"capabilities":["posts:write"]
}`
```

（测试文件里用真实 `model.PluginSource` 替换 `modelPluginSource` 占位，导入 `feedchamber/internal/model`。）

- [ ] **Step 2: 运行测试确认失败**

```bash
cd app/server && go test ./internal/service/ -run TestInstall -v
```
Expected: FAIL — `CreateSource` 未定义

- [ ] **Step 3: 实现 service 与仓储**

新增 `internal/repository/plugin_source.go`、`internal/repository/plugin_install.go`（照 `NavItemRepository` 的形态：接口 + `NewXxxRepository(store)` + SQL 实现）。

`internal/service/plugin_iface.go` 扩展：

```go
type PluginService interface {
	// 既有声明式 CRUD（保留）
	List(ctx context.Context) ([]model.Plugin, error)
	Update(ctx context.Context, input PluginInput) error
	SetEnabled(ctx context.Context, id string, enabled bool) error
	Delete(ctx context.Context, id string) error

	// 来源与审批
	CreateSource(ctx context.Context, name, kind, url, actorID string) (*model.PluginSource, error)
	ListSources(ctx context.Context) ([]model.PluginSource, error)
	ApproveSource(ctx context.Context, id, actorID string) error
	RejectSource(ctx context.Context, id, actorID string) error

	// 安装与运行时
	Install(ctx context.Context, in PluginInstallInput) (*model.PluginInstall, error)
	Uninstall(ctx context.Context, pluginID string) error
	SetRuntimeStatus(ctx context.Context, pluginID, status, lastError string) error
	ListResumable(ctx context.Context) ([]ResumablePlugin, error)
}

type PluginInstallInput struct {
	PluginID string
	SourceID string
	Manifest string // plugin.json 原文
}

// ResumablePlugin 是宿主重启时需要恢复的插件。
type ResumablePlugin struct {
	PluginID     string
	Manifest     string
	Capabilities []string
	Dir          string
}
```

`Install` 的关键校验顺序（写进实现）：

```go
func (s *pluginService) Install(ctx context.Context, in PluginInstallInput) (*model.PluginInstall, error) {
	m, err := plugin.ParseManifest([]byte(in.Manifest))
	if err != nil {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, err.Error())
	}
	if err := m.Validate(plugin.CapabilitySetFromNames(plugin.KnownCapabilities())); err != nil {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, err.Error())
	}

	src, err := s.sources.Get(ctx, in.SourceID)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "来源不存在")
	}
	// 未审批来源一律拒绝——这是信任模型第一层。
	if src.Status != model.PluginSourceApproved {
		return nil, pkg.NewAppError(pkg.FORBIDDEN, "插件来源尚未审批，无法安装")
	}
	// ……写 Plugin + PluginInstall，落审计
}
```

`internal/router/routes_admin.go` 的 `/plugins` Party 内补路由：

```go
			// 读接口挂 plugin:read，写接口挂 plugin:manage。
			p.Post("/list", middleware.RequirePermission(deps, model.PermPluginRead), h.Plugin.List)
			p.Post("/sources/list", middleware.RequirePermission(deps, model.PermPluginRead), h.Plugin.ListSources)
			p.Post("/install", middleware.RequirePermission(deps, model.PermPluginManage), h.Plugin.Install)
			p.Post("/sources/create", middleware.RequirePermission(deps, model.PermPluginManage), h.Plugin.CreateSource)
			p.Post("/sources/{id}/approve", middleware.RequirePermission(deps, model.PermPluginManage), h.Plugin.ApproveSource)
			p.Post("/sources/{id}/reject", middleware.RequirePermission(deps, model.PermPluginManage), h.Plugin.RejectSource)
			p.Post("/{id}/update", middleware.RequirePermission(deps, model.PermPluginManage), h.Plugin.Update)
			p.Post("/{id}/status", middleware.RequirePermission(deps, model.PermPluginRead), h.Plugin.SetEnabled)
			p.Post("/{id}/delete", middleware.RequirePermission(deps, model.PermPluginManage), h.Plugin.Delete)
```

- [ ] **Step 4: 运行测试确认通过**

```bash
cd app/server && go test ./internal/service/ -run TestInstall -race -v
```
Expected: PASS

- [ ] **Step 5: 钉住路由权限映射**

在 `internal/router/middleware_chain_test.go` 里，把 `"/api/v1/admin/plugins/"` 加进扫描列表，并显式断言读接口映射到 `plugin:read`：

```go
	for _, listPath := range []string{
		"/api/v1/admin/customization/snippets/list",
		// ……既有条目
		"/api/v1/admin/plugins/list",
		"/api/v1/admin/plugins/sources/list",
	} {
		// 断言读接口不被写权限码覆盖
	}
```

```bash
cd app/server && go test ./internal/router/ -run TestAdmin -v
```
Expected: PASS

- [ ] **Step 6: 提交**

```bash
cd /Users/noelorin/OrinForum
git add app/server/internal/service/ app/server/internal/repository/ app/server/internal/handler/ app/server/internal/router/
git commit -m "feat(plugin): source approval and install flow"
```

---

## Task 9: 壳层插槽后端

**Files:**
- Create: `app/server/internal/model/slot_entry.go`
- Create: `app/server/internal/repository/slot_entry.go`
- Create: `app/server/internal/service/slot.go`、`slot_iface.go`
- Create: `app/server/internal/handler/slot.go`
- Modify: `app/server/internal/router/routes_admin.go`、`routes_public.go`
- Test: `app/server/internal/service/slot_test.go`

**Interfaces:**
- Produces: `model.SlotEntry`、`model.Slot*` 常量、`service.SlotService`

- [ ] **Step 1: 写失败的测试**

创建 `internal/service/slot_test.go`：

```go
package service_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// 插槽名必须是枚举内的；未知插槽名的写入被拒绝。
func TestCreateSlotEntryRejectsUnknownSlot(t *testing.T) {
	svc := newSlotTestSvc(t)
	_, err := svc.Create(context.Background(), service.SlotEntryInput{
		Slot: "not.a.slot", Kind: model.SlotKindText,
		Props: map[string]any{"text": "hi"},
	})
	require.Error(t, err)
}

// 裸 HTML 必须被拒绝。
func TestCreateSlotEntryRejectsHTMLKind(t *testing.T) {
	svc := newSlotTestSvc(t)
	_, err := svc.Create(context.Background(), service.SlotEntryInput{
		Slot: model.SlotSidebarTop, Kind: "html",
		Props: map[string]any{"html": "<script>alert(1)</script>"},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "html")
}

// 可见性过滤：TL1 用户看不到 TL2 门槛的条目。
func TestListForViewerFiltersByLevel(t *testing.T) {
	svc := newSlotTestSvc(t)
	ctx := context.Background()
	_, err := svc.Create(ctx, service.SlotEntryInput{
		Slot: model.SlotSidebarTop, Kind: model.SlotKindText,
		Props: map[string]any{"text": "vip"},
		Visibility: model.NavVisibilityLevel, MinLevel: 2,
	})
	require.NoError(t, err)

	guest, err := svc.ListForViewer(ctx, false, 0)
	require.NoError(t, err)
	require.Empty(t, guest)

	tl2, err := svc.ListForViewer(ctx, true, 2)
	require.NoError(t, err)
	require.Len(t, tl2, 1)
}

// 排序按 sort 升序。
func TestListForViewerOrdersBySort(t *testing.T) {
	svc := newSlotTestSvc(t)
	ctx := context.Background()
	for _, pair := range []struct {
		text string
		sort int
	}{{"b", 20}, {"a", 10}} {
		_, err := svc.Create(ctx, service.SlotEntryInput{
			Slot: model.SlotSidebarTop, Kind: model.SlotKindText,
			Props: map[string]any{"text": pair.text}, Sort: pair.sort,
		})
		require.NoError(t, err)
	}
	got, err := svc.ListForViewer(ctx, true, 0)
	require.NoError(t, err)
	require.Len(t, got, 2)
	require.Equal(t, 10, got[0].Sort)
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
cd app/server && go test ./internal/service/ -run 'TestCreateSlotEntry|TestListForViewer' -v
```
Expected: FAIL — `model.SlotSidebarTop` 未定义

- [ ] **Step 3: 实现模型**

创建 `internal/model/slot_entry.go`：

```go
package model

import "time"

// 壳层插槽枚举。
//
// 固定集合的原因：前端只认这些名字，未知名字被忽略。
// 让插槽名自由化等于让前端无法预渲染，也失去了「哪里能插」的契约。
const (
	SlotSidebarTop          = "sidebar.top"
	SlotSidebarNavAfter     = "sidebar.nav.after"
	SlotSidebarSpacesBefore = "sidebar.spaces.before"
	SlotSidebarSpacesAfter  = "sidebar.spaces.after"
	SlotSidebarTagsAfter    = "sidebar.tags.after"
	SlotSidebarBottom       = "sidebar.bottom"
	SlotHeaderLeft          = "header.left"
	SlotHeaderRightBefore   = "header.right.before"
	SlotHeaderRightAfter    = "header.right.after"
	SlotFooterLeft          = "footer.left"
	SlotFooterRight         = "footer.right"
)

// AllSlots 是全部合法插槽名。
var AllSlots = []string{
	SlotSidebarTop, SlotSidebarNavAfter, SlotSidebarSpacesBefore,
	SlotSidebarSpacesAfter, SlotSidebarTagsAfter, SlotSidebarBottom,
	SlotHeaderLeft, SlotHeaderRightBefore, SlotHeaderRightAfter,
	SlotFooterLeft, SlotFooterRight,
}

func ValidSlot(s string) bool {
	for _, v := range AllSlots {
		if v == s {
			return true
		}
	}
	return false
}

// 条目类型。刻意不含 html：插槽是同源位置，
// 允许任意 HTML 等于允许全站 XSS（见 shell-slots 规范）。
const (
	SlotKindText    = "text"
	SlotKindLink    = "link"
	SlotKindImage   = "image"
	SlotKindDivider = "divider"
	SlotKindWidget  = "widget"
)

var SlotKinds = []string{SlotKindText, SlotKindLink, SlotKindImage, SlotKindDivider, SlotKindWidget}

func ValidSlotKind(k string) bool {
	for _, v := range SlotKinds {
		if v == k {
			return true
		}
	}
	return false
}

// SlotEntry 是插槽里的一条内容。
type SlotEntry struct {
	ID    string `gorm:"primaryKey;size:36" json:"id"`
	Slot  string `gorm:"size:32;not null;index" json:"slot"`
	Kind  string `gorm:"size:16;not null" json:"kind"`
	// Props 是 JSON 对象，形状由 Kind 决定。
	// 不用多列存是因为五种类型的字段互不重叠，多列会留下大量 NULL。
	Props string `gorm:"type:text" json:"props"`
	Sort  int    `gorm:"not null;default:0;index" json:"sort"`

	Visibility string `gorm:"size:16;not null;default:'public'" json:"visibility"`
	MinLevel   int    `gorm:"not null;default:0" json:"min_level"`

	Enabled bool `gorm:"not null;default:true;index" json:"enabled"`
	// OwnerPluginID 非空表示该条目由插件写入，用于卸载时清理与审计归属。
	OwnerPluginID string    `gorm:"size:64;index" json:"owner_plugin_id,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (SlotEntry) TableName() string { return "slot_entries" }
```

- [ ] **Step 4: 实现 service 与 handler**

`internal/service/slot_iface.go`：

```go
type SlotService interface {
	Create(ctx context.Context, in SlotEntryInput) (*model.SlotEntry, error)
	Update(ctx context.Context, id string, in SlotEntryInput) (*model.SlotEntry, error)
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, slot string) ([]model.SlotEntry, error)
	// ListForViewer 返回某访问者可见的启用条目，已按 sort 升序。
	ListForViewer(ctx context.Context, isLogin bool, level int) ([]model.SlotEntry, error)
	// SetForPlugin 供插件经宿主 API 写入；OwnerPluginID 由调用方给定。
	SetForPlugin(ctx context.Context, pluginID, slot, kind string, props map[string]any, sort int) error
	DeleteForPlugin(ctx context.Context, pluginID, slot string) error
}

type SlotEntryInput struct {
	Slot       string
	Kind       string
	Props      map[string]any
	Sort       int
	Visibility string
	MinLevel   int
	Enabled    bool
}
```

实现的关键校验（照 `customize` 的形态写，含审计）：

```go
func (s *slotService) Create(ctx context.Context, in SlotEntryInput) (*model.SlotEntry, error) {
	if !model.ValidSlot(in.Slot) {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "未知插槽: "+in.Slot)
	}
	if !model.ValidSlotKind(in.Kind) {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "不支持的条目类型: "+in.Kind)
	}
	// ……落库 + 审计
}
```

`internal/handler/slot.go` 提供 `List/Create/Update/Delete` 与公开的 `Public(ctx)`。

路由：
- `routes_admin.go` 新增 `a.PartyFunc("/slots", ...)`，权限码 `customize:read`（读）/ `customize:write`（写）——插槽属于站点自定义层，复用既有权限码，不新造。
- `routes_public.go` 新增 `v1.Post("/slots/public", h.Slot.Public)`。
- `internal/gateway/permission.go` 补 `p.addPrefix("/api/v1/admin/slots/", "customize:write")` 与读接口精确映射。

- [ ] **Step 5: 运行测试确认通过**

```bash
cd app/server && go test ./internal/service/ -run 'TestCreateSlotEntry|TestListForViewer' -race -v && go build ./...
```
Expected: PASS

- [ ] **Step 6: 提交**

```bash
cd /Users/noelorin/OrinForum
git add app/server/internal/model/slot_entry.go app/server/internal/repository/ app/server/internal/service/ app/server/internal/handler/slot.go app/server/internal/router/ app/server/internal/gateway/
git commit -m "feat(slots): shell slot entries with visibility and plugin ownership"
```

---

## Task 10: 壳层插槽前端

**Files:**
- Create: `app/web/components/slot/slot-host.tsx`
- Create: `app/web/components/slot/renderers/index.tsx`
- Create: `app/web/components/slot/renderers/list.tsx`
- Create: `app/web/components/slot/renderers/stats.tsx`
- Create: `app/web/components/slot/renderers/markdown.tsx`
- Create: `packages/api-client/src/services/slot.ts`
- Modify: `app/web/components/aside/sidebar-content.tsx`、`components/layout/nav.tsx`、`components/layout/footer.tsx`
- Modify: `packages/api-client/src/index.ts`、`src/constants.ts`
- Test: `app/web/components/slot/slot-host.test.tsx`

**Interfaces:**
- Consumes: `POST /api/v1/slots/public`（Task 9）
- Produces: `<SlotHost name="sidebar.top" />`

- [ ] **Step 1: 写失败的测试**

创建 `app/web/components/slot/slot-host.test.tsx`：

```tsx
import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';

import { SlotHost } from './slot-host';

const entries = [
	{ id: '1', slot: 'sidebar.top', kind: 'text', props: { text: 'hello' }, sort: 10 },
	{ id: '2', slot: 'sidebar.top', kind: 'divider', props: {}, sort: 20 },
];

describe('SlotHost', () => {
	it('按 sort 升序渲染', () => {
		render(<SlotHost name="sidebar.top" entries={entries} />);
		const items = screen.getAllByTestId('slot-entry');
		expect(items).toHaveLength(2);
	});

	it('未知插槽名不渲染任何内容', () => {
		const { container } = render(<SlotHost name="not.a.slot" entries={entries} />);
		expect(container).toBeEmptyDOMElement();
	});

	it('空条目不渲染容器', () => {
		const { container } = render(<SlotHost name="sidebar.top" entries={[]} />);
		expect(container).toBeEmptyDOMElement();
	});

	it('未知 kind 被跳过且不影响其他条目', () => {
		render(
			<SlotHost
				name="sidebar.top"
				entries={[
					{ id: '1', slot: 'sidebar.top', kind: 'unknown-kind', props: {}, sort: 1 },
					{ id: '2', slot: 'sidebar.top', kind: 'text', props: { text: 'ok' }, sort: 2 },
				]}
			/>,
		);
		expect(screen.getAllByTestId('slot-entry')).toHaveLength(1);
	});
});
```

- [ ] **Step 2: 运行测试确认失败**

```bash
cd app/web && pnpm vitest run components/slot/slot-host.test.tsx
```
Expected: FAIL — 模块不存在

- [ ] **Step 3: 实现组件**

`app/web/components/slot/renderers/list.tsx`：

```tsx
import type { SlotEntry } from '@feedchamber/api-client';

export function ListRenderer({ entry }: { entry: SlotEntry }) {
	const items = (entry.props?.items as string[]) ?? [];
	return (
		<ul className="flex flex-col gap-1 px-2 text-[13px] text-muted-foreground">
			{items.map((it) => (
				<li key={it}>{it}</li>
			))}
		</ul>
	);
}
```

`stats.tsx`：

```tsx
import type { SlotEntry } from '@feedchamber/api-client';

export function StatsRenderer({ entry }: { entry: SlotEntry }) {
	const value = String(entry.props?.value ?? '');
	const label = String(entry.props?.label ?? '');
	return (
		<div className="flex items-baseline gap-1.5 px-2">
			<span className="text-lg font-semibold tabular-nums text-foreground">{value}</span>
			<span className="text-xs text-muted-foreground">{label}</span>
		</div>
	);
}
```

`markdown.tsx`（复用站点既有的 markdown 渲染组件，路径以 `app/web/components/markdown-editor/` 为准）：

```tsx
import type { SlotEntry } from '@feedchamber/api-client';

import { MarkdownView } from '~/components/markdown-editor/markdown-view';

export function MarkdownRenderer({ entry }: { entry: SlotEntry }) {
	return (
		<div className="px-2 text-[13px] text-muted-foreground">
			<MarkdownView source={String(entry.props?.markdown ?? '')} />
		</div>
	);
}
```

`renderers/index.tsx` 注册表：

```tsx
import type { SlotEntry } from '@feedchamber/api-client';

import { ListRenderer } from './list';
import { MarkdownRenderer } from './markdown';
import { StatsRenderer } from './stats';

/**
 * 渲染器注册表。键是 Props 声明的内容类型（entry.props.renderer），
 * 不是 Kind——Kind=widget 时由 props.renderer 决定用哪个渲染器。
 */
export const slotRenderers: Record<string, (p: { entry: SlotEntry }) => React.ReactNode> = {
	list: ListRenderer,
	stats: StatsRenderer,
	markdown: MarkdownRenderer,
};

/** 降级渲染器：未注册的类型退回文本展示，而不是整块消失。 */
export function fallbackRenderer({ entry }: { entry: SlotEntry }) {
	return <span className="px-2 text-[13px] text-muted-foreground">{String(entry.props?.text ?? '')}</span>;
}
```

`slot-host.tsx`：

```tsx
'use client';

import type { SlotEntry } from '@feedchamber/api-client';

import { fallbackRenderer, slotRenderers } from './renderers';

const SLOTS = new Set([
	'sidebar.top', 'sidebar.nav.after', 'sidebar.spaces.before', 'sidebar.spaces.after',
	'sidebar.tags.after', 'sidebar.bottom', 'header.left', 'header.right.before',
	'header.right.after', 'footer.left', 'footer.right',
]);

export function SlotHost({ name, entries }: { name: string; entries: SlotEntry[] }) {
	if (!SLOTS.has(name)) return null;
	const mine = entries.filter((e) => e.slot === name).sort((a, b) => a.sort - b.sort);
	if (mine.length === 0) return null;

	return (
		<div className="flex flex-col gap-1">
			{mine.map((e) => {
				if (e.kind === 'divider') {
					return <hr key={e.id} data-testid="slot-entry" className="mx-2 border-border" />;
				}
				if (e.kind === 'text') {
					return (
						<span key={e.id} data-testid="slot-entry" className="px-2 text-[13px] text-muted-foreground">
							{String(e.props?.text ?? '')}
						</span>
					);
				}
				if (e.kind === 'link') {
					return (
						<a key={e.id} data-testid="slot-entry" href={String(e.props?.href ?? '#')}
							className="px-2 text-[13px] text-muted-foreground hover:text-foreground">
							{String(e.props?.label ?? '')}
						</a>
					);
				}
				if (e.kind === 'widget') {
					const key = String(e.props?.renderer ?? 'list');
					const Renderer = slotRenderers[key] ?? fallbackRenderer;
					return (
						<div key={e.id} data-testid="slot-entry">
							<Renderer entry={e} />
						</div>
					);
				}
				return null; // 未知 kind 跳过，不影响其他条目
			})}
		</div>
	);
}
```

- [ ] **Step 4: 运行测试确认通过**

```bash
cd app/web && pnpm vitest run components/slot/slot-host.test.tsx
```
Expected: PASS

- [ ] **Step 5: 接入三个壳层组件**

`components/aside/sidebar-content.tsx`：在既有位置插入六个 `<SlotHost>`（`sidebar.top` 在 `BackToPrev` 之前、`sidebar.nav.after` 在 `<CustomNav />` 之后、`sidebar.spaces.before/after` 在版面面板两侧、`sidebar.tags.after` 在标签面板之后、`sidebar.bottom` 在最末）。

`components/layout/nav.tsx`：`header.left` 在 `<SiteName>` 之后、`header.right.before` 在搜索按钮之前、`header.right.after` 在 `<UserInfo />` 之后。

`components/layout/footer.tsx`：`footer.left` 与 `footer.right` 各插一个。

三处都从父层接收 `entries`（由 RSC 侧或同一 query 提供），保持「插槽数据为空时组件返回 null」——这样壳层与引入前一致（spec 的回退要求）。

- [ ] **Step 6: 跑前端全量检查**

```bash
cd /Users/noelorin/OrinForum && pnpm check && pnpm build
```
Expected: 全绿

- [ ] **Step 7: 提交**

```bash
cd /Users/noelorin/OrinForum
git add app/web/components/ app/web/lib/ packages/api-client/src/
git commit -m "feat(web): shell slot host and renderers"
```

---

## Task 11: `fbp` CLI

**Files:**
- Create: `packages/plugin-cli/package.json`
- Create: `packages/plugin-cli/src/index.ts`（`bin` 入口）
- Create: `packages/plugin-cli/src/commands/init.ts`
- Create: `packages/plugin-cli/src/commands/dev.ts`
- Create: `packages/plugin-cli/src/commands/build.ts`
- Create: `packages/plugin-cli/templates/js/`（`plugin.json` + `index.js` + `package.json`）
- Create: `packages/plugin-cli/AGENTS.md`
- Test: `packages/plugin-cli/src/commands/init.test.ts`

**Interfaces:**
- Produces: `fbp init <name>` / `fbp dev` / `fbp build`

- [ ] **Step 1: 写失败的测试**

```ts
import { mkdtemp, readFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { describe, expect, it } from 'vitest';

import { initPlugin } from './init';

describe('fbp init', () => {
	it('生成可解析的 plugin.json', async () => {
		const dir = await mkdtemp(join(tmpdir(), 'fbp-'));
		await initPlugin({ name: 'my-plugin', dir });
		const raw = await readFile(join(dir, 'plugin.json'), 'utf8');
		const m = JSON.parse(raw);
		expect(m.name).toBe('my-plugin');
		expect(m.protocol_version).toBe('1');
		expect(Array.isArray(m.hooks)).toBe(true);
	});

	it('生成的入口文件不依赖任何 @feedchamber 包', async () => {
		const dir = await mkdtemp(join(tmpdir(), 'fbp-'));
		await initPlugin({ name: 'my-plugin', dir });
		const src = await readFile(join(dir, 'index.js'), 'utf8');
		expect(src).not.toContain('@feedchamber/');
	});
});
```

- [ ] **Step 2: 运行测试确认失败**

```bash
cd packages/plugin-cli && pnpm vitest run src/commands/init.test.ts
```
Expected: FAIL — 包不存在

- [ ] **Step 3: 实现 CLI**

`package.json` 关键字段（与 `packages/api-client` 保持同一套工具链）：

```json
{
	"name": "@feedchamber/plugin-cli",
	"version": "0.1.0",
	"private": true,
	"type": "module",
	"bin": { "fbp": "./dist/index.js" },
	"scripts": {
		"build": "tsup",
		"dev": "tsup --watch",
		"test": "vitest run",
		"lint": "biome lint ./src",
		"check": "biome check ./src"
	},
	"devDependencies": { "tsup": "^8.4.0", "typescript": "~5.9.3", "vitest": "^4.1.8" }
}
```

`src/commands/init.ts`：

```ts
import { copyFile, mkdir, writeFile } from 'node:fs/promises';
import { join } from 'node:path';

export interface InitOptions {
	name: string;
	dir: string;
}

/** 生成一个最小可用插件工程。产物零第三方依赖：协议客户端内联在 index.js 里。 */
export async function initPlugin({ name, dir }: InitOptions): Promise<void> {
	await mkdir(dir, { recursive: true });

	await writeFile(
		join(dir, 'plugin.json'),
		JSON.stringify(
			{
				name,
				version: '0.1.0',
				protocol_version: '1',
				description: '',
				entry: 'index.js',
				hooks: ['post.created'],
				capabilities: ['posts:read'],
			},
			null,
			'\t',
		) + '\n',
	);

	await writeFile(join(dir, 'index.js'), TEMPLATE);
}

const TEMPLATE = `// 插件入口。运行时由站点以 \`node index.js\` 拉起，
// 通过 stdin/stdout 上的 NDJSON 帧与宿主通信（协议见站点 docs/design/plugin-hooks.md）。
//
// 不要在这里 require 任何 @feedchamber/* 包：插件产物必须零框架依赖，
// 协议读写就下面这几十行。

let buf = '';

process.stdin.setEncoding('utf8');
process.stdin.on('data', (chunk) => {
	buf += chunk;
	let i;
	while ((i = buf.indexOf('\\n')) >= 0) {
		const line = buf.slice(0, i);
		buf = buf.slice(i + 1);
		if (!line.trim()) continue;
		handleFrame(JSON.parse(line));
	}
});

function reply(id, result) {
	process.stdout.write(JSON.stringify({ id, result }) + '\\n');
}

function replyError(id, message) {
	process.stdout.write(JSON.stringify({ id, error: { message } }) + '\\n');
}

async function handleFrame(frame) {
	if (frame.method === 'hook.post.created') {
		const post = frame.params ?? {};
		// TODO: 在这里写你的逻辑。
		// 需要写数据时调用宿主 API：
		//   await call('api.posts.addTag', { post_id: post.id, tag: 'auto' });
		reply(frame.id, { ok: true, saw: post.id });
		return;
	}
	replyError(frame.id, 'unknown method: ' + frame.method);
}

// 调用宿主 API 并等待结果。
let nextId = 1_000_000;
const pending = new Map();

function call(method, params) {
	const id = nextId++;
	return new Promise((resolve, reject) => {
		pending.set(id, { resolve, reject });
		process.stdout.write(JSON.stringify({ id, method, params }) + '\\n');
	});
}

// 宿主回包分流：id >= 1_000_000 是本端发起的 API 调用。
const originalReply = reply;
process.stdin.on('data', () => {}); // 帧解析已在上面统一处理，此处仅为可读性占位
`;
```

（说明：模板里 `pending` 的分流逻辑要接到主循环的 `reply` 分支——实现时把主循环改成「先查 `pending`，命中则 resolve，否则按钩子处理」。这是实现步骤的一部分，不要留 TODO。）

`src/index.ts` 用最简单的 argv 分派：

```ts
#!/usr/bin/env node
import { initPlugin } from './commands/init';
import { devPlugin } from './commands/dev';
import { buildPlugin } from './commands/build';

const [cmd, ...rest] = process.argv.slice(2);

switch (cmd) {
	case 'init':
		await initPlugin({ name: rest[0], dir: process.cwd() });
		break;
	case 'dev':
		await devPlugin({ site: flag(rest, '--site'), token: flag(rest, '--token') });
		break;
	case 'build':
		await buildPlugin({ dir: process.cwd() });
		break;
	default:
		console.error('用法: fbp <init|dev|build>');
		process.exit(1);
}
```

`fbp dev` 的两种模式：无 `--site` 时用内置 mock 宿主（把 Task 3 的 Go IPC 编解码编译成 `fbp-mockhost` 二进制随包分发，**不要用 TS 重写协议**）；有 `--site` 时直连真实站点。

- [ ] **Step 4: 运行测试确认通过**

```bash
cd packages/plugin-cli && pnpm vitest run && pnpm build
```
Expected: PASS

- [ ] **Step 5: 提交**

```bash
cd /Users/noelorin/OrinForum
git add packages/plugin-cli/ pnpm-workspace.yaml
git commit -m "feat(cli): fbp plugin scaffolding and local dev host"
```

---

## 收尾：文档与验收

- [ ] `docs/design/extension-points.md` 改写「不支持第三方热加载」原则为「支持子进程插件」，保留「不支持进程内加载」
- [ ] 新增 `docs/design/plugin-hooks.md`（钩子清单、IPC 帧格式、生命周期状态机图）
- [ ] 新增 `docs/design/plugin-security.md`（**必须**写明：插件与后端同用户，可自行连接数据库；「不直连数据库」是约定非强制）
- [ ] 新增 `app/server/internal/plugin/AGENTS.md`、`app/web/components/slot/AGENTS.md`、`packages/plugin-cli/AGENTS.md`
- [ ] 根 `AGENTS.md` 模块说明与文档索引
- [ ] `node scripts/check-agents-docs.mjs --write-index`
- [ ] `openspec validate --all --strict`
- [ ] `cd app/server && go build ./... && go test -race ./...`
- [ ] `pnpm check && pnpm build`
- [ ] `bash feedchamber_test.sh -all`
- [ ] 端到端：装示例插件 → 发帖 → 插件收到 `post.created` → 调 `api.slots.set` 写插槽 → 前台侧边栏显示
- [ ] 故障演练：`kill -9` 插件进程 → 宿主不受影响 → 自动重启 → 状态恢复为 running
- [ ] **安全验证**：在插件里 `console.log(process.env.DB_DSN)` → 必须输出 `undefined`

---

## Self-Review

**Spec 覆盖检查：**

| Spec Requirement | 对应 Task |
|---|---|
| 语义化钩子 | Task 2（钩子常量 + 未知名丢弃）、Task 5（分发） |
| 只异步钩子、不阻塞 | Task 5（`DispatchHook` 事后触发、超时 5s、失败只记日志） |
| 独立 Node 子进程 | Task 4（supervisor）、Task 3（IPC） |
| 生命周期状态机 | Task 7（模型 + `ListResumable` 恢复）、Task 4（退避重启） |
| 事件广播 | Task 1 |
| 写库经宿主 API + 能力校验 | Task 6（`lookupAPI` 的 `need()`）、Task 8 |
| 钩子外上下文失效 | Task 5（每次调用新建 `context.WithTimeout`） |
| 来源审批 + 展示能力 | Task 8 |
| npm 依赖隔离 | Task 8（`--prefix`，实现时补） |
| 环境变量白名单 | Task 4（`buildPluginEnv`，有正面测试） |
| 同用户边界如实声明 | Task 4 注释 + 收尾文档（`plugin-security.md`） |
| 插槽固定枚举 + 未知忽略 | Task 9（`ValidSlot`）、Task 10（`SLOTS` 集合） |
| 无裸 HTML | Task 9（`ValidSlotKind` 不含 html） |
| 渲染不调用插件 | Task 10（组件只读 `entries` prop） |
| 空配置回退 | Task 10（`mine.length === 0` 返回 null） |
| 插槽写入审计 | Task 9（service 里落 `AuditLogService`） |
| `internal/plugin` 不引用 repository | Task 7（Host 用回调注入，不持有 repo） |

**类型一致性检查：**
- `CapabilitySet` 在 Task 6 定义、Task 7 `HostOptions.APIDepsFor` 使用 —— 一致。
- Task 5 的 `Host` 在 Task 7 改名为 `dispatcher`，已在 Task 7 Step 6 明确写出 —— 需实现时注意。
- `PluginCaller` 签名 `(ctx, method, params) ([]byte, error)` 在 Task 5 定义，Task 6 的 `Conn` 需要适配 —— Task 5 的 `RegisterPlugin` 已提供 `conn.Call` 的适配闭包。

**已知缺口（实现时需补，非计划失败）：**
- Task 8 的 npm install 隔离：`--prefix` 与回滚的具体命令需在实现时确定。
- Task 11 模板里 `pending` 分流与主循环的衔接需在实现时写完整（测试已覆盖产物零依赖，但未覆盖请求/响应分流）。
- Task 10 的 `MarkdownView` 组件路径以实际源码为准。
