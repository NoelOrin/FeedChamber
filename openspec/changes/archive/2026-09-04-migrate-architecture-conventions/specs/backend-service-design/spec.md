## Purpose

定义 OrinForum 后端关键服务设计约束：Repository 双实现、统一响应、权限系统与路由组织方式，确保跨模块实现一致。

## ADDED Requirements

### Requirement: Repository 接口与双实现

每个数据资源 MUST 在 `repository/` 中定义接口；工厂函数 MUST 根据 `store.SQL() != nil` 返回 SQL（GORM）实现或 Mongo 实现。Service 层 MUST 仅依赖接口，不感知底层存储。

#### Scenario: 按存储后端选择实现

- WHEN 应用使用 SQL 后端
- THEN `NewXxxRepository(store)` MUST 返回 `sqlXxxRepo` 实现；使用 Mongo 后端时 MUST 返回 `mongoXxxRepo` 实现

#### Scenario: Service 不感知存储

- WHEN Service 调用 repository
- THEN 它 MUST 仅通过接口方法访问数据，编译期不依赖具体 SQL/Mongo 类型

### Requirement: 统一响应与错误映射

Handler MUST 使用 `pkg.Success` / `pkg.SuccessPaginated` 返回成功，使用 `pkg.HandleError` 返回失败，响应体固定为 `{ code, msg, data }`，错误码按段划分并自动映射 HTTP status。

#### Scenario: 分页响应结构一致

- WHEN Handler 返回列表
- THEN MUST 调用 `pkg.SuccessPaginated(ctx, data, total)`，响应含 `data` 与总数

### Requirement: 权限系统与动态鉴权

系统 MUST 在启动时 seed 约 20 个权限码与默认角色-权限映射（admin/moderator/user/ban）。`PermissionService` MUST 通过 `sync.RWMutex` 维护 `role → permission codes` 内存缓存，启动时从 DB 加载，变更经 KV Pub/Sub 多实例同步。路由级鉴权 MUST 通过 `RequirePermission(code)` 实现；资源级"作者 OR 有权限"校验 MUST 在 Service 层实现（R-06b）。

#### Scenario: 路由级权限拦截

- WHEN 受保护路由被无权限用户访问
- THEN `RequirePermission(code)` MUST 在路由层拒绝请求

#### Scenario: 多实例权限缓存同步

- WHEN 某实例更新角色权限映射
- THEN MUST 经 KV Pub/Sub 通知其他实例刷新内存缓存

### Requirement: 路由按版本与公开/受保护分组

所有路由 MUST 注册于 `router/router.go`，统一 `api/v1` 前缀。public 路由直接挂载；protected 路由 MUST 统一应用 `JWTAuth() + BanCheck()`，再按资源模块 `PartyFunc` 细分，权限粒度直接标注在路由上。

#### Scenario: 受保护路由默认鉴权

- WHEN 注册 protected 路由
- THEN MUST 应用 `JWTAuth() + BanCheck()` 中间件
