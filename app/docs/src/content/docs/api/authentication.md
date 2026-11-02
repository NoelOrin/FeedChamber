---
title: 认证与权限
description: 用会话 sid 或 API Key 通过认证，理解刷新轮换、OAuth/OIDC 三种凭证与权限码到路由的映射
---

Feed Chamber 的用户凭证是**服务端会话**，不是 JWT：登录拿到一个 64 位十六进制的
sid，服务端在 `user_sessions` 里存一行，行存在即有效。这一页讲清怎么拿到它、怎么带、
什么时候失效，以及 API Key、OAuth 客户端和权限码各管什么。

## 前置条件

- 服务已启动，`/api/v1/health` 可访问，有一个可登录账号。
- 管理员初始密码在首次启动日志里，或由 `ADMIN_PASSWORD` 指定。
- 要用 API Key 时，先在后台「服务设置」把 `api_key_enabled` 打开（默认关）。

## 登录

```bash
curl -i -X POST http://localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"login":"admin","password":"你的密码"}'
```

`data` 里是 `access_token`（sid）与 `user`：

```json
{ "code": 0, "msg": "success",
  "data": { "access_token": "9f3c…", "user": { "id": "01J…", "role": "admin" } } }
```

同时下发两个 cookie：`master_session`（HttpOnly，真正的凭证）与 `master_auth`
（值为 `1` 的可见标志，不含凭证）。登录限流 10 次/分钟。开了 TOTP 的账号不会直接
拿到 sid，而是 `{"mfa_required":true,"mfa_token":"…"}`，需再调
`POST /api/v1/auth/mfa/verify`。

## 两种携带方式

| 方式 | 请求头 / Cookie | 适用 |
|------|-----------------|------|
| Cookie | `master_session`（浏览器自动带） | Web 前端、同源页面 |
| Bearer | `Authorization: Bearer <sid>` | 脚本、CLI、移动端、SSR |

Bearer 优先级高于 cookie。`Authorization` 存在但不是 `Bearer` 形式（例如 `Basic`）
时直接按 `1002` 拒绝，不会回退到 cookie。

```bash
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"login":"admin","password":"你的密码"}' | jq -r '.data.access_token')

curl -s http://localhost:8080/api/v1/user/profile -H "Authorization: Bearer $TOKEN"
```

## 会话生命周期

| 属性 | 值 |
|------|----|
| 形态 | 32 字节随机数的十六进制串（64 字符），无结构、不可解析 |
| 权威来源 | `user_sessions` 的行；KV 的 `sess:<sid>` 只是 read-through 缓存 |
| 有效期 | 绝对 7 天；剩余寿命低于 50% 时任意已认证请求推回 7 天 |
| 吊销 | 删行：登出、改密、改角色、封号都会删除该用户全部会话 |

**没有 token 黑名单**，"sid 还能不能用"等价于"这一行还在不在"，所以改角色立即生效。
登出是 `POST /api/v1/auth/logout`（删行 + 清 cookie）；同源还有 `GET /sessions` 列会话、
`POST /sessions/{id}/revoke` 吊销单条、`POST /sessions/logout-all` 全吊销。

## 刷新是单用轮换

`POST /api/v1/auth/refresh` 签发新 sid 并**立即作废旧 sid**：

```bash
curl -s -X POST http://localhost:8080/api/v1/auth/refresh \
  -H "Authorization: Bearer $TOKEN" | jq -r '.data.access_token'
```

:::caution[Bearer 客户端必须换用新 token]
响应里的 `access_token` 是唯一仍有效的值，旧 sid 已被删除，继续用旧值会得到
`1014 令牌已被撤销`。浏览器客户端不用操心——新的 `master_session` 随响应写入。
:::

## API Key

```bash
curl -s -X POST http://localhost:8080/api/v1/api-keys \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"name":"ci","scopes":"read","rate_limit":60}'
```

| 请求字段 | 说明 |
|----------|------|
| `name` | 展示名，必填 |
| `scopes` | 逗号分隔：粗粒度 `read` / `write` / `admin`，或精确码如 `post:read` |
| `rate_limit` | 每分钟上限；不传用 `api_key_default_rate_limit`（默认 100） |
| `expires_at` | Unix 秒；不传即长期有效 |

响应里的 `data.api_key` 是**唯一一次**出现的明文，服务端只存 bcrypt 哈希。用
`X-API-Key: ak_…` 或 `Authorization: Bearer ak_…` 携带，两者等价。

两条规则决定 Key 能做多少事：创建时申请的 scope 不能超过创建者自身的权限；**scope 是
凭证上限，与实际角色权限取交集**——只读 Key 即使绑在 admin 账号上调写接口也是 403。
`customize:publish` 是唯一例外，只能由 Key 显式 scope 授予，不随 admin 角色继承。
`X-API-Key` 与 `Bearer ak_` 同时出现时按 API Key 处理，不走会话解析。

## OAuth、Provider 与 OIDC

| 方向 | 端点 | 凭证形态 |
|------|------|----------|
| 消费方登录 / 绑定（Google / GitHub / Discord） | `GET /oauth/providers`、`/oauth/{provider}/auth`、`/callback`；已登录时 `POST /oauth/{provider}/link` | 回调后签发本站 sid |
| 本站作为 Provider | `POST /api/v1/oauth/token`（`authorization_code` / `client_credentials`）、`/oauth/revoke` | `of_at_` 前缀 opaque token |
| 本站作为 OIDC IdP | `/.well-known/openid-configuration`、`/jwks.json`、`/api/v1/sso/token`、`/sso/userinfo` | access token 同上；`id_token` 是 RS256 JWT |

Provider 的 access token 只存 SHA-256 摘要，撤销靠 `revoked` 列，**不签发 refresh
token**。全系统唯一的 JWT 是 OIDC 的 `id_token`——规范要求它自包含。这些协议端点
返回 RFC 形状，不套 `{code,msg,data}`。

## 权限码与路由映射

平台级权限码共 **61 个**，判定只有一个实现 `authz.Allowed`：会话只看角色权限表；
API Key 要 scope **且** 角色权限；OAuth 客户端只认 scope；空权限码对登录用户放行。

受保护路由通过**路径 → 权限码**映射拦截，按 exact、prefix、参数化 pattern、带
`space_id` 的 pattern 四级匹配。没登记映射的受保护路径**默认拒绝**（403），新增端点
必须同时补映射。MCP 的 Enforcer 调的是同一个 `authz.Allowed`，所以 `/mcp` 与 HTTP
的授权语义一致，不存在"从 MCP 走一遍就宽一格"的情况。

| 端点 | 权限码 |
|------|--------|
| `POST /posts/create` | `post:create` |
| `POST /posts/{id}/status` | `report:manage` |
| `POST /user/{id}/role/update` | `user:update` |
| 任意 `/admin/settings/*` | `setting:manage` |

## 401、403 与 404

| 状态 | 业务码 | 含义 |
|------|--------|------|
| 401 | `1001` | 没带任何凭证 |
| 401 | `1002` | 凭证格式非法（非 Bearer 头、Key 不存在） |
| 401 | `1003` | 会话过期（行被清理） |
| 401 | `1014` | 会话被撤销（登出、改密、改角色、轮换后的旧 sid） |
| 403 | `1013` | 已登录但权限不足，或该路径未登记权限映射 |
| 404 | `1015` | **账号被封禁或禁言**——故意用 404 而非 403 |

封禁返回 404 是刻意的：探测者无法区分"账号被封"与"资源不存在"。所以别用"404 就是
资源没了"来判断——请求本身合法时，先看 `msg` 与账号状态。

## 验证与常见失败

```bash
curl -s -o /dev/null -w '%{http_code}\n' http://localhost:8080/api/v1/user/profile \
  -H "Authorization: Bearer $TOKEN"      # 200：会话有效
```

| 现象 | 原因 | 处理 |
|------|------|------|
| 刷新后立刻 `1014` | Bearer 客户端没换新 sid | 每次刷新后替换本地 token |
| API Key 一律 `1013` | `api_key_enabled` 未开，或 scope 缺权限码 | 打开开关；追加精确权限码 |
| admin 账号的 Key 被拒 | scope 是上限，不因角色放宽 | 给 Key 加对应 scope |
| 脚本 401、浏览器正常 | 脚本没带 `Authorization`，也没有 cookie | 显式带 Bearer |
| 被封禁账号收到 404 | 设计如此 | 查后台用户状态，不要重试 |
