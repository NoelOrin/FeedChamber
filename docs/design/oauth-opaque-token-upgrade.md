# OAuth / OIDC 令牌切换升级说明

> 对应 change：`2026-10-18-oauth-opaque-tokens`（已归档）
> 影响面：对外签发的 OAuth2 / OIDC access token 与内部 `JWT_SECRET` 配置

## 一句话结论

服务端的 **access token 已从 JWT 改为 opaque 字符串**。部署本版本的瞬间，此前签发的所有
access token 立即失效，调用方会收到 `401 {code: 1002}`。集成方必须重新走
`authorize` → `token` 换取新令牌。

## 升级前必须做的事

### 1. 通知所有 OAuth2 / OIDC 集成方

给他们这份说明与我方变更时间点。需要强调：

- 失效是**立即且全量**的，不存在宽限期，也不会平滑过渡；
- 失效表现是 `401` + `code: 1002`（「令牌错误」），**不是** 1003（过期）或 1014（已撤销）；
- 恢复方式只有一个：用原有 `client_id` / `client_secret` 重新执行授权码流程或
  `client_credentials` 流程取新令牌；
- 新令牌形如 `of_at_` 开头的 43 字符 base64url 串（总长 49），**不再是三段式 JWT**，
  调用方不要尝试解析其内容（claims 未编码在令牌里）。

### 2. 通知依赖 `refresh_token` 的集成方

本版本**不再签发 refresh token**（`token` 端点响应中已移除该字段）。此前透传保存过
refresh token 的集成方，刷新逻辑会直接失败，必须改为重新授权。

### 3. 若集成方把 id_token 当 access token 用，会失败

`id_token` 仍是 RS256 JWT（OIDC 规范强制），但**只能用于校验身份**，不能用来访问
`userinfo` 或业务端点。`userinfo` 只接受 opaque access token。

## 部署侧清理（不影响正确性，可选）

### JWT_SECRET

已从 `config.Validate()` 与所有 compose 文件中移除，**不设也能正常启动**。
确认无其他系统共用后，可从 secret manager 删除。

### 遗留 settings 行

`jwt_access_token_ttl` / `jwt_refresh_token_ttl` 两行已不被代码读取。
可保留（无害）或执行迁移脚本清理。

### 遗留 oauth_tokens 行

旧 JWT 时代 `oauth_tokens.access_token` 存的是 JTI（UUID 形态，36 字符）；
新逻辑按 `sha256(token)` 精确匹配（恒为 64 字符 hex），**永远不会命中这些行**。
它们不影响任何查询与校验，等待 `expires_at` 自然过期即可。

如需主动清理，执行：

```bash
# 先看规模
sqlite3 app/server/db/app.db "SELECT COUNT(*) FROM oauth_tokens WHERE LENGTH(access_token) <> 64;"

# 备份（建议）
sqlite3 app/server/db/app.db \
  "CREATE TABLE oauth_tokens_jti_backup AS SELECT * FROM oauth_tokens WHERE LENGTH(access_token) <> 64;"

# 清理遗留行与旧 settings
sqlite3 app/server/db/app.db < app/server/db/migrations/cleanup_legacy_oauth_tokens.sql
```

PostgreSQL 版本把 `sqlite3 <path>` 换成 `psql "$DB_DSN" -f`。

## 回滚

**不支持数据回滚。** 一旦新版本签发了 opaque 令牌，回到旧版本会让这些令牌无法校验
（旧逻辑期待 JWT 三段式）。若必须回滚：

1. 回滚代码；
2. 通知集成方**再次**全部重新授权；
3. 旧版本的 `JWT_SECRET` 必须与切换前一致，否则旧 JWT 也会验签失败。

因此建议把本变更安排在低峰期，并预留集成方重授权的时间窗口。

## 验证清单（升级后自查）

```bash
# 1. 不设 JWT_SECRET 也能启动
unset JWT_SECRET && pnpm --dir app/server start

# 2. 新令牌是 opaque 且能访问受保护端点
curl -s -X POST $BASE/oauth/token -d 'grant_type=client_credentials&client_id=...&client_secret=...'
# → access_token 以 of_at_ 开头，响应无 refresh_token

# 3. 旧 JWT 被拒
curl -s $BASE/api/v1/user/profile -H "Authorization: Bearer eyJhbGciOi..."
# → 401 {"code":1002}

# 4. 撤销后立即失效
curl -s -X POST $BASE/oauth/revoke -d 'token=<opaque>'
curl -s $BASE/api/v1/user/profile -H "Authorization: Bearer <opaque>"
# → 401 {"code":1014}
```

## 相关文档

- 设计决策：`docs/superpowers/specs/2026-10-18-oauth-opaque-tokens-design.md`
- 会话鉴权全貌：`docs/design/session-auth.md`
- 规范：`openspec/specs/backend-service-design/spec.md`（路由与鉴权分组）、
  `openspec/specs/operations/spec.md`
