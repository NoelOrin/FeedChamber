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
| email.go | EmailService | Mailer, KV (code 存储与验证) |
| email_iface.go | — | EmailService 接口定义 |
| oauth.go | OAuthService | UserRepository, OAuthRepository, SettingService, KV |
| checkin.go | CheckInService | CheckInRepository, UserRepository |
| announcement.go | AnnouncementService | AnnouncementRepository |
| space.go | SpaceService | SpaceRepository, PostRepository |
| audit_log.go | AuditLogService | AuditLogRepository (方法: Log/Record/List) |
| moderation.go | ModerationService | ModerationRepository + SettingService（待审队列/规则/配置管理、`CheckContent` 关键词与正则、`Mode` 读审核模式、`CheckExternal` 发布前外部审核、`SubscribeAsync` 异步复查消费者） |

> 上表未覆盖全部（现共 108 个文件 / 36 个接口）。另含（举例）：`space.go`、`group.go`、`trust.go`、`mfa.go`/`totp.go`、`pm.go`、`webhook.go`、`plugin.go`、`backup.go`、`draft.go`、`revision.go`、`trash.go`、`badge.go`、`theme.go`、`leaderboard.go`、`invite.go`、`watch.go`、`user_block.go`、`trust_level.go`、`digest.go` 等。

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

## 可见性与门控（2026-10-29 P2/P3）

- `visibility.go` 是内容可见性的**单一谓词**：`BuildVisibilityIndex` 把版面 `visibility`(public/login/level) 沿祖先链解析成有效策略（level > login > public，level 取链上最大 `min_level`）；`VisibleSpaceIDs(level, loggedIn)` 供 SQL 过滤，`PostRequiredLevel` 合并帖子 `min_level` 覆盖。游客一律按 TL0、`is_anonymous` 单独保留。
- `post.go` 的六个出口（List / GetByID / GetDetail / Search / SitemapEntries / ListByIDs）与置顶列表共用它：聚合出口把 `visibleSpaceIDs` 交给 repository 的 `buildVisibilityWhere` / `sitemapVisibilityWhere`；详情不足时返回 403「需要等级 N，当前等级 M」；作者例外为 `author_id = viewer`。有 `report:manage` 的审核者不受可见性限制（否则后台无法治理受限帖）。sitemap 与 RSS 固定按匿名（TL0）过滤。
- **从属内容出口与写入也走同一门槛**：`content.go` 的评论列表（`ListByPost`）、回复列表（`ListReplies`）与评论创建（`Create`，事务内加载帖子后立即判定）都通过 `PostViewer.CheckViewablePost`（`PostService` 实现）复用详情判定，帖子不可见时同样 404/403——否则受限帖的评论会绕开帖子门槛泄露，低等级用户也能盲写看不见的主题。作者例外与 `report:manage` 审核者由该入口放行。新增任何按 post/content 读或写内容的出口都必须接这个入口。
- `Space.Get` 也用它返回 403，`middleware.SpaceAuthGate` 已删除；`AuthGate` 保留为站点级登录墙。
- `gate.go` 是门控规则引擎：固定谓词集 + 4 个动作（allow/deny/require_review/rate_limit），按 `priority` 降序**首个命中**，无命中走 `gate_domain_defaults` 的显式默认；配置内存快照 30s + `gate:sync` Pub/Sub 失效；每次判定写带 trace 的结构化日志（命中规则 id / 默认动作），后台 `Simulate` 与线上共用 `Evaluate`。
- 发帖在 `post_create` 域、评论在 `comment_create` 域；两者只要正文含外链就额外进 `external_link` 域（只在含链接时求值，避免 deny 默认误伤纯文本）；`require_review` 会落 pending 并留痕 `strategy=gate`，`rate_limit` 按 `rl:gate:<domain>:u:<id>` / `lvl:<level>` 键执行。
- seed：外链域默认 deny + TL0 deny 规则（priority 100）+ 等级 ≥1 allow 规则（priority 50）；不 seed 发帖强制待审。

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
