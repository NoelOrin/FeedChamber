---
title: Contributing
description: Branch model, commit conventions, local checks, and the pull request checklist
---

Issues and pull requests are welcome. This repository is strict about verifiable changes: CI
checks formatting, lint, swagger drift, and tests, and documentation must ship in the same
change.

## Get running

```bash
pnpm install
cd app/server && go mod download
```

The backend needs an external NATS with JetStream:

```bash
docker run --rm -p 4222:4222 nats:2.10-alpine -js
```

Full steps: [Local development](/en/install/local/).

## Branch model

| Branch | Purpose | Rules |
|--------|---------|-------|
| `main` | Release trunk | Pull requests only; Quality must pass |
| `develop` | Daily integration | Direct pushes allowed; short-lived branches merge back here |
| `feat/*` `fix/*` `docs/*` … | Short-lived work | kebab-case, deleted after merge |

Names look like `feat/server/space-tree` or `fix/web/login-redirect`. The type prefix must come
from commitlint's `type-enum`: `feat` `fix` `docs` `style` `refactor` `perf` `test` `chore`
`build` `ci` `revert` `types` `wip`.

## Commits

Conventional Commits:

```text
feat(server): support include_sub when listing posts
fix(web): keep the redirect parameter after login
docs(api): document the pagination ceiling
```

`commitlint` runs from the `lefthook` commit-msg hook. `wip/*` branches never merge to trunk.

## Code conventions

### Backend (Go)

- One-way layering: `handler → service → repository → db`
- Writes are `POST` with action suffixes; responses go through `pkg.Success` / `pkg.Fail` / `pkg.HandleError`
- New error codes live only in `internal/pkg/errors.go`
- New services are wired explicitly in `server/wiring.go`; a missing mapping is a compile error
- Before pushing: `gofmt`, `go vet ./...`, `go test ./...`

### Frontend (React / TypeScript)

- Pages in `app/web/app/**`, domain components in `app/web/components/<domain>/`
- New admin page: create `app/web/app/admin/<route>/page.tsx`, register the menu entry in
  `components/admin/layout/admin-shell.tsx`, and the permission in
  `lib/admin/auth/route-access.ts`
- Format and lint with `pnpm check` from the root

## Tests

```bash
cd app/server && go test ./...
pnpm check
bash feedchamber_test.sh -all   # backend must be running
```

New endpoints should cover the happy path, insufficient permission, and invalid input. Bug-fix
PRs should include a regression check in a unit test or the API script.

## Documentation

Documentation is part of the change and CI enforces it:

- Behaviour changes: update the relevant `AGENTS.md`
- API changes: run `make swag` and commit `openapi.json` (otherwise `swag-check` fails)
- User-facing changes: update the matching page here; the docs build validates internal links
  and the base prefix

Authoring rules: [app/docs/AGENTS.md](https://github.com/NoelOrin/FeedChamber/blob/main/app/docs/AGENTS.md).

## Pull request checklist

- [ ] Branch name follows `<type>/<scope>/<desc>`
- [ ] Commit messages follow Conventional Commits
- [ ] `gofmt`, `go vet`, and `go test` pass
- [ ] `pnpm check` passes
- [ ] API changes regenerated `openapi.json`
- [ ] Affected `AGENTS.md` files and docs pages updated
- [ ] Description explains why the change exists and how it was verified

## Security and conduct

- Report vulnerabilities privately, per [SECURITY.md](https://github.com/NoelOrin/FeedChamber/blob/main/SECURITY.md)
- Discussion follows the [Code of Conduct](https://github.com/NoelOrin/FeedChamber/blob/main/CODE_OF_CONDUCT.md)
