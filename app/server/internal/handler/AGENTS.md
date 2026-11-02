# Handler 层

HTTP 接口层，解析请求、调用 service、返回统一 JSON 响应。

实测规模：43 个生产文件 / 8700 行，39 个 `XxxHandler` 接口 + 一个 `Handler` 聚合体。本层测试很少：`sse_metrics_test.go` 覆盖 SSE 连接数指标 `feedchamber_sse_connections{channel}` 的连接/断开计数，`post_status_test.go` 覆盖 `POST /posts/{id}/status` 的参数解析与统一响应信封（用嵌入 nil 接口的 stub 只实现被测方法），`seo_sitemap_test.go` 钉住 `/sitemap-spaces.xml` 只收录匿名可读的版面（login / level 版面不得进爬虫面）。

## 约定

- 每个文件定义 `XxxHandler` 接口 + 私有实现 struct
- 构造函数 `newXxxHandler(svc XxxService) XxxHandler`
- 分页列表用 `pkg.SuccessPaginated(ctx, items, total)` 返回（内部包成 `{items,total}`），非分页用 `pkg.Success(ctx, data)`；两者输出固定 `{code:0,msg:"success",data}`
- 错误统一用 `pkg.HandleError(ctx, err)`：`*pkg.AppError` 走其业务码，其余一律压平为 `500 INTERNAL_ERROR`
- 参数获取：路径参数 `ctx.Params().Get("id")`，请求体 `ctx.ReadJSON(&req)`；分页用 `readListParams(ctx)` 返回 `ListParams{Page, PageSize}`，时间范围用 `readTimeRange(ctx, p)`
- **列表接口必须容忍空 body**：`ctx.ReadJSON` 在无 body 的 POST 上返回 `io.EOF`，直接 `pkg.Fail(INVALID_PARAMS)` 会把「合法的空筛选」打成 400。写法是 `if err := ctx.ReadJSON(&req); err != nil && !errors.Is(err, io.EOF) { … }`（`notification.go` / `banned.go` 已按此处理，`readListParams` 内部同理）。`/admin/banned/list` 曾因此整个列表恒空——前端 `bannedService.list()` 不传 body。
- 当前用户来自**服务端会话**（不是 JWT）：`currentSession(ctx)` 取 `*model.UserSession`，`currentUser(ctx)` / `currentUserID(ctx)` / `currentRole(ctx)` / `requireUser(ctx)` 都从网关写入的 ctx values 派生（`internal/gateway/auth.go` 的 `applySession`）
- 这些 ctx values 的键名是跨包契约，用 `pkg.CtxUserID` / `CtxUsername` / `CtxRole` / `CtxSession` / `CtxAuthMethod` / `CtxAPIKeyScopes` / `CtxClientScopes` 常量（`internal/pkg/ctxkey.go`），不要写字面量；注意 `CtxUserID` 装的是 `User.ID`（ULID）；对应的 handler 辅助函数叫 `currentUserID`（2026-10-28 由 `currentUserUUID` 改名，旧名会让人误以为在读 UUID 列）
- 每个方法上方必须有 swag 注解 (@Summary, @Description, @Tags, @Param, @Success, @Router, @Security)；`make swag` 依赖它们生成 `docs/` 与根 `openapi.json`

## 文件列表

| 文件 | Handler | 代表方法（完整清单以源码为准） |
|------|---------|------|
| handler.go | Handler (聚合) + Health | `Deps`（本层消费的服务清单）+ `New(*Deps)` 接线 39 个子 handler（38 个 `newXxxHandler` + 导出的 `NewOAuthHandler`）+ `/health` 探针。不引用 container，也不再有 `Shutdown()`——后台 worker 关停归 `container.Runtime` |
| ctx.go | 工具 | `currentSession()`, `currentUser()`, `currentUserID()`, `currentRole()`, `requireUser()` |
| params.go | 工具 | `readListParams()`, `readTimeRange()` |
| auth.go | AuthHandler | Register, ApproveRegistration, Login, RefreshToken, ChangePassword, FirstChangePassword, ResetPassword, ForgotPassword, ResetPasswordByCode, SendVerifyEmail, VerifyEmail |
| user.go | UserHandler | GetProfile, GetByID, List, Search, UpdateProfile, UpdateRole, Delete |
| banned.go | BannedHandler | Ban, Unban, Get, ListByUserID, BanInSpace, UnbanInSpace, GetInSpace |
| post.go | PostHandler | List, GetByID, GetDetail, ListByIDs, Create, Update, Delete, Search, TogglePin, ToggleLock, SetClosed, SetStatus, SetFeatured, SetPin, Unpin, ListPinned, MarkRead |
| content.go | ContentHandler | ListByPost, ListReplies, ListAll, Create, Update, Delete, AdminDelete, AdminGet |
| tag.go | TagHandler | List, Create, Update, Delete, GetPostTags, SetPostTags |
| like.go | LikeHandler | ToggleContentLike, GetContentLikeStatus |
| favorite.go | FavoriteHandler | Toggle, GetStatus, ListByUser |
| follow.go | FollowHandler | Follow, Unfollow, GetStatus, GetFollowers, GetFollowing, GetCounts |
| role.go | RoleHandler | List, Create, Delete |
| permission.go | PermissionHandler | ListPermissions, GetRolePermissions, SyncRolePermissions, GetSpaceRolePermissions, GetEffectiveSpaceRolePermissions, GetSpacePermissionsTree, SyncSpaceRolePermissions, ClearSpaceRolePermissions |
| upload.go | UploadHandler | Upload, Presign, Confirm, List, GetFile, Delete |
| notification.go | NotificationHandler | List, CountUnread, MarkRead, MarkAllRead, Delete, Stream |
| report.go | ReportHandler | Create, List, Get, UpdateStatus, Delete |
| admin.go | AdminHandler | GetStats, Dashboard, Search, GetUserActivities, BatchUsers, BatchPosts, BatchComments, ReconcileCounters |
| gate.go | GateHandler | 门控规则 CRUD、能力域默认动作、后台「以用户试算」与元数据（域/字段/操作符/动作/窗口指标）。整段挂 `gate:manage` |
| trust.go | TrustHandler | 等级 CRUD（ListLevels/SaveLevel/DeleteLevel）、XP 规则与连续签到阶梯、全量重算、用户 XP 指标分解与人工调整（GetUserXP/AdjustXP/ListAdjustments/DeleteAdjustment）、公开等级表（GetPublicLevels → `GET /levels`）。管理路由挂 `level:manage` |
| setting.go | SettingHandler | Get, GetAll, GetPublic, Set, BatchSet, Delete |
| announcement.go | AnnouncementHandler | List, Create, Update, Delete, Publish, Unpublish, Get, ListPublished |
| oauth.go | OAuthHandler | GetAllProviderConfigs, GetProviderConfig, GetAuthURL, HandleCallback, GetLinkedAccounts, LinkAccount, UnlinkAccount |
| checkin.go | CheckInHandler | CheckIn, GetStatus, GetHistory |
| space.go | SpaceHandler | Tree, Get, Create, Update, Delete, ToggleActive, Reorder, Move, Flat, Roots, Children, Ancestors, Breadcrumb, Descendants, SubTree |
| dashboard.go | Handler.Dashboard | 首页聚合（用户资料+关注数+公告），5s 超时 + sync.WaitGroup 并发 |
| moderation.go | ModerationHandler | ListPending, Approve, Reject, CreateRule, UpdateRule, DeleteRule, ListRules, GetConfigs, SetConfig |
| mfa.go | MFAHandler | SetupTOTP, EnableTOTP, DisableTOTP, RegenerateBackup, VerifyLogin |
| group.go | GroupHandler | List, Create, Update, Delete, ListMembers, AddMember, RemoveMember, GetPermissions, SyncPermissions |
| pm.go | PMHandler | ListConversations, CreateConversation, ListMessages, SendMessage, MarkRead |
| watch.go | WatchHandler | Set, List, Delete, GetStatus |
| user_block.go | UserBlockHandler | Block, Unblock, Status, List |
| draft.go | DraftHandler | Save, List, Delete |
| revision.go | RevisionHandler | ListPost, RestorePost, ListComment, RestoreComment |
| trash.go | TrashHandler | ListPosts, RestorePost, ListComments, RestoreComment |
| backup.go | BackupHandler | Create, List, Cleanup, Restore, Download, Delete |
| webhook.go | WebhookHandler | Create, List, Update, Delete, ListDeliveries, Ping, RetryDelivery |
| plugin.go | PluginHandler | List, Install, Update, SetEnabled, Delete |
| badge.go | BadgeHandler | List, ListByUser |
| theme.go | ThemeHandler | List, GetPreference, SetPreference |
| leaderboard.go | LeaderboardHandler | List |
| invite.go | InviteHandler | Create, List, Delete |
| seo.go | SEOHandler | Sitemap, SitemapIndex, SitemapSpaces, SitemapTags, Robots, LatestRSS |
| realtime.go | RealtimeHandler | Stream |

## 不在 handler 层的 HTTP 端点

以下端点由 `internal/gateway` 自己注册，本层没有对应文件，找它们要去 `app/server/internal/gateway/`：

- **登出** `POST /auth/logout` → `SessionManager.LogoutHandler`（`session.go`）。曾经 handler 里有一份逐字相同的 `authHandler.Logout`，2026-10-14 删除，swag 注解一并搬到真正挂载的网关方法上。
- **会话列表 / 单点吊销 / logout-all** → `session.go`
- **Security（暴力破解、异常位置、安全事件、登录历史）** → `security.go`
- **API Key 增删列** → `apikey.go`
- **自建 OAuth2 Provider 与 OIDC SSO**（`/oauth/token`、`/.well-known/*`） → `oauth_provider.go`、`sso*.go`
- **`/metrics`** → 不在业务路由里，由 `METRICS_PORT`(9091) 独立 listener 暴露；原 `handler/metrics.go` 已删除。

> `category.go` 已随 Category 模块下线移除，分类能力由 `space.go`（版面树）承接；`cookie.go` 也已删除，cookie 读写统一在 `internal/pkg/cookie.go`。

## 添加新 Handler

1. 定义接口 + 私有 struct + 构造函数
2. 实现方法，每个方法加 swag 注解
3. 在 `handler.go` 的 `Deps` 加服务字段、`Handler` 加 handler 字段，并在 `New()` 中接线
4. 到 `server/wiring.go` 把 container 里的对应服务填进 `handler.Deps`（漏了会编译报错）
5. 在 `internal/router/router.go` 挂路由，然后 `make swag` 刷新 `openapi.json`

## 站点自定义的 writeActor（2026-10-30）

`customize.go` 的 `writeActor(ctx)` **不再假定调用方是人工**：网关的 Enforce 对 API Key / OAuth 客户端与用户会话一视同仁，两者都会进后台路由。来源由 `pkg.CtxAuthMethod` 决定——会话 → `WriteSourceUI`，机器凭证 → `WriteSourceMachine`，机器凭证的 `CanPublish` 只取凭证自身 scope（`authz.ScopeHas`），不继承角色。

漏掉这一步的表现是：持 `customize:write` 的 Key 调 `POST /admin/customization/snippets/{id}/toggle` 就能让 JS 上线，`customize:publish` 与草稿闸门同时失效。

## 前台聚合的等级过滤

`GetPublicCustomization` 用 `TrustService.LevelOf` 取访问者真实等级（游客 0）。此前固定传 0，`visibility=level` 的导航与首页模块对任何人都不显示。五个分区独立降级为空并落 warn 日志（`logCustomizeIssue`），DB 抖动不拖垮整站外观。
