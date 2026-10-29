# 修复「发帖成功但全站不可见」

## Why

线上站点 `forum.noelorin.top` 的站长发布了 3 篇文章，接口全部返回成功，但 `/spaces/tech`、时间线、`/sitemap-posts.xml`、`/latest.rss` 全部为空。实测证据（匿名可读的接口）：

- `POST /api/v1/leaderboard/list {"type":"posts"}` → `admin` 用户 `post_count: 3`、`total_points: 15`、`trust_level: basic`。15 分恰好等于 3 × `PointsPostCreate(5)`，签到 0 次，无其它加分来源 → 3 次发帖都成功落库。
- `POST /api/v1/posts/list`（不带任何筛选）→ `total: 0`；`/spaces/tech` 的 SSR 文本为「此视图暂无主题」。

根因是「发布成功」与「公开展示」被静默割裂：

1. `internal/model/constructors.go` 的 `NewPost()` 初始 `status=approved`，但 `internal/service/trust.go` 的 `ShouldForceApproval(TL0)` 读 `trust_tl0_require_approval`，**硬默认 `"true"`**，且该 key 不在 `model.DefaultSettings` 里 → 永远走默认 true；`internal/service/post.go` 只看信任等级、**不看角色**，把帖子改写成 `pending`。用户默认 `trust_level=basic`(TL0)，晋升条件为「≥1 天且 ≥1 篇」，因此站长账号同样是 TL0。
2. `internal/service/post.go` 的公开列表强制 `StatusIn = [approved, closed]` → 所有 `pending` 帖在公开面隐形。

而这 3 篇在后台也救不回来，三个缺陷叠加：

- `app/web/app/admin/posts/page.tsx` 默认筛选 `status='__all__'`，但 `__all__` 全仓只出现在两个前端文件里，后端 `internal/repository/post_filter.go` 把它当字面状态拼 SQL → `WHERE p.status = '__all__'` → 后台列表恒为空。
- `internal/service/post.go` 只在 `post_approval == "true"` 时才 `RecordPending`；`post_approval` 默认 `false` → 这 3 篇没有任何审核记录，`/admin/moderation/pending` 看不到，而 `Approve(id)` 只接受审核记录 id（`internal/service/moderation.go`）→ 无路可审。
- `internal/service/post.go` 的 `GetByID` / `GetDetail` 对非公开帖一律 404（除非有 `report:manage`）→ 作者连「帖子存在但审核中」都看不到。

## What Changes

- **信任审核门默认关闭**：`trust_tl0_require_approval` 的硬默认值改为 `false`，并写入 `model.DefaultSettings`（启动时 `syncDefaultSettings` 会补进设置表，后台可读写）。默认配置下 TL0 用户发帖不再被静默扣成待审。
- **列表状态过滤哨兵 `__all__`**：后端显式识别为「不过滤状态」，且只对具备 `report:manage` 的调用者生效；无权限者无论传什么一律只见 `approved` / `closed`。同一缺陷在举报列表（`service/report.go` 的 `List`）一并修掉。
- **待审帖作者可见**：`GetByID` / `GetDetail` 增加作者例外（`AuthorID == viewerID`），前端详情页明示「审核中」；他人仍 404。
- **待审必留痕**：任何被置为 `pending` 的帖子 MUST 写审核记录（去掉 `approval == "true"` 的前置条件），保证审核队列能看到它。
- **新增状态变更入口**：`POST /api/v1/posts/{id}/status`（body `{"status": ...}`，权限 `report:manage`），用于在没有审核记录时直接变更帖子状态；后台帖子列表提供「通过 / 拒绝」操作，用来救回存量待审帖。
- **发帖页反馈**：创建响应 `status=pending` 时提示「已提交审核」并跳转帖子详情，不再提示「发布成功」后落回一个看不到自己的空列表。

## Non-goals

- **不改 TL0 外链拦截**（`trust_tl0_block_links` 默认仍为 `true`）：TL0 用户发含 `http(s)://` / `www.` 的帖子仍会被拒（`信任等级不足，禁止外链`）。站长账号 15 分（< 50）仍是 TL0，写带链接的文章会失败——本次只做记录，不改变该策略。
- **不修 `space.post_count` 恒为 0**：`SpaceRepository.UpdatePostCount` 全仓无调用方，版面计数是死字段，另开变更。
- **不修默认积分榜 `post_count` 恒为 0**：`model.User.PostCount` 是 `gorm:"-"`，只有 `type=posts` 榜单会填充，另开变更。
- **不写存量数据迁移脚本**：存量 3 篇待审帖由站长在后台「帖子管理 → 待审」里点「通过」处理。
