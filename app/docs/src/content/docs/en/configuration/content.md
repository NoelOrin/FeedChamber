---
title: Content Pipeline
description: The four moderation modes, the review queue and its audit trail, reports, mutes and bans, three-level visibility, and gate rules
---

Content passes three gates before it is public: **moderation mode** decides whether a human must
look, **visibility** decides who can read, **gate rules** decide whether this user may write.

## Before you start

- `report:manage` for `/admin/moderation` and `/admin/reports`; `gate:manage` for `/admin/gates`.
- `banned:manage` / `user:update` for mutes and bans; the mode switch is in `?tab=content`.

## The four moderation modes

`moderation_mode` holds one of four values, **default `async`**.

| Mode | When it runs | Behaviour |
|------|--------------|-----------|
| `off` | Never | Published immediately; no rules, no records |
| `async` | After publishing, async | Goes live, rules run, a hit returns it to pending |
| `external` | Before publishing, sync | Calls an external service; falls back to `async` when unavailable |
| `manual` | At publish time | Waits in the queue until a human approves |

`async` only queues content — the final call belongs to a person. `external` returning `block`
fails the API call while recording it; `flag` / `replace` leave the item pending. The old
boolean `post_approval` applies only while `moderation_mode` was never stored, as `manual`.

## The review queue and its trail

`POST /api/v1/admin/moderation/pending` lists the queue;
`/moderation/{id}/approve` and `/reject` decide. Both take the **record id**, not the post id.

| Field | Contents |
|-------|----------|
| `target_type` / `target_id` | Post or comment, plus its id; `status` is `pending`/`approved`/`rejected`/`auto_pass` |
| `action` / `strategy` | `allow`/`block`/`replace`/`flag`, and origin `manual`/`async`/`external`/`gate`/`tl0` |
| `matched_rules` | Hit rules as JSON; `original_body` / `filtered_body` hold both texts |
| `reviewer_id` / `reviewed_at` | Who decided, and when |

Every pending item creates a record, or the queue would be empty.

## Reports, mutes, and bans

`POST /api/v1/reports/create` (10/min per IP) files a report, managed in `/admin/reports` with
statuses `pending`, `resolved`, `dismissed`. Reports never change content state themselves.

| Mechanism | Stored as | Effect |
|-----------|-----------|--------|
| Mute | `banned` rows, `global` or `space` | Global mutes are rejected at login (1015); existing sessions get 404 on protected endpoints |
| Ban | role `ban` | Login refused, and the role change revokes every session |

Mutes are `is_permanent=true` or temporary via `duration_minutes` / `expires_at`. Space-scoped
mutes block writes in that space; a global mute blocks protected endpoints while public content
stays readable. Use `/api/v1/user/{id}/banned` and `/unbanned`, plus
`/user/{id}/space/{space_id}/banned`.

## Three-level visibility

Spaces carry `spaces.visibility` (`public`, `login`, `level`) plus `spaces.min_level`.

| Value | Who can read |
|-------|--------------|
| `public` | Everyone, including guests (counted as TL0) |
| `login` | Signed-in users; equivalent to level 1 |
| `level` | Users at or above `min_level` |

The effective policy is the **strictest along the ancestry chain**: children inherit from
parents, multiple `level` ancestors resolve to the highest `min_level`, and `posts.min_level`
overrides by taking the higher of the two.

All six content exits share one predicate — list, detail, search, sitemap, RSS, batch — so
"invisible in the list but readable by URL" cannot happen. Detail returns 403 with "requires
level X, you are Y"; authors always see their own posts, reviewers with `report:manage` bypass
the gate, and `level` spaces stay out of the sitemap.

## Gate rules

Rules at `/admin/gates` are **priority + domain + conditions (JSON array) + action** over five
domains: `content_view`, `post_create`, `comment_create`, `external_link`, `rate_limit`, each
with an explicit default action (links default to `deny`).

| Predicates | Operators |
|------------|-----------|
| `level`, `xp`, `author_level` / `space_id`, `tag` | numeric comparisons / `eq ne in` |
| `is_author`, `is_anonymous`, `email_verified`, `banned`, `exiled`, `contains_external_link` | `eq` |
| `window_metric` | `active_days`, `post_count`, `likes_received`, `report_upheld` (numeric) |

Actions are only `allow`, `deny`, `require_review`, `rate_limit`. Evaluation runs in descending
priority, first hit wins; priorities are unique per domain, conditions AND, empty never matches.

Two link rules are seeded (TL0 denied, level ≥1 allowed), showing "default deny plus explicit
allow". The admin "simulate as user" action runs the production evaluator, so verdicts match
real requests, and every hit logs trace id, domain, action, rule, priority.

## Machine credentials

API keys and MCP tools follow the **same** mode and gate path; the forced-draft rule lives only
in [Site Customization](/configuration/customization/).

## Verify

```bash
# Publish a post and inspect its status
curl -s -X POST http://localhost:8080/api/v1/posts/create \
  -H 'Authorization: Bearer <sid>' -H 'Content-Type: application/json' \
  -d '{"title":"hello","content":"first post","space_id":""}' | jq '.data.status'
```

Under `manual` it returns `pending`; under `off` or `async`, `approved`. Read the mode with
`POST /api/v1/admin/settings/moderation_mode` — it is not a public setting.

## Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| Post succeeds but is invisible | `manual` mode, or a gate's `require_review` | Review in `/admin/moderation`; simulate the user in `/admin/gates` |
| A post disappears under `async` | Async rules hit and moved it to pending | Expected; approving restores it |
| `external` mode feels slow | It calls the service synchronously | Expected; switch back to `async` |
| A rule save says the priority is taken | Duplicate priority inside one domain | Pick another number |
