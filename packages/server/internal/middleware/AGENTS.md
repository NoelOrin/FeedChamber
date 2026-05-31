# Middleware 层

请求拦截层，负责认证、授权、安全检查。

## 中间件栈

请求流经顺序：`CORS → Logger → JWTAuth → BanCheck → RequirePermission(可选)`

## 中间件列表

| 中间件 | 说明 |
|--------|------|
| `CORS()` | 跨域放行，OPTIONS 直接返回 204 |
| `Logger()` | 请求日志 |
| `JWTAuth()` | 解析 Bearer token，设置 context: username, user_uuid, user_role, claims |
| `OptionalJWTAuth()` | 同 JWTAuth 但无 token 时不报错，用于公开但可选登录的端点 |
| `BanCheck()` | 检查 user_role 是否为 "ban"，是则返回 403 |
| `RequirePermission(code)` | 通过 permissionChecker 接口校验权限码 |
| `RequireOwnerOrPermission(key, code)` | 资源归属校验 或 权限码校验，满足其一即放行 |
| `RequireRole(roles...)` | 硬编码角色名校验 (旧模式，优先用 RequirePermission) |
| `PostAuthGate()` | 根据系统设置决定帖子列表/详情是否需要登录 |
| `CommentAuthGate()` | 根据系统设置决定评论列表是否需要登录 |

## permissionChecker 注入

`SetPermissionChecker(c permissionChecker)` 在 `server/iris.go` 启动时调用，将 PermissionService 注入 middleware。
PermissionService 实现了 `HasPermission(roleName, permCode string) bool` 接口。
