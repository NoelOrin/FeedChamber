# Repository 层

数据访问层：每个资源一个接口 + 一个 GORM/SQL 实现。**方法签名以源码为准，本文不复制**——此前文档里的方法清单长期与代码不一致（例如 `UserRepository` 早已没有 `FindByUUID`），维护成本高且误导读者。

实测规模：82 个生产文件 / 5335 行（42 个接口文件 + 40 个 `*_sql.go`），38 个 `XxxRepository` 接口，7 个测试文件。

## 约定

- 每个资源定义 `XxxRepository` 接口 + 工厂 `NewXxxRepository(store db.Store) XxxRepository`，工厂直接返回 SQL 实现；service 层只依赖接口。
- SQL 实现命名为 `sqlXxxRepo{db db.SQLStore}`，所有 DB 访问经 `r.db.DB().WithContext(ctx)`。
- 所有方法第一个参数为 `context.Context`（超时、trace、logger 都挂在它上面）。
- 列表方法成对出现：`List(ctx, offset, limit)` + `Count(ctx)`；计数变体命名 `CountXxx`。
- 错误一律 `WrapDBErr(err)`（errors.go），把驱动错误翻译为 `ErrNotFound` / `ErrDuplicate` 等哨兵值，由 service 层再包装成 `*pkg.AppError`。
- 软删除与审核相关的横切逻辑放在 `softdelete.go`、`checker_sql.go`、`post_filter.go`，不在各 repo 里重复。

## 文件布局（已统一）

每个域拆成两个文件，2026-10-28 起全层一致：

| 文件 | 内容 |
|------|------|
| `xxx.go` | 接口 + 工厂 `NewXxxRepository(store db.Store) XxxRepository` |
| `xxx_sql.go` | `sqlXxxRepo{db db.SQLStore}` 类型 + 全部方法实现 |

此前是两种布局并存（20 个域把实现塞在与接口同一个文件里），读代码要先推理"这个域的实现在哪"。现已全部拆开，新增 repository 请照此布局。

另有 4 个非 repo 文件承担横切职责：`errors.go`（哨兵错误与 `WrapDBErr`）、`softdelete.go`、`post_filter.go`、`checker_sql.go`（无对应接口文件，是内容审核检查器）。

`post_filter.go` 还导出列表哨兵 `StatusFilterAll`（`"__all__"` = 不过滤状态）：`buildPostListWhere`
对它一律跳过状态条件，避免被当作字面状态拼成 `WHERE p.status = '__all__'`（恒 0 行）。
「谁能用这个哨兵」由 service 层判定（`report:manage`），本层只保证它不会进 SQL。

## 接口清单

38 个接口，按域命名并一一对应 `internal/repository/<域>.go`：
`Admin` `Announcement` `AuditLog` `Backup` `Badge` `Banned` `CheckIn` `Content` `Conversation` `Credential` `Draft` `Favorite` `Follow` `Group` `Invite` `Like` `Meta` `Moderation` `Notification` `OAuth` `Permission` `Plugin` `Post` `Report` `Revision` `Role` `Setting` `Space` `SpaceRolePermission` `Tag` `TopicRead` `Trust` `Gate` `Upload` `User` `UserBlock` `UserMFA` `UserPreference` `Watch` `Webhook`（各 + `Repository` 后缀）。

几个容易误判的点：

- 版面级权限接口叫 `SpaceRolePermissionRepository`，文件却是 `space_permission.go`。
- 帖子评论的 repo 是 `Content`（`content.go` / `content_sql.go`），`PostRepository` 只管主题。
- `TrustRepository`（`trust.go` / `trust_sql.go`）除四张配置表外，还承担 XP 指标聚合：`MetricDailyCounts`（按自然日分组，**只返回计数不返回日期**，避免各驱动 DATE 类型差异）、`CountMetric`、`StreakHits`、`CountActiveDays`、`CountUpheldReports`，以及派生缓存写入 `SaveLevelSnapshot` / `SetManualLevel`（唯一调用方是 `service.TrustService`）。聚合只读既有业务表，不新增事件账本。
- `GateRepository`（`gate.go` / `gate_sql.go`）只管 `gate_rules` 与 `gate_domain_defaults` 两张表；求值、校验与缓存都在 `service/gate.go`。
- 可见性不是独立 repository：SQL 谓词在 `post_filter.go` 的 `buildVisibilityWhere`（列表/搜索/置顶共用）与 `sitemapVisibilityWhere`（sitemap/RSS），有效版面策略由 service 解析后以 `visibleSpaceIDs` 传入——SQL 不做祖先递归。
- 私信三张表（会话 / 成员 / 消息）由 `ConversationRepository` 一个接口承担。
- 会话（`user_sessions`）**没有 repository**：它由 `internal/gateway.SessionStore` 直接经 GORM 读写，是全链路唯一的会话吊销权威。同理 `api-keys`、`security_event`、`login_history`、`oauth_client` 也都在 `internal/gateway` 内自持数据访问，不在本层——这是待收敛的分层不一致。

## 添加新 Repository

1. 定义接口（列表方法同时定义 `List` + `Count`）
2. 实现 `sqlXxxRepo`（GORM），每个方法用 `WrapDBErr` 包装错误
3. 工厂函数返回 `&sqlXxxRepo{db: store.SQL()}`
4. 在 `internal/container/container.go` 装配，并在对应 service 构造函数里接收接口
