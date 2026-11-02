# api-style Specification

## Purpose
定义 Feed Chamber 后端 HTTP API 的统一约定：HTTP 方法按「有无副作用」选择（写操作 `POST`、只读查询 MAY `GET`），URL 遵循 RPC 风格而非 RESTful，所有出口共用同一响应信封与错误码命名空间。
## Requirements
### Requirement: HTTP 方法选择与 RPC 风格 URL

接口 HTTP 方法的唯一判据是**有无副作用**：写操作 MUST 使用 `POST`；只读查询 MAY 使用 `GET` 或 `POST`。

写操作（创建、更新、删除、状态变更，以及任何改变服务端状态的请求）MUST 使用 `POST`，URL MUST 以动词后缀表达动作：`/list`（列表 / 分页查询）、`/create`（创建）、`/{id}/update`（更新）、`/{id}/delete`（删除）、`/{id}/status`（状态变更）、`/{id}/<custom>`（其他业务动作，如 `/pin`、`/lock`、`/publish`）。

只读查询 MUST 无副作用、幂等、可安全重试，并且：

- 单资源读取与轻量状态查询 SHOULD 使用 `GET`，URL 用资源标识或既有业务名词（如 `GET /posts/{id}`、`GET /user/{id}/profile`、`GET /checkin/status`、`GET /sessions`）；
- 列表 / 分页 / 多条件筛选 SHOULD 使用 `POST` + `/list`，参数走 request body；
- 同一资源的读取 MUST 只暴露一种方法，MUST NOT 同时提供 `GET` 与 `POST` 两个读入口。

`PUT` / `PATCH` / `DELETE` MUST NOT 用于业务接口；URL MUST NOT 采用 RESTful 的资源层级语义（层级嵌套不改变方法含义）。

`GET` 允许用 query string 承载过滤与分页参数，分页字段名 MUST 仍为 `page` / `pageSize`；`POST` 接口 MUST NOT 用 query string 承载业务参数。

系统 / 协议 / 文件类端点 MUST 使用 `GET`，不受 RPC 风格约束：`/health`；`/sitemap.xml`、`/sitemap-posts.xml`、`/sitemap-spaces.xml`、`/sitemap-tags.xml`、`/robots.txt`、`/latest.rss`；OAuth 2.0 / OIDC 协议端点（`/.well-known/*`、authorize、callback、userinfo）；文件字节读取（`/upload/file/{key}` 流式、`/upload/{id}` 302）；认证流程端点（如 `GET /oauth/{provider}/auth` 返回授权 URL）。

爬虫端点的分页参数 `?page=` MUST 被视为上述「`GET` 允许 query 承载过滤与分页」的既有例外，MUST NOT 因「`POST` 参数走 body」的规则被改写成 `POST`——爬虫不会发起 `POST`。

**MCP 端点（`/mcp`）是协议端点**：MCP 客户端按 JSON-RPC 2.0 与服务协商，请求体 MUST 为 JSON-RPC 消息而非论坛的业务参数，响应 MUST 为 JSON-RPC 结果而非论坛信封。因此 `/mcp` MUST 使用 `POST` 承载请求（JSON-RPC 请求体不适用「写操作动词后缀」约定），MUST NOT 被改写成带动词后缀的 RPC 风格 URL，MUST NOT 套用 `pkg.Success` 信封。该例外 MUST NOT 被推广到任何业务接口。

示例 MUST 使用在用的模块。`Category` 模块已下线、由 `Space`（版面树）承接，规范与文档中的示例不得再引用 `/categories`。

#### Scenario: 写接口使用 POST + 动词后缀

- WHEN 客户端创建版面
- THEN 系统 MUST 接受 `POST /api/v1/spaces/create`

#### Scenario: 只读单资源允许 GET

- WHEN 客户端读取帖子详情
- THEN 系统 MUST 接受 `GET /api/v1/posts/{id}`，且 MUST NOT 同时提供同一读取路径的 `POST` 版本

#### Scenario: 列表接口使用 POST + /list 后缀

- WHEN 客户端请求版面列表
- THEN 系统 MUST 接受 `POST /api/v1/spaces/list`，且允许请求体为空对象

#### Scenario: 查询参数不得承载过滤条件（POST）

- WHEN 客户端按状态与分页筛选举报列表
- THEN 系统 MUST 接受 `POST /api/v1/reports/list`，过滤条件与分页（`page`/`pageSize`）通过 request body 传递，而非 query string

#### Scenario: GET 允许 query 承载过滤

- WHEN 客户端查询某月签到历史
- THEN 系统 MUST 接受 `GET /api/v1/checkin/history?year=2026&month=10`

#### Scenario: GET 不得有副作用

- WHEN 客户端执行任何改变服务端状态的写操作（如签到、点赞、删除）
- THEN 该接口 MUST 为 `POST`，MUST NOT 以 `GET` 暴露

#### Scenario: 协议与系统端点保持 GET

- WHEN 客户端请求 OIDC Discovery / JWKS / authorize / userinfo、sitemap、robots、RSS，或上传文件的字节流
- THEN 该端点 MUST 为 `GET`，MUST NOT 改为 `POST`

#### Scenario: 爬虫分页端点保持 GET

- WHEN 爬虫请求 `/sitemap-posts.xml?page=2`
- THEN 该端点 MUST 为 `GET` 且 MUST 接受 query 承载分片页码，MUST NOT 要求 `POST` 或请求体

#### Scenario: MCP 端点保持 JSON-RPC 形状

- WHEN 一个 MCP 客户端向 `/mcp` 发送 JSON-RPC 请求
- THEN 该端点 MUST 接受 `POST` 且请求体 MUST 按 JSON-RPC 解析
- AND 响应 MUST 为 JSON-RPC 结果，MUST NOT 包装为 `{code,msg,data}` 信封
- AND 该端点 MUST NOT 被改写为带动词后缀的业务 URL

### Requirement: 统一 JSON 响应结构

所有 HTTP 出口 MUST 只通过 `internal/pkg` 的响应封装写响应：`pkg.Success(ctx, data)`、`pkg.SuccessPaginated(ctx, items, total)`、`pkg.Fail(ctx, code, msg?)`、`pkg.FailStatus(ctx, status, code, msg?)`、`pkg.HandleError(ctx, err)`。这覆盖 handler、`internal/gateway` 的全部中间件与自注册端点、以及 maintenance / request-timeout / recovery 中间件。MUST NOT 用 `ctx.JSON` / `ctx.StopWithJSON` / `ctx.WriteString` 自行拼装信封。

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

`api-client` 的写操作 MUST 使用 `method: 'POST'`，URL MUST 带显式动词后缀（如 `/spaces/list`、`/spaces/create`）。只读查询 MAY 使用 `GET`（`client.ts` 提供 `get`），但 MUST 与后端实际方法一致；同一资源的读取方法 MUST 在前端只有一处定义。

示例 MUST 使用在用模块，不得再以已下线的 `/categories` 举例。

#### Scenario: 前端写操作使用 POST + 动词后缀

- WHEN 前端 api-client 创建版面
- THEN 请求 MUST 为 `POST /spaces/create`

#### Scenario: 前端只读查询可使用 GET

- WHEN 前端 api-client 读取帖子详情
- THEN 请求 MUST 为 `GET /posts/{id}`（与后端方法一致），MUST NOT 同时保留 POST 版本

