# Middleware 层

身份确定之后的门控与请求基础设施：限流、超时、请求体上限、维护模式、trace、请求日志。

**认证本身在 `internal/gateway`**（会话 sid / API Key 解析），本包不再持有任何 JWT 解析代码。

## 请求链路（实测挂载点）

| 位置 | 顺序 | 定义处 |
|------|------|--------|
| app 级（所有请求） | `CORS → Trace → Logger`；`METRICS_ENABLED=true` 时再加 `Prometheus` | `server/iris.go:247-253` |
| `/api/v1` party 级 | `PathBodyLimit(1MB，/upload 10MB) → RequestTimeout(30s) → gw.Preflight() → MaintenanceGate` | `internal/router/router.go` 的 `Register`（v1.Use 段） |
| public 路由 | 仅上述 party 级（登录/注册类再按路由加 `RateLimit`） | `routes_public.go` 的 `publicRoutes` |
| 公开但可登录（`/tags` `/spaces` `/posts` `/announcements` `/user/{id}/profile`） | party 级 + 按路由 `AuthGate(deps, key)` 或 `SpaceAuthGate(deps)` | `routes_public.go` 的 `publicOptionalRoutes`、`routes_content.go` 的 `postRoutes` |
| `protected` party | 一条 `gw.Enforce()`：必需凭证（会话 sid 或 API Key）→ banCheck（被封返回 404）→ 权限 → 安全风控 → 会话跟踪。tracing 与全局 IP 限流不在此段——它们属于 v1 层的 `gw.Preflight()`，每请求恰好一次 | `router.go` 的 `Register`（protected 段）、`internal/gateway/gateway.go` 的 `Enforce()` / `Preflight()` |

> `Trace` MUST 排在 `Logger` 之前：Iris 先跑 app 级中间件再跑 per-party 中间件，若 trace 只在网关的 `Tracing` 里建立，请求完成日志拿不到 `trace_id`。`Trace` 把 trace 写入 `request.Context()`，`Logger` 再把请求级 logger 写入同一 context，Service/Repository 层用 `logger.FromContext(ctx)` 即可取到带 trace 的实例。
>
> 指标不在这条链上：`/metrics` 由 `METRICS_PORT`(9091) 独立 listener 暴露，不经 nginx（见 `internal/metrics/AGENTS.md`）。

## 本包中间件

| 中间件 | 文件 | 说明 |
|--------|------|------|
| `CORS()` | middleware.go | 跨域放行，OPTIONS 直接返回 204 |
| `Recover()` | recovery.go | 捕获下游 panic → `500 {code:5001}`；堆栈只进日志（`panic`/`method`/`path`/`stack`），累加 `orinforum_http_panics_total`。MUST 排在 `Logger` 之后，否则请求日志不会写出；响应已开写时只 `StopExecution` 不伪造信封 |
| `Trace(enabled)` | trace.go | 确定/沿用 trace（`traceparent` > `X-Request-ID` 32hex > 新生成），写入 request context 并回写 `X-Request-ID`/`traceparent`；`RequestTrace(ctx)` 读取。`enabled=false` 时仍生成 trace 供日志关联，只是不解析/回写外部头 |
| `Logger()` | middleware.go | 请求日志：`trace_id`/`span_id`/`method`/`path`/`status`/`latency_ms`/`user_uuid`/`ip`，路由匹配后追加 `route`；原 `request_id` 字段已并入 `trace_id` |
| `Prometheus()` | prometheus.go | HTTP 指标；把路径中的 ID 段折叠成模板，避免高基数标签 |
| `PathBodyLimit(default, overrides)` | middleware.go | 超限返回 413 + JSON 信封（拦截 `MaxBytesReader` 错误，Iris 默认会写成 400）。无覆盖表版本 `BodyLimit(n)` 已于 2026-10-27 删除——唯一调用方是 `PathBodyLimit` |
| `RequestTimeout(d)` | request_timeout.go | 软超时，只设 deadline 不强杀；`*/stream`（SSE）自动跳过 |
| `RateLimit(limiter, name, limit, window)` | ratelimit.go | 命名端点的每 IP 预算，用于 register 5/min、login 10/min、mfa_verify 20/min、pm_message 60/min 等 12 处。与 `internal/gateway` 的全局每 IP 上限**共用 `internal/pkg/ratelimit.Limiter.Check`**（同一算法、同一指标与日志发出点），差别只在策略来源：这里在路由注册处声明，那里由 `rate_limit_enabled` 开关；两者的计数器键与配额互相独立 |
| `MaintenanceGate(settings)` | maintenance.go | `site_maintenance=true` 时拦截非管理员的写类 RPC，返回 `503 {"code":5005,...}`；挂在 v1 层 `gw.Preflight()` 之后以便识别角色 |
| `AuthGate(deps, settingKey)` | auth.go | 设置项为 `"true"` 时要求已登录 |
| `SpaceAuthGate(deps)` | auth.go | 按版面 `AuthRequired` 决定是否要求登录 |
| `RequirePermission(deps, code)` | auth.go | 路由级权限码；实际只用于 `/permission/*`(8) 与 `/admin/backups`、`/admin/webhooks`，其余受保护路由靠网关的 path→permission 映射兜住 |

## 依赖注入

`Deps`（deps.go）在 `server/iris.go:330` 构造一次并传给 `router.Register`：`Settings` / `Space` / `Perms` / `Ban` / `Limiter` 五个接口。
其中 `Perms`、`Ban` 会被再交给 gateway 组装（`router.go` 的 `Register` 收尾处把 `deps` 交给 `gateway.Deps` 组装），所以权限缓存与禁言判定在全链路只有一个实例，不存在两套。

早期通过包级 setter（`SetPermissionChecker`）注入的方式已废弃并删除。

## 已删除，文档中不要再引用

- `JWTAuth` / `OptionalJWTAuth` / `BanCheck` / `RequireRole` / `RequireOwnerOrPermission` 以及 `Deps.Blacklist` + `TokenRevoker`：会话鉴权迁移后生产调用数为 0，2026-10-14 删除。
- 本包 `client_ip.go`：与 `internal/gateway` 有一份 22 行逐字相同的实现，现统一为 `pkg.ClientIP(ctx)`（`internal/pkg/clientip.go`）。
- `auth_gate.go` 这个文件从未存在：`AuthGate`/`SpaceAuthGate` 一直在 `auth.go`。

## 重要设计

- **所有权检查在 Service 层**（R-06b）：`PostService`/`ContentService` 注入 `PermissionService`，在 `Update/Delete` 内部判断 `作者 || 有权限`，不在中间件里做。
- **封号与禁言双层拦截在网关**：`role=ban` 封号与会话吊销由 `SessionStore` 承担；`Banned` 表禁言由网关 `banCheck` 拦截写操作，故意返回 404 而非 403。
- **AuthGate 取代硬编码 public/protected 判断**：设置值经 `auth.go` 内 30s TTL 进程级缓存（key 为 `gateCache`），KV 故障时允许 2×TTL 的旧值降级，设置项缺失视为空值（fail-open），其它 KV 错误 fail-closed 返回 `INTERNAL_ERROR`。

> 引用本包以外的位置时用符号名而非行号——行号在任何一次拆分或插入后都会腐化（本轮 router 拆分后即出现大批失效行号）。
