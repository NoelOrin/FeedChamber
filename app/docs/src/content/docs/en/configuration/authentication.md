---
title: Authentication and Security
description: Server-side sessions, login flows, MFA, social login, OIDC SSO, API keys, password policy, and session management
---

There are no JWT access tokens here. Login mints a **server-side session id** (sid): 32 random
bytes as 64 hex characters. Every request resolves it against `user_sessions`, so deleting the
row kills the credential instantly — logout, password change, and role changes (a ban is one).

## Before you start

- Session and password policy: `/admin/settings?tab=user`; social login: `?tab=oauth`.
- Pages: `/admin/api-keys`, `/admin/oauth-clients`, `/admin/sessions`, `/admin/security`;
  codes: `setting:manage`, `api-key:self`, `session:self`, `security:read`.

## How sessions work

| Property | Value |
|----------|-------|
| Lifetime | 7 days absolute; below half remaining, an authenticated request renews it |
| Transport | HttpOnly cookie `master_session`, or `Authorization: Bearer <sid>` |
| Flags | `Path=/`, `HttpOnly`, `SameSite=Lax`; `Secure` follows `MASTER_COOKIE_SECURE` |
| Cache / revoke | KV `sess:<sid>` read-through; the DB row is authoritative and deleting it revokes |

The non-HttpOnly `master_auth` cookie only drives UI state; where cookies cannot be sent, pass
the same sid as a Bearer token.

## Login, refresh, logout, password change

| Action | Endpoint | Notes |
|--------|----------|-------|
| Login | `POST /api/v1/auth/login` | 10/min per IP; `access_token` is the sid |
| Refresh | `POST /api/v1/auth/refresh` | Single-use rotation: new sid issued, old revoked |
| Logout | `POST /api/v1/auth/logout` | Deletes the row and clears cookies |
| Change password | `POST /api/v1/auth/change_password` | Verifies the old password, revokes all sessions |

Refresh is single-use: a Bearer client that discards the new `access_token` gets 1014 next time.

With an empty `ADMIN_PASSWORD`, first startup generates an admin password and logs it. Imported
or reset accounts may carry a "must change password" flag (meta prefix `must_change_password:`),
so login returns `must_change_password: true` and the UI routes to
`/admin/first-change-password`; clear it with `POST /api/v1/auth/first_change_password`.

## Email codes and MFA

Enable mail first: `?tab=user` → "Email", turn on `email_enabled`, pick `log`, `smtp`,
`sendgrid`, or `resend`, and set the sender. `email_code_ttl` (600s), `email_code_length` (6),
`email_resend_cooldown` (60s), and `email_daily_limit` (10 per address per day) tune the codes;
they live per "purpose + email" in KV and die after 5 wrong attempts. Endpoints:
`send_verify_email`, `verify_email`, `forgot_password`, `reset_password_by_code`.

MFA is gated by `mfa_totp_enabled` (default `false`): standard TOTP, 6 digits, 30-second step,
±1 tolerance, ten single-view backup codes (hashes only). `login` then returns `mfa_required`
plus `mfa_token`, and `POST /api/v1/auth/mfa/verify` within 5 minutes exchanges a TOTP value or
backup code for the real sid.

## Social login

`?tab=oauth` keeps `oauth_<provider>_enabled`, `_client_id`, `_client_secret`, and `_callback`
for `github`, `google`, `discord`, `qq`, `wechat`, and `weibo`. Create the provider app with
callback `<site_url>/api/v1/oauth/<provider>/callback`, paste the credentials, enable it, and
the login page shows the option. `/admin/oauth-clients` is unrelated: it registers clients for
this server acting as an OAuth 2.0 provider.

## OIDC SSO

Keys are `sso_oidc_enabled` (default `false`), `sso_oidc_issuer` (public base URL, feeds
discovery and the `id_token` `iss`), `sso_oidc_client_id`, `sso_oidc_client_secret`, and
`sso_id_token_ttl` (default `3600`). Endpoints: `/.well-known/openid-configuration`,
`/.well-known/jwks.json`, `/api/v1/sso/token`, `/api/v1/sso/userinfo`. The `id_token` is the
**only** JWT in the product (RS256); its key persists in `sso_oidc_signing_key` — editing that
setting invalidates every issued token.

## API keys

`api_key_enabled` (default `false`) gates the mechanism; `api_key_default_rate_limit` (default
`100`) sets the per-key budget. Create keys at `/admin/api-keys`: `ak_` plus 64 hex characters,
plaintext shown **once**, only a hash stored. Send it as `X-API-Key` or
`Authorization: Bearer ak_...` (the header wins). Scopes must be a subset of the creator's
permissions or creation returns 403; revoke with `POST /api/v1/api-keys/{id}/revoke`.

## Password policy and brute force

`password_min_length` (default `8`, max 72 bytes) and `bcrypt_cost` (default `10`; below 10
clamps up, above 31 clamps down) set the policy. Brute force uses
`security_brute_force_max_attempts` (default `5`) over `security_brute_force_window` (default
`300` seconds); `security_unusual_location_enabled` (default `false`) logs an event the first
time an account logs in from an unseen IP. None of these has a form field:

```bash
curl -s -X POST http://localhost:8080/api/v1/admin/settings/password_min_length/update \
  -H 'Authorization: Bearer <sid>' -H 'Content-Type: application/json' -d '{"value":"12"}'
```

Exceeding the threshold blocks **the IP and the username for 30 minutes** and writes a
`brute_force` high-severity event; a successful login clears the counters.

## Sessions and security events

`/admin/sessions` lists only the **current account's** sessions with per-session revoke and
`logout-all`. `/admin/security` shows the last 100 security events and the current account's
last 50 logins; besides `security:read` it also requires the `admin` role.

## Verify

```bash
# Log in, keep the cookie, read your own profile
curl -s -c /tmp/fc.txt -X POST http://localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' -d '{"login":"admin","password":"<password>"}' | jq '.code'
curl -s -b /tmp/fc.txt http://localhost:8080/api/v1/user/profile | jq '.code'
```

After `POST /api/v1/auth/logout`, the same cookie must fail with 1001 or 1014.

## Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| Codes never arrive | `email_enabled=false` or SMTP misconfigured | Enable it; with `log` it lands in the log |
| API key requests return 403 | `api_key_enabled=false`, or scope exceeds the creator | Enable it; recreate with a stronger account |
