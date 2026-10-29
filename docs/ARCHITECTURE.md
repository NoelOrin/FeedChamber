# OrinForum 架构索引


> **规范真相源**：所有已采纳的稳定规范（API 风格、分层架构、服务设计、模块约定、前端共享包、运行配置）已迁移至 [OpenSpec](../openspec/specs/)。规范冲突时以 `openspec/specs/` 为准；后续规范变更须经 `openspec` change 提案流，不再直接改散文文档。

本文件是 OrinForum 架构文档的入口。阅读顺序建议：先读本页建立全局分层心智模型，再读 [system-map.md](system-map.md) 看跨栈现状地图（后端/Web/Dashboard/共享包/部署 + 分层与数据流图），然后按需要深入各层的 `AGENTS.md` 与设计文档。完整项目约定（API 风格、模块说明、环境变量）见仓库根目录 [AGENTS.md](../AGENTS.md)；API 端点清单见 [docs/api-reference.md](api-reference.md)；后端实测现状梳理见 [docs/backend-overview.md](backend-overview.md)。

## 系统组成

Go + React 社区论坛 monorepo，单一后端服务同时驱动两个前端：

- 后端：`app/server`（Go + Iris v12），RPC 风格 HTTP API（写操作 `POST` + 动词后缀，只读查询 MAY `GET`）。
- Web 前端：`app/web`（React 19 + vinext（Next App Router 的 Vite 实现）+ TanStack Query + shadcn 原语）。
- 管理后台：内嵌在主站 `app/web/app/admin/**`（`/admin`），与前台同构建同部署；Ant Design Pro + 纯客户端渲染，页面组件全部 `'use client'`，不做 SSR。
- 前端共享包：`packages/api-client`、`packages/sse-client`、`packages/shared-utils`、`packages/tokens`。

## 后端分层（单向依赖）

请求处理严格遵循单向依赖链，不允许跨层调用：

```
HTTP Request
  → Gateway / Middleware
      app 级：CORS → Trace → Logger
      v1 级：PathBodyLimit → RequestTimeout → gw.Preflight → MaintenanceGate
      protected：gw.Enforce()（会话 sid 或 API Key 必需校验 → banCheck → 权限 → 安全 → 会话跟踪）；
                trace / 全局限流 / 可选凭证在 v1 级的 gw.Preflight() 已完成，两段各自幂等、每请求恰好一次
    → Handler      (解析参数、调 service、统一 JSON 响应、swag 注解)
      → Service    (业务逻辑、所有权/权限校验，返回 *pkg.AppError)
        → Repository (接口 + GORM/SQL 实现，WrapDBErr 翻译错误)
          → Database (SQL via GORM：sqlite / mysql / postgres)
```

| 层 | 目录 | 约定文档 | 职责 |
|----|------|----------|------|
| 入口/启动 | `app/server/cmd`、`app/server/server` | [server/AGENTS.md](../app/server/AGENTS.md) | CLI 分发、`Start(env)` 启动主流程、种子数据 |
| HTTP 层 | `app/server/internal/handler` | [handler/AGENTS.md](../app/server/internal/handler/AGENTS.md) | 参数解析、调用 service、统一 JSON 响应、swag 注解（登出/会话/安全/API Key 不在此层，见网关行） |
| 业务层 | `app/server/internal/service` | [service/AGENTS.md](../app/server/internal/service/AGENTS.md) | 业务逻辑、权限/所有权校验、返回 AppError |
| 数据访问层 | `app/server/internal/repository` | [repository/AGENTS.md](../app/server/internal/repository/AGENTS.md) | 接口 + SQL 实现、错误包装 |
| 模型 | `app/server/internal/model` | [model/AGENTS.md](../app/server/internal/model/AGENTS.md) | 纯数据模型（GORM 标签，无业务） |
| 中间件 | `app/server/internal/middleware` | [middleware/AGENTS.md](../app/server/internal/middleware/AGENTS.md) | trace / 日志 / CORS / 限流 / 超时 / 体积 / 维护模式 / 动态登录门控 / 权限码门控——**不解析用户凭证** |
| 网关层 | `app/server/internal/gateway` | [gateway/AGENTS.md](../app/server/internal/gateway/AGENTS.md) | 凭证解析与会话（`SessionStore`：签发/Lookup/Revoke/Rotate）、API Key、权限映射、限流、安全风控、OAuth 消费方 + 自建 Provider、OIDC SSO |
| 路由 | `app/server/internal/router` | [router/AGENTS.md](../app/server/internal/router/AGENTS.md) | 所有路由注册（253 条：223 POST + 30 GET），按域拆 `router.go` + 8 个 `routes_*.go` |
| 依赖装配 | `app/server/internal/container`、`app/server/internal/handler/handler.go`、`app/server/server/wiring.go` | [container/AGENTS.md](../app/server/internal/container/AGENTS.md) | `New`（纯构造）/`Start`（起 worker）/`Shutdown` 三段；`wiring.go` 映射成 `handler.Deps` |
| 通用工具 | `app/server/internal/pkg` | [pkg/AGENTS.md](../app/server/internal/pkg/AGENTS.md) | 统一响应、错误码、cookie、分页、KV、限流、事件总线、日志、tracing |
| 基础设施 | `app/server/internal/config`、`db`、`storage`、`mailer`、`metrics` | [metrics/AGENTS.md](../app/server/internal/metrics/AGENTS.md) | 配置、存储抽象、邮件、Prometheus Registry |

> 命名提醒：`app/server/internal/pkg`（通用工具 + 基础设施子包）现已是唯一的 `pkg` 目录。`app/server/pkg` 曾是"模块外候选包"目录：网关层原在 `app/server/pkg/gateway`，2026-10-14 迁入 `app/server/internal/gateway`——它的两个消费者（`server/iris.go`、`internal/router`）都在模块内，放在 `pkg/` 只换来"是否对外共享"的误读；最后一个住户 `pkg/jwt` 也于 2026-10-18 整包删除（OAuth2/OIDC access token 改 opaque，JWT 只剩 OIDC `id_token`），该目录现为空且不再被版本控制跟踪，两处同名不同物的混淆随之消失。

## 关键设计决策（速查）

- **用户凭证是服务端会话，不是 JWT**：登录签发 64-hex sid（HttpOnly `master_session`，也可 `Authorization: Bearer <sid>`），有效性等价于 `user_sessions` 行是否存在，KV `sess:<sid>` 做 read-through。改密/改角色/封号/登出都是删行，没有 token 黑名单。对外 OAuth2/OIDC 的 access token 同样是 opaque（`of_at_` + 32B 随机串，服务端只存 SHA-256 hex，撤销翻 `revoked` 列）；唯一保留的 JWT 是 OIDC `id_token`（RS256，签名私钥 PEM 持久化在 settings `sso_oidc_signing_key`）。`/auth/refresh` 单用即轮换。设计记录见 [session-auth.md](design/session-auth.md) 与 [OAuth opaque token 提案](../openspec/changes/2026-10-18-oauth-opaque-tokens/proposal.md)。
- **Repository 单一 SQL 实现**：Mongo 后端已于 2026-10-13 整体移除；每资源一个接口 + 基于 `store.SQL()` 的实现，Service 只依赖接口。
- **统一响应**：`pkg.Success` / `pkg.SuccessPaginated` / `pkg.HandleError`，固定 `{ code, msg, data }`，成功为 `msg: "success"`（openspec api-style 钉死的协议字面量），失败文案为中文；`errToHTTPStatus` 把业务码映射到 HTTP status。
- **可观测性**：自研 W3C `traceparent`（无 OpenTelemetry 依赖）→ 单一关联 id `trace_id`（原 `request_id` 并入）；请求级 logger 经 `logger.FromContext(ctx)` 下传到 service/repository；`/metrics` 不在业务路由上，由 `METRICS_PORT`(9091) 独立 listener 暴露且不经 nginx。详见 [backend-overview.md](backend-overview.md)。
- **权限系统**：56 个平台级权限码 + 版面级权限（`SpaceRolePermission`，支持层级继承）+ 角色映射，`PermissionService` 用 `sync.RWMutex` 维护内存缓存，启动时从 DB 加载，变更经 KV Pub/Sub 多实例同步。
- **动态鉴权**：资源级「作者 OR 有权限」在 service 层实现（R-06b）；公开/强制登录切换经 `AuthGate`/`SpaceAuthGate`。
- **API 风格**：RPC 风格 URL，HTTP 方法按「有无副作用」选择——写操作 `POST` + 动词后缀，只读查询 MAY `GET`（单资源读取 SHOULD GET，列表/多条件筛选 SHOULD POST + `/list`），`PUT` / `PATCH` / `DELETE` 禁止。详见 [AGENTS.md](../AGENTS.md) 的「API 风格约定」与 `openspec/specs/api-style`；端点清单见 [docs/api-reference.md](api-reference.md)。

## 设计文档与 ADR

按主题归档，避免散落在仓库各处：

- 权限体系：[权限体系设计](02-设计/权限体系设计.md)、[版面级禁言系统设计](02-设计/版面级禁言系统设计.md)
- 认证：[session-auth.md](design/session-auth.md)（JWT → 服务端会话，含实现状态与遗留项；2026-10-18 后续：OAuth2/OIDC access token 亦改 opaque，JWT 仅剩 `id_token`，见 [opaque token 提案](../openspec/changes/2026-10-18-oauth-opaque-tokens/proposal.md)）；鉴权网关已落地于 `app/server/internal/gateway/`（方案文档已随实现归档删除）
- 可观测性：[observability-findings-2026-10-13.md](reviews/observability-findings-2026-10-13.md) 与 `openspec/changes/` 内的 change 记录
- 基础设施：[nats-deploy.md](design/nats-deploy.md)
- 迁移/对齐：[discourse-baseline.md](plans/discourse-baseline.md)（能力基线）、[import-discourse.md](design/import-discourse.md)
- ADR：[chat-mvp](adr/chat-mvp.md)、[no-otel-sdk-yet](adr/no-otel-sdk-yet.md)（为何只用 traceparent、不引 OpenTelemetry，以及保留的兼容缝）
- 评审与复盘：[docs/reviews/](reviews/)（`code-review-2026-06-20.md`、`security-audit-2026-06-27.md`、`phase4-discourse-alignment-retro.md`）
- 对齐与迁移：Discourse 对齐结论已沉淀进 `discourse-baseline.md` 与 `reviews/phase4-discourse-alignment-retro.md`；前端 vinext 迁移已合入主干（见 `app/web/AGENTS.md`）

## 运行与文档生成

- 后端启动：`cd app/server && go run main.go -e dev`（Cobra 用 `-e` 指定环境，`main.go dev` 会报 unknown command）；前端 `pnpm dev:web`（`/admin` 后台同进程）。
- Swagger：启动后访问 `/swagger/`，由 `make swag` 从 handler 注解生成并同步到 `openapi.json`。
- API 测试：根目录 `orinforum_test.sh` 全量脚本。
- 部署编排：仓库根 `deploy/`（2026-10 从 `app/server/deploy/` 移出），详见 [system-map.md §9.2](system-map.md#92-部署deploy仓库根)。

## 仓库杂项说明

- `agent_briefs/`、`agent_test_logs/`：agent 运行产物，非源码，已纳入 `.gitignore`。
- `scripts/`：一次性/辅助脚本（CI 护栏、阈值校准等），不在编译路径内。
- `docs/superpowers/`、`agent_briefs/`、`agent_test_logs/`：agent 运行产物，非源码。`.gitignore` 已覆盖，2026-10-27 起**不再被版本控制跟踪**（文件仍保留在本地工作区）。
