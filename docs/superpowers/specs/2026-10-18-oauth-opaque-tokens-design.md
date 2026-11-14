# OAuth2 令牌改为 opaque，JWT 仅保留 OIDC id_token

- 日期：2026-10-18
- 状态：已批准，实施中
- 背景：用户登录态早已迁移到服务端会话（`user_sessions` 行 + sid），JWT 只剩两条尾巴：本站作为 OAuth2 Provider 签发的 access/refresh token，以及 OIDC IdP 的 `id_token`。目标是让「用户/第三方令牌」彻底不再是 JWT，把 JWT 收窄到 OIDC 规范强制的那一处。

## 现状（实施前）

| 用途 | 位置 | 令牌 |
| --- | --- | --- |
| 用户登录/鉴权 | `internal/gateway/auth.go` | 已 opaque（`user_sessions` sid），无 JWT |
| OAuth2 authorization_code | `oauth_provider.go` | 用户 JWT（HS256，claims 带 role/user_uuid）+ 不可兑换的 refresh JWT |
| OAuth2 client_credentials | `oauth_provider.go` | 客户端 JWT（HS256，`client_id`/`scope`） |
| 令牌撤销 | `oauth_provider.go` `Revoke` + `auth.go` `clientTokenRevoked` | 解析 JWT 取 JTI，翻 `oauth_tokens.revoked` + KV `bl:<jti>` |
| OIDC `id_token` | `sso.go` `generateIDToken` | RS256 JWT（规范强制） |
| OIDC access token | `sso_oidc.go` `Token` / `UserInfo` | 用户 JWT（HS256） |
| 密钥/配置 | `JWT_SECRET`、`pkg/jwt`、`jwt_*_ttl` | HS256 全局密钥；启动强校验 |

已确认 `Token` 端点只支持 `authorization_code` 与 `client_credentials`——**没有 refresh grant**，因此现签发的 refresh token 无法兑换，属于死能力。

## 决策

1. **OAuth2 access token 改为 opaque**：`of_at_` + 32 字节 `crypto/rand` 的 base64url。服务端只存 SHA-256 hex。
2. **`id_token` 保留 JWT（RS256）**：OIDC Core 规定 id_token 必须是 JWT，无法移除；`golang-jwt` 依赖因此保留。
3. **refresh token 停止签发**：没有 refresh grant，发了也用不了；响应中移除该字段。等将来实现 refresh grant 时再以 opaque 令牌补回。
4. **`JWT_SECRET` 退场**：不再有 HS256 用户令牌，配置项、启动强校验、compose 环境变量一并删除。
5. **顺手修 id_token 签名密钥的生命周期缺陷**：现实现每次进程启动生成新 RSA 私钥，重启后 JWKS 变化导致旧 id_token 验不过，多副本集群各签各的。改为首次生成后以 PEM 存入 settings（`sso_oidc_signing_key`），之后复用。

## 令牌格式与存储

- 明文：`of_at_` + `base64.RawURLEncoding(32B rand)`，仅返回给客户端，不落库。
- 落库：`oauth_tokens.access_token = hex(sha256(明文))`。**列名保留**（`access_token`），Go 字段改为 `TokenHash` 并显式 `column:access_token`，避免 sqlite/mysql/postgres 三套迁移；语义变化只体现在模型注释。
- 不新增 `token_hash` 列的原因：`access_token` 现有 `NOT NULL + UNIQUE`，AutoMigrate 不会撤销，若新增列则插入空串会撞唯一索引；复用列是零迁移成本的唯一稳妥选择。
- 旧数据：历史行存的是 JWT 的 JTI，与新 hash 永不匹配 → 自然失效，无需清理迁移。
- `refresh_token` 字段从模型移除；旧列保持空置（无 `NOT NULL`，GORM 不写）。

## 校验路径（`gateway.AuthChecker`）

顺序不变：**会话 sid 优先**（用户登录态），未命中再按 opaque OAuth token 查表。

1. 计算 `hash = sha256(cred)`，按 `access_token` 查 `oauth_tokens`；
2. 行不存在 / `revoked` / `expires_at <= now` → 视为无效令牌（沿用现有错误码：缺凭证 1001、无效或过期 1002、过期 1003、撤销 1014）；
3. 用户令牌（`user_id != ""`）：按 `user_id` 现查 `model.User` 取 `username`/`role`——角色变更立即生效，不再有 24 小时陈旧的 `role` 声明；用户已被删除则令牌失效；
4. 客户端令牌（`user_id == ""`）：`username = client_id`、`role = oauth_client`、`scopes = row.scope`；
5. 写入 context 的键与旧实现完全一致（`CtxUsername`/`CtxUserID`/`CtxRole`/`CtxAuthMethod=oauth_client`/`CtxClientScopes`），下游无感知。

删除 `clientTokenRevoked` 与 KV `bl:` 撤销标记：撤销语义改为「翻 `revoked` 列」，查表即生效，不再需要第二处状态。

## 签发与撤销

- `authorization_code`：查用户 → 签发 opaque → `oauth_tokens` 落行（含 `user_id`、`scope`、`expires_at`、`client_id`）→ 响应 `{access_token, token_type: Bearer, expires_in, scope}`（无 refresh_token）。
- `client_credentials`：scope 校验逻辑不变 → 签发 opaque → 落行（`user_id` 空）。
- `Revoke`（RFC 7009）：按 hash 查行，命中且属于该 client 则 `revoked = true`；令牌不存在/格式任意一律返回 200（避免令牌探测）。

## `id_token`（唯一 JWT）

- RS256 + `kid=orinforum-rs256-1`，签名密钥 PEM 存 settings `sso_oidc_signing_key`（首次生成 2048 位，之后复用）；读到损坏值则重新生成并覆盖。
- TTL 从 `jwt.AccessTokenTTL` 拆为独立设置 `sso_id_token_ttl`（默认 3600 秒）。
- 删除从未接线的 `verifyIDTokenKeySep`。

## 配置与设置项

| 旧 | 新 |
| --- | --- |
| `JWT_SECRET` 环境变量 + 启动强校验 | 删除（不设也能启动） |
| `jwt_access_token_ttl`（86400） | `oauth_access_token_ttl`（默认仍 86400，客户端行为不变） |
| `jwt_refresh_token_ttl`（604800） | 删除（不再签发 refresh token） |
| `pkg.InitJWTTTL` | `pkg.InitOAuthTTL` |
| — | `sso_id_token_ttl`（3600） |

旧 key 行留在 settings 表中但不再被读取（无害，管理员可自行删除）。

## 退场清单

- `app/server/pkg/jwt/` 整包（含未跟踪的 `jwt_test.go`）。
- `internal/gateway/auth.go` 的 JWT 解析分支、`clientRevocationPrefix`/`clientTokenRevoked`。
- `internal/config` 的 `JWTConfig`、`server/iris.go` 的 `InitJWT` 与 `change-me-in-production` 致命校验。
- 5 个 compose 文件中的 `JWT_SECRET`。
- dashboard `settingsConfig.ts` 的两个 jwt key。
- 注释与文档中的「JWT required / JWT middleware / JWT claims」措辞 → session/opaque 措辞。

## 破坏性变更

1. **已签发的 OAuth/OIDC access token 全部失效**，第三方客户端需重新走 authorize/token 换取新令牌（`401 {code:1002}`）。
2. 授权码流响应不再包含 `refresh_token`（本来也无法兑换）。
3. 部署侧可删除 `JWT_SECRET`；留着也不会被读取。
4. 升级后首个请求会触发 RSA 私钥生成并写入 settings（一次性）。

## 验收

- `go build ./...` / `go vet ./...` / `go test ./...` 全绿。
- 单测：签发→鉴权链路（用户令牌、客户端令牌 scope/role）、撤销后 1002/1014、过期拒绝、未知令牌拒绝、无 user_id 的客户端令牌、用户被删即失效；OIDC token 端点返回 opaque access + JWT id_token、userinfo 接受 access 而拒绝 id_token、JWKS 跨调用/跨 provider 实例稳定。
- 冒烟：注册 OAuth client → client_credentials 取 token → 带 token 调受保护接口成功 → `oauth/revoke` → 再调失败；不设 `JWT_SECRET` 可正常启动；`:9091/metrics` 不受影响。

## 不做（YAGNI）

- 不实现 refresh_token grant / token introspection 端点。
- 不写旧 `oauth_tokens` 行清理迁移（自然过期）。
- 不引入 JWT 以外的令牌格式抽象层：`of_at_` 前缀 + sha256 就够，不预留多算法。
