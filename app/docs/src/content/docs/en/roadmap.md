---
title: Roadmap
description: What is stable, what is experimental, and what is explicitly out of scope
---

This page describes **what exists in the repository today**. It is not a promise. Features
marked experimental work but may change; features marked out of scope are not coming.

## Stable

| Area | Coverage |
|------|----------|
| Content | Space tree, posts, nested comments, tags, drafts, revisions, trash, pin/lock/feature |
| Social | Likes, favourites, follows, blocks, topic watches, direct messages |
| Auth | Server-side sessions, email codes, MFA (TOTP plus backup codes), six OAuth providers, OIDC SSO |
| Permissions | 61 platform codes, space-level overrides with inheritance, groups, API key scopes |
| Governance | Four moderation modes, review queue, reports, two-level bans, account bans, gate rules, audit log |
| Growth | XP aggregation, level promotion plus retention, check-in, badges, three leaderboards, manual adjustments |
| Site | Theme tokens, custom CSS/JS/HTML, navigation, home modules, custom pages |
| Integration | REST (313 paths), SSE notifications, webhooks (12 events), MCP server and client |
| Operations | Prometheus metrics, structured logs, traceparent, backup and restore, sitemap/robots/RSS |

## Experimental

| Feature | State |
|---------|-------|
| Community motions | Backend and admin work; frontend entry points are sparse |
| External MCP moderation | Works, but you must supply a streamable-HTTP MCP server |
| Plugins | Declarative metadata only (install/enable); no code execution |
| Discourse import | Four JSON files (users/categories/topics/posts) with fixed mappings |

## Not implemented

| Item | Note |
|------|------|
| Mobile builds | The Capacitor pipeline is paused and would need re-validation against vinext output |
| UI localisation | The application UI is zh-CN only; these docs are bilingual |
| Frontend first-password page | The backend supports `first_change_password`; only the admin panel has a page |
| Cross-user session management | `/admin/sessions` manages only your own sessions |
| Manual level UI | `POST /user/{id}/trust_level/update` exists; the admin UI does not expose it |
| Direct-to-object-storage uploads | Presigning exists, but the frontend mostly proxies through the server |

## Out of scope

| Idea | Why not |
|------|---------|
| REST-style API (PUT/PATCH/DELETE) | Conflicts with the action-oriented API; clients need one write method |
| Arbitrary plugin code | That is remote code execution; plugins stay declarative |
| JWT as the primary credential | Server-side sessions make "delete the row" the revocation story |
| Embedded NATS | KV and the event bus should not share a lifecycle with the app process |

## How to tell whether something works

Do not guess from this page. Check three places:

1. **Routes**: is it registered in `internal/router/routes_*.go`
2. **Endpoint index**: does the path appear in [Endpoint index](/en/reference/endpoints/)
3. **Admin menu**: is there an entry in the `/admin` sidebar

If all three agree, it is usable.
