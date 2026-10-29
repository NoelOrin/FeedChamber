# OrinForum 系统地图

> 状态：2026-10-27 · 范围：整个 monorepo（后端 + Web + Dashboard + 共享包 + 部署）
> 依据：源码实测。本文只描述「代码现在长什么样」，不评价优劣、不提案改法。
> 规范真相源是 [openspec/specs/](../openspec/specs/)；规范与代码冲突时以 specs 为准，本文的偏差清单只做记录。
> 相关文档：[ARCHITECTURE.md](ARCHITECTURE.md)（入口索引）、[backend-overview.md](backend-overview.md)（后端纵深）、[api-reference.md](api-reference.md)（端点清单）。

---

## 1. 系统组成

单一 Go 后端同时服务两个前端，前端共享 4 个包。

```
                    ┌──────────────────────────────────────────┐
                    │  app/server  (Go + Iris v12, 单进程)      │
                    │  /api/v1  ·  RPC 风格  ·  253 条路由      │
                    └───────▲──────────────────────▲───────────┘
                            │ HTTPS/JSON           │ SSE
        ┌───────────────────┴──────┐   ┌───────────┴──────────────┐
        │  app/web  (React 19)     │   │  app/dashboard (React 19) │
        │  vinext App Router SSR   │   │  Ant Design Pro + Vite    │
        │  面向用户                │   │  管理后台 SPA             │
        └───────────┬──────────────┘   └───────────┬──────────────┘
                    │                              │
        ┌───────────┴──────────────────────────────┴──────────────┐
        │  @orinforum/api-client  (唯一请求层, 40 个 service 模块) │
        │  @orinforum/sse-client  ·  @orinforum/shared-utils       │
        │  @orinforum/tokens      (仅 Dashboard 用)                │
        └─────────────────────────────────────────────────────────┘
```

### 包依赖方向（实测 `package.json`）

| 包 | 依赖 | 被谁依赖 |
|---|---|---|
| `@orinforum/api-client` | 无内部依赖 | web、dashboard、sse-client |
| `@orinforum/sse-client` | `api-client` | web |
| `@orinforum/shared-utils` | 无内部依赖 | web、dashboard |
| `@orinforum/tokens` | 无内部依赖 | **仅 dashboard**（Web 已不依赖，自带 shadcn token） |

`api-client` 是唯一请求层：`authFetch` 封装 Bearer、401 单飞刷新重试、默认 3s 超时。Web 通过 `lib/api/adapter.ts` 注入 `readRequestCookie`（把 SSR 请求的 cookie 转发给后端）并在服务端 `setApiBaseUrl`。

### 技术栈

| 维度 | 后端 | Web | Dashboard |
|---|---|---|---|
| 语言/框架 | Go 1.25 + Iris v12 | React 19 + vinext 1.1（Next App Router 的 Vite 实现） | React 19 + Vite |
| 路由 | 自建 `internal/router`（RPC 风格） | 文件系统路由 `app/` | TanStack Router（33 条 `createRoute`） |
| 取数 | — | TanStack Query 5 + RSC `cache()` | TanStack Query |
| UI | — | shadcn 基准原语 + Tailwind v4 | Ant Design 6 + `@orinforum/tokens` |
| ORM | GORM（sqlite/mysql/postgres） | — | — |
| 消息/KV | NATS（JetStream KV + core Pub/Sub），可内嵌 | — | — |

---

## 2. 后端分层

### 2.1 单向依赖链

```
HTTP Request
  → Gateway / Middleware
      app 级：CORS → Trace → Logger → Recover →（可选）Prometheus
      v1 级：PathBodyLimit → RequestTimeout → gw.Preflight() → MaintenanceGate
      protected：gw.Enforce()
    → Handler      (解析参数、调 service、统一 JSON 响应、swag 注解)
      → Service    (业务逻辑、所有权/权限校验，返回 *pkg.AppError)
        → Repository (接口 + SQL 实现，WrapDBErr 翻译错误)
          → Database (SQL via GORM)
```

### 2.2 层间 import 矩阵（实测生产文件）

行 = 引用方，数字 = 该层文件中出现该 import 的次数。**矩阵下三角为空，没有反向边。**

| 引用方 ＼ 被引用 | handler | service | repository | model | gateway | middleware | pkg |
|---|---|---|---|---|---|---|---|
| handler | — | 39 | — | 13 | — | — | 44 |
| service | — | — | 41 | 64 | — | — | 59 |
| repository | — | — | — | 53 | — | — | 10 |
| model | — | — | — | — | — | — | 4 |
| gateway | — | — | — | 14 | — | — | 17 |
| middleware | — | — | — | — | — | — | 12 |
| router | 1 | — | — | 4 | 1 | 7 | — |

两处需要说明：

- **`router` 是唯一的汇聚点**，同时 import handler / gateway / middleware——它的职责就是挂载，不代表业务层依赖。
- **`pkg` 反向引用 `storage` 与 `mailer`**（`internal/pkg/settings.go`）：`BuildStorageConfig` 按 settings → env → 默认 解析存储配置，需要知道 `storage` 的类型。这是矩阵里唯一一处基础设施之间的耦合。

### 2.3 目录与职责

| 层 | 目录 | 职责 | 约定文档 |
|---|---|---|---|
| 入口/启动 | `cmd`、`server` | CLI 分发、`Start(env)` 主流程、种子数据、`wiring.go` | [server/AGENTS.md](../app/server/AGENTS.md) |
| HTTP 层 | `internal/handler` | 参数解析、调 service、统一响应、swag 注解 | [handler/AGENTS.md](../app/server/internal/handler/AGENTS.md) |
| 业务层 | `internal/service` | 业务逻辑、权限/所有权校验、返回 AppError | [service/AGENTS.md](../app/server/internal/service/AGENTS.md) |
| 数据访问层 | `internal/repository` | 接口 + SQL 实现、错误包装 | [repository/AGENTS.md](../app/server/internal/repository/AGENTS.md) |
| 模型 | `internal/model` | 纯数据模型（GORM 标签，无业务） | [model/AGENTS.md](../app/server/internal/model/AGENTS.md) |
| 网关层 | `internal/gateway` | 凭证解析与会话、API Key、权限映射、限流、安全风控、OAuth 消费方 + Provider、OIDC SSO | [gateway/AGENTS.md](../app/server/internal/gateway/AGENTS.md) |
| 中间件 | `internal/middleware` | trace / 日志 / CORS / 限流 / 超时 / 体积 / 维护模式 / 动态登录门控。**不解析用户凭证** | [middleware/AGENTS.md](../app/server/internal/middleware/AGENTS.md) |
| 路由 | `internal/router` | 253 条路由注册，按域拆 9 个文件 | [router/AGENTS.md](../app/server/internal/router/AGENTS.md) |
| 依赖装配 | `internal/container` | repo/service 构造（`New`）+ 后台 worker（`Start`） | [container/AGENTS.md](../app/server/internal/container/AGENTS.md) |
| 通用工具 | `internal/pkg` | 统一响应、错误码、cookie、分页、KV、限流、事件总线、日志、tracing | [pkg/AGENTS.md](../app/server/internal/pkg/AGENTS.md) |
| 基础设施 | `internal/config`、`db`、`storage`、`mailer`、`metrics` | 配置、DB 抽象、存储、邮件、Prometheus Registry | [metrics/AGENTS.md](../app/server/internal/metrics/AGENTS.md) |

### 2.4 依赖装配三段式

`internal/container` 刻意拆成三个文件、三种语义：

| 段 | 文件 | 做什么 | 约束 |
|---|---|---|---|
| `New(Deps)` | `container.go`（240 行） | 纯构造对象图：36 个 repository + 35 个 service | **无副作用**：不起协程、不订阅、不写库，因此可在测试里安全调用 |
| `Start(ctx)` | `runtime.go`（66 行） | 订阅事件 + 起 5 个后台 worker + seed 默认徽章 | 写库失败必须返回给调用方 |
| `Shutdown()` | `runtime.go` | 冲刷浏览量 → 停 worker → 关事件总线 → 关通知分发 | 顺序有意义：浏览量在 KV 里，必须先落 SQL 再关 KV |

`Deps`（`deps.go`，31 行）取代了此前 11 个位置参数。`Container` 结构体有 36 个 repo 字段 + 40 个 service 字段，其中 5 个 service 字段**不由 container 构造**——`PermissionSvc` / `SettingSvc` / `BannedSvc` / `SpaceSvc` / `SessionSvc` 在 `server/iris.go` 启动期建好后注入（另有 `Security` 只注入给 AuthService），因为 middleware、gateway、业务层三方都要用，必须只有一份实例。

**后台 worker（`Start` 启动，`Shutdown` 按名归因关停）**：

| 名称 | 来源 | 周期/触发 |
|---|---|---|
| `view_count_flusher` | `PostSvc.StartViewCountFlusher` | 定时把 KV 里的浏览量缓冲刷进 SQL |
| `pin_expiry_checker` | `PostSvc.StartPinExpiryChecker` | 定时解除过期置顶 |
| `webhook_delivery` | `WebhookSvc.StartWorker` | 投递队列 + 重投 |
| `weekly_digest` | `DigestSvc.StartWeeklyTicker` | 周报 |
| `trust_promotion` | `TrustSvc.StartDailyPromotionTicker` | 信任等级按日晋升 |

外加 `service.SubscribeNotifications` 返回的 `NotifDispatcher`（事件 → 通知）。

**HTTP 层不接触 container**：`server/wiring.go`（61 行）把 39 个 service 逐字段映射成 `handler.Deps`。新增 service 若被 handler 消费，漏接线是编译错误；「HTTP 层依赖什么」可从一个文件读完。

### 2.5 启动顺序（`server.Start(env)`）

```
 1. loadEnv(env)               .env + .env.{dev|prod}，godotenv 覆盖式
 2. config.Load() + Validate()
 3. logger.SetGlobal(...)      必须早于任何业务日志
 4. db.New(cfg.DB)
 5. AutoMigrate(55 个模型)
 6. backfillDisplayName        一次性回填 display_name
 7. seedRoles → seedAdmin → seedGroups → syncDefaultSettings → syncModerationConfigs
 8. 读 MetaSeeded 判断首启
 9. NATS：NATS_URL 空则起内嵌 nats-server → kvstore → eventbus
10. seedPermissions           建 PermissionService / SettingService / BannedService
11. InitOAuthTTL / InitPagination   从 settings 表载入运行时参数
12. Dev 专属：ensureMockData + reseedPostsAndContents
13. 写回 MetaSeeded=true
14. BuildStorageConfig(settings→env→默认) → storage.New
15. permSvc.StartSyncSubscriber     多实例权限缓存 Pub/Sub
16. iris.New() + app 级中间件链
17. （METRICS_ENABLED）Prometheus 中间件 + 9091 独立 listener
18. 挂 /swagger/{any:path}
19. spaceSvc + 版面层级注入权限缓存
20. gwDeps → SessionStore → SecurityManager → middleware.Deps
21. container.New → c.Start(ctx) → newHandler(c)
22. router.Register(app, h, deps, gateway.New(gwDeps))
23. 预检端口 → goroutine 内 app.Listen
24. 阻塞等 SIGINT/SIGTERM → 10s 优雅关闭
```

关停顺序：`app.Shutdown` → `metricsSrv.Shutdown` → `runtime.Shutdown()`（冲刷浏览量、停 worker、关事件总线）→ 停权限同步 → 关 KV / 内嵌 NATS / DB。

---

## 3. 请求流水线

### 3.1 三段挂载

| 位置 | 中间件 | 作用 |
|---|---|---|
| app 级 | `CORS` → `Trace` → `Logger` → `Recover` →（可选）`Prometheus` | Trace **必须先于** Logger；Recover 排在 Logger 之后，panic 仍能产生带 `status=500` 的请求日志 |
| v1 级 | `PathBodyLimit(1MB, /upload 10MB)` → `RequestTimeout(30s, SSE 跳过)` → `gw.Preflight()` → `MaintenanceGate` | Preflight = trace 命名 + 全局每 IP 限流 + **可选**凭证解析，对 `/api/v1` 全量生效 |
| protected | `gw.Enforce()` | 必需凭证 → banCheck（被封 404）→ 权限 → 安全风控 → 会话跟踪 |

`Preflight` / `Enforce` 两段用 iris ctx 上的 stage 标记（`gateway.go` 的 `ranStage`/`markStage`）保证**幂等**，所以挂载方式不决定正确性（组合入口 `Handler()` 已于 2026-10-27 删除）。两个已修缺陷的因果：

1. **限流二次计数**：v1 层 Preflight 对 protected 同样生效，旧 `Handler()` 又跑一遍 preflight，同一 `"ratelimit:"+ip` 键计两次 → 60/分钟的上限在第 30 次就 429。
2. **权限门失效**：Iris 的 `Party()` 只复制父中间件切片，兄弟 party 的 `Use` 会写进父数组空闲槽。app 级中间件条数决定是否有空槽（`METRICS_ENABLED=true` 时 5 条 → 有），于是 5 个公开 party 各自的 `Use(gw.OptionalHandler())` 改写了其后注册的 protected 链，权限/banCheck/安全/会话跟踪一律不执行。实测默认配置下普通用户 `POST /admin/stats` 返回 200。修复方式是删掉那 5 处重复挂载，由 `internal/router/middleware_chain_test.go` 源码扫描钉住。

> **硬约束**：`protected := v1.Party("")` 与 `protected.Use(gw.Enforce())` 必须留在 `router.go` 的原位置（公开 party 之后、任何 protected 路由之前），`routes_*.go` 里禁止出现任何 `Use(gw.`。

### 3.2 网关层组件（`internal/gateway`，19 生产文件 / 3710 行）

通过 `Deps`（`SettingService` / `Perms` / `Ban` / `KV` / `DB`）与业务解耦：

| 组件 | 文件 | 职责 |
|---|---|---|
| `AuthChecker` | `auth.go` | 凭证解析：opaque sid（Bearer 或 `master_session` cookie）→ `user_sessions` 活行 + 滑动续期；未命中再按 `sha256(cred)` 查 `oauth_tokens` |
| `PermissionCheckerMiddleware` | `permission.go` | 路径 → 权限码映射表，**未映射的 protected 路由默认拒绝** |
| `RateLimiter` | `ratelimit.go` | 全局每 IP 上限，计数交给 `internal/pkg/ratelimit`（`GlobalKey`） |
| `Tracing` | `tracing.go` | 路由匹配后把端点模板写进 `trace.Name`，不调 `ctx.Next()` |
| `OAuthManager` | `oauth.go` + `oauth_{google,github,discord}.go` | 消费方 OAuth 登录 |
| `OAuthProvider` | `oauth_provider.go` + `token.go` | 服务端作为 OAuth 2.0 Provider：客户端注册 / authorize / token / revoke |
| `SSOProvider` | `sso.go` + `sso_oidc.go` | OIDC SSO：Discovery / JWKS / Token / UserInfo / Authorize |
| `APIKeyManager` | `apikey.go` | API Key 认证（`X-API-Key` 或 `Bearer ak_`），与会话互斥且优先 |
| `SessionManager` | `session.go` | 会话列表 / 单点吊销 / logout-all / `POST /auth/logout` + 会话跟踪 |
| `SessionStore` | `session_store.go` + `session_issue.go` | 会话权威：签发 / Lookup / Revoke / Rotate，`user_sessions` 行 + KV `sess:<sid>` read-through |
| `SecurityManager` | `security.go` | 暴力破解 / 异常位置 / 安全事件 / 登录历史 |

**三种凭证**：会话 sid（用户）、API Key（机器，`ak_` 前缀）、OAuth2/OIDC opaque access token（`of_at_` + 32B 随机串，库中只存 SHA-256 hex，撤销翻 `revoked` 列）。唯一的 JWT 是 OIDC `id_token`（RS256，私钥 PEM 持久化在 settings `sso_oidc_signing_key`）。

**权限映射**：`permission.go` 的 `initRouteMap` 用 exact / prefix / pattern / space-pattern 四类匹配。`RequirePermission` 中间件只额外挂在 `/permission/*`、`/admin/backups`、`/admin/webhooks`；其余 protected 路径由这张映射表负责。

---

## 4. 模块地图

### 4.1 三层对应关系

| 层 | 规模 | 组织方式 |
|---|---|---|
| Handler | 42 生产文件 / 7716 行 / **38 个 `XxxHandler` 接口** | 每资源一个接口 + 私有实现 + swag 注解；聚合体在 `handler.go` |
| Service | 80 生产文件（44 实现 + 36 iface）/ 9876 行 / **38 个 service 接口** | 接口与实现分离；9 个实现文件无独立 iface（`backup_snapshot` `badge` `digest` `leaderboard` `mention` `points` `slug` `totp` `webhook`），`session_iface.go` 定义的是 `SessionStore` 而实现在网关层 |
| Repository | 60 生产文件 / 4953 行 / **38 个接口** | 19 个资源已拆成 `xxx.go`（接口）+ `xxx_sql.go`（实现），其余单文件内定义 |

### 4.2 按域分组

| 域 | Handler | Service | 说明 |
|---|---|---|---|
| 认证与账号 | `auth` `mfa` `oauth` `banned` `user` `invite` | `auth` `mfa` `totp` `oauth` `banned` `user` `invite` `credential` | 会话 sid、MFA/TOTP、OAuth 消费方、禁言与封号、邀请注册 |
| 权限与组织 | `role` `permission` `group` | `role` `permission` `group` `trust` | 54 个平台级权限码 + 版面级权限（层级继承）+ 用户组 + 信任等级 |
| 内容 | `post` `content` `tag` `space` `draft` `revision` `trash` | `post` `content` `tag` `space` `slug` `draft` `revision` | 帖子/评论/标签/版面树；草稿、修订历史、回收站 |
| 互动 | `like` `favorite` `follow` `watch` `user_block` | `like` `favorite` `follow` `watch` `user_block` `mention` | toggle 模式 + 状态查询 + 列表 |
| 通讯 | `pm` `notification` `realtime` | `pm` `notification` `email` | 私信、通知（SSE 未读数）、通用 SSE、邮件 |
| 运营 | `report` `moderation` `announcement` `admin` `dashboard` | `report` `moderation` `announcement` `admin` `audit_log` | 举报、审核规则、公告、后台统计/搜索/批量/对账 |
| 成长 | `checkin` `badge` `leaderboard` | `checkin` `badge` `leaderboard` `points` `digest` | 签到、徽章、排行榜、积分、周报 |
| 基础设施 | `upload` `setting` `backup` `plugin` `webhook` `theme` `seo` | `upload` `setting` `backup` `backup_snapshot` `plugin` `webhook` `theme` | 上传、系统设置、备份、插件（仅元数据）、Webhook、主题、sitemap/RSS |

### 4.3 网关自注册的端点（handler 层没有对应文件）

登出、会话列表/吊销/logout-all、安全事件/登录历史、API Key、OAuth Provider、OIDC SSO 全部由 `router` 直接挂 `gw.*` 方法：

```
gw.Session.{LogoutHandler,ListSessions,RevokeSession,LogoutAll}
gw.Security.{ListSecurityEvents,ListLoginHistory,ReportSuspicious}
gw.APIKey.{Create,List,Revoke}
gw.Provider.{RegisterClient,ListClients,UpdateClient,ResetClientSecret,DeleteClient,Authorize,Token,Revoke}
gw.SSO.{Discovery,JWKS,Authorize,Token,UserInfo}
```

---

## 5. 数据与常量

| 项 | 实测值 | 位置 |
|---|---|---|
| AutoMigrate 模型 | **55 个** | `server/iris.go` |
| model 文件 | 47 个 / 68 个 struct | `internal/model/` |
| 平台级权限码 | **54 个**（`xxx:yyy`，无重复） | `internal/model/permission.go` |
| 默认角色 | 5 个：`admin` `moderator` `user` `pending_user` `ban` | `internal/model/role.go` |
| settings 默认 key | **123 个** | `internal/model/setting.go` 的 `DefaultSettings` |
| 事件类型 | **12 个** | `internal/pkg/eventbus/events.go` |
| 业务错误码 | **19 个**（0 + 1xxx/2xxx/3xxx/4xxx/5xxx） | `internal/pkg/errors.go` |
| 限流命名端点 | **12 个** | `internal/router/*.go` |

**错误码分段**：`1xxx` 认证（1001–1016）、`2xxx` 参数（2001）、`3xxx` 资源（3001/3002）、`4xxx` 限流与体积（4001/4002）、`5xxx` 服务端（5001/5004/5005）。`errToHTTPStatus` 负责业务码 → HTTP status（`5004→504`、`5005→503`）。

**限流预算**（`middleware.RateLimit`，滑动窗口，KV 支撑）：

| 端点 | 上限/分钟 | 端点 | 上限/分钟 |
|---|---|---|---|
| `register` | 5 | `post_create` | 10 |
| `login` | 10 | `comment_create` | 30 |
| `forgot_password` | 3 | `report_create` | 10 |
| `reset_password_by_code` | 5 | `pm_conversation_create` | 20 |
| `mfa_verify` | 20 | `pm_message_create` | 60 |
| `send_verify_email` | 3 | `verify_email` | 10 |

---

## 6. 路由

`internal/router` 拆成 `router.go` + 8 个 `routes_*.go`，共 15 个段函数。**实测 253 条注册：223 POST + 30 GET**。

| 文件 | 行数 | 段函数 | 覆盖 |
|---|---|---|---|
| `router.go` | 113 | `Register` + `reg` | v1 party、`/.well-known/*`、v1.Use 链、`/health`、SEO app 根注册、protected 挂载 |
| `routes_admin.go` | 138 | `spaceManageRoutes`, `adminRoutes` | 版面写操作；后台（邀请/审批/公告/设置/禁言/备份/插件/回收站/审核/webhook/统计/批量/审计/对账） |
| `routes_session.go` | 104 | `dashboardRoutes`, `protectedAuthRoutes`, `userRoutes` | 首页聚合、登出/刷新/改密/MFA、OAuth 绑定、SSO 授权、用户资料 |
| `routes_public.go` | 85 | `publicRoutes`, `publicOptionalRoutes` | 公开读 + 注册登录 + OAuth/SSO 公开面 + tags/spaces/announcements 可选登录读 |
| `routes_content.go` | 71 | `tagManageRoutes`, `postRoutes` | 标签写、帖子与评论 |
| `routes_engagement.go` | 70 | `engagementRoutes`, `uploadRoutes` | 收藏/点赞/回复/订阅/关注/拉黑；上传 |
| `routes_message.go` | 51 | `messageRoutes`, `reportRoutes` | 私信、通知、实时流；举报 |
| `routes_account.go` | 51 | `accountRoutes` | 签到、API Key、草稿、会话、主题偏好、安全风控 |
| `routes_access.go` | 46 | `accessRoutes` | 角色、权限（含版面级）、用户组 |

非 `/api/v1` 的系统端点：`/.well-known/openid-configuration`、`/.well-known/jwks.json`、`/sitemap.xml`、`/sitemap-posts.xml`、`/robots.txt`、`/latest.rss`、`/swagger/{any:path}`（后者在 `server/iris.go` 注册）。`/metrics` **不在**业务路由上，由 `METRICS_PORT`(9091) 独立 listener 暴露。

---

## 7. 前端

### 7.1 Web（`app/web`）

规模：`app/` 35 文件 + `components/` 108 文件 + `lib/` 18 文件，共 **13447 行** TS/TSX。

**路由**（`app/` 目录，24 个 `page.tsx`）：

| 路由 | 说明 |
|---|---|
| `/` | 重定向 → `/timeline` |
| `/timeline` | 帖子列表（`?tag= &sort_by= &list_view=`，nuqs 校验） |
| `/timeline/post` | 帖子详情，**`?id=` 查询参数，不是路径参数** |
| `/timeline/create` `/timeline/tag` | 发帖 / 标签 |
| `/spaces` `/spaces/{slug}` `/spaces/{slug}/post` | 版面总览 / 单版首页 / 版内发帖 |
| `/user` + `/user/{posts,notifications,follow,favorites}` | 我的主页与子页 |
| `/user/{userId}` | 他人主页（静态段优先） |
| `/messages` `/messages/{id}` | 私信 |
| `/home` `/leaderboard` `/search` | 聚合 / 排行 / 搜索 |
| `/t/{slug}/{id}` | 短链 |
| `/login` `/register` `/oauth` | 认证页（`?redirect=` 仅接受站内相对路径） |

页面组：`(app)/` 是公开内容与私有面共用的 AppShell，**layout 不兜底鉴权**（内容页按服务端设置 `requireAuthBySetting` 判定，结构性私有页各自在页面内 `requireAuth`）；`(auth)/` 免登录（layout 内 `requireGuest`）。详见 [app/web/app/AGENTS.md](../app/web/app/AGENTS.md)。

**取数层**：

- `lib/query/client.ts` — RSC 每请求一份 / 浏览器单例
- `lib/query/keys.ts` — 全站唯一 query key 来源（62 个 key 定义）
- `lib/queries/server.ts` — RSC 数据接缝，全部用 React `cache()` 包一层（同请求内 `generateMetadata` → layout → page 只发一次后端调用），失败降级为空值
- `lib/api/adapter.ts` — 注入 `readRequestCookie` + 服务端 `setApiBaseUrl`

**认证**：

- `lib/auth/session.ts` — zustand 状态机（unknown/anonymous/loading/authenticated/refreshing/expired）+ `authSession` 命令式门面
- `lib/auth/guards.ts` — 服务端 `requireAuth` / `requireGuest`（`headers()` + `redirect()`）。刻意不从 `lib/auth/index.ts` 再导出，否则 `next/headers` 会打进浏览器包
- `lib/auth/sync.ts` — BroadcastChannel + localStorage 跨标签同步
- `proxy.ts` — 边缘守卫：`PROTECTED` 正则未登录 → `307 /login?redirect=…`；`GUEST_ONLY` 已登录 → `/timeline`

**两道登录守卫**：`proxy.ts`（cookie `master_auth=1` 存在性检查）+ `(app)/layout.tsx` 的服务端 `requireAuth`（拿不到会话时兜底跳转）。

**组件分组**：`ui/`（19 个 shadcn 原语）、`layout/`（10）、`post/`（11）、`markdown-editor/`（11）、`user/`（24）、`content/`（9）、`auth/`（6）、`pm/`（3）、`search/`（3）、`aside/`（2）、`pagination/`（2）、`space/`（2）、`leaderboard/`（2）、`home/`（1）、`checkin/`（1）、`error/`（1）。

**约定出口**：`lib/icons.ts` 是图标唯一出口（`biome.json` 把直接 import `lucide-react` 列为 error）；`components/ui/prose.tsx` 是正文排版唯一出口。

### 7.2 Dashboard（`app/dashboard`）

规模：52 文件 / 8201 行，26 个页面目录，33 条 `createRoute`（全部手写在 `router.tsx`，无文件路由 codegen）。

| 分组 | 路由 |
|---|---|
| 认证 | `/login`、`/first-change-password` |
| 运营 | `/`、`/users`(+detail)、`/posts`(+detail)、`/comments`(+detail)、`/reports`(+detail)、`/moderation`、`/banned`、`/search` |
| 组织 | `/roles`(+detail)、`/groups`、`/invites` |
| 内容结构 | `/spaces`、`/tags`、`/announcements` |
| 系统 | `/settings`(+dashboard)、`/webhooks`、`/api-keys`、`/backups`、`/security`、`/sessions`、`/trash`、`/oauth-clients`、`/plugins`、`/audit-logs` |

结构要点：`router.tsx` 集中路由配置（含 ErrorBoundary 注册）、`stores/auth-store.ts` 认证状态、`components/error/`（`boundaries.tsx` + `ErrorFallback` 的 antd `Result` 兜底）。

---

## 8. 规模实测表

### 8.1 后端（`app/server`，不含 swaggo 生成的 `docs/`）

| 目录 | 生产文件 | 生产行数 | 测试文件 | 测试行数 |
|---|---|---|---|---|
| `internal/service` | 80 | 9876 | 28 | 3521 |
| `internal/handler` | 42 | 7716 | 1 | 344 |
| `internal/repository` | 60 | 4953 | 7 | 602 |
| `internal/gateway` | 19 | 3710 | 13 | 3763 |
| `internal/model` | 47 | 2352 | 2 | 73 |
| `server` | 3 | 1041 | 0 | 0 |
| `internal/middleware` | 9 | 819 | 6 | 695 |
| `internal/router` | 9 | 729 | 1 | 99 |
| `internal/mock` | 1 | 725 | 0 | 0 |
| `cmd` | 3 | 636 | 0 | 0 |
| `internal/metrics` | 10 | 513 | 2 | 233 |
| `internal/pkg`（根） | 9 | 471 | 1 | 85 |
| `internal/mailer` | 5 | 460 | 0 | 0 |
| `internal/storage` | 3 | 343 | 0 | 0 |
| `internal/container` | 3 | 337 | 1 | 85 |
| `internal/db` | 4 | 240 | 1 | 36 |
| `internal/config` | 1 | 132 | 1 | 38 |
| `internal/pkg/*`（7 子包） | 10 | 1548 | 8 | 1109 |

- **合计 319 个生产文件 / 36643 行**，另有 **71 个测试文件 / 10602 行**（共 390 个 `.go` 文件）。
- 测试分布：service 28、gateway 13、pkg 子包 8、repository 7、middleware 6、metrics 2、model 2、其余各 1。**`internal/handler` 只有 1 个测试文件**（`sse_metrics_test.go`，测指标埋点，不是 handler 行为）。
- `go build ./...` / `go vet ./...` / `go test ./...` 全绿（26 个包，18 个含测试）。

### 8.2 前端

| 包 | 文件 | 行数 |
|---|---|---|
| `app/web`（app + components + lib） | 161 | 13447 |
| `app/dashboard/src` | 52 | 8201 |
| `packages/api-client/src` | 53 | 3032 |
| `packages/sse-client/src` | 2 | 201 |
| `packages/shared-utils/src` | 4 | 58 |
| `packages/tokens` | 构建产物（`dist/`）+ `scripts/build.ts` | — |

---

## 9. 可观察性与部署

### 9.1 指标（独立 Registry，`internal/metrics` 10 个按域文件）

26 个指标族：HTTP（duration / total / size / panics）、DB 池（6 个）、EventBus（5 个）、KV（2 个）、限流（2 个）、权限判定、SSE 连接数、登录失败、Webhook（3 个）。

**基数护栏**：标签取值必须有界（method、归一化 path、status、事件名常量、KV op、判定结果）。trace id / span id / 用户 id / 原始 URL 段 / 错误文本一律不作标签。由 `observability_test.go` 的 `TestLabelNamesAreBounded` 钉成测试。

**暴露面**：`METRICS_PORT`(9091) 独立 listener，**无鉴权**；compose 用 `expose` 而非 `ports`，nginx 无指向 9091 的 location。

### 9.2 部署（`deploy/`，仓库根）

> 2026-10 从 `app/server/deploy/` 移到仓库根 `deploy/`，compose 的 build context 相应改为 `..`（仓库根）。

| 文件 | 形态 |
|---|---|
| `docker-compose.yml` | postgres + nats + backend + frontend + nginx（PostgreSQL + 外部 NATS，一键可起） |

这是**唯一的编排档**。原 `cluster` 档（postgres + nats-1/2/3 + prometheus + blackbox + grafana）已删除，`prometheus.yml` / `prometheus/rules/` / `blackbox/` / `grafana/` 保留为可选素材，供外部 Prometheus / Grafana 挂载；backend 的 `METRICS_PORT`(9091) 仅容器网络内可达。

原 `lite` / `basic` / `full` 三档已合并（与默认档、集群档重复）；Redis 与 `KV_PG_DSN` 已删除——后端从未读取，KV 的唯一实现是 NATS JetStream。

**两个 Dockerfile**：`app/server/Dockerfile`（仅后端，`golang:1.25-alpine` 构建 + alpine 运行，`WORKDIR /app/server`）、`app/web/Dockerfile`（仅前端 vinext）。两个 compose 服务的 build context 都是**仓库根**：Dockerfile 的 COPY 路径与根 `.dockerignore` 按仓库根书写，web 还需要根部的 `pnpm-lock.yaml` / `pnpm-workspace.yaml` / `packages/`。原根 `Dockerfile`（Go + Web 合并镜像）已删除。

**监控栈**（仅 cluster）：`prometheus.yml` 15s 抓取 + 13 条告警规则（4 组）+ Grafana 10 面板 + blackbox 探 `/api/v1/health`。Alertmanager 未部署（`alertmanagers.targets` 为空）。

---

## 10. 文档 vs 代码：偏差清单

> 本节只记录实测差异，不含处置建议。规范级偏差须走 `openspec` 提案流修正。

### 10.1 文档过期（数字与路径）

| # | 文档位置 | 文档写的 | 实测 |
|---|---|---|---|
| 1 | `backend-overview.md` §2/§6 | `internal/router` 1 文件 / 528 行 | 9 文件 / 729 行（已按域拆分） |
| 2 | `backend-overview.md` §2 | `internal/container` 1 文件 / 259 行 | 3 文件 / 337 行（`deps.go`+`runtime.go` 已拆） |
| 3 | `backend-overview.md` §2 | gateway 18 生产文件 / 3664 行 | 19 / 3710 |
| 4 | `backend-overview.md` §2 | handler 42 / 7683 | 42 / 7716 |
| 5 | `backend-overview.md` §7、`ARCHITECTURE.md` | 权限码 53 个 | **54 个** |
| 6 | `backend-overview.md` §2 | 305 生产文件 / 35977 行 | 319 / 36643 |
| 7 | `backend-overview.md` §6 | `router.go` 实测 223 POST + 30 GET | 与代码一致，但归属已不是单文件 |
| 8 | `backend-overview.md` §9.1 / §12 | 部署编排在 `app/server/deploy/` | 已移到仓库根 `deploy/` |
| 9 | `backend-overview.md` §0 | Web 前端「SolidJS」 | React 19 + vinext（Solid 树已删除） |

### 10.2 代码与契约不一致

| # | 偏差 | 证据 |
|---|---|---|
| 10 | **`openapi.json` 曾缺 67 条已挂载路由（2026-10-27 已补 61 条）** | 补齐后 openapi 从 **185 → 246** 个路径。根因是 12 个 handler 文件（`backup` `badge` `draft` `leaderboard` `mfa` `plugin` `realtime` `revision` `seo` `theme` `trash` `webhook`）与 `internal/gateway` 的 18 个生产文件（仅 `session.go` 有 1 条 `@Router`）没有注解。**仍缺 6 条**：`/.well-known/openid-configuration`、`/.well-known/jwks.json`、`/sitemap.xml`、`/sitemap-posts.xml`、`/robots.txt`、`/latest.rss`——它们是 app 根路径，而 swag 的 `@BasePath` 固定 `/api/v1`，标注会生成错误路径（`/api/v1/sitemap.xml`），故有意不标 |
| 11 | **followers/following 方法不一致** | 代码 `POST /users/{userId}/followers`（`routes_engagement.go:44-45`），swag 注解与 `packages/api-client/src/services/follow.ts:17-22` 都是 `GET` |
| 12 | ~~根 `Dockerfile` Go 版本落后~~ **已消除** | 根 `Dockerfile` 已删除，只保留前后端两个镜像；`app/server/Dockerfile` 的 Go 基础镜像对齐 `go.mod` 的 `1.25.0` |

### 10.3 已知结构性约束（不是缺陷，是设计后果）

| # | 约束 | 后果 |
|---|---|---|
| 13 | Iris 的兄弟 party 共享父切片空闲槽 | 任何新的子 party `Use` 都可能改写邻近 party 的链；只能靠 `middleware_chain_test.go` 与约定拦着 |
| 14 | ~~两个包各有一份同形接缝接口~~（2026-10-27 已合并） | 原来 `internal/gateway/deps.go` 与 `internal/middleware/deps.go` 各自声明 `SettingReader` / `PermissionChecker` / `BanChecker` vs `BannedChecker`。现已统一到 `internal/pkg/seams.go` 一份声明，`gateway/deps.go` 删除、`Deps` 并入 `gateway.go`；`BanChecker` 与 `BannedChecker` 合并为 `pkg.BannedChecker`（含 `IsBanned` + `IsBannedInSpace`） |
| 15 | `AutoMigrate` 承载全部建表（55 模型） | 与手写 `db/migrations/*.sql` 并存，存在「迁移来源二元」 |
| 16 | 插件系统仅声明式元数据 | `Plugin` 只有 install/enable 记录，无可执行扩展点 |
| 17 | `METRICS_PORT`(9091) 无鉴权 | 靠部署纪律兜底：改 `expose` → `ports` 或给 nginx 加 location 就会公网可读 |
| 18 | 端到端 trace 未打通 | nginx 只在 `location /api/v1/` 透传 `X-Request-ID`，`/swagger/` 与前端 location 未设 |
| 19 | 「不引入 OpenTelemetry」无 ADR | 决策语境只在 change 的 `design.md`，`docs/adr/` 只有 `chat-mvp.md` |
| 20 | 告警阈值全是初值 | 5xx>5%、P99>1s、登录失败>50/5m 均无 SLO 依据 |

### 10.4 规范文档偏差（须走 openspec 提案流）

| # | 位置 | 偏差 |
|---|---|---|
| 21 | `openspec/specs/operations/spec.md` | 仍写「存在 `dev`/`prod` 子命令」；实际只有持久标志 `-e/--env` + `backup` / `import-discourse` 两个子命令，`go run main.go dev` 会报 `unknown command` |

---

## 11. 阅读路径

| 想知道什么 | 读哪里 |
|---|---|
| 全局分层与依赖方向 | 本文 §2 |
| 一个请求经过哪些中间件 | 本文 §3 |
| 某个模块在哪一层、叫什么 | 本文 §4 |
| 后端纵深（每层细节、可观察性、风险逐条） | [backend-overview.md](backend-overview.md) |
| 具体端点与权限码 | [api-reference.md](api-reference.md)、运行时的 `/swagger/` |
| 各层编码约定 | 对应目录的 `AGENTS.md`（全量清单见根 [AGENTS.md](../AGENTS.md) 的「模块文档索引」） |
| 已采纳的稳定规范 | [openspec/specs/](../openspec/specs/) |
| 设计决策与历史 | [docs/design/](design/)、[docs/plans/](plans/)、[docs/adr/](adr/) |
