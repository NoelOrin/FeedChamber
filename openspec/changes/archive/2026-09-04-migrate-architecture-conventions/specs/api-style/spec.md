## Purpose

定义 OrinForum 后端 HTTP API 的统一 RPC 风格约定，确保所有业务接口（含前端、openapi.json、新增模块）遵循一致的动词后缀 URL、请求体与响应结构。

## ADDED Requirements

### Requirement: RPC 风格动词后缀 URL

所有业务 HTTP 接口 MUST 使用 `POST` 方法，并以动词后缀表达"资源 + 动作"，禁止 RESTful 风格。仅 `/health` 等纯系统探活接口允许 `GET`。

URL 路径段 MUST 表达资源与动作，统一动作词包括：`/list`（列表/分页查询）、`/create`（创建）、`/{id}/update`（更新）、`/{id}/delete`（删除）、`/{id}/status`（状态变更）、`/{id}/<custom>`（其他业务动作，如 `/pin`、`/lock`、`/publish`）。

#### Scenario: 列表接口使用 POST + /list 后缀

- WHEN 客户端请求论坛分类列表
- THEN 系统 MUST 接受 `POST /api/v1/categories/list`，且允许请求体为空对象

#### Scenario: 禁止 RESTful 方法用于业务接口

- WHEN 对业务资源执行增删改查
- THEN 系统 MUST NOT 暴露 `GET /categories`、`PUT /categories/{id}`、`PATCH /categories/{id}` 或 `DELETE /categories/{id}`

#### Scenario: 查询参数不得承载过滤条件

- WHEN 客户端按状态与分页筛选举报列表
- THEN 系统 MUST 接受 `POST /api/v1/reports/list`，过滤条件与分页（`page`/`pageSize`）通过 request body 传递，而非 query string

### Requirement: 统一 JSON 响应结构

所有接口响应 MUST 使用固定结构 `{ "code": 0, "msg": "success", "data": ... }`。列表分页接口 MUST 额外通过 `pkg.SuccessPaginated` 返回 `total`。

错误响应 MUST 通过 `pkg.HandleError` 返回，错误码分段为：`1xxx` 认证、`2xxx` 参数、`3xxx` 资源、`5xxx` 服务端；`errToHTTPStatus` MUST 将业务码映射为合适的 HTTP status。

#### Scenario: 成功列表响应携带总数

- WHEN 分类列表请求成功
- THEN 系统 MUST 返回 `code: 0`、`msg: "success"`，且 `data` 含列表与 `total` 字段

#### Scenario: 业务错误映射为结构化响应

- WHEN 业务层返回 `*pkg.AppError`
- THEN 系统 MUST 经 `pkg.HandleError` 返回 `{ code, msg, data: null }` 且 HTTP status 与错误段匹配

### Requirement: 前端 service 请求约定

前端 `api-client` 的所有请求 MUST 默认 `method: 'POST'`，URL MUST 带显式动词后缀（如 `/categories/list`），不得省略动作词。

#### Scenario: 前端调用带动词后缀

- WHEN 前端 api-client 调用分类列表
- THEN 请求 MUST 为 `POST /categories/list`，而非 `GET /categories/`
