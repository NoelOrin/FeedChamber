# api-style Specification (delta)

## MODIFIED Requirements

### Requirement: RPC 风格动词后缀 URL

所有业务 HTTP 接口 MUST 使用 `POST` 方法，并以动词后缀表达"资源 + 动作"，禁止 RESTful 风格。仅 `/health` 等纯系统探活接口允许 `GET`。

URL 路径段 MUST 表达资源与动作，统一动作词包括：`/list`（列表/分页查询）、`/create`（创建）、`/{id}/update`（更新）、`/{id}/delete`（删除）、`/{id}/status`（状态变更）、`/{id}/<custom>`（其他业务动作，如 `/pin`、`/lock`、`/publish`）。

示例 MUST 使用在用的模块。`Category` 模块已下线、由 `Space`（版面树）承接，规范与文档中的示例不得再引用 `/categories`。

#### Scenario: 列表接口使用 POST + /list 后缀

- WHEN 客户端请求版面列表
- THEN 系统 MUST 接受 `POST /api/v1/spaces/list`，且允许请求体为空对象

#### Scenario: 查询参数不得承载过滤条件

- WHEN 客户端按状态与分页筛选举报列表
- THEN 系统 MUST 接受 `POST /api/v1/reports/list`，过滤条件与分页（`page`/`pageSize`）通过 request body 传递，而非 query string

### Requirement: 统一 JSON 响应结构

所有 HTTP 出口 MUST 只通过 `internal/pkg` 的响应封装写响应：`pkg.Success(ctx, data)`、`pkg.SuccessPaginated(ctx, items, total)`、`pkg.Fail(ctx, code, msg?)`、`pkg.FailStatus(ctx, status, code, msg?)`、`pkg.HandleError(ctx, err)`。这覆盖 handler、`pkg/gateway` 的全部中间件与自注册端点、以及 maintenance / request-timeout / recovery 中间件。MUST NOT 用 `ctx.JSON` / `ctx.StopWithJSON` / `ctx.WriteString` 自行拼装信封。

响应体 MUST 为固定结构 `{ "code": 0, "msg": "success", "data": ... }`。`msg` 在成功路径上是协议字面量 `"success"`（不是给用户看的文案，客户端 MUST NOT 渲染它）；失败路径的 `msg` 取 `errMsg` 表中该码的中文文案，需要补充具体原因时由调用点作为第二个参数传入。列表分页 MUST 额外经 `pkg.SuccessPaginated` 返回 `total`。

错误码 MUST 使用 `internal/pkg/errors.go` 的单一共享命名空间，分段为：`1xxx` 认证与凭证、`2xxx` 参数校验、`3xxx` 资源、`4xxx` 限流与请求体、`5xxx` 服务端。任何子系统（含网关）MUST NOT 另起私有号段或把 HTTP 状态码当业务码使用；新增语义 MUST 先在该表登记，并同步 `packages/api-client` 的 `ErrorCode` 常量。

`errToHTTPStatus` MUST 是业务码 → HTTP status 的唯一映射点（含 `5004→504`、`5005→503`）。仅当状态本身属于契约、且不能由业务码推导时，MUST 使用 `pkg.FailStatus` 显式钉住状态——例如封禁账号返回 404 而非 403，以避免泄露资源存在性。

例外：OAuth 2.0 / OIDC 的协议端点（token、authorize、revoke、`/.well-known/*`）MUST 保持 RFC 规定的响应形状（如 `{"access_token",…,"token_type":"Bearer"}` 与 `{"error","error_description"}`），因为外部依赖方按标准解析，套用论坛信封会破坏互操作。

#### Scenario: 成功列表响应携带总数

- WHEN 版面列表请求成功
- THEN 系统 MUST 返回 `code: 0`、`msg: "success"`，且 `data` 含 `items` 与 `total`

#### Scenario: 业务错误映射为结构化响应

- WHEN 业务层返回 `*pkg.AppError`
- THEN 系统 MUST 经 `pkg.HandleError` 返回 `{ code, msg, data: null }` 且 HTTP status 与错误段匹配

#### Scenario: 网关拒绝必须可被客户端读取

- WHEN 网关在无凭证、权限不足或触发限流时中止请求
- THEN 响应 MUST 为 `{code, msg, data}` 且原因文本位于 `msg`（此前置于 `message` 会被 `api-client` 丢弃，只剩 `HTTP 401 Unauthorized`）

#### Scenario: 协议端点保持标准形状

- WHEN 外部 relying party 调用 `/oauth/token` 且 grant 非法
- THEN 系统 MUST 返回 RFC 6749 的 `{"error": "invalid_grant", ...}`，MUST NOT 包装为论坛信封

### Requirement: 前端 service 请求约定

前端 `api-client` 的所有请求 MUST 默认 `method: 'POST'`，URL MUST 带显式动词后缀（如 `/spaces/list`），不得省略动作词。示例 MUST 使用在用模块，不得再以已下线的 `/categories` 举例。

#### Scenario: 前端调用带动词后缀

- WHEN 前端 api-client 调用版面列表
- THEN 请求 MUST 为 `POST /spaces/list`，而非 `GET /spaces/`

