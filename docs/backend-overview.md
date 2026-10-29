# OrinForum 后端梳理（`app/server`）

> 状态：2026-10-27 · 范围：仅 `app/server`（Go 后端）
> 依据：源码实测（非转述既有文档）。跨栈全景见 [system-map.md](./system-map.md)，架构索引入口见 [ARCHITECTURE.md](./ARCHITECTURE.md)，端点清单见 [api-reference.md](./api-reference.md)。
> 已采纳规范以 `openspec/specs/` 为准；本文是「现状梳理」，规范冲突时以 specs 为准。

---

## 0. TL;DR

- 单一 Go 单体服务（Iris v12），RPC 风格 HTTP API，同时驱动 Web（React 19 + vinext）与 Dashboard（React + Ant Design）两个前端。
- 规模：**390 个 `.go` 文件**（不含 swaggo 生成的 `app/server/docs/`），其中 **319 个生产文件 / 36643 行**，71 个测试文件 / 10602 行。分层按行数排序：service(9.9k) → handler(7.7k) → repository(5.0k) → model(2.4k)，外加 `internal/gateway`(3.7k) 网关层与 `internal/pkg`(2.0k) 基础设施。
- 存储单一 SQL 实现：每个 repository 一个接口 + SQL(GORM) 实现，按 `DB_TYPE` 选择 sqlite / mysql / postgres。
- `NATS_URL` 必填，指向外部 NATS（JetStream KV + Pub/Sub），承载会话凭证读穿缓存、限流、权限缓存同步、设置缓存、浏览量缓冲。进程内 nats-server 已于 2026-10-28 剥离（`natsembedded` 仅供测试）。
- 可观察性已闭环（§9.1）：自研 W3C `traceparent`（刻意不引 OpenTelemetry）→ trace 写进 `request.Context()`，Service/Repository 用 `logger.FromContext(ctx)` 取到同一 `trace_id`；指标按域拆成 10 个文件、独立 Registry，只在 `METRICS_PORT`(9091) 内部 listener 暴露；Prometheus + 13 条告警 + Grafana provisioning + blackbox 探活编排在仓库根 `deploy/`。
- 业务面很宽：认证/MFA、用户/角色/权限/用户组、帖子/评论/点赞/收藏/关注/拉黑、版面(Space 树)、标签、通知/私信、举报/审核、公告、签到/徽章/积分/等级、草稿/修订历史、上传、OAuth 登录 + **自建 OAuth2 Provider** + **OIDC SSO**、API Key、会话管理、安全风控、Webhook、插件目录、备份/回收站、SEO（sitemap/RSS）。
- 端点规模：[router 包](../app/server/internal/router) 实测 **223 处 `POST` + 30 处 `GET`**（共 253 条注册，252 个唯一路径）；`/metrics` 不在此树中，由 9091 内部 listener 暴露；`/swagger/{any:path}` 注册在 `server/iris.go`。
- 构建/测试：`go build ./...`、`go vet ./...`、`go test ./...` 全绿（26 个包，18 个含测试）。

---

## 1. 技术栈

| 维度 | 选型 |
|---|---|
| 语言 / 运行时 | Go 1.25（`go.mod`） |
| Web 框架 | Iris v12（`kataras/iris/v12`） |
| ORM / 驱动 | GORM（sqlite / mysql / postgres） |
| 消息 / KV | NATS（`nats.go`）+ 内嵌 `nats-server`（JetStream） |
| 鉴权 | 服务端会话 sid + opaque OAuth2/OIDC access token；仅 OIDC `id_token` 用 `golang-jwt/jwt/v5`（RS256）；bcrypt（`x/crypto`） |
| 对象存储 | AWS SDK v2 S3（local / S3 双实现） |
| 可观测性 | Prometheus client_golang（独立 Registry，`:9091` 内部 listener）+ logrus（经 `internal/pkg/logger` 结构化封装）+ 自研 W3C `traceparent`（刻意不引 OpenTelemetry，见 §9.1） |
| CLI | Cobra（根命令 + 持久标志 `-e/--env`；子命令只有 `backup` / `import-discourse`，见 §12） |
| API 文档 | swaggo/swag → `docs/` + 根 `openapi.json` |
| HTML 清洗 | bluemonday |
| ID | oklog/ulid（业务主键）、google/uuid |

---

## 2. 目录结构与代码量

```
app/server/
├── main.go                 # 入口：chdir 后执行 cmd.RootCmd（含 swag 注解）
├── cmd/                    # Cobra：root.go(-e dev|prod) + backup.go + import_discourse.go
├── server/                 # iris.go(Start(env) 主流程) + seed.go(种子数据) + wiring.go(handler.Deps 映射)
├── internal/
│   ├── config/             # 环境变量加载 + 校验
│   ├── db/                 # Store 抽象：db.go / sql.go(GORM) / 事务管理器
│   ├── model/              # 纯数据模型（GORM 标签，无业务）
│   ├── repository/         # 数据访问：接口 + SQL 实现（19 个资源已拆 _sql.go）
│   ├── service/            # 业务逻辑：*_iface.go 接口 + *.go 实现
│   ├── handler/            # HTTP 层：接口 + 私有实现 + swag 注解
│   ├── gateway/            # 网关层：auth/permission/ratelimit/tracing/oauth*/sso/apikey/session/security
│   ├── middleware/         # CORS/Trace/Logger/Prometheus/RateLimit/Timeout/PathBodyLimit/AuthGate/Maintenance
│   ├── router/             # router.go + 8 个 routes_*.go（按域拆分）
│   ├── container/          # 依赖装配三段：deps.go / container.go(New) / runtime.go(Start·Shutdown)
│   ├── pkg/                # 通用工具：response/errors/code/settings/pagination/clientip/cookie
│   │   ├── kvstore/        #   KV 抽象（NATS JetStream 实现）
│   │   ├── natsembedded/   #   内嵌 nats-server
│   │   ├── eventbus/       #   NATS 事件总线 + events.go 事件常量
│   │   ├── ratelimit/      #   滑动窗口限流
│   │   ├── tracing/        #   自研 W3C traceparent（无 OTel 依赖）
│   │   ├── testnats/       #   测试用内嵌 NATS 夹具
│   │   └── logger/         #   结构化请求日志 + FromContext 请求级实例
│   ├── mailer/             # 邮件抽象：smtp / sendgrid / resend / log
│   ├── storage/            # 文件存储抽象：local / s3
│   ├── metrics/            # Prometheus 指标：独立 Registry，按域拆 10 个文件 + 9091 listener
│   └── mock/               # 测试替身
├── docs/                   # swaggo 生成的 swagger.yaml/json + docs.go
├── db/migrations/          # 手写迁移 SQL（如 add_banned_scope.sql）
├── Makefile / Dockerfile / .air.toml / go.mod / package.json
```

> 部署编排已移到**仓库根** `deploy/`（2026-10，此前在 `app/server/deploy/`）：`docker-compose.{basic,lite,full,cluster}.yml` + `nginx/` + `prometheus/` + `grafana/` + `blackbox/`，见 §12。

模块实测规模（仅生产文件，即 `*_test.go` 之外的 `.go`）：

| 目录 | 文件 | 行数 |
|---|---|---|
| `internal/service` | 80 | 9876 |
| `internal/handler` | 42 | 7716 |
| `internal/repository` | 60 | 4953 |
| `internal/gateway` | 19 | 3710 |
| `internal/model` | 47 | 2352 |
| `internal/pkg/*`（7 子包） | 10 | 1548 |
| `server` | 3 | 1041 |
| `internal/middleware` | 9 | 819 |
| `internal/router` | 9 | 729 |
| `internal/mock` | 1 | 725 |
| `cmd` | 3 | 636 |
| `internal/metrics` | 10 | 513 |
| `internal/pkg`（根） | 9 | 471 |
| `internal/mailer` | 5 | 460 |
| `internal/storage` | 3 | 343 |
| `internal/container` | 3 | 337 |
| `internal/db` | 4 | 240 |
| `internal/config` | 1 | 132 |
| `main.go` | 1 | 42 |

- 合计：**319 个生产文件 / 36643 行**，另有 71 个测试文件 / 10602 行。
- 测试分布：service 28、`internal/gateway` 13、`internal/pkg` 子包 7、repository 7、middleware 6、metrics 2、model 2，router / handler / db / container / config / `pkg` 根各 1。**`internal/handler` 的 1 个测试文件是 `sse_metrics_test.go`（测指标埋点），不是 handler 行为测试**。
- `go list ./...` = 26 个包（含 `cmd`、`docs`），18 个含测试，全绿。

---

## 3. 启动流程（`server.Start(env)`）

入口 [main.go](../app/server/main.go) → [cmd/root.go](../app/server/cmd/root.go)（Cobra，`-e dev|prod`，默认 `dev`）→ [server/iris.go](../app/server/server/iris.go) 的 `Start(env)`：

1. `loadEnv(env)`：依次加载 `.env` + `.env.{dev|prod}`（godotenv，覆盖式）。
2. `config.Load()` + `Validate()`：已**没有** `JWT_SECRET` 默认值强校验。随后立即 `logger.SetGlobal(logger.New(cfg.Log))`——必须早于任何业务日志，同时接管存量裸 `logrus.X()`。
3. `db.New(cfg.DB)` 建 Store → `AutoMigrate(...)`（**一次性注册 55 个模型**，见 §8）。
4. `backfillDisplayName`（一次性回填 `display_name = username`）。
5. 种子：`seedRoles` → `seedAdmin` → `seedGroups` → `syncDefaultSettings` → `syncModerationConfigs`。
6. 读元标记 `MetaSeeded` 判断是否首启（`alreadySeeded`）。
7. **NATS**：`NATS_URL` 空则 `natsembedded.StartOnPort` 起内嵌实例；随后建 `kvstore`（JetStream bucket）与 `eventbus`。
8. `seedPermissions(...)`：建 PermissionService（载入内存缓存）/ SettingService / BannedService；权限映射幂等（首启或表空才 destructive 重建，避免覆盖管理员自定义）。
9. 从 settings 表初始化运行时参数：`InitOAuthTTL`（`oauth_access_token_ttl`）、`InitPagination`；OIDC `id_token` 的 TTL 不走启动缓存，签发时现读 `sso_id_token_ttl`。
10. **Dev 专属**：`ensureMockData` + `reseedPostsAndContents`。
11. 写回 `MetaSeeded=true`。
12. `pkg.BuildStorageConfig(settings→env→默认)` → `storage.New`。
13. `permSvc.StartSyncSubscriber`（多实例权限缓存 Pub/Sub 同步）。
14. `iris.New()`：`WithRemoteAddrHeader(X-Forwarded-For / X-Real-IP)` → app 级 `CORS` → `Trace(cfg.TracingEnabled)` → `Logger` → `Recover` →（`METRICS_ENABLED` 时）`Prometheus`；同一分支内 `metrics.RegisterDBPoolCollector(store)` 并在 `:{METRICS_PORT}` 起独立指标 listener（失败只 warn，不中断启动）；挂 `/swagger/{any:path}`。
15. 构造共享依赖：`spaceSvc`（并把版面层级注入权限缓存）、`SessionStore`（`user_sessions` 行 + KV 读穿缓存，全链路唯一会话吊销权威）、`SecManager`（暴力破解防护）。
16. `gwDeps := gateway.Deps{SettingService, Perms, Ban, KV, DB}` → `middleware.Deps{Settings, Space, Perms, Ban, Limiter}`。
17. `container.New(container.Deps{...})` 完成全部 repo/service 装配 → `c.Start(ctx)` 订阅事件并起 5 个后台 worker → `server/wiring.go` 的 `newHandler(c)` 把 39 个 service 映射成 `handler.Deps` 后 `handler.New(...)`。
18. `router.Register(app, h, deps, gateway.New(gwDeps))` 注册全部路由。
19. 预检端口占用 → goroutine 内 `app.Listen`。
20. 阻塞等 `SIGINT/SIGTERM` → 10s 优雅关闭：`app.Shutdown` → `metricsSrv.Shutdown`（复用同一 ctx）→ `runtime.Shutdown()`（冲刷浏览量、停各 ticker/worker、关事件总线与通知分发）→ 停权限同步 → 关 KV / 内嵌 NATS / DB。

后台常驻协程（由 `container.Start` 启动、随 `Runtime.Shutdown` 取消，worker 名存于 map 以便关停归因）：`view_count_flusher`、`pin_expiry_checker`、`webhook_delivery`、`weekly_digest`、`trust_promotion`，外加通知侧的 `NotifDispatcher`。`container.New` 本身不产生任何副作用。

---

## 4. 分层架构与依赖装配

请求单向依赖链（不允许跨层）：

```
HTTP Request
  → Middleware / Gateway   (Tracing → RateLimit → APIKey|会话 sid|OAuth access token → BanCheck → Permission → Security → Session)
    → Handler               (解析参数、调 service、统一 JSON 响应、swag 注解)
      → Service             (业务逻辑、权限/所有权校验，返回 *pkg.AppError)
        → Repository        (接口 + SQL 实现，错误包装)
          → Database        (SQL via GORM)
```

关键点：

- **Repository 单一 SQL 实现**：每资源一个 `xxx.go`（定义接口 + 工厂 + SQL 实现）；19 个资源已进一步拆成 `xxx.go`（接口）+ `xxx_sql.go`（实现）。工厂直接返回基于 `store.SQL()` 的实现，Service 只依赖接口。
- **依赖装配三段**：[container.go](../app/server/internal/container/container.go) 的 `New()` 纯构造 36 个 repo + 35 个 service；[runtime.go](../app/server/internal/container/runtime.go) 的 `Start()` 订阅事件并起后台 worker；`Shutdown()` 按序收尾。`New` 无副作用，因此测试可安全调用。
- **共享权威**：`SessionStore`、`PermissionService`、`SettingService`、`BannedService`、`SpaceService` 在 `server/iris.go` 启动期构造，同时交给 middleware、gateway 与 handler，避免两套实例（M-25）。它们以 `PermissionSvc` / `SettingSvc` / `BannedSvc` / `SpaceSvc` / `SessionSvc` 字段名注入 container。
- **HTTP 层不依赖 container**：[server/wiring.go](../app/server/server/wiring.go) 把 39 个 service 逐字段映射成 `handler.Deps`；漏接线是编译错误，「HTTP 层依赖什么」可从一个文件读完。

### 4.1 `internal/gateway` —— 独立网关层

[gateway.go](../app/server/internal/gateway/gateway.go) 聚合 11 个子组件，通过 `Deps`（`pkg.SettingReader` / `pkg.PermissionChecker` / `pkg.BannedChecker` / `KV` / `DB`）与业务解耦，避免循环依赖。接缝接口统一定义在 [internal/pkg/seams.go](../app/server/internal/pkg/seams.go)，与 `internal/middleware` 共用一份声明：

| 子组件 | 文件 | 职责 |
|---|---|---|
| AuthChecker | auth.go | 凭证解析：opaque sid（`Authorization: Bearer` 或 `master_session` cookie）→ `user_sessions` 活行 + 滑动续期；未命中再按 `sha256(cred)` 查 `oauth_tokens`（OAuth2/OIDC opaque access token：`of_at_` + 32B base64url，库中只存 hash，撤销翻 `revoked` 列） |
| PermissionCheckerMiddleware | permission.go | 角色权限校验中间件；路径 → 权限码映射表，**未映射的 protected 路由默认拒绝** |
| RateLimiter | ratelimit.go | 网关级全局限流：读 settings 取上限，计数交给 `internal/pkg/ratelimit` 引擎（`GlobalKey`） |
| Tracing | tracing.go | 路由匹配后把 `ctx.GetCurrentRoute().Path()`（模板，非具体路径）写进 `trace.Name`，即 span 命名；trace 本身由 app 级 `middleware.Trace` 建立（§9.1），此处不调 `ctx.Next()` |
| OAuthManager | oauth.go + oauth_{google,github,discord}.go | **消费方** OAuth 登录（Google/GitHub/Discord；settings 另含 QQ/WeChat/Weibo 开关） |
| OAuthProvider | oauth_provider.go + token.go | **服务端作为 OAuth 2.0 Provider**：客户端注册、authorize、token、revoke |
| SSOProvider | sso.go + sso_oidc.go | **OIDC SSO**：Discovery、JWKS、Token、UserInfo、Authorize |
| APIKeyManager | apikey.go | API Key 认证（`X-API-Key` 或 `Bearer ak_`，与会话 sid / OAuth access token 互斥、优先） |
| SessionManager | session.go | 会话列表 / 单点吊销 / logout-all + 会话跟踪 |
| SessionStore | session_store.go + session_issue.go | 会话权威：签发 / Lookup / Revoke / Rotate，`user_sessions` 行 + KV `sess:<sid>` 读穿 |
| SecurityManager | security.go | 暴力破解、异常位置、安全事件、登录历史 |

两个流水线入口（可组合段）：

- `Preflight()`：tracing → 全局 IP ratelimit → 可选凭证解析。挂在 `v1` party 上，对 `/api/v1` 全量生效（含 protected）。
- `Enforce()`：必需凭证（API Key 或会话）→ banCheck（被封返回 404）→ permission → securityCheck → sessionTracking。挂在 `protected` party 上；若 Preflight 已解析出身份则不再二次查询会话。
- ~~`Handler()`~~：两者的组合入口，**已于 2026-10-27 删除**——生产只挂 `Preflight` + `Enforce`，它仅剩测试在调用。

两段以 iris ctx 上的 stage 标记幂等（`gateway.go` 的 `ranStage`/`markStage`），因此同一段被重复挂载也不会二次计数。

---

## 5. 业务模块清单

`Handler` 聚合体字段 ↔ Service ↔ Repository 对应关系（模块名即文件名前缀；**38 个 handler 接口**）：

| 模块 | Handler 方法（要点） | 说明 |
|---|---|---|
| Auth | Register/Login/Refresh/ChangePassword/FirstChangePassword/Forgot/ResetPassword(ByCode)/Send+VerifyEmail/ApproveRegistration | 会话 sid（64-hex，`user_sessions` 行 + KV 读穿缓存，滑动续期）/ `/auth/refresh` 单用轮换；吊销 = 删行；`Logout` 不在本层，走 `internal/gateway.SessionManager.LogoutHandler`（见 §13）；首登强制改密（meta 标记 R-05）；注册模式（开放/审批/邀请）|
| MFA | VerifyLogin/SetupTOTP/Enable/Disable/RegenerateBackup | TOTP + 备份码（settings 开关）|
| User | Profile/GetByID/List/Search/UpdateProfile/UpdateRole/UpdateTrustLevel/Delete/Activity/GetPublicProfile | 用户标识统一 ULID `User.ID`；改角色自动吊销该用户全部会话 |
| Banned | Ban/Unban/Get/ListByUserID + 版面级 BanInSpace/UnbanInSpace/GetInSpace | 两套独立机制：`role=ban`（封号，Login 拦截）与 `Banned` 表（禁言，BanCheck 拦截写）|
| Role / Permission / Group | Role CRUD；Permission 平台级 + **版面级**（space/effective/tree/sync/clear）；Group CRUD + 成员 + 权限同步 | 默认角色 admin/moderator/user/pending_user/ban；PermissionService 内存缓存 + 层级继承 |
| Post | List/Search/GetByID/GetDetail/ListByIDs/Create/Update/Delete/TogglePin/ToggleLock/SetClosed/MarkRead/Pin Set/Unpin/ListPinned/Suggestions | 所有权校验下沉 service（R-06b）；浏览量 KV 缓冲 + 定时冲刷 |
| Content(评论) | ListByPost/ListReplies/ListAll/Create/Update/Delete/AdminGet/AdminDelete | 支持 parentId 嵌套 |
| Like / Favorite / Follow / Watch / UserBlock | 点赞(内容)、收藏(帖子)、关注(用户)、订阅主题、拉黑 | 各 toggle + 状态查询 + 列表 |
| Tag | List/Create/Update/Delete/GetPostTags/SetPostTags | 按名称批量设置，不存在则自动创建 |
| Space(版面) | Tree/Flat/Roots/Children/Ancestors/Breadcrumb/Descendants/SubTree/Get/Create/Update/Delete/ToggleActive/Reorder/Move | 取代旧的 Category；带图标/slug/排序/启禁/树形层级 |
| Upload | Upload/Presign/Confirm/List/GetFile/GetFileByKey/Delete | local / S3 |
| Notification | List/CountUnread/MarkRead/MarkAllRead/Delete/**Stream(SSE)** | 未读数 KV 缓存；SSE 推送 |
| PM(私信) | ListConversations/CreateConversation/ListMessages/SendMessage/MarkRead | |
| Report / Moderation | 举报 CRUD + 状态；审核 pending/approve/reject + 规则与配置 | |
| Announcement | ListPublished/Get + admin List/Create/Update/Delete/Publish/Unpublish | markdown；toast/alert/modal |
| CheckIn / Badge / Leaderboard / Points | 签到/状态/历史；徽章列表/按用户；排行榜 | 连续签到、积分、信任等级 |
| Draft / Revision / Trash | 草稿保存/列表/删除；帖子与评论修订列表/恢复；回收站列表/恢复 | |
| Invite | List/Create/Delete | 邀请注册 |
| Webhook | CRUD + deliveries + ping + retry | 事件驱动，含 worker 重投 |
| Plugin | List/Install/Update/SetEnabled/Delete | 仅声明式元数据，可执行插件能力缺失 |
| Backup | Create/List/Cleanup/Download/Restore/Delete | `cmd backup.go` 亦可 CLI 触发 |
| OAuth(账号侧) | Providers/GetAuthURL/HandleCallback/Get+LinkedAccounts/Link/Unlink | 消费方 OAuth |
| OAuth Provider / SSO / APIKey / Session / Security | 见 §4.1 | 网关能力，直连 `gw.*` |
| Setting / Admin / SEO / Theme / Realtime / Metrics / Dashboard | 设置读写；统计/搜索/活动/批量/审计日志/对账；sitemap/robots/RSS；主题偏好；通用 SSE；Prometheus；首页聚合 | |
| ImportMap | （`cmd/import_discourse.go`）| Discourse 导入映射 |

**网关自注册的端点**（handler 层无对应文件，由 router 直接挂 `gw.*`）：

```
gw.Session.{LogoutHandler,ListSessions,RevokeSession,LogoutAll}
gw.Security.{ListSecurityEvents,ListLoginHistory,ReportSuspicious}
gw.APIKey.{Create,List,Revoke}
gw.Provider.{RegisterClient,ListClients,UpdateClient,ResetClientSecret,DeleteClient,Authorize,Token,Revoke}
gw.SSO.{Discovery,JWKS,Authorize,Token,UserInfo}
```

---

## 6. 路由与端点

全部注册在 [internal/router](../app/server/internal/router)，前缀 `/api/v1`，按域拆成 `router.go` + 8 个 `routes_*.go`（15 个段函数）。三条挂载路径：

1. **应用根（非 v1）**：`/.well-known/openid-configuration`、`/.well-known/jwks.json`、`/sitemap.xml`、`/sitemap-posts.xml`、`/robots.txt`、`/latest.rss`、`/swagger/{any:path}`。
2. **v1 全局中间件**：`PathBodyLimit(1MB，/api/v1/upload 例外 10MB)` → `RequestTimeout(30s，SSE 自动跳过)` → `gw.Preflight()` → `MaintenanceGate`（维护模式）。
3. **v1 public**（无需登录凭证，部分带 `AuthGate` 动态门控）：health、settings/public、themes/list、leaderboard/list、badges/list、upload/file/{key}；`/auth`（register/login/forgot/reset_by_code/mfa.verify，均限流）；`/oauth`（消费方 provider + 自建 provider 的 token/revoke）；`/sso`（token/userinfo）；`/tags/list`、`/spaces/*`、`/announcements/published|{id}`、`/posts/list|search|{id}|{id}/detail|{id}/suggestions|{postId}/contents/list|{postId}/tags`、`/user/{id}/profile`。
4. **v1 protected**（经 `gw.Enforce()`，其前置 trace/限流/可选凭证已由 v1 层 Preflight 完成）：dashboard、user/role/permission/groups/tags/posts(写)/comments/contents/favorites/watches/users{follow|block}/upload/notifications/realtime/reports/checkin/api-keys/drafts/sessions/themes/security/spaces(写)/admin/*。`/metrics` 不在此树中（见 §9.1）。

统计：router 包实测 **`POST` 223 处、`GET` 30 处**，共 **253 条注册 / 252 个唯一路径**（`/oauth/authorize` 同时注册 GET 与 POST）。含 `/.well-known/*`、`/sitemap*`、`/robots.txt`、`/latest.rss`、`/health` 等系统端点。方法判据为「有无副作用」——写操作 `POST` + 动词后缀，只读查询 MAY `GET`（见 §13.1-5）。

限流点（`middleware.RateLimit`，滑动窗口，KV 支撑，共 **12 个命名端点**）：register 5/min、login 10/min、forgot_password 3/min、reset_password_by_code 5/min、mfa_verify 20/min、send_verify_email 3/min、verify_email 10/min、post_create 10/min、comment_create 30/min、report_create 10/min、pm_conversation_create 20/min、pm_message_create 60/min。

**路由顺序是硬约束**：固定路径必须注册在参数路径之前；`protected := v1.Party("")` 与 `protected.Use(gw.Enforce())` 必须留在 `Register` 的原位置（公开 party 之后、任何 protected 路由之前），因为 Iris 在「注册路由那一刻」对 party 的中间件做快照。`routes_*.go` 里禁止出现任何 `Use(gw.`，由 `middleware_chain_test.go` 源码扫描钉住。

---

## 7. 权限与鉴权体系

- **权限码**：实测平台级 **54 个**（`xxx:yyy` 形式，如 `post:create`、`role:manage`、`security:report`、`reconcile:counters`），无重复；另有**版面级权限**（`space_permission.go` + `SpaceRolePermission` 模型，支持层级继承）。角色映射默认：admin / moderator / user / pending_user / ban。
- **缓存**：`PermissionService` 用 `sync.RWMutex` 维护 `role → set(perm)` 内存缓存，启动 `LoadCache`，变更经 KV Pub/Sub 跨实例同步（`StartSyncSubscriber`）。
- **中间件**：`RequirePermission(code)` 路由级拦截（只额外挂在 `/permission/*`、`/admin/backups`、`/admin/webhooks`）；其余 protected 路径由网关的 path→permission 映射表负责（exact / prefix / pattern / space-pattern 四类匹配，**未映射默认拒绝**）。资源级「作者 OR 有权限」在 service 层判定（R-06b）；`AuthGate(settingKey)` / `SpaceAuthGate` 按系统设置动态切换公开/鉴权。
- **封禁双层**：`role=ban`（封号，登录即拒 + 吊销全部会话）与 `Banned` 表（禁言，BanCheck 拦截写操作，网关层返回 404 而非 403）。
- **鉴权入口**：会话 sid（`Authorization: Bearer <sid>` 或 HttpOnly cookie `master_session`）与 API Key（`X-API-Key` / `Bearer ak_`）互斥、API Key 优先；自建 OAuth2/OIDC Provider 签发的 opaque access token（`of_at_`，库中只存 SHA-256）是第三种，只用于机器对机器；唯一仍是 JWT 的是 OIDC `id_token`（RS256）。
- **身份传递**：网关解析凭证后写入 iris `ctx.Values()`，键名集中在 [internal/pkg/ctxkey.go](../app/server/internal/pkg/ctxkey.go)（`CtxUserID`/`CtxUsername`/`CtxRole`/`CtxSession`/`CtxClaims`/`CtxAuthMethod`/`CtxAPIKeyScopes`/`CtxClientScopes`）。写入方只有 `internal/gateway`；`CtxUserID` 携带的是 `User.ID`（ULID），键名与各表的 `user_uuid` 列都是历史遗留命名。

---

## 8. 数据模型

`internal/model/`：**47 个文件 / 68 个 struct**（含少量请求/响应结构）。`AutoMigrate` 注册 **55 个**持久化模型，覆盖：

- 身份权限：`User`、`Credential`、`Role`、`Permission`、`RolePermission`、`SpaceRolePermission`、`Group`、`GroupMember`、`GroupPermission`、`UserSession`、`APIKey`、`UserMFA`、`OAuthAccount`、`OAuthClient`、`OAuthAuthorize`、`OAuthToken`
- 内容：`Post`、`Content`、`Space`、`Tag`、`PostTag`、`Revision`、`Draft`、`TopicReadState`
- 互动：`ContentLike`、`PostFavorite`、`UserFollow`、`Watch`、`UserBlock`
- 运营：`Notification`、`Report`、`ContentModeration`、`ModerationRule`、`ModerationConfig`、`Announcement`、`Banned`、`AuditLog`、`SecurityEvent`、`LoginHistory`、`Invite`
- 成长：`CheckIn`、`Badge`、`UserBadge`
- 通讯：`Conversation`、`ConversationMember`、`Message`
- 基础设施：`Setting`、`Meta`、`Upload`、`Backup`、`Plugin`、`Webhook`、`WebhookDelivery`、`UserPreference`、`ImportMap`

模型纯数据、无业务，多数提供 `NewXxx()` 构造函数（`constructors.go` + `constructors_test.go`）。

**settings 表**：`DefaultSettings` 定义 **123 个**默认 key，覆盖内容鉴权门控、OAuth 开关（GitHub/Google/QQ/WeChat/Weibo/Discord）、邮件（provider/SMTP/APIKey/验证码策略/通知/周报）、OIDC SSO、API Key、Passkey、注册模式/开关、MFA（TOTP/SMS/Email/备份码）、安全风控（暴力破解/异常位置/IP 黑名单）、备份保留天数。

---

## 9. 基础设施组件

| 组件 | 位置 | 说明 |
|---|---|---|
| DB Store | `internal/db/` | `Store` 接口 + SQL(GORM) 实现 + `BackendType` 枚举 + 事务管理器 + 连接池统计 |
| KV Store | `internal/pkg/kvstore/` | JetStream bucket 存数据、per-key TTL 走 companion bucket、Pub/Sub 走 NATS core（前缀 `nats.kv.`）；业务 key 统一编码为合法 JetStream key |
| 内嵌 NATS | `internal/pkg/natsembedded/` | **仅测试用**（`internal/pkg/testnats`）；生产路径已剥离，`NATS_URL` 必填 |
| EventBus | `internal/pkg/eventbus/` | 队列订阅、集群内每事件只消费一次；`events.go` 集中 **12 个**事件常量 |
| 通知订阅 | `service.SubscribeNotifications` + `NotifDispatcher` | 订阅事件 → 创建通知，解耦业务与通知 |
| Storage | `internal/storage/` | `Provider` 接口：local / S3 |
| Mailer | `internal/mailer/` | smtp / sendgrid / resend / log |
| RateLimit | `internal/pkg/ratelimit/` | 唯一限流引擎：KV 滑动窗口 `Limiter.Check(ctx, policy, key, limit, window)`；`EndpointKey`/`GlobalKey` 两个 key 形状常量。两个调用方——`middleware.RateLimit`（端点级）与 `internal/gateway.RateLimiter`（全局 IP 级）——只做装配与拒绝响应 |
| Tracing | `internal/pkg/tracing/` | 自研 W3C `traceparent`（`00-{32hex}-{16hex}-{2hexflags}`），`crypto/rand` 生成，只收 `00` 版本与小写十六进制；无 OTel/collector 依赖 |
| Logger | `internal/pkg/logger/` | 结构化请求日志（`trace_id`/`span_id`/`method`/`path`/`status`/`latency_ms`/`user_uuid`/`ip`/`route`）；`With()` 链式字段，`FromContext(ctx)` 取请求级实例；详见 §9.1 |
| Metrics | `internal/metrics/`（10 个按域文件）+ `middleware/prometheus.go` | 独立 Registry、DB 池采集；只在 `METRICS_PORT`(9091) 内部 listener 暴露，`/api/v1/metrics` 已删除；详见 §9.1 |
| 统一响应 | `internal/pkg/response.go` / `errors.go` | `Success` / `SuccessPaginated` / `HandleError`；固定 `{code,msg,data}`；**19 个**业务码，分段 1xxx 认证 / 2xxx 参数 / 3xxx 资源 / 4xxx 限流与体积 / 5xxx 服务端，`errToHTTPStatus` 映射 |
| OAuth access token | `internal/gateway/token.go` | opaque `of_at_` + 32B base64url；库中只存 SHA-256 hex（`oauth_tokens.access_token`），撤销翻 `revoked` 列；TTL 走 settings `oauth_access_token_ttl`（启动经 `InitOAuthTTL` 载入） |
| OIDC id_token | `internal/gateway/sso.go` | 唯一保留的 JWT（RS256）；签名私钥 PEM 持久化在 settings `sso_oidc_signing_key`，`kid` 由公钥推导，TTL 走 `sso_id_token_ttl` |
| Swagger | `docs/` + `Makefile swag` | handler 注解 → `docs/` + 根 `openapi.json`；`make swag-check` 供 CI 校验 |

**错误码全表（19 个）**：

| 段 | 码 | 常量 |
|---|---|---|
| 成功 | 0 | `SUCCESS` |
| 1xxx 认证 | 1001 `TOKEN_NOT_EXIST`、1002 `TOKEN_WRONG`、1003 `TOKEN_EXPIRED`、1010 `INVALID_PASSWORD`、1011 `USER_NOT_FOUND`、1012 `USERNAME_EXISTS`、1013 `FORBIDDEN`、1014 `TOKEN_REVOKED`、1015 `USER_BANNED`、1016 `EMAIL_EXISTS` | |
| 2xxx 参数 | 2001 `INVALID_PARAMS` | |
| 3xxx 资源 | 3001 `NOT_FOUND`、3002 `ALREADY_EXISTS` | |
| 4xxx 限流 | 4001 `RATE_LIMITED`、4002 `BODY_TOO_LARGE` | |
| 5xxx 服务端 | 5001 `INTERNAL_ERROR`、5004 `REQUEST_TIMEOUT`、5005 `MAINTENANCE` | |

**事件清单**（`eventbus/events.go`，12 个）：`EventPostCreated`、`EventPostClosed`、`EventContentCreated`、`EventContentLiked`、`EventContentReplied`、`EventContentWatched`、`EventUserCreated`、`EventUserRegistered`（历史别名，等价于 `EventUserCreated`，Auth 同时双发）、`EventUserFollowed`、`EventUserMentioned`、`EventReportCreated`、`EventModerationDone`。

### 9.1 可观察性（trace / 日志 / 指标 / 监控栈）

来源：openspec change `2026-10-13-build-observability`（实现期发现与缺陷清单见 [observability-findings-2026-10-13.md](./reviews/observability-findings-2026-10-13.md)）。三根支柱里 metrics 原本就有骨架，本轮补的是 trace 与「关联标识能穿过层边界」。

#### trace 生命周期

实现在 [internal/pkg/tracing/tracing.go](../app/server/internal/pkg/tracing/tracing.go)，挂载在 [internal/middleware/trace.go](../app/server/internal/middleware/trace.go)：

1. 入站优先级：合法 `traceparent` → `X-Request-ID` 恰为 32 位小写十六进制 → `crypto/rand` 新生成。沿用入站 `traceparent` 时走 `Child()`：trace id 不变、span id 新铸、入站 span id 记为 parent。
2. 畸形值（版本非 `00`、长度错、大写十六进制、全零）一律丢弃并新生成，不 panic、不拒请求。`ParseTraceparent` 自带小写字符集校验。
3. `ctx.ResetRequest(req.WithContext(tracing.ContextWithTrace(...)))` 把 trace 写进 `request.Context()`，这是它能穿过 Handler → Service 边界的唯一原因。
4. 响应头：`X-Request-ID` 恒等于 trace id；`TRACING_ENABLED` 为真时另回写 `traceparent`。
5. `TRACING_ENABLED=false` 只停「解析入站头 + 回写 `traceparent`」，**仍生成 trace 并写入 ctx**，`X-Request-ID` 也照旧回写。
6. span 命名在网关侧完成：`internal/gateway/tracing.go` 在路由匹配后把端点模板写进 `trace.Name`，不调 `ctx.Next()`。

**顺序是硬约束**：`server/iris.go` 固定 `CORS → Trace → Logger → Recover →（可选）Prometheus`。`Recover` 排在 `Logger` 之后，panic 先被接住并写下 500 信封，unwind 到 `Logger` 时状态码已定，请求日志仍带 `trace_id` 与 `status=500`。

#### 只有一个关联 id

原 `request_id` 字段并入 `trace_id`，全链路不再并存两个 id。nginx 内置 `$request_id` 恰好是 32 位十六进制，与 W3C trace id 同形同宽，直接用作回退即可让代理访问日志与后端应用日志共享一个 id。

#### 业务层取请求级 logger

```
middleware.Trace   → tracing.ContextWithTrace          → request.Context()
middleware.Logger  → logger.Global().With(trace_id,span_id,method,path)
                   → logger.NewContext                 → 同一个 request.Context()
Handler            → ctx.Request().Context()
Service/Repository → logger.FromContext(ctx).WithContext(ctx).Info(...)
```

- `logger.SetGlobal(logger.New(cfg.Log))` 紧跟 `cfg.Validate()`、先于任何业务日志，并同步 `logrus.SetLevel/SetFormatter/SetOutput`，使存量裸 `logrus.X()` 调用一并受 `LOG_LEVEL/LOG_FORMAT/LOG_OUTPUT` 管辖。
- 文件输出打不开时回退 `stderr` 并告警，不中断启动；`LOG_LEVEL` 取值非法回退 `info`。
- 后台与消费侧自成 trace 根：EventBus 消费端在新 background ctx 上重建 trace，只搬 trace 值不搬发布方 ctx。

#### 指标：独立 Registry + 按域拆分的 10 个文件

[internal/metrics/](../app/server/internal/metrics) 用自己的 `prometheus.Registry`（非全局默认），经 `promhttp.HandlerFor` 暴露。**26 个指标族**：

| 文件 | 指标族 | 埋点位置 |
|---|---|---|
| `metrics.go` | `orinforum_http_request_duration_seconds` / `http_requests_total` `{method,path,status}`、`http_request_size_bytes`、`http_response_size_bytes`、`http_panics_total` + Go/Process collector | `middleware/prometheus.go`，`path` 经 ID 折叠归一化 |
| `collector.go` | `orinforum_db_pool_*`：max_open / open / in_use / idle / wait_count / wait_duration | 启动时 `RegisterDBPoolCollector(store)` |
| `http.go` | — | `Serve(addr)`：同步 `net.Listen`（端口冲突由调用方 warn 而非 crash） |
| `eventbus.go` | `eventbus_publish_total{event,result}`、`dispatch_duration_seconds{event}`、`dispatch_errors_total{event}`、`queue_depth`、`dropped_total{event}` | `internal/pkg/eventbus` |
| `kv.go` | `kv_operation_duration_seconds{op,outcome}`、`kv_cas_exhausted_total` | `kvstore/observe.go` 单点 helper |
| `gateway.go` | `ratelimit_rejections_total{name}`、`ratelimit_failopen_total{name}` | `internal/pkg/ratelimit/ratelimit.go` 的 `Check`/`recordFailure` |
| `permission.go` | `permission_decisions_total{scope,result}` | `service/permission.go` 具名返回值 + defer |
| `sse.go` | `sse_connections{channel}`（notification / realtime） | 两个 Stream handler |
| `auth.go` | `login_failures_total{reason}`（unknown_user / no_credential / bad_password） | `service/auth.go` 三个失败分支 |
| `webhook.go` | `webhook_deliveries_total{outcome}`、`webhook_delivery_duration_seconds`、`webhook_pending_deliveries` | `service/webhook.go` |

- **基数护栏**：标签取值必须有界——`method`、归一化 `path`、`status`、事件名常量、KV op、判定结果。trace id / span id / 用户 id / 原始 URL 段 / 自由文本 / 错误文本一律不作标签；权限族刻意不带 `role`。由 `internal/metrics/observability_test.go` 的 `TestLabelNamesAreBounded` 钉成测试。
- **seed 策略**：标签静态可知的族在 `init()` 里 seed 零值，首次抓取即有 family；标签运行时才知的族由生产侧在接线点 seed。这样 `MetricsFamilyMissing` 的 `absent()` 才是真探针。

#### 暴露面：9091 内部 listener

`/api/v1/metrics` 已从业务路由删除（此前挂在 `admin:metrics` 权限码之后，Prometheus 匿名抓取拿不到数据），现由 `METRICS_PORT`（默认 9091）独立 listener 暴露，**该端口无鉴权**：

- `deploy/docker-compose.yml` 用 `expose: ["8080","9091"]` 而非 `ports`，只在容器网络内可达；
- `deploy/nginx/nginx.conf` 没有任何指向 9091 的 location，经 nginx 请求 `/metrics` 一律 404；
- `METRICS_ENABLED=false` 时 Prometheus 中间件与该 listener 都不注册；
- `model.PermAdminMetrics` 权限码保留——`internal/gateway/apikey.go` 的 API Key 白名单仍引用它。

#### 监控栈编排（仓库根 `deploy/`）

| 文件 | 内容 |
|---|---|
| `prometheus.yml` | 15s 抓取；`orinforum` → `backend:9091/metrics`；`orinforum-health` → 经 blackbox 探 `http://backend:8080/api/v1/health`；`rule_files: /etc/prometheus/rules/*.yml`；`alertmanagers.targets` 为空（尚未部署 Alertmanager） |
| `prometheus/rules/orinforum.yml` | 4 组共 **13 条**告警：availability、saturation、business、metric-gaps |
| `blackbox/blackbox.yml` | `http_2xx` 模块。`/api/v1/health` 返回 JSON 而非 Prometheus 文本，不能直接当抓取目标，健康类告警必须有 prober |
| `grafana/provisioning/datasources/prometheus.yml` | `uid: orinforum-prom`，指向 `http://prometheus:9090`，默认数据源、不可编辑 |
| `grafana/provisioning/dashboards/orinforum.yml` | file provider，读 `/var/lib/grafana/dashboards` |
| `grafana/dashboards/orinforum.json` | 10 面板：请求速率、5xx 占比、延迟分位、DB 池、KV P99、EventBus、Webhook、SSE/登录/限流、权限判定、panic 计数 |

- 这些文件是**可选素材**：仓库只保留 `docker-compose.yml` 一个编排档，不含监控栈；接入方式是把它们挂到外部 Prometheus / Grafana（抓取目标 `backend:9091`）。backend 在无监控栈时正常启动。
- nginx 侧统一关联 id：`map $http_x_request_id $req_id`，`location /api/v1/` `proxy_set_header X-Request-ID $req_id`，`log_format orinforum_main` 带 `rid=`。**目前只有 `/api/v1/` 透传该头**，`/swagger/` 与前端 location 未设，端到端关联尚未打通。

#### 刻意不用 OpenTelemetry

只实现 W3C Trace Context 的头部格式与 context 传递，不引 OTel SDK / OTLP exporter / collector。理由：仓库没有 collector/Tempo/Jaeger，只加 SDK 会产出无处可去的 span。**兼容缝保留**：入站/出站用的就是 OTel 使用的同一个 `traceparent` 头。代价：没有 span 树、采样、导出与后端可视化，只有「一个 trace id 贯穿日志与响应头」。

#### 已知留白（如实记录，未标完成）

| 缺口 | 为什么没做 | 现有补偿 |
|---|---|---|
| SSE gauge 无 handler 级测试 | 计数路径需要鉴权 + KV 订阅 + 支持 Flusher 的 ResponseWriter，现有测试基建无此夹具 | 只有族级测试；`Inc()` / `defer Dec()` 靠静态正确性 |
| `login_failures_total` 三个 `reason` 无测试覆盖 | `internal/service` 下无任何测试调用 `Login`，覆盖需 mock 全套 | 埋点是三条同构 `Inc()`，位置经 `service/auth.go` 逐行核对 |
| `LOG_LEVEL=debug` 无正向验证 | 非测试代码里 debug 级调用点为 0 | 用 `LOG_LEVEL=warn` 反向判据实测 |
| 监控栈未真实 `up` 过 | 本机 Docker daemon 未运行 | `compose config` 退出码 0；后端指标侧已本地冒烟（`:9091/metrics` 返 200、`/api/v1/metrics` 404） |
| 告警阈值全是初值 | 无 SLO 依据，5xx>5%、P99>1s、登录失败>50/5m 均为拍的 | 需跑满真实流量两周后按分位回归校准 |

---

## 10. 外部依赖（go.mod 直接依赖）

AWS SDK v2 S3、golang-jwt v5（仅 OIDC `id_token` 签名/验签）、google/uuid、iris-contrib/swagger、godotenv、Iris v12、bluemonday、nats-server v2（内嵌）、nats.go、oklog/ulid、prometheus client_golang、logrus、cobra、testify、swaggo/swag、x/crypto、x/oauth2、x/sync、GORM（+ sqlite/mysql/postgres driver）。

---

## 11. 配置与环境变量

`config.Load()` 读取（`internal/config/config.go`）：

| 变量 | 默认 | 说明 |
|---|---|---|
| `PORT` | 8080 | 端口 |
| `DB_TYPE` | sqlite | sqlite/mysql/postgres |
| `DB_DSN` | 空 | 非 sqlite 必填；连接池另有 `DB_MAX_OPEN_CONNS(25)`/`DB_MAX_IDLE_CONNS(10)`/`DB_CONN_MAX_LIFETIME(300)`/`DB_CONN_MAX_IDLE_TIME(60)` |
| `NATS_URL` | 空 | 空则内嵌 nats-server |
| `NATS_BUCKET` | orin_kv | JetStream KV bucket |
| `METRICS_ENABLED` | true | 同时控制 `middleware.Prometheus()` 与 9091 指标 listener；`false` 时两者都不注册 |
| `METRICS_PORT` | 9091 | 指标独立端口，无鉴权，MUST NOT 经 nginx 暴露；`Validate()` 在启用时校验 1–65535 |
| `TRACING_ENABLED` | true | `false` 时不解析入站 `traceparent`/`X-Request-ID`、不回写 `traceparent`，但仍生成 trace 供日志关联 |
| `LOG_LEVEL` | info | debug / info / warn / error；取值非法回退 info |
| `LOG_FORMAT` | json | json / text |
| `LOG_OUTPUT` | stderr | stdout / stderr / 文件路径；文件打不开则回退 stderr 并告警 |
| `ADMIN_PASSWORD` | 空 | 空则随机生成并打印 |

- **存储配置**（`STORAGE_TYPE`/`STORAGE_LOCAL_DIR`/`S3_*`）不在 `Load()` 读取，改由 `pkg.BuildStorageConfig(settings → env → 默认)` 解析，保证 settings 表为唯一可信来源（H-04）。存储为启动期单例，改 settings 需重启。
- **运行时参数**（`oauth_access_token_ttl` / `sso_id_token_ttl`、分页上限/默认值）从 settings 表加载。`JWT_SECRET` 已从 `config.Load()` 与启动校验中删除（不设也能启动）；`jwt_access_token_ttl` / `jwt_refresh_token_ttl` 两个旧 key 不再被读取。
- settings 表 `DefaultSettings` 共 **123 个** key（见 §8）。

---

## 12. 构建与部署

- **构建**：`make` 交叉编译多平台（`windows`/`linux`/`darwin`）；`make swag` 生成 Swagger 并同步 `openapi.json`。
- **Dockerfile**：三个，职责不同。
  - 根 `Dockerfile`：Go + Web 合并镜像（`entrypoint.sh` 同时起后端和 SSR）。**Go 基础镜像仍是 `golang:1.24-alpine`，落后于 `go.mod` 的 `1.25.0`**（见 §13.2）。
  - `app/server/Dockerfile`：仅后端，两阶段（`golang:1.25-alpine` 构建，CGO 开 → alpine 运行，含 sqlite-libs），入口 `orinforum -e prod`。
  - `app/web/Dockerfile`：仅前端（vinext SSR）。
  - 各 compose 服务的 build context 指向 `../../app/server` 或 `../../app/web`，用的是后两个。
- **部署编排**（仓库根 `deploy/`，2026-10 从 `app/server/deploy/` 移出）：

  | 文件 | 形态 |
  |---|---|
  | `docker-compose.yml` | postgres + nats + backend + frontend + nginx（PostgreSQL + 外部 NATS） |

  **这是唯一的编排档**。原 `lite` / `basic` / `full` / `cluster` 四档均已删除（`cluster` 的 PG + nats×3 + 监控栈不再随仓库发布）。`prometheus.yml`、`prometheus/rules/`、`blackbox/`、`grafana/` 保留为可选素材，挂到外部 Prometheus / Grafana 使用；backend 的 `METRICS_PORT`(9091) 仅在容器网络内可达。
- **热重载**：`.air.toml`。
- **CLI 子命令**：只有 `backup` 与 `import-discourse`（[cmd/backup.go](../app/server/cmd/backup.go)、[cmd/import_discourse.go](../app/server/cmd/import_discourse.go)）。环境经根命令的持久标志 `-e/--env` 选择（默认 `dev`），**不存在 `dev`/`prod` 子命令**——`go run main.go dev` 会报 `unknown command`。
- **API 测试**：仓库根 `orinforum_test.sh`（纯 curl 全量脚本）。

---

## 13. 现状偏差与风险（文档 vs 代码）

### 13.1 已修正

**（文档失真）**

1. **`Category` 模块已下线但仍在文档中** → 已从根 `AGENTS.md`、`docs/api-reference.md` 及各层 `AGENTS.md` 移除；`Space`（版面树）承接分类能力。
2. **权限数量失真**（曾写「约 20 个权限码」）→ 现为 **54 个**平台级权限码 + 版面级权限。
3. **`internal/gateway` 未文档化** → 已在根 `AGENTS.md`、`ARCHITECTURE.md` 与本文 §4.1 补入。
4. **大量新模块未收录** → 已在根 `AGENTS.md`「模块说明」与各层 `AGENTS.md` 补入 MFA / PM / Group / Trust / Webhook / Invite / Draft / Revision / Trash / Badge / Leaderboard / Watch / UserBlock / Backup / Plugin / Theme / SEO / Realtime / Security / Session / APIKey。
5. **RPC 风格 GET 偏差** → 2026-10-18 拍板：允许只读查询使用 `GET`，规则改为「按有无副作用选方法」。`router` 的 30 处 GET 在新规则下全部合规，零代码改动。规范经 change `2026-10-18-allow-get-reads` 合入 `openspec/specs/api-style`。
6. **文档路径过时**（`packages/server/...`）→ 已改为 `app/server/...`。
7. **`/metrics` 可见性** → 已从业务树整体删除、改由 9091 内部 listener 暴露（§9.1）；权限码 `PermAdminMetrics` 保留，只剩 `internal/gateway/apikey.go` 的 API Key 白名单引用它。
8. **「存在 `dev`/`prod` 子命令」** → 已按 [cmd/root.go](../app/server/cmd/root.go) 实测改写（只有持久标志 `-e/--env` + `backup` / `import-discourse`），见 §12。`openspec/specs/operations/spec.md` 的相应表述仍错，须走 openspec 提案流。
9. **「`docs/superpowers/` 已忽略」** → 2026-10-28 起已 `git rm --cached`，7 个文件从版本控制移除（本地文件保留），`git check-ignore` 现已命中，与 `.gitignore` 和 `ARCHITECTURE.md` 的表述一致。
10. **`openapi.json` 覆盖率（2026-10-27 补）** → 为 12 个无注解的 handler 文件与 `internal/gateway` 的 5 个组件补齐 61 条 `@Router` 注解，openapi 路径数 185 → **246**；`make swag` 幂等（连跑两次产物哈希一致），`make swag-check` 在提交后即为绿。剩余 6 条 app 根端点因 `@BasePath` 限制有意不标，见 §13.2-39。

**（代码死线与重复实现）**

10. **`internal/gateway/tracing.go` 空函数体却每请求调用** → 实化为「路由匹配后写 span 名」；trace 本体改由 app 级 `middleware.Trace` 建立。
11. **`logger.Config` 是死代码** → `server/iris.go` 在 `cfg.Validate()` 之后立即 `SetGlobal`，`LOG_*` 经 `config` 实际生效。
12. **关联标识在 Handler→Service 边界被静默丢弃** → 改为 `ResetRequest` 写入 ctx，字段更名 `trace_id`，由 `trace_test.go` 锁住。
13. **logger 包 4 个零调用者符号** → 删除，通路收敛到 `NewContext` / `FromContext`。
14. **cluster compose 挂载仓库中不存在的 `./prometheus.yml`** → 补齐 `prometheus.yml` / 告警规则 / Grafana provisioning / blackbox；同批修掉 NATS 锚点 `dependency cycle` 与 Grafana 明文口令。
15. **两份逐字相同的 client_ip helper** → 合并为 `pkg.ClientIP`。
16. **两份逐字相同的登出实现** → 只剩 `internal/gateway/session.go` 的 `SessionManager.LogoutHandler`，挂在 `POST /api/v1/auth/logout`。
17. **`internal/gateway` 本地 `pkgSuccess` 复制品** → 删除，成功响应统一走 `pkg.Success`。
18. **成功信封文案失真** → `msg` 现为协议字面量 `"success"`，与 `openspec/specs/api-style` 钉住的 `{code:0,msg:"success"}` 对齐。
19. **迁移遗留的死鉴权路径** → `middleware.JWTAuth` / `OptionalJWTAuth` / `BanCheck` / `RequireRole` / `RequireOwnerOrPermission` 及 `Deps.Blacklist`、`gateway.Deps.TokenRevoker` 全删。
20. **`pkg/jwt.SigningKey`** → 删除；该包本身随后于 2026-10-18 整包删除：OAuth2/OIDC access token 改 opaque 后，JWT 只剩 OIDC `id_token`。
21. **网关自成一套错误码命名空间** → 93 处响应出口改经 `pkg.Fail` / `pkg.Success`，号段收回共享表（现 19 个码）。`{"code":5030}` → `MAINTENANCE(5005)`→503；超时体字符串码 → `REQUEST_TIMEOUT(5004)`→504（此前 `packages/api-client/src/error-view.ts` 的 `5004` 分支永不命中）。
22. **网关错误体前端读不到** → `client.ts` 只解析 `json.msg` 而网关写 `json.message`，信封统一后原因可达。封禁用户 `404 + 空 body` 改为 `FailStatus(404, USER_BANNED)`。OAuth 2.0 / OIDC 协议端点**故意不并入**，仍是 RFC 6749 形状。
23. **全仓没有 panic recovery** → 新增 [internal/middleware/recovery.go](../app/server/internal/middleware/recovery.go)，堆栈只进日志，响应固定 `{code:5001}`；响应已开写时不伪造信封。3 项 httptest 覆盖。
24. **子 party 重复挂载 `gw.OptionalHandler()` 摘掉 protected 的网关链（2026-10-14 修复）**：Iris 的 `Party()` 仅复制父中间件切片，兄弟 party 的 `Use` 会写进同一数组槽位。app 级中间件条数决定是否有空槽（`METRICS_ENABLED=true` → 5 条 → 有），故 `/tags` `/spaces` `/announcements` `/user` `/posts` 五个公开 party 各自的 `Use(gw.OptionalHandler())` 会把其后注册的 protected 链里的 `gw.Handler()` 换掉。实测默认配置 + 普通注册用户 `POST /api/v1/admin/stats` 返回 200。修复是删掉这五处重复挂载，由 `internal/router/middleware_chain_test.go` 两条测试钉住。
25. **网关流水线不可组合，protected 请求两次过同一限流计数（2026-10-14 修复）**：实测 `rate_limit_enabled=true`、上限 60/分钟/IP 时，第一个 429 落在第 **30** 次 protected 请求；修复后落在第 **58** 次。修法：拆成 `Preflight()` / `Enforce()` 两段，用 stage 标记幂等，并挂载为 `v1.Use(Preflight)` + `protected.Use(Enforce)`。（当时的第三个组合入口 `Handler()` 已于 2026-10-27 删除。）
26. **两套限流实现并存（2026-10-14 合并为单引擎）**：引擎只有 `internal/pkg/ratelimit.Limiter.Check`（返回 `Allowed`/`Denied`/`FailOpen`，埋点与日志单一出口），两个调用方只做装配与拒绝响应。KV key 形状逐字保留（`EndpointKey`/`GlobalKey`），否则升级即清空正在计数的窗口。实测 `login` 10/min 首个 429 落在第 10 次，全局 60/min 落在第 60 次上下。
27. **网关层住在 `pkg/` 却从未对外共享（2026-10-14 迁入 `internal/gateway`）**：消费者只有 `server/iris.go` 与 `internal/router` 两个文件。搬迁为 `git mv` 整包，`git diff --cached -M --stat` 显示 0 insertions / 0 deletions。规则写进 `openspec/specs/layered-architecture`。

### 13.2 仍存在的风险（未改动代码）

**架构层**

28. **`AutoMigrate` 承载全部建表**：55 个模型集中于启动时迁移，且与手写 `db/migrations/*.sql` 并存，存在「迁移来源二元」的维护风险。
29. **插件系统仅元数据**：`Plugin` 只有 install/enable 的声明式记录，无可执行扩展点。
30. **`internal/pkg` 与 `pkg/` 命名易混（2026-10-18 已关闭）**：`app/server/pkg` 现为空目录（不被版本控制跟踪），混淆源消失。
31. **party 中间件仍依赖父切片容量这一隐式条件**：修掉的是「重复挂载」这个具体触发点，Iris 的兄弟 party 共享空闲槽机制本身还在——任何新的子 party `Use` 都可能再次改写邻近 party 的链，只能靠 `middleware_chain_test.go` 与约定拦着。
32. **两个包各有一份同形的鉴权接缝接口（2026-10-27 已合并）**：原 `internal/gateway/deps.go` 与 `internal/middleware/deps.go` 各自声明 `SettingReader`、`PermissionChecker`、`BanChecker` vs `BannedChecker`。现统一到 `internal/pkg/seams.go`：`SettingReader` / `PermissionChecker` / `BannedChecker`（含 `IsBanned` + `IsBannedInSpace`）/ `SpaceReader` 各一份；`gateway/deps.go` 删除，`Deps` 结构体并入 `gateway.go`，`middleware/deps.go` 只留 `Deps` 结构体。

**可观察性层**（逐条来自 [observability-findings-2026-10-13.md](./reviews/observability-findings-2026-10-13.md)）

33. `METRICS_PORT`(9091) 无鉴权，靠部署纪律兜底：有人把 compose 的 `expose` 改成 `ports`、或给 nginx 加一条 location，DB 池与流量内部数就公网可读。缺一道 CI 检查。
34. **告警阈值全是初值**：5xx>5%、P99>1s、登录失败>50/5m 等均无 SLO 依据；`LoginFailureSpike` 的绝对值应改成相对基线。
35. **埋点空白区**：NATS 连接状态与内嵌 server、SSE 写失败与重连、`webhook.Enqueue` 积压、GORM 慢查询、权限缓存规模、备份任务成败仍无指标——其中 NATS 断连与 SSE 写失败是静默故障，代价最高。
36. **端到端关联未打通**：nginx 只在 `location /api/v1/` 透传 `X-Request-ID`，`/swagger/` 与前端 location 未设。
37. **「不引入 OpenTelemetry」这条不可逆取向没有 ADR**：`docs/adr/` 目前只有 `chat-mvp.md`，决策语境只存在 change 的 `design.md`。
38. **测试留白**：SSE gauge 无 handler 级测试、`login_failures_total` 三个 `reason` 无覆盖，原因见 §9.1 末表。

**文档与契约层**（本轮实测新增）

39. **`openapi.json` 曾缺 67 条已挂载路由（2026-10-27 已补 61 条）**：补齐后 openapi 从 **185 → 246** 个路径，与路由表逐条核对后仅剩 6 条 app 根端点未纳入。根因是 12 个 handler 文件（`backup.go` `badge.go` `draft.go` `leaderboard.go` `mfa.go` `plugin.go` `realtime.go` `revision.go` `seo.go` `theme.go` `trash.go` `webhook.go`）以及 `internal/gateway` 的 18 个生产文件（仅 `session.go` 有 1 条 `@Router`）没有 `@Router` 注解。**仍缺的 6 条**是 `/.well-known/openid-configuration`、`/.well-known/jwks.json`、`/sitemap.xml`、`/sitemap-posts.xml`、`/robots.txt`、`/latest.rss`——它们是 app 根路径，而 swag 的 `@BasePath` 固定为 `/api/v1`，标注后会生成 `/api/v1/sitemap.xml` 这类错误路径，因此有意保持不标；这 6 条的权威定义在 `router.go` 的 app 根注册区。
40. **followers/following 方法与注解不一致**：代码是 `POST /users/{userId}/followers`（`routes_engagement.go:44-45`），swag 注解写 `[get]`，`packages/api-client/src/services/follow.ts:17-22` 也用 `post`。openapi 与实际路由因此不一致（注：此条本轮**未修**）。
41. **根 `Dockerfile` Go 版本落后**：`go.mod` 要求 `go 1.25.0`，根 `Dockerfile` 用 `golang:1.24-alpine`（`app/server/Dockerfile` 已是 1.25）。`GOTOOLCHAIN` 默认 `auto` 时会在线拉取 1.25 工具链，所以**不必然构建失败**；代价是与仓库其余镜像不一致、构建期多一次网络依赖，且离线环境会失败。
42. **`docs/superpowers/specs/2026-10-18-web-shadcn-refactor-design.md` 不存在（已修复）**：该文件从未存在于 git 历史。2026-10-28 已把 `app/web/AGENTS.md` 与 `documents/designs/current-ui_design_system.md` 的引用改为指向真实载体 `app/web/app/globals.css` 与 `components/ui/` 原语。同类死链：`ARCHITECTURE.md` 曾引用 `superpowers/specs/2026-10-18-oauth-opaque-tokens-design.md`（实际内容在 `openspec/changes/2026-10-18-oauth-opaque-tokens/`），已改为指向 proposal。

---

## 14. 新增模块 Checklist（后端）

1. `internal/model/xxx.go` — 模型（优先 `NewXxx()`）
2. `internal/repository/xxx.go`（或 `xxx.go` 接口 + `xxx_sql.go` 实现）— `WrapDBErr` 统一错误
3. `internal/service/xxx_iface.go` + `xxx.go` — 业务（返回 `*pkg.AppError`）
4. `internal/handler/xxx.go` — handler + **swag 注解**（缺注解会让端点从 `openapi.json` 消失，见 §13.2-39）
5. `internal/handler/handler.go` — `Deps` 加服务字段、`Handler` 加 handler 字段、`New()` 接线
6. `server/wiring.go` — 把容器里的服务映射进 `handler.Deps`（漏了编译报错）
7. `internal/router/routes_<域>.go` — 路由注册（保持注册顺序；见 `internal/router/AGENTS.md` 的文件表）
8. `server/iris.go` — `AutoMigrate` 追加模型；`server/seed.go` — 种子数据（如需）
9. `internal/container/container.go` — repo/service 构造与顺序；起后台任务或订阅事件则改 `runtime.go` 的 `Start`（`New` 必须保持无副作用）
10. 若新增错误语义：`internal/pkg/errors.go` 登记码与中文文案，并同步 `packages/api-client` 的 `ErrorCode`
11. 若新增权限码：`internal/model/permission.go` 加常量与 `DefaultPermissions` 条目，并在 `internal/gateway/permission.go` 的 `initRouteMap` 登记路径映射（**未映射默认拒绝**）
