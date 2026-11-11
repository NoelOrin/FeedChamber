# Router 层

路由注册。原先 500 余行的单个 `Register` 已按域拆成 `router.go` + 10 个 `routes_*.go`：`Register` 只留 v1 party 的创建、app 根注册、`v1.Use` 中间件链和 protected 的网关挂载，其余按域写成挂在 `*reg` 上的段函数，由 `Register` **按拆分前的顺序**依次调用。拆分是纯代码移动，函数体是原 `Register` 体的逐字连续切片。

## 文件

| 文件 | 段函数 | 覆盖 |
|------|--------|------|
| router.go | `Register` + `reg` | `v1 := app.Party("/api/v1")`、`/.well-known/*`、`v1.Use` 链（PathBodyLimit → RequestTimeout → `gw.Preflight()` → MaintenanceGate）、`/health`、SEO 的 app 根注册、`protected := v1.Party("")` + `protected.Use(gw.Enforce())`；随后按序调用下面 15 个段函数 |
| routes_public.go | `publicRoutes`, `publicOptionalRoutes` | 公开读接口（settings/themes/leaderboard/badges/upload 文件按 key 读）+ 注册/登录/找回密码/MFA 验证 + OAuth 消费者与自建 provider 公开面 + SSO 公开面；以及 tags/spaces/announcements 的"可选登录 + AuthGate"读 |
| routes_session.go | `dashboardRoutes`, `protectedAuthRoutes`, `userRoutes` | 首页聚合与用户徽章列表；protected 的登出/刷新/改密/邮箱验证/MFA、OAuth 账号绑定与客户端管理、SSO 授权；用户资料（公开 profile 走 v1 + 可选登录，其余走 protected） |
| routes_access.go | `accessRoutes` | 角色、权限（含版面级权限）、用户组 |
| routes_content.go | `tagManageRoutes`, `postRoutes` | 标签写操作；帖子与评论（公开读 + protected 写、置顶/加锁/版本/收藏、admin 全局评论列表、pinned list） |
| routes_engagement.go | `engagementRoutes`, `uploadRoutes` | 收藏列表、评论管理端读删与版本、点赞与回复、帖子订阅、关注与拉黑；上传 |
| routes_message.go | `messageRoutes`, `reportRoutes` | 私信、通知、通用 SSE 实时流；举报 |
| routes_account.go | `accountRoutes` | 签到、API Key、草稿、登录会话、主题偏好、安全风控 |
| routes_admin.go | `spaceManageRoutes`, `adminRoutes` | 版面写操作；管理后台（邀请、注册审批、公告、设置、禁言列表、备份、插件、回收站、审核、webhook、统计检索、批量、审计日志、对账） |
| routes_gate.go | `gateRoutes` | 门控规则后台（`/admin/gates/*`：规则 CRUD、域默认动作、meta、试算），整段 `RequirePermission(gate:manage)` |
| routes_trust.go | `trustRoutes` | XP / 等级后台配置（`/admin/levels/*`、`/admin/xp-rules/*`、`/admin/trust/recompute`、`/admin/users/{id}/xp*`、`/admin/xp-adjustments/{id}/delete`），整段 `RequirePermission(level:manage)`；精华操作在 `routes_content.go` 的 `POST /posts/{id}/feature`（网关映射 `report:manage`） |

`reg`（router.go）是一次 `Register` 调用内的共享状态：`app` / `v1` / `protected` / `h` / `deps` / `gw`，段函数签名为 `func (r *reg) xxxRoutes()`。每个函数体开头给这几个字段起局部别名（`v1, h, deps, gw := r.v1, r.h, r.deps, r.gw`），只为让函数体与拆分前的原文逐字一致——读代码时把别名行当成"这里在用一个父级句柄"即可，不要替换成 `r.h.` 之类的写法。

## 加路由去哪儿

- 按域定位文件（上表），再在文件内找拆分前就存在的 `// ── xxx ──` 分节注释。
- 新域：在同包加 `routes_<domain>.go` + 一个 `(r *reg)` 段函数，并在 `Register` 里**该域原先所在的位置**插入调用。插入位置是语义，不是排版。
- 同一前缀的公开面与 protected 面属两个不同段函数（`/auth` `/oauth` `/sso` `/tags` `/spaces` `/posts` `/user` 都注册了两次），不要合并到一处。
- 需要 app 根级别的路径（不在 `/api/v1` 下）时，写在 `Register` 内的 app 根注册区，别塞进 `/api/v1` 的段函数。
- PartyFunc 的闭包参数名沿用拆分前原文（`t` `s` `a` `p` `u` `c` `w` `m` `n` `b` `g` `r` …）。其中 `/role` 与 `/reports` 两处的闭包参数就叫 `r`，会遮蔽 `*reg` 接收者：段函数体内只用函数开头起好的局部别名（`protected` `h` `deps` `gw` `v1`），不要在闭包里访问接收者字段。不要为了"好看"重命名这些闭包参数——它们属于逐字切片。

## 顺序为什么是硬约束

- 固定路径 MUST 注册在参数路径之前（同一路径层、同一 method）。现存两处偏离，都是靠"静态段优先匹配"兜住的，改附近路由前先看全路径：`postRoutes` 末尾的 `POST /posts/pinned/list` 排在同层 `POST /posts/{id}/update` 等参数路由之后；`publicOptionalRoutes` 的 `POST /spaces/{space_id}` 又排在 `spaceManageRoutes` 的 `POST /spaces/create`、`POST /spaces/reorder` 之前。
- Iris 在**注册那条路由的那一刻**对 party 当前的中间件做快照（`core/router/api_builder.go` 的 `createRoutes` 读 `api.middleware`），`Use` 写在路由之后对已注册路由无效。
- `Party()` 只是复制父 party 的中间件切片：`context.JoinHandlers(parent, nil)` 在父切片非空、增量为空时直接返回父切片本身，于是兄弟 party 的 `Use` 会写进父切片空闲容量里的同一个槽位，进而改写已经包含该槽位的兄弟 party 的链。
- 所以 `protected := v1.Party("")` 与 `protected.Use(gw.Enforce())` MUST 留在原位置（公开 party 之后、任何 protected 路由之前）。把 protected party 提到 `Register` 开头统一装配，实测会让 protected 路由链丢掉 enforce 段。
- **两个同源缺陷已于 2026-10-14 修复，均由本包测试钉住**：
  1. **权限门失效**。app 级中间件条数决定父 party 是否留有空闲容量：`METRICS_ENABLED=true`（5 条）留空槽，`false`（4 条）不留。公开 `/tags` `/spaces` `/announcements` `/user` `/posts` 五个 party 各自的 `X.Use(gw.OptionalHandler())` 会覆盖**其后**注册的 protected 路由链里的网关段，导致权限、banCheck、安全风控、会话跟踪一律不执行。实测（默认配置 + 普通注册用户）`POST /admin/stats` 返回 200 与真实统计数据，metrics 关掉时才正确返回 403。修复：删掉这五处重复挂载（v1 层已覆盖全量，子 party 再挂没有增量收益），并由 `TestGatewayStagesMountedOnlyInRegister` 源码扫描禁止 `routes_*.go` 出现任何 `Use(gw.`。
  2. **全局限流二次计数**。v1 层的 preflight 对 protected 同样生效，而旧的 `Handler()` 把 preflight 又跑一遍，于是受保护请求两次累加同一个 `"ratelimit:"+ip` 计数、会话也解析两次。实测（`rate_limit_enabled=true`，上限 60/分钟/IP）第一个 429 出现在第 **30** 次 protected 请求（2×30=60 再加窗口内的公开请求计数）；修复后同一探测落在第 **58** 次（3 次前置 + 每请求一次），与 1× 计数吻合。修复方式见下条。
- **网关流水线是两段**：`gw.Preflight()`（trace + 全局限流 + 可选凭证解析）挂 v1，`gw.Enforce()`（必需凭证 → banCheck → 权限 → 安全 → 会话跟踪）挂 protected。两段以 ctx 上的 stage 标记幂等（`internal/gateway/gateway.go`），所以重复挂载不会二次计数（由 `internal/gateway/pipeline_test.go` 锁住，含幂等标记被移除时计数翻倍的变异验证）。组合入口 `Handler()` 已于 2026-10-27 删除：生产只用这两段，它只剩测试在调用。

## 约定

- 路由顺序重要：固定路径必须在参数路径之前
- 公开端点注册在 `v1` party 上，无需认证（v1 层 `gw.Preflight()` 提供可选登录解析）
- 受保护端点注册在 `protected` party 上，经 `gw.Enforce()`（会话 sid 或 API Key 必需校验 → banCheck（被封 404）→ 权限 → 安全 → 会话跟踪）；它前面的 tracing / 全局 IP 限流 / 可选凭证解析已由 v1 层的 `gw.Preflight()` 恰好执行一次
- 网关 `gw` 由 `server/iris.go` 装配后作为参数传入 `Register`；本层不再 `gateway.New(...)`，也不接收 `*gorm.DB`
- 权限控制通过 `middleware.RequirePermission(model.PermXxx)` 实现
- 帖子/内容列表等公开端点的可选会话解析来自 v1 层 `gw.Preflight()`，再叠 `middleware.AuthGate()`；不存在 `OptionalJWTAuth`，用户凭证不是 JWT
- crud 类操作权限门已放宽（所有权检查下沉 service 层）：post update/delete → `PermPostCreate`，content update/delete → `PermCommentCreate`

## API 结构

```
/api/v1
├── /health                        GET  (public)
# /metrics 不在此树中：已移出 /api/v1，由 METRICS_PORT(9091) 独立 listener 暴露（见 internal/metrics/AGENTS.md）
├── /settings/public               POST (public)
├── /dashboard                     POST (protected, auth)
├── /auth
│   ├── /register                  POST (public, rate-limited)
│   ├── /login                     POST (public, rate-limited)
│   └── /refresh                   POST (protected，单用轮换；`/refresh_token` 从未存在)
├── /auth (forgot/reset)
│   ├── /forgot_password           POST (public, rate-limited)
│   └── /reset_password_by_code    POST (public, rate-limited)
├── /tags
│   ├── /list                      POST (public, optional auth)
│   └── protected Party (perm:post:*)
├── /spaces
│   ├── /tree /flat /roots /{space_id} ... POST (public, optional auth + gate)
│   └── protected Party (create/reorder/{id}/update/{id}/delete/{id}/status/{id}/move；无路由级权限码，权限在 service/组层)
├── /announcements
│   ├── /published                 POST (public, optional auth)
│   ├── /{id}                      GET  (public)
│   └── rest under /admin
├── /posts
│   ├── /list / search /{id}       (public, optional auth + gate)
│   ├── /{postId}/contents/list    (public, optional auth + gate)
│   ├── /{postId}/tags             POST (public)
│   └── protected Party
│       ├── /batch                 (auth)
│       ├── /create                perm:post:create
│       ├── /{id}/update           perm:post:create (ownership in service)
│       ├── /{id}/delete           perm:post:create (ownership in service)
│       ├── /{id}/pin              perm:post:pin
│       ├── /{id}/lock             perm:post:lock
│       ├── /{id}/close            perm:post:create
│       ├── /{id}/status           perm:report:manage (状态直改，后台「通过/拒绝」)
│       ├── /{postId}/tags/set     perm:post:create
│       ├── /{postId}/contents/create           perm:comment:create
│       ├── /{postId}/contents/{id}/update      perm:comment:create (ownership)
│       ├── /{postId}/contents/{id}/delete      perm:comment:create (ownership)
│       ├── /{postId}/favorite                  (auth)
│       ├── /{postId}/favorite/status           (auth)
│       └── /comments/list          perm:comment:delete
├── /favorites
│   └── /list                      (auth)
├── /contents/{contentId}
│   ├── /like / like/status        (auth)
│   └── /replies                   (auth)
├── /user
│   ├── /profile                   GET (auth)
│   ├── /profile/update            POST (auth)
│   ├── /search                    perm:user:read
│   ├── /list                      perm:user:read
│   ├── /{id}                      GET (perm:user:read)
│   ├── /{id}/delete               perm:user:delete
│   ├── /{id}/role/update          perm:user:update
│   ├── /{id}/banned               perm:user:update
│   ├── /{id}/unbanned             perm:user:update
│   ├── /{id}/banned/status        (auth)
│   ├── /{id}/banned/overview      (auth)
│   ├── /{id}/space/{space_id}/banned/unbanned/status  perm:user:update/(auth)
├── /users/{userId}
│   └── /follow /unfollow /status /followers /following /counts  (auth)
├── /role
│   ├── /list                      perm:role:read
│   ├── /create                    perm:role:manage
│   └── /{id}/delete               perm:role:manage
├── /permission
│   ├── /list                      perm:role:read
│   ├── /role                      perm:role:read
│   └── /sync                      perm:role:manage
├── /upload
│   ├── /                          POST (auth)
│   ├── /presign                   POST (auth)
│   ├── /confirm                   POST (auth)
│   ├── /list                      POST (auth)
│   ├── /{id}                      GET  (auth)
│   └── /{id}/delete               POST (auth)
├── /notifications
│   ├── /list                      POST (auth)
│   ├── /unread_count              POST (auth)
│   ├── /{id}/read                 POST (auth)
│   ├── /read_all                  POST (auth)
│   ├── /{id}/delete               POST (auth)
│   └── /stream                    POST (JWT, SSE)
├── /reports
│   ├── /create                    perm:report:create
│   ├── /list                      perm:report:manage
│   ├── /{id}/status               perm:report:manage
│   └── /{id}/delete               perm:report:manage
├── /checkin
│   ├── /                          POST (auth)
│   ├── /status                    GET  (auth)
│   └── /history                   GET  (auth)
├── /auth (protected)
│   ├── /logout                    POST (auth)
│   ├── /refresh                   POST (auth)
│   ├── /change_password           POST (auth)
│   └── /first_change_password     POST (auth)
├── /oauth
│   ├── (public) GET /providers/{provider}/auth/callback/config
│   └── (protected) GET /accounts POST /{provider}/link/unlink
├── /comments
│   └── /{id}/delete               perm:comment:delete (admin)
└── /admin
    ├── /moderation/*              perm:report:manage (待审/规则/配置)
    ├── /stats /dashboard /search /user/{id}/activities
    ├── /batch/users /batch/posts /batch/comments
    ├── /reconcile/counters
    ├── /banned/list
    ├── /announcements/* (list/create/{id}/update/delete/publish/unpublish)
    └── /settings/* (list/batch/{key}/{key}/update/{key}/delete)
        ALL under perm:role:manage
```

> 上表为常用端点速查，未完全展开。完整、权威的端点定义以本包的 `router.go` + `routes_*.go` 为准（按域见上方文件表）；另有 metrics/dashboard、groups、mfa、pm、watches、user-blocks、drafts、revisions、trash、backups、webhooks、plugins、badges、theme、leaderboard、invite、security、sessions、api-keys、seo/realtime 等路由未在树中展开。非 `/api/v1` 系统端点还有 `/.well-known/*`（OIDC Discovery/JWKS）、`/sitemap*.xml`、`/robots.txt`、`/latest.rss`、`/swagger/{any:path}`（后者在 `server/iris.go` 注册，不在本包）。

## 注意事项

- 所有列表接口均为 POST，请求体传 `{ page, page_size, ...filters }`（网络字段名统一下划线，2026-11-11 定案）
- 所有列表响应格式：`{ code, msg, data, pagination: { currentPage, page_size, totalItems, totalPages } }`
- `/comments/list` 挂在 `/posts` party 下（admin 全局评论列表），`/comments/{id}/delete` 单独 party
- Admin search 支持各实体独立分页参数：`users_page/users_page_size`, `posts_page/posts_page_size`, `comments_page/comments_page_size`

> 2026-10-29：项目更名为 Feed Chamber（Go module `feedchamber`、npm `@feedchamber/*`、镜像 `feedchamber-*`）。

## 站点自定义的发布门（2026-10-30）

网关的 path→permission 映射把 `/api/v1/admin/customization/` 整体挂在 `customize:write`（前缀优先于 pattern），因此 `publish` 与 `toggle` **额外**挂 route 级 `middleware.RequirePermission(deps, model.PermCustomizePublish)`。这两条动作等于「让任意脚本对全站访客生效」，不能与「改配置」共用同一个门。

## 插件后台端点（2026-11-05）

`routes_admin.go` 的 `/plugins` Party 现有 11 条 `POST`：`list` / `sources/list` / `installs/list`（`plugin:read`），`install` / `sources/create` / `sources/{id}/approve|reject` / `{id}/update|status|uninstall|delete`（`plugin:manage`）。

固定段 `/sources/*` `/installs/*` 必须排在参数段 `/{id}/*` 之前。`/{id}/status` 是写操作，权限码为 `plugin:manage`（此前错挂 `plugin:read`）；网关 `permission.go` 的映射必须同步（新增端点默认拒绝）。`middleware_chain_test.go` 的 `TestAdminPluginRoutesHaveGatewayPermission` 同时扫描两处源码钉住读/写分离与端点齐全。

## 插件预览 / 版本 / 回滚路由（2026-11-05）

`/plugins` Party 追加：`preview`（manage）、`versions/list`（read）、`{id}/rollback`（manage）。固定段 `versions/*` 必须排在 `/{id}/*` 之前。

## 浏览记录 / 续读位置（2026-11-09）

新增 `routes_history.go` 段函数 `historyRoutes()`：`POST /history/list`、`/history/clear`、`/history/{postId}/delete` 三条 protected；`routes_content.go` 的 `/posts` Party 追加 `POST /{id}/view`（`h.Post.RecordView`，与 `/{id}/read` 分途）。规范见 `openspec/changes/2026-11-09-browse-history-read-position/`。

- **固定段先于参数段**：`/list`、`/clear` MUST 排在 `/{postId}/delete` 之前，否则 `{postId}` 会吞掉固定段（本包既有硬约束，见「顺序为什么是硬约束」）。
- **插入位置是语义**：`Register` 里 `r.historyRoutes()` 紧跟 `r.postRoutes()`（在 `engagementRoutes` 之前）——它是帖子读取面的第 7 个出口，与 `postRoutes` 共用同一份可见性谓词与 `topic_read_states`，放一起才便于一起被读到；不要挪到文件末尾或别的域段之后。新段只挂路由，无 `Use(gw.*)`。

## 路由层依赖注入（Iris hero DI）：已验证，未采纳（2026-11-11）

曾在本包试做「段函数用 `HandleFunc` + `handler.XxxHandler` 接口形参注入，取代 `r.h`」，
完整实现并跑通测试后**主动回滚**。结论留档，避免下次重走：

### Iris DI 的硬约束（实测）

1. **依赖注册必须早于任何 `Party()`**。`Party()` 复制父容器此刻的依赖快照
   （`core/router/api_container.go` 的 `convertHandlerFuncs` 读 `apiBuilderDI.Container`），
   晚注册的依赖在子 party 里不可见，注入点直接注册期 panic
   （`expected [2] bindings ... Missing: [2] Xxx`）。
2. **只有 `HandleFunc` 支持注入**。`Get`/`Post`/`Delete` 的签名是
   `handlers ...context.Handler`，传注入型函数是**编译错误**。
3. **注入型 handler MUST 无返回值**。hero 会把返回值交给自己的 result handler 直接写响应
   （实测返回 `string` 时响应体就是该字符串），绕过 `pkg.Success` 统一信封。
4. 注入型 handler 不自动 `ctx.Next()`，与原生 handler 语义一致。
5. 具体类型注册可注入到接口形参；接口类型变量（`container` 持有的形态）同样可注入。

### 为什么回滚

按「收益 / 代价」算不划算：

| 维度 | 现状 `h.Post.List` | DI 版 | 谁更好 |
|---|---|---|---|
| handler 字段名拼错 | 编译错误 | 编译错误 | 打平 |
| `handler.New` 漏构造字段 | 运行期 panic | 注册期 panic | DI 略好，但场景罕见 |
| 依赖可读性 | `h.Post.List` 一眼可见 | 形参可见 | 打平 |
| 代码量 | 1 行/路由 | 3 行/路由 | 现状 |
| 编译期保证强度 | 强 | 弱（需手写源码扫描兜底） | 现状 |

关键判断：**漏接线从编译错误退化为注册期 panic 是能力退化**，而它针对的
「`handler.New` 漏构造字段」场景，已被「45 个 handler 全在单一构造函数里」覆盖。
用反射换取一个已有的性质，再用自写正则测试去守卫更弱的新不变量，方向是反的。
规模数据：308 条路由 × 2 行 ≈ 净增 600 行，会稀释「扫一眼就知道有什么端点」这个路由文件的核心价值。

Iris DI 真正的适用场景是 handler 多、依赖按请求动态构造；本项目 45 个 handler 全部在
启动时静态构造，已是依赖关系最简形态。

> 若将来重提此方案，直接读上面 5 条硬约束即可，不必重新验证。
