---
title: Local Development
description: Run the Go backend, the React frontend, and an external NATS on your machine
---

Local mode differs from Compose in two ways: **there is no nginx** (each app uses its own
port) and **the database defaults to SQLite** (no Postgres needed). Everything else is the
same — especially NATS, which is not optional.

## Prerequisites

| Requirement | Version | Check |
|-------------|---------|-------|
| Go | 1.25+ | `go version` |
| Node.js | 22+ | `node -v` |
| pnpm | 10+ | `pnpm -v` |
| NATS Server | 2.10+, **JetStream enabled** | `nats-server --version` |

Docker works for NATS too:

```bash
docker run --rm -p 4222:4222 -p 8222:8222 nats:2.10-alpine -js -m 8222
```

## 1. Install dependencies

```bash
# Run from the repository root
pnpm install
```

## 2. Configure the backend

The backend reads `.env` and `.env.<env>` from `app/server/`, with the latter overriding.

```dotenv
PORT=8080
DB_TYPE=sqlite
NATS_URL=nats://127.0.0.1:4222
LOG_LEVEL=debug
LOG_FORMAT=text
```

- Empty `DB_DSN` puts SQLite at `app/server/db/app.db`
- For Postgres: `DB_TYPE=postgres` and `DB_DSN="host=127.0.0.1 user=… dbname=… port=5432 sslmode=disable"`
- The first boot seeds an admin; set `ADMIN_PASSWORD=admin123` to know it up front

## 3. Start the backend

```bash
cd app/server
go run main.go -e dev
```

A healthy start looks like:

```text
level=info msg="seeded 61 permissions"
level=info msg="permission cache loaded"
level=info msg="server listening" port=8080
```

`go run main.go dev` fails with `unknown command` — the environment is selected only via `-e`.

## 4. Start the frontend

In a second terminal, from the repository root:

```bash
pnpm dev:web
```

The frontend talks to `http://localhost:8080` by default. Open the URL printed by vinext
(usually <http://localhost:3000>) and sign in as `admin` with the password from step 2.

## 5. Verify

| Check | Expected |
|-------|----------|
| `curl -s localhost:8080/api/v1/health` | `code: 0` |
| Open the frontend timeline | Posts render |
| Sign in → `/admin` | Dashboard renders |
| Publish a post | It appears immediately (`async` moderation) |

## Everyday commands

```bash
pnpm dev:server      # backend
pnpm dev:web         # frontend and workspace packages
pnpm dev:docs        # this documentation site
bash feedchamber_test.sh -all   # API regression, backend must be running
pnpm check           # format, lint, and type checks
```

## Reset local data

```bash
rm -f app/server/db/app.db
```

The next boot re-seeds roles, permissions, spaces, tags, and the admin account.

## Common failures

| Symptom | Cause | Fix |
|---------|-------|-----|
| `NATS_URL is required` | NATS is not running | Start `nats-server -js` first |
| `failed to initialize database` | Bad DSN or Postgres is down | Test the connection with `psql` |
| Frontend loops on 401 | Backend down or wrong port | Check `/api/v1` responses in the browser |
| Cookie not stored | Not using `localhost` | Use `localhost`, or set `MASTER_COOKIE_SECURE=false` |
| Port already in use | 8080 or 3000 taken | `PORT=8081`; check vinext's output for the frontend |

More in [Troubleshooting](/en/operations/troubleshooting/).
