---
title: Admin Overview
description: How the admin area is entered, who gets in, how modules are gated, and the full route table
---

The admin area is a route segment of the main site (`app/web/app/admin/**`), not a separate deployment. Same build output, same origin, same deployment, and the same `components/ui/` primitives the public site uses. It is also not server-rendered: `app/admin/layout.tsx` wraps the whole segment in a client mount gate, so the server only ships the site shell and every page component is `'use client'`. The first paint is always the loading skeleton; data comes from API calls made by the browser.

Rule of thumb: **the admin area is a section of the frontend, not a second application.**

## Access: three outcomes

`lib/admin/auth/access.ts` is the single entry check. It calls `GET /user/profile` with the session cookie and trusts only what the server returns, never a client-side role cache.

1. Role is `admin` or `moderator` → allow.
2. Otherwise fetch the role's permission codes; any hit on the admin permission list → allow
   (this is the escape hatch for custom roles).
3. Anything else → deny.

Denial behaves differently depending on the session:

| Situation | Result |
|-----------|--------|
| Not signed in | Redirect to `/login?redirect=/admin/...`, then back to the admin area |
| Signed in without admin access | Sent back to the page you came from |

For the second case the return target is resolved from the stored referrer, then `document.referrer`,
then the site URL. Only when nothing usable exists does the full-page 403 appear, so a wrong click
does not trap a regular user on an error page.

The admin permission list deliberately omits read permissions that every registered user has, such as
`user:read`, `role:read`, and `stats:read`. Otherwise any account could enter. The list holds 21 codes,
including `setting:manage`, `role:manage`, `user:update`, `user:delete`, `banned:manage`, `batch:edit`,
`announcement:manage`, `report:manage`, `backup:manage`, `plugin:manage`, `level:manage`, `gate:manage`.

:::note
There is no separate admin login page. Sign-in always goes through `/login`; `?redirect=` only accepts
same-site relative paths.
:::

## Module gating

Getting in does not mean every module is readable. `lib/admin/auth/route-access.ts` maps path prefixes
to permission code arrays with **anyOf** semantics. The same table filters the sidebar and renders
"no permission" inside the shell instead of a broken page.

| Path | Allowed by any of |
|------|-------------------|
| `/admin` | `stats:read` |
| `/admin/posts` | `post:read` |
| `/admin/comments` | `comment:delete` |
| `/admin/reports`, `/admin/moderation` | `report:manage` |
| `/admin/users` | `user:read` |
| `/admin/roles` / `/admin/roles/detail` | `role:read` / `role:manage` |
| `/admin/settings`, `/admin/mcp`, `/admin/oauth-clients` | `setting:manage` |
| `/admin/trash` | `post:delete` or `comment:delete` |

Every other module is listed in the same file. Paths without an entry have no extra gate.

:::caution
The frontend gate is UX only: it hides entry points and avoids pages that would certainly 403.
The real boundary is the API gateway, which maps each request path to a permission code and checks it.
Session credentials are judged by role permissions; API keys by the intersection of scopes and role
permissions. Changing the frontend table never widens an endpoint.
:::

## Route table

| Group | Routes | Purpose |
|-------|--------|---------|
| Home | `/admin` | Dashboard: stats, shortcuts, global search, recent content |
| Forum | `/admin/posts`, `/admin/comments`, `/admin/banned`, `/admin/tags`, `/admin/trash` | Posts, comments, bans, tags, trash |
| Operations | `/admin/announcements`, `/admin/reports`, `/admin/moderation`, `/admin/spaces` | Announcements, reports, review queue, spaces |
| Site customization | `/admin/customize/*` | Snippets, theme, navigation, home modules, pages |
| Users and permissions | `/admin/users`, `/admin/invites`, `/admin/roles`, `/admin/groups`, `/admin/levels`, `/admin/xp-rules`, `/admin/gates` | Users, invites, roles, groups, levels, XP, gates |
| System | `/admin/api-keys`, `/admin/webhooks`, `/admin/backups`, `/admin/oauth-clients`, `/admin/plugins`, `/admin/mcp`, `/admin/mcp-servers`, `/admin/security`, `/admin/sessions`, `/admin/settings?tab=`, `/admin/audit-logs` | Settings and integrations |
| Hidden | `/admin/search` | Global search, requires `admin:search` |
| Auth | `/admin/first-change-password` | Forced password change |

Detail pages take an `?id=` query parameter, for example `/admin/users/detail?id=<user-id>`.
Site customization has its own guide: [Site customization](/configuration/customization/).

## Registering a page

The menu is not generated. A new admin page needs two edits, or the route exists but nobody can find it:

1. `components/admin/layout/admin-shell.tsx` → `menuItems` for group, icon, and order.
   Add `hideInMenu` for detail pages and internal screens such as `/admin/search`.
2. `lib/admin/auth/route-access.ts` → `ROUTE_PERMISSIONS` for the required permission code.
   A missing entry means the page is open to anyone already in the admin area.

## First password change

The seeded `admin` account carries a must-change flag, and the login response reports
`must_change_password: true`.

1. Sign in with the initial credentials and open `/admin/first-change-password`.
2. Submit the new password (optionally a new username) → `POST /auth/first_change_password`.
3. The flag is cleared, the page signs you out, and you sign in again with the new password.

The endpoint only accepts the `admin` role. The minimum length comes from the `password_min_length`
setting, default 8.

## Verify

1. Open `/admin` as a regular user: you are sent back where you came from (or shown a 403), never left on `/admin`.
2. Open `/admin` as an admin: every group you have permissions for appears. Remove `report:manage` from a
   role, sign in again, and Reports plus Moderation vanish from the menu; navigating directly shows the
   in-shell "no permission" page.
3. Call `POST /api/v1/admin/stats` with a regular user session: it must return 403.

## Common failures

| Symptom | Cause | Fix |
|---------|-------|-----|
| Stuck on the loading skeleton | The browser cannot reach `/user/profile` | Check API reachability, the session cookie, and the `/api/v1` proxy |
| Only one or two menu groups | The role lacks permission codes | Add codes in Roles, then sign in again |
| In-shell "no permission" on a direct URL | No `anyOf` code matched | Note that `/admin/posts` needs `post:read`, not `post:create` |
| New page returns 404 | Wrong directory, or the page is not a client component | Admin pages are always `'use client'` |
