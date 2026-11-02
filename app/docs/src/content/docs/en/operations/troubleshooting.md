---
title: Troubleshooting
description: Symptom-first fixes for startup, NATS, the database, login, permissions, uploads, SSE, email, OAuth, moderation, XP, metrics and backups
---

Search by symptom, not by module. Each item is symptom → cause → fix; under Docker, prefix
commands with `docker compose -f deploy/docker-compose.yml exec backend`.

## Startup and dependencies

### The process exits immediately
**Symptom** it stops at once, last line `invalid config: …` or `failed to initialize database`.\
**Cause** boot-time config errors are fatal, never degraded: `NATS_URL is required…`, `DB_TYPE is required`, `unsupported DB_TYPE "…"`, `DB_DSN is required when DB_TYPE=…`, `METRICS_PORT must be between 1 and 65535…`.\
**Fix** set the variable in `.env` or `.env.<env>` (the latter wins). `端口 8080 已被占用` is a pre-listen probe.

### Cannot reach NATS
**Symptom** `初始化 KV Store 失败: nats connect: …` or `nats kv bucket "feedchamber_kv": …`.\
**Cause** the first is connectivity; the second usually means the server runs without JetStream (`-js`).\
**Fix** start NATS with `-js` and reach port 4222. A later partition does **not** exit the
process (reconnects are unlimited), so only metrics show it: `feedchamber_nats_connected`
at 0 means that client is unusable.

### Database locks or a saturated pool
**Symptom** SQLite reports `database is locked`; PostgreSQL times requests out while `/health` still answers slowly.\
**Cause** SQLite allows one writer, so concurrent transactions collide (the default DSN sets `busy_timeout=5000`); or the pool is full of slow queries.\
**Fix** read the pool and the slow-query ratio first. `DB_MAX_OPEN_CONNS` defaults to 25 and raising it only moves the queue into the database.

```bash
curl -s localhost:8080/api/v1/health | grep db_pool
curl -s localhost:9091/metrics | grep -E 'feedchamber_db_pool_in_use|feedchamber_db_slow'
```

## Login and permissions

### The admin password is unknown
**Symptom** you cannot get into the admin area and nobody wrote the initial password down.\
**Cause** `ADMIN_PASSWORD` is only used on the **first** seed — an existing `admin` user skips the whole block. Left empty, a random password is generated and logged as a WARN.\
**Fix** grep the log; if it is gone, let another account with `user:update` reset it.

```bash
docker compose -f deploy/docker-compose.yml logs backend | grep -i '管理员密码'
curl -s -X POST localhost:8080/api/v1/auth/reset_password \
  -H 'Content-Type: application/json' -H "Authorization: Bearer $SID" \
  -d '{"username":"admin","new_password":"new-secret"}'
```

If nobody can log in, edit the database — `credentials` holds a bcrypt hash per `user_id`.

### Login returns 401
**Symptom** `/auth/login` returns 1001 / 1010 / 1011 / 1015, or login succeeds and the next call fails.\
**Cause** `1001` no credential, `1010` wrong password, `1011` no such user, `1015` banned. A success followed by 401 usually means the cookie is `Secure` while you browse over HTTP, so it was never stored.\
**Fix** branch on `code` and read `msg`. For the cookie, terminate TLS or set `MASTER_COOKIE_SECURE=false` locally. Password change, role change and bans all delete session rows, so old sids stop working instantly.

### 403 or an unexpected 404
**Symptom** `1013 FORBIDDEN`, or a 404 where you expected 403.\
**Cause** `1013` is a missing permission code. A **banned** caller gets 404 (`USER_BANNED`) on purpose, so probes cannot distinguish "no permission" from "no resource". An API key's scope is its ceiling: a code outside the scope fails no matter how high the role is.\
**Fix** check the role matrix and the credential type (session, `X-API-Key`, `Bearer ak_…`). The admin pages are Roles → permission matrix and Users → mute/ban.

## Content and files

### Uploads fail
**Symptom** 413, `文件大小超过 10MB 限制`, or `不允许的文件类型`.\
**Cause** two layers: the body limit (`/api/v1/upload` 10MB, everything else 1MB) and the settings `upload_max_size` (10MB) plus `upload_allowed_types` (four image types, MIME sniffed from content, not the extension).\
**Fix** 4002 is `BODY_TOO_LARGE` (first layer); business rejections are 2001 with a message. Adjust both settings in the admin area.

### Images return 404
**Symptom** post images break and `/api/v1/upload/file/<key>` answers 404.\
**Cause** the upload volume is not mounted, the directory was lost with a recreated container, or `uploads` has a row whose file is gone.\
**Fix** the working directory is always `app/server`, so local storage resolves to
`app/server/uploads`. `down -v` deletes the volume — by far the most common cause.

### SSE streams keep dropping
**Symptom** notifications or realtime pushes drop about every 60 seconds and the client reconnects forever.\
**Cause** an idle SSE stream sends no bytes, so any intermediary read timeout cuts it (nginx defaults to 60 s); buffering turns pushes into batches.\
**Fix** keep a dedicated nginx location for SSE (buffering off, read timeout 3600 s) and confirm the handlers still send heartbeats (the notification stream writes `: keepalive` every 15 s). The connection gauge also drops on a normal page close; `feedchamber_sse_write_failures_total` is the truncation signal.

### Mail never arrives
**Symptom** registration and password reset report success, but no mail shows up.\
**Cause** `email_provider` defaults to `log`, which only writes log lines. With SMTP configured, check host, port and TLS mode (465 implicit, 587 STARTTLS), plus the limits (`email_daily_limit` 10, `email_resend_cooldown` 60 s).\
**Fix** confirm the provider is not `log`, then grep the backend log for `mail`, `smtp` or
`verification` lines.

### OAuth callback fails
**Symptom** `invalid or expired state`, or you land back on the login page after authorizing.\
**Cause** the state lives in KV and is **single use** (5 minutes for social login, 10 for gateway OAuth); the callback URL must match the registered one exactly.\
**Fix** re-check client id, secret and callback, confirm `site_url` is externally reachable, then run the flow once from the start.

## Operations and metrics

### Moderation appears stuck
**Symptom** a post is not visible and is not in the review queue either.\
**Cause** the initial status follows `moderation_mode`: `off` publishes, `async` (default) publishes then decides asynchronously and moves hits to `pending`, `external` calls an external service first, `manual` sends everything to `pending`. `pending` content stays out of public lists.\
**Fix** approve or reject it in the admin moderation page. Changing the mode affects new posts only; the existing queue is not emptied.

### XP does not grow
**Symptom** you post and check in, but `total_xp` stays put.\
**Cause** XP is derived from behaviour aggregates and only `TrustService.Recompute` writes the snapshot. Editing rules changes the **next** recompute only; the daily full recompute first runs 2 minutes after boot, then every 24 hours. Users with a manually set level (`level_manual`) are never pulled back by a recompute.\
**Fix** reconcile by hand, then read the XP breakdown on the user's growth tab.

```bash
go run main.go -e prod recompute-xp      # recomputed 1234 users
```

### The metrics port refuses connections
**Symptom** `curl localhost:9091/metrics` fails while the site is fine.\
**Cause** `METRICS_ENABLED=false`; the port is taken (the listener logs `metrics listener on :9091 unavailable` and carries on); or you are hitting a container port from the host — compose uses `expose`, not `ports`.\
**Fix** scrape from inside the container network (`wget -qO- http://127.0.0.1:9091/metrics`
inside the backend), and never route 9091 through nginx — it has no authentication.

### Backups fail
**Symptom** `backup --out` errors, or the output only holds `db-note.txt`.\
**Cause** `backup` is a local-site backup: it copies the SQLite file and writes an uploads inventory. It does not dump MySQL or PostgreSQL, and it does not package the uploaded files.\
**Fix** use the engine's own tool, back the upload directory up separately, and rehearse a
restore on a copy first:

```bash
go run main.go -e prod backup --out /backup/2026-10-30
docker compose -f deploy/docker-compose.yml exec postgres \
  pg_dump -U feedchamber feedchamber > backup-2026-10-30.sql
```
