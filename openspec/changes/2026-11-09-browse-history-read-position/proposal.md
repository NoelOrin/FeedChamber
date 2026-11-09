# 浏览记录 + 评论级续读位置

提案状态：**草案（待评审）** · 创建日期：2026-11-09

## Why

问「有没有上次浏览记忆位置，精确到评论」时，答案是**字段有、链路断、体验无**；问「有没有历史浏览记录」时，答案是**没有**。两件事共用同一份数据，必须一起修，否则第二遍还是断的。

现状（实测）：

1. `topic_read_states` 只有 `(user_id, post_id, last_read_at, last_content_id)`（[model/topic_read.go](../../../app/server/internal/model/topic_read.go)）。`last_content_id` 是评论级游标，`POST /posts/{id}/read` 也确实在收它（[handler/post.go:581-589](../../../app/server/internal/handler/post.go)）。
2. **但没有任何出口能把它读回来**：`PostListItem` DTO 里没有这个字段（[model/post.go:85-95](../../../app/server/internal/model/post.go)），`EnrichWithReadState` 只回填 `has_new` / `last_read_at` / `unread_content_count`。游标写进去就是死数据。
3. **前端也从来没传过它**：详情页「标记已读」调 `postService.markRead(postId)` 不带第二参（[post-detail.tsx:122](../../../app/web/components/post/post-detail.tsx)），后端于是 fallback 写 `post.ContentID`——那是**主楼** content，不是「读到第几条评论」。
4. **没有任何定位/滚动恢复**：`components/content`、`components/post` 内 `scrollIntoView` / hash 锚点 0 处；未读数还是按时间窗算的（`CountUnreadByRoot(contentID, last_read_at)`）。
5. 浏览记录：库里名字相近的三个都不是它——`login_history`（安全登录史）、`GET /checkin/history`（签到）、详情页「历史版本」（revision 编辑史）。`posts.view_count` 是全站聚合数，不留 per-user 记录。
6. 更要命的是**写入时机**：`GetDetail` 只 `incrViewCount`，不写 `topic_read_states`。也就是说现在"记录"只在用户**手动点按钮**时才产生——那是书签，不是浏览记录。

## What Changes

存储沿用 `topic_read_states`（每 `(user, post)` 一行），**不新建流水表**：一张表描述同一件事的两半，避免双写漂移。三个事实拆成三列：

| 列 | 语义 | 谁写 |
|----|------|------|
| `last_viewed_at`（新增） | 上次打开这个帖 | 服务端在详情读取时自动写 |
| `last_read_at`（改为可空） | 上次**显式标记已读** | 只由 `POST /posts/{id}/read` 写 |
| `last_content_id` | 读到的那条评论（续读游标） | 浏览上报与标记已读都可写 |

- **被动浏览落库**：`GetDetail` 与 `GetByID` 两个公开详情出口共用一个 `recordBrowse` 钩子，对已登录访问者 upsert 一行（只写 `last_viewed_at` / `last_content_id`，**不碰** `last_read_at`）。匿名访问不记录；写失败只降级（不能让读详情的请求因读状态表抖动 500）。
- **评论级游标上报**：新增 `POST /posts/{id}/view`，body `{ last_content_id }`；前端评论列表观察可见性，防抖上报最后一条可见评论。游标 MUST 校验 `content.RootID == post.ContentID`（防拿别人帖子的评论 id 盲写）。
- **身份只从 ctx 取**：四个出口写的都是「谁读过什么」，`RecordView` / `ListHistory` / `DeleteHistory` / `ClearHistory` 均从 `WithViewerID` 取访问者，**不接收 userID 参数**——参数化的身份早晚会成为越权读写口的入口。网关映射走 `favorite:self` / `post:read`（自助数据一族），新增端点默认拒绝，必须同步 `initRouteMap`。
- **游标回读**：`PostListItem` 补 `last_content_id`，`EnrichWithReadState` 与历史出口 SQL 都填充；详情页进入时定位到该评论并画「上次读到这里」分隔线。
- **历史列表**：`POST /history/list`（分页，按 `COALESCE(last_viewed_at, last_read_at) DESC`）返回「我看过哪些帖 + 各看到哪条评论」，复用帖子可见性谓词（SQL 里过滤，不在 Go 里取一页再筛，否则 `total` 说谎并泄露受限帖存在性）。
- **隐私出口**：`POST /history/{postId}/delete` 删单条、`POST /history/clear` 清空。
- **守住一个既有语义**（这条是本提案最容易改坏的地方）：`list_view=new`（「没读过」筛选，`/timeline?list_view=new` 是被链接的公开入口）今天的定义是「`topic_read_states` 里没有我的行」。被动浏览一旦建行，帖子就会从 `new` 里凭空消失。因此 `new` 谓词 MUST 改为「没有 `last_read_at IS NOT NULL` 的行」，把"没读过"与"没标记已读"分开。同理 `has_new`（有新回复角标）MUST 只在 `last_read_at` 非空时参与比较，被动浏览不得点亮或熄灭角标。

## Non-goals

- **不做访问流水**：不记「同一帖 3 月看过 5 月又看过」的明细；一行一帖只留最后一次。真要明细是另一份数据、另一个提案。
- **不做时间清理与配额**：不做"N 天后自动删"，也不做每用户条数上限。列表分页天然封顶，写满一亿行之前不是问题。
- **不做管理端**：不给管理员查他人浏览记录，不做全站浏览分析。这是隐私边界，不是功能缺失。
- **不动通知深链**：`lib/notification-target.ts` 的 type→路由映射不在本提案范围（评论定位走游标，不走通知锚点）。
- **不做未读跨页跳转**（「跳到第一条未读」按钮）：本提案只恢复**上次位置**，未读导航留待后续。
