# post-moderation Specification

## Purpose
定义帖子「发布 → 可见 → 可审核」的契约：新帖落库时的初始状态如何决定、列表如何按状态过滤、非公开帖对作者与审核者的可见性，以及审核记录与状态变更入口的必备性。目的是让「发布成功」与「公开展示」不再被静默割裂——任何不可见的帖子都必须存在一条可见、可操作的审核路径。
## Requirements
### Requirement: 新帖初始状态默认公开

帖子创建时 MUST 依据 `moderation_mode` 决定初始状态：`off` / `async` / `external`（外部判定放行）MUST 落 `approved`；`manual` MUST 落 `pending`。信任等级门控 `trust_tl0_require_approval` MUST 默认 `false`，且只在非 `off` 模式下生效：默认配置下，TL0 用户创建帖子 MUST NOT 被强制置为 `pending`。

该设置的默认值 MUST 同时存在于 `model.DefaultSettings`（启动时写入设置表，后台设置页可读写）与 `ShouldForceApproval` 的硬默认值中，二者 MUST 一致；二者不一致时以设置表为准。

任何审核门（`manual` 模式、门控规则的 `require_review`（含原 TL0 强制待审）、外部判定命中、异步规则命中）导致的 `pending` 帖子 MUST 同时写入一条审核记录，使其出现在审核队列中；MUST NOT 存在「状态为 `pending` 但没有任何审核记录」的帖子。

#### Scenario: 默认配置下 TL0 用户发帖

- WHEN `trust_tl0_require_approval` 未在设置表与环境变量中配置，`moderation_mode` 为默认 `async`，且 TL0 用户创建帖子
- THEN 该帖 `status` MUST 为 `approved`
- AND 该帖 MUST 出现在匿名可见的 `POST /api/v1/posts/list` 结果中

#### Scenario: 显式开启信任审核门

- WHEN 设置 `trust_tl0_require_approval=true`，且 TL0 用户创建帖子
- THEN 该帖 `status` MUST 为 `pending`
- AND 系统 MUST 为该帖写入一条审核记录，使其可在审核队列中被处理

#### Scenario: 不审核模式忽略信任门

- WHEN `moderation_mode=off` 且 `trust_tl0_require_approval=true`，且 TL0 用户创建帖子
- THEN 该帖 `status` MUST 为 `approved`
- AND 系统 MUST NOT 写入审核记录

### Requirement: 列表状态过滤的哨兵语义

列表接口的状态过滤参数 MUST 识别哨兵值 `__all__` 为「不过滤状态」。该语义 MUST 只对具备审核权限（`report:manage`）的调用者生效；不具备该权限的调用者 MUST 始终只能看到 `approved` / `closed` 的帖子，无论其传入何种 `status` 值。

#### Scenario: 后台列表请求全部状态

- WHEN 具备 `report:manage` 的调用者以 `status="__all__"` 请求 `POST /api/v1/posts/list`
- THEN 响应 MUST 包含全部未软删状态的帖子（含 `pending` 与 `rejected`）

#### Scenario: 匿名调用者请求全部状态

- WHEN 匿名调用者以 `status="__all__"` 请求 `POST /api/v1/posts/list`
- THEN 响应 MUST 只包含 `approved` / `closed` 的帖子

### Requirement: 非公开帖的可见性与作者例外

非公开状态（非 `approved` / `closed`）的帖子 MUST 对无审核权限的调用者返回 `404`（MUST NOT 泄露资源存在性），但**作者本人 MUST 例外**：作者读取自己处于非公开状态的帖子 MUST 成功，响应 MUST 携带真实 `status`，前端 MUST 明示「审核中」。

#### Scenario: 作者读取自己的待审帖

- WHEN 作者请求 `GET /api/v1/posts/{id}` 或 `GET /api/v1/posts/{id}/detail`，该帖 `status=pending` 且当前登录用户即作者
- THEN 系统 MUST 返回该帖，MUST NOT 返回 404
- AND 响应中的 `status` MUST 为 `pending`

#### Scenario: 他人读取待审帖

- WHEN 另一名不具备 `report:manage` 的用户请求同一篇 `status=pending` 的帖子
- THEN 系统 MUST 返回 404

### Requirement: 审核状态变更入口

系统 MUST 提供 `POST /api/v1/posts/{id}/status`，请求体 `{"status": "..."}`，取值限 `approved` / `pending` / `rejected` / `closed`，受 `report:manage` 权限约束。该入口 MUST 能在帖子没有任何审核记录时直接变更其状态，用于救回存量待审帖。管理后台的帖子列表 MUST 提供触发该入口的操作。

#### Scenario: 审核者通过一篇无审核记录的待审帖

- WHEN 具备 `report:manage` 的调用者请求 `POST /api/v1/posts/{id}/status`，body 为 `{"status":"approved"}`，该帖当前为 `pending` 且不存在审核记录
- THEN 该帖状态 MUST 变为 `approved`
- AND 该帖 MUST 出现在匿名可见的列表接口中

#### Scenario: 非法状态值被拒

- WHEN 调用者传入 `{"status":"whatever"}`
- THEN 系统 MUST 返回参数错误（`2xxx`）
- AND 该帖状态 MUST NOT 发生改变

### Requirement: 文章审核模式四选一

系统 MUST 以系统设置 `moderation_mode` 表达文章（帖子）的审核节奏，取值限 `off` / `async` / `external` / `manual`，默认 MUST 为 `async`。

该默认值 MUST 同时存在于 `model.DefaultSettings`（启动时写入设置表）与 `model.DefaultModerationMode`（`ParseModerationMode` 的回落值），二者 MUST 一致。写入非法取值时设置服务 MUST 返回参数错误（`2xxx`）并且 MUST NOT 落库；读取到缺失或非法取值时 MUST 回落 `async`（MUST NOT 回落为免审）。

- `off` — 不审核：帖子创建即 `approved`，MUST NOT 运行审核规则，MUST NOT 写审核记录，`trust_tl0_require_approval` 门 MUST NOT 生效。
- `async` — 发布后异步审核：帖子创建即 `approved`，发帖请求内 MUST NOT 同步运行规则；帖子创建事件 MUST 携带 `moderation_async=true` 交给事件总线消费者，命中规则的帖子 MUST 转为 `pending` 并写入一条审核记录（`strategy=async`），交由人工判断。
- `external` — 外部自动化审核：帖子创建前 MUST 同步调用外部审核服务，`allow` 放行；`flag` / `block` / `replace` MUST 置 `pending` 并写入审核记录（`strategy=external`）；服务不可用（未配置、超时、非 2xx、未知结论）MUST 回退为 `async`，MUST NOT 因外部服务不可用而阻塞发帖。
- `manual` — 全部人工：帖子创建即 `pending`，且 MUST 写入审核记录（`strategy=manual`）。

模式只作用于文章；评论 MUST 继续按 `moderation_configs` 的关键词/正则规则判定，不受该设置影响。

#### Scenario: 默认配置下发帖

- WHEN `moderation_mode` 未在设置表配置，且用户创建帖子
- THEN 该帖 MUST 立即为 `approved`
- AND 该帖 MUST 出现在匿名可见的 `POST /api/v1/posts/list` 结果中
- AND 发帖请求 MUST NOT 因规则检查而返回「含违规内容」

#### Scenario: 异步模式命中规则

- WHEN `moderation_mode=async` 且内容命中一条 `flag` 规则
- THEN 发帖请求 MUST 成功返回且帖子初始为 `approved`
- AND 审核消费者处理发帖事件后，该帖 MUST 变为 `pending`
- AND MUST 存在一条 `strategy=async` 的待审记录

#### Scenario: 外部服务不可用

- WHEN `moderation_mode=external` 且外部审核服务超时或未配置 endpoint
- THEN 发帖 MUST 成功，帖子 MUST 为 `approved`
- AND 系统 MUST 按 `async` 模式安排异步复查

#### Scenario: 写入非法模式

- WHEN 调用 `POST /api/v1/admin/settings/moderation_mode/update`，body 为 `{"value":"whatever"}`
- THEN 系统 MUST 返回参数错误（`2xxx`）
- AND `moderation_mode` 的既有值 MUST NOT 改变

### Requirement: 旧布尔审核开关降级为兼容别名

`post_approval` MUST 只作为一次性迁移输入存在：启动时 MUST 在补齐默认设置之前执行迁移——`moderation_mode` 缺行时，`post_approval=true` MUST 写成 `moderation_mode=manual`，其余写成默认 `async`；已有 `moderation_mode` 行时 MUST 保持现值不变。迁移之后该开关 MUST 被忽略，新代码与后台 MUST 只读写 `moderation_mode`。

运行时兜底：`moderation_mode` 行缺失（例如迁移未执行）时，`post_approval=true` MUST 仍等价于 `manual`；一旦 `moderation_mode` 有值，`post_approval` MUST NOT 再影响判定——否则一条无法从界面关闭的历史配置会把站点永久锁在「全部人工」。

#### Scenario: 存量站点仍开着旧开关

- WHEN 升级启动时 `post_approval=true` 且 `moderation_mode` 缺行
- THEN 迁移 MUST 写入 `moderation_mode=manual`
- AND 该站点的用户创建帖子时 MUST 得到 `pending`
- AND MUST 写入一条 `strategy=manual` 的待审记录

#### Scenario: 存量站点未开审核

- WHEN 升级启动时 `post_approval=false`（或不存在）且 `moderation_mode` 缺行
- THEN 迁移 MUST 写入 `moderation_mode=async`
- AND 用户创建帖子时 MUST 立即得到 `approved`

#### Scenario: 新开关写过后旧开关失效

- WHEN `moderation_mode=async` 已落库，`post_approval` 仍残留 `true`，用户创建帖子
- THEN 该帖 MUST 为 `approved`
- AND 系统 MUST NOT 为该帖写入待审记录

