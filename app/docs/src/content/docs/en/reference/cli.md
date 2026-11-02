---
title: CLI
description: The server binary, its environment flag, and the backup, import-discourse, and recompute-xp commands
---

The entry point is a Cobra root command. **There are no `dev` / `prod` subcommands**: the
environment is selected only with `-e`.

```bash
go run main.go -e dev      # reads .env + .env.dev
go run main.go -e prod     # reads .env + .env.prod
go run main.go dev         # unknown command
```

`.env` loads first, `.env.<env>` second and overrides it. Positional arguments fail because the
root command takes none.

## Commands

| Command | Purpose | Required |
|---------|---------|----------|
| `backup` | Export a local site backup (DB snapshot plus uploads inventory) | `--out` |
| `import-discourse` | Import a Discourse JSON export directory | `--dir` |
| `recompute-xp` | Recompute XP and levels for every user | none |

### `backup`

```bash
go run main.go -e prod backup --out /backup/2026-10-30
```

Artifacts:

| File | Contents |
|------|----------|
| `app.db` | **SQLite only**: a copy of the database file |
| `db-note.txt` | MySQL / PostgreSQL: a reminder to run `mysqldump` / `pg_dump` yourself |
| `uploads-manifest.json` | Upload inventory (relative path, size, mtime) |
| `meta.json` | Created-at, DB type, Go version, upload count |

:::caution[Not a full backup]
The command copies a SQLite file and writes an uploads inventory; it does **not** archive the
upload files themselves and does not dump MySQL or PostgreSQL. Pair it with volume snapshots or
engine-native tools — see [System and integrations](/en/administration/system/).
:::

### `import-discourse`

```bash
go run main.go -e prod import-discourse \
  --dir /import/discourse-export \
  --source discourse
```

`--dir` must contain `users.json`, `categories.json`, `topics.json`, and `posts.json`.
`--source` is recorded in `import_maps` for de-duplication; it defaults to `discourse`.

Mapping: Discourse categories become spaces, topics become posts, posts become comments, and
`reply_to_post_number` restores nesting. Imports are idempotent per `import_maps`, and they do
not overwrite locally edited content.

### `recompute-xp`

```bash
go run main.go -e prod recompute-xp
# recomputed 1234 users
```

XP is derived from behaviour, so recomputing cannot double-count. Use it for migrations, after
changing XP rules or level definitions, or to repair a single user's level. It is safe to run
against a live service; it only writes `users.total_xp`, `trust_level`, and `level_metrics`.

## Building

```bash
cd app/server

make build          # current platform
make linux          # cross-compile linux/amd64
make darwin
make windows
make swag           # regenerate Swagger and sync openapi.json
make swag-check     # CI: fail when swagger is stale
```

Binaries land in `app/server/build/`. After `make swag`, commit `openapi.json`: the docs
[Endpoint index](/en/reference/endpoints/) is generated from it.

## Exit behaviour

Configuration errors are fatal at startup — the process exits instead of degrading:

| Scenario | Output |
|----------|--------|
| Empty `NATS_URL` | `NATS_URL is required: KV Store and EventBus need an external NATS…` |
| Unsupported `DB_TYPE` | `unsupported DB_TYPE "…"` |
| Missing DSN for `mysql`/`postgres` | `DB_DSN is required when DB_TYPE=…` |
| Invalid metrics port | `METRICS_PORT must be between 1 and 65535…` |
| No admin password | Does not exit; generates one and logs a warning |
