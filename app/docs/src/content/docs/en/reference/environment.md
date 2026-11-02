---
title: Environment Variables
description: Variables read by the backend and frontend, their defaults, and how they relate to the settings table
---

Environment variables only cover **what a process needs to start**: listening port, database
connection, NATS address, logging. Site behaviour (registration, moderation, email, OAuth,
upload limits) is runtime configuration stored in `settings`, applied without a restart, and
should not be baked into a compose file.

The rule of thumb: **change behaviour in the admin panel, change dependencies with a restart.**

## Backend

| Variable | Default | Notes |
|----------|---------|-------|
| `PORT` | `8080` | HTTP listen port |
| `DB_TYPE` | `sqlite` | `sqlite` / `mysql` / `postgres` |
| `DB_DSN` | empty | **Required** for `mysql` / `postgres`; a file path for `sqlite` |
| `DB_MAX_OPEN_CONNS` | `25` | Maximum open connections |
| `DB_MAX_IDLE_CONNS` | `10` | Maximum idle connections |
| `DB_CONN_MAX_LIFETIME` | `300` | Maximum connection lifetime in seconds |
| `DB_CONN_MAX_IDLE_TIME` | `60` | Idle connection timeout in seconds |
| `NATS_URL` | empty | **Required.** External NATS with JetStream; startup fails when empty |
| `NATS_BUCKET` | `feedchamber_kv` | JetStream KV bucket name |
| `ADMIN_PASSWORD` | empty | Password for the seeded admin; random and logged when empty |
| `SITE_URL` | empty | Fallback for `site_url`, used by sitemap and canonical |
| `METRICS_ENABLED` | `true` | Start the 9091 metrics listener |
| `METRICS_PORT` | `9091` | Metrics port, **internal network only** |
| `TRACING_ENABLED` | `true` | Parse inbound `traceparent` and echo it back |
| `LOG_LEVEL` | `info` | `debug` / `info` / `warn` / `error` |
| `LOG_FORMAT` | `json` | `json` / `text` |
| `LOG_OUTPUT` | `stderr` | `stdout` / `stderr` or a file path |

### Storage and email

These have **lower priority than `settings`**: a stored value wins, the environment variable is
only a fallback for empty values. Storage changes need a restart because storage is a
startup singleton.

| Variable | Settings key | Default |
|----------|--------------|---------|
| `STORAGE_PROVIDER` | `storage_provider` | `local` |
| `STORAGE_LOCAL_DIR` | `storage_local_dir` | `uploads` |
| `S3_ENDPOINT` | `s3_endpoint` | empty |
| `S3_REGION` | `s3_region` | `us-east-1` |
| `S3_BUCKET` | `s3_bucket` | empty |
| `S3_ACCESS_KEY_ID` | `s3_access_key_id` | empty |
| `S3_ACCESS_KEY_SECRET` | `s3_access_key_secret` | empty |
| `S3_USE_SSL` | `s3_use_ssl` | `true` |
| `S3_CUSTOM_DOMAIN` | `s3_custom_domain` | empty |
| `EMAIL_PROVIDER` | `email_provider` | `log` |
| `EMAIL_FROM` | `email_from` | empty |
| `EMAIL_FROM_NAME` | `email_from_name` | `Feed Chamber` |
| `EMAIL_SMTP_HOST` | `email_smtp_host` | empty |
| `EMAIL_SMTP_PORT` | `email_smtp_port` | `587` |
| `EMAIL_USERNAME` | `email_username` | empty |
| `EMAIL_PASSWORD` | `email_password` | empty |
| `EMAIL_USE_TLS` | `email_use_tls` | `false` |
| `EMAIL_API_KEY` | `email_api_key` | empty |

### Time zone

There is no time-zone variable: check-in dates and reporting windows follow the process time
zone. Pin it in the container when that matters:

```yaml
environment:
  TZ: Asia/Shanghai
```

## Frontend (build time)

The public API base URL is injected at build time; same-origin deployments (nginx proxying
`/api/v1`) need nothing.

| Variable | Default | Notes |
|----------|---------|-------|
| `NEXT_PUBLIC_API_URL` | empty (same origin) | API base URL for split-domain deployments |
| `DOCS_SITE` | `https://noelorin.github.io` | Docs site only: the Pages site origin |
| `DOCS_BASE` | `/` | Docs site only: project-page path prefix |

## Startup validation

`config.Validate()` runs at boot and **exits immediately** on any violation:

1. `DB_TYPE` must be `sqlite`, `mysql`, or `postgres`
2. `mysql` / `postgres` require `DB_DSN`
3. `NATS_URL` must not be empty
4. With `METRICS_ENABLED=true`, `METRICS_PORT` must be within 1–65535

:::tip[Container deployments]
`deploy/docker-compose.yml` already wires Postgres, NATS, the upload volume, and nginx. The only
value worth overriding is `ADMIN_PASSWORD`. See
[Install with Docker Compose](/en/install/docker-compose/).
:::
