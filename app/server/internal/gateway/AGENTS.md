# Gateway 网关层

请求流水线里的**凭证解析与授权子系统**。它只被 `server/iris.go`（构造 + 传给 router）与 `internal/router`（挂载两段中间件、注册网关自持的端点）消费。

> 上级：[app/server/AGENTS.md](../../AGENTS.md) · 相邻：[middleware](../middleware/AGENTS.md)（身份确定**之后**的门控）、[router](../router/AGENTS.md)（挂载点）、[handler](../handler/AGENTS.md)（身份确定**之后**的业务层）、[container](../container/AGENTS.md)（装配顺序）

实测规模：31 个文件 / 7625 行，其中 13 个测试文件（`pipeline_test.go` 钉住两段幂等，`permission_test.go` 钉住路由→权限映射，`session_store_test.go` 钉住会话生命周期）。

## 为什么在这一层而不是 `middleware`

`middleware` 只做「身份已确定之后」的门控（trace、日志、CORS、限流、超时、体积、维护模式、动态登录门、权限码门），**不解析任何用户凭证**。凭证解析、会话权威、权限映射、OAuth/OIDC 协议端点都在这里。这条分界由 `middleware/AGENTS.md` 与本节共同钉死，改动任一侧前先读另一侧。

## 依赖注入：`Deps`

`Deps` 由 `server/iris.go` **一次构造**并复用给 `Gateway` / `SessionStore` / `SecurityManager`：

```go
type Deps struct {
    SettingService pkg.SettingReader      // 限流开关、暴力破解阈值、SSO 配置
    Perms          pkg.PermissionChecker  // 权限判定（内存缓存）
    Ban            pkg.BannedChecker      // 禁言判定
    KV             kvstore.Store          // 会话缓存 / 限流 / OAuth state / 安全事件
    DB             *gorm.DB               // 会话行、OAuth token 行
}
```

字段类型是 `internal/pkg` 里的接缝接口，**不是本包另起一套同形接口**。`KV` 此前声明为 `any`（号称避免直接依赖），实际代价是每个消费点做一次运行时断言、接错线时静默降级成「限流关闭 / 无会话」——现为具体类型。

`NewSessionStore` 在 `DB` 或 `KV` 为 nil 时返回 nil（降级成「无会话」而不是 panic）；`AuthChecker` 会跳过 nil 的 sessions。

## 两段流水线（本包最重要的契约）

```
v1.Use(gw.Preflight())        # 每个 /api/v1 请求
  ├─ tracing      : 路由匹配后把端点模板写进 trace（route 字段）
  ├─ rateLimit    : 全局每 IP 上限（settings rate_limit_enabled，默认关，60/min）
  └─ credential   : **可选**凭证解析（缺失/伪造一律放行成匿名）

protected.Use(gw.Enforce())
  ├─ 必需凭证（无身份时按 X-API-Key / ak_ 前缀走 APIKey，否则走会话）
  ├─ banCheck     : role=ban 或 Banned 表命中 → **404**（不是 403，避免泄露资源存在性）
  ├─ permission   : path → 权限码映射（未映射 = 拒绝）
  ├─ SecurityCheck: 风控（IP 黑名单 / 用户黑名单 / 异常位置）
  └─ TrackingMiddleware: 会话活跃时间节流更新
```

两段以 ctx values 上的 `gw_stages` 位标记**各自幂等**（`ranStage` / `markStage`），所以重复挂载不会二次计数。

**这不是防御性冗余，而是修掉的真实缺陷**：v1 层的 preflight 对 protected 同样生效，旧的组合入口 `Handler()` 又把 preflight 跑一遍，于是受保护请求以同一个 `rl:ratelimit:<ip>` 键计数两次、会话也解析两次——实测 60/min 的上限在第 **30** 次 protected 请求就 429，修复后落在第 58 次。`Handler()` 已于 2026-10-27 删除，**不要为「方便」重新引入整体挂载入口**；`pipeline_test.go` 会捕获计数翻倍。

## 文件表

| 文件 | 组件 | 职责 |
|------|------|------|
| `gateway.go` | `Gateway` + `New` | 聚合体、`Deps`、两段流水线（`preflight` / `enforce`）、`Preflight()` / `Enforce()`、`isAPIKeyRequest`、`banCheck` |
| `auth.go` | `AuthChecker` | `presentCredential`（Bearer → 非 Bearer 头拒绝 → HttpOnly `master_session` cookie）；`Middleware`（必需）与 `OptionalMiddleware`（可选）；`applySession` / `applyOAuthToken` 把身份投影进 ctx |
| `permission.go` | `PermissionCheckerMiddleware` | `initRouteMap()` 里的 path → 权限码映射（exact / prefix / space pattern / pattern 四级），**未映射的 protected 路由默认拒绝** |
| `ratelimit.go` | `RateLimiter` | 全局每 IP 上限，走 `internal/pkg/ratelimit` 引擎，策略名 `global` |
| `tracing.go` | `Tracing` | 路由匹配后把端点模板写进 trace（`route` 日志字段），不调 `ctx.Next()` |
| `session_store.go` | `SessionStore` | 会话权威：`Create` / `Lookup`（DB 行 + KV `sess:<sid>` read-through）/ `Revoke` / `RevokeAllForUser` / `Touch` / `Extend` |
| `session_issue.go` | `SessionStore` | `Issue`（登录签发）/ `Rotate`（`/auth/refresh` 单用轮换）/ `shouldRenew` |
| `session.go` | `SessionManager` | 会话列表 / 单点吊销 / logout-all / `POST /auth/logout`（**handler 层没有登出文件**）+ `TrackingMiddleware` |
| `apikey.go` | `APIKeyManager` | `X-API-Key` 或 `Bearer ak_` 认证、每 key 限流、Create/List/Revoke。scope → 权限码的注册表已移到 `internal/authz`（网关、middleware、MCP 共用一份） |
| `security.go` | `SecurityManager` | 暴力破解计数、IP/用户黑名单、异常位置、安全事件与登录历史、可疑上报 |
| `oauth.go` + `oauth_{google,github,discord}.go` | `OAuthManager` | **消费方** OAuth2 登录/绑定（Google / GitHub / Discord） |
| `oauth_provider.go` + `token.go` | `OAuthProvider` | **自建** OAuth 2.0 Provider：客户端注册 / authorize / token（authorization_code + client_credentials）/ revoke；access token 为 opaque |
| `sso.go` + `sso_oidc.go` | `SSOProvider` | OIDC SSO IdP：Discovery / JWKS / Authorize / Token / UserInfo；唯一签发 JWT 的地方是 `id_token`（RS256） |

## 凭证模型（不要退回 JWT）

| 凭证 | 形态 | 权威来源 |
|------|------|----------|
| 用户会话 | 64 位 hex sid，HttpOnly `master_session` cookie 或 `Authorization: Bearer <sid>` | `user_sessions` 行存在即有效；KV `sess:<sid>` 只做 read-through 缓存 |
| OAuth2 / OIDC access token | `of_at_` + 32B base64url，库中只存 SHA-256 hex | `oauth_tokens` 行（`revoked` 列控制撤销） |
| OIDC `id_token` | RS256 JWT | 签名私钥 PEM 持久化在 settings `sso_oidc_signing_key`，`kid` 由公钥推导 |
| API Key | `ak_` 前缀 | `api_keys` 行 + scope 注册表（`internal/authz`） |

**没有 token 黑名单**：「凭证是否有效」等价于「行是否存在」。登出 / 改密 / 改角色 / 封号都是删行。`pkg/jwt` 整包已删除；除 `id_token` 外不要再引入任何 JWT 签发或解析。

## 本包自持的 HTTP 端点

这些端点由本包注册（router 里直接挂 `gw.Xxx.Method`），**handler 层没有对应文件**——找它们来这里：

| 端点 | 方法 |
|------|------|
| `POST /api/v1/auth/logout` | `Session.LogoutHandler` |
| `GET /api/v1/sessions`、`POST /api/v1/sessions/{id}/revoke`、`POST /api/v1/sessions/logout-all` | `Session.*` |
| `GET /api/v1/security/events`、`GET /api/v1/security/login-history`、`POST /api/v1/security/report` | `Security.*` |
| `POST /api/v1/api-keys`、`POST /api/v1/api-keys/list`、`POST /api/v1/api-keys/{id}/revoke` | `APIKey.*` |
| `POST /api/v1/oauth/token`、`POST /api/v1/oauth/revoke` | `Provider.*`（协议端点，响应体是 RFC 形状，**不走** `pkg.Success` 信封） |
| `GET /api/v1/oauth/authorize`、`POST /api/v1/oauth/authorize`、`POST /api/v1/oauth/clients*` | `Provider.*` |
| `GET /api/v1/sso/authorize`、`POST /api/v1/sso/token`、`GET /api/v1/sso/userinfo` | `SSO.*`（协议端点） |
| `GET /.well-known/openid-configuration`、`GET /.well-known/jwks.json` | `SSO.Discovery` / `SSO.JWKS` |

`/metrics` **不在**这里，也不在任何业务路由上：它由 `METRICS_PORT`(9091) 独立 listener 暴露（见 [metrics/AGENTS.md](../metrics/AGENTS.md)）。

## 改这里要注意

- **新增 protected 路由必须同时改 `permission.go` 的 `initRouteMap`**，否则默认拒绝（`permission_test.go` 会扫源码对比 router 注册表）。这条规则的反面是安全兜底：宁可 403，不可静默放行。最近的例子：`POST /api/v1/posts/{id}/status`（帖子状态直改）→ `report:manage`，与审核队列同权限域。
- **响应信封**：网关端点同样只能用 `pkg.Success` / `pkg.Fail` / `pkg.FailStatus` / `pkg.HandleError`。唯一例外是 OAuth 2.0 / OIDC 协议端点（必须保持 `{"access_token":…,"token_type":"Bearer"}` 与 `oauthError()` 的 RFC 形状）。
- **错误码共用同一命名空间**（`internal/pkg/errors.go`）：`1xxx` 认证/凭证、`2xxx` 参数、`3xxx` 资源、`4xxx` 限流与请求体、`5xxx` 服务端。不得自造号段。
- **不要在任何 `routes_*.go` 里挂 `Use(gw.*)`**：Iris 在注册那一刻对 party 中间件做快照，兄弟 party 的 `Use` 会覆盖父切片空槽、改写邻居的链（2026-10-14 实测导致权限门整体失效）。`router` 包的 `TestGatewayStagesMountedOnlyInRegister` 会源码扫描禁止。
- **ctx values 的键是跨包契约**：用 `pkg.CtxUserID` / `CtxUsername` / `CtxRole` / `CtxSession` / `CtxAuthMethod` / `CtxAPIKeyScopes` / `CtxClientScopes` 常量（`internal/pkg/ctxkey.go`），不要写字面量。`CtxUserID` 装的是 `User.ID`（ULID）。
- **封禁返回 404 而非 403** 是刻意的：探测者无法区分「被禁」与「资源不存在」。改这个状态码等于改安全契约。
- **吊销是一次性的，不能被缓存写回复活**。`sess:<sid>` 是 read-through 缓存，写入方（`Create` / `Lookup` 未命中路径 / `Touch` / `Extend`）都持有调用方快照，若无条件写回，并发请求就会把已吊销的会话重新塞回 KV，凭证在整个 TTL（最长 7 天）内继续有效。现约束：`revokeRow` **先删行、再删缓存**；`Touch` / `Extend` 以 `UPDATE` 的 `RowsAffected` 作为「行仍存在」的证明，为 0 时只删缓存不回写；`Lookup` 未命中路径写完缓存后再 `sessionExists` 复核，行已消失就丢弃并以 `ErrSessionNotFound` 返回。`session_store_test.go` 的 `TestSessionStore_RevokedSessionStaysDead*` 逐条钉住这些路径。
- 新增测试优先用 `kvstub_test.go` 里的 KV 替身，别连真 NATS。

## 本文件何时更新

- 新增/删除子组件（`Gateway` 的字段、文件表一行）
- 流水线阶段增减或顺序变化（尤其 `banCheck` / `permission` / `SecurityCheck` 的相对次序）
- 凭证模型变化（会话 sid、opaque OAuth token、API Key scope 表）

> 2026-10-30：授权判据统一到 `internal/authz.Allowed`——会话只看角色表，API Key 要 scope **且** 角色，OAuth 客户端只认 scope。此前这段逻辑在本包 `permission.go`、`middleware.RequirePermission`、`internal/mcp` 各写一份且互不一致（MCP 那条是「或」，比网关宽一格）。
- 本包新增或移除自持 HTTP 端点
- `Deps` 增删字段

## XP / 门控权限（2026-10-29）

`initRouteMap` 为新增后台面补了映射（未映射的受保护路径一律 403，这是默认拒绝，不是放行）：

- `/api/v1/admin/levels/`、`/admin/xp-rules/`、`/admin/xp-adjustments/`、`/admin/trust/recompute`、`/admin/users/{id}/xp*` → `level:manage`
- `/api/v1/admin/gates/` → `gate:manage`
- `/api/v1/posts/{id}/feature` → `report:manage`

> 2026-10-29：项目更名为 Feed Chamber（Go module `feedchamber`、npm `@feedchamber/*`、镜像 `feedchamber-*`）。
