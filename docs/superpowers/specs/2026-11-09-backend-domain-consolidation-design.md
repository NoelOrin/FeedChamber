# 后端领域收拢：把「技术分层」变成「领域骨架 + 域内分层」

- 状态：**草案（待评审，未动代码）**
- 日期：2026-11-09
- 关联提案：[openspec/changes/2026-11-09-backend-domain-consolidation](../../../openspec/changes/2026-11-09-backend-domain-consolidation/proposal.md)
- 前序变更：`2026-10-14-tidy-backend-architecture`（已归档，清理死码/统一信封/装配三段，**不涉及目录重组**）
- 证据来源：本文所有数字与依赖关系均为 2026-11-09 对 `app/server` 源码实测（脚本口径见 §1.4），不转述既有文档

---

## 1. 为什么做（实测证据）

### 1.1 现状规模

| 目录 | 生产文件 | 生产行数 | 备注 |
|---|---:|---:|---|
| `internal/service` | 91 | 16094 | 其中 40 个是 `_iface.go`，其余是混在一起的实现 |
| `internal/handler` | 48 | 10684 | `handler.go` 是 39 个 handler 的聚合体 + `Deps` 38 个字段 |
| `internal/repository` | 106 | 6830 | 19 个资源已拆 `_sql.go`，其余接口与实现同文件 |
| `internal/model` | 60 | 3578 | 纯数据 |
| `internal/gateway` | 18 | 3899 | 已是独立子系统的正确形态 |
| `internal/router` | 12 | 946 | `router.go` + 11 个 `routes_*.go` |
| `internal/container` | 4 | 694 | |
| `internal/middleware` | 9 | 756 | |
| `internal/pkg` | 21 | 2278 | |

`internal/service` + `internal/handler` 合计 **139 文件 / 26778 行**，是这次要重组的对象。其余目录不动。

### 1.2 三个可验证的坏味道

**（a）改一个功能要横跨四个平铺大包。** 以「发帖」为例：入口在 `handler/post.go`，逻辑在 `service/post.go`（1164 行），同时散落到 `service/gate.go`（门控）、`service/trust_level.go`（XP 回收）、`service/moderation.go`（审核档位）、`service/mention.go`（@提及扇出），数据在 `repository/post_sql.go`（605 行），模型在 `model/post.go`。想知道「发帖会碰哪些东西」，需要读 6 个不同目录下的 8 个文件。

**（b）依赖关系不可读。** `postService` 有 **18 个构造依赖**（6 个 repository + 8 个服务/检查器 + KV/Bus/Store）。因为都在同一个 Go 包里，没有编译器帮你判断「这个依赖是域内还是跨域」「跨域方向对不对」——91 个文件平铺时，任何人新增一条 `xxxSvc.Foo()` 调用都不会被任何机制拦下。

**（c）架构元信息已经无法自维护（这是最硬的证据）。** 同一份 `docs/backend-overview.md`（2026-10-27 实测版）与今天的代码对比：

| 文档记载 | 2026-11-09 实测 |
|---|---|
| 253 条路由注册 | `315` 处 `.Post("` + `36` 处 `.Get("` = **351** |
| 55 个 `AutoMigrate` 模型 | **73** 个 `model.*` 参数 |
| `internal/service` 9876 行 | **16094** 行 |

不是文档作者不用心，而是「域」这个概念在代码里不存在，只能靠人肉在文档里维护一张影子地图，于是必然漂移。

### 1.3 为什么这不是「推倒重写」（可行性实测）

我把 `internal/service` 里 43 个 Service 接口与 40 个实现结构体的字段依赖全部解析出来，得到接口级依赖图：

**结论一：依赖图是无环的（DAG），并且自然分成 6 层。** 现有代码其实已经通过「结构体字段声明为接口」在做解耦了，只是接口和实现住在同一个包里，这层解耦没有被编译器兑现成边界。

| 层 | 代表服务 | 实测依赖方向 |
|---|---|---|
| T1 core | `SettingService` `PermissionChecker` `SessionStore` `EmailService` `AuditLogService` `MCPServerService` | 只依赖 T0（repository/kv/db） |
| T2 identity | `UserService` `AuthService` `MFAService` `RoleService` `GroupService` `InviteService` `BannedService` `OAuthService` | → T1 |
| T3 growth | `TrustService` `GateService` `CheckInService` `BadgeService` | → T1 |
| T4 engagement | `LikeService` `FavoriteService` `FollowService` `WatchService` `UserBlockService` `NotificationService` `PMService` `ReportService` | → T1 T2 T3 |
| T5 content | `PostService` `ContentService` `TagService` `SpaceService` `DraftService` `RevisionService` `ModerationService` | → T1 T2 T3 T4 |
| T6 ops | `AdminService` `AnnouncementService` `BackupService` `WebhookService` `PluginService` `MotionService` `CustomizeService` `ThemeService` `UploadService` | → T1 T2 T4 T5（读） |

只有一条边违反这个分层：`moderationService.mcpSvc`（T5 → T6 里的 `MCPServerService`）。它是 **setter 注入的可选依赖**（`SetMCPServerService`，`s.mcpSvc == nil` 时走 HTTP 兜底），处置方式见 §4.3——把 MCP 客户端适配器归入 T1。

**结论二：跨文件共享的未导出 helper 只有 10 个。** 全量扫描「定义在某文件、被同包其他文件调用」的非导出函数，结果是：

| 函数 | 定义处 | 被谁用 |
|---|---|---|
| `displayNameOf` | `auth.go` | content, follow, like, pm |
| `gateInputForUser` `gateDenyMessage` | `gate.go` | content, post |
| `publishUserMentions` | `mention.go` | content, post |
| `validateDisplayName` | `auth.go` | user |
| `isValidPostStatus` | `post.go` | content |
| `truncate` `mapRepoErr` | `customize.go` | mcp_server |
| `buildBackupSnapshot` `restoreBackupSnapshot` | `backup_snapshot.go` | backup |

也就是说：**拆包的真实成本不是「重写逻辑」，而是「搬 139 个文件 + 处置这 10 个函数 + 把 40 个 iface 归位」**。这就是本方案选择「原地重组」而不是「另起新架构」的依据。

### 1.4 测量口径

```bash
# 规模
find internal/$d -name '*.go' ! -name '*_test.go' -exec cat {} + | wc -l
# 路由
grep -rho '\.Post("' internal/router | wc -l      # 315
grep -rho '\.Get("'  internal/router | wc -l      # 36
# AutoMigrate
sed -n '/store.AutoMigrate(/,/)/p' server/iris.go | grep -c 'model\.'   # 73
# 接口级依赖图 / 共享 helper：对 service 包做 AST 级正则解析（脚本见 change 的 tasks.md P0）
```

---

## 2. 目标与非目标

### 2.1 目标（每条都可机器验证）

1. **域即目录**：每个业务域一个 `internal/<domain>/`，域内保留 `service/` 与 `handler/`；「发帖」这类功能的所有代码落在一处（跨域影响面仍是显式的 import 行）。
2. **依赖方向可编译期强制**：域间只允许 T1→T6 单向引用，由一条 import 边界测试钉住；违反即 CI 红。
3. **层边界不退化**：`handler` 仍然 **不可能** 直接 import `repository`/`db`（保持现有 Go 包边界提供的编译期保证）。
4. **对外契约零变化**：URL、请求/响应形状、权限码、数据库 schema、`openapi.json` 内容全部不变。
5. **文档可自维护**：目录结构直接表达域，`backend-overview.md` 不再需要维护「影子域地图」。

### 2.2 非目标

- 不改业务逻辑、不改 SQL、不改 API、不引入新框架。
- 不拆 `internal/model`、不拆 `internal/repository`（理由见 §4.2）。
- 不把单体拆成服务/进程；`internal/gateway`、`internal/plugin`、`internal/mcp` 维持现有形态。
- 不动 `internal/pkg`、`internal/config`、`internal/db`、`internal/storage`、`internal/mailer`、`internal/metrics`。
- 前端后台脚手架（`app/web/app/admin` 的 CRUD 去重）**不在本 change 内**，见 §9。

---

## 3. 目标结构

```
app/server/internal/
├── core/                     # T1：被所有域依赖的共享服务
│   ├── setting/              #   SettingService（settings 读写 + 后台端点）
│   ├── permission/           #   PermissionService + perm_seed.go
│   ├── session/              #   SessionStore 接口与适配（实现仍在 gateway）
│   ├── email/                #   EmailService + weekly digest
│   ├── audit/                #   AuditLogService
│   └── mcpclient/            #   MCPServerService（把出站 MCP 客户端从 service 挪出，见 §4.3）
├── identity/                 # T2
│   ├── service/              #   auth mfa totp user role group invite banned oauth
│   └── handler/
├── growth/                   # T3
│   ├── service/              #   trust level gate visibility checkin badge leaderboard
│   └── handler/
├── engagement/               # T4
│   ├── service/              #   like favorite follow watch block notification pm report
│   └── handler/
├── content/                  # T5
│   ├── service/              #   post content tag space draft revision moderation mention slug
│   └── handler/
├── ops/                      # T6
│   ├── service/              #   admin announcement backup webhook plugin motion customize theme upload slot
│   └── handler/              #   含 dashboard seo realtime trash
├── app/                      # 装配（composed root）：container/ router/ wiring/ bootstrap/
├── gateway/ middleware/ plugin/ mcp/ authz/ mock/     # 形态不变
├── model/ repository/                                 # 共享内核，形态不变
├── pkg/ config/ db/ storage/ mailer/ metrics/          # 技术底座，形态不变
└── （迁移完成后删除）service/ handler/                 # 空壳包
```

**handler 的共享工具**：`handler/ctx.go`（`currentSession`/`currentUser`/`currentRole`）与 `handler/params.go`（分页/排序解析）被所有域使用，移入 `internal/pkg/httpx/`，各域 handler 引用它。

**域归属速查**（模块 → 域；逐文件落位在 P1 试点时按实测确认，不在本文硬编码行数）：

| 域 | 模块 |
|---|---|
| core | setting, permission(+seed), session iface, email(+digest), audit log, mcp client |
| identity | auth, mfa, totp, user, role, group, invite, banned, oauth |
| growth | trust/XP, level, gate, visibility, checkin, badge, leaderboard |
| engagement | like, favorite, follow, watch, user_block, notification, pm, report |
| content | post, content(comment), tag, space, draft, revision, moderation, mention, slug |
| ops | admin, dashboard, announcement, backup, webhook, plugin, motion, customize, theme, upload, slot, seo, realtime, trash |

`apikey` / `session` / `security` 的**实现**继续留在 `internal/gateway`（凭证与会话是网关职责），`core/session` 只持有被服务层消费的接口形状。

---

## 4. 依赖方向与强制规则

### 4.1 规则

- **R1 单向**：`content` MAY import `engagement`/`growth`/`identity`/`core`；反向 MUST NOT。完整顺序：`core → identity → growth → engagement → content → ops`。
- **R2 层内**：`<domain>/handler` MAY import `<domain>/service` 与更低层的 `<domain>/service`；`<domain>/service` MUST NOT import 任何 `handler`。
- **R3 层边界**：任何 `handler` 包 MUST NOT import `internal/repository`、`internal/db`、`*gorm.DB`。
- **R4 共享内核只读语义**：`model`、`repository` 可被任意域 import（它们是词汇表与数据访问接口）；但任何域 MUST NOT 在 `repository` 里加业务判断。
- **R5 域间只走 Service 接口**：跨域调用 MUST 经提供方域的 Service 接口，MUST NOT 直接持有对方域的 repository。

R1–R3 由 **一条新增的 import 边界测试**（`internal/app/importboundary_test.go`）静态解析 import 图实现，失败信息输出「文件 → 被禁包 → 违反的规则号」。

### 4.2 为什么 `model` / `repository` 不拆

`User`、`Post`、`Space` 被几乎所有域引用；`UserRepository` 被 20+ 服务注入。按域拆这两个包会立刻制造大量「为了用一个类型而反向依赖上层域」的假边，把 §1.3 里干净的 DAG 变成一个网状图。它们本来就是**共享内核**（纯数据 + 数据访问接口，无业务规则），保持不变是这条设计里最有价值的一次 YAGNI。

### 4.3 唯一跨界边的处置：`ModerationService → MCPServerService`

现状是 setter 注入的可选依赖（`SetMCPServerService`），语义上 `MCPServerService` 是**出站适配器**（按配置调外部 MCP 服务），不是业务域。处置：连同 `mcp_server_iface.go` 移入 `core/mcpclient`，`content/service` 依赖它属于 T5→T1，合法。

### 4.4 10 个共享 helper 的处置

| 函数 | 处置 |
|---|---|
| `displayNameOf` | 归 `identity/service` 并导出为 `DisplayNameOf`（被 content/engagement 跨域使用） |
| `validateDisplayName` | 留在 `identity/service`（内部） |
| `gateInputForUser` `gateDenyMessage` | 已属 `growth`，导出为 `growth/service` 的公开函数 |
| `publishUserMentions` | 归 `content/service`（只被 post/content 用，域内） |
| `isValidPostStatus` | 归 `content/service`（域内） |
| `truncate` `mapRepoErr` | `truncate` 移入 `internal/pkg`（纯字符串工具）；`mapRepoErr` 移入 `internal/pkg`（错误翻译） |
| `buildBackupSnapshot` `restoreBackupSnapshot` | 两文件同属 `ops`，跨文件域内调用，无需改动 |

---

## 5. 迁移机制

**strangler，一域一提交，任何中间状态都必须 `go build ./...` 通过。** 每个域的迁移是一个自包含步骤：

1. `git mv` 该域的文件到 `internal/<domain>/service|handler/`，包名改为 `service` / `handler`（域内唯一，无冲突）。
2. 改 package 声明与 import；跨域引用按 `growthsvc "…/growth/service"` 别名约定命名（实测跨域引用点约 10 处）。
3. 把该域的 `Handler` 聚合从 `handler/handler.go` 拆出为 `<domain>/handler/handlers.go`，`app/wiring.go` 逐域构造。
4. 删除 `handler.go` / `service` 里对应的残留。
5. 提交：`refactor(<domain>): move <domain> services and handlers into internal/<domain>`。

**接口归属**：Service 接口跟随**提供方**域（`TrustService` 住在 `growth/service`，由 content/engagement import）。不建共享 `ports` 包——实测依赖是 DAG，建中立接口包只会把「谁依赖谁」再藏起来一次。

**顺序**：`core`（先抽出 T1）→ `growth`（试点）→ `identity` → `engagement` → `content` → `ops`。理由：`core` 是所有域的新底座必须先有；`growth` 体量适中且同时有出边与入边，能一次验证两个方向的规则；`content` 最大最晚。

---

## 6. 分阶段计划

| 阶段 | 内容 | 出口条件 |
|---|---|---|
| **P0 工位与护栏** | 从 `develop` 开 worktree（当前有 32 个未提交改动，不动它）；记录基线（`go build/vet/test`、`feedchamber_test.sh -all`、`make swag` 的 `openapi.json` 哈希、路由数 351）；写 `importboundary_test.go` 与域依赖快照 | 基线全绿且数字入库 |
| **P1 core + growth 试点** | 抽 `internal/core/{setting,permission,session,email,audit,mcpclient}`；迁 `growth`；`pkg/httpx` 落地 | 编译绿 + `openapi.json` 哈希不变 + 边界测试绿 |
| **P2 批量迁移** | `identity` → `engagement` → `content` → `ops`，每域一提交 | 同上，逐域 |
| **P3 装配收口**（**未纳入本次 change**，见 §10-1） | `container`/`router`/`wiring` 收进 `internal/app/`；`iris.go` 754 行的 `Start()` 拆成具名步骤（`bootstrap` 子包），保持顺序与副作用位置不变 | 启动链路等价：`feedchamber_test.sh` 全量绿 |
| **P4 文档与规范收口** | 45 份模块 `AGENTS.md` 按新目录重写/合并；`docs/ARCHITECTURE.md`、`system-map.md`、`backend-overview.md` 实测更新；`node scripts/check-agents-docs.mjs --write-index`；openspec `archive` | `make swag-check`、文档门禁、`openspec validate --all` 全绿 |

**可独立回退**：P0 无代码改动；P1/P2 每域一个 commit，回退单个 commit 即回到上一形态；P3 单独回退；P4 纯文档。全程无数据库迁移、无 API 变更，因此**不存在需要数据回滚的窗口**。

---

## 7. 验收（不变量清单）

| 不变量 | 验证方式 | 判据 |
|---|---|---|
| 端点集合不变 | `make swag` 前后对比 `openapi.json` 的 sha256 | **完全相等** |
| 路由注册数不变 | `grep -rho '\.Post("'` / `\.Get("` | 315 / 36 |
| 无逻辑删除 | `git diff -M --stat` 重命名检测 + 全仓 LOC 净值 | 生产行数变化应为 0（仅 import/package 行变化） |
| 依赖方向合法 | `importboundary_test.go` | 无违规（T1..T6 顺序 + handler 不碰 repo） |
| 编译与静态检查 | `go build ./... && go vet ./...` | 通过 |
| 单测 | `go test ./...` | 26 包全绿 |
| 行为不变 | 新库 `DB_TYPE=sqlite DB_DSN=/tmp/of_dc.db` 启动后 `bash ../feedchamber_test.sh -all` | 与 P0 基线同等通过 |
| 文档门禁 | `node scripts/check-agents-docs.mjs` + `make swag-check` | 通过 |

**诚实声明**：`internal/service`（28 个测试）与 `internal/handler`（1 个测试，还是指标埋点）没有行为级测试覆盖。行为安全网只有上面三条：编译期、`openapi.json` 哈希、`feedchamber_test.sh` 全量 curl。所以本方案把「一次迁移一个域」写成硬规则——不接受任何跨域的大爆炸提交。

---

## 8. 风险

| 风险 | 影响 | 缓解 |
|---|---|---|
| import 大面积改动引入笔误 | 编译失败（可发现）/ 误改语义（不可发现） | 一域一提交；每步 `git diff` 只允许 package/import 行变化；`-M` 重命名检测确认是纯搬迁 |
| 域边界判断错误（如把 `email` 放 `ops`） | 制造环，边界测试红 | 已在 §1.3 实测依赖方向；`moderation→mcp` 是唯一例外且已定位 |
| 接口归属选错导致循环 import | 编译期直接报 import cycle | Go 编译器会立刻拦下，不会静默 |
| 域间辅助函数被重复实现 | 同一逻辑两份 | §4.4 表格穷举了 10 个共享 helper，逐个处置 |
| 文档 45 份 AGENTS.md 滞后 | CI 红 | P4 单独一个阶段，跟 `check-agents-docs.mjs` 一起收口 |
| 迁移期与本仓其他并行开发冲突 | 合并冲突 | 独立 worktree；一域一 PR；迁移期内约定「新功能先落到目标域目录」 |

---

## 9. 后续（独立 change，不在本次范围）

前端后台 `app/web/app/admin` 12k 行、7 个 470–841 行的同构 CRUD 页面，去重方向已确认（页面级脚手架：查询态 + DataTable + 表单弹窗 + 变更后失效策略），但它属于另一栈、另一套评审关注点（`components/ui` 原语 + biome），应与本 change 分开提案。建议 change 名：`2026-11-XX-admin-console-scaffold`。

---

## 10. 待评审拍板的三个问题

1. **`app/` 收拢是否本批做**（P3）。收益是「装配不再是第 7 个域」；成本是再多搬 20+ 文件。也可以拆走单独一个 change。
2. **`core` 的成员**。当前定 6 个（setting/permission/session iface/email/audit/mcpclient）。`mcpclient` 是否该算 core 有争议——它是出站适配器而非业务服务；替代方案是放进现有的 `internal/mcp`（但那包是 **入站** MCP server，语义不同）。
3. **是否接受「10 个 helper 里 2 个进 `internal/pkg`」**（`truncate`/`mapRepoErr`）。这会让 `pkg` 继续变大；另一种选择是给它们各自的域内副本（接受少量重复）。
