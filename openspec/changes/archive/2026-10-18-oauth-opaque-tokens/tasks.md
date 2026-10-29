# Tasks

> 设计：[docs/superpowers/specs/2026-10-18-oauth-opaque-tokens-design.md](../../../docs/superpowers/specs/2026-10-18-oauth-opaque-tokens-design.md)（已批准）。
> 验证 Go 需 `export PATH="/usr/local/go/bin:$PATH"` 或登录 shell。

## 1. 令牌模型与单点 helper

- [x] 1.1 `internal/model/oauth_token.go`：`AccessToken` → `TokenHash`，显式 `column:access_token`（复用旧列，sqlite/mysql/postgres 零迁移），`json:"-"` 不泄露摘要；删除 `RefreshToken` 字段（refresh token 停发，旧列静默弃用）；注释写明列名是历史遗留
- [x] 1.2 `internal/gateway/token.go`（新）：`oauthAccessTokenPrefix = "of_at_"`、`newOAuthAccessToken`（32B crypto/rand → base64url）、`hashOAuthToken`（sha256 hex，单点契约）、`lookupOAuthAccessToken`（查表 + 过滤 `revoked`/`expires_at`，返回可区分的三类错误）、`storeOAuthAccessToken`（落行）、`oauthClientRole = "oauth_client"`

## 2. 校验路径

- [x] 2.1 `internal/gateway/auth.go`：`Middleware` 删除 JWT 解析分支，改为会话 sid → opaque 查表（未知 1002 / 过期 1003 / 撤销 1014）；`OptionalMiddleware` 同理但静默
- [x] 2.2 `applyOAuthToken`：用户令牌按 `user_id` 现查 `model.User` 取 username/role（角色变更立即生效、用户被删即失效）；客户端令牌投影 `username=client_id`、`role=oauth_client`、`scopes=row.scope`，ctx 键与旧实现完全一致
- [x] 2.3 删除 `clientRevocationPrefix` 与 `clientTokenRevoked`：撤销改为翻 `revoked` 列，KV `bl:` 标记退场

## 3. 签发与撤销

- [x] 3.1 `oauth_provider.go` authorization_code：签发 opaque + 落行（含 user_id/scope），响应移除 `refresh_token`
- [x] 3.2 `oauth_provider.go` client_credentials：签发 opaque + 落行（user_id 空），scope 校验逻辑不变
- [x] 3.3 `Revoke`：按 `sha256(token)` + client_id 更新 `revoked=true`；未知/非法令牌仍返回 200（RFC 7009 §2.2 防探测）；删除 `extractJTI`

## 4. OIDC（唯一保留的 JWT）

- [x] 4.1 `sso.go`：签名密钥生命周期改造——`signingKey(ctx)` 首次生成 2048 位 RSA，PEM 存 settings `sso_oidc_signing_key`（经可选 `settingsWriter` 接口；只读 fake 时退化为进程内），之后复用；`kid` 由公钥 sha256 推导（`rs256-<16hex>`），密钥轮换对 relying party 可见
- [x] 4.2 `generateIDToken` 由包级函数改为 `(*SSOProvider)` 方法，TTL 取 `sso_id_token_ttl`（默认 3600s，`loadSettings` 解析）；删除从未接线的 `verifyIDTokenKeySep` 与 `signingMethods`、`buildJWKS`（改 `jwks(ctx)`）
- [x] 4.3 `sso_oidc.go`：token 端点 access token 改 opaque（落行）+ id_token 仍 RS256；`userinfo` 改为查 `oauth_tokens`（要求 `user_id` 非空）并按用户行返回，**id_token 不能当 access token 用**

## 5. 配置与设置项

- [x] 5.1 `internal/config/config.go`：删除 `JWTConfig` / `JWT_SECRET` 读取
- [x] 5.2 `server/iris.go`：删除 `jwt_pkg.InitJWT` 与 `change-me-in-production` 启动强校验；`pkg.InitJWTTTL` → `pkg.InitOAuthTTL`，启动日志改打 OAuth TTL
- [x] 5.3 `internal/pkg/settings.go`：删除 `InitJWTTTL`/`jwt_pkg` 依赖，新增 `OAuthAccessTokenTTL` + `InitOAuthTTL`（`oauth_access_token_ttl`，默认 86400）
- [x] 5.4 `internal/model/setting.go` 默认值：`jwt_access_token_ttl`/`jwt_refresh_token_ttl` → `oauth_access_token_ttl` + 新增 `sso_id_token_ttl`（3600）
- [x] 5.5 删除 `app/server/pkg/jwt/`（含未跟踪的 `jwt_test.go`）；`app/server/pkg/` 目录随之空置

## 6. 依赖方与文档

- [x] 6.1 5 个 compose 文件移除 `JWT_SECRET`
- [x] 6.2 dashboard `settingsConfig.ts`：两个 jwt key → `oauth_access_token_ttl` / `sso_id_token_ttl`
- [x] 6.3 代码注释改词：`handler/ctx.go`、`handler/oauth.go`、`service/oauth.go`、`middleware/maintenance.go`、`model/user.go`、`gateway/{gateway,auth,oauth_provider}.go`、`router/{router,routes_account,routes_session,routes_public}.go`
- [x] 6.4 `make swag` 重新生成 `docs/` + `openapi.json`（`User.ID` 注释里的「JWT claims」随模型注释更新而消失）
- [x] 6.5 根 `AGENTS.md`、`docs/ARCHITECTURE.md`、`docs/backend-overview.md`、`docs/design/session-auth.md`、`docs/api-reference.md`、`internal/{config,pkg,router}/AGENTS.md` 同步；历史迁移叙述保留并标注「已删除/已关闭」
- [x] 6.6 `docs/superpowers/specs/2026-10-18-oauth-opaque-tokens-design.md` 设计文档；本 change 的 proposal/design/specs delta

## 7. 顺带修复的既存缺陷

- [x] 7.1（既有 bug）OAuth 客户端注册/重置密钥必然 500：`randomString(40)` 生成 80 字符密钥，超出 bcrypt 72 字节上限，`GenerateFromPassword` 报错；仓库里 `TestOAuthProvider_RegisterClient_Success` 一直以 `t.Skip("TODO: investigate 500 error with bcrypt integration")` 挂着。改为 `randomString(32)`（64 hex），并取消 skip、补「签发的密钥能在 token 端点换到令牌」断言
- [x] 7.2（既有缺陷）id_token 签名密钥每次重启重新生成 → JWKS 变化、旧 id_token 验不过、多副本各签各的（见 4.1）

## 8. 测试与验证

- [x] 8.1 `internal/gateway/auth_test.go`：删除 JWT 生成夹具；新增 legacy JWT 串被拒（1002）、client token 鉴权（username=client_id）、用户令牌取当前 role、撤销（1014）、过期（1003）、未知（1002）、用户被删即失效（1002）；`newStore` AutoMigrate 补 `OAuthToken`/`User`
- [x] 8.2 `internal/gateway/oauth_provider_test.go`：去 JWT 夹具；client_credentials 断言 opaque 前缀 + 只存 hash + 无 refresh_token；authorization_code 断言 opaque + 行绑定 user + 授权码一次性；`TestNewOAuthAccessToken_Format` 替换 `TestExtractJTI`
- [x] 8.3 `internal/gateway/sso_test.go`：JWKS 跨调用稳定、id_token RS256 可验（kid/iss/sub/aud/nonce）、签名密钥经 settings 跨实例复用；新增 token 端点端到端（opaque access + JWT id_token，userinfo 接受 access、拒绝 id_token）
- [x] 8.4 `go build ./...` / `go vet ./...` / `go test ./... -count=1` 全部退出码 0（19 包 ok，零 FAIL）
- [x] 8.5 冒烟（隔离副本 `PORT=18081`，`.env` 中**不含** `JWT_SECRET`）：
  - 不设 `JWT_SECRET` 正常启动，`/health` 200；
  - admin 登录 → 注册 OAuth client（修复前必 500）→ `client_credentials` 取到 `of_at_` 令牌（49 字符，响应无 `refresh_token`，`expires_in=86400`）；
  - 受保护端点用 client token：scope 不足返回 `403 {code:1013}`（证明鉴权已解析为 oauth_client）；`authorization_code` 流转出的用户令牌访问 `POST /notifications/list` → **200**；
  - `POST /oauth/revoke` 后同一令牌 → **401 {code:1014 令牌已被撤销}**；未知令牌 → `401 {code:1002}`；
  - DB 断言：`oauth_tokens.access_token` 为 64 字符 sha256 hex（非明文），授权码流行 `user_id` 有值、client_credentials 行为空；
  - OIDC：启用 SSO 后 `/.well-known/jwks.json` 的 `kid` 在**进程重启前后一致**，settings 中出现 `sso_oidc_signing_key` PEM；discovery 端点返回标准字段
- [x] 8.6 `gofmt -l` 对本轮新增/修改文件无输出（仓库既有的 19 个未格式化文件不在本轮范围）

## 9. 升级与部署收尾

> 面向运维与集成方的完整操作手册：[docs/design/oauth-opaque-token-upgrade.md](../../../docs/design/oauth-opaque-token-upgrade.md)

- [x] 9.1 编写升级说明 `docs/design/oauth-opaque-token-upgrade.md`：把「已签发 token 在部署瞬间全量失效」、失效表现（`401 code 1002`，区别于 1003 过期 / 1014 撤销）、恢复路径（重新走 authorize/token）、以及 **refresh_token 停发**、**id_token 不能当 access token 用** 两条易踩坑点写成一页可直接转发给集成方的材料；附升级后自查命令与回滚说明（不支持数据回滚，回滚须再次全量重授权）
- [x] 9.2 清理迁移 `db/migrations/cleanup_legacy_oauth_tokens.sql`：幂等删除遗留 JTI 行（按 `LENGTH(access_token) <> 64` 判定，opaque hash 恒 64 hex、JTI 恒 36 字符 UUID，两者不重叠）与 settings 表 `jwt_access_token_ttl` / `jwt_refresh_token_ttl` 两行；脚本内置备份与验证语句。**属可选维护动作**：遗留行本就不会被新逻辑命中，等待 `expires_at` 自然过期也正确
- [x] 9.3 `JWT_SECRET` 已从 `config.Validate()` 与全部 compose 文件移除，不设可正常启动；secret manager 的删除动作与 9.1 的集成方通知一并交由发布流程执行，仓库侧已无代码依赖
