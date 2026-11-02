---
title: Content and Moderation
description: Spaces, posts and comments, the review queue, reports, trash, tags, announcements, and revisions
---

Content administration runs on two tracks. **Structure** decides where content lives and how it is shown: spaces, tags, announcements. **Disposition** decides whether a single item stays: posts, comments, the review queue, reports, trash. Both live in the admin area; no database access is needed.

## Prerequisites

You are in the admin area with the right codes: `space:update` for spaces, `post:read` for posts, `comment:delete` for comments, `report:manage` for the queue and reports, `post:delete` or `comment:delete` for trash, `tag:update` for tags, `announcement:manage` for announcements. Whether content is reviewed before publishing is controlled by the `moderation_mode` setting.

## Spaces

`/admin/spaces` shows a tree with unlimited nesting; each row offers "+ child", "Edit", "Permissions", and "Delete".

1. Create a root section → `POST /spaces/create`, then nest children under any node.
2. The edit dialog covers name, `slug` (URL identifier, globally unique), icon, description, parent, sort order, and visibility.
3. Reordering → `POST /spaces/reorder`; reparenting → `POST /spaces/{id}/move`.
4. Enable or disable → `POST /spaces/{id}/status`. Omit `is_active` to flip it, or pass a value. This is an operational flag; **what visitors see is controlled by visibility**, independently.

| Visibility | Meaning | Extra field |
|------------|---------|-------------|
| `public` | Anonymous readers allowed | none |
| `login` | Sign-in required | none |
| `level` | Minimum level required | `min_level` |

Child spaces inherit the strictest policy along the ancestor chain (`level` > `login` > `public`), and `min_level` takes the maximum of the chain. All six content outlets (list, detail, search, sitemap, RSS, batch) share one predicate, so a list never shows an item whose detail page would 403. Authors and reviewers holding `report:manage` are exempt.

The "Permissions" dialog sets per-space overrides, which only work for the 8 space-scoped codes (`post:update`, `post:delete`, `post:pin`, `post:lock`, `comment:update`, `comment:delete`, `user:update`, `space:update`). Inheritance rules are in [Users and permissions](/administration/users/).

:::caution
Deleting a space fails while it still has children or posts. Move or clear the content first.
:::

## Posts

`/admin/posts` filters by status (`approved`, `pending`, `rejected`, `closed`).

| Action | Endpoint | Permission |
|--------|----------|-----------|
| Pin / unpin | `POST /posts/{id}/pin` | `post:pin` |
| Lock / unlock | `POST /posts/{id}/lock` | `post:lock` |
| Feature / unfeature | `POST /posts/{id}/feature` | `report:manage` |
| Set status directly | `POST /posts/{id}/status` | `report:manage` |
| Close discussion | `POST /posts/{id}/close` | `post:create` (author or admin) |

Featuring writes `featured_at` and counts toward the author's XP; removing the feature reclaims that XP on the next recompute, so it is not a plain counter. Batch operations go through `POST /admin/batch/posts` (delete, pin/unpin, lock/unlock), up to 100 items per request. The detail page (`/admin/posts/detail?id=`) lists the body, interaction counts, and the post's comments (addable and deletable in place), plus a one-click mute on the author.

## Comments

`/admin/comments` is the global comment list (`POST /posts/comments/list`, requires `comment:delete`). The detail page shows one comment in context (`GET /comments/{id}`); deletion is `POST /comments/{id}/delete`, and batch deletion is `POST /admin/batch/comments`. Deletion is soft, so anything removed here can be restored from trash.

## Review queue

`/admin/moderation` starts with a mode card that decides the initial state of new posts:

| Mode | Behaviour |
|------|-----------|
| `off` | Published immediately, no rules, no queue |
| `async` | Published, then flipped to pending when a rule matches (default) |
| `external` | Calls an external moderation service before publishing; falls back to `async` |
| `manual` | Everything waits for human review |

:::note
Comments always go through keyword rules and are not affected by this switch.
:::

Each pending row (`POST /admin/moderation/pending`) has two actions. **Approve** (`POST /admin/moderation/{id}/approve`) marks the record approved and sets the target to `approved`. **Reject** (`POST /admin/moderation/{id}/reject`) optionally takes a reason and sets the target to `rejected`.

Every record keeps the target type and id, the deciding strategy (`async`, `external`, `manual`), and the original body, so "who rejected this, when, and why" stays answerable. Each decision also emits a `moderation.decided` event for webhook subscribers. Rule management (`/admin/moderation/rules/*`) and config (`/admin/moderation/configs`) sit on the same page; when unsure, changing the mode is safer than piling up rules.

## Reports

`/admin/reports` filters by status (`pending`, `resolved`); the "count" column is how many times the same target has been reported. Open the detail page (`/admin/reports/detail?id=`) to read the reason and the target body, then handle it with `POST /reports/{id}/status` using `pending`, `resolved`, or `dismissed`. Resolving accepts an action: `none` (mark only), `delete_target`, `lock_post`, or `ban_user`. Handling records the actor, time, and note; dismiss false positives instead of leaving them pending.

## Trash

`/admin/trash` has two tabs (`POST /admin/trash/posts/list`, `POST /admin/trash/comments/list`). "Restore" calls `POST /admin/trash/posts/{id}/restore` or `POST /admin/trash/comments/{id}/restore`, returning the item with its status unchanged. Trash is not an archive: restored content still obeys the visibility policy of its space or post.

## Tags and announcements

`/admin/tags` manages tags themselves (`POST /tags/create`, `POST /tags/{id}/update`, `POST /tags/{id}/delete`). Tags are many-to-many with posts, set by name when publishing; unknown names are created on the fly.

`/admin/announcements` manages site-wide announcements:

| Field | Meaning |
|-------|---------|
| `status` | `draft` or `published`; `publish` and `unpublish` are separate actions |
| `display_type` | 1 = toast, 2 = alert banner, 3 = modal, 4 = iframe embed (`content` holds a URL) |
| `position` | Toast placement, six values, default `top-right` |
| `is_pinned`, `sort` | Ordering when several are live |
| `expires_at` | The frontend stops showing it after this time |

Create the announcement, save it as a draft, then publish from the row action. Use `expires_at` for campaigns instead of remembering to unpublish them.

## Revision history

Every edit of a post or comment keeps a revision. List them with `POST /posts/{id}/revisions/list` or `POST /comments/{id}/revisions/list`, and restore with the matching `/{revisionId}/restore`. There is **no dedicated admin page**: the entry point is the "History" button on the public post detail, available to the author and to accounts with moderation rights. Restoring updates the body immediately.

## Verify

1. Set a space to "level ≥ 2" and browse as a level-1 user: content is hidden, and a direct post link returns 403.
2. In `async` mode, publish a post matching a keyword rule: it flips to `pending`, then approving it makes it public.
3. Report a post and resolve it with `delete_target`: the post lands in trash and the report is resolved.
4. Unpublish an announcement and refresh the public site: its toast or modal is gone.

## Common failures

| Symptom | Cause | Fix |
|---------|-------|-----|
| New posts land in review immediately | Mode is `manual`, or `async` matched a rule | Change the mode or approve in the queue |
| Approved post still invisible | Visibility is stricter than the new status | Check space visibility and the post's `min_level` |
| Space deletion fails | Child spaces or posts remain | Move or clear them first |
| Published announcement does not show | Still a draft, expired, or the iframe URL is unreachable | Check status and `expires_at` |
| Restored content still 404 | The original visibility gate still applies | Expected; adjust visibility to change it |
