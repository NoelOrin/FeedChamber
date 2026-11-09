## ADDED Requirements

### Requirement: 浏览记录与已读标记共用一行但分列存储

系统 MUST 以 `topic_read_states` 表作为「某人看过某帖、看到哪条评论」的唯一事实源，身份为 `(user_id, post_id)` 复合唯一索引（`idx_user_post`），MUST NOT 为同一事实另建第二张表。

三个语义不同的事实 MUST 各占一列：

- `last_viewed_at` — 上次打开该帖的时间，由服务端在详情读取与游标上报时写入。
- `last_read_at` — 上次**显式标记已读**的时间，MUST 可空（`*time.Time`），且 MUST 只由 `POST /posts/{id}/read` 写入。
- `last_content_id` — 续读游标，值为该帖下用户最后读到的一条评论 id；空串表示没有可用游标。

被动浏览 MUST NOT 写 `last_read_at`，MUST NOT 清除已有的 `last_content_id`（上报为空时保留旧值）。同一行的多路写入 MUST 走 upsert，MUST NOT 因写全列而把另一路写入的列覆盖为零值。

#### Scenario: 只打开帖子不标记已读

- WHEN 已登录用户打开帖子详情，且从未点过「标记已读」
- THEN `topic_read_states` MUST 出现该 `(user, post)` 行且 `last_viewed_at` 非空
- AND `last_read_at` MUST 为 NULL

#### Scenario: 空游标上报不清旧值

- WHEN 某行 `last_content_id = 'c-42'`，随后 `POST /posts/{id}/view` 的 body 不带 `last_content_id`
- THEN `last_viewed_at` MUST 被刷新
- AND `last_content_id` MUST 仍为 `c-42`

### Requirement: 已登录访问者打开详情即产生浏览记录

系统 MUST 在登录用户读取帖子详情（`GET /posts/{id}`、`GET /posts/{id}/detail` 与规范地址详情出口）时自动记录浏览，MUST NOT 依赖前端按钮或任何客户端配合。匿名访问 MUST NOT 记录（无归属主体）；不可见帖子在门槛判定处即失败，MUST NOT 产生记录。

#### Scenario: 匿名访客不记录

- WHEN 未登录用户读取一篇匿名可见帖子
- THEN `topic_read_states` MUST NOT 新增任何行

#### Scenario: 帖子不可见时不记录

- WHEN 等级不足的用户访问 `min_level` 高于自己的帖子并得到 403
- THEN MUST NOT 写入该用户的浏览记录

### Requirement: 评论级续读游标可上报且可读回

系统 MUST 提供 `POST /posts/{id}/view`（body `{ last_content_id }`）供前端上报阅读位置，且 MUST 校验该评论存在并属于该帖——评论不存在或不属于本帖时 MUST 返回参数/未找错误并 MUST NOT 落库（防盲写他人帖子的评论 id）。

游标 MUST 能被读回：`PostListItem` DTO MUST 暴露 `last_content_id`，由 `EnrichWithReadState` 与 `has_new` / `last_read_at` / `unread_content_count` 一并填充，使列表与详情共用同一个出口。游标 MUST NOT 只写不读——历史上它正是死数据。

#### Scenario: 上报不存在的评论

- WHEN 调用 `POST /posts/{id}/view`，body 为 `{"last_content_id":"不存在的 id"}`
- THEN 系统 MUST 返回错误
- AND 该行的 `last_content_id` MUST NOT 被改写

#### Scenario: 详情返回游标

- WHEN 已登录用户请求自己读到过第 12 条评论的帖子的详情
- THEN 响应的 `data.last_content_id` MUST 等于该评论 id

### Requirement: 被动浏览不改变「没读过」筛选与未读角标语义

`list_view=new`（「没读过」筛选，`/timeline?list_view=new` 为被外链与 SEO 依赖的公开入口）的谓词 MUST 以「不存在 `last_read_at IS NOT NULL` 的行」定义，MUST NOT 退化为「不存在任何行」——否则被动浏览落库会让用户刚打开过的帖子从该筛选里凭空消失。`read` / `unread` 两档对 `last_read_at` 的比较 MUST 在 NULL 行上天然不成立，MUST NOT 为它们额外放行。

`has_new`（列表「有新回复」角标）MUST 只在 `last_read_at` 非空时与 `last_active_at` 比较；`last_read_at` 为 NULL 的行 MUST 视同「从未标记已读」，即 `has_new=false` 且 `last_read_at` 输出为 null。被动浏览 MUST NOT 点亮或熄灭任何角标。

#### Scenario: 打开过但没标记已读的帖子仍在「没读过」里

- WHEN 用户打开过帖子 A（产生 `last_viewed_at`）但从未标记已读，随后按 `list_view=new` 浏览时间线
- THEN 帖子 A MUST 出现在结果中

#### Scenario: 被动浏览不点亮角标

- WHEN 用户打开过帖子 B，帖子 B 自上次以来没有任何新回复
- THEN 帖子 B 的 `has_new` MUST 为 false

### Requirement: 历史列表只暴露访问者本来就能看见的帖子

`POST /history/list` MUST 在 SQL 侧与帖子列表共用同一条可见性谓词（`repository/buildVisibilityWhere`：帖子 `min_level` 覆盖、版面可见性策略、作者例外），MUST NOT 在 Go 里「先取一页再过滤掉看不见的」。被删除、进回收站或因等级/版面策略不可见的帖子 MUST NOT 出现在历史列表，MUST NOT 通过历史接口泄露其存在性。

分页总数 MUST 来自过滤后的同一谓词，否则前端分页条会给出翻不到的页。排序 MUST 为 `COALESCE(last_viewed_at, last_read_at) DESC`，使迁移前的存量行（只有 `last_read_at`）不至于从历史里消失。

#### Scenario: 等级不足的帖子不进历史

- WHEN 用户历史上读过一篇现在等级不够的帖子，随后请求 `POST /history/list`
- THEN 结果 MUST NOT 包含该帖
- AND `total` MUST NOT 计入该帖

#### Scenario: 已删除的帖子不进历史

- WHEN 用户读过的帖子随后被作者删除
- THEN `POST /history/list` MUST NOT 返回它

### Requirement: 历史可逐条删除与整体清空

系统 MUST 提供 `POST /history/{postId}/delete`（删除当前调用者的单条记录）与 `POST /history/clear`（清空当前调用者全部记录），二者 MUST 以调用者身份为作用域，MUST NOT 接受他人 user id 作为参数。对不存在的记录执行删除 MUST 幂等成功，MUST NOT 返回 404。

删除/清空 MUST NOT 连带影响帖子本身或其他用户的记录。

#### Scenario: 清空只影响自己

- WHEN 用户 A 调用 `POST /history/clear`
- THEN 用户 A 的历史 MUST 为空
- AND 用户 B 的历史 MUST 不变

#### Scenario: 删除不存在的记录

- WHEN 调用 `POST /history/{postId}/delete` 而当前用户从未读过该帖
- THEN 系统 MUST 返回成功

### Requirement: 历史接口遵循 RPC 风格与唯一读入口

历史记录相关端点 MUST 遵守 api-style 规范：写操作（上报、删除、清空）MUST 用 `POST` 且动词后缀化，业务参数 MUST NOT 走 query string；列表 MUST 为 `POST /history/list` 且分页字段固定 `page` / `pageSize`。同一资源的读取 MUST 只保留一种方法，MUST NOT 同时提供 `GET /history/list` 与 `POST /history/list`。

所有响应 MUST 经 `pkg.Success` / `pkg.SuccessPaginated` 写出统一信封。

#### Scenario: 列表分页参数走 body

- WHEN 调用 `POST /api/v1/history/list`，body 为 `{"page":2,"pageSize":20}`
- THEN 系统 MUST 返回 `{code:0,msg:"success",data:{items,total}}`
- AND MUST NOT 存在等价的 `GET /history/list` 读入口
