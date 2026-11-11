# Service 层

业务逻辑层，接收 repository 接口，处理业务规则，返回 `error`（通常是 `*pkg.AppError`）。

## 约定

- 每个文件定义 `XxxService` 接口（`*_iface.go`）+ 私有实现 struct（`*.go`）
- 构造函数 `NewXxxService(repo XxxRepo, ...) XxxService`
- 错误用 `pkg.NewAppError(pkg.XXX, msg...)` 返回
- 分页逻辑在 service 层处理 (page/pageSize → offset/limit)
- 列表方法返回三元组 `(items, totalItems, error)`，handler 层据此构造分页响应
- UUID 生成在 service 层 (`uuid.NewString()`)
- 密码哈希在 service 层 (`bcrypt.GenerateFromPassword`)
- 事务通过 `db.TransactionManager.WithTx()` 实现

## 帖子状态与可见性（2026-10-28 修复）

`post.go` 是「发布成功」与「公开展示」之间唯一的一致性关口，四条规则不可拆开改：

- **创建默认公开**：`NewPost()` 落 `approved`；初始状态由 `moderation_mode` 决定（见下条），
  只有 `manual` 模式、`external` 判定命中、`post_approval=true`（兼容别名，等价 `manual`）
  或门控规则 `post_create → require_review` 才改写成 `pending`。旧的
  `trust_tl0_require_approval` / `ShouldForceApproval` 已在 P3 删除（key 仅留兼容）。
- **四种审核模式**（系统设置 `moderation_mode`，默认 `async`，只作用于文章）：
  `off` 不跑规则也不留痕（门控的 require_review 一并跳过；外链 deny 仍生效）；`async` 先公开、发帖事件在后台跑规则，命中转
  `pending` 交人工；`external` 发布前同步调外部服务（3s 超时），不可用回退 `async`；
  `manual` 一律 `pending`。模式的读取与外部调用在 `moderation.go`（`Mode` / `CheckExternal`），
  异步消费者是 `SubscribeAsync` → `handlePostCreated`，由 `container.Runtime` 在启动时订阅。
- **待审必留痕**：任何最终为 `pending` 的帖子 MUST 写一条审核记录（`RecordPendingFrom`，
  `strategy` 标注 `manual` / `tl0` / `external` / `async`），否则审核队列看不到它，
  而 `moderation.Approve(id)` 只接受审核记录 id → 无路可审。
- **状态过滤哨兵**：`List` 的 `status="__all__"`（`repository.StatusFilterAll`）表示不过滤，
  且只对 `canViewNonPublic(ctx)`（`report:manage`）为真者生效；无权限者无论传什么一律
  `StatusIn=[approved,closed]`。同一哨兵在 `report.go` 的 `List` 也要识别。
- **作者例外**：`canViewPost(ctx, status, authorID)` = 公开状态 OR 审核权限 OR 本人；
  `GetByID`/`GetDetail` 用它替代裸 `canViewNonPublic`，否则作者看不到自己刚发的待审帖。
- **状态直改**：`SetStatus(ctx, id, status)`（`approved|pending|rejected|closed`）不要求审核记录，
  用于救回存量待审帖；权限门在网关（`report:manage`），非法值返回 `INVALID_PARAMS`。
- **版面递归聚合**：`List` 只有同时拿到 `spaceID != "" && includeSub` 时才走
  `SpaceRepository.FindDescendantIDs`，把过滤展开成 `SpaceIDs = [本版 + 全部后代版]`
  并清空 `SpaceID`（两者并存时 IN 子句优先）。`space_slug`/`space_name` 路径不参与递归；
  子版查询报错时退回单版 `SpaceIDs=[spaceID]`，父版面不会因一次 DB 抖动变空列表。
  契约由 `post_include_sub_test.go` 钉住。

## 文件列表

| 文件 | 服务 | 关键依赖 |
|------|------|---------|
| auth.go | AuthService | UserRepository, CredentialRepository, MetaRepository, SettingService, BannedService, EmailService |
| auth_iface.go | — | AuthService 接口定义 |
| user.go | UserService | UserRepository, TokenBlacklist |
| user_iface.go | — | UserService 接口定义 |
| post.go | PostService | PostRepository, ContentRepository, BannedService, KV, SettingService, SpaceRepository, **PermissionService** |
| post_iface.go | — | PostService 接口定义 (含 ListByIDs) |
| content.go | ContentService | ContentRepository, PostRepository, UserRepository, EventBus, BannedService, TransactionManager, **PermissionService** |
| content_iface.go | — | ContentService 接口定义 |
| permission.go | PermissionService | PermissionRepository (带 `sync.RWMutex` 内存缓存) |
| permission_iface.go | — | PermissionService 接口定义 |
| tag.go | TagService | TagRepository |
| like.go | LikeService | LikeRepository, ContentRepository, UserRepository, EventBus |
| favorite.go | FavoriteService | FavoriteRepository, PostRepository |
| follow.go | FollowService | FollowRepository, UserRepository, EventBus |
| upload.go | UploadService | UploadRepository, Storage Provider, SettingService |
| notification.go | NotificationService | NotificationRepository, KV (用于未读计数缓存) |
| notification_iface.go | — | NotificationService 接口定义 (含 CountByUser) |
| report.go | ReportService | ReportRepository |
| setting.go | SettingService | SettingRepository |
| admin.go | AdminService | AdminRepository, UserRepository, KV, TokenBlacklist |
| admin_iface.go | — | AdminService 接口 + `AdminStats`, `DashboardData`, `SearchResults`, `EntitySearchResult[T]`, `UserActivities` |
| banned.go | BannedService | BannedRepository（`List` / `ListByScope` 返回 `(items, total, error)`，分页条依赖 total） |
| banned_iface.go | — | BannedService 接口定义 |
| token_blacklist.go | TokenBlacklist | KV Store (jti + user 級黑名单) |
| token_blacklist_iface.go | — | TokenBlacklist 接口 |
| role.go | RoleService | RoleRepository |
| role_iface.go | — | RoleService 接口定义 |
| email.go | EmailService | Mailer, KV (code 存储与验证)、SettingService（`BuildMailerConfig` 每次发送重读）；含后台自测 `SendTest` |
| email_iface.go | — | EmailService 接口定义 |
| oauth.go | OAuthService | UserRepository, OAuthRepository, SettingService, KV |
| checkin.go | CheckInService | CheckInRepository, UserRepository |
| announcement.go | AnnouncementService | AnnouncementRepository |
| space.go | SpaceService | SpaceRepository, PostRepository |
| audit_log.go | AuditLogService | AuditLogRepository (方法: Log/Record/List) |
| moderation.go | ModerationService | ModerationRepository + SettingService（待审队列/规则/配置管理、`CheckContent` 关键词与正则、`Mode` 读审核模式、`CheckExternal` 发布前外部审核、`SubscribeAsync` 异步复查消费者） |

> 上表未覆盖全部（现共 108 个文件 / 36 个接口）。另含（举例）：`space.go`、`group.go`、`trust.go`、`mfa.go`/`totp.go`、`pm.go`、`webhook.go`、`plugin.go`、`backup.go`、`draft.go`、`revision.go`、`trash.go`、`badge.go`、`theme.go`、`leaderboard.go`、`invite.go`、`watch.go`、`user_block.go`、`trust_level.go`、`digest.go` 等。

## 插件管理动作与运行时接线（2026-11-05）

`plugin.go` 的 `PluginService` 经 `SetRuntime(PluginRuntime)` 注入子进程宿主（窄接口：`StartPlugin` / `StopPlugin` / `Running`），四个管理动作因此不再是纯元数据登记：

- `Install`：先落两张表，再 `StartPlugin`；失败回滚两表。只有接了 runtime 才把 `plugin_installs.status` 写成 `running`（`ListResumable` 只认 running，否则新装插件永远不被恢复拉起）。
- `SetEnabled(false)` → `StopPlugin` + 置 `stopped`；`SetEnabled(true)` → 从落库 manifest/caps 快照复原并 `StartPlugin` + 置 `running`。
- `Delete` 与 `Uninstall` 等价：先 `StopPlugin`，再删 `plugins` 与 `plugin_installs` 两张表，最后落审计（此前 `Delete` 只删一行，留下孤儿安装记录与运行中进程）。
- `ListInstalls`：返回全部安装记录（status / restarts / last_error），供后台展示运行状态。
- 审计：`InstallAs` / `UninstallAs` / `SetEnabledAs` / `UpdateAs` / `DeleteAs` 接收真实 `actorID`（handler 传 `currentUserID(ctx)`），审计记操作人；无 actor 的旧方法委托并落 `"system"`。
- `Update` **不写 `enabled`**：元数据编辑与启停解耦。启停唯一入口是 `SetEnabled`（它同时驱动进程）；若 `Update` 接受 `enabled`，漏传该字段的客户端会用 Go 零值 `false` 把插件静默禁用，且会出现「库里已禁用、进程还在跑」的不一致。

`HostOptions` 的适配层在 `internal/container/plugin_api_deps.go`，`PluginRuntime` 的实现 `hostRuntime` 固定用 `context.Background()` 拉起子进程——`exec.CommandContext(ctx)` 的语义决定不能把 HTTP 请求 ctx 透进去，否则请求结束即杀掉插件。

`content.go` 新增 `SetStatusForPlugin`（`ContentService` 接口）：插件宿主 API 的 `comments.setStatus` 经 service 层写库并自带取值域校验，不给插件直连 repository 的后门。

## Trust / XP（2026-10-29 P1）

`trust_level.go` 是 XP 与等级体系的**唯一写入点**（`TrustService.Recompute`）：

- XP 由行为聚合派生，不建事件账本：`Σ 指标计数 × 当前 XP 规则` + 连续签到里程碑 + 人工调整。删帖 / 取消点赞 / 取消收藏 / 取消精华 / 审核拒绝都会在下次求值时**自动回收**，没有「扣分」路径。
- 日上限按「自然日 × 指标」截断（`daily_cap` 是条数上限）；被赞按点赞行创建日归集；只计 `approved` 且未删除的内容。
- 连续签到按「连续天数**恰好达到**」计一次（`check_ins.streak = N` 的行数），断签重来可再得。
- 等级 = 累计 XP 定级 + 滚动窗口保级，逐级下探降级；`manual_only` 等级不自动进入；`users.level_manual=true`（管理员手工设级）时 Recompute 只刷新 XP 与指标、不改等级。
- 配置（`xp_rules` / `xp_streak_bonuses` / `trust_levels`）在进程内缓存 60s，写配置时立即失效并广播 `trust:sync`：其它实例收到广播后同样清空快照（`StartSyncSubscriber` 由 `container.Start` 订阅），所以改分值无需重启，也不必等 TTL 过期。
- 触发点：发帖/删帖/状态变更、评论/删评论、审核通过/拒绝、点赞/取消、收藏/取消、加精/取消、签到，都只调 `Recompute`，**MUST NOT** 直接写 `users.total_xp` / `trust_level` / `level_metrics`。
- `points.go` / `trust.go` 旧实现已删除：`awardPoints`、`syncTrustLevel`、`trustLevelForPoints`、`evaluatePromotion`、`TrustLevel0..4` 常量都不存在了；旧 TL0 门已删除（P3）：`ShouldForceApproval` / `ShouldBlockLinks` 与 `trust_tl0_*` 设置都不再被读取，改由门控规则引擎接管。
- 每日 `StartDailyRecomputeTicker` 全量重算一次（启动 2 分钟后先跑一次），处理不活跃衰减降级；`recompute-xp` CLI 与后台「全量重算」复用 `RecomputeAll`。

## 权限检查方案 (R-06b)

所有权检查不通过中间件，改在 service 层实现：
- PostService.Update/Delete 和 ContentService.Update/Delete 接收 `actorUUID, actorRole string` 参数
- 内部判断：`isAuthor := resource.AuthorID == actorUUID` + `hasPerm := s.perms.HasPermission(actorRole, permCode)`
- 仅作者 OR 拥有权限码的用户可操作

## PermissionService 特殊说明

使用 `sync.RWMutex` 维护 `map[string]map[string]struct{}` 缓存 (role → permission codes)。
- `LoadCache()` 启动时从 DB 加载
- `HasPermission()` 读锁查询
- `SyncRolePermissions()` 写锁更新
- 变更后通过 kvstore Pub/Sub (`perm:sync` 频道) 通知多实例同步缓存 (R-07)
- R-04: 幂等 seed — `alreadySeeded` 元标记 + `roleMappingsEmpty` 双重门控，重启不覆盖自定义权限

## WatchService（2026-10-28 收口）

`watch.go` 只服务**帖子关注**（`target_type: post`）与**标签关注**（`tag`，后端支持但前台暂无入口）。
版面关注已于 2026-10-28 整体移除：`validTargetType` 不再接受 `"space"`，`NotifyWatchers` 不再向版面关注者扇出，
`NewWatchService` 的 `postRepo` 入参也随之摘掉（它当时只为从 post 取 `SpaceID`，去掉 space 分支后成了纯多余的依赖）。
`watch_test.go` 用 `assert.False(validTargetType("space"))` 钉住这条收口。

## 通知链路修复（2026-11-09）

三处都落在 `notification.go` / `watch.go`，均有对应测试钉住：

- **一条评论只产生一条通知**：`NotifyWatchers` 的发布循环 MUST 只遍历「本次调用新加入的关注者」（`newRecipients`），不能遍历共享 `notified` 集合。后者还含帖子作者与 @提及者（更早扇出步骤已通知），遍历它会给这些人再发一条 `content.watched`。`watch_test.go` 的 `TestNotifyWatchers_SkipsAlreadyNotifiedRecipients` 钉住。
- **dispatcher 入队不阻塞 EventBus worker**：`NotifDispatcher.enqueue` 用 `select/default` 非阻塞入队，队列满则丢弃并记 `feedchamber_notif_dispatcher_dropped_total{type}` + warn 日志。EventBus worker 是所有订阅共用的，慢 DB/KV 不能拖住无关事件。`notification_dispatcher_test.go` 钉住。
- **未读缓存 TTL 与 DB 对账**：`Create` 落库前先调 `CountUnread` 校准缓存。`CountUnread`（`Get`）会判 5min TTL 并回源 DB，而 `IncrBy` 读 JetStream 原值、**不判 TTL**；TTL 到期且无人读过时直接 `IncrBy` 会把「过期脏值 +1」推给 SSE。校准放在落库前，使后续 `IncrBy` 恰好是「为新行 +1」。`notification_cache_ttl_test.go` 钉住。

## 可见性与门控（2026-10-29 P2/P3）

- `visibility.go` 是内容可见性的**单一谓词**：`BuildVisibilityIndex` 把版面 `visibility`(public/login/level) 沿祖先链解析成有效策略（level > login > public，level 取链上最大 `min_level`）；`VisibleSpaceIDs(level, loggedIn)` 供 SQL 过滤，`PostRequiredLevel` 合并帖子 `min_level` 覆盖。游客一律按 TL0、`is_anonymous` 单独保留。
- `post.go` 的六个出口（List / GetByID / GetDetail / Search / SitemapEntries / ListByIDs）与置顶列表共用它：聚合出口把 `visibleSpaceIDs` 交给 repository 的 `buildVisibilityWhere` / `sitemapVisibilityWhere`；详情不足时返回 403「需要等级 N，当前等级 M」；作者例外为 `author_id = viewer`。有 `report:manage` 的审核者不受可见性限制（否则后台无法治理受限帖）。sitemap 与 RSS 固定按匿名（TL0）过滤。
- **从属内容出口与写入也走同一门槛**：`content.go` 的评论列表（`ListByPost`）、回复列表（`ListReplies`）与评论创建（`Create`，事务内加载帖子后立即判定）都通过 `PostViewer.CheckViewablePost`（`PostService` 实现）复用详情判定，帖子不可见时同样 404/403——否则受限帖的评论会绕开帖子门槛泄露，低等级用户也能盲写看不见的主题。作者例外与 `report:manage` 审核者由该入口放行。新增任何按 post/content 读或写内容的出口都必须接这个入口。
- `Space.Get` 也用它返回 403，`middleware.SpaceAuthGate` 已删除；`AuthGate` 保留为站点级登录墙。
- `gate.go` 是门控规则引擎：固定谓词集 + 4 个动作（allow/deny/require_review/rate_limit），按 `priority` 降序**首个命中**，无命中走 `gate_domain_defaults` 的显式默认；配置内存快照 30s + `gate:sync` Pub/Sub 失效；每次判定写带 trace 的结构化日志（命中规则 id / 默认动作），后台 `Simulate` 与线上共用 `Evaluate`。
- 发帖在 `post_create` 域、评论在 `comment_create` 域；两者只要正文含外链就额外进 `external_link` 域（只在含链接时求值，避免 deny 默认误伤纯文本）；`require_review` 会落 pending 并留痕 `strategy=gate`，`rate_limit` 按 `rl:gate:<domain>:u:<id>` / `lvl:<level>` 键执行。
- seed：外链域默认 deny + TL0 deny 规则（priority 100）+ 等级 ≥1 allow 规则（priority 50）；不 seed 发帖强制待审。

## 邮件诊断（2026-11-06）

`emailService.SendTest(ctx, to)` 是后台「发送测试邮件」的实现：
用**当前已保存**的配置真发一封，返回 `MailTestReport`（provider/host/port/tls_mode/from/
email_enabled/elapsed_ms/error/notice）。

- 配置错误与投递失败**都写进 report（`ok=false`），err 返回 nil**——这条路径存在的意义就是
  把原因交给管理员，打成 500 只会显示「Internal Server Error」。只有收件人非法才回
  `*pkg.AppError(INVALID_PARAMS)`。
- 报告结构体刻意不含凭据字段，密码/API Key 不可能被序列化出去
  （`email_test.go` 的 `TestSendTest_ReportNeverContainsCredentials` 钉住）。
- `email_enabled` 只拦用户侧验证码流程；自测照跑，但会在 `notice` 里说明「用户侧仍被拦截」。
- `provider=log` 时 `ok=true` 但带提示「只写日志、收件人不会收到」——否则管理员会误判成真发成功。

## 读穿缓存（readcache）

三个热点只读路径接了 `internal/pkg/readcache`（NATS KV + TTL + 前缀失效）：

| service | 缓存内容 | TTL | 失效时机 |
|---------|----------|-----|----------|
| `postService.List` | **匿名**列表页（page/pageSize/tag/author/space/sort 全部进 key） | 10s | Create / Update / Delete / SetStatus / 版面写操作 |
| `tagService.List` | 标签分页 | 60s | 标签 Create / Update / Delete / SetPostTags |
| `spaceService.Tree` / `Flat` | 版面树 / 扁平列表 | 60s | 版面 Create / Update / Delete |

**登录态一律不进共享缓存**：`postService.List` 只在 `viewerID` 缺失时读写缓存，
因为登录列表会注入已读状态（`ReadStates`）、拉黑作者排除（`ExcludeAuthorIDs`）
与等级可见性——缓存它就是跨用户串数据。新增过滤条件时把它加进 `readcache.Key`
的参数列表，否则不同过滤会命中同一条缓存。

> 2026-10-29：项目更名为 Feed Chamber（Go module `feedchamber`、npm `@feedchamber/*`、镜像 `feedchamber-*`）。

## 站点自定义的写入校验（2026-10-30）

`customize.go` 的校验是**策略边界**（渲染侧只负责按 kind 选容器）：

- `validateSnippetInput`：`position` 一律用 `model.Snippet{…}.EffectivePosition()` 归一化后落库（html 恒为 `body_end`）；`validateSnippetContent` 拒绝 css 含 `</style`、js 含 `</script`——这两类内容会被原样内联进 SSR 的 `<style>` / `<script>`，结束标签能闭合容器并把剩余内容当正常标记执行。
- `ActiveSnippets(ctx, position)` 按 `EffectivePosition()` 分组（不是按 `position` 列），存量数据里写成 head 的 html 片段因此仍能发出去。
- `validateNavHref`：只放行 `/…` 与 `http(s)://`；`javascript:` / `data:` / `//host` 一律拒（导航会被渲染进 `<Link href>`，点击即执行）。
- `validateThemeToken`：token 必须是 `^--[a-zA-Z0-9-_]+$`，值禁 `; { } < >` 与换行、限长 512。token 覆盖只负责改色改圆角，不承担任意 CSS（那是 css 片段）。

## 外部 MCP 客户端硬化（2026-10-30）

`mcp_server.go` 是「管理员填 URL、服务端去请求」的 SSRF 面，四件事必须一起成立：

- `validateEndpoint`：只放行 http(s)，拒回环 / 私网 / 链路本地（含 `169.254.169.254`）。
- `CheckRedirect` 返回 `http.ErrUseLastResponse`：Go 只跨域剥离 Authorization/Cookie，Headers JSON 里的自定义鉴权头会被原样带到重定向目标。
- `parseHeadersWithValidation`：必须是「字符串→字符串」的 JSON 对象，写入侧（`validateHeaderFormat`）同样校验。旧的 `json.Valid` 放行 `[1,2]`、`{"X-Key":123}`，随后反序列化失败被吞掉 → 鉴权头静默全丢。
- `call` 用入参 ctx 派生超时（不再 `context.Background()`），响应超 1MB 报「响应过大」而不是「不是合法 JSON-RPC」，`id` 用 `json.RawMessage` 并按值比对（字符串 `"1"` 也算匹配），并缓存/回传 `Mcp-Session-Id`（有状态实现在第二个请求上会校验它）。

## 唯一冲突与分页

- `repository.WrapDBErr` 现在统一把三家后端的唯一冲突翻译成 `repository.ErrDuplicate`（`checker_sql.go` 注册 checker）。service 只判断 `errors.Is(err, ErrDuplicate)`，不要再用 `strings.Contains(err.Error(), "UNIQUE")`。
- 分页 offset 一律用 `pkg.Offset(page, pageSize)`：`(page-1)*pageSize` 在 page 取极大值时溢出为负数，PostgreSQL 会直接报 `OFFSET must not be negative`（500 而不是空列表）。
- `SetThemeTokens` 整批在一个事务里（先全量校验再写入），中途失败不留半套覆盖；`theme_tokens` 的身份是 `(token, mode)`，同一 token 的明暗两套覆盖各自一行。

## 权限补齐的「只补新码」

`service/perm_seed.go` 的 `RolePermissionTopUp` 是纯函数：只补 `applied`（历史上已应用过的权限码）里没有的码。`server/iris.go` 把新码并入 meta `seeded_role_permission_codes`。这样升级能拿到新权限，而管理员撤销过的默认权限不会在重启时复活（R-04 的两半都成立）。

## 插件代码拉取与版本历史（2026-11-05）

- `plugin_fetch.go`：`PluginFetcher` 接缝（`Fetch` / `InstallDeps`）+ `execPluginFetcher` 真实实现（`git clone --depth 1 --single-branch`、`npm install --omit=dev --ignore-scripts`，均带超时与失败清理）。
- `ValidateSourceURL` 是唯一的来源地址校验：只放行 http/https/git/git@，拒绝 `file://`、含空白/壳元字符、以 `-` 开头（会被 git 当选项）。命令走 `exec.Command` 而非 shell，但这条仍不可删。
- `Install` 顺序固定为：校验 manifest → 校验来源已审批 → 落两表 → 拉代码 → 装依赖 → 算校验和 → 起进程。任一前置步骤失败必须回滚两表并删目录。
- `HashPluginDir` 计算目录内容 sha256（排除 `node_modules` / `.git`），落 `plugin_installs.checksum` 与版本历史。
- `Preview` 返回能力/依赖清单（临时目录拉一份读 `package.json`，用完即删），**不落库、不起进程**。
- `ListVersions` / `RollbackAs`：每次安装/升级落一条 `plugin_versions`（note = install/upgrade/rollback）；回滚按历史 manifest 与来源重装。

## 浏览记录 / 续读位置（2026-11-09）

四个新方法 `RecordView(ctx, postID, cursor)` / `ListHistory(ctx, page, pageSize)` / `DeleteHistory(ctx, postID)` / `ClearHistory(ctx)` **都长在 `postService`（`PostService` 接口）里，没有新建 service**；`handler/history.go` 直接复用 `d.Post`。

- **actor 一律从 ctx 取**：四个出口用 `WithViewerID`（私有出口再叠 `viewerOrFail` → 401），**不接受 userID 参数**。这些接口写的都是「谁读过什么」，把 userID 做成参数迟早变成越权读写别人阅读历史的口子。`ListHistory` 的可见性与状态过滤全部交给 SQL（`applyVisibility` + `StatusIn`），不在 Go 里筛。
- **`recordBrowse` 是 `GetDetail` 与 `GetByID` 两个公开详情出口共用的钩子**：可见性已在调用方判过后直接写 `RecordView`（不再多查一次 `FindByID`）；匿名无归属主体跳过，写失败只 `Warnf` 降级，不能让读详情因读状态表抖动而 500。
- `checkCursorForPost` 校验 `content.RootID == post.ContentID`（帖子↔评论唯一关联，与 `content.go` 同源）：少了它，拿别人帖子里的评论 id 就能盲写读状态并顺带探测评论存在性。
- `EnrichWithReadState` 现在回填 `LastContentID`，并在 `LastReadAt == nil` 时**短路** `has_new` 与 `CountUnreadByRoot`——「有行」不再等于「标过已读」，否则一次点开就熄灭全站角标；游标则照常回传（只点开过也要能续读）。
- `MarkRead` 不再 fallback 写 `post.ContentID` 当游标：主楼不代表任何阅读进度，反会把已有的评论级位置抹回开头；留空即保持原游标。
- 新增 `history_test.go`。`ListHistory` 分页走 `pkg.Offset(page, pageSize)`（Go 侧变量名用驼峰），**网络字段名是 `page`/`page_size`**。
