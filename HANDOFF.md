# Handoff — pick this up cold

You are an agent resuming work on **balances-v2**. This document is the live state: what's true now,
what's next, the conventions to keep, the deferred backlog. Pair it with the durable design docs
(`CONTEXT.md`, `docs/adr/*`, `docs/ROADMAP.md`).

For detail behind anything shipped, the record lives in **GitHub issues + PRs** and the **GitHub
Releases** notes (per tag) — not a hand-maintained changelog. The pre-alpha journal is frozen at
`docs/history/CHANGELOG-pre-alpha.md` (ADR-0029).

Read these first, in order:
1. `CLAUDE.md` (project instructions; points to `docs/agents/*`)
2. `docs/ROADMAP.md` (six milestones)
3. `CONTEXT.md` (domain language)
4. This document
5. `docs/adr/README.md` (one-line ADR index — open the ones touching your task)
6. Closed GitHub issues / Releases (detail of an already-shipped item)
7. `git log --oneline -20` (most recent direction)

## Where we are now

M1–M5 complete; **M6 (v1 polish) is closed** — fully landed with alpha.5 (the 0.6 line). CI is green.
**`v0.7.0-alpha.5` is the latest preview release**; **`v0.7.0-rc.1` is demo's current release** (M7
line) on the `preview`/`demo` environments (`https://preview.<personal-domain>` /
`https://demo.<personal-domain>`) via the tag-driven pipeline (ADR-0029/0030/0031). Single-origin: one
Fly app per environment (region `sin`) serves the SPA + `/api`; Neon Postgres (per-env branch), Resend
mail, Google + optional local OAuth. Custom domain on Cloudflare DNS-only with Fly-managed TLS.

- **M1–M5** (closed) — walking skeleton → OAuth + invites → all four position groups + five investment
  subtypes + transaction ledger + Income + position lifecycle → materialized monthly net-worth report
  + dashboard. All tenancy-tested. Detail in closed issues + Release notes.
- **M6** (closed at alpha.5) — v1 polish + approachability: xlsx importer, React Router + shadcn
  Sidebar (ADR-0025), EN+ID i18n (ADR-0035), error-code envelope (ADR-0027), investment analytics
  (ADR-0008/0009), position Tags (ADR-0028), migration baseline (ADR-0031), whole-household
  backup/restore (epic #52, ADR-0036), QA invariant matrix (19 zones/103 invariants), group-Home
  parity (#204). Migrations: `00002`–`00005` (additive). Detail in closed issues + the
  alpha.1–alpha.5 Release notes.
- **M7** (open, productization) — one line per tag; full detail lives in each tag's Release notes:
  - `v0.7.0-alpha.1` — self-host stack, `docker-compose.yml` + `APP_URL` collapse (#116, ADR-0037). No migration.
  - `v0.7.0-alpha.2` — self-host rehearsal fixes: multi-arch image, deep-route 404 (#241/#242). No migration.
  - `v0.7.0-alpha.3` — coupon disposition + first backup format transform (#66). Migration: additive (`00006`).
  - `v0.7.0-alpha.4` — onboarding gate (#158, ADR-0038), local password auth epic (#277, ADR-0039),
    household erasure (#300, ADR-0040), `FOUNDING_DISABLED` (#302). Migration: additive (`00007`–`00010`).
  - `v0.7.0-rc.1` — demo's first release; promotes the `alpha.4` commit verbatim, no changes. First
    cut on the `*-rc.N` → `demo` routing.
  - `v0.7.0-alpha.5` — demo shared-account auth + Erasure block + nightly-reset endpoint (#217,
    ADR-0041). No migration. Pending: promote to `rc.2` for demo, GitHub Actions nightly-reset cron,
    end-to-end verify (see below).

## What's next

**M6 closed (alpha.5, the 0.6 line); M7 (productization) is open (latest preview release
v0.7.0-alpha.5; latest demo release v0.7.0-rc.1).** Next, in order:

1. **M7 = productization (now the active line).** Onboarding gate, local password auth, and household
   erasure all shipped in alpha.4 (see above) — remaining M7 gate items: **demo standup** (below),
   production Resend domain, **#93** landing. Prod itself is **deferred indefinitely** (2026-07-02) —
   see the data-protection note below. See ROADMAP M7.
2. **M8 = next domain features**, prioritized by real-user feedback from M7 (not pre-specified).
   Includes the M6→M8 pivot of **PDF export (#187)**. See ROADMAP M8.

**Demo/prod launch prep (prod deferred indefinitely as of 2026-07-02; demo is the active line):** #215
subdomain scheme — **decided: nested product subtree** (`app.balances.<domain>` prod unmarked,
`balances.<domain>` landing, `preview.`/`demo.` siblings), **DNS-only never proxied**; preview
migrated, demo DNS in progress. #216 single Resend sending domain — **DONE & closed**. #218 rescoped
2026-07-02 — prod's Neon-isolation + PITR-retention decision (incl. the erasure-purge window) parks
with prod; demo instead follows ADR-0030's already-decided single-project-per-env-branch shape (no
isolation): Neon `demo` branch, Fly app `balances-demo`, GitHub Environment `demo` all provisioned.
#217 demo readiness — OAuth consolidated under one new GCP project (dev/preview/demo clients,
consent screen **published to Production**, preview re-verified on the new client first) — **DONE**.
Guest auth / nightly reset (shared-account auth, Erasure block, `DEMO_MODE`, reset endpoint) shipped
in code at `v0.7.0-alpha.5` (ADR-0041) — demo Household founded with the shared local-auth
credentials, `FOUNDING_DISABLED`/`DEMO_MODE`/`DEMO_RESET_TOKEN`/`AUTH_LOCAL_ENABLED` Fly secrets set on
`balances-demo`, but **inert until promoted to demo** (the running build still predates this code).
Remaining: promote to `rc.2`, write the GitHub Actions nightly-reset cron, verify end-to-end, then
close #217. DNS (`demo.balances.<domain>`) set. Feeds M7.

**Production SaaS data-protection decision (2026-07-02):** #222 (originally: maintainer structurally
unable to read any user data — zero-knowledge encryption) closed as disproportionate; conflicts with
core server-side aggregation (monthly reports) and isn't what GDPR requires. Decided: ordinary GDPR
compliance is sufficient — lawful basis, privacy policy naming subprocessors, honoring access/erasure
requests, bounded breach process. Rescoped into **#299** (privacy policy — still open) and **#300**
(household erasure "DELETE ME" — shipped alpha.4, see above, ADR-0040). Access/portability already
satisfied by the backup/export epic (#52). Self-host (#116) remains the zero-exposure option for
anyone unwilling to accept hosted SaaS. **Prod itself stays deferred indefinitely** — the
"non-disposable environment" M7 gate item doesn't apply until prod is unparked; demo (below) is the
closest thing to a public-facing env for now.

Smaller open items ride a convenient batch, not their own cut: #163 (email wordmark raster).
Hardening follow-ups: `actions/checkout` Node-20 bump, HSTS header, `cloudflared` dev-tunnel.

**Label convention (release notes):** every PR carries exactly one type label at merge —
`enhancement`/`bug`/`documentation`/`dependencies`. Test-only and CI/dev/build tooling PRs go under
**`enhancement`** (decided 2026-06-17 — no dedicated `chore`/`test` label).

**demo / production** — first prod is **not** pinned to `v1.0.0` (ADR-0033 amended 2026-07-02): it
lands on whatever `0.x` minor is current when prod actually unparks. SemVer = operator upgrade
contract, not the "Balances" brand; migration immutability + major-vs-minor discipline switch on at
*first production deploy*, not a specific number. Self-host (#116, the prior blocker) is done/closed.
Milestone-close still rolls to the next minor's alpha (M6→M7 precedent) unless a milestone happens to
coincide with dropping the suffix for a real production cut.

**Deploying:** push a SemVer tag — `*-alpha.N` → `preview`, `*-rc.N`/`*-beta.N` → `demo` (both auto).
`deploy.yml` routes by tag and runs `flyctl deploy` (builds the SPA+API image, `goose up` via
`release_command`, rolls out). Backend runtime secrets live on Fly (`fly secrets`); only
`FLY_API_TOKEN` is in each env's GitHub Environment (`preview`, `demo`).

Don't auto-start the next item — the user pauses between items to direct. The deferred backlog below
holds the smaller, optional items.

## Conventions to keep, not to break

Not ADRs because they're tactical, but load-bearing:

- **One snapshot table per position group** (ADR-0022). Don't merge them or build a polymorphic
  snapshot table.
- **Belt + suspenders tenancy.** Every SQL query touching a position-related table filters by
  `household_id` *in SQL*, not just middleware. Snapshot queries JOIN the parent table to verify
  ownership. Pattern: `backend/queries/asset_snapshots.sql`.
- **Subtype guards.** For entities in a shared table (`assets`, `investments`), `Delete{Subtype}` and
  `Update{Subtype}` must verify the subtype before mutating. See `DeleteBankAccount` calling
  `GetBankAccount` first, `DeleteStock` calling `GetStock` first.
- **Investment subtype→snapshot-shape validation lives in the repo, not the DB.**
  `validateInvestmentSnapshotShape(subtype, quantity, pricePerUnit, accruedInterest)` switches on
  subtype and returns `ErrInvalidSnapshotShape` on a wrong value-column combo. The DB CHECK only
  enforces "exactly one shape." Adding a subtype: update both this switch and the `subtype` CHECK in
  the baseline migration's investments table.
- **Transaction wrapping.** No `pool.Begin` in `Create{Liability|Receivable}` (no extension table to
  also write). **Wrap when there is** (e.g. `CreateBankAccount` writes assets + bank_account_details).
  Applies to all five investment subtypes.
- **Snapshot UI is split by shape (three forks).** Amount-only (asset, liability, receivable) →
  `Create/EditSnapshotDialog` + `SnapshotRow`. Quantity+price (stock, mutual_fund, gold) →
  `Create/EditQuantityPriceSnapshotDialog` + `QuantityPriceSnapshotRow`. Accrued-interest (bond,
  time_deposit) → `Create/EditAccruedInterestSnapshotDialog` + `AccruedInterestSnapshotRow`. Each
  fork's `useMutation` is owned by the parent detail page and passed in as props. Convention: **name
  by shape, not by group** — new subtype sharing a shape reuses its dialog set; new shape forks.
- **Transaction UI is split by shape (four forks).** Trade (Buy/Sell) →
  `Create/EditTradeTransactionDialog`; CashIncome (Coupon/Dividend/Distribution) →
  `Create/EditCashIncomeTransactionDialog`; Fee → `Create/EditFeeTransactionDialog`; Maturity →
  `Create/EditMaturityTransactionDialog`. **One shared `TransactionRow`** routes to the right Edit
  dialog via switch on `transaction.transaction_type` (the backend update endpoint is unified — one
  route, one updateMutation per page). Dialogs covering multiple types take a `txnType` prop rather
  than splitting per type. New shape → fork + add a branch to `TransactionRow`.
- **Income is a flat flow event, distinct from positions.** No subtype, extension tables, snapshots,
  transactions, or lifecycle (`status`/`terminated_at`/`termination_note`). The mass-noun route is
  `/api/income` (singular collection) — diverges from the plural convention because "incomes" reads as
  a count noun we don't intend. Ownership defaults to **Sole + current user** in the Create dialog (vs
  the position-level Joint default) — the salary-dominant case argued for it (M4.5 grilling). Category
  is mutable post-create (all categories share one row shape, unlike
  `investment_transactions.transaction_type` which would invalidate the DB CHECK). Adding income
  categories: extend the income CHECK in the baseline migration, the validator `oneof=…` tag in both
  `createReq` and `updateReq` in `internal/income/income.go`, the `IncomeCategory` union in
  `api/types.ts`, and the `categoryOptions.<key>` labels in both locale catalogs
  (`locales/{en,id}/income.json`) — no `CATEGORY_LABEL` TS map anymore (i18n sweep, #11). Note
  `regularity` (`routine`/`incidental`) is an independent stored field with its own `oneof` validator,
  not derived from category.
- **Transaction validation is two-layer.** DB CHECK enforces type→shape integrity (`buy/sell` rows
  need quantity AND price_per_unit). The repo's `validateInvestmentTransactionType(subtype, type)`
  enforces the subtype→type matrix (`Coupon` only on Bond); `validateInvestmentTransactionShape`
  re-checks the shape combo with friendlier messages. Adding a type or subtype: update the type-enum
  CHECK in the baseline migration's investment_transactions table, the per-type WHEN branch in the
  same CHECK, and the `allowed` matrix + switch in the two repo helpers. Surfaces as
  `ErrInvalidTransactionType` or `ErrInvalidTransactionShape`, both 400.
- **`transaction_type` is immutable post-create.** Update payload omits it. To change a type, delete
  and re-create — changing it would invalidate the shape.
- **`SnapshotChart` is shared.** Don't fork per group — it's already generic over `{year_month,
  amount}[]`.
- **Title Case** for nav labels, page H1s, data-section card titles. **Sentence case** for
  descriptions, empty-state messages, verb-phrase button labels. See M4.1 close commit.
- **Routing is React Router** (ADR-0025). URLs mirror the domain hierarchy; every path comes from
  `src/lib/routes.ts` constants/builders, never a literal string — the deliberate link-safety
  convention (stand-in for a type-safe router). Screens/details stay router-unaware (their
  `onSelect`/`onBack`/id-prop contract is unchanged); the `ListRoute`/`DetailRoute` wrappers in
  `App.tsx` bridge them to `useNavigate`/`useParams`. Adding a route = a `routes.ts` entry + one
  wrapper line; don't reach for `useNavigate` inside a screen.
- **Nav is the shadcn Sidebar** (`AppSidebar`, data-driven from a single `NAV` array): persistent on
  desktop, drawer on phones. Subtyped groups (Assets, Liabilities, Investments) show always-expanded
  sub-items and get a **group home** page (`/assets`, `/liabilities`, `/investments`). `/investments`
  is a real dashboard (`InvestmentsHome`, cost-basis + time-series + pie/stack charts, #14); `/assets`
  + `/liabilities` are still placeholder stubs awaiting per-group dashboards. Flat groups (Receivables,
  Income) list at their root path, no home. Liability **detail nests under its subtype**
  (`/liabilities/personal/:id`) so the dynamic `:id` never overlaps the literal subtype segments. Add
  a destination = add it to `NAV`.
- **E2E navigates by URL.** Specs `goto('/path')` to enter a screen; for mid-test nav that must avoid
  a reload, click persistent sidebar `link`s (the old `getByRole('tab', …)` nav is gone). See
  `rebuild.spec` (preserves client-side `['reports']` invalidation) and `currency-display.spec`.
- **Reports auto-invalidate after every write.** A global `MutationCache` in `main.tsx` calls
  `invalidateQueries({ queryKey: ['reports'] })` on every successful mutation, so monthly reports +
  dashboard regenerate lazily on next read (ADR-0006) without each hook opting in. Don't hand-wire
  per-screen `['reports']` invalidation; keep report-feeding queries under the `['reports']` key
  prefix so they're swept.
- **React Query useEffect gotcha.** Never put a `useMutation` result in a `useEffect` deps array —
  it's recreated every render and will loop. Edit dialogs sidestep this (no `useEffect`; form state
  seeded from the entity prop with `key={entity.id}` remount); keep it that way.
- **Decimals are strings on the wire**, `decimal.Decimal` in Go. Three precision shapes (ADR-0011):
  DECIMAL(20,4) for monetary amounts, DECIMAL(20,8) for instrument quantities **and** rates/FX. Lone
  exception: `gold_details.purity` is DECIMAL(5,4) (a 0–1 fraction). A new quantity column takes
  (20,8), not (20,4).
- **Rates are stored as percentage** (e.g. `5.5` for 5.5%), not a decimal fraction. Frontend
  reads/writes the same number the user sees — no client-side scaling. Applies to
  `liabilities.interest_rate`, `property_details.annual_appreciation_rate`,
  `vehicle_details.annual_depreciation_rate`, `bond_details.coupon_rate`,
  `time_deposit_details.interest_rate`.
- **Maturity urgency styling** (`lib/maturity.ts`): 4 states, 3 colour treatments — default (>90d,
  muted) and matured (<0d, muted + ⚠ prefix) share `text-muted-foreground`; approaching (≤90d, bold)
  and imminent (≤30d, bold + amber, countdown format) are the two distinct accents. States differ by
  label even where colour repeats. Used by **Bond + TimeDeposit list rows only** — detail pages
  dropped the inline urgency label (#55) and just show `formatDate(maturity_date)`. List rows
  **suppress the label when terminated** (`!terminated && …`). Don't reinvent the date-comparison
  logic inline.
- **Soft-delete everything**, including snapshots (ADR-0007). Hard-delete is not a UI feature — "can
  be undone via the database" is the line in confirm dialogs.
- **Backend lint is enforced.** `golangci-lint run` from `backend/` must be clean. Config at repo
  root in `.golangci.yml`. `revive`'s `exported` and `package-comments` rules are deliberately
  disabled — don't reintroduce godoc-on-every-export expectations for application code. New shared
  blank imports (e.g. SQL drivers) need a justifying comment.
- **Frontend lint is enforced.** `npm run lint` from `frontend/` must be clean.
  `react-refresh/only-export-components` is disabled for `components/ui/**` (shadcn-generated).
  `react-hooks/set-state-in-effect` is enforced everywhere else — no `setState` inside `useEffect`
  body.
- **Indonesian copy follows `docs/glossary-id.md`** — the canonical EN↔ID dictionary
  (Liability→Liabilitas, Receivable→Piutang, Snapshot stays English, etc.). New term lands → extend
  the glossary in the same PR; don't decide translations inline in catalog JSON.
- **Pagination clamp is derived during render**, not in an effect: `const effectivePage =
  Math.min(page, totalPages)`. Use `effectivePage` for slicing and the `PaginationControls page`
  prop; keep raw `setPage` for click handlers. Don't reintroduce `useEffect(() => if (page >
  totalPages) setPage(totalPages))`.
- **Edit dialogs do not reset form state via `useEffect`.** Initial form state comes from the entity
  prop in `useState(() => toForm(entity))` or inline initializer. Parents pass `key={entity.id}` so
  React remounts on entity switch. Within the same entity, form state persists across
  open/cancel/reopen — by design.
- **Defer cleanup that returns an error must swallow it explicitly**: `defer func() { _ =
  tx.Rollback(ctx) }()`. Applies to `pgxpool.Tx.Rollback` and `sql.DB.Close()`. errcheck catches the
  bare form.
- **E2E selectors use `data-testid` over structural DOM traversal.** Specs target interacted/asserted
  elements via `page.getByTestId('...')` with a matching `data-testid` on the DOM node, never tag/CSS
  locators or `.filter({hasText})` chains. Test IDs are an explicit component↔spec contract that
  survives copy edits, restyling, and shadcn quirks (e.g. `CardTitle` is a `<div>`, not a heading).
  **No spec uses `page.locator()` structural selectors.** Stable role/label selectors
  (`getByRole('button'|'link')`, `getByLabel` on properly-associated inputs) and `getByText` for
  stable copy are fine; the point is to ban brittle structural traversal, not to testid every button.
  New structural-locator need → add a test id. **Lone exception:** `theme.spec.ts` uses
  `page.locator('html')` to assert the dark-mode class on the root element (can't carry a test id).
- **Tenancy test pattern**: every group's `*_tenancy_test.go` covers both the cross-tenant rejection
  path (bob attempts X, expects `ErrNotFound`) and the alice-side happy-path CRUD success (update +
  delete on entity and snapshot, then verify Get/List). Cross-tenant alone leaves
  `Update*`/`Delete*`/`softDeleteAsset` success branches uncovered (the rejection short-circuits at
  the GetX guard). **List must be tested with the entity still present** (alice creates entity +
  snapshot, lists, asserts shape) — testing only the post-delete empty list leaves the
  detail+snapshot join loop in `List*` unexercised.
- **HTTP error responses ship the ADR-0027 envelope.** Every 4xx/5xx from `internal/*` goes through
  `internal/httperr` (`Write` / `WriteRepo` / `WriteValidation`) and ships `{"code": "<CODE>", "args":
  {...}}` — never raw `http.Error(...)`. Codes are the wire contract; human copy lives in the FE i18n
  catalogs (`errors:code.<CODE>`); no `message` field on the wire. Sentinel error vars live in
  `internal/errs` (leaf, dependency-free); `internal/repo/errors.go` re-exports them via aliases so
  `repo.ErrFoo` keeps working at call sites. **Exceptions:** the OAuth callback flow in
  `internal/auth/handlers.go:handleCallback` (redirect-based) and the mock OIDC subcommand in
  `cmd/balances/mockoidc.go` (dev-only) keep plain `http.Error` bodies. New handlers reach for
  `httperr.Write(w, status, code, args)`, not `http.Error`. New validator-emitted errors need only the
  catalog entry — `WriteValidation` handles field/rule extraction via the JSON-tag-name func
  registered by `httperr.NewValidator()`. Repo's `ErrUnauthenticated` stays deliberately unmapped
  (RequireAuth gates every route, so a repo seeing no user is a server bug, not a client error — falls
  through to 500 INTERNAL). Adding a code: declare it in `internal/httperr/codes.go` + emit it + add
  the catalog entry in both locales.

## Things explicitly NOT to do

- **Don't autoflush commits.** When work seems ready, stage + show the diff + ask. Push only on
  explicit green light. After every push, watch CI to completion (`gh run list --branch <branch>` /
  `gh run watch <id>`); if a workflow fails, surface the failure with logs and ask whether to fix now
  or defer. Don't declare a commit done while runs are queued or in_progress.
- **Don't dive into UI alone.** User has near-zero frontend skill and relies heavily on you for UI —
  but expects to be consulted on UX choices (form density, navigation, button labels). Always surface
  tradeoffs.
- **Don't fear backtracking on prior decisions** if suboptimal — pre-alpha migrations are not sacred.
  User explicitly accepted this. Flag the issue, propose the better path, let user decide.
- **Don't create planning/analysis documents** unless asked. Live state goes here or in memory;
  design decisions go in ADRs; nothing else.
- **Don't bypass `--no-verify` or `--no-gpg-sign`** on git commits.
- **Don't add features beyond the task.** No speculative abstractions. Three similar lines beats
  premature abstraction.
- **Don't add comments that just restate the code.** Only when WHY is non-obvious.
- **Don't auto-start the next milestone** without explicit user instruction. User pauses between
  milestones to direct.

## Running, linting, testing locally

See `docs/agents/dev.md` — Makefile-based run loop (`make up` / `make restart`), the
backend-restart-after-Go-edits gotcha, the session-token smoke-test recipe, lint, and the test
suites. `make help` lists every target.

## Deferred backlog

Tracked in GitHub now, not here — filter the [`backlog`](https://github.com/kerti/balances-v2/labels/backlog)
and [`security`](https://github.com/kerti/balances-v2/labels/security) labels. Migrated from this doc
on 2026-06-10: #65 (link existing TD as rollover successor), #66 (per-bond coupon disposition — pulled
forward as the #229 upgrade-leg migration vehicle), #67 (transaction-list aggregations), #68 (gold
purity UX), #69 (component tests RTL/MSW),
#70 (pre-alpha security hardening — e2e-in-CI / SHA-pin actions / gitleaks). Full original wording of
already-resolved items is in `docs/history/CHANGELOG-pre-alpha.md`.

## Updating this document

Keep it a **live-state pointer**: current status, what's next, conventions, deferred backlog — not a
journal. When you close a milestone or cut a release, update this file in the same commit and don't
let it drift more than one milestone behind reality.

Shipped detail does **not** go here — it lives in the closed issue / PR and the GitHub Release notes
(ADR-0029). At each release (tag), **prune the shipped bullets** in "Where we are now" down to
one-line-per-theme. Hard-wrap prose at ~100 columns so the file stays diff-friendly.
