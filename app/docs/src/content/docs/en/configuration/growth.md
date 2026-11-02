---
title: Growth and XP
description: The XP aggregation formula and rule tables, streak milestones, level definitions and retention, manual adjustments, check-in, badges, and leaderboards
---

XP is not an incrementing counter; it is **aggregated from behaviour**:

```text
total_xp = Σ(metric count × current value) + Σ(streak milestones × bonus) + Σ(manual adjustments)
```

Weighting happens at read time: deleting, unliking, unfeaturing, or a moderation rollback reduces
XP automatically, and editing a value applies to everyone at once. The recompute path is the only
writer of `total_xp`, `trust_level`, and `level_metrics`.

## Before you start

- `level:manage` covers `/admin/xp-rules` and `/admin/levels`; per-user detail is
  `/admin/users/detail?id=<user id>` → "Growth".

## XP rules

`/admin/xp-rules` → "Behaviour metrics": the metric list is fixed, you edit values and caps.

| Metric | Default | Cap | Counts |
|--------|---------|-----------|--------|
| `post_approved_count` | 10 | 25 | Approved posts, including closed ones |
| `comment_approved_count` | 3 | 20 | Approved comments, excluding post bodies |
| `likes_received_count` | 1 | 30 | Likes on approved content |
| `favorite_received_count` | 2 | none | Times your posts were favorited |
| `featured_post_count` | 10 | none | Posts carrying a featured mark |
| `checkin_count` | 2 | none | Days checked in |

`active_day_count` and `report_upheld_count` exist only for retention.

Caps apply per calendar day: counts are grouped by day, **truncated to the cap, then summed** —
with "post = 10, cap 25", a third post that day adds nothing.

## Streak milestones

The "Streak ladder" section of `/admin/xp-rules`. A milestone pays each time a streak lands on it.

| Streak days | 3 | 7 | 15 | 30 | 60 | 100 |
|-------------|---|---|----|----|----|-----|
| Bonus | 3 | 10 | 20 | 40 | 80 | 150 |

Breaking a streak restarts the counter but never claws back earned milestones.

## Level definitions

`/admin/levels`. A level is a row: `min_xp`, name, retention window, and retention conditions.

| Level | Default name | Start XP | Window | Retention |
|-------|--------------|----------|--------|-----------|
| TL0 | 新手上路 | 0 | — | none |
| TL1 | 基础 | 50 | 100 days | none |
| TL2 | 成员 | 200 | 100 days | active ≥ 10 days, upheld reports ≤ 5 |
| TL3 | 常客 | 500 | 100 days | active ≥ 50, posts ≥ 10, likes ≥ 20, reports ≤ 3 |
| TL4 | 意见领袖 | 1000 | — | granted manually only (`manual_only`) |

Ranking and retention are one pass: sort enabled levels by `min_xp`, walk from the highest,
skipping `manual_only` levels and those whose `min_xp` is not met, and compute each level's window
metrics. The first level whose retention holds wins; otherwise the user drops to the lowest one
(`window_days = 0` means no conditions), so levels also go **down**, one step at a time. Admin-set
levels (the `level_manual` flag) survive recompute.

## Manual adjustments

`/admin/users/detail?id=<id>` → "Growth" → adjustments. Each entry is a ledger row (`delta`,
`reason`, actor, time); deleting one recomputes and undoes the grant. The same page sets a level
by hand, which is how TL4 is awarded.

## Check-in, badges, and leaderboards

`POST /api/v1/checkin` checks in once per day (a unique index enforces it) and returns current
streak, best streak, and total days; it grants no XP directly. Badges come from
`GET /api/v1/badges/list` (first post, 7-day streak, 10 likes). Leaderboards use
`POST /api/v1/leaderboard/list` with `type` = `points` (`total_xp`), `checkin`, or `posts`.

## Recompute: the only writer

`Recompute` converts every input into `total_xp`, `trust_level`, and the retention snapshot.

| Scope | How | Notes |
|-------|-----|-------|
| Full | `POST /api/v1/admin/trust/recompute` | Admin button; safe to repeat |
| Full (CLI) | `go run main.go -e prod recompute-xp` | Prints `recomputed N users` |

A 24-hour ticker also runs a full recompute, mainly to demote inactive users; because it
re-aggregates rather than accumulates, repeating it never double-counts.

## What takes XP away

| Action | Why |
|--------|-----|
| Deleting a post or comment | Soft-deleted rows leave the aggregates |
| Moderation rollback, or a hit moving content to pending | Only `approved` counts |
| Unliking or unfavoriting | The metric count drops by one |
| Removing a featured mark, or deleting an adjustment | The metric or the ledger loses an entry |
| Prolonged inactivity | Retention fails on active days |

## Verify

```bash
# XP breakdown for one user (admin)
curl -s http://localhost:8080/api/v1/admin/users/<user_id>/xp \
  -H 'Authorization: Bearer <sid>' | jq '.data | {total_xp, trust_level, xp_metrics}'
# Full recompute prints how many users it touched
cd app/server && go run main.go -e prod recompute-xp
```

## Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| Changing a value moves nobody | Rule disabled, or `delta=0` skips it | Enable it, then refresh the detail |
| Level is lower than XP suggests | Retention failed (active days, upheld reports) | Inspect window metrics |
| A hand-set level gets reverted | Not `manual_only`, and the user lacks `level_manual` | Use the manual level control, not XP edits |
| Someone dropped a level overnight | Daily full recompute failed retention | Expected |
| Leaderboard is empty | `type` misspelled, so it fell back to the default | Use `points`, `checkin`, `posts` |
