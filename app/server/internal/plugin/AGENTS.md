# Plugin 子进程插件（`internal/plugin/`）

以 **Node 子进程**运行的第三方插件子系统。`plugin.json` 是插件与站点之间唯一的契约文件，
本包负责读取它、校验它，并把钩子调用转成子进程上的 JSON-RPC。

> 上级：[app/server/AGENTS.md](../../AGENTS.md) · 规范：[openspec/changes/2026-11-04-plugin-hooks](../../../../openspec/changes/2026-11-04-plugin-hooks/proposal.md) · 相邻：[eventbus](../pkg/eventbus/)、[service](../service/AGENTS.md)（宿主 API 的真实业务入口）

## 为什么是子进程而不是进程内

| 方案 | 否决理由 |
|------|----------|
| `plugin.Open`（Go 原生插件） | 要求插件与宿主用**完全相同**的 Go 版本与依赖，且不支持交叉编译——会直接废掉 Makefile 的 `windows`/`darwin` 目标 |
| 进程内脚本求值（`eval` / goja / yaegi） | 插件代码与宿主共享地址空间，崩溃即宿主崩溃；Discourse 的 `eval` 模型依赖 Ruby 动态特性，Go 无等价物 |
| **Node 子进程 + stdio JSON-RPC** | 保留 npm 生态与热重启能力，不用 CGO，不影响交叉编译；进程隔离让崩溃只影响该插件 |

代价：IPC 序列化开销、进程管理复杂度（退避重启、进程组终止）。`supervisor.go` 与 `ipc.go` 各自收敛这部分。

## 三条硬约束

1. **子进程，非进程内**。插件 MUST NOT 被加载进宿主 Go 进程。宿主进程的存活与插件进程的崩溃完全解耦。
2. **stdio JSON-RPC**。没有网络协议栈、没有 WebSocket、没有端口。插件进程的 stdin/stdout 就是通道。
3. **不引用 `internal/repository`，不持有 `*gorm.DB`**。插件要写数据只能经宿主 API，由宿主在 `service` 层执行，
   与管理员操作走**同一业务入口**（审计、事件等副作用必须一致）。
   规范见 `openspec/changes/2026-11-04-plugin-hooks/specs/layered-architecture/spec.md`。

   **这条目前没有自动化门禁**，CI 不会替你拦。改动本包后请手工验证：

   ```bash
   cd app/server && go list -deps ./internal/plugin | grep repository   # 必须无输出
   cd app/server && go list -deps ./internal/plugin | grep -c '^gorm.io'  # 必须是 0
   ```

   第二条不是重复检查。`*gorm.DB` 不从 repository 来，而是从 `internal/db` 来：
   `eventbus → internal/metrics → internal/db → gorm`。**只要 `internal/plugin` 直接 import
   `internal/pkg/eventbus`，生产依赖图里就会凭空多出 10 个 gorm 包**（基线 0）。
   这正是 `pluginevent` 子包存在的原因（见「分层与事件接缝」）。
   忘了跑不会立刻出错——依赖会一路编译通过，直到某个插件路径真的碰到数据库才暴露。

第 3 条是分层要求的直接推论：`internal/plugin` 只消费 composition root 注入的接口，
它不知道数据存在哪、也不该知道。插件直连数据库在 v1 是**同用户进程的能力**，
不是文档承诺的强制隔离——见「安全边界」一节。

## 钩子：未知钩子丢弃，未知能力报错

两个方向刻意不对称，`manifest_test.go` 各钉一侧：

| 字段 | 未知值处理 | 理由 |
|------|-----------|------|
| `hooks` | **丢弃，不报错** | 核心新增钩子是常事（spec：`未定义的钩子名 MUST 被忽略`）；老插件声明的旧钩子名不该让它在升级后直接装不上 |
| `capabilities` | **报错** | capability 决定权限。写错一个字符却静默降级，等于给管理员展示一份与运行时不一致的能力清单 |

钩子名是**核心白名单**（`knownHooks` / `KnownHooks()`），插件 MUST NOT 自造钩子名。
v1 的 8 个钩子：`post.created` `post.closed` `comment.created`
`user.registered` `user.followed` `report.created` `moderation.decided` `cron.daily`。

### 钩子名 ≠ 事件名（`hookEvents`）

钩子名是给插件作者看的**语义名**，eventbus 的事件名是站点**内部传输标识**，两者会分叉。
`hookEvents` 是唯一的绑定表，**Task 5 必须查表，不得按字符串拼接推导**：

| 钩子 | 事件 | 备注 |
|------|------|------|
| `post.created` | `post.created` | 同名 |
| `post.closed` | `post.closed` | 同名 |
| `comment.created` | **`content.created`** | 分叉：评论的传输事件叫 content |
| `user.registered` | `user.registered` | 见下 |
| `user.followed` | `user.followed` | 同名 |
| `report.created` | `report.created` | 同名 |
| `moderation.decided` | `moderation.decided` | 同名 |
| `cron.daily` | **（无）** | ticker 驱动，不在事件映射里 |

- `post.updated` **已删除**：`internal/pkg/eventbus/events.go` 里没有任何对应事件，
  留着等于让插件声明一个永远不触发的钩子（能装、能加载、静默收不到调用）。
- `cron.daily` **由 `Host.StartDailyHookTicker` 自己的 ticker 触发**（首次延迟 2 分钟、之后每 24h），
  不经事件总线。`HookIsEventDriven("cron.daily") == false`，`SubscribeEvents` 据此把它排除在事件订阅之外。
  ticker 由 `container.Start` 装配为 `plugin_cron_daily` worker。

  这条**必须有 ticker**：`cron.daily` 没有事件源，不接 ticker 的话插件声明了该钩子却永远收不到调用，
  且症状是静默的（装得上、状态 running、就是不触发）。回归网 `TestDispatchReachesCronDailyHook`。
- `user.registered` 绑定**别名事件本身**。Auth 注册成功时同时发布 `user.created` 与
  `user.registered`（`internal/service/auth.go`，`EventUserRegistered` 是 `EventUserCreated`
  的历史别名）。绑成 `user.created` 会让 Task 5 按事件反查钩子时两条发布都命中，
  同一插件对**同一次注册收到两次调用**；绑别名事件则恰好只有一条命中。

`TestHookEventsAllExistInEventbus` 把映射表与 `eventbus.AllEventTypes()` 对照，
任何指向不存在事件的条目立刻变红——这是 I1 修复的回归网。

v1 **只做异步钩子**，MUST NOT 有 `*.before_*` 形态的写路径前置钩子：钩子在业务动作完成之后触发，
插件失败/超时/崩溃 MUST NOT 回滚已完成的业务动作。需要前置拦截的场景走新增 `moderation_mode` 档位，
MUST NOT 插进现有审核链中间。

## 文件

| 文件 | 职责 |
|------|------|
| `manifest.go` | `Manifest` 结构、`ParseManifest`、`(*Manifest).Validate`、`ProtocolVersion1`、8 个钩子常量与白名单、`hookEvents` 绑定表、`HookEvents()` / `HookIsEventDriven()`、`entry` 路径校验 |
| `manifest_test.go` | 解析/校验的契约：合法 manifest、未知协议版本、缺 name/version、未知钩子丢弃、未知能力报错、坏 JSON、钩子→事件映射、`entry` 逃逸拒绝 |
| `ipc.go` | stdio JSON-RPC 编解码：`Frame`/`FrameError`、`Conn`、`NewConn`/`Start`/`Close`、`Call`/`Notify`/`Handle`、读循环与写串行化 |
| `ipc_test.go` | IPC 契约（外部测试 `package plugin_test`，用 `net.Pipe()` 对接两条 `Conn`）：结果回传、handler 错误透传、ctx 超时、未知方法报错、通知不等待响应、关闭解除在途调用、坏行跳过、写入串行、重复响应不卡死 |
| `supervisor.go` | `os/exec` 拉起 `node`、`buildPluginEnv` 白名单重建环境、退避重启（1s→2s→4s→8s→16s，`MaxRestarts` 默认 5）、三步关停、`Running()` / **`Conn()`（取当前代）** |
| `supervisor_test.go` | 内部测试（`package plugin`）：白名单重建/空值跳过/白名单本身无敏感名、退避阶梯、默认值、拉起握手、**子进程 env 端到端**、退避重启与超限停止、**Stop 解开卡死写** |
| `hooks.go` | **`dispatcher`**（插件注册表 + 钩子分发）：`PluginCaller`、`newDispatcher`、`RegisterPlugin`（收 `*Supervisor`）/`RegisterPluginWithCaller`/`UnregisterPlugin`、`Dispatch`/`dispatchHooks`、`SubscribeEvents`、`Broadcaster`/`TraceBroadcaster` 接缝、`invertHookEvents`、`supervisorCaller` |
| `hooks_test.go` | 分发契约（内部测试 `package plugin`）：fan-out 全员收到、非订阅者不调用、失败隔离、重复声明去重、摘除、**Dispatch 不阻塞**、**每调用超时**、**不继承调用方取消**、倒排索引、`content.created→comment.created`、**`user.registered` 双发只投一次**（fake + 真 eventbus 两条）、**换代后走新连接**、真实 IPC 往返、并发注册/分发 |
| `api.go` | **宿主 API**：9 个 capability 常量与白名单、`capabilityWithoutAPI`（空能力白名单，只许变短）、`KnownCapabilities()`（**已排序**）、`IsKnownCapability`、`CapabilitySet`、`CapabilitySetFromNames`、7 个接缝接口（`PostWriter` 等）、`APIDeps`、**`Func`**（解码后的处理函数）、`apiSpecs` 规格表（params + AuditSpec 声明式）、`lookupAPI`（查找即校验）、**`adapt`**（解码→校验→执行→审计，取代旧的 `wrap`）、`RegisterAPIs`、`pluginKVKey`（KV 命名空间） |
| `args.go` | **参数声明与解码**：`FieldType`（`FieldString`/`FieldInt`/`FieldObject`/`FieldBool`，零值必须是 `string`）、`Field`（`Name`/`Type`/`Required`/`AllowEmpty`/`NoTrim`/`Enum`/`Desc`）、`Args`（`String` 统一 TrimSpace、`Int`、`Raw`、`Has`）、`decodeArgs`（**全部宿主 API 的唯一解码入口**，原先在 api.go 里重复的取参/校验逻辑已并入这里）。错误文案与重构前逐字一致 |
| `audit.go` | **审计声明**：`AuditScope`（`AuditScopePlain`/`AuditScopePluginKV`）、`AuditSpec`（`Action`/`Target`/`Detail` 是**参数名**，不是第二份 Unmarshal 闭包）、`isZero`、`extract`、`joinArgs` |
| `schema.go` | **JSON Schema 派生产物**：`schemaType`（对未映射的 `FieldType` panic）、`schemaFor`（由同一份 `[]Field` 生成，空 `Field.Name` panic）、`APISchemas()`（方法名 → schema，序列化失败跳过而非让调用失败） |
| `api_test.go` | 宿主 API 契约（内部测试 `package plugin`）：未声明能力被拒、已声明能力放行、**nil/空能力集拒绝每一个写类 API**、**写类 API 全部审计**、**写类规格必须声明 audit 且 audit 字段必须存在于 params**（这两条才是审计口径的真正担保，`TestWriteAPIsAreAudited` 只数字节数）、审计失败不翻转结果、缺字段不触达 service、**KV 键逃逸拒绝**（`../`、绝对路径、冒充他人命名空间）、**KV 按插件隔离**、`kv:read`/`kv:write` 互不蕴含、`CapabilitySetFromNames` 能直接喂 `Manifest.Validate`、**反向校验空能力**（`TestEveryCapabilityUsedByAnAPI` + 白名单 key set 上界）、`status` 枚举字面量与 `model.PostStatus*` 对齐 |
| `args_test.go` | 参数解码（`package plugin`）：缺必填、纯空白算缺失、缺失名排序、`AllowEmpty` 放行、`String` 统一 trim、未知字段容忍、空 params 合法、坏 JSON、类型错误、**`NoTrim` 保留原值**、**`NoTrim` 仍拒纯空白**、`Int`/`Raw` 存值、`FieldType` 零值是 string |
| `audit_test.go` | 审计提取（`package plugin`）：按声明字段取三元组、多字段拼接、空 `Detail`、**`AuditScopePluginKV` 落全键**、`pluginKVKey` 失败时保留原值、`isZero` 判定 |
| `schema_test.go` | schema 与 `params` 同源（`package plugin`）：覆盖全部方法、`required` 与声明一致、无必填时不写 `required`、`Enum` 进文档、类型映射、根为 object 且带 description、可序列化、空字段名 panic、**未映射 `FieldType` panic** |
| `pluginevent/broadcast.go` | `pluginevent.Broadcaster(*eventbus.EventBus)`：把 eventbus 广播适配成 `plugin.Broadcaster`+`TraceBroadcaster`。**独立成包是为了不让 eventbus 的 gorm 依赖进入 `internal/plugin`**，见「分层与事件接缝」 |
| `pluginevent/broadcast_test.go` | 适配语义（`package pluginevent_test`）：type/payload 透传、实现 `TraceBroadcaster` 且 ctx 非 nil、nil bus/handler 安全空操作 |

后续任务追加（同一包，**内部测试用 `package plugin`、外部契约测试用 `package plugin_test`**，两者可共存）：

| 文件 | 职责 |
|------|------|
| `host.go` | 对外的 `Host`（内嵌 `*dispatcher`）：`NewHostWithOptions`、`StartPlugin`/`StopPlugin`/`StopAll`/`Running`/`Invoke`、`HasAPIDeps`、`HostOptions.OnStatus`（状态落库回调）、`HostOptions.APIDepsFor`（按插件构造宿主 API 依赖）。**不拉起任何进程**——那是 `StartPlugin` 的事 |

`HasAPIDeps()`（2026-11-05）不是给运行时用的，是给 composition root 的装配断言用：漏接 `APIDepsFor` 时 `RegisterAPIs` 永不执行、9 个 `api.*` 空壳且没有任何现有测试会变红。容器侧由 `plugin_wiring_test.go` 的 `TestNew_WiresPluginHostRuntime` 钉住。真实接缝适配在 `internal/container/plugin_api_deps.go`（本包保持不 import repository / 不持有 gorm）。

### `Host` 与 `dispatcher` 的分工

`dispatcher`（hooks.go）只记「谁订阅了哪个钩子」并扇出；`Host`（host.go）管进程起停、
能力授予与代际轮换。分两层的实际收益：`dispatcher` 的测试可以塞假 caller 不拉真进程，
`Host` 的测试才需要 node。

`Host.StartPlugin` 里的顺序是刻意的：

1. **占位防并发**（`supers[id] = nil`）——两个 goroutine 同时 `StartPlugin` 同一 ID 时，
   只靠「先查后写」会双双通过检查，最后跑两份进程、其中一份成孤儿。
2. **先起进程再注册钩子**——反过来的话，钩子已注册但 Conn 还没有，分发会撞上空连接。
3. **`RegisterPlugin(id, m, sup)` 传的是 supervisor 而不是 `conn`**——`dispatcher` 每次分发
   都重新取 `sup.Conn()`，崩溃重启后自动用新一代。传 `*Conn` 就是 Task 4 点名的失效模式。

`StopPlugin` 相反：**先摘钩子再停进程**。否则进程已死、钩子还在，分发会持续撞死 Conn。

`OnStatus` 在 supervisor 的 goroutine 上同步执行，`container` 里因此把它包成 `go func()`——
同步写库会拖慢退避重启节奏（见 `supervisor.go` 的 `OnCrash` 注释）。

## 钩子分发：`dispatcher`（`hooks.go`）

`dispatcher` 只做两件事：记住「哪个插件声明了哪些钩子」，把一次钩子调用扇出到所有订阅者。
它**不**管理生命周期、不碰数据库——那些是 Task 7 的 `Host`（内嵌 `*dispatcher`）。

名字刻意不叫 `Host`：Task 7 另有一个对外的 `Host` / `NewHostWithOptions()`。
本类型与其构造函数保持**未导出**（`dispatcher` / `newDispatcher`），避免两个 `Host` 打架。

### 分发的四条语义（每条都有测试钉住）

| 语义 | 实现 | 回归网 |
|------|------|--------|
| **异步**：不等插件完成 | 每个目标一个 `go func`，`Dispatch` 立即返回 | `TestDispatcherDispatchDoesNotBlockOnSlowPlugin` |
| **独立超时**：每次调用都有上限 | `context.WithTimeout(ctx, d.timeout)`，默认 `hookTimeout`=5s | `TestDispatcherDispatchAppliesPerCallTimeout` |
| **不继承调用方取消** | `context.WithoutCancel` 只保留值（trace） | `TestDispatcherDispatchSurvivesDeadCallerContext` |
| **失败隔离** | 每个目标各自的 goroutine，错误只记日志 | `TestDispatcherDispatchFailureDoesNotBlockOthers` |

三条容易弄反的地方：

1. **超时是必须的，不是防御性代码**。`Conn.Call` 会 honour ctx，但调用方是事件总线的
   广播 goroutine；一次不返回的钩子会永久占住一个 goroutine，且在 `Dispatch` 改成同步时
   直接拖住后续所有同类型事件的投递。
2. **必须 `context.WithoutCancel`**。钩子在业务动作**完成之后**触发，而请求返回时取消 ctx
   是常态。继承那次取消＝钩子被静默掐断，症状是「插件有时收到、有时收不到」。反过来，
   `Dispatch` 拿到的 ctx 仍然只用于传值，**不**用于控制生命周期。
3. **IPC 方法名是 `hook.<钩子名>`**，不是裸钩子名，也不是事件名。插件侧按方法名分派
   （`hook.post.created`），与 `ipc_test.go` / Task 6 的宿主 API 同一命名空间。

### 事件桥接：必须用倒排表

`SubscribeEvents` 遍历的**不是** `KnownHooks()` 做恒等映射，而是 `invertHookEvents(HookEvents())`
（事件 → 钩子列表）。三种错法各有明确症状：

| 错法 | 症状 |
|------|------|
| 遍历 `KnownHooks()` 恒等映射 | `comment.created` **永远收不到**（真实事件叫 `content.created`）；`cron.daily` 订阅一个不存在的事件 |
| 把 `user.registered` 绑成 `user.created` | 一次注册**投递两次**（auth 双发两条都命中） |
| 为每个钩子各建一条订阅 | 一事件多钩子时同一事件被订阅多次，未来加钩子就会重复投递 |

倒排天然幂等：**每个事件只订阅一次**。当前 7 个事件驱动钩子对应 7 个事件，
`cron.daily` 不在表内（`TestDispatcherSubscribeEventsSubscribesEventDrivenHooksOnly` 断言 `Len==7`）。

### 分层与事件接缝：`pluginevent` 子包为什么存在

`dispatcher` 只认 `Broadcaster` 接口，不 import `eventbus`。真实接线在
`internal/plugin/pluginevent`：

```go
// composition root（Task 7）这样接：
bus := eventbus.New(...)                    // 已存在
host.SubscribeEvents(pluginevent.Broadcaster(bus))
```

分成两个包**不是为了好看**，而是因为依赖方向：
`eventbus → internal/metrics → internal/db → gorm`。把 `eventbus` 引进 `internal/plugin`
会让本包的生产依赖图凭空多出 **10 个 gorm 包**（基线 0），第 3 条硬约束就只剩文档承诺。
接缝留在本包、接线放在子包，是本包唯一能同时满足「不碰 gorm」与「能收到真实事件」的形态。

`TraceBroadcaster` 是**可选**增强接缝：eventbus 的广播 handler 拿得到带 traceparent 的
`Event.Ctx`，只实现二元 `Broadcaster` 会静默丢掉它，插件钩子的日志就没有 trace_id 了。
`pluginevent.Broadcaster` 同时实现两者，`SubscribeEvents` 用类型断言择优。

## IPC：帧格式与请求/响应关联

子进程的 stdin/stdout 就是通道，**没有长度前缀、没有分隔符协商**：每个帧是
**一行紧凑 JSON**，以 `\n` 结尾（NDJSON）。读侧用 `bufio.Scanner` 按行切
（单行上限 8MB，超长帧会让读循环结束并关闭连接），写侧 `json.Marshal` 后补 `\n`。

```json
{"id":7,"method":"hook.post.created","params":{"id":"p1"}}
{"id":7,"result":{"seen":"p1"}}
{"id":7,"error":{"message":"unknown method: boom"}}
{"method":"event","params":{"k":"v"}}
```

| 字段 | 出现于 | 说明 |
|------|--------|------|
| `id` | 请求、响应 | 关联键。`Call` 用单调递增计数器分配，**只有 `Call` 分配 id**；`Notify` 不分配 |
| `method` | 请求 | **方向判定只看它**：有 `method` 是请求，无 `method` 是响应 |
| `params` | 请求 | 任意 JSON，原样交给 handler |
| `result` | 响应 | handler 返回值的 `json.Marshal` 结果（`nil` 会变成字面量 `null`） |
| `error` | 响应 | `{"message": "..."}`，非空时 `Call` 返回它并丢弃 `result` |

**关联规则：响应靠 `id` 回到等待者，方向靠 `method` 是否为空区分。**
`Call` 在发出请求**之前**把 `id → chan Frame` 注册进 `pending`，读循环收到无
`method` 的帧就按 `id` 查表投递；`Call` 在 defer 里注销。因此：

- 响应**先于**注册到达是不可能发生的（同一 goroutine 里先注册后写）。
- `id` 对不上任何在途调用（超时后迟到的响应、对端乱填的 id）被**静默丢弃**，
  不报错、不重试——超时已经让调用方拿到了 `ctx.Err()`。
- **重复响应必须丢弃，投递必须非阻塞**。读循环用 `select { case ch <- f: default: }`。
  改成阻塞式 `ch <- f` 会引入一个隐蔽的永久卡死：`ch` 容量 1，第一条响应被
  `Call` 取走、`pending` 条目随后被删，此时第二条同 id 的响应到达，读循环就
  永远等不到接收者——连接看起来还活着，之后**每个 `Call` 都只会超时**。
  任何写错或恶意的插件都能触发。`TestConnDuplicateResponseDoesNotWedge` 钉住它。
- 请求**不按 `id` 去重**，同一个 `id` 收到两次就是两个独立的请求。

四条实现红线，改动时别弄反：

1. **写必须串行**（`writeMu`）。两个 goroutine 各写半行会让对端读到一行拼接的
   坏 JSON；坏行在读侧被跳过，症状是「帧凭空消失」而不是报错。
2. **坏帧跳过，不致命**。单行 `json.Unmarshal` 失败只 `continue`——插件打印的
   调试日志（`console.log` 走同一个 stdout）不该拆掉整条连接。
3. **读循环绝不持锁调用 handler**。`dispatch` 先取表、**释放 `c.mu`**、再执行
   handler；每个入站请求还各自起一个 goroutine。handler 里回调对端（插件问宿主、
   宿主再问插件）是合法用法，一旦读循环阻塞或持锁，这类回调就永远等不到自己的
   响应——表现为双向死锁而不是错误。
4. **响应的投递必须非阻塞**，见上一条关联规则里的重复响应说明。

### 写的阻塞边界：只有 `Call` 能被打断

`Call` 的写跑在独立 goroutine 上、且有两段 `select`（先等 write 完成，再等响应），
所以 ctx 到期一定能解开它。**但 `Notify` 与 `dispatch` 的写是同步的**——它们在
调用方 goroutine 上直接拿 `writeMu` 并 `Write`，没有 ctx、没有超时：

| 路径 | 写在哪 | 对端不消费 stdin 时 | 谁能解开 |
|------|--------|---------------------|----------|
| `Call` | 独立 goroutine | 卡在 goroutine 里，`Call` 按时返回 `ctx.Err()` | ctx 到期 / `Close` |
| `Notify` | **调用方 goroutine** | **调用方被阻塞** | **只有杀掉对端进程** |
| `dispatch` | **该请求的 goroutine** | 该 dispatcher goroutine 被阻塞 | **只有杀掉对端进程** |

**关键区别：`Close` 和「杀进程」不是同级的两条路。** 对一次**已经进入 `write(2)`** 的写，
`Close` 解不开它——Go 的 `internal/poll.FD` 在写期间持 refcount，`Close` 只置 `closing`
位并把 refcount 减一，真正的 fd 要等那次写返回、refcount 归零才释放。**只有对端进程死掉**
（它持有的读端随之关闭 → 管道破裂 → 阻塞的 `write(2)` 拿到 EPIPE 返回）才能解开它。

`Close` 真正能解开的是另外两类：阻塞在 `Scan` 上的**读**，以及**尚未进入 `write(2)`**、
还在等 `writeMu` 的写。所以：

- 一个卡死的 `writeMu` **只能靠杀掉对端进程解除**。Task 4 的
  「超时 → kill 子进程」不是礼貌收尾，是**唯一解药**，supervisor MUST 保留它。
  删掉 kill 退路后，一个不读 stdin 的插件能把宿主的调用方 goroutine 一条条耗光。
- 这正是 `Stop` 里 `SIGTERM` → 宽限 → `SIGKILL` 必须存在的原因；`conn.Close()`
  只是关停三步里的第一步（阻止新的写进来），不是解除阻塞的手段。
- 回归网：`TestSupervisorStopUnblocksWedgedWrite`。删掉 `Stop` 里**全部**杀进程代码即红
  （只剩主路径 SIGTERM 时仍绿——宽限期兜底会接住，这是刻意的双层设计）。

注意 `Call` 里那个写 goroutine **不保证会退出**：它只在 `Write` 返回后才结束，
若对端永久卡住且 `w` 不是 `io.Closer`，它就被永久 pin 住。容量 1 的缓冲只保证
它回送结果时不阻塞、不泄漏 channel，不是「goroutine 一定会回收」的保证。

### 流的所有权与 supervisor 关停顺序

**`Conn` 接管 `r`/`w` 的所有权，`Close` 会主动关闭它们**（实现了 `io.Closer` 时）。
它解开的是阻塞的 **`Scan`（读）** 与**尚未进入 `write(2)`** 的写；已经卡在 `write(2)`
里的写只能靠杀掉对端进程解开（原因见上一节）。代价是：调用方 MUST NOT 与 `Close` 并发地
关同一对管道，尤其不能让它与 `cmd.Wait()` 抢——`os/exec` 会在 `Wait` 里关闭
`StdinPipe`/`StdoutPipe`，并发关闭是数据竞争，在部分平台会返回一个伪错误，
supervisor 误判成插件崩溃就会白白触发 1s→2s→4s 重启阶梯。

Task 4 的推荐顺序：`conn.Close()` → 结束子进程（SIGTERM 宽限后 SIGKILL）→ `cmd.Wait()`。
**永远不要自己关 `StdoutPipe`/`StdinPipe`**。

### 关闭后的错误身份不确定

`Close` 之后 `Call` 返回 `ErrClosed`（用 `errors.Is` 判断，别匹配文案）。但若 `Close`
与某次在途写并发，那次写可能先把管道写失败，返回 `net.ErrClosed` /
`io.ErrClosedPipe` / EPIPE 一类 I/O 错误而非 `ErrClosed`。两者都只意味着「连接没了」，
调用方 MUST NOT 依赖具体是哪一个。

未知方法**必须回错误帧**（`unknown method: <名>`），不能静默丢弃——否则对端的
`Call` 只能等到自己的 ctx 超时，症状是「钩子调用莫名变慢」，排查时看不到任何线索。

## supervisor：管道用 `os.Pipe()`，不用 `StdinPipe`/`StdoutPipe`

上面那条「不要与 `Wait` 抢着关管道」的约束，`supervisor.go` 是**从构造上绕开**的，不是靠
小心避让：它用 `os.Pipe()` 自己建两条管道，把**子进程端**以 `*os.File` 交给
`cmd.Stdin`/`cmd.Stdout`。`os/exec` 对 `*os.File` 直接继承 fd——不包装、不起拷贝
goroutine、也不把父进程端登记进 `parentIOPipes`。于是：

| | `StdinPipe`/`StdoutPipe` | `os.Pipe()` + `*os.File`（本实现） |
|---|---|---|
| 父进程端由谁关 | `cmd.Wait` | `Conn.Close`（唯一所有者） |
| 与 `Wait` 并发关闭 | **会**（Task 3 点名的假 Wait 错误来源） | 不可能重叠 |
| 「读完才能 Wait」 | 是（文档约束） | 不适用，`Wait` 只管回收进程 |
| 额外负担 | — | Start 后必须由本文件关掉子进程端的另外两个 fd |

最后一行是这套写法的**唯一陷阱**：父进程若留着 `inR`（子进程的 stdin 读端）或 `outW`
（子进程的 stdout 写端），「写端拿到 EPIPE」和「读端看到 EOF」都会失灵——插件崩溃会变成
永久静默，而不是可观测的退出。见 `spawnLocked` 里的 `closeAll(inR, outW)`。

`superviseLoop` 是**唯一**调用 `cmd.Wait()` 的地方（重启循环也在它内部），所以
「等 `Wait` 返回」＝「等这个 goroutine 结束」，`Stop` 靠 `sync.WaitGroup` 收尾。
`Conn.Close` 与 `cmd.Wait` 因此永远是**先后**关系，不是并发。

### 进程终止：SIGTERM 宽限后 SIGKILL，且 `cmd.Process.Kill()` 是唯一解药

`Stop` 三步：`conn.Close()` → `SIGTERM`（宽限 `stopGrace`）→ 等 `Wait` 收尾（超时则 SIGKILL）。

单靠 `conn.Close()` **解不开一个已经卡在写上的 `Notify`**：并发写会把 `poll.FD` 的
refcount 抬起来，`Close` 只置 `closing` 位，真正的 fd 要等那次写返回才释放。这正是
「超时 → 杀进程」必须存在的原因——**杀进程让对端读端关闭，阻塞的 `write(2)` 立刻拿到
EPIPE 返回**。`TestSupervisorStopUnblocksWedgedWrite` 钉住它：把 `Stop` 里两处杀进程的
代码都删掉，该测试会以「停止超时，进程未被回收」变红（只删主路径不会红，因为宽限期兜底
会接住——这是刻意的双层设计，不是冗余）。

### `Start` 只返回第一代 `Conn`，用 `Conn()` 取当前代

`Start` 只能返回一次，所以它返回的永远是**第一代**连接。崩溃自动重启后，新一代进程有
自己的管道与 `Conn`，**不经 `Start` 返回**——宿主手里的那个已经死了，但 `Running()` 是
`true`。只看 `Running()` 就继续用旧 Conn 发钩子，**每个钩子都会拿到 `ErrClosed`，
而状态面板显示「运行中」**，这是最难排查的一类故障。

因此 supervisor 提供 `Conn()`，返回**当前这一代**的连接（无锁外泄漏，start 前/Stop 后为 nil）：

```go
sup.Start(ctx)                       // 首次拉起
sup.OnCrash = func(reason string) {  // 收到崩溃信号：标记状态
    // 注意：不要在这里同步调 Stop()，见下
}
conn := sup.Conn()                   // 任何时刻取「现在」那条，别缓存 Start 的返回值
```

**正确用法是把 `ErrClosed` 当作「这一代结束了」的信号**，去等下一次 `Conn()` 换代，
而不是重试同一条连接。`Conn()` 返回的指针仍可能在你拿到之后失效（对端恰好此刻崩溃）——
这是分布式生命周期的固有竞态，不是缺陷。

回归网：`TestSupervisorConnReturnsCurrentGeneration` 用「应答一次即崩溃」的插件逼出重启，
断言 (a) 重启后 `Conn()` 指针变了、(b) 旧 Conn 拿到 `ErrClosed`、(c) 新 Conn 能完成往返。
把 `Conn()` 改成永远返回第一代即红。

`OnCrash` 在 supervisor 自己的 goroutine 上**同步**执行、且不持锁（回调里调 `Running()` /
`Conn()` 不会死锁），但**同步调用 `Stop()` 会死锁**——`Stop` 要等这个 goroutine 结束，
而 goroutine 正卡在回调里等 `Stop` 返回。耗时的回调请自行开 goroutine；
Task 7 的 `OnStatus` 写库正是这么做的。

`Start` 被重复调用时会先停掉上一代（`replaced` 分支），避免同一插件跑两份。

### 刻意不做的事：进程组管理与「退出码零也算崩溃」

两处**有意识的取舍**，不是遗漏。改动前先读理由。

**1. 不管理进程组，孙进程可能变孤儿（v1 刻意不做，交 Task 8 安装流程）。**
杀 `cmd.Process` 只杀 node 本身。插件若自己 spawn 子进程（`npm install`、worker、
`sleep` 一类），node 死后这些孙进程会成孤儿继续跑。v1 的运行时钩子路径不 spawn 子进程，
风险集中在**安装/依赖安装**（Task 8）——那里有明确的超时与清理窗口，是正确的位置。
若将来要在运行时管理进程组，属 Unix-only（`Setpgid` + `kill(-pgid)`），
Windows 需要 Job Object，且与 `exec.CommandContext` 的取消语义有交互，改动面不小。

**2. `exit 0` 也算崩溃，进重启阶梯。**
当前实现里任何非 `stopped`/`replaced` 的退出都算崩溃，包括插件自己 `process.exit(0)`。
这是刻意的：**让一个本想常驻的插件静默消失，比多重启几次更糟**——后者在日志里可见、
会被退避与上限收敛，前者只表现为「钩子莫名不再触发」。spec 只要求「异常退出必须重启」，
没有要求「正常退出必须不重启」，因此这不违反规范。
若将来需要区分，需要给 `OnCrash` 传结构化原因（退出码/信号），而不是 error 文案——
但 `SupervisorOptions.OnCrash` 的签名是 `func(reason string)`，改它是接口变更，需单独裁定。

## manifest 的校验分工

`ParseManifest` 只做**不需要外部输入**的校验：JSON 合法性、`name`/`version` 非空、
`protocol_version` 必须等于 `ProtocolVersion1`、`entry` 缺省为 `index.js` **且必须是插件目录内的相对路径**、
钩子白名单过滤。

`entry` 的校验不是形式主义：supervisor 会把它拼成 `<插件目录>/<entry>` 交给 `node` 执行。
绝对路径（`/etc/passwd`）与含 `..` 段的路径（`../../../etc/passwd`、`sub/../ok.js`）
都能逃出插件目录，等于把 manifest 变成任意文件执行入口。`validateEntry` 因此拒绝
绝对路径、任一 `..` 路径段（按 `/` 与 `\` 双分隔符切分），以及 trim 后为空的 entry。

`Validate(knownCaps)` 需要站点已知的能力集，而能力集由 `api.go` 提供——放在 `ParseManifest` 里
会让 `manifest.go` 反向依赖 `api.go`。因此 capability 校验是**第二道门**，`ParseManifest` 通过
不代表 manifest 可用。`Validate` 同时拒绝「无钩子」的 manifest：钩子被全部过滤掉后，
插件没有任何入口，加载它只会得到一个永不工作的进程。

## 宿主 API：capability 校验放在查找阶段（`api.go`）

插件调用宿主 API 走**同一对管道**：`method` 形如 `api.posts.setStatus`，`params` 是参数对象。
`RegisterAPIs(conn, pluginID, caps, deps)` 在插件启动后调用一次。

### 三个错法的对策（这是本节的核心）

| 错法 | 症状 | 本实现的对策 | 回归网 |
|------|------|-------------|--------|
| `caps` nil/空时默认放行 | 没申请任何能力的插件能改库 | `CapabilitySet.grants` 对 nil 天然为 false，且 **`cap == ""` 恒不通过**——某个 spec 漏写 cap 时该 API 变成**不可调用**，而不是人人可调 | `TestLookupAPIFailsClosedWithoutCapabilities`（遍历 `writeAPIMethods()`） |
| 未知能力在运行期静默忽略 | 管理员看到的清单与运行时权限不一致 | `Manifest.Validate` 在**安装期**报错（`ParseManifest` 不管能力，见「manifest 的校验分工」）；运行期 `CapabilitySetFromNames` 只**收窄**（丢弃未知名），永不放宽 | `TestManifestValidateRejectsUnknownCapabilities`、`TestCapabilitySetFromNamesFeedsManifestValidate` |
| 为未授予的能力也注册 handler，让 handler 自己记得检查 | 新增 API 漏写一行即越权，且无人提醒 | 未授予时注册的是 **`denyHandler`**——它**不持有 deps**，里面没有任何 service 可碰；删除它内部的判定也执行不到业务代码 | `TestRegisterAPIsDoesNotHandleUndeclaredCapability`、`TestRegisterAPIsRejectsUndeclaredCapability` |

第三条的取舍值得单独说明：**「注册了 handler 但 handler 检查」与「不注册」在插件侧都表现成一次拒绝**，
但前者的安全性依赖「每个 handler 都写对了校验」，后者的安全性依赖「`lookupAPI` 这一条路径」。
后者只有一个地方会错，且那个地方的第一个动作就是查表——所以选它。
`RegisterAPIs` 里仍有第二道 `lookupAPI` 复查（纵深防御），但那不是主防线。

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

type Func func(ctx context.Context, a *Args) (any, error)
```

`params []Field` 是**唯一**的参数声明处，同时产出三样东西：

| 产物 | 实现 | 谁用 |
|------|------|------|
| 运行期校验 | `args.go` 的 `decodeArgs` | 每次调用 |
| 取值器 | `Args.String` / `Int` / `Raw` / `Has`（`String` 统一 TrimSpace） | handler 内部 |
| JSON Schema | `schema.go` 的 `schemaFor` / `APISchemas()` | 安装确认页、插件作者 |

重构前的形态是每个方法写两份匿名 struct：`build` 里一份、`audit` 里再 `json.Unmarshal` 一份
（共 17 处匿名 struct + 7 个 audit 闭包 + 2 次重复 Unmarshal）。**改一个字段名只改一处不报错**——
audit 按旧名解析得到零值，写操作照常生效、审计目标变成空串。现在字段名只出现在 `params` 里。

`AuditSpec` 把审计落点声明成**字段引用**，而不是提取闭包：

```go
audit: AuditSpec{Action: "post.setStatus", Target: []string{"post_id"}, Detail: []string{"status"}}
```

`api.kv.set` 需要把 target 落成**加过前缀的全键**，用 `Scope: AuditScopePluginKV` 表达，
由统一提取器调 `pluginKVKey`——不再为它单开一个闭包。`pluginKVKey` 失败时保留原值
（业务调用大概率也已失败，但审计仍应记下插件试图访问的键）。

三处收益，都是「让错误更难犯」而不是「少写几行」：

- `apiMethodNames()` / `writeAPIMethods()` / `apiCapabilities()` 全部**从规格表推导**，
  不再维护第二份手写字面量清单（brief 里的清单与 switch 是两份数据，少一条＝该方法永远 unknown method，不编译报错）。
- 审计在 `adapt` 里统一落，**handler 内部没有审计调用点**——「写类 API 忘审计」不可能发生。
- `TestEveryAPIMethodDeclaresACapability` 要求每个方法都登记已知能力：`cap` 漏写在这一层就被拦。

**「写类必须有审计」的真正担保是两条，不是一条。** `TestWriteSpecsDeclareAudit`（写类 `audit`
非零、读类为零）与 `TestAuditFieldReferencesExistInParams`（`audit.Target`/`Detail` 引用的
名字必须真在 `params` 里）在**静态规格表**上做全量断言；`TestWriteAPIsAreAudited` 只负责数
「每个写类各多了一条记录」，它查不出 `action` 退化成方法名——那是 `adapt` 里 `action == ""`
时用方法名兜底的代码路径，能落一条记录，所以只数条数会全绿。

`Field.NoTrim` 存在的唯一理由：`api.kv.get.key` / `api.kv.set.key` / `api.kv.set.value` /
`api.notifications.send.body` **必须原样传递**。`pluginKVKey` 用 `trimmed != key` 拒绝首尾空白，
预先把 `" k"` trim 成 `"k"` 会让它静默接受，等于削弱 KV 命名空间隔离。注意**必填非空仍用
trim 后的值判定**（纯空白 key 仍算缺失），`NoTrim` 只影响存储与传递。

**审计失败不翻转结果。** 业务动作已经生效，把 error 返回给插件只会让它重试，而重试会**再执行一次**写操作。
因此只记 error 日志（含 plugin/api/action 字段），返回成功。反过来，`deps.Audit == nil`（漏接线）
也只记 error 不 panic——handler 跑在 ipc 的 dispatch goroutine 里，未捕获的 panic 会终结整个后端进程
（`adapt` 自己的 `recover` 与 ipc 的 `callHandlerSafely` 各是一道网，两层都要留）。

### 参数校验：只查必填 / 空串 / 类型，不查取值域

`decodeArgs`（`args.go`）拒绝缺失或纯空白的必填字段（错误里**点名**哪个字段，按名排序），
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

空值绝不能透传到 service：`{}` 解出来全是零值。`SetStatus("", "")` 可能命中一条空主键记录，
也可能在 service 层变成一句看不懂的 SQL 错误。`value`（kv.set）是**唯一**允许为空串的字段
（`Field.AllowEmpty`）。

错误文案是**契约**，重构不得改变：坏 JSON 仍是「参数不是合法 JSON 对象: …」，
缺字段仍是「参数缺失: a, b」（按名排序，同一请求永远得到同一条文案）。
`decodeArgs` 刻意**不** panic：它跑在每次插件调用的热路径上，把编程错误变成每个调用
都返回「宿主内部错误」，比在构建期炸掉难排查得多。

### 接缝：`APIDeps` 只暴露业务动作

`PostWriter.SetStatus` / `AddTag`、`CommentWriter.SetStatus`、`UserReader.GetLevel` / `GetEmail`、
`Notifier.Send`、`KVStore.Get` / `Set`、`SlotWriter.Set` / `Delete`、`AuditRecorder.RecordPluginAction`。

刻意**没有**通用 CRUD（`Save(any)` / `Update(fields map[string]any)`）：收窄到枚举动作后，
每一个动作都能对应一条有意义的审计记录；通用 CRUD 的审计只能写成「插件改了啥」，
而且等于把 repository 的写权限整个交给插件。`deps` 字段为 nil（能力有但宿主没接线）时返回
`errAPINotWired`，与 `ErrCapabilityDenied` 分开——前者该找运维，后者该找管理员申请权限。

### KV 命名空间：前缀由 `api.go` 自己拼

`pluginKVKey(pluginID, key)` 返回 `plugin:<pluginID>:<key>`，实现方拿到的**已经是**限定好的全键：

- 前缀交给插件自觉 = 没有隔离；交给 `KVStore` 实现 = Task 7 用真实 kvstore 适配时要再写一遍，
  两份逻辑迟早分叉。`api.go` 是插件 key 进入系统的唯一入口，作用域拼在这里。
- 拒绝的形态：`..` / `.` 路径段（按 `/` 切分）、空路径段（`a//b`）、绝对路径（`/x`）、
  反斜杠、含 `:`（冒充/混淆命名空间）、含空白或控制字符、以空白开头/结尾、超过 200 字节、空 key。
- `pluginID` 自己也是逃逸面，同样校验：空 ID 会让所有插件共享 `plugin::`；含 `:` 的 ID（`"a:b"`）
  与另一个插件（`"a"` + `"b:c"`）拼出同一个全键。两者都直接拒绝**整个** KV 操作。
- `TestKVIsolatesPluginsFromEachOther` 端到端钉住：alpha 写 `shared`，beta 读 `shared` 拿到空；
  beta 直接点名 `plugin:alpha:shared` 被拒绝。
- **`kv:read` 与 `kv:write` 互不蕴含**（`TestKVCapabilitiesAreIndependent`）：只写不读是合法组合
  （插件把状态放进 KV 但不想读回来），合并它们会凭空扩大权限。

## 安全边界（必须如实表述）

插件子进程以**与后端相同的系统用户**运行。由此：

- 「插件 MUST NOT 直连数据库」是**约定**，MUST NOT 在文档或界面里表述为强制隔离；
- 系统 MUST NOT 基于「插件已被限制无法访问数据库」这一前提做任何安全假设；
- 唯一可执行的防线是**环境变量白名单**：拉起插件时 `cmd.Env` 由白名单**重建**，
  MUST NOT 以 `os.Environ()` 为基底再删除——白名单以外的一切（`DB_DSN`、`ADMIN_PASSWORD`、
  任何 `*_SECRET` / `*_KEY`）都不下发。

插件来源 MUST 经白名单与人工审批；安装前 MUST 向管理员展示其申请的能力清单与 npm 依赖清单。

## 事件来源：广播而非队列

插件订阅事件走 `eventbus.SubscribeBroadcast`（无 queue group），不是 `Subscribe`。
队列订阅在集群内**每个事件只消费一次**，多实例部署时插件会被随机分发到某一个实例，
「两个插件都收到同一事件」和「每个实例都要处理」都不成立。广播订阅与队列订阅各有独立的
handler/subscription map，互不污染——`internal/pkg/eventbus/broadcast_test.go` 钉住这一点。

## 测试

| 测试 | 钉住什么 |
|------|----------|
| `TestParseManifestValid` | 合法 manifest 的字段落位（含 `ProtocolVersion1` 与 cap 顺序不变） |
| `TestParseManifestRejectsUnknownProtocol` | 协议版本不在支持集合内即拒绝，错误里带 `protocol_version` |
| `TestParseManifestRequiresNameAndVersion` | `name` / `version` 为空各自报错 |
| `TestParseManifestDropsUnknownHooks` | **未知钩子丢弃而非报错**（spec 强制） |
| `TestParseManifestRejectsBadJSON` | 坏 JSON 报错 |
| `TestManifestValidateRejectsUnknownCapabilities` | **未知 capability 报错**（与钩子相反的半边） |
| `TestManifestValidateAcceptsKnownCapabilitiesAndRequiresHook` | 能力全已知时通过；钩子被过滤空后拒绝 |
| `TestHookEventsAllExistInEventbus` | **映射表里每个事件都在 `eventbus.AllEventTypes()` 中**，且钩子都在白名单里 |
| `TestHookEventsCoversEveryHookExceptCron` | 除 `cron.daily` 外每个钩子都有映射（漏一个＝该钩子静默失效） |
| `TestHookCommentCreatedBindsToContentCreated` | `comment.created` → `content.created` |
| `TestHookUserRegisteredBindsToAliasEvent` | `user.registered` → 别名事件本身，不是 `user.created`（防双投递） |
| `TestHookPostUpdatedIsGone` | `post.updated` 不在白名单、不在映射里 |
| `TestParseManifestRejectsEscapingEntry` | `entry` 为 `../../../etc/passwd` / `/etc/passwd` / `sub/../ok.js` / 空白时拒绝 |
| `TestParseManifestAcceptsPlainEntryAndDefaults` | 合法相对路径照常通过；缺省仍是 `index.js` |
| `TestConnCallReturnsResult` | 请求 `id`/`method`/`params` 落位，响应按 `id` 回到原调用方并解出 `result` |
| `TestConnCallPropagatesHandlerError` | handler 的 error 变成错误帧，`Call` 侧能读到原文（`unexpected EOF`） |
| `TestConnCallTimesOut` | 对端 handler 不返回时，`Call` 在 ctx 到期即返回 `context.DeadlineExceeded`（`errors.Is` 成立） |
| `TestConnUnknownMethodReturnsError` | **未知方法回错误帧而非挂起**，错误里带 `unknown method` |
| `TestConnNotifyDoesNotWaitForResponse` | `Notify` 立即返回，但请求确实被对端处理 |
| `TestConnCallReturnsOnClose` | 对端已关闭时在途 `Call` 立即返回错误，不空等到自己的 ctx 到期（`Call` 里 `select` 的 `c.closed` 分支）；关闭后新发起的 `Call` 直接拿到 `ErrClosed` |
| `TestConnSkipsMalformedLines` | **坏行跳过而不断连**：非 JSON、非对象 JSON、空行之后，合法请求仍被处理并回包 |
| `TestConnWritesAreSerialized` | **写必须串行**：16×8 个并发 `Notify`，对端读到的每一行都是完整合法 JSON 且条数不差（少 `writeMu` 就会读到拼接坏行） |
| `TestConnDuplicateResponseDoesNotWedge` | 同一 `id` 的第二条响应被丢弃而非堵死读循环；之后连接仍可正常 `Call`（阻塞式投递会永久卡死读循环） |
| `TestBuildPluginEnvDoesNotInheritHost` | 宿主设了 `DB_DSN`/`ADMIN_PASSWORD` 时 `buildPluginEnv` 不得带出它们；`extra` 照常追加 |
| `TestBuildPluginEnvOmitsUnsetWhitelistEntries` | 白名单名字存在但宿主未设置 → 不写 `K=` |
| `TestBuildPluginEnvDropsHostEmptyValues` | 宿主设成**空值**时同样不下发（空 `PATH` 会让 node 找不到自己） |
| `TestEnvWhitelistHasNoSensitiveNames` | **白名单本身**不含 `DB_DSN`/`ADMIN_PASSWORD`/`*_SECRET`/`*_KEY`，且必须含 `PATH`（防「把白名单清空当更安全」） |
| `TestBackoffSequence` | 退避阶梯恰为 1s→2s→4s→8s→16s，超出封顶 16s，负数不 panic |
| `TestNewSupervisorDefaults` | `MaxRestarts` 缺省 5（＝五级阶梯）、`Entry` 缺省 `index.js`、`NodePath` 缺省 `node` |
| `TestSupervisorStartFailsWhenNodeMissing` | 可执行文件不存在时 `Start` 返回错误、不 panic、`Stop` 仍安全（**不需要 node，任何平台都真跑**） |
| `TestSupervisorStartsAndHandshakes` | 真拉起 node，`conn.Call` 往返成功 |
| `TestSupervisorChildEnvIsWhitelisted` | **端到端安全防线**：真拉起 node 后读 `process.env` 的键集合，`DB_DSN`/`ADMIN_PASSWORD`/`*_KEY` 不在其中，`PATH` 与 `ExtraEnv` 在。纯函数测试抓不到「spawn 里漏写 `cmd.Env` 赋值」 |
| `TestSupervisorRestartsWithBackoff` | 崩溃后回调被观测到，超限后 `Running()` 为 false |
| `TestSupervisorStopsAfterMaxRestarts` | 崩溃回调次数恰为 `MaxRestarts+1`，且超限后不再重启（只看 `Running()` 抓不到 off-by-one） |
| `TestSupervisorStopUnblocksWedgedWrite` | **kill 是卡死写的唯一解药**：node 不读 stdin 时 `Notify` 永久阻塞，`Stop` 必须能在超时内解开它（删掉 `Stop` 里全部杀进程代码即红） |
| `TestSupervisorStopIsIdempotent` | `Stop` 可重复调用，之后 `Running()` 为 false |
| `TestSupervisorConnReturnsCurrentGeneration` | **换代语义**：重启后 `Conn()` 指针与上一代不同；旧 Conn 拿到 `ErrClosed`；新 Conn 能完成真实往返（把 `Conn()` 改成永远返回第一代即红） |
| `TestSupervisorConnNilBeforeStart` | `Conn()` 在 Start 之前与 Stop 之后返回 nil，不 panic |
| `TestDispatcherDispatchFansOutToAllSubscribers` | 两个插件都收到；IPC 方法名是 `hook.post.created` |
| `TestDispatcherDispatchIgnoresNonSubscribers` | 未订阅该钩子的插件**不得**被调用（订阅判断按钩子而非「有插件就投」） |
| `TestDispatcherDispatchFailureDoesNotBlockOthers` | 一个插件返回错误不影响其他插件收到 |
| `TestDispatcherDispatchDedupesDuplicateHookDeclaration` | manifest 重复声明同一钩子只投一次（按插件为粒度，不按声明条目） |
| `TestDispatcherUnregisterStopsDispatch` | `UnregisterPlugin` 之后不再收到钩子 |
| `TestDispatcherToleratesNilManifestAndNilContext` | nil manifest / nil ctx 不 panic（panic 会掐断整条事件订阅） |
| `TestDispatcherDispatchDoesNotBlockOnSlowPlugin` | **`Dispatch` 在插件完成前返回**（改成同步即红；测试自带 1s 兜底释放，保证是「红」而不是「挂住」） |
| `TestDispatcherDispatchAppliesPerCallTimeout` | 每次调用都有 deadline，且不超过配置上限；到期取消这次调用 |
| `TestDispatcherDispatchSurvivesDeadCallerContext` | **不继承调用方已取消的 ctx**（继承即静默丢钩子） |
| `TestInvertHookEventsGroupsHooksPerEvent` | 一事件多钩子聚合到同一键，且顺序稳定（倒排而非恒等映射） |
| `TestDispatcherSubscribeEventsSubscribesEventDrivenHooksOnly` | 订阅集合**恰好等于** `HookEvents()` 的事件集（7 条，每事件一次）；不含 `cron.daily`、不含 `user.created` |
| `TestDispatcherDeliversContentCreatedToCommentHook` | `content.created` → `comment.created` 真的送达（分叉路径的端到端） |
| `TestDispatcherUserRegisteredFiresOnceOnDoublePublish` | auth 双发 `user.created`+`user.registered` 时，`user.registered` 钩子**恰好一次** |
| `TestDispatcherUserRegisteredFiresOnceOverRealEventbus` | 同上，但走**真 eventbus + 真 NATS**（钉住装配而非只有 fake 语义） |
| `TestDispatcherReachesRealPluginConn` | 经真实 `Conn.Call` 完成一次 IPC 往返，方法名与 payload 都落位 |
| `TestSupervisorCallerFollowsConnectionGeneration` | **`supervisorCaller` 每次重取 `Conn()`**：换代后用新连接（缓存旧指针即红） |
| `TestSupervisorCallerReportsClosedConnection` | 已关闭的连接立刻失败，不空等到 timeout |
| `TestSupervisorCallerWithoutRunningSupervisor` | supervisor 为 nil / 未 Start 时立刻返回 `errPluginNotRunning` |
| `TestDispatcherConcurrentDispatchAndRegistration` | 并发分发与注册/摘除无数据竞争（`-race` 下有效）；分发在锁外调用 |
| `TestRegisterAPIsRejectsUndeclaredCapability` | 只声明 `posts:read` 的插件调 `api.posts.setStatus` 被拒，错误含 `capability`，**service 一次都没被调用** |
| `TestRegisterAPIsAllowsDeclaredCapability` | 声明 `posts:write` 时调用成功，且**恰好一条**审计 |
| `TestRegisterAPIsDoesNotHandleUndeclaredCapability` | 经**真实 IPC** 调用：未声明能力拿到 capability 错误（不是静默、不是超时） |
| `TestRegisterAPIHandlesDeclaredCapabilityOverIPC` | 经真实 IPC：已声明能力真的执行并返回 `{"ok":true}`（防「全都不注册」也能让上一条变绿） |
| `TestLookupAPIFailsClosedWithoutCapabilities` | **nil 与空能力集拒绝每一个写类 API**，且不触达 service / KV / 审计 |
| `TestWriteAPIsAreAudited` | 每一个写类 API 成功后**恰好**多一条审计，且带插件标识 |
| `TestWriteAPIAuditFailureDoesNotFailTheCall` | 审计失败不把已生效的写动作翻成失败（否则插件重试会再写一次） |
| `TestWriteAPIsRejectMissingRequiredFields` | 13 组缺字段参数全部报错，且 service / KV / 审计一个都没被碰 |
| `TestKVRejectsKeysEscapingPluginNamespace` | `api.kv.get`/`set` 对 `../../x`、`/abs`、`plugin:other:k`、`sub\\..\\..\\x`、空 key 等 8 种形态全部拒绝，存储零调用 |
| `TestKVScopesKeysUnderPluginPrefix` | 存储实现收到的 key **已是** `plugin:alpha:counter`（作用域在 api.go 拼，不靠插件自觉） |
| `TestKVIsolatesPluginsFromEachOther` | beta 读不到 alpha 的同名键；beta 点名 `plugin:alpha:shared` 被拒绝 |
| `TestKVRejectsUnsafePluginIDs` | 空/含 `:`/含 `/`/含 `..`/带空白的 pluginID 被拒绝（否则命名空间可能与其他插件重合） |
| `TestKVCapabilitiesAreIndependent` | `kv:write` 不隐含 `kv:read`，反之亦然 |
| `TestEveryAPIMethodDeclaresACapability` | 每个 API 都登记了**已知**能力（漏写 cap 会让该 API 不可调，是刻意的 fail-closed） |
| `TestAPIMethodNamesMatchLookup` | `apiMethodNames()` 与 `lookupAPI` 严格一致；写类清单是其子集；方法名不重复 |
| `TestKnownCapabilitiesAreSortedAndKnown` | `KnownCapabilities()` **排序**（连取 20 次比对，抓未排序）、内容与 9 个常量逐一对应、重复项为零 |
| `TestCapabilitySetFromNamesFeedsManifestValidate` | `CapabilitySet` 能**直接**喂 `Manifest.Validate`；未知能力仍报错；nil/空 knownCaps 仍拒绝 |
| `TestCapabilityErrorsAreIdentifiable` | 权限拒绝可被 `errors.Is(err, ErrCapabilityDenied)` 识别（插件据此区分「没申请」与「宿主出错」） |
| `TestAuditTargetComesFromParamDeclarations` | 审计的 target/detail 来自**同一份** `params`（读 `post_id` / `status`），不是第二份匿名 struct |
| `TestWriteSpecsDeclareAudit` | **写类规格的 `audit` 必须非零、`Action` 非空；读类必须为零**。这才是「写类必须有审计」的担保，`TestWriteAPIsAreAudited` 只数条数，查不出 `action` 退化成方法名 |
| `TestAuditFieldReferencesExistInParams` | `audit.Target`/`Detail` 引用的名字必须真实存在于该规格的 `params` 里（改字段名忘了同步 audit 时，`joinArgs` 只会静默取到空串） |
| `TestEveryCapabilityUsedByAnAPI` | **反向校验**：每个已声明能力都被 ≥1 个 API 使用；空能力必须在 `capabilityWithoutAPI` 白名单里。此前 `posts:read`/`comments:read` 一直是空的，安装确认页批准的是不存在的能力 |
| `TestCapabilityWithoutAPIListIsAccurate` | 白名单不得含已被使用的能力、不得是幽灵条目；**key set 是 `{posts:read, comments:read}` 的子集**——用 `require.Subset` 而非 `ElementsMatch`，因为收缩无需改测试、扩张必须显式改断言 |
| `TestStatusEnumMatchesModelConstants` | 插件侧 `status` 枚举字面量与 `model.PostStatus*` 常量一致（内部测试 import model，不影响 `go list -deps` 的 gorm 计数——它只算非测试构建图） |
| `TestAPISchemasCoverAllMethods` / `TestSchema*` | schema 与 `params` 同源：覆盖全部方法、`required` 与声明一致、无必填时不写 `required`、`Enum` 进文档、类型映射、根是 object 且带 description、结果可序列化 |
| `TestSchemaTypePanicsOnUnmappedFieldType` | 新增 `FieldType` 忘了同步 `schemaType` 时 panic。这条只能单独测：`apiSpecs` 里每个类型都有显式 case，`TestAPISchemasCoverAllMethods` 永远走不到 default 分支 |
| `TestDecodeArgsNoTrimPreservesValue` / `TestDecodeArgsNoTrimStillRejectsBlank` | `NoTrim` 字段的**值**原样保留（`" k"` 不变成 `"k"`，否则 `pluginKVKey` 的空白拒绝被绕过）；但**纯空白仍算缺失**——`NoTrim` 只影响存储与传递，不影响校验 |
| `TestArgsIntStoresValue` / `TestArgsRawStoresObject` | `FieldInt` 真的落进 `Args.Int`、`FieldObject` 真的落进 `Args.Raw`（取值器全覆盖，不只是 `String`） |

`TestDispatcherDispatchDoesNotBlockOnSlowPlugin` 有一个刻意的设计：它用 `time.AfterFunc`
在 1s 后无条件释放插件。**没有这个兜底，把 `go func` 改成同步调用会让测试永久挂住**，
在 CI 里表现为整包超时而不是一条可读的失败——「挂住 ≠ 变红」，调试成本差一个量级。

`TestDispatcherUserRegisteredFiresOnceOnDoublePublish`（fake）与 `...OverRealEventbus`（真总线）
是**刻意重复的一对**：fake 版本能精确断言「订阅了哪些事件」，真总线版本能抓住「接缝装错」
（例如 `SubscribeEvents` 拿到 nil broadcaster 却不报错）。只留 fake 会漏装配，只留真总线
则在 NATS 抖动时会偶发失败。

`TestSupervisorCallerFollowsConnectionGeneration` 是 Task 4 换代语义在**消费侧**的对应物：
supervisor 侧的 `TestSupervisorConnReturnsCurrentGeneration` 只证明 `Conn()` 会换代，
不会发现分发器缓存了 `*Conn`。把 `supervisorCaller` 改成「首次调用时缓存」即红。

supervisor 的测试**分两层**，缺一层就有盲区：`buildPluginEnv`/`backoffDelay` 是纯函数，
断言精确但抓不到装配错误；node 端到端测试抓装配，但依赖 PATH 上有 node，缺失时
`requireNode` 会 `t.Skip`。**看到 skip 不等于通过**——`TestSupervisorStartFailsWhenNodeMissing`
是唯一不依赖 node 的 spawn 路径覆盖，因此任何环境都保留一条真实的进程路径回归。

`DropsUnknownHooks` 与 `RejectsUnknownCapabilities` 是**刻意相反**的一对：任何让两个方向趋同的
改动（「未知的都丢弃」或「未知的都报错」）都会让其中一个变红。

`TestConnCallTimesOut` 与 `TestConnUnknownMethodReturnsError` 是 IPC 侧的对应一对：
「对端不回」必须落到 ctx 超时，「对端不认识这个方法」必须立刻拿到错误。
两者的区别只在于对端 handler 是否存在——所以超时测试**不能**用「对端没注册 handler」
来制造超时（那样会立刻收到 unknown method 错误帧），必须让已注册的 handler 阻塞不返回。

## 改这里要注意

- 新增钩子：三处**同一次改动**补齐——`knownHooks`（否则被 `ParseManifest` 当未知名丢掉，
  症状是插件静默收不到事件、不报错）、`hookEvents`（否则 `TestHookEventsCoversEveryHookExceptCron` 红）、
  钩子常量。同时确认 `internal/pkg/eventbus/events.go` 里**真的有**对应事件常量；
  没有就是 I1 的 `post.updated` 场景，别加这个钩子。
  同期更新 `docs/design/extension-points.md` 的事件目录与 `packages/api-client` 等消费方。
- 新增钩子时**优先复用现成事件**再考虑新增事件：`comment.created` → `content.created` 就是这么来的。
  插件侧的语义名可以更自然，传输侧不必跟着改。
- 新增 capability：在 `api.go` 的 `knownCapabilities` 登记，并在后台的能力清单里展示——插件安装前
  管理员看的就是这份清单。**同时必须让至少一个 `apiSpec` 要求它**，否则它会出现在安装确认页
  却没有任何 API 使用（`TestEveryAPIMethodDeclaresACapability` 查的是反方向：每个 API 都登记了已知能力；
  `TestEveryCapabilityUsedByAnAPI` 查这一方向）。
  真想先声明后实现，只能把能力加进 `capabilityWithoutAPI` 白名单——**那张表只许变短**，
  增长必须同步改 `TestCapabilityWithoutAPIListIsAccurate` 的 `require.Subset` 断言并说明理由。
- 新增宿主 API：**只加 `apiSpecs` 一条**（method / cap / write / params / audit / build）。
  参数写进 `params []Field`，审计写进 `AuditSpec`——**不要再写第二个匿名 struct 或
  `json.Unmarshal` 提取审计字段**，那正是 2026-11-06 重构消掉的失效模式。
  `apiMethodNames()` 之类的字面量清单全部从规格表推导，不要手工维护。
  测试里补一条 `writeAPISampleParams`（写类），`TestWriteAPIsAreAudited` 会自动纳入覆盖；
  `TestWriteSpecsDeclareAudit` 与 `TestAuditFieldReferencesExistInParams` 会自动校验新规格的审计口径。
- 不要给 `Field.Enum` 补运行期校验。取值域的权威定义在 service 层（`isValidPostStatus`），
  在插件层再校验一遍会制造第二个事实来源：加枚举值时改 service 忘了改这里，
  症状是「文档说支持、实际被拒」。`Enum` 只进 JSON Schema（见「参数校验」一节）。
- 给字段加 `NoTrim` 前先确认它真的需要原值。目前只有 KV 的 `key`/`value` 与
  通知的 `body` 需要：`pluginKVKey` 靠 `trimmed != key` 拒绝首尾空白，
  预修剪会让 `" k"` 静默通过，等于放宽 KV 命名空间隔离。
- 新增 `FieldType` 时必须**同时**改 `schema.go` 的 `schemaType` 与 `decodeArgs` 的 switch：
  `schemaType` 对未映射类型 panic（构建期暴露），`decodeArgs` 静默退化成字符串校验。
  两者不同步的症状是「文档写 object、实际拒绝非字符串」——错得一致且无声。
- 放宽 `pluginKVKey` 的任何一条拒绝规则都等于放宽插件 KV 隔离：`..` / 空段 / 绝对路径 / 含 `:`
  这四类放开后，一个插件能读到另一个插件的键。改前先加测试再改实现。
- 把能力校验从 `lookupAPI` 挪进某个 handler 是本文件最反对的改动：那会把
  「一个地方会错」变成「每个 handler 都可能错」。若确需在 handler 内二次校验，
  只能作为**额外的**纵深防御，不能删掉 `lookupAPI` 里那道。
- 写类 API 的审计只在 `apiSpec.adapt` 里落。若新增一条绕过 `adapt` 的注册路径，
  「所有写类 API 必须审计」这条 spec 就失去唯一保证。
- 提升协议版本：`ProtocolVersion1` 是**唯一**可接受值，没有「兼容旧版本」分支。真要加 v2，
  必须同时定义 v1 的兼容策略，否则所有存量插件的 manifest 会在升级瞬间全部被拒。
- `internal/plugin` 一旦 import 了 `internal/repository`，分层约束与 spec 同时失效
  （**无 CI 门禁**，只能靠手工跑上面那条 `go list -deps` 命令）。需要数据时注入接口，不要把仓储拉进来。
- **别在本包 import `internal/pkg/eventbus`**（或任何间接依赖 `internal/db` 的包）。
  它会带进 10 个 gorm 包。接线放 `pluginevent` 子包，本包只认 `Broadcaster` 接口。
- 改分发语义前先想清楚「谁在调用 `Dispatch`」：它是事件总线的广播 goroutine。
  任何让 `Dispatch` 等插件的改动（同步调用、`wg.Wait()`、无缓冲 channel）都等于
  让一个卡死的插件拖住后续所有同类型事件——这正是 `Dispatch` 异步 + 每次调用独立超时的理由。
- `hookTimeout` / `dispatcher.timeout` 是**上限**，不是「等这么久」。插件正常返回时立刻结束。
  缩短它会让慢插件被频繁掐断（症状：插件侧 handler 跑到一半 ctx 取消）；加长它则会延长
  goroutine 占用。改默认值前先确认为什么 5s 不够。
- 分发失败**只记日志、不重试、不入队**。加任何重试都在把「事后通知」变成「至少一次投递」，
  需要幂等键与去重存储——那是新特性，不是修 bug。
- 改 `validateEntry` 等于改插件的可执行入口约束：放宽任何一条都意味着 manifest 能指定插件目录外的文件。
  若将来要支持子目录入口（合法需求），放行的是「不带 `..` 的相对路径」，不是「任意路径」。

## 本文件何时更新

- 新增/删除文件（上方两张文件表各一行）
- 钩子清单增减、**钩子→事件映射变化**、协议版本策略变化
- `entry` 校验规则变化（放行/拒绝哪些路径形态）
- capability 校验语义变化（哪一半报错、哪一半丢弃）
- **新增/删除宿主 API 方法**（`apiSpecs` 一条即一处契约；插件安装确认页、JSON Schema 与审计动作名都会变）
- **`Field` / `Args` / `decodeArgs` 的语义变化**（必填判定、`AllowEmpty`、`NoTrim`、错误文案）
- **`AuditSpec` 的形态变化**（字段引用 vs 提取闭包、`AuditScope` 取值、target/detail 的拼接方式）
- **`APISchemas()` 的输出变化**（字段名、类型映射、`required` 与 `enum` 的呈现）
- **`capabilityWithoutAPI` 增减**（空能力清单是管理员装机确认页的事实来源）
- **KV 命名空间规则变化**（前缀形态 `plugin:<id>:`、拒绝的 key 形态、pluginID 校验）
- **写类 API 的审计口径变化**（在哪一层落、失败是否翻转结果、`action`/`target` 的取值）
- 安全边界表述变化（同用户运行、环境变量白名单）
- **IPC 帧格式或请求/响应关联规则变化**（`Frame` 字段、方向判定依据、`id` 分配与丢弃策略）
- 流的**所有权**或 supervisor 关停顺序变化；写入从同步改成异步（或反之）会改变「谁能解开一个卡死的 `writeMu`」
- **`envWhitelist` 增减**（安全边界的变化，且在 `TestEnvWhitelistHasNoSensitiveNames` 的断言里）
- **退避阶梯或 `MaxRestarts` 语义变化**（阶梯本身、封顶值、`MaxRestarts+1` 的崩溃计数口径）
- supervisor 的**管道构造方式**变化（换回 `StdinPipe`/`StdoutPipe` 会重新引入与 `Wait` 的关闭竞态）
- 新增对外导出的类型/函数（`Manifest` 字段、`ParseManifest`/`Validate` 签名、`HookEvents`/`HookIsEventDriven`、`Conn`/`Frame` 的方法集、`Supervisor`/`SupervisorOptions` 的字段）
- **`Conn()` 的语义变化**（返回哪一代、是否为 nil、是否缓存）——Task 7 的钩子分发直接依赖它
- **分发语义变化**（异步/同步、超时值与来源、是否继承调用方 ctx、失败是否重试）
- **事件桥接变化**（用倒排还是恒等映射、订阅哪些事件、`Broadcaster`/`TraceBroadcaster` 接缝形态）
- **`internal/plugin` 的依赖图里出现 `internal/pkg/eventbus` 或 `gorm`**（分层被破坏，且无 CI 门禁）
- 新增 `pluginevent` 之类的接线子包，或改变「接口留在本包、接线放子包」的分工
- 「刻意不做的事」两条取舍（进程组管理、`exit 0` 算崩溃）被改变
