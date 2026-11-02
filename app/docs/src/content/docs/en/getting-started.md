---
title: Quick Start
description: Run Feed Chamber with Docker Compose in five minutes, then sign in and publish your first post
---

The only goal of this page: **five minutes from now you are signed in to the admin panel and
have published a post**.

You do not need the architecture pages or an understanding of XP or gate rules first. Get the
site running, then follow the navigation.

## What you need

| Path | Prerequisites |
|------|---------------|
| Docker Compose (recommended) | Docker Engine with Compose v2 |
| Local development | Go 1.25+, Node.js 22+, pnpm 10+, and a NATS server with JetStream |

Both paths expose the same features. If you already run Kubernetes or your own reverse proxy,
see [Choose a deployment](/en/install/choose-path/).

## Five minutes

```bash
git clone https://github.com/NoelOrin/FeedChamber.git
cd FeedChamber
docker compose -f deploy/docker-compose.yml up -d --build
```

The first build pulls images and compiles both applications, usually 3–8 minutes. Later starts
take seconds.

The stack has five services: Postgres, NATS, backend, frontend, and nginx. Data lives in three
named volumes (`feedchamber_pg`, `feedchamber_nats`, `feedchamber_uploads`), so recreating
containers does not lose it.

## Get the admin account

The first boot seeds a single `admin`:

```bash
docker compose -f deploy/docker-compose.yml logs backend | grep -i admin
```

If you set `ADMIN_PASSWORD`, that is the password. Otherwise one is generated and printed with
a `WARN` line.

## Sign in and change the password

1. Open <http://localhost/login>
2. Sign in as `admin` with the password above
3. You are redirected to `/admin/first-change-password`; set a new password
4. Continue to `/admin`

The forced password change is intentional: the seeded password is single-use.

## Create a space and publish a post

1. Go to `/admin/spaces` and create a space (or use a seeded one)
2. Return to `/timeline` and choose **New post**
3. Write a title and body, then submit

The default moderation mode is `async`: the post is public immediately, rules run in the
background, and only a hit moves it to review. To require review for everything, set
`moderation_mode` to `manual` under `/admin/settings?tab=site`.

## Verify

| Check | Expected |
|-------|----------|
| `curl -s localhost/api/v1/health` | `{"code":0,...}` (HTTP 2xx) |
| Open <http://localhost> | Timeline renders, assets return 200 |
| `curl -s localhost/sitemap.xml` | A sitemap index document |
| `docker compose ... ps` | Five services running, four healthy |

Backend logs are JSON by default (`LOG_FORMAT=json`). When you see a `level":"error"` line,
take its `trace_id` and grep for it to follow the whole request.

## If it does not come up

| Symptom | Fix |
|---------|-----|
| `port 80 already in use` | `HTTP_PORT=8080 docker compose ... up -d` |
| Backend restart loop | Check `logs backend`; Postgres and NATS may still be starting |
| Password rejected | Read it from the logs again, or reset the volumes (loses data) |
| Pages load but API returns 502 | The backend is down: `docker compose ... logs backend` |

More in [Troubleshooting](/en/operations/troubleshooting/).

## Next

- Why it is built this way → [Architecture](/en/operations/architecture/)
- Site name, registration, moderation → [Site settings](/en/configuration/site/), [Content pipeline](/en/configuration/content/)
- API or AI clients → [API conventions](/en/api/conventions/), [MCP integration](/en/api/mcp/)
