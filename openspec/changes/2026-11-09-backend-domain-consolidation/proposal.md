# 后端领域收拢：技术分层 → 领域骨架 + 域内分层

提案状态：**草案（待评审）** · 创建日期：2026-11-09
设计文档：[docs/superpowers/specs/2026-11-09-backend-domain-consolidation-design.md](../../../docs/superpowers/specs/2026-11-09-backend-domain-consolidation-design.md)
前序：`2026-10-14-tidy-backend-architecture`（已归档）清理了死码、统一了响应信封、把装配拆成 `New`/`Start`，但**没有触及目录结构**。

## Why

`app/server/internal` 现在按**技术层**切包，而领域边界只活在文件名前缀里。实测（2026-11-09）几个直接后果：

1. **改一个功能要横跨四个平铺大包。** 「发帖」涉及 `handler/post.go`、`service/post.go`(1164 行)、`service/gate.go`、`service/trust_level.go`、`service/moderation.go`、`repository/post_sql.go`、`model/post.go`——8 个文件横跨 4 个目录。`internal/service` 现有 **91 个生产文件 / 16094 行**，`internal/handler` **48 / 10684 行**。

2. **依赖关系不可读，也没有被任何机制强制。** `postService` 有 18 个构造依赖；因为都在同一个包里，新增一条跨域调用不会被编译器或 CI 拦下。我把 43 个 Service 接口的依赖边全解析后确认：**依赖图本身是无环的**（自然分成 core → identity → growth → engagement → content → ops 六层），现有代码用「结构体字段声明为接口」已经在做解耦——只是接口和实现住同一个包，这层解耦没有被兑现成边界。

3. **架构元信息无法自维护。** `docs/backend-overview.md`（2026-10-27 实测版）记载 253 条路由、55 个 AutoMigrate 模型、service 9876 行；2026-11-09 实测为 **351 条（315 POST + 36 GET）、73 个、16094 行**。不是文档作者不用心，而是「域」在代码里不存在，只能靠人肉维护一张影子地图，于是必然漂移。

4. **迁移成本已被测出，且不高。** 全量扫描「定义在某文件、被同包其他文件调用」的非导出函数，只有 **10 个**。也就是说拆包的真实成本是「搬 139 个文件 + 处置 10 个 helper + 归位 40 个 `_iface.go`」，而不是重写逻辑。

## What Changes

### 目标结构

- 每个业务域一个 `internal/<domain>/`，域内保留 `service/` 与 `handler/`：

```
internal/
├── core/        setting permission session(iface) email audit mcpclient    # T1
├── identity/    auth mfa totp user role group invite banned oauth           # T2
├── growth/      trust level gate visibility checkin badge leaderboard       # T3
├── engagement/  like favorite follow watch block notification pm report     # T4
├── content/     post content tag space draft revision moderation mention    # T5
├── ops/         admin announcement backup webhook plugin motion customize   # T6
│                theme upload slot (+ dashboard seo realtime trash)
├── gateway/ middleware/ plugin/ mcp/ authz/ mock/        # 形态不变
├── model/ repository/                                     # 共享内核，不拆
└── pkg/ config/ db/ storage/ mailer/ metrics/             # 技术底座，不动
```

- **域间依赖单向**：`core → identity → growth → engagement → content → ops`。跨域只走提供方域的 Service 接口，MUST NOT 直接持有对方域的 repository。
- **handler 层边界不退化**：`<domain>/handler` 仍不可能 import `repository`/`db`（Go 包边界给出编译期保证）。
- **共享内核不动**：`model` 与 `repository` 保持扁平。`User`/`Post`/`Space` 被几乎所有域引用，`UserRepository` 被 20+ 服务注入；拆它们会制造大量「为了用一个类型而反向依赖上层域」的假边，把干净的 DAG 变成网状图。它们本来就是纯数据 + 数据访问接口，符合「共享内核」定义。
- **接口归属跟随提供方域**：`TrustService` 住在 `growth/service`，由 content/engagement import。不建中立 `ports` 包——实测依赖是 DAG，建中立接口包只会把「谁依赖谁」再藏起来一次。
- **handler 共享工具** `ctx.go` / `params.go` 移入 `internal/pkg/httpx/`。

### 唯一跨界边的处置

`moderationService → MCPServerService` 是 T5→T6 的逆边，现状为 setter 注入的可选依赖（`s.mcpSvc == nil` 时走 HTTP 兜底）。语义上它是**出站适配器**而非业务域，故连同 `mcp_server_iface.go` 移入 `core/mcpclient`，该边变为 T5→T1，合法。

### 强制机制

新增 import 边界测试（`internal/app/importboundary_test.go`），静态解析 import 图并断言：域顺序单向、域内 `service` 不 import `handler`、任何 `handler` 不 import `repository`/`db`。失败信息输出「文件 → 被禁包 → 规则号」。

### 迁移方式

strangler，**一域一提交，任何中间状态 `go build ./...` 必须通过**。顺序：`core` → `growth`（试点，同时有出边与入边，能一次验证两个方向）→ `identity` → `engagement` → `content`（最大最晚）→ `ops`。

### 明确不做

- 不改业务逻辑、SQL、API、数据库 schema、权限码。
- 不拆 `model` / `repository`。
- 不把单体拆成服务。
- `internal/app/`（container/router/wiring 收拢 + `iris.go` 拆分）**不在本次范围**，见 proposal 末尾「待拍板」。
- 前端后台 CRUD 脚手架是独立 change（`app/web` 另一栈、另一套评审关注点）。

## Capabilities

### New Capabilities

（无）

### Modified Capabilities

- `layered-architecture` — 「单向分层依赖链」由「技术层」扩展为「领域骨架 + 域内分层」，并明确 `model`/`repository` 为共享内核的例外；新增「域间依赖方向由 import 边界强制」要求（含规则号与失败形态）。
- `module-conventions` — 「模块分层文件组织」与「新增模块改动清单」中的路径由 `internal/service/xxx.go` 等改为 `internal/<domain>/service/...`，并补「新模块落在哪个域」的判定规则。

## Impact

- **代码**：`app/server/internal` 内 `service`(91) + `handler`(48) 两个包的文件搬迁；`container` / `router` / `server/wiring.go` 的 import 改指；新增一个边界测试与 `pkg/httpx`。**无业务逻辑变更。**
- **对外契约**：URL、请求/响应形状、权限码、数据库 schema **全部不变**；`openapi.json` 在迁移前后 MUST 字节级一致（sha256 相等），这是本次的第一验收项。
- **文档**：45 份模块 `AGENTS.md` 需按新目录重写/合并（`agents-docs` spec 要求「改代码必须同步文档」，属 P4 单独阶段）；`docs/ARCHITECTURE.md`、`system-map.md`、`backend-overview.md` 同步实测数字。
- **风险**：主要风险是「搬家时顺手改坏语义」。缓解：一域一提交、`git diff -M` 确认纯重命名、生产行数净变化为 0、`openapi.json` 哈希不变、`feedchamber_test.sh -all` 对同一套 curl 断言。
- **诚实声明**：`internal/service` 有 28 个测试，`internal/handler` 只有 1 个（还是指标埋点），**没有行为级覆盖**。所以安全网只有编译期 + openapi 哈希 + 全量 curl 脚本三条，这也是把「一次一个域」写成硬规则的原因。

## 待拍板（评审时请明确）

1. `internal/app/` 装配收拢是否纳入本 change。收益是「装配不再是第 7 个域」，成本是再多搬 20+ 文件；建议拆成独立 change。
2. `core` 的成员清单：当前 6 个（setting / permission / session-iface / email / audit / mcpclient）。`mcpclient` 算 core 有争议，替代方案是并入现有 `internal/mcp`（但那包是**入站** server，语义相反）。
3. 10 个共享 helper 里有 2 个（`truncate`、`mapRepoErr`）建议移入 `internal/pkg`。代价是 `pkg` 继续变大；替代方案是接受域内少量重复。
