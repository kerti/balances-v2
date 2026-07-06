# Local dev, lint, and tests

Operational recipes for running balances-v2 locally and checking work before pushing. The domain and
conventions live in `CONTEXT.md` and `HANDOFF.md`; this file is just the how-to-run reference.

`make help` lists every target. The common loop uses the **background dev-server** targets so the
servers keep running across edits.

## First clone

```bash
make setup            # git hooks (PII guard) + frontend deps + seed .env from .env.dev.example
```

Bundles `hooks-install` and `frontend-install` so neither is missable on a fresh clone; idempotent,
safe to re-run.

## Running locally

```bash
make up               # start the compose stack (postgres etc.) in the background
make restart          # (re)start both background dev servers — backend + frontend
make servers-status   # show which dev servers are running
make logs             # follow compose logs   (server logs: /tmp/balances-{backend,frontend}.log)
```

Foreground equivalents when you want a single attached process: `make backend-run`,
`make frontend-dev`. Migrations auto-run on backend `serve` startup; run them manually with
`make backend-migrate-up` / `-down` / `-status`.

**Restart the backend after Go edits — `make backend-restart`.** The dev server runs a *built binary*
and does **not** auto-reload (no air/CompileDaemon wired). File edits don't take effect until the
binary is rebuilt, and the auto-migrate-on-`serve` masks this — migrations apply but stale Go code
keeps running. If a backend test passes locally but the dev server shows old behavior, restart first;
don't go hunting for a code bug. `make restart` bounces both servers.

## Codegen

```bash
make backend-sqlc            # regenerate backend/internal/db from queries/ + migrations
make backend-gen-ts-types    # regenerate frontend/src/api/generated.types.ts from the Go structs
```

Run `backend-gen-ts-types` after a migration/sqlc regen changes a wire-facing struct's fields (see
`frontend/src/api/types.ts`'s header and `backend/tools/gen-ts-types`, issue #365). `make check` /
CI (`backend-gen-ts-types-check`) fails if the generated file is stale relative to the Go source.

## Auth and smoke-testing the API

The backend command is `serve`, not `server`. There is **no dev-login backdoor** — auth is real
Google OAuth. For backend smoke tests against authenticated endpoints, pull a current session token
from the `sessions` table and pass it as a cookie:

```bash
docker exec balances-v2-postgres-1 psql -U balances -d balances \
  -c "SELECT s.id as token FROM sessions s WHERE s.expires_at > now() LIMIT 1;"
# then: curl -H "Cookie: session=<token>" http://localhost:8080/api/...
```

## Lint (clean before pushing)

```bash
cd backend && golangci-lint run    # config: .golangci.yml (repo root)
cd frontend && npm run lint        # config: frontend/eslint.config.js
cd frontend && npm run format      # config: frontend/.prettierrc.json — npm run format:check for CI's read-only check
```

CI runs all three on every push. `revive`'s `exported` / `package-comments` are deliberately disabled for
application code; `react-refresh/only-export-components` is off for `components/ui/**` (shadcn);
`react-hooks/set-state-in-effect` is enforced everywhere else.

`make check` runs both lints + Go tests + vitest as a pre-push gate, printing one pass/fail line per
step (full output in `/tmp/balances-check-*.log`, read only on a ✗); e2e is excluded.

## Tests (run the suite for the area you touched)

```bash
make backend-test                  # all Go tests   (or: cd backend && go test ./...  [-race])
cd frontend && npm run test:coverage   # vitest
make e2e                           # full Playwright run — create+seed db, mock OIDC, suite (ADR-0024)
```

Match the suite to the area touched; don't skip just because lint + build pass. The Codecov config
(`codecov.yml`) keeps coverage status informational-only — failing CI from coverage drops is a
deliberate non-goal until alpha.

## Pre-commit hook (PII guard)

Run **once per clone** (bundled into `make setup` — see "First clone" above):

```bash
make hooks-install
```

This sets `core.hooksPath=.githooks` and seeds a local, gitignored `.pii-patterns` denylist (from
`.pii-patterns.example` + your git identity). The `.githooks/pre-commit` guard then scans each
commit's staged **additions** against those rules (case-insensitive ERE, one per line) and blocks the
commit if any match, printing only the offending **filenames** — never the matched content, so a
blocked commit doesn't echo PII back into the terminal.

Add your real dev-data terms (account names, employer, figures you reuse) to `.pii-patterns`; it's
never committed, because the terms are themselves the PII. **Do not bypass with `--no-verify`** — the
guard is the backstop for the public repo. It complements, not replaces, scrubbing to neutral
fixtures + toy numbers before staging.

