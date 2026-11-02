# frontend-packages Specification (delta)

## MODIFIED Requirements

### Requirement: api-client 共享包契约

`@feedchamber/api-client` MUST 为 Web 与 Dashboard 共同依赖，提供：`client.ts`（authFetch 封装，自动携带 Bearer token，401 自动刷新并重试）、`auth.ts`（token 存取与 `isAuthenticated`）、`types.ts`（所有 API 响应类型）、`constants.ts`（ErrorCode / PermissionCode / SettingKey / RoleName / PostStatus / API_BASE_URL 等类型化常量）、`services/*.ts`（各模块 API service：写操作 `method: 'POST'` 且 URL 带动词后缀，只读查询 MAY 使用 `GET` 且 MUST 与后端方法一致）。

#### Scenario: 401 自动刷新重试

- WHEN api-client 收到 401 且存在 refresh token
- THEN `authFetch` MUST 静默刷新 token 并重试原请求

#### Scenario: 只读查询使用 GET

- WHEN 页面需要读取帖子详情
- THEN api-client 的对应 service 方法 MUST 发出 `GET`，不得改成对同一路径的 `POST`

#### Scenario: 写操作使用 POST

- WHEN 页面执行创建、更新、删除或状态变更
- THEN api-client 的对应 service 方法 MUST 发出 `POST`，且 URL 带显式动词后缀
