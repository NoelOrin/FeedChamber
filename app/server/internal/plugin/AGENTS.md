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
   这条有自动化门禁：`go list -deps ./internal/plugin | grep repository` 必须为空
   （规范见 `openspec/changes/2026-11-04-plugin-hooks/specs/layered-architecture/spec.md`）。

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
v1 的 9 个钩子：`post.created` `post.updated` `post.closed` `comment.created`
`user.registered` `user.followed` `report.created` `moderation.decided` `cron.daily`。

v1 **只做异步钩子**，MUST NOT 有 `*.before_*` 形态的写路径前置钩子：钩子在业务动作完成之后触发，
插件失败/超时/崩溃 MUST NOT 回滚已完成的业务动作。需要前置拦截的场景走新增 `moderation_mode` 档位，
MUST NOT 插进现有审核链中间。

## 文件

| 文件 | 职责 |
|------|------|
| `manifest.go` | `Manifest` 结构、`ParseManifest`、`(*Manifest).Validate`、`ProtocolVersion1`、9 个钩子常量与白名单 |
| `manifest_test.go` | 解析/校验的契约：合法 manifest、未知协议版本、缺 name/version、未知钩子丢弃、未知能力报错、坏 JSON |

后续任务追加（同一包，**内部测试用 `package plugin`、外部契约测试用 `package plugin_test`**，两者可共存）：

| 计划文件 | 职责 |
|----------|------|
| `ipc.go` | stdio JSON-RPC 编解码（请求/响应/通知三种帧） |
| `supervisor.go` | `os/exec` 拉起 `node`、进程组管理、退避重启（1s→2s→4s→8s→16s，上限 5 次） |
| `hooks.go` | 钩子注册与分发（按事件类型 fan-out 到各插件） |
| `api.go` | 宿主 API 注册与 capability 校验 |
| `host.go` | 生命周期状态机（`installed`/`starting`/`running`/`crashed`/`stopped`） |

## manifest 的校验分工

`ParseManifest` 只做**不需要外部输入**的校验：JSON 合法性、`name`/`version` 非空、
`protocol_version` 必须等于 `ProtocolVersion1`、`entry` 缺省为 `index.js`、钩子白名单过滤。

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

`DropsUnknownHooks` 与 `RejectsUnknownCapabilities` 是**刻意相反**的一对：任何让两个方向趋同的
改动（「未知的都丢弃」或「未知的都报错」）都会让其中一个变红。

## 改这里要注意

- 新增钩子：`knownHooks` 与钩子常量**同一次改动**补齐，否则新钩子会被 `ParseManifest` 当未知名丢掉，
  症状是插件静默收不到事件（不报错）。同期更新 `internal/pkg/eventbus/events.go`
  的对应事件常量与 `docs/design/extension-points.md` 的事件目录。
- 新增 capability：在 `api.go` 的 `knownCapabilities` 登记，并在后台的能力清单里展示——插件安装前
  管理员看的就是这份清单。
- 提升协议版本：`ProtocolVersion1` 是**唯一**可接受值，没有「兼容旧版本」分支。真要加 v2，
  必须同时定义 v1 的兼容策略，否则所有存量插件的 manifest 会在升级瞬间全部被拒。
- `internal/plugin` 一旦 import 了 `internal/repository`，分层门禁与 spec 同时失效。
  需要数据时注入接口，不要把仓储拉进来。

## 本文件何时更新

- 新增/删除文件（上方两张文件表各一行）
- 钩子清单增减、协议版本策略变化
- capability 校验语义变化（哪一半报错、哪一半丢弃）
- 安全边界表述变化（同用户运行、环境变量白名单）
- 新增对外导出的类型/函数（`Manifest` 字段、`ParseManifest`/`Validate` 签名）
