---
title: Database, NATS, and Storage
description: Database choice and migrations, why external NATS is required and what lives in its KV, local or S3 storage, and mail providers
---

Database, NATS, file storage, and mail share one property: **they are read at startup**, so a
wrong value either stops the process or needs a restart. **Change behaviour in the admin UI,
change dependencies by restarting.**

## Before you start

- Shell access to the host — none of this lives in an admin form.
- Database and NATS reachable; see [Environment Variables](/reference/environment/) for the list.

## Database

| Variable | Default | Notes |
|----------|---------|-------|
| `DB_TYPE` | `sqlite` | `sqlite` / `mysql` / `postgres` |
| `DB_DSN` | empty | File path for `sqlite`; required for the others |
| `DB_MAX_OPEN_CONNS` / `DB_MAX_IDLE_CONNS` | `25` / `10` | Connection pool ceiling and idle keep |
| `DB_CONN_MAX_LIFETIME` / `DB_CONN_MAX_IDLE_TIME` | `300` / `60` | Max age and idle reclaim (seconds) |

`sqlite` suits single-node installs; an empty `DB_DSN` puts the file at `db/app.db` under the
backend working directory. Multi-instance deployments need `postgres` or `mysql`, because
concurrent SQLite writes serialize behind locks.

Migrations run two ways: **AutoMigrate at startup** adds missing columns and indexes
(repeatable), while hand-written SQL in `app/server/db/migrations/` **never runs automatically**.

## NATS: why it is mandatory

An empty `NATS_URL` makes the process **exit**, not degrade: NATS carries the **KV storage** for
session caching, rate-limit counters, settings, OAuth state, and codes, plus the **event bus** for
notifications, moderation, and webhooks. It must be external with **JetStream enabled**:

```bash
nats-server -js                       # local
docker run --rm -p 4222:4222 nats:2.10-alpine -js   # container
```

| Variable | Default | Notes |
|----------|---------|-------|
| `NATS_URL` | empty | **Required**, e.g. `nats://127.0.0.1:4222` |
| `NATS_BUCKET` | `feedchamber_kv` | JetStream KV bucket; must differ per environment |

| Data | Prefix | If lost |
|------|--------|---------|
| Session and settings cache | `sess:`, `setting:` | Falls back to normal DB reads |
| Rate-limit windows | `rl:` | Windows reset, limits briefly loosen |
| OAuth state | `oauth:`, `oauth_state:` | In-flight authorization fails; retry |
| Email codes, MFA challenges | `emailcode:`, `mfa:login:` | Those codes become invalid |
| View-count buffer | `vc:` | Unflushed counts are lost (60s flush) |

:::caution[Never share a bucket between environments]
Two sites on one `NATS_BUCKET` overwrite each other's settings and session caches. Rename it for
staging.
:::

The event bus guarantees "one consumption per event per cluster" via queue subscriptions, so
multi-instance deployments do not duplicate notifications. Events use core pub/sub, so anything
published during an outage is not replayed — a deliberate trade-off.

## Storage

Storage is a **startup singleton**: read once, then fixed — restart after changing it.

| Setting key | Env fallback | Default |
|-------------|--------------|---------|
| `storage_provider` / `storage_local_dir` | `STORAGE_PROVIDER` / `STORAGE_LOCAL_DIR` | `local` / `uploads` |
| `s3_endpoint` / `s3_bucket` / `s3_region` | same names | empty / empty / `us-east-1` |
| `s3_access_key_id` / `s3_access_key_secret` | same names | empty |
| `s3_use_ssl` / `s3_custom_domain` | `S3_USE_SSL` / `S3_CUSTOM_DOMAIN` | `true` / empty |

Precedence is **settings table → environment → default**; the admin entry is
`/admin/settings?tab=storage`. To switch to S3, fill endpoint, region, bucket, and keys, set the
provider to `s3`, restart, and upload an image to check the returned URL.

## Mail

Configured at `/admin/settings?tab=user` → "Email". The master switch is `email_enabled`; with
it off, requesting a code fails immediately.

| Key | Default | Notes |
|-----|---------|-------|
| `email_provider` | `log` | `log` / `smtp` / `sendgrid` / `resend` |
| `email_from` / `email_from_name` | empty / `Feed Chamber` | Sender address and display name |
| `email_smtp_host` / `email_smtp_port` | empty / `587` | SMTP endpoint and port |
| `email_username` / `email_password` | empty | SMTP credentials |
| `email_use_tls` | `false` | `true` = implicit TLS (465); `false` = STARTTLS (587) |
| `email_api_key` | empty | SendGrid or Resend key |

`log` writes the message to the server log instead of sending — the cheapest local baseline for
"did SMTP actually send?". `resend` and `sendgrid` fail loudly without `email_api_key`. Sending
limits live with the codes: `email_resend_cooldown` (60s), `email_daily_limit` (10/day),
`email_code_ttl` (600s), `email_code_length` (6); see
[Authentication and Security](/configuration/authentication/).

Notification mail has its own switches: `email_notify_enabled` (reply / mention emails) and
`email_digest_enabled` (weekly digest). Both default to `false` and **only affect notification
mail** — the code flow is gated by `email_enabled`.

### Sending a test message

The bottom of the "Mail" group has **Send test email**: it uses the *saved* configuration to
deliver one real message to an address you enter, and reports the effective route
(provider / host / port / TLS mode) plus the failure reason.

- A delivery failure is not a 500 — the endpoint returns `ok: false` with the reason, so a wrong
  host, password or port is visible directly.
- With the `log` provider it says "written to the log only, the recipient got nothing" so nobody
  mistakes it for a real delivery.
- Save the config first — the test always uses what is persisted.

## Verify

```bash
# Database: the server answers and reads settings
curl -s -X POST http://localhost:8080/api/v1/settings/public | jq '.code'
# NATS: startup log shows "EventBus/KV Store: external NATS at nats://..." and no KV error

# Storage: request a presigned upload and inspect the returned URL prefix
curl -s -X POST http://localhost:8080/api/v1/upload/presign \
  -H 'Authorization: Bearer <sid>' -H 'Content-Type: application/json' \
  -d '{"file_name":"t.png","content_type":"image/png"}' | jq '.data.fileUrl'
```

## Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| Exits with `NATS_URL is required` | Not configured, or the name is wrong | Start `nats-server -js`, then set `NATS_URL` |
| `初始化 KV Store 失败` | JetStream is not enabled | Start with `-js` |
| `failed to initialize database` | Bad DSN, missing database, no privileges | Connect with `psql` / `mysql` first |
| Random 429s with several instances | They point at different NATS or buckets | Align both variables |
| S3 set but uploads stay local | Storage is a startup singleton | Restart the backend |
| Uploads return 403 | Wrong keys, bucket policy, or SSL mismatch | Verify with `aws s3 ls` |
