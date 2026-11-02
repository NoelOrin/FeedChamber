---
title: Upgrades and Migrations
description: Backups before upgrading, rebuilding, how AutoMigrate and hand-written SQL split, rolling restarts, rollback and post-upgrade reconciliation
---

The hard part is not swapping an image — **schema changes only move forward**. Back up,
upgrade, reconcile, and only then delete the backup.

## Before upgrading

Three commands cover what you cannot rebuild:

```bash
docker compose -f deploy/docker-compose.yml exec postgres \
  pg_dump -U feedchamber feedchamber > backup-$(date +%F).sql
docker run --rm -v feedchamber_uploads:/data -v "$PWD":/out alpine \
  tar czf /out/uploads-$(date +%F).tar.gz -C /data .
cp deploy/docker-compose.yml "compose-$(date +%F).yml" && git rev-parse --short HEAD
```

On SQLite use the CLI (same code as the admin **System → Backups** button):
`go run main.go -e prod backup --out /backup/2026-10-30` inside `app/server`.

:::caution[Not a full backup]
`backup` writes `app.db` (SQLite only), an uploads manifest and `meta.json` — it does not
package uploaded files and does not dump MySQL or PostgreSQL.
:::

Confirm the starting point: `docker compose ... ps` all healthy and `/api/v1/health` green.
Never upgrade on top of a red probe — you lose the ability to tell new breakage from old.

There is no upgrade-check command and no version endpoint. Judge by what the release touches:
dropped columns, renamed settings keys, renamed events, changed KV key shapes, changed
permission codes or response semantics — any hit means the checklist at the end.

## Rebuild

```bash
git pull && docker compose -f deploy/docker-compose.yml up -d --build
docker compose -f deploy/docker-compose.yml ps      # wait for healthy
```

Compose builds from this checkout (`build:`); to pin a tag, set `image:` to yours and drop
`build:`. Because `depends_on` uses `condition: service_healthy`, backend waits for
PostgreSQL and NATS by itself.

## How migrations happen

| Change | Executed by | Notes |
|--------|-------------|-------|
| New tables, columns, indexes | `AutoMigrate`, every boot | additive, safe for existing data |
| Existing row upgrades | one-shot boot functions | e.g. string levels to integers, `total_points` to `total_xp` |
| Dropped columns / type changes | `db/migrations/*.sql`, **by hand** | nothing runs these; each states background and idempotency |

**PostgreSQL and MySQL type changes need the script run manually**; SQLite is handled at boot.
`db/migrations/2026-10-29_trust_level_xp.sql` is the example: its header states the
requirement, and it is idempotent.

Order: back up → stop the old instance → start the new one (so `AutoMigrate` adds columns) →
run any required script → restart once → reconcile. Scripts run after boot because they need
columns `AutoMigrate` creates.

## Rolling restarts and NATS/KV compatibility

- **Sessions survive a restart.** The authority is the `user_sessions` row; KV is only a
  read-through cache, so wiping the NATS volume does not log everyone out.
- **The bucket name is fixed** (`feedchamber_kv` plus a `<bucket>_ttl` companion). Renaming it
  gives an empty store: session cache, rate limits, OAuth state, ban cache.
- **KV key shapes are a contract.** Changing `rl:<ip>:<policy>` or `rl:ratelimit:<ip>` resets
  the windows currently counting, so limits briefly loosen.
- **Running two versions at once** requires additive-only event payloads: removed or redefined
  fields corrupt the other instance's notifications and XP.
- **Keep the JetStream volume.** Losing `feedchamber_nats` is survivable, but an OAuth flow in
  progress fails once.

The safe pattern is one instance at a time, waiting for `/api/v1/health` before the next.

## Rolling back

Check out the previous commit and rebuild: `git checkout <previous commit> && docker compose
-f deploy/docker-compose.yml up -d --build`. Code rolls back; **data may not**. Old code ignores added columns and tables, so a rollback
usually works. Type changes, dropped columns and data moves do not: restore the backup. Your
rollback window equals your backup retention, so keep the pre-upgrade backup until the new
version proves stable.

## After upgrading

Run `cd app/server && go run main.go -e prod recompute-xp` (prints `recomputed N users`). It
is idempotent — XP derives from behaviour aggregates, so re-running never double-counts. The
daily full recompute first runs two minutes after boot; run it yourself instead of waiting.

## Swagger and openapi.json

`openapi.json` is a build artifact of `make swag`, never edited by hand:

```bash
cd app/server
make swag            # regenerate docs/ and copy to the repo-root openapi.json
make swag-check      # CI: non-zero exit when the committed copy is stale
```

Commit that file, then refresh the docs data with `node scripts/gen-reference.mjs` in
`app/docs`. The docs build also copies `openapi.json` into `public/`, so the download page
always follows the repository copy.

## Destructive change checklist

- [ ] Dropped a column or table, or changed a column type
- [ ] Renamed a setting key or changed its meaning
- [ ] Renamed an event type, or removed an event payload field
- [ ] Changed KV key shapes (`rl:`, `sess:`, `setting:`, `ban:`)
- [ ] Changed `NATS_BUCKET` or replaced the JetStream store
- [ ] Added or renamed permission codes (existing role mappings lose them)
- [ ] Changed error codes or response field semantics
- [ ] Changed the storage provider or upload directory (old file paths break)

Last step: `docker compose ... ps` all healthy, `/api/v1/health` green, then confirm three
behaviours — an existing account logs in, a new post can be written and read, and a
notification arrives.
