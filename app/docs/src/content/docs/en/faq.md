---
title: FAQ
description: Startup, login, uploads, notifications, API, and MCP questions, organised by symptom
---

Organised by what you see. Each entry gives a cause and a fix; for a systematic path see
[Troubleshooting](/en/operations/troubleshooting/).

## Startup

### The backend exits immediately, mentioning NATS

`NATS_URL` is required and there is no embedded NATS:

```bash
nats-server -js
NATS_URL=nats://127.0.0.1:4222 go run main.go -e dev
```

With Compose, make sure the `nats` container is healthy first; the bundled file already has the
dependency, but custom orchestration does not.

### `go run main.go dev` says unknown command

The environment is selected with `-e`, not a subcommand:

```bash
go run main.go -e dev      # correct
go run main.go dev         # unknown command
```

### Port conflicts

Backend `8080`, frontend `3000`, docs site `4321`, Compose nginx `80`. Override with `PORT` /
`HTTP_PORT`; the frontend reports its port on startup.

### `unsupported DB_TYPE` or `DB_DSN is required`

`DB_TYPE` accepts `sqlite`, `mysql`, and `postgres`; the latter two need a full `DB_DSN`.
SQLite needs none and defaults to `app/server/db/app.db`.

## Accounts

### What is the admin password

The first boot seeds `admin` with `ADMIN_PASSWORD`; when unset, a random password is printed:

```bash
docker compose -f deploy/docker-compose.yml logs backend | grep -i admin
```

### Signed out immediately, or the cookie is not stored

- Plain HTTP while `MASTER_COOKIE_SECURE=true`: use `false` locally, keep `true` with TLS in production
- Mixing `127.0.0.1` and `localhost`: the cookie domain differs, pick one
- The proxy does not forward `X-Forwarded-Proto`, so the backend thinks it is not HTTPS

### Forced password change on first login

Intentional. The seeded password is single-use, and all its sessions are revoked after the change.

### Locked out of every admin account

Two options. If another administrator can sign in, reset via the API (requires `user:update`):

```bash
curl -s -X POST http://localhost:8080/api/v1/auth/reset_password \
  -H "Authorization: Bearer $SID" \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","new_password":"a-new-strong-password"}'
```

The target user's sessions are revoked immediately. If nobody can sign in, back up and write a
new bcrypt hash directly into the database.

:::caution[Do not delete the credentials row to "reset"]
That only makes the account unusable; it triggers no reset flow. The email reset needs a
reachable mailbox, which the admin account may not have.
:::

## Content and moderation

### The post was created but is not listed

Default `async` moderation publishes first and reviews in the background. If the site is set to
`manual`, every new post is `pending` until approved.

### Post detail returns 403

The space is `login` or `level` gated and the viewer's level is too low. The response names the
required and current level; the author and reviewers with `report:manage` bypass it.

### XP does not grow, or a level dropped

XP is derived: deleting a post, unliking, un-favouriting, un-featuring, or a rejected review
removes it. After changing XP rules, recompute existing users:

```bash
go run main.go -e prod recompute-xp
```

### Notifications are duplicated or missing

One comment yields at most one notification (author > mention > watch, de-duplicated). If they
are missing, check whether the proxy cuts `/api/v1/notifications/stream`; idle streams need a
keepalive and a longer read timeout. See [Realtime](/en/api/realtime/).

## Uploads and images

### Upload returns 413

The proxy's `client_max_body_size` is smaller than the upload limit. The backend defaults to
`upload_max_size=10485760` (10 MiB); nginx defaults to 1 MiB:

```nginx
client_max_body_size 10m;
```

### Images return 404

With local storage, a recreated container without the `feedchamber_uploads` volume loses files.
With S3, confirm `storage_provider=s3` and the credentials. Switching storage needs a restart.

## API

### Why are PUT / PATCH / DELETE missing

The API is RPC style: writes are `POST` with an action suffix (`/create`, `/{id}/update`,
`/{id}/delete`). Do not guess REST paths.

### 401 vs 403

- `1001/1002/1003/1014`: missing, malformed, expired, or revoked credential → sign in or refresh
- `1013` / HTTP 403: authenticated but not allowed
- A banned account asking for protected resources gets **404**, deliberately hiding existence

### Pagination parameters

Always `page` and `page_size` (**snake_case**). A `page_size` below 1 or above
`page_max_size` (default 100) falls back to 20 rather than being clamped; `pageSize` is ignored.

## MCP

### `/mcp` says not enabled

Requires `mcp_enabled=true`, `api_key_enabled=true`, and a valid API key. Both switches default
to off.

### The AI can read but not write

Write tools are disabled by default and enabled individually in the admin MCP page. Tools listed
in `mcp_disabled_tools` are also blocked.

### Tool errors vs protocol errors

Tool failures use `isError: true` in the result; the JSON-RPC `error` object is reserved for
protocol problems (unknown method, invalid params). Handle the two layers separately.
