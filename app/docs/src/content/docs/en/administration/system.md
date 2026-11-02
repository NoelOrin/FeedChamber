---
title: System and Integrations
description: Service settings, API keys, OAuth clients, webhooks, backups, plugins, MCP, audit logs, and security events
---

"System" is the only admin group that changes runtime behaviour directly. Settings are written to the `settings` table and apply immediately, while integrations hand out machine credentials. Their blast radius is larger than everyday content work, so each module requires `setting:manage` or its own code.

## Prerequisites

You are in the admin area and hold the module's code: `setting:manage` for settings, OAuth clients, and MCP; `role:manage` for webhooks; `backup:manage` for backups; `plugin:manage` for plugins. API key authentication is disabled by default (`api_key_enabled=false`); until you enable it, every key-authenticated request returns 403.

## Service settings

`/admin/settings?tab=<key>` has six tabs:

| Tab | Covers |
|-----|--------|
| `site` | Site name, description, logo, URL, SEO and share image, verification, maintenance |
| `user` | Registration switch and mode, invites, password policy, email verification, MFA, sessions |
| `content` | Moderation mode, page sizes, upload limits and types, SSE and view-count intervals |
| `oauth` | Third-party sign-in (GitHub, Google, QQ, WeChat, Weibo) and app credentials |
| `storage` | Local storage or S3 parameters |
| `dashboard` | Your own admin account: language, account info, password change |

Saving submits every key in the current tab (`POST /admin/settings/batch`), so switching tabs never drops fields you did not look at. Each save writes an audit entry (`settings.batch_update`).

:::caution
Storage configuration is a startup-time singleton. After changing the `storage` tab you must restart the backend; other settings apply immediately.
:::

## API keys

1. "New key" → `POST /api-keys` with a name, `scopes`, `rate_limit`, and `expires_at`.
2. The `api_key` in the response (format `ak_<64 hex chars>`) is shown **once**. The list never returns it again.
3. Call with `X-API-Key: ak_...` or `Authorization: Bearer ak_...`; revoke with `POST /api-keys/{id}/revoke`.

Implementation details that matter:

- Only a **bcrypt hash** is stored (`api_keys.secret_hash`); database access cannot recover the plaintext.
- `scopes` accepts the coarse values `read`, `write`, and `admin`, and also exact permission codes (for example `post:read,report:manage`). At creation every expanded code is checked against the creator's own permissions; asking for more than you have returns 403.
- Rate limiting is per key, per minute: the key's `rate_limit` wins, and 0 falls back to the `api_key_default_rate_limit` setting (default 100). Exceeding it returns 429.
- `expires_at` is a Unix timestamp; an expired key fails with a credential-expired error. Empty means no expiry.

Authorization at request time is an **intersection**: scopes must allow it and the owning user's role must allow it. Adding an `admin` scope to a key never exceeds that account's own permissions.

## OAuth clients

1. Create → `POST /oauth/clients` with a name, `redirect_uris` (JSON array), and allowed `scopes`.
2. The `client_secret` is returned once; the database keeps a bcrypt hash.
3. Rotate a leaked secret with `POST /oauth/clients/{id}/reset-secret`; remove the client with `POST /oauth/clients/{id}/delete`.

Tokens issued to clients are opaque strings, not JWTs, and revocation is immediate. OAuth clients have no user role, so only scopes are checked. Protocol endpoints (`/oauth/authorize`, `/oauth/token`, `/oauth/revoke`) keep their RFC response shapes and do not use the standard envelope.

## Webhooks

1. Create → `POST /admin/webhooks/create` with `url`, `secret`, `events`, `enabled`, `tls_verify`.
2. "Ping" → `POST /admin/webhooks/{id}/ping`, which sends `{"event":"ping"}` to prove connectivity.
3. "Deliveries" → `POST /admin/webhooks/{id}/deliveries/list` for status, attempts, response code, and latency.
4. Replay a failed delivery with `POST /admin/webhooks/{id}/deliveries/{deliveryId}/retry`.

Twelve events are accepted: `post.created`, `post.closed`, `content.created`, `content.liked`, `content.replied`, `content.watched`, `user.created`, `user.registered`, `user.followed`, `user.mentioned`, `report.created`, `moderation.decided`. Any other name is rejected.

Delivery semantics: the body is `{"event": "...", "data": {...}, "ts": <unix>}` with `Content-Type: application/json`. With a `secret`, requests carry `X-Orin-Signature: sha256=<HMAC-SHA256(secret, body)>`, verified against the raw body. Any non-2xx response is a failure, retried up to 5 attempts with backoff of 1 minute, 5 minutes, 30 minutes, 2 hours, then 12 hours; after the fifth failure the delivery is marked `failed` and only a manual replay can retry it. `tls_verify=false` skips certificate validation, so use it only for self-signed internal endpoints.

## Backups and restore

1. "Create backup" → `POST /admin/backups/create` with an optional name (defaults to a timestamp). The artifact is a database snapshot JSON plus a `tar.gz` of the upload directory.
2. List → `POST /admin/backups/list`; download → `POST /admin/backups/{id}/download` (JSON attachment).
3. Restore → `POST /admin/backups/{id}/restore`, which **overwrites current data**. Confirm the file first.
4. Cleanup → `POST /admin/backups/cleanup`, deleting backups older than `backup_retention_days` (default 30).

Files are written to the active storage backend (local directory or S3), so old backups stay where they were after a storage change. You can also export a local backup from the CLI (the output directory is required): `cd app/server && go run main.go -e prod backup --out /path/to/backup-dir`.

## Plugins

`/admin/plugins` stores **declarative metadata only**: name, version, author, description, homepage, settings JSON, and an enabled flag. Install registers it, status toggles the flag, and **no plugin code is ever executed** — there is no plugin runtime or extension point. Treat the page as a bill of materials.

## MCP

`/admin/mcp` lists this server's MCP tools and their switches: 15 tools (6 read, 9 write), with write tools disabled by default. The master switch `mcp_enabled` defaults to `false`, and while it is off `/mcp` rejects everything. The disabled list lives in `mcp_disabled_tools`; to enable a write tool, add `-<tool-name>` (the minus sign means explicit enable). That encoding exists so an accidentally cleared form can only turn everything off, never everything on. `mcp_max_result_bytes` (default 8192) caps a single response.

`/admin/mcp-servers` registers external MCP services: `name`, `endpoint`, `transport` (only `streamable_http` today), `purpose` (`general` or `moderation`), `moderation_tool` (the tool called for moderation, default `check_content`), and `enabled`. Auth headers are redacted in responses; test connectivity with `POST /admin/mcp-servers/{id}/test`. A server with `purpose=moderation` is called first when `moderation_mode=external`, and a failure falls back to async review. See [MCP](/api/mcp/) for the protocol and tool contracts.

## Audit logs

`/admin/audit-logs` is the sensitive-operation trail (`POST /admin/audit_logs/list`), filterable by actor and action. Each entry has `actor_id`, `action`, `target`, `detail`, and `created_at`. Typical actions: `settings.update`, `settings.batch_update`, `user.ban`, `user.unban`, `user.role_update`, `user.delete`, `moderation.approve`, `moderation.reject`, `report.status`, `trash.restore_post`, `trust.xp_adjust`, `gate.rule_save`, and `admin.batch_users.*`. When you need to know who changed a record, start here.

## Security events and sessions

`/admin/security` shows security events with type, severity (`low`, `medium`, `high`, `critical`), user, IP, detail, and a handled flag. `/admin/sessions` shows sessions (device, last activity) with single revoke and "sign out everywhere". Both cover **the current account only**, not a site-wide view. There is no separate switch for another user's sessions; changing their role or banning them revokes all of their sessions, which is the only supported path.

## Verify

1. Create a read-only API key and call `POST /posts/create` with `X-API-Key`: it must return 403.
2. Ping a webhook: the delivery list shows `event=ping` with `status=success`, and the receiver sees the signature header.
3. Create a backup, download it, confirm it is JSON; restore it and spot-check a setting and a post.
4. Open `/admin/mcp`: write tools are off by default; enable one and the matching tool call succeeds.

## Common failures

| Symptom | Cause | Fix |
|---------|-------|-----|
| API key request returns 403 "not enabled" | `api_key_enabled` is still `false` | Enable it in service settings |
| Key creation returns 403 "exceeds creator" | Expanded scopes include codes the creator lacks | Lower the scopes or use a stronger account |
| Webhook keeps retrying | Receiver returns non-2xx or times out | Read the delivery's response code and `last_error` |
| Settings unchanged after a restore | Older snapshot, or the restore failed | Check the timestamp and size, retry, read logs |
| MCP tool calls are rejected | Master switch off, tool disabled, or key lacks the code | Check `mcp_enabled`, `mcp_disabled_tools`, and the key scope |
| Uploads still use the old backend | Storage is a startup-time singleton | Restart the backend process |
