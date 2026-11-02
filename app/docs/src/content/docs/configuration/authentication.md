---
title: 认证与安全
description: 服务端会话、登录与改密、MFA、第三方登录、OIDC SSO、API Key、密码策略与会话管理
---

这个站点没有 JWT 访问令牌。登录后签发的是**服务端会话标识**（sid）：32 字节随机数的
十六进制串，共 64 位，本身不带信息。每次请求都回表查 `user_sessions`——行存在就有效，
删掉就失效。登出、改密码、改角色、封号都是同一种操作：删行。

## 前置条件

- 会话与密码策略在 `/admin/settings?tab=user`，第三方登录在 `?tab=oauth`。
- 后台页面：`/admin/api-keys`、`/admin/oauth-clients`、`/admin/sessions`、`/admin/security`；
  权限码 `setting:manage`、`api-key:self`、`session:self`、`security:read`。

## 会话的工作方式

| 特征 | 取值 |
|------|------|
| 有效期 | 7 天绝对时长；剩余不足一半时，一次已认证请求会把它续到满 7 天 |
| 携带方式 | HttpOnly cookie `master_session`，或 `Authorization: Bearer <sid>` |
| Cookie 属性 | `Path=/`、`HttpOnly`、`SameSite=Lax`，`Secure` 由 `MASTER_COOKIE_SECURE` 控制 |
| 缓存与失效 | KV 键 `sess:<sid>` 读透缓存（权威仍是数据库行）；删行即失效 |

前端另有非 HttpOnly 的 `master_auth` 标志 cookie，只做界面判断、不参与鉴权；发不出
cookie 时用 Bearer 带同一个 sid，行为等价。

## 登录、刷新、登出、改密

| 动作 | 接口 | 说明 |
|------|------|------|
| 登录 | `POST /api/v1/auth/login` | 每 IP 每分钟 10 次；返回 `access_token`（即 sid） |
| 刷新 | `POST /api/v1/auth/refresh` | 单用轮换：签发新 sid 并作废旧 sid |
| 登出 | `POST /api/v1/auth/logout` | 删当前会话行并清 cookie |
| 改密 | `POST /api/v1/auth/change_password` | 校验旧密码；成功后吊销该用户全部会话 |

:::caution[刷新是单用的]
用 Bearer 的客户端调了 `/auth/refresh` 却不保存新的 `access_token`，下一次请求会拿到
1014（令牌已被撤销）。这不是故障，是轮换的预期结果。
:::

## 首次改密

`ADMIN_PASSWORD` 为空时首次启动会随机生成管理员密码并打印日志。导入或重置过的账号可能
带「必须先改密」标记（meta 表，前缀 `must_change_password:`），登录响应里
`must_change_password` 为 true，前端把人送到 `/admin/first-change-password`。改密走
`POST /api/v1/auth/first_change_password`（只接受 `admin`，可选 `name` 改用户名），
管理员重置他人密码用 `/auth/reset_password`；成功即清标记并吊销旧会话。

## 邮箱验证码

先打开邮件：`/admin/settings?tab=user` →「邮件服务」启用 `email_enabled`，选提供商
（`log` / `smtp` / `sendgrid` / `resend`）并填发件人。

| 键 | 默认值 | 说明 |
|----|--------|------|
| `email_code_ttl` | `600` | 验证码有效期（秒） |
| `email_code_length` | `6` | 位数 |
| `email_resend_cooldown` | `60` | 同用途同邮箱的重发冷却（秒） |
| `email_daily_limit` | `10` | 同一邮箱每天最多几封 |

验证码按「用途 + 邮箱」隔离存在 KV 里，连续输错 5 次即作废。发送与校验接口分别是
`send_verify_email`（3 次/分钟）、`verify_email`（10 次/分钟）、`forgot_password`
（3 次/分钟）与 `reset_password_by_code`（5 次/分钟）。

## MFA

站点开关 `mfa_totp_enabled`（默认 `false`）。启用后是标准 TOTP：6 位、30 秒步长、
容忍前后各一步；绑定返回 `otpauth://` 链接，确认后一次性展示 10 个备份码（只存哈希，
必须当场保存）。登录变成两步：`login` 返回 `mfa_required` 与 `mfa_token`，5 分钟内调
`POST /api/v1/auth/mfa/verify` 换正式 sid。维护接口：`/auth/mfa/totp/setup`、`/enable`、
`/disable`、`/auth/mfa/backup/regenerate`。

## 第三方登录

`/admin/settings?tab=oauth` 为六家提供商各留四个键：`oauth_<provider>_enabled`、
`_client_id`、`_client_secret`、`_callback`。provider 取值 `github`、`google`、
`discord`、`qq`、`wechat`、`weibo`。

配法三步：提供商后台建应用、回调填 `<site_url>/api/v1/oauth/<provider>/callback`；凭据
粘进对应分组并打开开关；前台登录页即出现入口（公开配置读 `/api/v1/oauth/providers`）。
已登录用户绑定/解绑用 `/api/v1/oauth/accounts` 与 `/{provider}/link`、`/unlink`。
`/admin/oauth-clients` 是「让第三方应用接入本站」的客户端登记，与社交登录无关。

## OIDC SSO

| 键 | 默认值 | 说明 |
|----|--------|------|
| `sso_oidc_enabled` | `false` | 打开后暴露 OIDC 端点 |
| `sso_oidc_issuer` | 空 | 外部可访问的完整地址，参与发现文档与 `id_token` 的 `iss` |
| `sso_oidc_client_id` / `sso_oidc_client_secret` | 空 | 服务端自用凭据 |
| `sso_id_token_ttl` | `3600` | `id_token` 有效期（秒） |

端点：`/.well-known/openid-configuration`（发现）、`/.well-known/jwks.json`（公钥）、
`/api/v1/sso/token`、`/api/v1/sso/userinfo`。本站**唯一**用 JWT 的地方就是 `id_token`
（RS256），签名私钥持久化在 `sso_oidc_signing_key`——别手工改，改了等于让所有已签发
令牌失效。

## API Key

| 键 | 默认值 | 说明 |
|----|--------|------|
| `api_key_enabled` | `false` | 关闭时所有 API Key 请求被拒，即使密钥本身有效 |
| `api_key_default_rate_limit` | `100` | 单把 Key 的默认限流预算 |

创建入口 `/admin/api-keys`（`POST /api/v1/api-keys`），密钥形如 `ak_` + 64 位十六进制，
**明文只在创建响应里出现一次**，服务端只存哈希。客户端用 `X-API-Key` 头或
`Authorization: Bearer ak_...`，同时出现时 API Key 优先。

申请的 scope 必须是创建者自身权限的子集，否则创建直接 403；请求被 403 时先确认
`api_key_enabled` 已打开。撤销走 `POST /api/v1/api-keys/{id}/revoke`，立即生效。

## 密码策略与暴力破解

| 键 | 默认值 | 说明 |
|----|--------|------|
| `password_min_length` | `8` | 注册与改密的最短长度；上限固定 72 字节 |
| `bcrypt_cost` | `10` | 低于 10 被钳到 10，高于 31 被钳到 31 |
| `security_brute_force_max_attempts` | `5` | 窗口内允许的失败次数 |
| `security_brute_force_window` | `300` | 计数窗口（秒） |

`security_unusual_location_enabled`（默认 `false`）打开后，从没见过的 IP 登录会记一条
中危事件。这几项后台表单没有输入框，用设置接口写：

```bash
curl -s -X POST http://localhost:8080/api/v1/admin/settings/password_min_length/update \
  -H 'Authorization: Bearer <sid>' -H 'Content-Type: application/json' \
  -d '{"value":"12"}'
```

超过阈值时**IP 与登录名分别临时封禁 30 分钟**，各写一条 `brute_force` 高危事件；
登录成功会清掉计数。改完即按新阈值计算，不需要重启。

## 会话与安全事件

`/admin/sessions` 只看**当前账号**的会话：设备、IP、最近活跃时间，可吊销单条或
`logout-all` 全清，不是全站会话总览。`/admin/security` 展示最近 100 条安全事件与当前
账号最近 50 条登录历史；它除了要求 `security:read`，接口内部还硬性要求 `admin` 角色，
给别的角色授了权限码也读不到数据。

## 验证

```bash
# 1. 登录（存 cookie），再用 cookie 读自己的资料
curl -s -c /tmp/fc.txt -X POST http://localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' -d '{"login":"admin","password":"<密码>"}' | jq '.code'
curl -s -b /tmp/fc.txt http://localhost:8080/api/v1/user/profile | jq '.code'

# 2. 登出后同一个 cookie 应立即失效（1001 / 1014）
curl -s -b /tmp/fc.txt -X POST http://localhost:8080/api/v1/auth/logout >/dev/null
curl -s -b /tmp/fc.txt http://localhost:8080/api/v1/user/profile | jq '.code'
```

## 常见失败

| 现象 | 原因 | 处理 |
|------|------|------|
| 验证码收不到 | `email_enabled=false` 或 SMTP 配置错 | 打开开关；用 `log` 提供商在日志里看验证码 |
| MFA 绑定报错 | `mfa_totp_enabled` 仍是 false | 先打开站点级开关 |
| `/admin/security` 打开是空的 | 当前角色不是 `admin` | 换管理员账号 |
