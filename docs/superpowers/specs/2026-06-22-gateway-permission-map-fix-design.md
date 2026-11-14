# Gateway Permission Map 修复设计

## 问题

Gateway `PermissionCheckerMiddleware.initRouteMap()` 的路径映射与 `router.go` 实际注册路由不一致：
1. 用户/角色/权限/标签/空间端点路径用了 `/admin/` 前缀，实际路由无此前缀
2. 参数名不匹配（`{spaceId}` vs `{space_id}`、`{id}` vs `{postId}`）
3. 缺少 `pin`/`lock`/`batch`/`banned status`/`activity` 映射
4. `categories` 映射无对应路由（死代码）
5. router.go 多处端点仅有 JWT 认证、缺少权限检查

## 方案：Gateway 统一权限，移除 router 级 RequirePermission

### 核心变更

1. **修正 `permission.go` 映射路径**：对齐 `router.go` 实际路由
2. **补全缺失映射**：`pin`/`lock`/`batch`/`banned status`/`activity`
3. **移除 `categories` 死映射**
4. **`router.go` 移除所有 `RequirePermission` 调用**：Gateway Handler 中的 PermissionCheckerMiddleware 统一执行
5. **`RequireSpacePermission` 保留**：Gateway 无空间级权限概念，此中间件仍在 router 层

### 路径修正表

| 原 Gateway 路径 | 修正后 | 权限码 |
|---|---|---|
| `/api/v1/admin/users/*` | `/api/v1/user/*` | 各 user 权限 |
| `/api/v1/admin/roles/*` | `/api/v1/role/*` | 各 role 权限 |
| `/api/v1/admin/permissions/*` | `/api/v1/permission/*` | 各 role 权限 |
| `/api/v1/admin/tags/*` | `/api/v1/tags/*` | 各 tag 权限 |
| `/api/v1/admin/spaces/*` | `/api/v1/spaces/*` | 各 space 权限 |
| `/api/v1/admin/categories/*` | 删除 | N/A |
| `{spaceId}` → `{space_id}` | 统一 snake_case | N/A |

### 新增映射

| 路径 | 权限码 |
|---|---|
| `/api/v1/posts/{id}/pin` | `post:pin` |
| `/api/v1/posts/{id}/lock` | `post:lock` |
| `/api/v1/posts/batch` | `post:read` |
| `/api/v1/user/{id}/banned/status` | `user:update` |
| `/api/v1/user/{id}/banned/overview` | `user:update` |
| `/api/v1/user/{id}/activity` | `user:activity:read` |
| `/api/v1/user/{id}/space/{space_id}/banned/status` | `user:update` |

## 影响范围

- `pkg/gateway/permission.go` — 路径映射修正+补全
- `internal/router/router.go` — 移除 `RequirePermission`（保留 `RequireSpacePermission`）
