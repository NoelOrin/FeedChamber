---
title: Glossary
description: The terms used across these docs, one line each
---

| Term | Meaning |
|------|---------|
| **Space** | The container a post belongs to; a tree with visibility, icon, ordering, and enable state |
| **Visibility** | Read policy: `public` (guests), `login`, or `level`; the strictest policy along the ancestor chain wins |
| **Gate** | A rule engine that decides `allow` / `deny` / `require_review` / `rate_limit` from conditions |
| **Gate domain** | The capability a rule applies to (posting, comments, links); unmatched requests use the domain default |
| **TL0** | Level 0, the starting level for new accounts and guests (historically Trust Level 0) |
| **XP** | Experience derived from behaviour: `Σ(metric × rule) + streak bonuses + manual adjustments`; deleting content reclaims it |
| **Level retention** | Levels are evaluated over a rolling window, so inactive users can drop; promotion and demotion walk one level at a time |
| **Manual adjustment** | An admin add/subtract of XP with a reason; it participates in aggregation and survives recomputation |
| **Moderation mode** | `off` / `async` (default) / `external` / `manual`, deciding the initial state of new content |
| **Graduated enforcement** | Machine credentials writing high-risk content (JS/HTML) are forced into drafts for human approval |
| **Rotation** | `/auth/refresh` issues a new sid and revokes the old one immediately; single use |
| **Server-side session** | The credential is a row in `user_sessions`; validity equals row existence |
| **Read-through cache** | Check KV first, fall back to the database on a miss and backfill |
| **Envelope** | The standard response shape `{ code, msg, data }` |
| **Protocol endpoint** | An endpoint that skips the envelope (OAuth / OIDC / MCP) to keep its RFC shape |
| **Scope** | The permission range on an API key — a **ceiling**, intersected with the owner's role permissions |
| **Deps** | The explicit dependency list; handlers consume `handler.Deps`, wired in `server/wiring.go` |
| **Wiring** | The layer that injects constructed services into handler dependencies |
| **Event bus** | NATS core pub/sub; services publish events, the notification service subscribes |
| **JetStream KV** | NATS persistent KV used for session cache, rate-limit windows, settings cache, and OAuth state |
| **SSE** | Server-Sent Events, used for notifications and generic realtime; idle streams get a 15s keepalive comment |
| **MCP** | Model Context Protocol; the site acts as a server for AI clients and as a client for external services |
| **Motion** | A community proposal with voting and a resolution |
| **Watch** | Following a topic to be notified about replies and likes |
| **Trash** | Where soft-deleted content waits for restore; not a hard delete |
| **Revision** | A historical version of a post or comment that can be viewed and restored |
| **Audit log** | A record of admin actions and MCP calls: actor, action, target, timestamp |
| **Backup** | A database export plus upload inventory, from the admin panel or CLI; NATS data is rebuildable and not part of it |
