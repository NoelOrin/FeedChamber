---
title: 内容与审核
description: 版面、帖子与评论、审核队列、举报处理、回收站、标签、公告与修订历史
---

内容侧的管理分两条线：**结构**（版面、标签、公告）决定内容放在哪里、怎么被展示；
**处置**（帖子、评论、审核队列、举报、回收站）决定单条内容能不能留下。两条线都在后台完成，不需要连数据库。

## 前置条件

- 已进入后台并具备对应权限码：版面要 `space:update`，帖子要 `post:read`，评论要 `comment:delete`，
  审核与举报要 `report:manage`，回收站要 `post:delete` 或 `comment:delete`，标签要 `tag:update`，
  公告要 `announcement:manage`。
- 内容是否先审后发由设置 `moderation_mode` 决定，见下方「审核队列」。

## 版面管理

`/admin/spaces` 是一棵无限嵌套的版面树，行内可「+子版」「编辑」「权限」「删除」。

1. 新建大区 → `POST /spaces/create`；在任意版面下「+子版」创建子版面。
2. 编辑弹窗里可改名称、`slug`（URL 标识，全局唯一）、图标、描述、父版面、排序值、可见性策略。
3. 拖动或调排序值 → `POST /spaces/reorder` 批量落位；换父级也可以走 `POST /spaces/{id}/move`。
4. 启禁用 `POST /spaces/{id}/status`：不传 `is_active` 就取反，传了就设为该值。
   这是「版面是否在运营」的标记，**访客看到什么由可见性策略决定**，两者独立。

可见性有三档，`level` 档另有最低等级：

| 取值 | 含义 | 附加字段 |
|------|------|----------|
| `public` | 匿名可读 | 无 |
| `login` | 需登录 | 无 |
| `level` | 需达到等级 | `min_level` |

子版面继承祖先策略：沿祖先链取**最严格**的一档（`level` > `login` > `public`），`min_level` 取链上最大值。
六个内容出口（列表、详情、搜索、sitemap、RSS、批量）共用同一套判定，不会出现「列表能看到、点进去 403」。
作者本人与持 `report:manage` 的审核者不受限制。

行内「权限」按钮打开版面级覆盖：只对 8 个按版面校验的权限码有效（`post:update`、`post:delete`、
`post:pin`、`post:lock`、`comment:update`、`comment:delete`、`user:update`、`space:update`），
语义与继承规则见 [用户与权限](/administration/users/)。

:::caution
删除版面时若还有子版面或帖子，接口会失败。先移走或清空内容，再删。
:::

## 帖子管理

`/admin/posts` 支持按状态筛选（`approved` / `pending` / `rejected` / `closed`），行内动作：

| 动作 | 接口 | 需要的权限码 |
|------|------|-------------|
| 置顶 / 取消置顶 | `POST /posts/{id}/pin` | `post:pin` |
| 锁定 / 解锁 | `POST /posts/{id}/lock` | `post:lock` |
| 设为 / 取消精华 | `POST /posts/{id}/feature` | `report:manage` |
| 直接改状态 | `POST /posts/{id}/status` | `report:manage` |
| 关闭讨论 | `POST /posts/{id}/close` | `post:create`（作者或管理员） |

精华会写 `featured_at`，并计入作者 XP——取消精华后这部分 XP 会随重算回收，不是简单的计数加减。

批量操作走 `POST /admin/batch/posts`，支持删除、置顶/取消置顶、锁定/解锁，单次最多 100 条。
帖子详情页（`/admin/posts/detail?id=`）列出正文、互动计数与该帖评论（可直接增删），作者一栏还能一键禁言，
处置时不必来回跳页。

## 评论管理

`/admin/comments` 是全局评论列表（`POST /posts/comments/list`，需要 `comment:delete`），
详情页可看单条评论上下文（`GET /comments/{id}`），删除走 `POST /comments/{id}/delete`。
批量删除走 `POST /admin/batch/comments`。评论删除是软删除，可在回收站恢复。

## 审核队列

`/admin/moderation` 顶部是审核模式卡片，决定新帖的初始状态：

| 模式 | 行为 |
|------|------|
| `off` | 发布即公开，不跑规则、不进队列 |
| `async` | 先公开，命中规则立刻转待审（默认） |
| `external` | 发布前调用外部审核服务，服务不可用回退 `async` |
| `manual` | 所有新帖先待审，人工通过后才公开 |

:::note
评论始终走关键词规则，不受这个开关影响。
:::

待审列表（`POST /admin/moderation/pending`）每行两个动作：

- **通过** → `POST /admin/moderation/{id}/approve`：审核记录置为通过，目标内容状态改为 `approved`。
- **拒绝** → `POST /admin/moderation/{id}/reject`：可填拒绝原因，目标内容改为 `rejected`。

每条审核记录保留目标类型、目标 ID、判定来源（`async` / `external` / `manual`）与原始正文，
所以「谁在什么时候因为什么拒了这条」事后可查。每次判定还会发布 `moderation.decided` 事件，
订阅了 Webhook 的系统能收到。

同页的规则管理（`/admin/moderation/rules/*`）与配置（`/admin/moderation/configs`）用于维护关键词/规则命中集；
没有把握时改模式比堆规则更安全。

## 举报处理

`/admin/reports` 按状态筛选（`pending` / `resolved`），列表里「聚合数」是同一目标被举报的次数。

1. 打开详情（`/admin/reports/detail?id=`）看清楚举报原因与目标正文。
2. 「处理」→ `POST /reports/{id}/status`。状态取 `pending` / `resolved` / `dismissed`。
3. 置为 `resolved` 时可附带处置动作：`none`（仅标记）、`delete_target`（删内容）、
   `lock_post`（锁定帖子）、`ban_user`（禁言作者）。

处理会写入处理人、处理时间与备注；被误报的内容用 `dismissed` 关掉，不要留着 `pending` 污染队列。

## 回收站

`/admin/trash` 分「帖子 / 评论」两段（`POST /admin/trash/posts/list`、`POST /admin/trash/comments/list`），
列出软删除的内容。行内「恢复」调 `POST /admin/trash/posts/{id}/restore` 或
`POST /admin/trash/comments/{id}/restore`，内容回到原处且状态不变。

回收站不是归档：恢复后可见性仍按原版面/帖子策略判定，不会因为恢复而对外可见。

## 标签

`/admin/tags` 维护标签本身（`POST /tags/create`、`POST /tags/{id}/update`、`POST /tags/{id}/delete`）。
标签与帖子是多对多关系，发帖时按名称批量设置，不存在的标签会被自动创建。后台只需要做重命名、合并前清理与删除。

## 公告

`/admin/announcements` 管理全站公告，字段与行为：

| 字段 | 说明 |
|------|------|
| `status` | `draft` / `published`，另有 `publish` / `unpublish` 单独动作 |
| `display_type` | 1 = Toast，2 = Alert 条幅，3 = Modal 弹窗，4 = iframe 嵌入（此时 `content` 填 URL） |
| `position` | Toast 的落点，六档（`top-left` … `bottom-center`），默认 `top-right` |
| `is_pinned` / `sort` | 置顶与排序，多个公告同时生效时按此排队 |
| `expires_at` | 过期时间，到点后前端不再展示 |

1. 新建公告 → 填标题与内容（Markdown，`display_type=4` 时填 URL）→ 保存为草稿。
2. 列表行「发布」→ `POST /admin/announcements/{id}/publish`；需要下线时「撤回」。
3. 用 `expires_at` 处理活动公告，别靠人工记得去撤。

## 修订历史

帖子与评论的每次编辑都会留版本，接口是 `POST /posts/{id}/revisions/list` 与
`POST /comments/{id}/revisions/list`，恢复用对应的 `/{revisionId}/restore`。

后台**没有**独立的「修订历史」页面：入口在前台帖子详情的「历史」按钮里，作者本人与有审核权限的账号都能看，
恢复后正文立即更新。做内容纠纷取证时用这个入口，而不是翻数据库。

## 验证

1. 把某版面设为「需等级 ≥ 2」，用 1 级账号打开列表：应看不到该版内容；直接打开帖子详情返回 403 并提示所需等级。
2. 在 `async` 模式下发一条命中关键词的帖子：帖子应立刻转 `pending`，出现在审核队列；通过后前台可见。
3. 举报一条帖子并置为 `resolved` + `delete_target`：帖子进入回收站，举报状态变为已处理。
4. 撤回一条公告：刷新前台，对应 Toast / 弹窗不再出现。

## 常见失败

| 现象 | 原因 | 处理 |
|------|------|------|
| 帖子发出去直接待审 | `moderation_mode` 是 `manual`，或 `async` 命中规则 | 改模式，或在队列里通过 |
| 审核通过后前台仍看不到 | 帖子状态已改，但可见性门槛更高 | 查版面可见性与帖子 `min_level` |
| 版面删除失败 | 还有子版面或帖子 | 先清空或移动内容 |
| 公告发布了但不显示 | `status` 还是草稿、已过期，或 `display_type=4` 的 URL 不可达 | 检查状态与 `expires_at` |
| 恢复的内容仍 404 | 原版面/帖子门槛仍生效 | 这是预期行为，调整可见性再看 |
