# frontend-packages Specification

## Purpose
定义 Feed Chamber 前端共享包的契约与职责边界，确保 Web 与 Dashboard 共用一致的类型、客户端与工具。
## Requirements
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

### Requirement: sse-client 框架无关契约

`@feedchamber/sse-client` MUST 提供框架无关的 `SSEClient` 类，支持 Bearer token 鉴权、401 自动刷新 token 重连、指数退避（3s→30s，最多 5 次）、ReadableStream 解析与 AbortController 取消。Web 端 MUST 通过 SolidJS adapter hook 订阅。

#### Scenario: 断线指数退避重连

- WHEN SSE 连接断开
- THEN `SSEClient` MUST 以 3s→30s 指数退避自动重连，最多 5 次

### Requirement: shared-utils 工具契约

`@feedchamber/shared-utils` MUST 提供跨前端共享工具：`formatSmartDate()`（相对时间）、`formatTime()`（`YY-MM-DD HH:mm:ss`）、`cx()` / `sortCx()`（Tailwind class 合并，基于 clsx + tailwind-merge）。

#### Scenario: 相对时间格式化

- WHEN 传入 3 分钟前的时间戳
- THEN `formatSmartDate` MUST 返回"3 分钟前"

