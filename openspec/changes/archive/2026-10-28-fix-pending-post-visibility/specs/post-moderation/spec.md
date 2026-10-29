## Purpose

定义帖子「发布 → 可见 → 可审核」的契约：新帖落库时的初始状态如何决定、列表如何按状态过滤、非公开帖对作者与审核者的可见性，以及审核记录与状态变更入口的必备性。目的是让「发布成功」与「公开展示」不再被静默割裂——任何不可见的帖子都必须存在一条可见、可操作的审核路径。

## ADDED Requirements

### Requirement: 新帖初始状态默认公开

帖子创建时 MUST 默认落为 `approved`。信任等级门控 `trust_tl0_require_approval` MUST 默认 `false`：默认配置下，TL0 用户创建帖子 MUST NOT 被强制置为 `pending`。

该设置的默认值 MUST 同时存在于 `model.DefaultSettings`（启动时写入设置表，后台设置页可读写）与 `ShouldForceApproval` 的硬默认值中，二者 MUST 一致；二者不一致时以设置表为准。

审核门被显式开启时，被置为 `pending` 的帖子 MUST 同时写入一条审核记录，使其出现在审核队列中；MUST NOT 存在「状态为 `pending` 但没有任何审核记录」的帖子。

#### Scenario: 默认配置下 TL0 用户发帖

- WHEN `trust_tl0_require_approval` 未在设置表与环境变量中配置，且 TL0 用户创建帖子
- THEN 该帖 `status` MUST 为 `approved`
- AND 该帖 MUST 出现在匿名可见的 `POST /api/v1/posts/list` 结果中

#### Scenario: 显式开启信任审核门

- WHEN 设置 `trust_tl0_require_approval=true`，且 TL0 用户创建帖子
- THEN 该帖 `status` MUST 为 `pending`
- AND 系统 MUST 为该帖写入一条审核记录，使其可在审核队列中被处理

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
