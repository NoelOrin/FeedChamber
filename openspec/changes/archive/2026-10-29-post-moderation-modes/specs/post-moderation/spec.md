## ADDED Requirements

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

## MODIFIED Requirements

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
