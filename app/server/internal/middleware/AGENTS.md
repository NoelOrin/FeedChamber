# Middleware 层

身份确定之后的门控与请求基础设施：限流、超时、请求体上限、维护模式、trace、请求日志。

**认证本身在 `internal/gateway`**（会话 sid / API Key 解析），本包不再持有任何 JWT 解析代码。

## 请求链路（实测挂载点）

| 位置 | 顺序 | 定义处 |
|------|------|--------|
| app 级（所有请求） | `CORS → Trace → Logger`；`METRICS_ENABLED=true` 时再加 `Prometheus` | `server/iris.go:247-253` |
| `/api/v1` party 级 | `PathBodyLimit(1MB，/upload 10MB) → RequestTimeout(30s) → gw.Preflight() → MaintenanceGate` | `internal/router/router.go` 的 `Register`（v1.Use 段） |
| public 路由 | 仅上述 party 级（登录/注册类再按路由加 `RateLimit`） | `routes_public.go` 的 `publicRoutes` |
| 公开但可登录（`/tags` `/spaces` `/posts` `/announcements` `/user/{id}/profile`） | party 级 + 按路由 `AuthGate(deps, key)`；版面 `login`/`level` 由 `service/visibility.go` 判定，**不经中间件** | `routes_public.go` 的 `publicOptionalRoutes`、`routes_content.go` 的 `postRoutes` |
| `protected` party | 一条 `gw.Enforce()`：必需凭证（会话 sid 或 API Key）→ banCheck（被封返回 404）→ 权限 → 安全风控 → 会话跟踪。tracing 与全局 IP 限流不在此段——它们属于 v1 层的 `gw.Preflight()`，每请求恰好一次 | `router.go` 的 `Register`（protected 段）、`internal/gateway/gateway.go` 的 `Enforce()` / `Preflight()` |

> `Trace` MUST 排在 `Logger` 之前：Iris 先跑 app 级中间件再跑 per-party 中间件，若 trace 只在网关的 `Tracing` 里建立，请求完成日志拿不到 `trace_id`。`Trace` 把 trace 写入 `request.Context()`，`Logger` 再把请求级 logger 写入同一 context，Service/Repository 层用 `logger.FromContext(ctx)` 即可取到带 trace 的实例。
>
> 指标不在这条链上：`/metrics` 由 `METRICS_PORT`(9091) 独立 listener 暴露，不经 nginx（见 `internal/metrics/AGENTS.md`）。

## 本包中间件

| 中间件 | 文件 | 说明 |
|--------|------|------|
| `CORS()` | middleware.go | 跨域放行，OPTIONS 直接返回 204 |
| `Recover()` | recovery.go | 捕获下游 panic → `500 {code:5001}`；堆栈只进日志（`panic`/`method`/`path`/`stack`），累加 `feedchamber_http_panics_total`。MUST 排在 `Logger` 之后，否则请求日志不会写出；响应已开写时只 `StopExecution` 不伪造信封 |
| `RegisterNotFoundHandlers(app)` | notfound.go | 把 Iris 默认的纯文本 `404 Not Found` / `405 Method Not Allowed` 补成 `{code,msg,data}` 信封。配合 `iris.WithFireMethodNotAllowed`（在 `server/iris.go` 配置）让「方法用错」返回 405 而不是 404。前端按信封解析，裸文本会静默失败 |
| `Trace(enabled)` | trace.go | 确定/沿用 trace（`traceparent` > `X-Request-ID` 32hex > 新生成），写入 request context 并回写 `X-Request-ID`/`traceparent`；`RequestTrace(ctx)` 读取。`enabled=false` 时仍生成 trace 供日志关联，只是不解析/回写外部头 |
| `Logger()` | middleware.go | 请求日志：`trace_id`/`span_id`/`method`/`path`/`status`/`latency_ms`/`user_uuid`/`ip`，路由匹配后追加 `route`；原 `request_id` 字段已并入 `trace_id` |
| `Prometheus()` | prometheus.go | HTTP 指标；把路径中的 ID 段折叠成模板，避免高基数标签 |
| `PathBodyLimit(default, overrides)` | middleware.go | 超限返回 413 + JSON 信封。**无请求体的方法（GET/HEAD/OPTIONS）直接放行**，不包装 body、也不替换 ResponseWriter——`/api/v1/upload/file/{key}` 的字节流因此完全不被干扰。正常响应直接透传（流式），只有判定超限时才把错误响应整份换成 413 信封。无覆盖表版本 `BodyLimit(n)` 已于 2026-10-27 删除——唯一调用方是 `PathBodyLimit` |
| `RequestTimeout(d)` | request_timeout.go | 软超时，只设 deadline 不强杀；`*/stream`（SSE）自动跳过 |
| `RateLimit(limiter, name, limit, window)` | ratelimit.go | 命名端点的每 IP 预算，用于 register 5/min、login 10/min、mfa_verify 20/min、pm_message 60/min 等 12 处。与 `internal/gateway` 的全局每 IP 上限**共用 `internal/pkg/ratelimit.Limiter.Check`**（同一算法、同一指标与日志发出点），差别只在策略来源：这里在路由注册处声明，那里由 `rate_limit_enabled` 开关；两者的计数器键与配额互相独立 |
| `MaintenanceGate(settings)` | maintenance.go | `site_maintenance=true` 时拦截非管理员的写类 RPC，返回 `503 {"code":5005,...}`；挂在 v1 层 `gw.Preflight()` 之后以便识别角色。**放行读类**：只读 GET、读 RPC 白名单（`isMaintenanceReadRPC`），以及所有 `*/stream` SSE 长连接（POST 承载的读）。SSE 必须放行——否则维护期通知铃/实时流被 5005 静默掐断，`SSEClient` 只对 (0,5000) 段上报错误，用户看不到任何提示 |
| `RegisterNotFoundHandlers(app)` | notfound.go | 把 404/405 补成统一信封（Iris 默认写纯文本 "Not Found"）。**必须配合 `app.Configure(iris.WithFireMethodNotAllowed)`**，否则方法不匹配也走 404。业务主动 `pkg.FailStatus(404)` 不会被二次触发（有测试钉住） |
| `AuthGate(deps, settingKey)` | auth.go | 设置项为 `"true"` 时要求已登录 |
> `SpaceAuthGate` **已删除**（2026-10-28 起版面可见性由 `service/visibility.go` 的 `BuildVisibilityIndex` 谓词判定）。本包只剩 `AuthGate` 一个登录门。
| `RequirePermission(deps, code)` | auth.go | 路由级权限码，2026-11-11 实测约 30 处（权限管理、admin 子域、gate/trust/站点自定义/插件等）。**它排在网关 permission 中间件之后**：网关 `initRouteMap` 未映射即 403 返回，本中间件根本不会执行，所以新增 protected 路由 MUST 先登记 `initRouteMap` |

## 依赖注入

`Deps`（deps.go）在 `server/iris.go:330` 构造一次并传给 `router.Register`：`Settings` / `Space` / `Perms` / `Ban` / `Limiter` 五个接口。
其中 `Perms`、`Ban` 会被再交给 gateway 组装（`router.go` 的 `Register` 收尾处把 `deps` 交给 `gateway.Deps` 组装），所以权限缓存与禁言判定在全链路只有一个实例，不存在两套。

早期通过包级 setter（`SetPermissionChecker`）注入的方式已废弃并删除。

## 已删除，文档中不要再引用

- `JWTAuth` / `OptionalJWTAuth` / `BanCheck` / `RequireRole` / `RequireOwnerOrPermission` 以及 `Deps.Blacklist` + `TokenRevoker`：会话鉴权迁移后生产调用数为 0，2026-10-14 删除。
- 本包 `client_ip.go`：与 `internal/gateway` 有一份 22 行逐字相同的实现，现统一为 `pkg.ClientIP(ctx)`（`internal/pkg/clientip.go`）。
- `auth_gate.go` 这个文件从未存在：`AuthGate` 一直在 `auth.go`；`SpaceAuthGate` 已删除。

## 重要设计

- **所有权检查在 Service 层**（R-06b）：`PostService`/`ContentService` 注入 `PermissionService`，在 `Update/Delete` 内部判断 `作者 || 有权限`，不在中间件里做。
- **封号与禁言双层拦截在网关**：`role=ban` 封号与会话吊销由 `SessionStore` 承担；`Banned` 表禁言由网关 `banCheck` 拦截写操作，故意返回 404 而非 403。
- **AuthGate 取代硬编码 public/protected 判断**：设置值经 `auth.go` 内 30s TTL 进程级缓存（key 为 `gateCache`），KV 故障时允许 2×TTL 的旧值降级，设置项缺失视为空值（fail-open），其它 KV 错误 fail-closed 返回 `INTERNAL_ERROR`。

> 引用本包以外的位置时用符号名而非行号——行号在任何一次拆分或插入后都会腐化（本轮 router 拆分后即出现大批失效行号）。

## RequirePermission 的凭证语义（2026-10-30）

`RequirePermission(deps, code)` 不再只查角色权限表：它调用 `authz.Allowed(authz.CredentialFrom(ctx.Values()), …)`，与会话/网关同一判据——API Key 必须 scope 与角色同时覆盖，OAuth 客户端只认 scope。此前这里忽略 scope，等于让 route 级门控（`/admin/mcp`、`/admin/backups`、`/admin/webhooks`）对机器凭证失去上限。

## 响应缓冲缺陷修复（2026-11-11）

`PathBodyLimit` 此前给**每个请求**替换 `ResponseWriter`，把响应收集进 `bytes.Buffer`，
理由是想在 handler 把 `MaxBytesReader` 错误写成 400 之后，再把它改写成 413。

代价被实测确认：`/api/v1/upload/file/{key}`（上限 10MB）的下载要等整个文件生成完
才发首字节，并发下载时内存按文件大小成倍占用。测试 `TestPathBodyLimitStreamsLargeResponse`
用「首字节必须在 handler 返回前到达客户端」钉住了这个性质——它在旧实现下会超时失败。

修法（不牺牲 413）：

1. **GET/HEAD/OPTIONS 直接放行**。没有请求体就不可能超限，顺带让文件下载完全不经过任何包装。
2. **正常路径透传**。`bodyLimitWriter` 的 `Write` 直接转给底层 writer，不再攒 buffer。
3. **413 在 `WriteHeader` 里改写**。超限发生在 handler 读 body 时（早于写响应），
   所以状态码还有机会改；`WriteHeader` 必须连信封 body 一起写出，否则 handler 的
   400 body 会盖上来。

三条性质各有测试（超限→413 信封 / 正常透传 / GET 不包装 / 首字节流式），
且做过变异验证：去掉 GET 快路径会让 `TestPathBodyLimitSkipsBodylessMethods` 变红。

## 404/405 信封（2026-11-11）

Iris 默认对未匹配路由与方法不匹配直接写纯文本 `Not Found`，绕过统一信封。前端按
`{code,msg,data}` 解析会静默失败（拿不到 code，看起来像"请求失败但无错误信息"）。

`middleware.RegisterNotFoundHandlers(app)` + `iris.WithFireMethodNotAllowed` 一起解决：

- 404 → `{code: 3001, msg: "接口不存在"}`
- 405 → `{code: 2001, msg: "请求方法不允许：…"}`（复用 `INVALID_PARAMS`，不自造错误码）
- **业务主动返回的 404 不被覆盖**：`OnErrorCode` 只在框架自己产生错误码时触发，
  `pkg.FailStatus(ctx, 404, ...)` 不会二次触发（`TestDeliberateNotFoundIsNotOverwritten` 钉住）

`OnErrorCode` 只能在 app 上注册（中间件拿不到 `*iris.Application`），所以是普通函数
而不是 `iris.Handler`。
