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
   ```

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
- `cron.daily` 由 `TrustService` 的每日 ticker 触发，不经事件总线。
  `HookIsEventDriven("cron.daily") == false`，Task 5 据此把它排除在事件订阅之外。
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

后续任务追加（同一包，**内部测试用 `package plugin`、外部契约测试用 `package plugin_test`**，两者可共存）：

| 计划文件 | 职责 |
|----------|------|
| `supervisor.go` | `os/exec` 拉起 `node`、进程组管理、退避重启（1s→2s→4s→8s→16s，上限 5 次） |
| `hooks.go` | 钩子注册与分发（按事件类型 fan-out 到各插件） |
| `api.go` | 宿主 API 注册与 capability 校验 |
| `host.go` | 生命周期状态机（`installed`/`starting`/`running`/`crashed`/`stopped`） |

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
| `Notify` | **调用方 goroutine** | **调用方被阻塞** | 只有 `Close` 或杀掉对端进程 |
| `dispatch` | **该请求的 goroutine** | 该 dispatcher goroutine 被阻塞 | 只有 `Close` 或杀掉对端进程 |

即：**一个卡死的 `writeMu` 只能靠 `Close`/杀进程解除**。Task 4 的
「超时 → kill 子进程」正是这个兜底，supervisor MUST 保留它。删掉 kill 退路后，
一个不读 stdin 的插件能把宿主的调用方 goroutine 一条条耗光。

注意 `Call` 里那个写 goroutine **不保证会退出**：它只在 `Write` 返回后才结束，
若对端永久卡住且 `w` 不是 `io.Closer`，它就被永久 pin 住。容量 1 的缓冲只保证
它回送结果时不阻塞、不泄漏 channel，不是「goroutine 一定会回收」的保证。

### 流的所有权与 supervisor 关停顺序

**`Conn` 接管 `r`/`w` 的所有权，`Close` 会主动关闭它们**（实现了 `io.Closer` 时）。
这是解开阻塞 `Write`/`Scan` 的唯一手段。代价是：调用方 MUST NOT 与 `Close` 并发地
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
  管理员看的就是这份清单。
- 提升协议版本：`ProtocolVersion1` 是**唯一**可接受值，没有「兼容旧版本」分支。真要加 v2，
  必须同时定义 v1 的兼容策略，否则所有存量插件的 manifest 会在升级瞬间全部被拒。
- `internal/plugin` 一旦 import 了 `internal/repository`，分层约束与 spec 同时失效
  （**无 CI 门禁**，只能靠手工跑上面那条 `go list -deps` 命令）。需要数据时注入接口，不要把仓储拉进来。
- 改 `validateEntry` 等于改插件的可执行入口约束：放宽任何一条都意味着 manifest 能指定插件目录外的文件。
  若将来要支持子目录入口（合法需求），放行的是「不带 `..` 的相对路径」，不是「任意路径」。

## 本文件何时更新

- 新增/删除文件（上方两张文件表各一行）
- 钩子清单增减、**钩子→事件映射变化**、协议版本策略变化
- `entry` 校验规则变化（放行/拒绝哪些路径形态）
- capability 校验语义变化（哪一半报错、哪一半丢弃）
- 安全边界表述变化（同用户运行、环境变量白名单）
- **IPC 帧格式或请求/响应关联规则变化**（`Frame` 字段、方向判定依据、`id` 分配与丢弃策略）
- 流的**所有权**或 supervisor 关停顺序变化；写入从同步改成异步（或反之）会改变「谁能解开一个卡死的 `writeMu`」
- 新增对外导出的类型/函数（`Manifest` 字段、`ParseManifest`/`Validate` 签名、`HookEvents`/`HookIsEventDriven`、`Conn`/`Frame` 的方法集）
