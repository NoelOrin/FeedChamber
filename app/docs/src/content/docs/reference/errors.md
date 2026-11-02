---
title: 错误码
description: 统一错误码命名空间、默认文案与 HTTP 状态映射
---

错误码是**单一共享命名空间**（`internal/pkg/errors.go`）：网关、handler、service 用同一批号段，
不允许各模块自造。响应体固定为：

```json
{ "code": 2001, "msg": "参数无效", "data": null }
```

`code` 是业务码，`msg` 是给人看的默认文案，`data` 失败时恒为 `null`。判断成功只看
`code === 0`，不要依赖 HTTP 状态——两者在少数场景下故意不一致（见文末）。

## 认证与凭证 `1xxx`

| 码 | 名称 | HTTP | 默认文案 | 典型触发 |
|----|------|------|----------|----------|
| `1001` | `TOKEN_NOT_EXIST` | 401 | 令牌不存在 | 未带 `Authorization` 或 `X-API-Key` |
| `1002` | `TOKEN_WRONG` | 401 | 令牌错误 | sid 格式不合法、API Key 不存在 |
| `1003` | `TOKEN_EXPIRED` | 401 | 令牌已过期 | 会话过期（服务端行被清理） |
| `1010` | `INVALID_PASSWORD` | 401 | 密码错误 | 登录密码不匹配 |
| `1011` | `USER_NOT_FOUND` | 401 | 用户不存在 | 登录名不存在；为防枚举通常不区分 1010 |
| `1012` | `USERNAME_EXISTS` | 401 | 用户名已存在 | 注册或改名冲突 |
| `1013` | `FORBIDDEN` | 403 | 禁止访问 | 权限不足、版面级权限拒绝 |
| `1014` | `TOKEN_REVOKED` | 401 | 令牌已被撤销 | 登出、改密、改角色后的旧 sid |
| `1015` | `USER_BANNED` | 403 | 用户已被封禁 | `role=ban` 或禁言期内写操作 |
| `1016` | `EMAIL_EXISTS` | 401 | 邮箱已存在 | 注册邮箱冲突 |

## 参数 `2xxx`

| 码 | 名称 | HTTP | 默认文案 | 典型触发 |
|----|------|------|----------|----------|
| `2001` | `INVALID_PARAMS` | 400 | 参数无效 | 缺字段、类型错误、越界、非法排序键 |

## 资源 `3xxx`

| 码 | 名称 | HTTP | 默认文案 | 典型触发 |
|----|------|------|----------|----------|
| `3001` | `NOT_FOUND` | 404 | 资源不存在 | 帖子 / 用户 / 版面不存在，或不可见 |
| `3002` | `ALREADY_EXISTS` | 409 | 资源已存在 | 唯一约束冲突（slug、邀请码等） |

## 限流与请求体 `4xxx`

| 码 | 名称 | HTTP | 默认文案 | 典型触发 |
|----|------|------|----------|----------|
| `4001` | `RATE_LIMITED` | 429 | 请求过于频繁，请稍后再试 | 端点预算或全局每 IP 上限 |
| `4002` | `BODY_TOO_LARGE` | 413 | 请求体过大 | 超过 `PathBodyLimit` 或上传上限 |

## 服务端 `5xxx`

| 码 | 名称 | HTTP | 默认文案 | 典型触发 |
|----|------|------|----------|----------|
| `5001` | `INTERNAL_ERROR` | 500 | 服务器内部错误 | 未分类的内部错误、panic 兜底 |
| `5004` | `REQUEST_TIMEOUT` | 504 | 请求超时 | 软超时中间件（SSE 不参与） |
| `5005` | `MAINTENANCE` | 503 | 站点维护中，请稍后再试 | `site_maintenance=true` 时的写请求 |

## 两个故意的例外

**封禁用户返回 404，不是 403。** 网关的 `banCheck` 对已封禁账号直接按资源不存在处理，
避免把「这个账号存在且被封」泄露给攻击者。所以你会看到 `USER_BANNED` 的默认映射是 403，
但实际封禁路径返回 404——**状态码本身是契约**，用 `pkg.FailStatus` 显式指定。

**OAuth / OIDC 协议端点不套信封。** `/.well-known/*`、`token`、`userinfo` 必须保持 RFC 形状
（如 `{"access_token": "...", "token_type": "Bearer"}`、`{"error": "invalid_grant"}`），
否则标准客户端直接失败。这是唯一允许绕过统一响应的地方。

## 客户端处理建议

```ts
const res = await fetch('/api/v1/posts/list', { method: 'POST', body: JSON.stringify(body) });
const payload = await res.json();

if (payload.code === 0) return payload.data;
if (payload.code === 1003 || payload.code === 1014) {
  // 会话失效：清本地凭证 → 跳登录
}
throw new ApiError(payload.code, payload.msg);
```

不要按 HTTP 状态分支：`1003` 与 `2001` 可能都来自 4xx，但处理方式完全不同。
