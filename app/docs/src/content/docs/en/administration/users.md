---
title: Users and Permissions
description: Managing users, roles and permission codes, groups, mutes and bans, invites, and manual XP changes
---

The "Users and permissions" area answers three questions: what role someone has, what they may do, and what to do when they misbehave. Roles and permission codes answer the first two; mutes (cannot post) and bans (cannot sign in) handle the third, as two independent mechanisms.

## Prerequisites

You are signed in with an `admin` or `moderator` account (see [Admin overview](/administration/overview/)). Changing a role needs `user:update`, editing role permissions needs `role:manage`, the mute list needs `banned:manage` while applying a mute needs `user:update`, and adjusting XP needs `level:manage`. The menu shrinks to what you hold.

## User list and detail

`/admin/users` is a paginated list. Rows offer role change, mute, and a detail link; accounts pending approval show an "Approve" action. Batch delete and batch role change are capped at **100 users per request**.

`/admin/users/detail?id=<user-id>` has five sections:

| Section | Content | Source |
|---------|---------|--------|
| Profile | Username, email, role, join date | `GET /user/{id}` |
| Growth | XP breakdown (count × per-unit XP), level, adjustments | `GET /admin/users/{id}/xp` |
| Posts | Authored posts | `POST /admin/user/{id}/activities` |
| Comments | Authored comments | Same |
| Reported | Reports against the user's content | Same |

## Changing a role

Open "Change role" and confirm → `POST /user/{id}/role/update`.

- **A role change revokes every session of that user**, including a ban (role set to `ban`). The next request fails; nobody waits for expiry.
- The API refuses to demote the last user holding `role:manage` (403). Without that guard, nobody could administer permissions afterwards.
- System group membership follows the role automatically.

| Role | Meaning |
|------|---------|
| `admin` | Administrators, all permission codes by default |
| `moderator` | Content and report operations |
| `user` | Regular members: post, comment, report |
| `pending_user` | Awaiting approval: signs in, cannot post or comment |
| `ban` | Banned: sign-in rejected, sessions revoked |

## Permission codes and the role matrix

There are **61** platform permission codes, grouped by prefix (`post`, `comment`, `tag`, `space`, `user`, `role`, `report`, `announcement`, `setting`, `stats`, `banned`, `batch`, `admin`, `reconcile`, plus domains such as `motion`, `api-key`, `session`, `security`, `backup`, `plugin`, `level`, `gate`, `customize`).

`/admin/roles/detail?id=<role>` is the matrix. The top half holds global checkboxes that apply to every space. The bottom half holds per-space overrides, and only for the **8 space-scoped codes**: `post:update`, `post:delete`, `post:pin`, `post:lock`, `comment:update`, `comment:delete`, `user:update`, `space:update`. Other codes are global; overriding them per space has no effect.

An override **replaces** the global grant instead of extending it. Child spaces inherit the nearest override up the ancestor chain and fall back to global only when the whole chain is empty — the classic trap being that once a child has an override, the parent's setting no longer applies to it.

Defaults worth remembering: `admin` holds everything, including `setting:manage`, `role:manage`, `backup:manage`, `plugin:manage`, `level:manage`, `gate:manage`, and `customize:*` (customization can inject scripts, so admins only). `moderator` holds posts, comments, tags, spaces, reports, mutes, batches, stats, and `role:read`, but not `role:manage`, `setting:manage`, `user:update`, or `backup:manage`. `user` holds posting, commenting, reporting, and self-service codes. `pending_user` is read-only plus self-service. `ban` holds no business permissions.

## Groups

`/admin/groups` manages two kinds. **System groups** share names with roles (`admin`, `moderator`, `user`, `pending_user`, `ban`) and follow the user's role automatically — do not maintain them by hand. **Custom groups** are populated manually; visibility (`public`, `closed`, `secret`) is a display concern.

Member roles are `owner`, `moderator`, and `member`. The "Permissions" dialog attaches codes to a group (`POST /groups/{id}/permissions/sync`). Note that admin access, the menu, and the gateway read **role permissions only**: group codes never substitute for a role, so adding codes to a group does not give its members admin access.

## Mutes and bans

| Mechanism | Effect | Can the user sign in? |
|-----------|--------|----------------------|
| Mute (`banned` table) | Blocks write operations | Yes |
| Ban (role set to `ban`) | Sign-in rejected, sessions revoked | No |

Mutes come in two scopes. A **global mute** is `POST /user/{id}/banned` with `reason`, `is_permanent`, and `duration_minutes` for a temporary mute (or `expires_at`). A **space-scoped mute** is `POST /user/{id}/space/{space_id}/banned`, blocking writes in that space only.

If neither a duration nor the permanent flag is supplied, the API records a permanent mute, which removes the "muted forever, expires immediately" ambiguity. `/admin/banned` lists every mute record and filters by `scope` and `space_id`; the row action calls `POST /user/{id}/unbanned` or the matching space endpoint. Each mute and unmute writes an audit entry.

:::caution
A mute does not revoke sessions: the user can still browse and sign in. To take someone offline immediately, change their role (ban) or revoke their sessions.
:::

## Invites and registration modes

Registration follows the `register_mode` setting: `all` (open), `email` or `username` (identifier restriction), `invite` (code required), and `approve` (new accounts start as `pending_user`).

In invite mode, `/admin/invites` handles the flow: create with `POST /admin/invites/create` (maximum uses, 0 means unlimited, plus an expiry), review usage in the list, and invalidate with `POST /admin/invites/{id}/delete`. Registration without a valid `invite_code` is rejected.

In approval mode, pending accounts show an "Approve" action → `POST /admin/users/{id}/approve_registration`, which promotes them to regular users.

## Levels and manual XP

`/admin/levels` defines the level value, name, key, XP threshold, retention window in days, and a manual-only flag. A window of 0 means no retention condition; manual-only levels never change automatically. `/admin/xp-rules` defines metric → XP per unit, per-metric daily caps (0 means unlimited), enabled flags, and the check-in streak ladder.

In the user's Growth tab, review the breakdown and adjust with a `delta` (which may be negative) and a **required reason** → `POST /admin/users/{id}/xp/adjust`. Adjustments are listed and can be deleted.

XP is an aggregate: deleting posts, removing a feature, or undoing likes reclaims it, and rule changes apply to everyone immediately. Reconcile with `POST /admin/trust/recompute` or the `recompute-xp` CLI; never edit the database by hand.

## Sessions and security events

`/admin/sessions` and `/admin/security` show **the current account's** sessions and events, not a site-wide view. Sessions list the device and last activity with single revoke and "sign out everywhere". Security events list time, type, severity (`low`, `medium`, `high`, `critical`), IP, detail, and a handled flag.

There is no button to kill another user's sessions. Changing their role or banning them revokes all of their sessions; that is the supported path.

## Verify

1. Change a user's role and have them refresh: they lose access immediately.
2. Apply a global mute, then post as that user: 403. Unmute and the request succeeds.
3. Uncheck one permission on a role, sign in again, and the menu entry disappears; a direct call returns 403.
4. Adjust a user by `+50` XP with a reason and confirm the Growth tab shows the delta and the record.

## Common failures

| Symptom | Cause | Fix |
|---------|-------|-----|
| 403 "cannot remove the last administrator" | Target is the last user with `role:manage` | Grant the code to another account first |
| A muted user can still post | Space-scoped mute, or already expired | Check `scope` and `expires_at` |
| A per-space permission has no effect | Not one of the 8 space-scoped codes | Use the global checkbox |
| Growth tab empty or 403 | Current account lacks `level:manage` | Use an account that holds it |
| Invite registration fails | Code used up or expired, or mode is not `invite` | Check `/admin/invites` and the setting |
