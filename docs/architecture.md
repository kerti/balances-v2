# Architecture (one page)

The mental model you'd otherwise assemble by reading a dozen ADRs. This page is the map; the
[ADRs](adr/) are the territory (each decision links back to one). Domain language is in
[`CONTEXT.md`](../CONTEXT.md); the route inventory is [`api-routes.md`](api-routes.md).

## What it is

A single Go binary that serves a JSON API under `/api` **and** the built React SPA from the same
origin (ADR-0030). In dev the two run split — Vite on `:5173` proxying `/api` + `/healthz` to the
backend on `:8080`; in production/self-host one container serves both (`WEB_DIR` set → the SPA
catch-all mounts; unset in dev). Postgres is the only backing store (ADR-0013); there is no cache,
queue, or second service.

## Request flow

```
Browser (React SPA, TanStack Query)
   │  fetch /api/... with session cookie
   ▼
chi router  ──  middleware chain (backend/internal/httpserver/server.go)
   │   RequestID → Logger → Recoverer → securityHeaders → SessionMiddleware
   │   (SessionMiddleware resolves the cookie to a user; RequireAuth per-route rejects if absent)
   ▼
Handler        internal/<domain>/   — decode, validate (go-playground/validator), authz
   │                                  scope to the caller's Household, map errors → httperr envelope
   ▼
Repo           internal/repo/       — SQL via sqlc-generated internal/db/ (pgx pool)
   │                                  every query filtered by household_id (ADR-0005 row-level tenancy)
   ▼
Postgres       migrations embedded in the binary, applied by goose on boot (ADR-0019)
```

Responses are the ADR-0027 `httperr` envelope (stable error codes, not free-text). Reads of net
worth come from **materialized monthly report rows** (ADR-0006/0012), rebuilt from snapshots — not
recomputed per request.

## Backend package map (`backend/`)

| Package | Role |
| ------- | ---- |
| `cmd/balances/` | main: config load, pool, wire handlers, goose migrate, serve; also the e2e-seed / reset subcommands |
| `internal/httpserver/` | router assembly, middleware chain, security headers, SPA static serving |
| `internal/auth/` | Google OAuth + local password (ADR-0017/0039), server-side sessions, onboarding gate (ADR-0038), household/invitations |
| `internal/assets/`, `liabilities/`, `receivables/`, `investments/`, `income/` | the domain groups (`CONTEXT.md`) — one HTTP handler package each |
| `internal/reports/` | materialized monthly net-worth reports (ADR-0006) |
| `internal/tags/`, `fxrates/` | user-defined position tags (ADR-0028); multi-currency FX rates (ADR-0002) |
| `internal/backup/` | household export / restore / erasure (ADR-0036/0040); demo reset (ADR-0041) |
| `internal/repo/` | the data-access layer — one repo per group, wrapping sqlc queries |
| `internal/db/` | **generated** by sqlc (ADR-0018) — do not hand-edit |
| `internal/migrations/` | goose SQL migrations, embedded via `//go:embed`; immutable once released (ADR-0033) |
| `internal/config/` | env → typed `Config` (see `.env.example`) |
| `internal/httperr/`, `errs/` | the ADR-0027 error-code envelope and domain error taxonomy |
| `internal/email/` | SMTP/Resend mailer + i18n templates (ADR-0035); `NoopMailer` when mail is off |
| `internal/importcreate/`, `snapshotimport/` | the `.xlsx` position/snapshot import pipeline |
| `internal/assets`… (shared) `testutil/` | test-container helpers (ADR-0021) |
| `tools/` | codegen run by `make`: `gen-routes` (this repo's route doc), `gen-ts-types` (#365), `qa-matrix` |

## Frontend map (`frontend/src/`)

| Path | Role |
| ---- | ---- |
| `main.tsx`, `App.tsx` | entry + React Router routes and the sidebar shell (ADR-0025) |
| `api/` | typed API client; `generated.types.ts` mirrors the Go wire structs (#365, `make backend-gen-ts-types`) |
| `components/` | shared UI (shadcn/ui, ADR-0015); `positionList/` is the descriptor-driven list engine (ADR-0043) |
| `assets/`, and the per-group screens | the domain screens |
| `hooks/` | TanStack Query hooks (server state; no global store) |
| `i18n/`, `locales/` | react-i18next; pre-auth language picker + persisted user locale (ADR-0026/0035) |
| `theme/`, `lib/` | theming; pure helpers (the vitest-covered slice, see README) |

## Cross-cutting invariants

- **Tenancy (ADR-0005):** every query is scoped by `household_id`. This is the single most important
  invariant — the QA matrix's TENANCY zone guards it (`docs/qa/`).
- **Soft delete (ADR-0007):** domain mutations set `deleted_at`; they don't hard-delete.
- **Snapshots are the source of truth (ADR-0001/0022):** net worth is derived from month-end
  snapshots, never from a running transaction ledger. Investment ledgers exist only for cost-basis /
  income (ADR-0003/0008/0023).
- **Decimal money (ADR-0011):** amounts/quantities/FX are fixed-precision decimals, never floats.
- **Upgrade contract (ADR-0033):** released migrations are immutable; the binary version gates them.

## Where to go next

- Run it locally: [`README.md`](../README.md) → `make setup`.
- Contribute: [`CONTRIBUTING.md`](../CONTRIBUTING.md).
- How work flows idea→release: [`docs/agents/sdlc.md`](agents/sdlc.md).
- Current state / roadmap: [`HANDOFF.md`](../HANDOFF.md), [`docs/ROADMAP.md`](ROADMAP.md).
