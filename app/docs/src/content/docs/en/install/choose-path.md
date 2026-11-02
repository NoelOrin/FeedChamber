---
title: Choose a Deployment
description: Docker Compose, local development, or your own orchestration — how to choose
---

Feed Chamber is a backend, a frontend, and two stateful dependencies (a database and NATS).
The deployment path changes how those processes run, not what the product can do.

## Three paths

| Path | Best for | You own | Guide |
|------|----------|---------|-------|
| **Docker Compose** | A single host, NAS, or VM; fastest path | A machine with Docker | [Install with Docker Compose](/en/install/docker-compose/) |
| **Local development** | Changing code, debugging APIs, building UI | Go / Node / pnpm plus a NATS server | [Local development](/en/install/local/) |
| **Your own orchestration** | Existing Kubernetes, Traefik, Cloudflare, or corporate proxy | Database, NATS, TLS, process supervision | [Production checklist](/en/install/production/) |

All three run the same backend binary and the same frontend build. There is no "lite" or
"enterprise" edition.

## How to decide

**Start with Compose.** It wires Postgres, NATS with JetStream, the backend, the frontend, and
nginx, including health checks, named volumes, and an internal metrics port. Split it up later
if you need to.

**Use local development only when you change code.** There is no nginx, the two apps listen on
their own ports, and cookies/CORS behave differently from production. Do not use it to debug
"login breaks after deploy".

**Bring your own orchestration when you already have a cluster.** The backend is stateless
(sessions live in the database plus NATS KV) and scales horizontally, but it will not start NATS
or the database for you.

## Three things that hold on every path

1. **NATS must have JetStream enabled.** Startup fails when `NATS_URL` is empty; without
   JetStream the KV bucket cannot be created and session caching, rate limiting, and settings
   caches stop working.
2. **The first boot seeds an administrator.** The username is `admin`, the password comes from
   `ADMIN_PASSWORD`, and a random one is printed if unset. The first login forces a password
   change.
3. **Uploads default to local disk.** For multiple replicas, mount shared storage or switch to
   S3 under `/admin/settings?tab=storage` (restart required).

## Sizing

For a small community (tens of daily users, mostly text), **2 vCPU / 2 GB RAM / 20 GB disk** is
usually enough; add disk if you host many images or keep many backups. Decide from metrics, not
guesses: p95 of `feedchamber_http_request_duration_seconds`,
`feedchamber_db_pool_wait_count_total`, and `feedchamber_eventbus_queue_depth` — see
[Observability](/en/operations/observability/).

## Next

- One host → [Install with Docker Compose](/en/install/docker-compose/)
- Changing code → [Local development](/en/install/local/)
- Going public → [Production checklist](/en/install/production/)
