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

M1–M7 complete; **M8 (next domain features) is now the active line** — first M8 alpha cut
2026-07-06. CI is green. **`v0.9.0-alpha.3` is the latest preview release** and, promoted manually
(ADR-0049), **demo's current release too** — on the `preview`/`demo` environments
(`https://preview.<personal-domain>` / `https://demo.<personal-domain>`) via the tag-driven pipeline
(ADR-0029/0030/0031, routing revised by 0049). Single-origin: one
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
- **M7** (closed, productization) — one line per tag; full detail lives in each tag's Release notes:
  - `v0.7.0-alpha.1` — self-host stack, `docker-compose.yml` + `APP_URL` collapse (#116, ADR-0037). No migration.
  - `v0.7.0-alpha.2` — self-host rehearsal fixes: multi-arch image, deep-route 404 (#241/#242). No migration.
  - `v0.7.0-alpha.3` — coupon disposition + first backup format transform (#66). Migration: additive (`00006`).
  - `v0.7.0-alpha.4` — onboarding gate (#158, ADR-0038), local password auth epic (#277, ADR-0039),
    household erasure (#300, ADR-0040), `FOUNDING_DISABLED` (#302). Migration: additive (`00007`–`00010`).
  - `v0.7.0-rc.1` — demo's first release; promotes the `alpha.4` commit verbatim, no changes. First
    cut on the `*-rc.N` → `demo` routing.
  - `v0.7.0-alpha.5` — demo shared-account auth + Erasure block + nightly-reset endpoint (#217,
    ADR-0041). No migration.
  - `v0.7.0-rc.2` — promotes the `alpha.5` commit verbatim, no changes. Demo standup complete (#217
    closed) — see below.
- **M8** (active) — next domain features, prioritized by real-user feedback from M7:
  - `v0.8.0-alpha.1` — descriptor-driven Position list + `PositionFormDialog` migration for all
    position types (#330–#334), generated FE wire-types from Go (#383), security response headers
    (#384), reliability tail (session housekeeping/limiter eviction/DB pool bounds/HTTP
    timeouts/gzip-bomb cap, #378/#379/#381), post-deploy version verification (#380), CONTRIBUTING +
    SECURITY.md (#387/#388), AGPL-3.0 license (ADR-0042). No migration.
  - `v0.8.0-alpha.2` — PDF export shipped both client-side (#187/#393, ADR-0044) and server-side
    native-Go (#413, ADR-0045 supersedes 0044's render location); demo enhancements (multi-year
    history, ledger detail, multi-currency, sign-in notice, #396/#415); indigo theme (#416); security
    tail (session tokens hashed at rest #361/#398, deploy-aware rate-limiter IP #363/#399, local-invite
    race hardening #340/#400, healthz leak + second CSRF layer + image scan #364/#402). Migration:
    additive backfill (`00011`, in-place session-id hash, non-destructive).
  - `v0.8.0-rc.1` — demo promotion under the old `rc→demo` routing; no new features over alpha.2
    (routing later decoupled from maturity, ADR-0049).
  - `v0.9.0-alpha.1` — financial statistics panel (#412, ADR-0048: four household-health ratios +
    Pension/Interest income categories + monthly inflation input); bulk monthly-entry mode (#372,
    ADR-0046); deploy target decoupled from release maturity (#489/#491, ADR-0049 — pre-releases →
    `preview`, `demo` manual-promoted); Settings reorg (FX/inflation subpages, Tags → report, #488);
    PDF version footer (#414); month-picker prev/next arrows (#487). Migration: additive (`00012`).
  - `v0.9.0-alpha.2` — PDF monthly report reorganised into five page groups with the net-worth trend
    as the headline (#495); earned-income drill-down under Cash Flow — Active/Passive by-source split
    + coupons on their own line (#496, ADR-0048 PR2, INV-FINANCE-26); investment-performance block —
    return-as-rate (this-month vs trailing-12) three ways + net placement line, leading the investments
    page (#499, ADR-0048 amendment, INV-FINANCE-29/30/31/32); dashboard net-worth chart decomposed into
    assets/liabilities/investments lines (#500); paid-out bond coupons counted as passive cash in the
    stats (#476/#493, INV-FINANCE-25); demo data reconciled as one household cash flow (#497/#498).
    Migration: additive (`00013` + `00014`, engine_version → 4).
  - `v0.9.0-alpha.3` — mobile–web layout divergence landed (epic #428, ADR-0050: per-breakpoint list
    and history layouts, card idioms, tap floor moved into the `Button`/`Input`/`Select` primitives,
    dialog form bodies at phone width, `OwnershipField`); descriptor-driven detail-page consolidation
    across all ten position types (epic #503, ADR-0051, incl. two-row headers + promoted primary
    action, #542); Settings regrouped into one card per section with two semantic control-width
    tokens (#562/#563, `common/SettingsSurface.tsx`); bundled Geist webfont actually paints (#565,
    INV-PRESENTATION-09). No migration.

## What's next

**M8 = next domain features (active line), first cut 2026-07-06 (`v0.8.0-alpha.1`); latest
`v0.9.0-alpha.3` (2026-07-29).** Prioritized by real-user feedback from M7 (not pre-specified).
PDF export (#187/#413, ADR-0044/0045) shipped in alpha.2; financial statistics panel shipped in
`v0.9.0-alpha.1` (#412 closed, ADR-0048); the ADR-0048 PDF/stats amendment tail shipped in
`v0.9.0-alpha.2`; the mobile line (ADR-0050/0051 epics #428/#503 + Settings rework) closed out in
`v0.9.0-alpha.3`. Unreleased on `main`: the mobile line's first **real-device** pass (#572,
INV-PRESENTATION-08) — date/month fields and `<select>` no longer keep iOS Safari's native control
metrics (`appearance-none` in both primitives; reverses #541's "keep the native arrow", so selects
draw `--select-chevron`), pagination renders a constant-width window (`lib/pageWindow.ts`) at the tap
floor instead of one link per page running off the screen, the income category select gets its own
full-width row, and the card ⋮ menus **open at all** on iOS — Radix's menu trigger is
`pointerdown`-only, which mobile WebKit wasn't delivering, so `useMenuOpenOnClick` adds a
self-cancelling click path (+ `modal: false`), and the floor finally reaches the menu *items*.
Standing warning behind all of it: Chrome DevTools device mode emulates the viewport, not the engine
— five defects, none visible in it.
Also unreleased on `main`: **#575** — deleting a Position now cascades the tombstone to its children
(snapshots for all four groups, plus transactions for Investment) inside one transaction
(INV-SOFT-DELETE-05). `SoftDelete<Group>` was a bare parent-row UPDATE, so live children hung off
tombstoned parents — invisible only because every named read path re-joins the parent, while five
`…ByIDs` child queries carry no parent join at all. Child visibility is now a write-path guarantee;
the read filters stay as defence in depth. **No migration** — the orphan backfill was built, verified
against a reproduced orphan, then dropped deliberately: it is pure cleanup of already-invisible rows
and there is no live production data to clean (dev is orphan-free; preview/demo may still carry a
few, dormant). If a repair is ever wanted, it must inherit each child's `deleted_at` from its parent
and leave `updated_at` alone — that column feeds `MaxReportInputUpdatedAt`, whose snapshot subqueries
do not filter `deleted_at`, so bumping it would mark every report in every household stale to fix
rows that change no number. Lifecycle/integrity shipped as **#576** (terminating a position with
no offsetting cash flow); the active line is now its sibling, **#591** (a position being *born* with
no offsetting flow).
Also unreleased on `main`: **ADR-0052** — #576's ADR, docs only, no code yet. Settles what a terminal
status means across all four groups. The 0-value close snapshot generalises from Investment-only to
every group (fixes the cash-settled timing split, where the cash leg lands in `M` and the position
drop in `M+1`); a new signed **Write-Off** term enters the identity for the non-cash terminals
(`disposed` / `forgiven` / `written_off`), which had no income-statement counterpart at all and were
landing whole in the Living-Expenses plug. Two findings worth keeping: Investment needs **no**
write-off status — a total loss is a truthful negative Investment Return (`sold` + 0-proceeds Sell) —
and a 0 close snapshot on a *cash-settled* property sale would have let `asset_value_change` eat it as
depreciation, so the termination month is now excluded from that loop entirely. Close snapshots
displace by soft-delete + insert (the partial unique index already allows both rows), making
un-terminate restore the user's own value and bulk correction reversible; Investment gets retrofitted.
**No backfill migration** — same call as #575, existing data corrected by hand on the two live
households and recorded in the release notes. Sliced as three sub-issues under #576.
Also unreleased on `main`: **#585** — ADR-0052 slice 1, the timing half. Asset, Liability and
Receivable now write the 0-value close snapshot their terminal flip always implied, inside the same
transaction as the lifecycle UPDATE, and all four groups share one codepath. Displacement is
soft-delete + insert: the archived row is paired to the close row by transaction timestamp
(`archived.deleted_at == close.created_at`), which is what lets un-terminate restore the user's own
value instead of leaving the month empty — and a re-asserted flip refreshes the close row in place so
that pairing survives. Re-terminating after the user recorded a fresh value at the termination month
leaves theirs alone. Cash-settled terminals (`closed`/`sold`/`paid_off`/`collected`/`matured`) now
reconcile: both legs land in `M` and the residual is untouched. The non-cash terminals still land in
the plug — moved from `M+1` to `M`, same magnitude, no new error — until #586 adds the Write-Off term.
No migration, no engine-version bump.
Also unreleased on `main`: **#586** — ADR-0052 slice 2, the missing-term half. `write_offs` joins the
comprehensive-income identity, fired off the terminal **status** (`disposed` / `forgiven` /
`written_off`; Investment never), signed by the effect on net worth so a forgiven debt reads positive,
and computed through the same carry-forward + FX calls the net-worth pass uses so it cancels ΔNW
structurally. `asset_value_change` now excludes the termination month for *every* terminated position
— without that, the 0-value close snapshot #585 introduced would have let a cash-settled property sale
read as depreciation and understate the residual by the whole sale value. `unsettled_terminations`
advises on Investments terminated with no `sell`/`maturity` recorded (the restore-from-backup path);
the transaction *amount* is never inspected, so a deliberate 0-proceeds Sell settles it rather than
tripping it forever. Surfaced on the PDF (own section, constituents beneath the line) and the
dashboard statement. Migration `00015` (additive: `write_offs`, `write_off_positions`,
`unsettled_terminations`), `reportEngineVersion` 4 → 5 so every materialized report regenerates —
historical months change for any household holding a terminated Position, which is the point. New
invariants INV-FINANCE-33/34/35; -05/-06/-10 reconciled. **Still owed at release:** the by-hand 0-value
close snapshots for already-terminated A/L/R positions on the two live households, plus the
release-notes line saying historical months were regenerated (ADR-0052 §8).

Also unreleased on `main`: **#587** — ADR-0052 slice 3, closing #576. Terminating an Investment now
captures its settling `sell`/`maturity` in the **same** database transaction as the status flip, so
neither half can survive alone. The dialog's settlement block is **subtype-shaped** (quantity × price
for a Sell, principal + interest for a Maturity) rather than one "proceeds" scalar — a Sell is
quantity-denominated and a Maturity is a pair, so a single number could only be split by fabricating
it — and the Investment status dropdown narrows to the pairs a transaction can express, which is what
makes the capture total. The write-off escape sends a **0-priced** Sell, not nothing: the quantity
still leaves the position, so the cost basis closes out and #586's `unsettled_terminations` advisory is
settled rather than tripped forever. Captured only on the active → terminal edge; re-asserting a
terminal status books no second sale. The matrix is also enforced at the API — a *transition into* an
unsupported pair (matured Stock, sold TimeDeposit) is refused, while a position that arrived on one
via restore stays editable, so it is never stranded. No migration, no engine-version bump. ADR-0052 §6
amended to record the shape, the API rule, and the default policy (a never-marked position that holds
something leaves the price blank and required — defaulting it to 0 would book a real sale as a total
loss; only a position holding nothing defaults to 0 so it stays closeable). Also recorded there:
dropping the terminate action for Investment was considered and rejected — a Sell is not inherently
terminal, and the dialog is the only surface for un-terminate. New invariant INV-LIFECYCLE-08.

The ADR-0052 §8 manual correction is **done on the dev DB** (7 positions: 5 Assets, 2 Liabilities;
the demo household had none, and no Investment sat on an unsettleable status). Archived row and
0-value close row share one transaction timestamp, so every one stays restorable. Still owed at
release: the same pass on the demo household if it ever gains terminated A/L/R, plus the release-notes
line saying historical months were regenerated.

Also unreleased on `main`: **ADR-0053** — #591's ADR (slice #593), docs only, no code yet. Adds a third
correction term, **Tracking Change**: value that crossed the *edge of the books* without being earned,
spent or invested — a spouse's accounts joining at marriage, a dormant passbook finally entered, a
departing member's positions leaving. One signed term covering **both** directions, per ADR-0052 §4's
reasoning. Two claims in #591 as filed are falsified in the ADR and matter more than the fix:
`INV-FINANCE-23` suppresses the investment **return line only**, never the NW contribution — what
protects a normal acquisition is that it is *funded from tracked wealth*, so the other leg moves — which
means **all four groups** are affected on the way in, and a blanket birth-month suppression would fix
onboarding while breaking **every acquisition** by its full value (both are the same `!okPrev` branch).
So it is **declared, never inferred**: `entry_type` (`acquired` default / `newly_tracked`) on all four
position tables, plus `untracked`, the one terminal status available to **every** group including
Investment — **amending ADR-0052 §5**, since a departing portfolio is not a total loss, and exempt from
§6's settlement capture. Fires at the **first snapshot month** in / termination month out (forced — that
is where the position enters `nwTotal`), so the entry side needs a new `now − 0` shape, not the
`!okPrev`-bailing loops. No `entry_date` boundary: the household already chooses what history to enter.
Feeds no ADR-0048 statistic, never Earned Income, no per-owner breakdown, and **no advisory** — a
one-sided birth is not a fact the engine can see, so the editable control is the only remedy. Epic
#591 → #593 (ADR) → #594 (DDL + both engine terms + entry-side UI, one migration, engine_version 5→6)
→ #595 (terminate dialog) ‖ #596 (PDF + waterfall). ADR stays mutable on the epic branch (ADR-0051
precedent). Existing data corrected by hand on the two live households, no backfill migration.

Next, in order:

1. **Production Resend domain** — the one M7 bullet that didn't literally close — moves with prod's
   eventual standup, tracked via #218 (Neon isolation) and #299's remaining GDPR scope, not its own
   milestone. Prod itself stays deferred indefinitely.

**Demo/prod launch prep (prod deferred indefinitely as of 2026-07-02; demo is the active line):** #215
subdomain scheme — **decided: nested product subtree** (`app.balances.<domain>` prod unmarked,
`balances.<domain>` landing, `preview.`/`demo.` siblings), **DNS-only never proxied**; preview and
demo both migrated. #216 single Resend sending domain — **DONE & closed**. #218 rescoped
2026-07-02 — prod's Neon-isolation + PITR-retention decision (incl. the erasure-purge window) parks
with prod; demo instead follows ADR-0030's already-decided single-project-per-env-branch shape (no
isolation): Neon `demo` branch, Fly app `balances-demo`, GitHub Environment `demo` all provisioned.
De-milestoned from M7 2026-07-03 — parked-with-prod items don't carry a milestone (matches #279's
earlier treatment).
#217 demo readiness — **DONE & closed** (2026-07-03): OAuth consolidated under one new GCP project,
consent screen published to Production, shared-account auth + Erasure block + nightly-reset endpoint
shipped (`v0.7.0-alpha.5`/`rc.2`, ADR-0041), GitHub Actions cron wired and verified live against demo.
DNS (`demo.balances.<domain>`) set. Demo standup complete.

**Production SaaS data-protection decision (2026-07-02):** #222 (originally: maintainer structurally
unable to read any user data — zero-knowledge encryption) closed as disproportionate; conflicts with
core server-side aggregation (monthly reports) and isn't what GDPR requires. Decided: ordinary GDPR
compliance is sufficient — lawful basis, privacy policy naming subprocessors, honoring access/erasure
requests, bounded breach process. Rescoped into **#299** (privacy policy — open, moved to M8, not
blocked on prod) and **#300** (household erasure "DELETE ME" — shipped alpha.4, see above, ADR-0040).
Access/portability already satisfied by the backup/export epic (#52). Self-host (#116) remains the
zero-exposure option for anyone unwilling to accept hosted SaaS. **Prod itself stays deferred
indefinitely.** (Correction 2026-07-03: the "non-disposable environment" M7 gate item is in fact
satisfied by self-host — #116 shipped and closed — not blocked on prod; see ROADMAP M7 status. Demo
remains the closest thing to a public-facing env for real-usage feedback, independent of that gate.)

Smaller open items ride a convenient batch, not their own cut.
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

**Deploying (ADR-0049 — target decoupled from maturity):** push any **pre-release** tag
(`*-alpha.N`/`*-rc.N`/`*-beta.N`) → auto-deploy to `preview` (private, maintainer-only, signups
closed). `demo` (public, disposable) is **promoted manually** — a `workflow_dispatch` deploys a chosen
tag's GHCR image to `balances-demo` with no rebuild (build-once/promote-many, ADR-0033). Maturity
shapes only the release notes now, not the target; `rc` again means only "feature-frozen candidate."
Bare `vX.Y.Z`→production route stays dormant (prod deferred indefinitely). `deploy.yml` runs `flyctl
deploy --image ghcr.io/kerti/balances:<tag>` (`goose up` via `release_command`). Backend runtime
secrets live on Fly (`fly secrets`); only `FLY_API_TOKEN` is in each env's GitHub Environment
(`preview`, `demo`). *(deploy.yml `route`-job rewrite is a pending slice — #489.)*

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
