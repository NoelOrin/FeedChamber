---
title: Data and Events
description: How the tables are grouped, what lives in NATS KV, how events flow, and how notifications and caches invalidate
---

Two common misreadings during an incident: that data lives only in the database, and that
notifications are sent synchronously. Neither is true.

## Table groups

`AutoMigrate` registers 69 models at boot. By purpose they fall into five groups:

| Group | Main tables |
|-------|-------------|
| Identity | `users`, `credentials`, `user_sessions`, `user_mfa`, `api_keys`, `oauth_accounts`, `roles`, `role_permissions`, `groups`, `group_members`, `group_permissions`, `invites`, `banned`, `user_follows`, `user_blocks`, `login_histories`, `security_events` |
| Content | `posts`, `contents`, `spaces`, `tags`, `post_tags`, `drafts`, `revisions`, `uploads`, `announcements`, `topic_read_states`, `badges`, `user_badges` |
| Engagement | `content_likes`, `post_favorites`, `watches`, `notifications`, `conversations`, `conversation_members`, `messages`, `check_ins` |
| Governance | `reports`, `content_moderations`, `moderation_rules`, `moderation_configs`, `motions`, `motion_votes`, `audit_logs` |
| Config | `settings`, `meta`, `permissions`, `space_role_permissions`, `xp_rules`, `xp_streak_bonuses`, `trust_levels`, `xp_adjustments`, `gate_rules`, `gate_domain_defaults`, `webhooks`, `webhook_deliveries`, `backups`, `plugins`, `import_maps`, `mcp_servers` |

Two boundaries matter. **`AutoMigrate` only adds** — dropping columns, changing types and
moving data never happen automatically. And **the SQL scripts are not executed for you**:
files in `app/server/db/migrations/` are run by hand, and each opens with background,
whether it is optional, whether it is idempotent, and its compatibility caveats.

`users.total_xp`, `trust_level` and `level_metrics` have exactly one writer,
`TrustService.Recompute`. Anything else that updates them gets overwritten by the next
recompute.

## What NATS KV holds

KV is not a decorative cache — remove it and the site stops working (sessions, rate limits
and settings all depend on it). An empty `NATS_URL` aborts startup instead of falling back
to local memory.

| Key | TTL | Purpose |
|-----|-----|---------|
| `sess:<sid>` | remaining session lifetime | session read-through; `user_sessions` stays authoritative |
| `rl:<ip>:<policy>` | window (usually 1 min) | per-endpoint IP budget, e.g. `rl:1.2.3.4:login` |
| `rl:ratelimit:<ip>` | window | global per-IP ceiling (policy `global`) |
| `apikey_rl:<keyID>` | window | per-key API budget |
| `setting:<key>` | 60 s (miss marker 10 s) | settings cache; writes delete the key |
| `oauth:<state>` / `oauth_state:<state>` | 5 / 10 min | CSRF state for social login and gateway OAuth, single use |
| `ban:<userID>` / `ban:<userID>:<spaceID>` | 30 s | global / per-space ban cache |
| `vc:<postID>` | until flushed | view-count delta, flushed to SQL every 60 s |
| `notif:unread:<userID>` | 5 min | unread counter behind the SSE push |
| `emailcode:<purpose>:<email>` | `email_code_ttl` (600 s) | email verification code |

Keys are hex-encoded before they reach JetStream, and per-key TTL uses a companion
`<bucket>_ttl` bucket — NATS does not show you the plaintext key.

## Event types

The bus is NATS core pub/sub with **one queue subscription per event type**: each event is
consumed by exactly one instance, which then fans it out to every local subscriber. The
in-process buffer holds 100 events with 4 workers; when it is full the event is dropped and
counted in `feedchamber_eventbus_dropped_total`. Drops are permanent — the missing
notification or XP never arrives later.

| Event | Published by | Consumed by | Effect |
|-------|--------------|-------------|--------|
| `post.created` | post creation | webhooks, async moderation | with `moderation_mode=async`, decides whether the post is queued for review |
| `post.closed` | close/reopen | webhooks | external sync |
| `content.created` | new comment | webhooks | external sync |
| `content.replied` | comment or reply | notifications, webhooks | one `comment` notification |
| `content.liked` | comment like | notifications, webhooks | one `like` notification |
| `content.watched` | watched topic updated | notifications, webhooks | one `watch` notification |
| `user.created` | registration | webhooks | `user.registered` is emitted alongside as a legacy alias |
| `user.followed` | follow | notifications, webhooks | one `follow` notification |
| `user.mentioned` | @mention | notifications, webhooks | one `mention` notification |
| `report.created` | report filed | webhooks | external ticketing |
| `moderation.decided` | approve/reject | webhooks | `decision` is `approved` or `rejected` |
| `motion.created` / `motion.resolved` | motion opened / settled | notifications, webhooks | one `motion` notification on the same SSE channel |

The trace context travels in the `traceparent` header, so a consumer's log lines join the
HTTP request that published the event.

## Notification fan-out dedup

One comment can hit three fan-out paths at once: **post (or parent comment) author →
@mentions → topic watchers**. All three share a single `notified` set and honour that
priority, so **one comment produces exactly one notification**. A new fan-out step must
take the same set, or recipients start getting duplicates. Commenting on your own post
notifies nobody.

## Cache invalidation

With more than one instance, config written on one instance must take effect on the
others. Three pub/sub channels do that:

| Channel | Written by | Invalidates |
|---------|-----------|-------------|
| `perm:sync` | role / space / group permission changes | the permission cache (role → codes, space hierarchy) |
| `trust:sync` | XP rules, level definitions | the XP config snapshot |
| `gate:sync` | gate rules and domain defaults | the gate rule snapshot |

**Settings have no broadcast channel.** `setting:<key>` is a 60-second read cache; the
writer deletes its own key and other instances converge within 60 seconds. "Changed a
setting, one instance still behaves the old way, correct a minute later" is by design.

## SSE heartbeats

- `POST /api/v1/notifications/stream` writes an SSE comment `: keepalive` every 15
  seconds. An idle stream carries no bytes, so any intermediary with a read timeout cuts
  it — nginx defaults to 60 s and the disconnect was measured at 61 s. The frontend parser
  only reacts to `data: ` lines, so the comment cannot surface as an event.
- `POST /api/v1/realtime/stream` emits a `heartbeat` event every 15 seconds.

`deploy/nginx/nginx.conf` carries a dedicated SSE location (buffering off, timeout 3600 s).

## Verify

```bash
curl -s localhost:9091/metrics | grep feedchamber_eventbus_publish_total
curl -s localhost:8080/api/v1/health | grep kvstore
```

Missing notifications: check whether `feedchamber_eventbus_dropped_total` is rising, then
`feedchamber_sse_write_failures_total`. Changes that will not apply: confirm which instance
you changed, wait 60 seconds, then look for a WARN about a failed sync subscription.
