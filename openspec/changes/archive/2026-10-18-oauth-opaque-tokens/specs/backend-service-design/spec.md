# backend-service-design Specification (delta)

## MODIFIED Requirements

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
