---
title: API Conventions
description: RPC-style paths and methods, the response envelope, pagination, sorting, IDs and timestamps, retries and versioning
---

Every business endpoint lives under `/api/v1` and follows one set of rules. Parameter details
are in the [endpoint index](/en/reference/endpoints/) and the OpenAPI spec.

## Choosing a method

The test is whether the request can change server state:

| Case | Method | Example |
|------|--------|---------|
| Create, delete, state change | `POST` | `POST /api/v1/posts/create` |
| Lists, pagination, filters | `POST` + `/list` | `POST /api/v1/posts/list` |
| Reads, file bytes, protocol endpoints | `GET` | `GET /api/v1/posts/{id}` |

`PUT`, `PATCH` and `DELETE` do not exist for business endpoints, and a resource has exactly one
read method. Of the 314 operations in the spec, 278 are `POST` and 36 are `GET`.

## Path verbs

| Segment | Meaning | Example |
|---------|---------|---------|
| `/list` | List and paginate | `POST /notifications/list` |
| `/create` | Create | `POST /posts/create` |
| `/{id}/update` | Update | `POST /posts/{id}/update` |
| `/{id}/delete` | Delete | `POST /posts/{id}/delete` |
| `/{id}/status` or `/{id}/<verb>` | State change, other actions | `POST /posts/{id}/pin` |

Read-only `GET` endpoints take no verb suffix (`/posts/{id}/detail`, `/checkin/status`);
nesting expresses ownership.

## Parameters

All business parameters of a `POST` go in the JSON body, never the query string. `GET` uses
the query for filtering and pagination only. List endpoints take `page`, `page_size` and
optionally `start_time` / `end_time` (RFC 3339 or `YYYY-MM-DD`); the names are **snake_case**,
so `pageSize` is silently ignored. Send `{}` when you have no filters: most list endpoints
tolerate an empty body, but `/posts/list` parses JSON directly and returns `2001`.

Limits: 1 MB request body (10 MB on `/api/v1/upload`) and a 30 s soft timeout returning
`5004`; SSE endpoints are exempt.

## Response envelope

```json
{ "code": 0, "msg": "success", "data": {} }
{ "code": 0, "msg": "success", "data": { "items": [], "total": 128 } }
```

On success `code` is always `0` and `msg` is the literal `success`; on failure `data` is
`null`. **Branch on `code === 0`**, never on the HTTP status — see
[error codes](/en/reference/errors/). Paginated payloads carry `items` and `total` inside
`data`, where `total` counts all matching rows, not the current page; `GET` lists use the
same shape.

## Pagination and sorting

`page` defaults to 1, `page_size` to 20, and `page_max_size` caps a page at 100. Out-of-range
`page_size` values **fall back to 20** rather than being clamped: `page_size: 500` returns 20
rows, so ask for exactly 100 to get 100. A huge page number yields an empty list, not an error.

`sort_by` picks the order; unknown values fall back to the default instead of failing:

| Value | Order |
|-------|-------|
| absent or `latest` | pinned first, then `last_active_at` / `created_at` descending |
| `hot` | pinned first, then likes + comments ×2 + views/10 descending |
| `most_commented` | pinned first, then comment count descending |

## IDs and timestamps

Content and user IDs are **ULID strings**: 26 Crockford Base32 characters whose first 10
encode a millisecond timestamp, so lexicographic order approximates creation order. A few
auto-increment tables still use integers — `POST /api-keys/{id}/revoke` and
`POST /sessions/{id}/revoke` take the database primary key.

Entity fields (`created_at`, `updated_at`) are **RFC 3339 strings**; credential and security
records (API keys, sessions) use **Unix seconds**. Both appear in the same API, so decide per
object instead of calling `new Date(x)` everywhere.

## Idempotency and retries

There is no `Idempotency-Key` mechanism, so writes are **not idempotent**: replaying
`POST /posts/create` creates two posts.

| Situation | Action |
|-----------|--------|
| `401` (`1003` / `1014`) | call `POST /auth/refresh`, retry once with the new sid |
| `429` (`4001`) | back off exponentially; do not spin |
| `5xx` (`5001` / `5004`) | reads may retry; for writes, read state first |
| Likes, favorites, follows | toggle semantics — a retry undoes the action |

After a write times out, query by business key (for example `/posts/list` with `author_id`)
before retrying.

## Versioning and compatibility

The version lives only in the path prefix `/api/v1`: no header negotiation, no deprecation
window. New capabilities get new paths; responses only gain fields, so clients must ignore
unknown ones; protected routes are **deny-by-default** (an unmapped route is a 403), so a new
endpoint can never silently become public. Three families skip the envelope: `POST /mcp` speaks JSON-RPC 2.0 (see
[MCP integration](/en/api/mcp/)); `/api/v1/oauth/token`, `/api/v1/sso/token`,
`/api/v1/sso/userinfo` and `/.well-known/*` follow the OAuth 2.0 / OIDC wire format (see
[authentication](/en/api/authentication/)); crawler files (`/sitemap*.xml`, `/robots.txt`,
`/latest.rss`) are served from the app root as XML or text.

## Anti-patterns and failures

Never `PUT`/`PATCH`/`DELETE` (`POST /spaces/{id}/update` instead), never a write over `GET`
(`GET /posts/{id}/delete`), never filters in the query string
(`POST /reports/list?status=pending`), never camelCase paging (`pageSize`).

| Symptom | Cause | Fix |
|---------|-------|-----|
| Lists always return 20 rows | sent `pageSize`, or hit the cap fallback | use `page_size`, at most 100 |
| `2001` | empty body, misspelled field, wrong type | send at least `{}`; check the schema |
| `4002` / `403` | oversized body / missing permission | presigned upload / check the permission map |
