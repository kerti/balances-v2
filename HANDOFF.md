# Handoff — pick this up cold

You are an agent resuming work on **balances-v2**. This document is the live state of the project:
what's true now, what's next, the conventions to keep, and the deferred backlog. Pair it with the
durable design docs (`CONTEXT.md`, `docs/adr/*`, `docs/ROADMAP.md`).

For the detail behind anything already shipped, the record now lives in **GitHub issues + PRs** and
the **GitHub Releases** notes (per tag) — not in a hand-maintained changelog. The pre-alpha
blow-by-blow journal is frozen at `docs/history/CHANGELOG-pre-alpha.md` (see ADR-0029).

Read these first, in order:
1. `CLAUDE.md` (project instructions; points to `docs/agents/*`)
2. `docs/ROADMAP.md` (six milestones)
3. `CONTEXT.md` (domain language)
4. This document
5. `docs/adr/*` (design decisions; skim the index, read the ones touching your task)
6. Closed GitHub issues / Releases (when you need the detail of an already-shipped item)
7. `git log --oneline -20` (most recent direction)

## Where we are now

M1–M5 are complete; **M6 (v1 polish) is nearly closed.** CI is green. **`v0.6.0-alpha.1` is
DEPLOYED** to the `preview` environment (`https://preview.<personal-domain>`) via the tag-driven
pipeline (ADR-0029/0030/0031). Single-origin: one Fly app (region `sin`) serves the SPA + `/api`;
Neon Postgres (preview branch), Resend mail, Google OAuth (Testing mode). Custom domain on Cloudflare
DNS-only with Fly-managed TLS.

- **M1–M3** — walking skeleton, Google OAuth + invites, first vertical slice (bank-account asset
  with snapshots), all tenancy-tested.
- **M4.1** — property + vehicle asset subtypes through the full stack; two-level nav; Title Case.
- **M4.2** — liability + receivable groups end-to-end.
- **M4.3** — investments group, all five subtypes (stock, mutual_fund, gold, bond, time_deposit).
- **M4.4** — investment transaction ledger (Buy/Sell/Coupon/Dividend/Distribution/Fee/Maturity).
- **M4.5** — Income: a flat flow-event entity (no subtype/snapshots/transactions/lifecycle).
- **M4.6** — position lifecycle UI (status / terminated_at) across all groups.
- **M5** — materialized monthly net-worth report + dashboard (net-worth headline,
  comprehensive-income lines, side-by-side currency display, Q15c).

**M6 shipped** (detail in the matching closed issue / the alpha release notes):

- **Importer & owner UX** — xlsx snapshot importer (all 10 groups + 5 investment subtypes); self-set
  `users.nickname`; Google profile-picture avatar (`users.picture_url`); list-screen polish swept
  across all 10 groups.
- **Routing & nav** — React Router migration + shadcn Sidebar shell (ADR-0025); fixes mobile tab
  overflow.
- **Internationalization (EN+ID)** — full `t()` sweep across every screen, issues #5–#11 (chrome,
  dashboard, bank-accounts template, properties/vehicles, liabilities/receivables, income,
  investments); e2e locale pin #12; bundled-resource i18next init; canonical `docs/glossary-id.md`.
- **Backend error-code envelope** (ADR-0027, issue #13) — `{code, args}` wire shape, `internal/httperr`
  + `internal/errs`; frontend envelope sweep with `errors:code.*` catalogs.
- **Investment analytics** — cost-basis + unrealised P/L headlines and time graphs, cross-subtype
  `/investments` dashboard, continuous month-walk graphs, closed-position handling (issues #14, #18,
  #21, #22, #24); backend `cost_basis` on ListItems + `GET /api/investments/time-series`.
- **Investment correctness** — capital excluded at entry and exit; truthful 0-value close snapshots;
  maturity/rollover return-continuity; rollover helper + successor linking (issues #16, #17, #25,
  #27, #29, #61). ADR-0008/0009 amended.
- **Valuations & taxonomy** — gold marked at buyback price (#19); mutual-fund `fund_type` enum (#20);
  property/vehicle revaluation-rate helper (rename → `annual_appreciation_rate`).
- **Guidance & approachability** — driver.js guided help tours on all detail screens (#23) + e2e
  (#26); built-in instruction copy EN+ID.
- **UX polish** — date 4-digit-year caps (#15); month-picker popover; position-control buttons (#31);
  fee cash→quantity helper (Q12); faster dev-server restart (#30).
- **Theming & brand** — per-user theme switcher (#33); logo / brand mark (`docs/brand/`).
- **User-defined position Tags** (ADR-0028, issue #28) — household-scoped grouping label, ≤1 per
  position; `PUT /api/tags/assignments` + `GET /api/tags/breakdown`; Settings card + `/tags` report.
- **Migration baseline** (ADR-0031) — 25 incremental migrations squashed to one `00001_baseline.sql`;
  existing DBs' goose markers collapsed in place after a zero-drift check.
- **Security & CI** — CodeQL SAST + govulncheck + Dependabot; path-gated CI with a `ci-gate`
  aggregator giving one stable status for future branch protection. Coverage thresholds
  informational until alpha. **Reassess deferred security items before alpha** (`docs/ci-tooling.md`).
- **Sidebar footer** (#75) — app version + deploy-env chip + GitHub/maintainer links; version & env
  baked into the SPA bundle at build via `deploy.yml --build-arg` → Dockerfile `ARG` → Vite `VITE_*`.
- **Toast feedback for buttonless autosaves** (#54, ADR-0032) — sonner `<Toaster>` at root;
  Tag-dropdown / Language / Appearance selects confirm via accent toast (errors stay destructive,
  optimistic value rolls back on failure).

## What's next

Alpha is deployed; M6 effectively closes with it. Open work, in rough priority:

- **Alpha bug fixes** (dogfood targets) — #53 (tag assign not reflected), #56 (maturity snapshot not
  instant), #58 (maturity-date edit doesn't update termination) are the recommended alpha blockers;
  #57 (edit snapshot month-year) is a fast-follow. Fix via branch → PR → squash-merge.
- **PDF export** of monthly reports (Q22) — still open.
- **Hardening / follow-ups** — bump `actions/checkout` (Node 20 deprecation), add an HSTS header,
  wire the `cloudflared` dev-tunnel (`make dev-tunnel`), document DB backup/restore (Neon branch +
  `pg_dump`), and the deferred security items (#70: e2e-in-CI, SHA-pin actions, gitleaks).
- **demo / production** — stand up when a beta/RC exists; same image, add Cloudflare *in front*
  (proxied) for CDN/WAF (ADR-0030).

**Deploying:** push a SemVer tag — `v0.6.0-alpha.N` → `preview` (auto). `deploy.yml` routes by tag and
runs `flyctl deploy` (builds the SPA+API image, `goose up` via `release_command`, rolls out). Backend
runtime secrets live on Fly (`fly secrets`); only `FLY_API_TOKEN` is in the GitHub `preview`
environment.

Don't auto-start the next item — the user pauses between items to direct. The deferred backlog below
holds the smaller, optional items.

## Conventions to keep, not to break

These are not ADRs because they're tactical, but they're load-bearing:

- **One snapshot table per position group** (ADR-0022). Don't try to merge them or build a
  polymorphic snapshot table.
- **Belt + suspenders tenancy.** Every SQL query that touches a position-related table filters by
  `household_id` *in SQL*, not just in middleware. Snapshot queries JOIN the parent table to verify
  ownership. See `backend/queries/asset_snapshots.sql` for the pattern.
- **Subtype guards.** When an entity is in a shared table (`assets` and `investments`),
  `Delete{Subtype}` and `Update{Subtype}` must verify the subtype before mutating. See
  `DeleteBankAccount` calling `GetBankAccount` first, and `DeleteStock` calling `GetStock` first.
- **Investment subtype→snapshot-shape validation lives in the repo, not the DB.**
  `validateInvestmentSnapshotShape(subtype, quantity, pricePerUnit, accruedInterest)` switches on
  subtype and returns `ErrInvalidSnapshotShape` if the value-column combo is wrong. The DB's CHECK
  only enforces "exactly one shape." When adding a new investment subtype, update both the switch in
  this helper and the `subtype` CHECK in the baseline migration's investments table.
- **No transaction wrapping** in `Create{Liability|Receivable}` because there's no extension table
  to also write. **Wrap in `pool.Begin` when there is** (e.g., `CreateBankAccount` writes assets +
  bank_account_details). This applies to all five investment subtypes.
- **Snapshot UI is split by shape (three forks).** Amount-only (asset, liability, receivable) →
  `Create/EditSnapshotDialog` + `SnapshotRow`. Quantity+price (stock, mutual_fund, gold) →
  `Create/EditQuantityPriceSnapshotDialog` + `QuantityPriceSnapshotRow`. Accrued-interest (bond,
  time_deposit) → `Create/EditAccruedInterestSnapshotDialog` + `AccruedInterestSnapshotRow`. Each
  fork's `useMutation` is owned by the parent detail page and passed in as props. The convention is
  **name by shape, not by group** — if a new subtype shares a shape, reuse its dialog set; if a new
  shape appears, fork by shape.
- **Transaction UI is split by shape (four forks).** Trade (Buy/Sell) →
  `Create/EditTradeTransactionDialog`; CashIncome (Coupon/Dividend/Distribution) →
  `Create/EditCashIncomeTransactionDialog`; Fee → `Create/EditFeeTransactionDialog`; Maturity →
  `Create/EditMaturityTransactionDialog`. **One shared `TransactionRow`** routes to the right Edit
  dialog via switch on `transaction.transaction_type` because the backend update endpoint is unified
  (one route, one updateMutation per page). Dialogs within a shape that cover multiple types take a
  `txnType` prop rather than splitting per type. If a new transaction shape appears, fork by shape
  and add a new `Edit*Dialog` branch to `TransactionRow`.
- **Income is a flat flow event, distinct from positions.** No subtype, no extension tables, no
  snapshots, no transactions, no lifecycle (`status`/`terminated_at`/`termination_note`). The
  mass-noun route lives at `/api/income` (singular collection) — diverges from the plural-collection
  convention elsewhere because "incomes" reads as a count noun we don't intend. Ownership defaults
  to **Sole + current user** in the Create dialog (vs the position-level Joint default) — the
  salary-dominant income case argued for the divergence (M4.5 grilling). Category is mutable
  post-create because all categories share one row shape (unlike
  `investment_transactions.transaction_type` which would invalidate the DB CHECK). When adding new
  income categories: extend the income CHECK in the baseline migration, the validator `oneof=…` tag in
  both `createReq` and `updateReq` in `internal/income/income.go`, and the `IncomeCategory` union +
  `CATEGORY_LABEL` map in the frontend.
- **Transaction validation is two-layer.** DB CHECK enforces type→shape integrity (e.g., `buy/sell`
  rows must have quantity AND price_per_unit). The repo's
  `validateInvestmentTransactionType(subtype, type)` enforces the subtype→type matrix (e.g.,
  `Coupon` is only allowed on Bond); `validateInvestmentTransactionShape` re-checks the shape combo
  with friendlier error messages. When adding a new transaction type or subtype: update the
  type-enum CHECK in the baseline migration's investment_transactions table, the per-type WHEN branch
  in the same CHECK, and the `allowed` matrix + switch in the two repo helpers. Each surfaces as
  `ErrInvalidTransactionType` or `ErrInvalidTransactionShape`, both 400.
- **`transaction_type` is immutable post-create.** Update payload omits it. To change a
  transaction's type, delete and re-create — changing it would invalidate the shape.
- **`SnapshotChart` is shared.** Don't fork it per group — it's already generic over `{year_month,
  amount}[]`.
- **Title Case** for nav labels, page H1s, data-section card titles. **Sentence case** for
  descriptions, empty-state messages, verb-phrase button labels. See M4.1 close commit for examples.
- **Routing is React Router** (ADR-0025). URLs mirror the domain hierarchy; every path comes from
  `src/lib/routes.ts` constants/builders, never a literal string — that's the deliberate link-safety
  convention (the stand-in for a type-safe router). Screens/details stay router-unaware (their
  `onSelect`/`onBack`/id-prop contract is unchanged); the `ListRoute`/`DetailRoute` wrappers in
  `App.tsx` bridge them to `useNavigate`/`useParams`. Adding a route = add a `routes.ts` entry + one
  wrapper line in the router config; don't reach for `useNavigate` inside a screen.
- **Nav is the shadcn Sidebar** (`AppSidebar`, data-driven from a single `NAV` array): persistent on
  desktop, drawer on phones. Subtyped groups (Assets, Liabilities, Investments) show always-expanded
  sub-items and get a placeholder **group home** page (`/assets`, `/liabilities`, `/investments`) —
  stubs for the future per-group dashboards. Flat groups (Receivables, Income) list at their root
  path, no home. Liability **detail nests under its subtype** (`/liabilities/personal/:id`) so the
  dynamic `:id` never overlaps the literal subtype segments. Add a destination = add it to `NAV`.
- **E2E navigates by URL.** Specs `goto('/path')` to enter a screen; for mid-test nav that must avoid
  a reload, click persistent sidebar `link`s (the old `getByRole('tab', …)` nav is gone). See
  `rebuild.spec` (preserves client-side `['reports']` invalidation) and `currency-display.spec`.
- **React Query useEffect gotcha.** Never put a `useMutation` result in a `useEffect` deps array —
  it's recreated every render and will loop. There's a comment to this effect in
  `EditSnapshotDialog`; replicate the pattern when needed.
- **Decimals are strings on the wire**, `decimal.Decimal` in Go, with DECIMAL(20,4) for amounts and
  DECIMAL(20,8) for rates/FX. ADR-0011.
- **Rates are stored as percentage** (e.g., `5.5` for 5.5%), not as decimal fraction. Frontend
  reads/writes the same number the user sees on screen — no client-side scaling. Applies to
  `liabilities.interest_rate`, `property_details.annual_appreciation_rate`,
  `vehicle_details.annual_depreciation_rate`, `bond_details.coupon_rate`,
  `time_deposit_details.interest_rate`.
- **Maturity urgency styling** (`lib/maturity.ts`): 4-tier — default (>90d, muted), approaching
  (≤90d, bold), imminent (≤30d, bold + amber, countdown format), matured (muted + ⚠ prefix). Bond +
  TimeDeposit list rows + detail pages share this helper. Don't reinvent the date-comparison logic
  inline.
- **Soft-delete everything**, including snapshots. ADR-0007. Hard-delete is not a UI feature — "can
  be undone via the database" is the line we use in confirm dialogs.
- **Backend lint is enforced.** `golangci-lint run` from `backend/` must be clean. Config at repo
  root in `.golangci.yml`. `revive`'s `exported` and `package-comments` rules are deliberately
  disabled — don't reintroduce godoc-comment-on-every-export expectations for application code. New
  shared blank imports (e.g. SQL drivers) need a justifying comment.
- **Frontend lint is enforced.** `npm run lint` from `frontend/` must be clean.
  `react-refresh/only-export-components` is disabled for `components/ui/**` (shadcn-generated).
  `react-hooks/set-state-in-effect` is enforced everywhere else — no `setState` inside `useEffect`
  body.
- **Indonesian copy follows `docs/glossary-id.md`.** That file is the canonical EN↔ID dictionary
  (Liability→Liabilitas, Receivable→Piutang, Snapshot stays English, etc.). When a new term lands,
  extend the glossary in the same PR — don't decide translations inline in catalog JSON.
- **Pagination clamp is derived during render**, not done in an effect. Pattern: `const
  effectivePage = Math.min(page, totalPages)`. Use `effectivePage` for slicing and for the
  `PaginationControls page` prop; keep raw `setPage` for click handlers. Don't reintroduce
  `useEffect(() => if (page > totalPages) setPage(totalPages))`.
- **Edit dialogs do not reset form state via `useEffect`.** Initial form state comes from the entity
  prop in `useState(() => toForm(entity))` or inline initializer. Parents pass `key={entity.id}` so
  React remounts the dialog on entity switch. Within the same entity, form state persists across
  open/cancel/reopen — by design.
- **Defer cleanup that returns an error must swallow it explicitly**: `defer func() { _ =
  tx.Rollback(ctx) }()`. Applies to `pgxpool.Tx.Rollback` and `sql.DB.Close()`. errcheck catches the
  bare form.
- **E2E selectors use `data-testid` over structural DOM traversal.** Playwright specs target
  interacted/asserted elements via `page.getByTestId('...')` with a matching `data-testid` on the DOM
  node, never tag/CSS locators or `.filter({hasText})` chains. Test IDs are an explicit
  component↔spec contract that survives copy edits, restyling, and shadcn quirks (e.g. `CardTitle` is
  a `<div>`, not a heading). **No spec uses `page.locator()` structural selectors.** Stable
  role/label selectors (`getByRole('button'|'link')`, `getByLabel` on properly-associated inputs) and
  `getByText` for stable copy are fine to keep; the point is to ban brittle structural traversal, not
  to testid every button. When you add a new structural-locator need, add a test id instead.
- **Tenancy test pattern**: every position group's `*_tenancy_test.go` covers both the cross-tenant
  rejection path (bob attempts X, expects `ErrNotFound`) and the alice-side happy-path CRUD success
  (update + delete on entity and snapshot, then verify Get/List). Cross-tenant alone leaves
  `Update*`/`Delete*`/`softDeleteAsset` success branches uncovered because the rejection
  short-circuits at the GetX guard. **List must be tested with the entity still present** (alice
  creates entity + snapshot, then lists, asserts shape) — testing only the post-delete empty list
  leaves the detail+snapshot join loop in `List*` unexercised.
- **HTTP error responses ship the ADR-0027 envelope.** Every 4xx/5xx from `internal/*` goes through
  `internal/httperr` (`Write` / `WriteRepo` / `WriteValidation`) and ships
  `{"code": "<CODE>", "args": {...}}` — never raw `http.Error(...)`. Codes are the wire contract;
  human copy lives in the FE i18n catalogs (`errors:code.<CODE>`); no `message` field on the wire.
  Sentinel error vars live in `internal/errs` (leaf, dependency-free); `internal/repo/errors.go`
  re-exports them via aliases so `repo.ErrFoo` keeps working at call sites. **Exceptions:** the
  OAuth callback flow in `internal/auth/handlers.go:handleCallback` (redirect-based) and the
  mock OIDC subcommand in `cmd/balances/mockoidc.go` (dev-only) keep their plain `http.Error`
  bodies. New handlers reach for `httperr.Write(w, status, code, args)`, not `http.Error`. New
  validator-emitted errors need only the catalog entry — `WriteValidation` handles the field/rule
  extraction via the JSON-tag-name func registered by `httperr.NewValidator()`. Repo's
  `ErrUnauthenticated` stays deliberately unmapped (RequireAuth gates every route, so a repo
  seeing no user is a server bug, not a client error — falls through to 500 INTERNAL).
  Adding a new code: declare it in `internal/httperr/codes.go` + emit it + add the catalog entry
  in both locales.

## Things explicitly NOT to do

- **Don't autoflush commits.** When work seems ready, stage + show the diff + ask. Push only on
  explicit green light. After every push, watch CI to completion (`gh run list --branch <branch>` /
  `gh run watch <id>`); if a workflow fails, surface the failure with logs and ask the user whether
  to fix now or defer. Don't declare a commit done while runs are still queued or in_progress.
- **Don't dive into UI alone.** User has near-zero frontend skill and relies heavily on you for UI —
  but expects to be consulted on UX choices (form density, navigation, button labels). Always
  surface tradeoffs.
- **Don't fear backtracking on prior decisions** if they're suboptimal — pre-alpha migrations are
  not sacred. User explicitly accepted this. Flag the issue, propose the better path, let user
  decide.
- **Don't create planning/analysis documents** unless asked. Live state goes in this file or in
  memory; design decisions go in ADRs; nothing else.
- **Don't bypass `--no-verify` or `--no-gpg-sign`** on git commits.
- **Don't add features beyond the task.** No speculative abstractions. Three similar lines beats
  premature abstraction.
- **Don't add comments that just restate the code.** Only add when WHY is non-obvious.
- **Don't auto-start the next milestone** without explicit user instruction. User pauses between
  milestones to direct.

## How to run locally

```bash
# Backend
cd /Users/rad/Documents/Code/src/balances-v2
set -a && source .env && set +a
cd backend && go run ./cmd/balances serve

# Frontend (separate terminal)
cd frontend && npm run dev

# Migrations (auto-run on serve, but to run manually)
cd backend && go run ./cmd/balances migrate up
```

The backend is `serve`, not `server`. There is **no dev-login backdoor** — auth is real Google
OAuth. For backend smoke tests against authenticated endpoints, pull a current session token from
the `sessions` table:

```bash
docker exec balances-v2-postgres-1 psql -U balances -d balances \
  -c "SELECT s.id as token FROM sessions s WHERE s.expires_at > now() LIMIT 1;"
```

Pass via `Cookie: session=<token>` header.

## Lint locally before pushing

```bash
# Backend
cd backend && golangci-lint run

# Frontend
cd frontend && npm run lint
```

CI runs both on every push. golangci-lint config is at `.golangci.yml` (repo root); ESLint config is
`frontend/eslint.config.js`. The Codecov config (`codecov.yml`) keeps coverage status
informational-only — failing CI from coverage drops is a deliberate non-goal until alpha.

## Deferred backlog

Tracked in GitHub now, not here — filter the [`backlog`](https://github.com/kerti/balances-v2/labels/backlog)
and [`security`](https://github.com/kerti/balances-v2/labels/security) labels. Migrated from this doc
on 2026-06-10: #65 (link existing TD as rollover successor), #66 (per-bond coupon disposition),
#67 (transaction-list aggregations), #68 (gold purity UX), #69 (component tests RTL/MSW),
#70 (pre-alpha security hardening — e2e-in-CI / SHA-pin actions / gitleaks). Full original wording of
already-resolved items is in `docs/history/CHANGELOG-pre-alpha.md`.

## Updating this document

Keep it a **live-state pointer**: current status, what's next, conventions, deferred backlog —
not a journal. When you close a milestone or cut a release, update this file in the same commit and
don't let it drift more than one milestone behind reality.

Shipped detail does **not** go here — it lives in the closed issue / PR and the GitHub Release notes
(per ADR-0029). At each release (tag), **prune the shipped bullets** from "Where we are now" down to
one-line-per-theme, since the granular record is now in the release; keep this file focused on
in-progress / next-up. Hard-wrap prose at ~100 columns so the file stays diff-friendly.
