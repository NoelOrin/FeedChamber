---
title: Install with Docker Compose
description: Run the full stack on a single host with Compose — ports, volumes, passwords, and upgrades
---

The stack lives in `deploy/docker-compose.yml` and has five services:

| Service | Role | Exposed |
|---------|------|---------|
| `nginx` | Single entry point: `/` frontend, `/api/v1` backend, `/mcp` | Yes (`HTTP_PORT`, default 80) |
| `frontend` | React 19 SSR process (includes `/admin`) | No, compose network only |
| `backend` | Go / Iris API | No; metrics on 9091 are internal too |
| `postgres` | Primary database (PostgreSQL 16) | No |
| `nats` | JetStream: KV and event bus | No |

A `nats-exporter` service converts NATS metrics to Prometheus format; it is also internal.

## Prerequisites

- Docker Engine 24+ with Compose v2 (`docker compose version` works)
- Network access to `postgres:16-alpine`, `nats:2.10-alpine`, and `nginx:alpine`
- Port 80 free, or a different `HTTP_PORT`

## Install

```bash
git clone https://github.com/NoelOrin/FeedChamber.git
cd FeedChamber

ADMIN_PASSWORD='choose-a-strong-one' HTTP_PORT=8080 \
  docker compose -f deploy/docker-compose.yml up -d --build
```

Slow dependency downloads can be routed through mirrors:

```bash
GOPROXY=https://goproxy.cn,direct \
NPM_REGISTRY=https://registry.npmmirror.com \
  docker compose -f deploy/docker-compose.yml build
```

## Overridable variables

| Variable | Default | Notes |
|----------|---------|-------|
| `HTTP_PORT` | `80` | Host port nginx binds |
| `ADMIN_PASSWORD` | empty | Seeded admin password; empty means random and logged |
| `POSTGRES_USER` / `POSTGRES_PASSWORD` / `POSTGRES_DB` | `feedchamber` | Must match `DB_DSN` |
| `SITE_URL` | empty | Public origin for sitemap / canonical / OG (the `site_url` setting wins) |
| `MASTER_COOKIE_SECURE` | `true` | Session cookie `Secure` flag; set `false` only for plain-HTTP debugging |
| `LOG_LEVEL` / `LOG_FORMAT` / `LOG_OUTPUT` | `info` / `json` / `stderr` | Backend logging |
| `TZ` (postgres) | `Asia/Shanghai` | Keep aligned with `DB_DSN`'s `TimeZone` |

## Verify

```bash
docker compose -f deploy/docker-compose.yml ps
# five services running; postgres, nats, backend, frontend also healthy

curl -s http://localhost:8080/api/v1/health
# {"code":0,"msg":"success","data":{...}}

curl -sI http://localhost:8080/ | head -1
# HTTP/1.1 200 OK
```

Read the generated admin password:

```bash
docker compose -f deploy/docker-compose.yml logs backend | grep -i admin
```

## Where the data lives

| Volume | Contents | Backup |
|--------|----------|--------|
| `feedchamber_pg` | PostgreSQL data directory | `pg_dump` or a volume snapshot |
| `feedchamber_nats` | JetStream (KV plus event queues) | Rebuildable; losing it drops session caches and rate-limit windows |
| `feedchamber_uploads` | Local uploads | File sync or object storage |

Deleting the volumes deletes the site:

```bash
# Destructive: removes containers and data
docker compose -f deploy/docker-compose.yml down -v
```

## Upgrades

```bash
git pull
docker compose -f deploy/docker-compose.yml up -d --build
```

The backend runs `AutoMigrate` on boot, which adds new columns and tables. Destructive changes
are covered in [Upgrades and migrations](/en/operations/upgrades/). Back up first:

```bash
docker compose -f deploy/docker-compose.yml exec postgres \
  pg_dump -U feedchamber feedchamber > backup-$(date +%F).sql
```

## Common failures

| Symptom | Cause | Fix |
|---------|-------|-----|
| `port is already allocated` | Port 80 taken | Re-run with `HTTP_PORT=8080` |
| Backend restart loop | Postgres or NATS not healthy | Wait 30s; then check `logs backend` |
| Logged out immediately | Plain HTTP with `MASTER_COOKIE_SECURE=true` | Use `false` locally; keep TLS in production |
| Images 404 | Upload volume missing or wiped | Check `feedchamber_uploads`; avoid `down -v` |
| `/mcp` returns 404 | `mcp_enabled` is off | Enable it in the admin settings |

More in [Troubleshooting](/en/operations/troubleshooting/).
