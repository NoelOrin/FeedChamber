# OrinForum

Go + React 社区论坛 monorepo。后端 Iris v12，前端 Web (React 19 + vinext + TanStack Query + shadcn 原语)；管理后台内嵌在 Web 的 `/admin` 段（与前台共用 shadcn 原语，纯 CSR）。


架构总览、分层依赖与各层约定文档入口见 [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)；后端现状梳理（实测规模、模块清单、网关层、现状偏差）见 [docs/backend-overview.md](docs/backend-overview.md)。

> 已采纳的稳定规范（API 风格、分层架构、服务设计、模块约定、前端共享包、运行配置）以 [openspec/specs/](openspec/specs/) 为单一可信来源；规范冲突时以 specs 为准，后续变更须经 `openspec` 提案流。
## 项目结构

```
app/server/
├── main.go                    # 入口，chdir 到自身目录后调用 cmd.RootCmd
├── cmd/root.go                # Cobra 根命令，持久标志 -e/--env（子命令：backup / import-discourse / recompute-xp）
├── server/
│   ├── iris.go                # Start(env) — 应用启动主流程
│   └── seed.go                # 角色、权限、管理员、设置等种子数据
├── internal/
│   ├── config/                # 环境变量加载 (PORT/DB_TYPE/DB_DSN/Storage)
│   ├── db/                    # 数据库抽象层
│   │   ├── db.go              #   Store 接口 + 工厂 (sqlite/mysql/postgres)
│   │   └── sql.go             #   GORM 实现
│   ├── model/                 # 纯数据模型 (GORM 标签，无业务)
│   │   ├── user.go role.go permission.go
│   │   ├── post.go content.go tag.go space.go
│   │   ├── like.go favorite.go follow.go
│   │   ├── checkin.go upload.go notification.go report.go setting.go
│   │   ├── announcement.go meta.go moderation.go oauth.go
│   │   ├── trust.go             # XP / 等级 / 连续签到阶梯 / 人工调整
│   │   ├── gate.go              # 门控规则 / 能力域默认动作
│   │   └── credential.go banned.go
│   ├── repository/            # 数据访问层 (每资源一个文件，接口+SQL实现)
│   │   └── checker_sql.go     #   内容审核检查器 (SQL)
│   ├── service/               # 业务逻辑层 (接收 repo 接口，返回 *pkg.AppError)
│   │   ├── *_iface.go         #   接口定义独立于实现
│   │   └── *.go               #   实现 + 构造函数
│   ├── handler/               # HTTP 层 (43 文件 / 8700 行，39 个 XxxHandler；3 个测试文件)
│   │   ├── handler.go         #   Handler 聚合体 + New() 依赖装配 → container
│   │   ├── ctx.go             #   currentSession/currentUser/currentRole 等上下文工具
│   │   ├── params.go          #   分页/排序等通用请求解析
│   │   ├── dashboard.go       #   首页聚合接口
│   │   ├── moderation.go      #   内容审核 Handler
│   │   ├── auth.go user.go banned.go announcement.go
│   │   ├── post.go tag.go content.go space.go
│   │   ├── like.go favorite.go follow.go watch.go user_block.go
│   │   ├── upload.go notification.go pm.go report.go checkin.go
│   │   ├── admin.go setting.go permission.go role.go oauth.go group.go
│   │   ├── mfa.go invite.go draft.go revision.go trash.go backup.go
│   │   ├── webhook.go plugin.go badge.go theme.go leaderboard.go
│   │   └── seo.go realtime.go
│   │                          #   注：登出/会话/安全事件/API Key/OAuth Provider/OIDC 端点
│   │                          #   在 internal/gateway 内自注册，/metrics 走 9091 独立 listener
│   ├── gateway/               # 网关层 (18 生产文件 3869 行 + 13 测试文件)：auth / permission /
│   │                          #   ratelimit / tracing / OAuth 消费方 / OAuth Provider /
│   │                          #   OIDC SSO / API Key / session / security，见 §Gateway
│   ├── middleware/            # 身份确定后的门控与请求基础设施 (9 文件)
│   │   ├── auth.go            # AuthGate (站点级动态登录门控) / RequirePermission
│   │   │                      # RequirePermission (权限码)
│   │   ├── trace.go           # Trace：建立/沿用 traceparent，必须先于 Logger
│   │   ├── middleware.go      # CORS / Logger / PathBodyLimit
│   │   ├── request_timeout.go # 软超时（SSE 跳过）
│   │   ├── ratelimit.go       # 命名端点的每 IP 预算（调 internal/pkg/ratelimit 引擎）
│   │   ├── maintenance.go     # MaintenanceGate：维护期只放读类 RPC
│   │   ├── prometheus.go      # HTTP 指标埋点（路径 ID 段折叠）
│   │   ├── recovery.go        # Recover：panic → 500 信封（挂在 Logger 之后）
│   │   └── deps.go            # Deps 五接口（Settings/Space/Perms/Ban/Limiter）
│   ├── router/router.go       # 所有路由注册
│   ├── container/container.go # AppContainer 依赖注入聚合
│   ├── pkg/                   # 通用工具
│   │   ├── response.go        #   统一响应 Success / HandleError / 分页
│   │   ├── errors.go          #   ErrCode + AppError + 错误码映射
│   │   ├── code.go            #   验证码生成
│   │   ├── settings.go        #   BuildStorageConfig settings→env→default
│   │   ├── pagination.go      #   分页规范化
│   │   ├── kvstore/           #   KV 存储抽象 (NATS JetStream)
│   │   ├── natsembedded/      #   内嵌 nats-server (JetStream)
│   │   ├── ratelimit/         #   唯一的滑动窗口限流引擎（Check + 键形状 + 指标/日志发出）
│   │   ├── eventbus/          #   NATS 事件总线
│   │   └── logger/            #   结构化请求日志
│   ├── mailer/                # 邮件发送抽象 (smtp/sendgrid/resend/log)
│   ├── storage/               # 文件存储抽象 (local / S3)
│   ├── metrics/               # Prometheus 指标注册表 (独立 Registry)
│   └── mock/                  # 测试替身 (mock user repo 等)
├── docs/                      # swaggo 生成的 Swagger 文档
├── db/migrations/             # 手写迁移 SQL (与 AutoMigrate 并存)
├── Makefile                   # 构建脚本 (含 swag / linux / darwin / windows 等 target)
├── .air.toml                  # 热重载 (air)
├── go.mod / go.sum
└── package.json               # pnpm workspace 集成

app/server/db/            # SQLite 数据库文件目录 (app.db / migrations)
app/server/uploads/       # 本地上传目录

deploy/                   # 容器编排（仓库根）：docker-compose.yml 默认档 + .cluster.yml + nginx/ + prometheus/ + grafana/ + blackbox/

app/web/                  # Web 前端 (React 19 + vinext App Router；旧 Solid 树已删除，见 git 历史)
packages/api-client/      # 前端 API 共享包 (类型 + 客户端 + 常量 + 认证工具)
packages/sse-client/      # 框架无关 SSE 客户端 (Bearer Auth + 自动重连)
packages/shared-utils/    # 跨前端共享工具函数 (时间格式化 / cx() class合并)
```

## 开发架构

### API 风格约定（强制）

**HTTP 方法按「有无副作用」选择：写操作 `POST`，只读查询 MAY `GET`。URL 使用 RPC 风格，禁止 RESTful。** 这是项目的硬性约定，前后端、openapi.json、所有新增模块都必须遵守。

- **方法**：
  - 写操作（创建 / 更新 / 删除 / 状态变更 / 任何改变服务端状态的请求）**必须 `POST`**。
  - 只读查询**可以 `GET`**（单资源读取、轻量状态查询），也可以 `POST`；列表 / 分页 / 多条件筛选**推荐 `POST` + `/list`**，参数走 body。
  - **不得**用 `GET` 承载任何有副作用的操作；`PUT` / `PATCH` / `DELETE` 一律禁止用于业务接口。
  - 同一资源的读取**只保留一种方法**，不允许 `GET` / `POST` 双读入口。
  - 系统 / 协议 / 文件类端点**必须 `GET`**：`/health`；sitemap / robots / RSS；OAuth2 / OIDC 协议端点（`/.well-known/*`、authorize、callback、userinfo）；文件字节读取（`/upload/file/{key}`、`/upload/{id}` 302）；认证流程端点（`GET /oauth/{provider}/auth`）。
- **URL**：写接口动词后缀化，路径段表达"资源 + 动作"。统一使用以下动作词：
  - `/list` — 列表 / 分页查询
  - `/create` — 创建
  - `/{id}/update` — 更新
  - `/{id}/delete` — 删除
  - `/{id}/status` — 状态变更（如举报处理、禁言等）
  - `/{id}/<custom>` — 其他业务动作（如 `/pin`、`/lock`、`/publish`）
  - 只读 `GET` 用资源标识或既有业务名词（`/posts/{id}`、`/checkin/status`），不强制动词后缀。
- **参数传递**：`POST` 一律走 request body（`application/json`），不得用 query string 承载业务参数；`GET` 允许 query 承载过滤与分页。分页字段固定 `page` / `pageSize`。
- **响应**：统一 `pkg.Success(ctx, data)` 或 `pkg.SuccessPaginated(ctx, data, total)`，固定 `{ "code": 0, "msg": "success", "data": ... }`。
- **前端 service**：写请求 `method: 'POST'`，URL 必须带显式动词后缀（如 `/spaces/create` 而非 `/spaces/`）；只读查询 MAY `GET`，但必须与后端方法一致。

**反例（禁止）**：
- ❌ `PUT /spaces/{id}`、`PATCH /spaces/{id}`
- ❌ `DELETE /spaces/{id}`
- ❌ `GET /spaces/{id}/delete`（写操作不得用 GET）
- ❌ `POST /reports/list?status=pending&page=1`（POST 不得用 query 承载业务参数）
- ❌ 同一读取路径同时提供 `GET` 与 `POST`

**正例**：
- ✅ `POST /spaces/list`（body 可为空对象）
- ✅ `POST /spaces/create`，body `{ name, description }`
- ✅ `POST /spaces/{id}/delete`
- ✅ `POST /reports/list`，body `{ status, page, pageSize }`
- ✅ `GET /posts/{id}`、`GET /user/{id}/profile`、`GET /checkin/history?year=2026&month=10`（只读查询）

> 规则来源：[openspec/specs/api-style/spec.md](openspec/specs/api-style/spec.md)。历史：2026-06-04 曾要求「一律 POST」，因与 OAuth/OIDC/爬虫/文件端点语义冲突且从未被执行（`router.go` 现存 30 处 GET），已于 2026-10-18 经 change `2026-10-18-allow-get-reads` 改为「按副作用选方法」。

### 分层模式

请求处理严格遵循单向依赖链：

```
HTTP Request
  → Gateway / Middleware (Preflight: Trace / 全局限流 / 可选凭证 → Enforce: 会话sid|APIKey / Ban / Permission / Security)
    → Handler (解析参数、调 service、封装响应)
      → Service (业务逻辑、返回 AppError)
        → Repository (数据访问接口)
          → Database (SQL via GORM)
```

各层职责清晰隔离，不允许跨层调用。

### 关键设计

**1. Repository 单一 SQL 实现**  
每个 repository 文件定义接口，工厂函数直接返回基于 `store.SQL()` 的 SQL 实现。Service 层只依赖接口，不关心底层存储。示例模式：

```go
type UserRepository interface { ... }
func NewUserRepository(store db.Store) UserRepository {
    return &sqlUserRepo{db: store.SQL()}
}
```

**2. 统一响应**  
**所有** HTTP 出口——handler、`internal/gateway` 的鉴权/权限/限流/会话/安全端点、以及 maintenance 与 request-timeout 两个中间件——都只能通过 `pkg.Success(ctx, data)` / `pkg.SuccessPaginated(ctx, items, total)` / `pkg.Fail(ctx, code, msg?)` / `pkg.FailStatus(ctx, status, code, msg?)` / `pkg.HandleError(ctx, err)` 写响应。禁止裸用 `ctx.JSON` / `ctx.StopWithJSON` / `ctx.WriteString` 拼信封（唯一例外：OAuth 2.0 / OIDC 协议端点必须保持 RFC 形状，如 `{"access_token":…,"token_type":"Bearer"}` 与 `oauthError()`）。响应体固定为：

```json
{ "code": 0, "msg": "success", "data": {...} }
```

错误码是**单一共享命名空间**（`internal/pkg/errors.go`），网关与 handler 共用，不得自造号段；`msg` 取该码的默认中文文案，需要补充具体原因时作为第二个参数传入。分段：`1xxx` 认证/凭证、`2xxx` 参数、`3xxx` 资源、`4xxx` 限流与请求体、`5xxx` 服务端。`errToHTTPStatus` 负责业务码 → HTTP status（`5004→504`、`5005→503`）。`FailStatus` 只在"状态本身是契约"时使用，例如封禁用户返回 404 而非 403，避免泄露资源存在性。

**3. 依赖装配**  
`internal/container` 分两步：`container.New(Deps)` 只构造对象图（不起协程、不订阅事件、不写库），`(*Container).Start(ctx)` 才订阅事件并启动 5 个后台 worker，返回的 `*Runtime` 负责关停。`server/iris.go` 编排 `config → db.Store → AutoMigrate → seed → NATS/KV/EventBus → 共享服务 → container.New → Start → gateway.Deps → middleware.Deps → handler.New(wiring.go 映射) → Router → Listen`。handler 层不引用 container：它消费 `handler.Deps` 里显式列出的服务，映射写在 `server/wiring.go`，所以「HTTP 层依赖什么」可在一个文件里读完，新增服务漏接线会编译报错。

**4. 权限系统**  
启动时 seed 56 个平台级权限码 + 默认角色-权限映射（admin/moderator/user/pending_user/ban），另有版面级权限（`SpaceRolePermission`，支持层级继承）。`PermissionService` 通过 `sync.RWMutex` 维护 `map[string]map[string]struct{}` 内存缓存 (role → permission codes)。middleware 通过 `RequirePermission(code)` 在路由级拦截。资源粒度的"作者 OR 有权限"校验在 service 层实现（R-06b 方案），注入 `PermissionService` 到 PostService/ContentService。

**5. 路由组织**  
`internal/router` 按域拆分（`router.go` + `routes_<域>.go`），只挂路由与中间件——网关实例由 composition root 传入（`Register(app, h, deps, gw)`）。网关流水线是**可组合的两段**：`v1.Use(gw.Preflight())`（trace + 全局 IP 限流 + 可选凭证解析，对 `/api/v1` 全量生效）与 `protected.Use(gw.Enforce())`（必需凭证 → banCheck（被封 404）→ 权限 → 安全风控 → 会话跟踪）。两段各自幂等，因此一个 protected 请求的限流与凭证解析都只发生一次（此前 `Handler()` 会把 preflight 再跑一遍，实测 60/分钟的上限在第 30 次就 429）。权限粒度直接标注在路由上。

### 新增模块 checklist

添加新功能需改动的文件（后端）：
1. `internal/model/xxx.go` — 数据模型（优先使用 `NewXxx()` 构造函数）
2. `internal/repository/xxx.go` — 接口 + SQL 实现（`WrapDBErr` 统一错误）
3. `internal/service/xxx_iface.go` + `internal/service/xxx.go` — 业务逻辑（返回 `*pkg.AppError`）
4. `internal/handler/xxx.go` — HTTP handler + swag 注解
5. `internal/handler/handler.go` — Handler struct 添加字段，New() 中注册
6. `internal/router/router.go` — 添加路由
7. `server/iris.go` — AutoMigrate 添加模型
8. `server/seed.go` — 种子数据
9. `internal/container/container.go` — 新 repo/service 的构造与顺序；若它起后台任务或订阅事件，改 `runtime.go` 的 `Start`

添加新 API（前端）：
10. `packages/api-client/src/types.ts` — 类型定义
11. `packages/api-client/src/services/xxx.ts` — API service 模块
12. `packages/api-client/src/constants.ts` — 若有新的权限码/设置 key
13. `app/web/app/(app)/…` + `app/web/components/<domain>/` — Web 页面与组件（App Router）
14. `app/web/app/admin/<route>/page.tsx` — 后台管理页面（DataTable/Descriptions + useQuery，直接调 `@orinforum/api-client`；新建目录即新增 `/admin/<route>` 路由，菜单在 `app/web/components/admin/layout/admin-shell.tsx` 的 `menuItems` 登记，权限码在 `app/web/lib/admin/auth/route-access.ts` 登记）

## 模块说明

### Auth — 认证 (`handler/auth.go`, `service/auth.go`)
注册、登录、会话管理、密码变更。登录签发**服务端会话**：凭证是 64 位十六进制 sid，写入 HttpOnly `master_session` cookie，同时允许用 `Authorization: Bearer <sid>` 携带；sid 对应 `user_sessions` 表行 + KV `sess:<sid>` read-through 缓存。没有 token 黑名单——"凭证是否有效"等价于"会话行是否存在"，登出/改密/改角色/封号都直接删行。`/auth/refresh` 是单用轮换：签发新 sid 并立即作废旧 sid，Bearer 客户端必须采纳响应里的新 `access_token`。bcrypt cost 与密码最小长度通过 settings 表动态配置。支持邮箱验证码流程（forgot/reset_password_by_code/first_change_password），`FirstChangePassword` 通过 meta 表 KV 标记首次改密。用户登录态与对外 OAuth/OIDC 的 access token 都已是 opaque：前者是服务端会话 sid，后者是 `of_at_` + 32 字节随机串（服务端只存 SHA-256 hex 于 `oauth_tokens.access_token`，撤销翻 `revoked` 列）。`pkg/jwt` 整包已删除，唯一保留的 JWT 是 OIDC `id_token`（RS256，规范强制；签名私钥 PEM 持久化在 settings `sso_oidc_signing_key`，`kid` 由公钥推导，TTL 走 `sso_id_token_ttl`）。

### User — 用户 (`handler/user.go`, `service/user.go`, `repository/user.go`)
用户 CRUD、资料编辑 (username/avatar/bio)、角色变更(UpdateRole 吊销该用户全部会话)、禁言/解禁。密码字段 JSON 序列化时隐藏。用户标识统一使用 `User.ID`（ULID），不再使用 `UUID` 字段。

### Banned — 禁言 (`handler/banned.go`, `service/banned.go`, `repository/banned.go`)
全局/版面两级禁言管理，支持指定禁言时长（永久或临时）。两套独立机制：role="ban"（封号，Login 拦截）+ Banned 表（操作层面禁言，网关 banCheck 拦截写操作）。封号时 `UpdateRole` 自动吊销该用户全部会话。用户标识通过 `resolveUserID()` 统一解析（支持 ID 或 UUID 输入）。

### Post — 帖子 (`handler/post.go`, `service/post.go`, `repository/post.go`)
帖子 CRUD、搜索、浏览计数、批量查询（ListByIDs）。R-06b 方案下 `Update/Delete` 走 service 层所有权检查（`post.AuthorID == actorID || hasPerm`），router 权限门仅为 `PermPostCreate`。发帖的初始状态由系统设置 **`moderation_mode`** 决定：`off` 不审核 / `async` 发布后异步审核（**默认**，命中转人工）/ `external` 发布前调外部审核服务 / `manual` 全部人工；后台入口在「设置 → 内容」与 `/admin/moderation` 顶部卡片，规范见 `openspec/changes/2026-10-29-post-moderation-modes/`。

### Content — 评论 (`handler/content.go`, `service/content.go`, `repository/content.go`)
帖子评论 CRUD，支持 parentId 嵌套。R-06b 方案下 `Update/Delete` 走 service 层所有权检查。`AdminDelete` 供管理员后台直接删除。

### Tag — 标签 (`model/tag.go`, `handler/tag.go`, `service/tag.go`, `repository/tag.go`)
标签系统，帖子多对多关联。通过 SetPostTags 按名称批量设置标签（不存在则自动创建）。

### Like — 点赞 (`model/like.go`, `handler/like.go`, `service/like.go`, `repository/like.go`)
帖子/评论点赞 toggle 模式。toggle 后同步更新 `Content.LikeCount` 计数器字段。

### Favorite — 收藏 (`model/favorite.go`, `handler/favorite.go`, `service/favorite.go`, `repository/favorite.go`)
帖子收藏 toggle 模式，支持按用户列出收藏列表。toggle 后同步更新 `Post.FavoriteCount`。

### Follow — 关注 (`model/follow.go`, `handler/follow.go`, `service/follow.go`, `repository/follow.go`)
用户关注系统，路由用 `/users/{userId}` (复数)，与 `/user` (单数) 区分。禁止自关注。

### Role — 角色 (`handler/role.go`, `repository/role.go`)
角色 CRUD + 进程级缓存。默认角色：admin、moderator、user、ban。R-02 已增加删除保护（内置角色不可删、有用户分配不可删、级联清理关联权限）。

### Permission — 权限 (`model/permission.go`, `handler/permission.go`, `service/permission.go`, `repository/permission.go`)
权限定义 + 角色-权限映射。PermissionService 维护内存缓存，启动时从 DB 加载。R-04 已实现幂等 seed（alreadySeeded 元标记门控，重启不覆盖自定义权限）。R-07 已实现 kvstore Pub/Sub 多实例缓存同步。

### Upload — 上传 (`model/upload.go`, `handler/upload.go`, `service/upload.go`, `repository/upload.go`, `storage/`)
文件上传抽象层，支持本地存储和 S3。提供预签名签名、上传确认、文件列表和删除。

### Notification — 通知 (`model/notification.go`, `handler/notification.go`, `service/notification.go`, `repository/notification.go`)
用户通知系统，支持列表、未读计数、标记已读、批量标记已读和删除。`/stream` 端点以 SSE 方式推送未读数，前端通过 `@orinforum/sse-client` 的 `SSEClient` 订阅（自动重连 + token 刷新）。未读计数使用 KV Store 缓存，实时更新无需查 DB。

### Report — 举报 (`model/report.go`, `handler/report.go`, `service/report.go`, `repository/report.go`)
内容举报系统，支持创建、列表和状态更新（待处理/已处理/已关闭）。

### Admin — 管理 (`handler/admin.go`, `service/admin.go`, `repository/admin.go`)
管理员统计、搜索、用户活动记录、批量操作（delete/set_role/pin/lock）。搜索支持各实体独立分页（`EntitySearchResult[T]` 泛型结构）。

### Setting — 设置 (`model/setting.go`, `handler/setting.go`, `service/setting.go`, `repository/setting.go`)
系统设置管理，支持公开设置读取、所有设置获取、单个设置读写、批量设置。S3 配置统一以 settings 表为唯一可信来源（H-04 修复）。

### Space — 版面 (`model/space.go`, `handler/space.go`, `service/space.go`, `repository/space.go`)
社区版面系统，**取代原 Category 分类模块**，扩展为带图标、slug、排序、启禁状态的树形版面体系。支持树形/扁平/祖先/后代/面包屑等查询，帖子关联 space_id，列表接口支持 spaceName 筛选。Dashboard 提供版面管理 CRUD 页面。删除前检查关联帖子。

### Check-in — 签到 (`model/checkin.go`, `handler/checkin.go`, `service/checkin.go`, `repository/checkin.go`)
每日签到系统，支持签到、获取状态（今日是否签到、连续签到天数、最大连续天数、总积分）、按月查询历史记录。每天限签一次，数据库复合唯一索引防并发重复签到。签到**不再**直写积分与等级：`check_ins` 只是 XP 聚合的输入（`checkin_count` + 连续里程碑），由 `TrustService.Recompute` 统一刷新快照。

### Trust / XP — 成长体系 (`model/trust.go`, `service/trust_level.go`, `repository/trust.go`, `handler/trust.go`)
XP 是**行为聚合**而不是累加计数器：`total_xp = Σ(指标计数 × 当前 XP 规则) + Σ(连续签到里程碑) + Σ(人工调整)`，因此删帖 / 取消点赞收藏 / 取消精华 / 审核拒绝都会自动回收，改分值立即对全员生效（配置写后本地失效并广播 `trust:sync`，多实例同步失效）。四张配置表 `xp_rules` / `xp_streak_bonuses` / `trust_levels` / `xp_adjustments` 全部后台可配；等级 = 累计 XP 定级 + 滚动窗口保级（可升可降，逐级下探），`manual_only` 等级仅人工授予，管理员手工设级（`users.level_manual`）不被重算打回。`TrustService.Recompute` 是 `users.total_xp` / `trust_level` / `level_metrics` 的唯一写入点；发帖/评论/点赞/收藏/加精/签到/审核等触发点只调它。每日 ticker 全量重算一次；`go run main.go -e prod recompute-xp` 可手动对账。`total_points` 列与 API 别名均已在 P4 下线（字段统一 `total_xp`）；签到 `points_earned` 恒 0，旧 `trust_tl0_*` 设置不再被读取。

### Visibility / Gate — 可见性与门控（P2/P3，`service/visibility.go`、`service/gate.go`、`routes_gate.go`）
- **可见性**：`spaces.visibility ∈ public|login|level` + `spaces.min_level`，`posts.min_level` 可单帖覆盖；有效策略沿祖先链取最严格一档（`BuildVisibilityIndex`）。六个内容出口（列表 / 详情 / 搜索 / sitemap / RSS / 批量）共用同一谓词：聚合出口 SQL 过滤、详情 403 并提示所需/当前等级、作者例外、游客 = TL0；有 `report:manage` 的审核者不受限。评论列表、回复列表与评论创建等从属内容出口通过 `PostViewer.CheckViewablePost` 复用同一门槛（帖子不可见时同样 404/403，也不能盲写评论）。`AuthGate` 退化为站点级登录墙，`SpaceAuthGate` 删除。
- **门控规则引擎**：`gate_rules(priority, domain, conditions JSON, action)` + `gate_domain_defaults`，谓词集固定（等级/XP/版面/标签/作者等级/是否作者/游客/邮箱验证/封禁/暗区/外链/窗口指标），动作仅 `allow|deny|require_review|rate_limit`；按优先级降序首个命中，无命中走域默认；配置快照 + `gate:sync` Pub/Sub 失效，命中写带 trace 的日志，后台 `/admin/gates` 支持 CRUD、域默认与「以用户试算」。发帖/评论（含正文外链）/外链已接管，`trust_tl0_*` 旧门删除。
- **后台**：`/admin/levels`、`/admin/xp-rules`（P1）、`/admin/gates`（P3）；版面管理页可配可见性；用户详情「成长」页展示 XP 分解与人工调整。权限码 `level:manage` / `gate:manage`。

### Announcement — 公告 (`model/announcement.go`, `handler/announcement.go`, `service/announcement.go`, `repository/announcement.go`)
公告系统，支持 markdown 内容、草稿/发布状态、置顶排序、过期时间、浏览计数。显示组件类型：toast(1)/alert(2)/modal(3)。

### OAuth — 社交登录 (`handler/oauth.go`, `service/oauth.go`, `repository/oauth.go`)
多提供商 OAuth2 登录/绑定功能。消费方支持 Google、GitHub、Discord（settings 另含 QQ/WeChat/Weibo 开关）。Callback 使用 KV Store 存 CSRF state，已验证的 LinkAccount 跳过 state 校验。支持 RefreshToken 透传存储。服务端同时可作为 OAuth 2.0 Provider 与 OIDC SSO IdP，见下节网关层。

### Email — 邮件 (`mailer/`, `service/email.go`)
邮件发送抽象层，支持 SMTP/SendGrid/Resend/Log 四种实现。用于邮箱验证码流程（forgot/reset_password_by_code）与通知/周报（`service/digest.go` 周报 ticker）。

### Gateway — 网关层 (`internal/gateway/`)
请求流水线里的认证与授权子系统（2026-10-14 由 `pkg/gateway` 迁入 `internal/`：它只被 `server/iris.go` 与 `internal/router` 消费，从未对外共享，放在 `pkg/` 只换来 `internal/pkg` vs `pkg/` 的命名混淆）。用 `Deps`（`SettingService SettingReader` / `Perms PermissionChecker` / `Ban BanChecker` / `KV kvstore.Store` / `DB *gorm.DB`）与业务解耦，聚合以下子组件。`Deps` 由 `server/iris.go` 一次构造并复用给 SessionStore / SecurityManager / Gateway 三个组件；`KV` 此前声明为 `any`（号称避免直接依赖，实际是同包早已 import kvstore），其代价是每个消费点做一次运行时断言、接错线时静默降级成"限流关闭/无会话"，现为具体类型。
- `AuthChecker`(auth.go) / `PermissionChecker`(permission.go) / `RateLimiter`(ratelimit.go) / `Tracing`(tracing.go) — `Tracing` 在路由匹配后把端点模板写进 trace（`route` 日志字段），trace 本身由 app 级 `middleware.Trace` 建立
- `OAuthManager`(oauth.go + oauth_{google,github,discord}.go) — 消费方 OAuth 登录
- `OAuthProvider`(oauth_provider.go + token.go) — 服务端作为 OAuth 2.0 Provider：客户端注册 / authorize / token / revoke；access token 为 opaque（`of_at_` + 32B base64url，库中只存 SHA-256 hex，撤销翻 `revoked` 列），不再签发 refresh token
- `SSOProvider`(sso.go + sso_oidc.go) — OIDC SSO：Discovery / JWKS / Token / UserInfo / Authorize；access token 同 opaque，唯一签发 JWT 的地方是 `id_token`（RS256，签名密钥持久化在 settings `sso_oidc_signing_key`）
- `APIKeyManager`(apikey.go) — API Key 认证（`X-API-Key` 或 `Bearer ak_`，优先于用户会话凭证）
- `SessionManager`(session.go) — 会话列表 / 单点吊销 / logout-all / `POST /auth/logout` + 会话跟踪（活跃时间节流更新）
- `SessionStore`(session_store.go + session_issue.go) — 会话权威：签发 / Lookup / Revoke / Rotate，`user_sessions` 表行 + KV `sess:<sid>` read-through
- `SecurityManager`(security.go) — 暴力破解 / 异常位置 / 安全事件 / 登录历史

流水线入口：`Preflight()`（trace + 全局 IP 限流 + 可选会话解析，挂 v1）与 `Enforce()`（必需凭证 + ban/权限/安全/会话跟踪，挂 protected）。两段按 ctx 上的 stage 标记幂等，重复挂载不会二次计数。（此前的组合入口 `Handler()` 已于 2026-10-27 删除：生产只用这两段，它只剩测试在调用。）

### MFA — 多因素认证 (`handler/mfa.go`, `service/mfa.go`, `service/totp.go`)
TOTP 绑定/启用/禁用 + 备份码重生成；登录时 `/auth/mfa/verify` 校验。开关走 settings（`mfa_totp_enabled` 等）。

### PM — 私信 (`handler/pm.go`, `service/pm.go`, `model/conversation.go`)
会话列表 / 创建会话 / 消息列表 / 发消息 / 标记已读；发送受 `UserBlock` 与限流约束。

### Group — 用户组 (`handler/group.go`, `service/group.go`)
用户组 CRUD + 成员 + 组权限同步；默认系统组随角色 seed（`DefaultSystemGroups`）。等级与信任门槛见上文 **Trust / XP**（旧的 `service/trust.go` 与 `StartDailyPromotionTicker` 已删除，等级由 `StartDailyRecomputeTicker` 每日重算）。

### Webhook (`handler/webhook.go`, `service/webhook.go`, `model/webhook.go`)
订阅事件总线，投递记录 + 重投（后台 worker，`StartWorker`）+ ping 测试。

### Invite — 邀请注册 (`handler/invite.go`, `service/invite.go`)
邀请码 CRUD；注册模式支持开放 / 审批（`pending_user` 角色）/ 邀请。

### Draft / Revision / Trash (`handler/draft.go`, `revision.go`, `trash.go`)
草稿保存/列表/删除；帖子与评论的修订历史列表/恢复；回收站列表/恢复（软删除）。

### Badge / Leaderboard (`handler/badge.go`, `leaderboard.go`)
徽章默认集 seed、按用户查询；排行榜（积分榜读 `total_xp`、签到榜 `total_checkins`、发帖榜 `post_count`；`total_points` 别名已下线）。旧的 `service/points.go`（`awardPoints` / `trustLevelForPoints`）已删除，积分唯一来源见 **Trust / XP**。

### Watch / UserBlock (`handler/watch.go`, `user_block.go`)
主题订阅（回复/点赞事件通知）；用户拉黑（影响私信与互动）。

### Backup / Plugin / Theme / SEO / Realtime (`handler/backup.go`, `plugin.go`, `theme.go`, `seo.go`, `realtime.go`)
备份创建/列表/下载/恢复/清理（`cmd backup.go` 亦可 CLI 触发）；插件仅声明式元数据（install/enable，无执行扩展点）；主题偏好（用户级）；通用 SSE。

**爬虫面由后端独占**：`/sitemap.xml`（索引）、`/sitemap-posts.xml?page=N`、`/sitemap-spaces.xml`、`/sitemap-tags.xml`、`/robots.txt`、`/latest.rss` 全部挂在 app 根（不在 `/api/v1` 下），前端**不生成任何爬虫文件**。部署层 `deploy/nginx/nginx.conf` 用 6 条精确 `location =` 把这 6 条路径转给后端——少了它们会落到前端并 404。索引策略（内容页是否要登录）由设置 `post_auth_required` / `space_view_auth_required` / `user_profile_auth_required` 决定，`user_profile_indexable`（默认 false）控制他人主页是否进索引；`/sitemap-spaces.xml` 只收录匿名（TL0）可读的启用版面，login / level 版面不进爬虫面。规范见 `openspec/specs/seo`。

### Security / Session / APIKey（网关能力，经 `gw.*` 直接注册）
安全事件与登录历史查询、可疑上报；会话列表/吊销/logout-all；API Key 创建/列表/吊销。**登出 `POST /auth/logout` 也在这一层**（`SessionManager.LogoutHandler`），handler 层没有对应文件。

### Middleware (`middleware/auth.go`, `middleware/middleware.go`, `middleware/trace.go`, `middleware/deps.go`)
本层只做"身份已确定之后"的门控与请求基础设施，**不解析任何用户凭证**（用户凭证是服务端会话 sid，由网关解析）。

- `Trace(enabled)` — 建立/沿用 W3C traceparent，写入 `request.Context()`，MUST 排在 `Logger` 之前
- `Logger()` — 结构化请求日志（`trace_id`/`span_id`/`method`/`path`/`route`/`status`/`latency_ms`/`user_uuid`/`ip`），并把请求级 logger 注入 context 供 `logger.FromContext(ctx)` 使用
- `AuthGate(deps, settingKey)` — 通过 system setting 动态切换公开与强制登录（站点级）；版面 login/level 由 `service/visibility.go` 判定，`SpaceAuthGate` 已删除
- `RequirePermission(deps, code)` — 权限码门控，现只挂在 `/permission/*`、`/admin/backups`、`/admin/webhooks`；其余受保护路径由网关的 path→permission 映射负责
- `CORS()` / `Prometheus()` / `RateLimit(limiter, name, limit, window)` / `RequestTimeout(duration)` / `PathBodyLimit(size, overrides)` / `MaintenanceGate(settings)`
- `Deps`（deps.go）在 `server/iris.go` 构造一次，其 `Perms`/`Ban` 会再交给网关，全链路只有一份权限缓存与禁言判定

> 已删除且不要再引用：`JWTAuth` / `OptionalJWTAuth` / `BanCheck` / `RequireRole` / `RequireOwnerOrPermission` / `SetPermissionChecker` / `Deps.Blacklist`（会话鉴权迁移后生产调用数为 0，2026-10-14 清理）。所有权检查在 service 层（R-06b），封号与禁言拦截在网关。

### Database (`db/db.go`, `db/sql.go`)
Store 接口抽象，支持 SQLite/MySQL/PostgreSQL（GORM）。`BackendType` 枚举：`sqlite` / `mysql` / `postgres`。

### Config (`config/config.go`)
环境变量配置：`PORT`、`DB_TYPE`、`DB_DSN`、`Storage`（local / s3）、`KV`（NATS）。Storage 以 settings 表为来源，env 为回退（H-04 修复）。

### KV Store (`internal/pkg/kvstore/`, `docs/design/nats-deploy.md`)
KV 存储抽象层，支持 TTL、原子计数器、Pub/Sub、前缀扫描。唯一实现是 NATS JetStream：
- JetStream KV bucket 保存数据，per-key TTL 通过 companion bucket 实现。
- 变更通知与通用 Pub/Sub 使用 NATS core pub/sub（subject 前缀 `nats.kv.`），跨实例消息自动到达。
- 业务 key 统一编码为 JetStream KV 合法 key，对外接口仍使用原始 key。

`NATS_URL` **必填**，必须指向外部 NATS（JetStream 已启用）：KV 支撑会话 read-through 缓存、限流、OAuth state、View Count 缓冲、Settings/Ban 缓存、权限多实例同步（Pub/Sub）。进程内 nats-server 已于 2026-10-28 剥离——KV/EventBus 的生命周期不该绑在应用进程上，`natsembedded` 包保留仅供单元测试。

### Makefile
支持交叉编译：`windows` / `linux` / `darwin`。
`make swag` 重新生成 Swagger 文档并同步到 `openapi.json`。`make swag-check` 在 CI 中校验 swagger 是否过期。

### Rate Limiter (`internal/pkg/ratelimit/`)
唯一的限流引擎：滑动窗口近似（SetNX 起窗 + Incr 计数 + 比较），基于 KV Store。`Limiter.Check(ctx, policy, key, limit, window)` 返回 `Allowed`/`Denied`/`FailOpen`，并在此单点发出 `orinforum_ratelimit_*{name}` 指标与带 trace 的 fail-open 日志。两个调用方：`middleware.RateLimit`（路由注册的每端点预算，键 `rl:<ip>:<policy>`）与 `internal/gateway.RateLimiter`（`rate_limit_enabled` 开关下的全局每 IP 上限，键 `rl:ratelimit:<ip>`，策略名 `global`）。键形状由 `EndpointKey`/`GlobalKey` 单点生成并有测试钉住——改形状等于清零在计数的窗口。用于防登录/注册暴力破解。

### EventBus (`internal/pkg/eventbus/`)
基于 NATS core pub/sub 的分布式事件总线，每个事件类型使用队列订阅，集群内每事件只消费一次。业务 Service 发布事件（如 `EventContentReplied`、`EventContentLiked`），NotificationService 订阅并创建通知，解耦业务逻辑与通知发送。

### Logger (`internal/pkg/logger/`)
结构化请求日志接口 + 中间件。`Logger()` 输出 `trace_id`、`span_id`、`method`、`path`、`status`、`latency_ms`、`user_uuid`、`ip`，路由匹配后追加 `route`。请求级 Logger 由 `middleware.Trace` 建立 trace、`middleware.Logger` 经 `logger.NewContext` 注入 `request.Context()`，业务层用 `logger.FromContext(ctx)` 取用；`Config{Level,Format,Output}` 由 `server/iris.go` 调 `SetGlobal` 生效并同步标准 logrus。原 `request_id` 字段已并入 `trace_id`（单一关联 id，`X-Request-ID` 头取值同它）。

## 前端共享包

### `@orinforum/api-client` (`packages/api-client/`)
前端 API 共享包，Web 和 Dashboard 共同依赖。包含：
- `client.ts` — authFetch 封装，自动携带 Bearer token，401 自动刷新并重试
- `auth.ts` — 认证工具 (`setTokens` / `getAccessToken` / `getRefreshToken` / `clearAuthTokens` / `isAuthenticated`)
- `types.ts` — 所有 API 响应类型（User、PostListItem、ContentListItem、AdminStats、SearchResults、EntitySearchResult 等）
- `constants.ts` — 类型化常量（ErrorCode、PermissionCode、SettingKey、RoleName、PostStatus、API_BASE_URL）
- `services/*.ts` — 20+ 个 API service 模块（auth、post、content、admin、notification 等）

### `@orinforum/sse-client` (`packages/sse-client/`)
框架无关 SSE 客户端。`SSEClient` 类提供：Bearer token 鉴权、401 自动刷新 token 重连、指数退避（3s→30s，最多 5 次）、ReadableStream 解析、AbortController 取消。Web 端通过 `app/web/lib/hooks/use-notification-sse.ts` 调用。

### `@orinforum/shared-utils` (`packages/shared-utils/`)
跨前端共享工具函数：
- `formatSmartDate()` — 相对时间格式化（"3 分钟前"）
- `formatTime()` — `YY-MM-DD HH:mm:ss` 格式化
- `cx()` / `sortCx()` — Tailwind CSS class 合并（clsx + tailwind-merge）

### Storage (`storage/storage.go`, `storage/local.go`, `storage/s3.go`)
文件存储 Provider 接口，支持本地和 S3 两种实现。

### Swagger (`docs/`)
swaggo/swag 生成，启动后访问 `/swagger/`。注解写在 handler 方法上方。

### CLI (`cmd/root.go`)
Cobra 根命令 + 持久标志 `-e/--env`（默认 `dev`）。**不存在 `dev`/`prod` 子命令**——`go run main.go dev` 会报 `unknown command`；环境只经 `-e` 选择，读取 `.env` 与 `.env.dev` / `.env.prod`（后者覆盖前者）。真正的子命令只有 `backup`、`import-discourse` 与 `recompute-xp`（全量重算 XP / 等级）。

## 环境变量

| 变量 | 默认值 | 说明 |
|------|--------|------|
| PORT | 8080 | 服务端口 |
| DB_TYPE | sqlite | 数据库类型 (sqlite/mysql/postgres)。本地开发默认 sqlite；`deploy/docker-compose.yml` 显式用 postgres |
| DB_DSN | (空) | 数据库连接字符串 |
| STORAGE_TYPE | local | 存储类型 (local / s3) |
| S3_ENDPOINT | (空) | S3 endpoint (仅 s3 模式) |
| S3_BUCKET | (空) | S3 bucket (仅 s3 模式) |
| S3_REGION | (空) | S3 region (仅 s3 模式) |
| S3_ACCESS_KEY_ID | (空) | S3 access key (仅 s3 模式) |
| S3_SECRET_ACCESS_KEY | (空) | S3 secret key (仅 s3 模式) |
| NATS_URL | (空) | **必填**：外部 NATS 地址（需 `-js`），为空则启动失败 |
| NATS_BUCKET | orin_kv | JetStream KV bucket |
| METRICS_ENABLED | true | 是否启用 Prometheus 中间件与指标 listener |
| METRICS_PORT | 9091 | 指标端口，**仅内部可达**：不在 nginx 上游中，compose 用 `expose` 而非 `ports` |
| TRACING_ENABLED | true | 是否解析入站 `traceparent` 并回写该头（false 时仍生成 trace 供日志关联） |
| LOG_LEVEL | info | debug / info / warn / error；同时管辖存量裸 `logrus.X()` 调用 |
| LOG_FORMAT | json | json / text |
| LOG_OUTPUT | stderr | stdout / stderr 或文件路径；文件打不开时回退 stderr |
| ADMIN_PASSWORD | (空) | 初始 admin 密码；空则随机生成并打印日志 |
| DB_MAX_OPEN_CONNS | 25 | 连接池最大连接数 |
| DB_MAX_IDLE_CONNS | 10 | 连接池最大空闲连接 |
| DB_CONN_MAX_LIFETIME | 300 | 连接最长存活（秒） |
| DB_CONN_MAX_IDLE_TIME | 60 | 空闲连接回收（秒） |

> 说明：`STORAGE_TYPE` / `STORAGE_LOCAL_DIR` / `S3_*` 不在 `config.Load()` 直接读取，改由 `pkg.BuildStorageConfig(settings 表 → env → 默认)` 解析；存储为启动期单例，改 settings 需重启。OAuth access token TTL（`oauth_access_token_ttl`，默认 86400s）、OIDC `id_token` TTL（`sso_id_token_ttl`，默认 3600s）、分页上限/默认值同样从 settings 表加载。`JWT_SECRET` 已从 config 与启动校验中删除，不设也能启动；`jwt_access_token_ttl` / `jwt_refresh_token_ttl` 两个旧 settings key 不再被读取（`jwt_refresh_token_ttl` 对应的 refresh token 已停止签发）。

## API 测试

项目根目录有 `orinforum_test.sh` 全量 API 测试脚本。启动服务器后运行：

```bash
# 全量测试（建议在新的 SQLite 数据库上运行）
cd app/server && DB_TYPE=sqlite DB_DSN=/tmp/of_test.db go run main.go -e dev &
sleep 4
bash ../orinforum_test.sh -all

# 按模块测试
bash ../orinforum_test.sh -auth -post -user
```

详细用法见 `.agents/skills/api-test/SKILL.md`。

## 运行

```bash
# 后端
cd app/server && go run main.go -e dev

# 前端（含 /admin 后台）
pnpm dev:web

# Swagger 文档
open http://localhost:8080/swagger/
```

## 前端路由 (Web)

| 路由 | 页面 | 说明 |
|------|------|------|
| `/` | 重定向 | → `/timeline` |
| `/timeline` | 时间线 | 综合帖子列表，支持 ?category= &tag= 筛选 |
| `/timeline/post` | 帖子详情 | ?id= 查询参数 |
| `/spaces` | 版面大全 | Grid 展示所有版面 |
| `/spaces/{slug}` | 单版首页 | 版面信息 + 帖子列表 |
| `/login` | 登录 | 认证页 |

## 后台管理路由（`/admin`，内嵌在主站）

后台不是独立应用：它就在 `app/web/app/admin/**`，与前台同一个构建、同一个 origin、同一套部署。路由由 vinext App Router 目录约定产生。

后台**不走 SSR**：`app/admin/layout.tsx` 用客户端挂载门（`components/admin/admin-root.tsx`）把整段路由挡在服务端渲染之外，服务端只吐主站骨架；页面组件全部 `'use client'`。

准入独立成模块 `lib/admin/auth/access.ts`：按会话 cookie 拉 `/user/profile`，角色为 `admin` / `moderator` 或命中后台权限码才放行；未登录跳主站 `/login?redirect=/admin/...`，**普通用户直接原路返回**，拿不到回跳目标才显示整页 403。进了后台再按 `lib/admin/auth/route-access.ts` 做模块级门控（菜单过滤 + 壳层内 403）。

| 分组 | 路由 | 说明 |
|------|------|------|
| 首页 | `/admin` | 运营概览 |
| 论坛管理 | `/admin/banned` | 禁言管理 |
| | `/admin/posts`、`/admin/posts/detail?id=` | 帖子列表 / 详情 |
| | `/admin/comments`、`/admin/comments/detail?id=` | 评论列表 / 详情 |
| | `/admin/tags` | 标签管理 |
| | `/admin/trash` | 回收站 |
| 运营管理 | `/admin/announcements` | 公告管理 |
| | `/admin/reports`、`/admin/reports/detail?id=` | 举报列表 / 详情 |
| | `/admin/moderation` | 内容审核 |
| | `/admin/spaces` | 版面管理（含版面权限弹窗） |
| | `/admin/search` | 全局搜索（`hideInMenu`，不在菜单里） |
| 用户与权限 | `/admin/users`、`/admin/users/detail?id=` | 用户列表 / 详情 |
| | `/admin/invites` | 邀请码 |
| | `/admin/roles`、`/admin/roles/detail?id=` | 角色列表 / 权限矩阵 |
| | `/admin/groups` | 用户组 |
| | `/admin/levels` | 等级定义（名称 / XP 门槛 / 保级条件，可增删） |
| | `/admin/xp-rules` | 指标 → XP、每日上限、连续签到阶梯 |
| | `/admin/gates` | 门控规则、能力域默认动作、以用户试算 |
| 系统设置 | `/admin/api-keys` | API Key |
| | `/admin/webhooks` | Webhook |
| | `/admin/backups` | 备份 |
| | `/admin/oauth-clients` | OAuth 客户端 |
| | `/admin/plugins` | 插件 |
| | `/admin/security` | 安全事件 |
| | `/admin/sessions` | 会话管理 |
| | `/admin/settings?tab=` | 服务设置（`/admin/settings/dashboard` 重定向到 `?tab=dashboard`） |
| | `/admin/audit-logs` | 审计日志 |
| 认证 | `/login?redirect=/admin`、`/admin/first-change-password` | 登录复用主站；首改密在后台 |

> 后台登录页已删除，统一走主站 `/login`（带 `redirect` 回后台）。


## API 端点总览

后端 HTTP 接口的完整端点清单（按模块分组、含权限码）已下沉到 [docs/api-reference.md](docs/api-reference.md)。
权威来源为仓库根目录的 `openapi.json` 与运行时的 `/swagger/` 页面，由 handler 注解经 `make swag` 生成。

## 模块文档索引

<!-- AGENTS-INDEX:BEGIN -->

<!-- 由 `node scripts/check-agents-docs.mjs --write-index` 生成，请勿手改。 -->

共 39 个模块文档。改动某模块的代码时，必须在同一次变更内更新它的 AGENTS.md（CI 会阻断）。

| 模块目录 | 文档 | 标题 |
|----------|------|------|
| `.github/workflows/` | [AGENTS.md](.github/workflows/AGENTS.md) | CI 工作流（`.github/workflows/`） |
| `app/server/` | [AGENTS.md](app/server/AGENTS.md) | Server 应用启动 |
| `app/server/cmd/` | [AGENTS.md](app/server/cmd/AGENTS.md) | Cmd CLI 入口 |
| `app/server/internal/config/` | [AGENTS.md](app/server/internal/config/AGENTS.md) | Config 配置 |
| `app/server/internal/container/` | [AGENTS.md](app/server/internal/container/AGENTS.md) | Container 依赖装配 |
| `app/server/internal/db/` | [AGENTS.md](app/server/internal/db/AGENTS.md) | DB 数据库抽象层 |
| `app/server/internal/gateway/` | [AGENTS.md](app/server/internal/gateway/AGENTS.md) | Gateway 网关层 |
| `app/server/internal/handler/` | [AGENTS.md](app/server/internal/handler/AGENTS.md) | Handler 层 |
| `app/server/internal/mailer/` | [AGENTS.md](app/server/internal/mailer/AGENTS.md) | Mailer 邮件发送抽象 |
| `app/server/internal/metrics/` | [AGENTS.md](app/server/internal/metrics/AGENTS.md) | Metrics Prometheus 指标 |
| `app/server/internal/middleware/` | [AGENTS.md](app/server/internal/middleware/AGENTS.md) | Middleware 层 |
| `app/server/internal/mock/` | [AGENTS.md](app/server/internal/mock/AGENTS.md) | Mock 测试替身 |
| `app/server/internal/model/` | [AGENTS.md](app/server/internal/model/AGENTS.md) | Model 层 |
| `app/server/internal/pkg/` | [AGENTS.md](app/server/internal/pkg/AGENTS.md) | Pkg 通用工具包 |
| `app/server/internal/repository/` | [AGENTS.md](app/server/internal/repository/AGENTS.md) | Repository 层 |
| `app/server/internal/router/` | [AGENTS.md](app/server/internal/router/AGENTS.md) | Router 层 |
| `app/server/internal/service/` | [AGENTS.md](app/server/internal/service/AGENTS.md) | Service 层 |
| `app/server/internal/storage/` | [AGENTS.md](app/server/internal/storage/AGENTS.md) | Storage 文件存储抽象 |
| `app/server/server/` | [AGENTS.md](app/server/server/AGENTS.md) | Server 应用启动 |
| `app/web/` | [AGENTS.md](app/web/AGENTS.md) | Web 前端 |
| `app/web/app/` | [AGENTS.md](app/web/app/AGENTS.md) | Web App Router（`app/`） |
| `app/web/app/admin/` | [AGENTS.md](app/web/app/admin/AGENTS.md) | 管理后台（`app/admin/`） |
| `app/web/components/` | [AGENTS.md](app/web/components/AGENTS.md) | Web Components（`components/`） |
| `app/web/components/admin/` | [AGENTS.md](app/web/components/admin/AGENTS.md) | 后台壳层与准入组件（`components/admin/`） |
| `app/web/components/markdown-editor/` | [AGENTS.md](app/web/components/markdown-editor/AGENTS.md) | Markdown 编辑器（`components/markdown-editor/`） |
| `app/web/components/ui/` | [AGENTS.md](app/web/components/ui/AGENTS.md) | UI 原语（`components/ui/`） |
| `app/web/lib/` | [AGENTS.md](app/web/lib/AGENTS.md) | Web Lib（`lib/`） |
| `app/web/lib/auth/` | [AGENTS.md](app/web/lib/auth/AGENTS.md) | Web Auth（`lib/auth/`） |
| `deploy/` | [AGENTS.md](deploy/AGENTS.md) | 部署编排（`deploy/`） |
| `docs/` | [AGENTS.md](docs/AGENTS.md) | 文档目录（`docs/`） |
| `openspec/` | [AGENTS.md](openspec/AGENTS.md) | OpenSpec（`openspec/`） |
| `packages/` | [AGENTS.md](packages/AGENTS.md) | Packages 前端共享包 |
| `packages/api-client/` | [AGENTS.md](packages/api-client/AGENTS.md) | API Client 前端 API 共享包 |
| `packages/api-client/src/services/` | [AGENTS.md](packages/api-client/src/services/AGENTS.md) | API Service 模块（`packages/api-client/src/services/`） |
| `packages/loadtest/` | [AGENTS.md](packages/loadtest/AGENTS.md) | Loadtest 压测（`packages/loadtest/`） |
| `packages/shared-utils/` | [AGENTS.md](packages/shared-utils/AGENTS.md) | Shared Utils 跨前端共享工具 |
| `packages/sse-client/` | [AGENTS.md](packages/sse-client/AGENTS.md) | SSE Client 框架无关 SSE 客户端 |
| `packages/tokens/` | [AGENTS.md](packages/tokens/AGENTS.md) | Tokens 视觉设计 Token |
| `scripts/` | [AGENTS.md](scripts/AGENTS.md) | 仓库脚本（`scripts/`） |

<!-- AGENTS-INDEX:END -->
