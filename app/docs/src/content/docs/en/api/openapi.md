---
title: OpenAPI
description: Where the spec comes from, how to regenerate it, the runtime Swagger UI, the downloadable copy, generated clients and the CI staleness check
---

The API spec is a **build artifact**, not a hand-written document. The single source of truth is
the swag annotations on handlers; `make swag` compiles them into JSON, and `openapi.json` at the
repository root is a copy of that output. Today it covers **313 paths and 314 operations** (278
`POST`, 36 `GET`), 66 definitions and 49 tags.

## Regenerating

```bash
cd app/server
make swag-install        # go install github.com/swaggo/swag/cmd/swag@latest
make swag
```

`make swag` does two things: it runs
`swag init -g main.go -o docs --parseInternal` to scan handler annotations into
`app/server/docs/` (`swagger.json`, `swagger.yaml`, `docs.go`), then copies
`docs/swagger.json` to `openapi.json` at the repo root.

Both outputs are **committed**. Changing an endpoint without committing them lets the docs drift
from the implementation with no compile error — which is why CI checks it (below).

## What the spec contains

| Field | Value |
|-------|-------|
| `swagger` | `"2.0"` (Swagger 2.0, not OpenAPI 3) |
| `info.title` | `Feed Chamber API` |
| `info.version` | `1.0` |
| `host` | `localhost:8080` |
| `basePath` | `/api/v1` |
| Security | `BearerAuth`: `apiKey` in `header`, named `Authorization` |

`basePath` is `/api/v1`, so paths in the spec are **prefix-free** (`/posts/list`, not
`/api/v1/posts/list`); generated clients add the prefix automatically, which also puts `/health`
at `/api/v1/health`. One trap: the crawler endpoints (`/sitemap*.xml`, `/robots.txt`,
`/latest.rss`) are annotated too, but they are served from the **app root**, so a generated
client would prefix them and get 404s. Handle those separately.

`BearerAuth` means "put `Bearer <sid or ak_...>` in `Authorization`" — sessions and API keys share
one header and differ only by value; see [authentication](/en/api/authentication/).

## Three outlets

| Outlet | Location | Use |
|--------|----------|-----|
| Repo file | `openapi.json` | client generation, API tooling, diffing |
| Runtime UI | `http://localhost:8080/swagger/` | try calls by hand; reads `/swagger/doc.json` |
| Docs download | site root `/openapi.json` | hand readers the current spec |

The UI is served by the app itself, so no extra process is needed. The docs copy is produced by
`prebuild` / `predev` running `scripts/sync-openapi.mjs`, which copies the repo file into
`app/docs/public/` (that copy is not committed), so the two can never drift.

## Generating a client

The spec is Swagger 2.0, which mainstream generators accept:

```bash
npx @openapitools/openapi-generator-cli generate \
  -i openapi.json -g typescript-fetch -o ./src/api
```

Two things must be fixed afterwards or it works locally and fails in production:

- **Base URL**: `host` is `localhost:8080`; point it at your domain (browsers usually want the
  relative `/api/v1`).
- **Auth**: `BearerAuth` is an `apiKey` definition, so pass the value with the `Bearer ` prefix.

The generated models cover only the 66 entries in `definitions`. The `{items,total}` paging
envelope and the `{code,msg,data}` error envelope are conventions the generator knows nothing
about.

## CI staleness check

```bash
cd app/server
make swag-check
```

It runs `make swag` and then `git diff --quiet`: if `docs/` or `openapi.json` changed, it fails and
prints the diff. `quality.yml` installs the pinned `swag` version and runs this, so "edited an
annotation without regenerating" is caught on the pull request rather than in production docs.

Regenerate when you add or remove a route, change a `@Param` / `@Success` / `@Router` / `@Tags` /
`@Security` line, or change a request/response struct field or JSON tag. Internal service changes
and comment-only edits need nothing, and neither does `/mcp` — it is a protocol endpoint with no
swag annotation and deliberately absent from the spec, since JSON-RPC discovers its tools at
runtime (see [MCP integration](/en/api/mcp/)).

## Verification and failures

`make swag-check` should be silent. `curl -s http://localhost:8080/swagger/doc.json | jq '.paths |
length'` should print 313, and the built docs site should expose `openapi.json` at its root — the
`prebuild` step fails loudly if the repo file is missing.

| Symptom | Cause | Fix |
|---------|-------|-----|
| `swag: command not found` | CLI not installed | `make swag-install` |
| CI reports "swagger docs are stale" | annotations changed, artifacts not committed | run `make swag` and commit both |
| `/swagger/` renders blank | proxy eats the static assets | forward `/swagger/` to the backend |
| generated client 404s everywhere | missing `basePath`, or SEO paths prefixed | add `/api/v1` only for business endpoints |
| generated types miss fields | the field is unexported or lacks a JSON tag | add the tag and regenerate |
