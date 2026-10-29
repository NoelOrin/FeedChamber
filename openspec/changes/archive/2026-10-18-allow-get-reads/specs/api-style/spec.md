# api-style Specification (delta)

## REMOVED Requirements

### Requirement: RPC 风格动词后缀 URL

移除原因：「所有业务 HTTP 接口 MUST 使用 `POST`」与代码现实（`router.go` 30 处 `GET`，其中 13 条为协议/爬虫/文件语义、17 条为只读查询）长期矛盾，且没有任何决策记录。该要求被下面的「HTTP 方法选择与 RPC 风格 URL」整体替代：方法判据由「一律 POST」改为「有无副作用」。

## ADDED Requirements

### Requirement: HTTP 方法选择与 RPC 风格 URL

接口 HTTP 方法的唯一判据是**有无副作用**：写操作 MUST 使用 `POST`；只读查询 MAY 使用 `GET` 或 `POST`。

写操作（创建、更新、删除、状态变更，以及任何改变服务端状态的请求）MUST 使用 `POST`，URL MUST 以动词后缀表达动作：`/list`（列表 / 分页查询）、`/create`（创建）、`/{id}/update`（更新）、`/{id}/delete`（删除）、`/{id}/status`（状态变更）、`/{id}/<custom>`（其他业务动作，如 `/pin`、`/lock`、`/publish`）。

只读查询 MUST 无副作用、幂等、可安全重试，并且：

- 单资源读取与轻量状态查询 SHOULD 使用 `GET`，URL 用资源标识或既有业务名词（如 `GET /posts/{id}`、`GET /user/{id}/profile`、`GET /checkin/status`、`GET /sessions`）；
- 列表 / 分页 / 多条件筛选 SHOULD 使用 `POST` + `/list`，参数走 request body；
- 同一资源的读取 MUST 只暴露一种方法，MUST NOT 同时提供 `GET` 与 `POST` 两个读入口。

`PUT` / `PATCH` / `DELETE` MUST NOT 用于业务接口；URL MUST NOT 采用 RESTful 的资源层级语义（层级嵌套不改变方法含义）。

`GET` 允许用 query string 承载过滤与分页参数，分页字段名 MUST 仍为 `page` / `pageSize`；`POST` 接口 MUST NOT 用 query string 承载业务参数。

系统 / 协议 / 文件类端点 MUST 使用 `GET`，不受 RPC 风格约束：`/health`；`/sitemap.xml`、`/sitemap-posts.xml`、`/robots.txt`、`/latest.rss`；OAuth 2.0 / OIDC 协议端点（`/.well-known/*`、authorize、callback、userinfo）；文件字节读取（`/upload/file/{key}` 流式、`/upload/{id}` 302）；认证流程端点（如 `GET /oauth/{provider}/auth` 返回授权 URL）。

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

## MODIFIED Requirements

### Requirement: 前端 service 请求约定

`api-client` 的写操作 MUST 使用 `method: 'POST'`，URL MUST 带显式动词后缀（如 `/spaces/list`、`/spaces/create`）。只读查询 MAY 使用 `GET`（`client.ts` 提供 `get`），但 MUST 与后端实际方法一致；同一资源的读取方法 MUST 在前端只有一处定义。

示例 MUST 使用在用模块，不得再以已下线的 `/categories` 举例。

#### Scenario: 前端写操作使用 POST + 动词后缀

- WHEN 前端 api-client 创建版面
- THEN 请求 MUST 为 `POST /spaces/create`

#### Scenario: 前端只读查询可使用 GET

- WHEN 前端 api-client 读取帖子详情
- THEN 请求 MUST 为 `GET /posts/{id}`（与后端方法一致），MUST NOT 同时保留 POST 版本
