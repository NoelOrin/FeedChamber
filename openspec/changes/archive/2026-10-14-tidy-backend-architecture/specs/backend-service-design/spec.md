# backend-service-design Specification (delta)

## MODIFIED Requirements

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

路由级鉴权 MUST 通过权限码判定：`pkg/gateway` 的 path→permission 映射覆盖绝大多数受保护路由，`middleware.RequirePermission(code)` 仅用于映射表不便表达的 `/permission/*`、`/admin/backups`、`/admin/webhooks`。资源级"作者 OR 有权限"校验 MUST 在 Service 层实现（R-06b）。

#### Scenario: 路由级权限拦截

- WHEN 受保护路由被无权限用户访问
- THEN 鉴权 MUST 在路由/网关层拒绝请求，MUST NOT 把判定下推到 handler

#### Scenario: 权限数量与代码一致

- WHEN 新增权限码
- THEN MUST 同时更新 seed 列表与本规范中的计数，避免文档停在历史数值（此前长期写作「约 20 个」而实测为 53）

#### Scenario: 多实例权限缓存同步

- WHEN 某实例更新角色权限映射
- THEN MUST 经 KV Pub/Sub 通知其他实例刷新内存缓存
