---
title: Architecture
description: How a request flows through the layers, where the code lives, and the tradeoffs you should not undo casually
---

Two audiences: maintainers reading the code, and operators deciding **which layer** a
problem belongs to.

## Request path

Every `/api/v1` request takes the same one-way chain. Layers never depend upward.

```text
HTTP Request
  └─ /api/v1 middleware
       PathBodyLimit(1MB) → RequestTimeout(30s)
       → gw.Preflight() → MaintenanceGate
     └─ protected routes  gw.Enforce()
        └─ Handler         parse input → call a service → write envelope
           └─ Service      business rules → *pkg.AppError
              └─ Repository  interface + SQL (GORM)
                 └─ Database  SQLite / PostgreSQL / MySQL
```

Routes are registered in `app/server/internal/router/`, split per domain into
`routes_*.go`. Middleware order is load-bearing: `Trace` runs before `Logger`, and
`Recover` after it — otherwise a panic produces no request log line at all.

## The gateway runs in two segments

| Segment | Mounted on | Does |
|---------|-----------|------|
| `Preflight()` | all of `/api/v1` | names the trace route, global per-IP ceiling (`rate_limit_enabled`, off by default), **optional** credential parsing |
| `Enforce()` | protected routes | required credential → ban check → permission → risk control → session activity |

Both mark their progress in the request context (a bitmask over tracing / ratelimit /
credential / enforce), so neither stage runs twice. This fixes a real defect: preflight
already covers protected routes, and running it again counted the same
`rl:ratelimit:<ip>` key twice, so with limiting enabled a 60-per-minute ceiling returned 429
at request 30.

A banned caller gets **404, not 403**: a probe cannot tell "no permission" from "no such
resource", while the authenticated caller still finds the reason in the envelope.

`/mcp` and `/.well-known/` are mounted at the app root and get neither segment — their
authentication, ban, permission and rate-limit checks happen inside `internal/mcp`.

## Code layout

```text
app/server/
├── main.go              # chdir to its own directory, then hand over to cmd.RootCmd
├── cmd/                 # Cobra: backup / import-discourse / recompute-xp
├── server/              # iris.go (boot), wiring.go, seed.go, migrate.go
├── internal/
│   ├── handler/         # HTTP layer          ├── service/    # business logic
│   ├── repository/      # interface + SQL     ├── gateway/    # auth, perms, sessions
│   ├── middleware/      # request gates       ├── container/  # graph + workers
│   ├── metrics/         # Prometheus          └── pkg/        # envelope, KV, bus
└── db/migrations/       # hand-written SQL (nothing runs these automatically)
```

## Boot sequence

`Start(env)` in `server/iris.go` is ordered, and the order is a hard constraint: load and
`Validate()` config (a bad `NATS_URL`, `DB_TYPE` or `METRICS_PORT` fails here) → open the
database and `AutoMigrate` every model (69 today) → upgrade existing rows → seed roles,
admin, settings and defaults → connect NATS and create KV plus event bus → build shared
services → assemble the container and start it → register handlers and routes → listen.

Assembly has two phases for a reason. `container.New(Deps)` only builds the object graph:
no goroutines, no subscriptions, no writes. `(*Container).Start(ctx)` subscribes to events
and runs six workers (view-count flush, pin expiry, webhook delivery, weekly digest, daily
XP recompute, motion expiry); the returned `*Runtime` shuts them down after flushing view
counts, because those live in KV and must reach SQL before KV goes away. A container that
was never started spawns nothing, which makes it safe to build inside a test.

`handler` never imports `container`. It consumes the services listed in `handler.Deps`,
mapped in `server/wiring.go` — so "which services does the HTTP layer use" needs no grep,
and a forgotten wire is a compile error rather than a runtime 500.

## One permission predicate

`internal/authz.Allowed(cred, perms, code)` is the only authorization decision point —
the gateway, route-level `RequirePermission` and MCP all call it.

| Credential | Predicate |
|------------|-----------|
| User session | role → permission table |
| API key | scope **and** role permissions (scope is the ceiling) |
| OAuth client | scope only (a client is not a user and has no role) |
| Empty permission code | authentication is enough, always allowed |
| Unknown source | denied |

Five default roles (`admin`, `moderator`, `user`, `pending_user`, `ban`) and 61 platform
permission codes; space-level permissions live in `space_role_permissions` and inherit
along the ancestor chain. Sessions and API keys must not share one predicate — otherwise a
read-only admin key walks through write endpoints.

## Tradeoffs worth keeping

- **Writes are `POST` with an action suffix**; `GET` is for reads. See [API conventions](/api/conventions/).
- **Sessions are not JWTs**: a `user_sessions` row is the authority, and logout, password
  change, role change and bans all delete it. See [Authentication](/api/authentication/).
- **Notifications are event-driven**: a service publishes `content.replied`, the
  notification service subscribes, stores and pushes over SSE. Posting does not know the
  notification tables.
- **MCP shares services with HTTP**, including the same permission, ban and limiter
  instances — a parallel implementation produces "the gateway says banned, MCP says fine".

## Verify

```bash
curl -s localhost:8080/api/v1/health        # db / kvstore / storage all "ok"
cd app/server && go test ./internal/router/ ./internal/authz/
```

The `route` log field carries the route template, so `/api/v1/posts/01J...` appears as
`/api/v1/posts/{id}`. Data and events are covered in
[Data and events](/operations/data-and-events/).
