---
title: Error Codes
description: The shared error-code namespace, default messages, and HTTP status mapping
---

Error codes are a **single shared namespace** (`internal/pkg/errors.go`): the gateway, handlers,
and services use the same ranges, and modules may not invent their own. The response body is
always:

```json
{ "code": 2001, "msg": "参数无效", "data": null }
```

`code` is the business code, `msg` is the default human-readable text, and `data` is `null` on
failure. Check `code === 0` for success rather than the HTTP status — the two intentionally
differ in a few cases.

## Authentication and credentials `1xxx`

| Code | Name | HTTP | Default message | Typical trigger |
|------|------|------|-----------------|-----------------|
| `1001` | `TOKEN_NOT_EXIST` | 401 | 令牌不存在 | No `Authorization` or `X-API-Key` |
| `1002` | `TOKEN_WRONG` | 401 | 令牌错误 | Malformed sid, unknown API key |
| `1003` | `TOKEN_EXPIRED` | 401 | 令牌已过期 | Session expired or its row was cleaned up |
| `1010` | `INVALID_PASSWORD` | 401 | 密码错误 | Wrong password |
| `1011` | `USER_NOT_FOUND` | 401 | 用户不存在 | Unknown username; usually indistinguishable from 1010 |
| `1012` | `USERNAME_EXISTS` | 401 | 用户名已存在 | Registration or rename conflict |
| `1013` | `FORBIDDEN` | 403 | 禁止访问 | Insufficient permission or space-level denial |
| `1014` | `TOKEN_REVOKED` | 401 | 令牌已被撤销 | Old sid after logout, password, or role change |
| `1015` | `USER_BANNED` | 403 | 用户已被封禁 | `role=ban` or a write during a ban |
| `1016` | `EMAIL_EXISTS` | 401 | 邮箱已存在 | Duplicate registration email |

## Parameters `2xxx`

| Code | Name | HTTP | Default message | Typical trigger |
|------|------|------|-----------------|-----------------|
| `2001` | `INVALID_PARAMS` | 400 | 参数无效 | Missing field, wrong type, out of range, invalid sort key |

## Resources `3xxx`

| Code | Name | HTTP | Default message | Typical trigger |
|------|------|------|-----------------|-----------------|
| `3001` | `NOT_FOUND` | 404 | 资源不存在 | Missing or invisible post, user, or space |
| `3002` | `ALREADY_EXISTS` | 409 | 资源已存在 | Unique constraint (slug, invite code) |

## Rate limits and bodies `4xxx`

| Code | Name | HTTP | Default message | Typical trigger |
|------|------|------|-----------------|-----------------|
| `4001` | `RATE_LIMITED` | 429 | 请求过于频繁，请稍后再试 | Endpoint budget or the global per-IP ceiling |
| `4002` | `BODY_TOO_LARGE` | 413 | 请求体过大 | `PathBodyLimit` or the upload limit |

## Server `5xxx`

| Code | Name | HTTP | Default message | Typical trigger |
|------|------|------|-----------------|-----------------|
| `5001` | `INTERNAL_ERROR` | 500 | 服务器内部错误 | Unclassified internal error, panic fallback |
| `5004` | `REQUEST_TIMEOUT` | 504 | 请求超时 | Soft-timeout middleware (SSE excluded) |
| `5005` | `MAINTENANCE` | 503 | 站点维护中，请稍后再试 | Writes while `site_maintenance=true` |

## Two deliberate exceptions

**Banned users get 404, not 403.** The gateway's ban check treats a banned account as a missing
resource so it cannot be used to confirm that an account exists. `USER_BANNED` maps to 403 in
general, but the ban path returns 404 via `pkg.FailStatus` — the status itself is the contract.

**OAuth / OIDC protocol endpoints skip the envelope.** `/.well-known/*`, `token`, and `userinfo`
must keep their RFC shapes (`{"access_token": "...", "token_type": "Bearer"}`,
`{"error": "invalid_grant"}`), or standard clients break. This is the only sanctioned bypass.

## Client handling

```ts
const res = await fetch('/api/v1/posts/list', { method: 'POST', body: JSON.stringify(body) });
const payload = await res.json();

if (payload.code === 0) return payload.data;
if (payload.code === 1003 || payload.code === 1014) {
  // Session is gone: clear local credentials and redirect to login.
}
throw new ApiError(payload.code, payload.msg);
```

Do not branch on the HTTP status: `1003` and `2001` may both arrive as 4xx but need different
handling.
