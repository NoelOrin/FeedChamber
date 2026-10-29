# backend-service-design Specification

## Purpose
定义 OrinForum 后端关键服务设计约束：Repository 的 SQL 单实现、统一响应与错误映射、权限系统与路由组织方式，确保跨模块实现一致。
## Requirements
### Requirement: 统一响应与错误映射

Handler 与网关 MUST 统一使用 `internal/pkg` 的响应封装（`Success` / `SuccessPaginated` / `Fail` / `FailStatus` / `HandleError`），响应体固定为 `{ code, msg, data }`，业务码按段划分并由 `errToHTTPStatus` 唯一映射到 HTTP status。细则见 `api-style` 的「统一 JSON 响应结构」，本要求 MUST NOT 与它产生第二套约定。

#### Scenario: 分页响应结构一致

- WHEN Handler 返回列表
- THEN MUST 调用 `pkg.SuccessPaginated(ctx, items, total)`，响应的 `data` 为 `{ items, total }`

#### Scenario: 非 AppError 的兜底

- WHEN service 返回的错误不是 `*pkg.AppError`
- THEN `pkg.HandleError` MUST 压平为 `500 / INTERNAL_ERROR`，MUST NOT 把底层错误文本透传给客户端

### Requirement: 权限系统与动态鉴权

系统 MUST 在启动时 seed 53 个平台级权限码（形如 `post:create`、`role:manage`、`reconcile:counters`）与默认角色-权限映射（admin / moderator / user / pending_user / ban），另有版面级权限 `SpaceRolePermission`（沿版面层级继承）。`PermissionService` MUST 通过 `sync.RWMutex` 维护 `role → permission codes` 内存缓存，启动时从 DB 加载，变更经 KV Pub/Sub 多实例同步。

路由级鉴权 MUST 通过权限码判定：`internal/gateway` 的 path→permission 映射覆盖绝大多数受保护路由，`middleware.RequirePermission(code)` 仅用于映射表不便表达的 `/permission/*`、`/admin/backups`、`/admin/webhooks`。资源级"作者 OR 有权限"校验 MUST 在 Service 层实现（R-06b）。

#### Scenario: 路由级权限拦截

- WHEN 受保护路由被无权限用户访问
- THEN 鉴权 MUST 在路由/网关层拒绝请求，MUST NOT 把判定下推到 handler

#### Scenario: 权限数量与代码一致

- WHEN 新增权限码
- THEN MUST 同时更新 seed 列表与本规范中的计数，避免文档停在历史数值（此前长期写作「约 20 个」而实测为 53）

#### Scenario: 多实例权限缓存同步

- WHEN 某实例更新角色权限映射
- THEN MUST 经 KV Pub/Sub 通知其他实例刷新内存缓存

### Requirement: 路由按版本与公开/受保护分组

所有路由 MUST 注册于 `router/router.go`，统一 `api/v1` 前缀。public 路由直接挂载；protected 路由 MUST 统一挂在网关门控 `protected.Use(gw.Enforce())` 之下（必需凭证 → banCheck → 权限 → 安全风控 → 会话跟踪），再按资源模块 `PartyFunc` 细分，权限粒度直接标注在路由上。

凭证解析 MUST 只发生在网关：用户凭证是服务端会话 sid，机器凭证是 API Key 或 opaque OAuth2/OIDC access token（`of_at_` + 32B，库中只存 SHA-256 hex）。路由层 MUST NOT 再拼装凭证解析或禁言中间件——`JWTAuth` / `OptionalJWTAuth` / `BanCheck` 已于 2026-10-14 会话鉴权迁移中删除，MUST NOT 在任何新路由上引用。本条此前仍写 `JWTAuth() + BanCheck()`，是迁移遗留的 spec 漂移，本轮校正为现状。

#### Scenario: 受保护路由默认鉴权

- WHEN 注册 protected 路由
- THEN MUST 挂在 `protected.Use(gw.Enforce())` 之下，由网关统一完成凭证校验、banCheck、权限与安全风控，MUST NOT 逐个路由手工拼装鉴权或禁言中间件

#### Scenario: 封禁用户按资源不存在处理

- WHEN 已封禁（`role="ban"`）或已被禁言的用户访问受保护端点
- THEN 网关 MUST 返回 `404`（信封携带 `USER_BANNED`）而不是 `403`，使探测者无法区分「被封禁」与「资源不存在」

#### Scenario: 未认证请求不被放过

- WHEN 受保护端点收到没有任何有效凭证的请求
- THEN 网关 MUST 以 `401 / code 1001` 拒绝，MUST NOT 把判定下推到 handler

### Requirement: Repository 接口与 SQL 实现

每个数据资源 MUST 在 `repository/` 中定义接口；工厂函数 MUST 返回基于 `store.SQL()` 的 SQL（GORM）实现，不再存在 Mongo 分支。Service 层 MUST 仅依赖接口，不感知底层存储。

#### Scenario: 工厂返回 SQL 实现

- WHEN 应用启动并构造 Repository
- THEN `NewXxxRepository(store)` MUST 返回 `sqlXxxRepo{db: store.SQL()}` 实现

#### Scenario: Service 不感知存储

- WHEN Service 调用 Repository
- THEN 它 MUST 仅通过接口方法访问数据，编译期不依赖具体实现类型

### Requirement: 数据库后端取值域

`db.Store` MUST 仅支持 SQL 后端，`DB_TYPE` MUST 限于 `sqlite` / `mysql` / `postgres`；`Store` 接口 MUST 暴露 `SQL()` 供 Repository 获取 GORM 句柄，且 MUST NOT 暴露 `Mongo()`。

#### Scenario: 拒绝已下线的后端

- WHEN 配置 `DB_TYPE=mongo`
- THEN 应用启动 MUST 失败并返回「unknown database backend」错误
