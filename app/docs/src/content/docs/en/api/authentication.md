---
title: Authentication and Permissions
description: Authenticate with a server-side session sid or an API key, understand refresh rotation, OAuth/OIDC credentials, and how permission codes map to routes
---

User credentials are **server-side sessions**, not JWTs. Logging in returns a 64-character hex
`sid` backed by a row in `user_sessions`; the row existing means the credential is valid. This
page covers how to get one, how to send it, when it dies, and what API keys, OAuth clients and
permission codes add on top. To use API keys, enable `api_key_enabled` in the admin settings
first — it defaults to off.

## Logging in

```bash
curl -i -X POST http://localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"login":"admin","password":"your-password"}'
```

The payload is `{"access_token":"9f3c…","user":{"id":"01J…","role":"admin"}}`.

`login` accepts a username or an email and is rate limited to 10 per minute. Two cookies are
set too: `master_session` (HttpOnly, the real credential) and `master_auth` (a readable `1`
flag). Accounts with TOTP enabled get `{"mfa_required":true,"mfa_token":"…"}` and must finish
at `POST /api/v1/auth/mfa/verify`.

## Sending the credential

| Mode | Header / cookie | Use |
|------|-----------------|-----|
| Cookie | `master_session` (sent by browsers) | the web app |
| Bearer | `Authorization: Bearer <sid>` | scripts, CLIs, mobile, SSR |

Bearer wins over the cookie, and a non-Bearer `Authorization` header (say `Basic`) is rejected
with `1002` — there is no fallback.

## Session lifecycle

Sessions are 32 random bytes as hex, valid for 7 days absolute, renewed back to 7 days once
below 50% remaining. There is no token blacklist: validity is exactly "the `user_sessions` row
exists", which is why logout, password change, role change and bans delete rows and take effect
immediately (KV `sess:<sid>` is only a read-through cache). Logout is
`POST /api/v1/auth/logout`; `GET /sessions` lists sessions, `POST /sessions/{id}/revoke` kills
one by numeric primary key, `POST /sessions/logout-all` kills all.

### Refresh rotates

`POST /api/v1/auth/refresh` mints a new sid and **revokes the old one immediately**.

:::caution[Bearer clients must adopt the new token]
The returned `access_token` is the only valid value; the old sid is gone and returns `1014`.
Browsers need no action — the new `master_session` cookie comes with the response.
:::

## API keys

```bash
curl -s -X POST http://localhost:8080/api/v1/api-keys \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"name":"ci","scopes":"read","rate_limit":60}'
```

`name` is required; `scopes` is comma separated (`read` / `write` / `admin`, or exact codes
like `post:read`); `rate_limit` defaults to `api_key_default_rate_limit` (100 per minute);
`expires_at` is Unix seconds and optional. `data.api_key` comes back **once** — only a bcrypt
hash is stored. Send it as `X-API-Key: ak_…` or `Authorization: Bearer ak_…`.

Two rules bound a key: requested scopes cannot exceed the creator's permissions, and **the
scope is a ceiling intersected with the owner's role permissions** — a read-only key stays
read-only even on an admin account. `customize:publish` is the exception: only an explicit key
scope grants it, never the admin role. When both headers appear, the API key path wins.

## OAuth, provider and OIDC

| Direction | Endpoints | Credential |
|-----------|-----------|------------|
| Social login / linking (Google, GitHub, Discord) | `GET /oauth/providers`, `/oauth/{provider}/auth`, `/callback` | a site sid after the callback |
| Site as OAuth 2.0 provider | `POST /api/v1/oauth/token` (`authorization_code`, `client_credentials`), `/oauth/revoke` | opaque `of_at_` token |
| Site as OIDC IdP | `/.well-known/openid-configuration`, `/jwks.json`, `/api/v1/sso/token`, `/sso/userinfo` | access token as above; `id_token` is an RS256 JWT |

Provider access tokens are stored as SHA-256 digests and revoked through a `revoked` column;
**no refresh token is issued**. The OIDC `id_token` is the only JWT in the system, because the
spec requires it to be self-contained. These endpoints return RFC-shaped bodies, not the forum
envelope.

## Permission codes and route mapping

There are **61 platform permission codes**, decided in one place, `authz.Allowed`: sessions
against role permissions; API keys need scope **and** role; OAuth clients against scope alone;
an empty code only requires being signed in.

Protected routes are gated by a **path → permission code** map matched in four passes (exact,
prefix, parameterised pattern, `space_id` pattern); an unmapped protected route is **denied by
default** (403). The MCP enforcer calls the same `authz.Allowed`, so `/mcp` and HTTP share
authorization semantics.

| Endpoint | Permission |
|----------|------------|
| `POST /posts/create` | `post:create` |
| `POST /posts/{id}/status` | `report:manage` |
| `POST /user/{id}/role/update` | `user:update` |

## 401, 403 and 404

| Status | Code | Meaning |
|--------|------|---------|
| 401 | `1001` / `1002` / `1003` / `1014` | missing, malformed, expired or revoked credential |
| 403 | `1013` | signed in but missing the permission, or unmapped route |
| 404 | `1015` | **account banned or muted** — deliberately not 403 |

Bans answer 404 so a prober cannot tell "banned" from "no such resource". For an otherwise
valid request, never read 404 as "the resource is gone" — check `msg` and the account state.

## Failures

| Symptom | Cause | Fix |
|---------|-------|-----|
| `1014` right after refreshing | Bearer client kept the old sid | replace the token on every refresh |
| every API key call is `1013` | `api_key_enabled` off, or scope missing the code | enable it; add the exact code |
| scripts 401, browser fine | no `Authorization` and no cookie | send Bearer explicitly |
