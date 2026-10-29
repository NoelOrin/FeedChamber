## Why

用户登录态在 2026-10-14 的会话鉴权迁移后已经是 opaque（`user_sessions` 行 + 64 位十六进制 sid），但对外 OAuth2/OIDC 的 access token 仍是 HS256 JWT，于是同一套系统里并存**两种凭证语义**：登录态靠「行是否存在」，OAuth 令牌靠「签名是否有效」。具体代价：

1. **同一身份两种凭证**：撤销、过期、封号的处理方式分叉，读代码的人必须同时理解「删行」与「验签 + 黑名单」两条路径。
2. **JWT 黑名单双写**：`POST /oauth/revoke` 既要翻 `oauth_tokens.revoked`，又要在 KV 写 `bl:<jti>`；`auth.go` 的 `clientTokenRevoked` 每次请求多一次 KV 读。两处状态一旦不同步，就是「撤销了还能用」或「没撤销却被拒」。
3. **`JWT_SECRET` 成为启动硬依赖**：`config.Load()` 默认 `change-me-in-production`，`server/iris.go` 见到该默认值直接 `logrus.Fatal`。想跑一个实例必须先造一个密钥，而它只服务于那些本不该是 JWT 的令牌。
4. **role 声明可陈旧 24h**：授权码流签发的 JWT 把 `role` 写进 claims，TTL 由 `jwt_access_token_ttl` 决定（默认 86400 秒）。期间降权/封号对该令牌无效，最长 24 小时后才自然过期。
5. **id_token 签名密钥每次重启重新生成**：`sso.go` 用包级 `sync.Once` + `rsa.GenerateKey` 产出进程级私钥，JWKS 随重启变化 → 旧 id_token 验不过；多副本各签各的 key，relying party 取到的 JWKS 取决于打到哪个副本。这是既有缺陷，与本轮同源（密钥生命周期无人负责），一并修掉。

## What Changes

- **OAuth2/OIDC access token 改 opaque**：`of_at_` + 32 字节 `crypto/rand` 的 base64url 明文，服务端只存 `hex(sha256(明文))` 于 `oauth_tokens.access_token`。列名保留，Go 字段改 `TokenHash` 并显式 `column:access_token`，sqlite/mysql/postgres 三套零迁移。新增 `internal/gateway/token.go` 承载签发 / 查表 / 哈希单点。
- **撤销改为翻列**：`Revoke`（RFC 7009）按 hash 查行、置 `revoked=true`；删除 `auth.go` 的 `clientRevocationPrefix` / `clientTokenRevoked` 与 KV `bl:` 标记，撤销语义不再有第二处状态。
- **refresh token 停发**：`Token` 端点只支持 `authorization_code` 与 `client_credentials`，**没有 refresh grant**，签出来的 refresh token 根本不可兑换（死能力）。响应移除该字段，模型删 `RefreshToken` 列映射。
- **JWT 仅存 OIDC `id_token`**：RS256 不变（OIDC Core 强制 id_token 是 JWT）；签名私钥首次生成后以 PEM 存入 settings `sso_oidc_signing_key`，之后复用，读到损坏值重新生成并覆盖；`kid` 由公钥推导（`rs256-<16hex>`）；TTL 由新设置 `sso_id_token_ttl`（默认 3600 秒）管理。`generateIDToken` 从包级函数改方法，删除从未接线的 `verifyIDTokenKeySep` 与 `signingMethods`。
- **`pkg/jwt` 整包删除**：`app/server/pkg/jwt/` 退场，`JWT_SECRET` 环境变量与启动强校验移除，`jwt_access_token_ttl` / `jwt_refresh_token_ttl` 换成 `oauth_access_token_ttl` + 新增 `sso_id_token_ttl`，`pkg.InitJWTTTL` → `pkg.InitOAuthTTL`。
- **顺带修正既存 spec 漂移**：`backend-service-design` 第 43/48 行仍要求 protected 路由 MUST 应用 `JWTAuth() + BanCheck()`，而这两个中间件在 2026-10-14 会话鉴权迁移中已删除（生产调用数实测为 0）。本轮把该 requirement 改写为现状（`protected.Use(gw.Enforce())`：必需凭证 → banCheck → 权限 → 安全风控 → 会话跟踪）。**这是本轮之前就存在的漂移，不是本轮引入的行为变更**——在此说明是为了避免归档后被读成「本轮删了这两个中间件」。

## Capabilities

### Modified Capabilities

- `operations` — 环境变量清单删除 `JWT_SECRET`；补充 OAuth2/OIDC access token 生命周期由 settings `oauth_access_token_ttl` 管理、id_token 由 `sso_id_token_ttl` 管理，两者均不再是启动必填环境变量。
- `layered-architecture` — 删除「现存唯一例外：`app/server/pkg/jwt`」段：该包已整包删除，例外不复存在，`pkg/` 下不再有任何包。
- `backend-service-design` — protected 路由鉴权描述从 `JWTAuth() + BanCheck()` 改为 `gw.Enforce()` 的现状。

## Impact

- **代码**：`internal/gateway/{token.go(新),auth.go,oauth_provider.go,sso.go,sso_oidc.go,gateway.go}`、`internal/model/{oauth_token.go,setting.go}`、`internal/config/config.go`、`internal/pkg/settings.go`、`internal/{handler/ctx.go,handler/oauth.go,service/oauth.go}`、`server/iris.go`；删除 `app/server/pkg/jwt/`。
- **破坏性变更（对外可见）**：
  - 已签发的 OAuth2/OIDC access token **全部失效**（旧行存的是 JWT 的 JTI，与新 hash 永不匹配），第三方客户端需重新走 authorize/token 换取新令牌，表现为 `401 {code:1002}`。生产环境须提前通知集成方。
  - 授权码流 token 响应不再包含 `refresh_token`（原本也无法兑换）。
  - 部署侧可删除 `JWT_SECRET`；留着也不会被读取，不再影响启动。
  - 升级后首个 OIDC 请求会触发 RSA 私钥生成并写入 settings（一次性）。
- **依赖方**：5 个 compose 文件删 `JWT_SECRET`；dashboard `settingsConfig.ts` 的两个 jwt key 换为 `oauth_access_token_ttl` / `sso_id_token_ttl`。
- **不在本轮范围**：不实现 `refresh_token` grant / token introspection 端点；不写旧 `oauth_tokens` 行清理迁移（旧行自然过期、永不匹配）；不引入令牌格式抽象层（`of_at_` 前缀 + sha256 就够，不预留多算法）。
- **文档**：根 `AGENTS.md`（结构树 / Auth / Gateway / Config / 环境变量表）、`internal/config/AGENTS.md`、`docs/ARCHITECTURE.md`、`docs/backend-overview.md` 中「JWT required / `pkg/jwt` 仍存在 / `JWT_SECRET` 仍为默认则拒绝启动」等措辞同步；本轮实测仍有残留，见 tasks.md 的「仍需人工确认」。
