# Codebase conventions

Tactical rules that are load-bearing but don't rise to an ADR: if you break one, something quietly
stops working or a future change gets harder. ADRs (`docs/adr/`) hold the decisions with alternatives
and consequences; `CONTEXT.md` holds the domain language; this file holds the "when you touch X, also
do Y" knowledge that has nowhere else to live.

Split out of `HANDOFF.md` (ADR-0029 amendment, 2026-08-04) — these are project instruction, not live
working state, and they belong next to the other `docs/agents/*` references CLAUDE.md indexes.

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

