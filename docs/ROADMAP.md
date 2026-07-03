# Roadmap

The design phase is captured in `CONTEXT.md` and `docs/adr/0001–0021.md`. This document is the
implementation outline — eight milestones, each shippable, each a useful place to pause and resume.
M1–M5 done; M6 closes with the group-Home dashboard-polish epic (#204); PDF export (#187) was pivoted
to M8 on 2026-06-17 (polish the live dashboard before snapshotting it to a report). M7
(productization) and M8 (domain features, feedback-driven) were added 2026-06-17.

Reorder, split, or merge milestones as reality demands. This is a north-star, not a contract.

M1–M5 are shipped; their done-when checklists are satisfied history (detail in the closed issues +
Release notes). One-line goals kept below for orientation; the live forward outline is M6–M8.

## Milestone 1 — Walking skeleton — ✅ shipped

End-to-end stack runs locally: Vite frontend → Go backend → Postgres, `balances migrate up` clean,
`/healthz` proves the DB connection. Wiring only, no business logic.

## Milestone 2 — Auth end-to-end — ✅ shipped

Google OAuth login + server-side session (ADR-0017); email invites create `household_invitations`
and the invite link signs the invitee into the Household. The auth + Mailer + frontend roundtrip.

## Milestone 3 — First Position CRUD slice — ✅ shipped

Bank-account Asset with full CRUD + monthly Snapshots, tenancy-tested across Households (ADR-0005,
ADR-0021). Established the handler → repository → sqlc → migration pattern.

## Milestone 4 — All Positions + Snapshots + Income — ✅ shipped

Every Position subtype (asset/liability/receivable/investment) CRUD + Snapshots; Investment
Transactions (Buy/Sell/Coupon/Dividend/Distribution/Fee/Maturity); Income events (ADR-0008
categories); position lifecycle (ADR-0009). The data-entry surface is feature-complete.

## Milestone 5 — Materialized monthly report — ✅ shipped

`monthly_reports` via the lazy + staleness regeneration flow (ADR-0006); dashboard with net-worth
headline, group/User breakdowns, comprehensive-income lines, time-series chart; manual rebuild;
`stale_positions` warning; side-by-side multi-currency (Q15c). The app's reason to exist.

## Milestone 6 — v1 polish

**Goal:** ready to depend on.

**Done when:**
- Group-Home dashboard parity (#204) lands — Assets/Liabilities Homes reach InvestmentsHome parity
  and Receivables gets a total-over-time chart (the live dashboard now supersedes the Google-Sheets
  workbook this app replaced)
- Property/vehicle amortization-rate UI helper (Q8a) is in
- Fee cash→quantity helper (Q12 follow-up) is in
- TimeDeposit "duplicate matured TD" helper (Q14c-iv) is in
- Migrations consolidated — the accumulated pre-alpha migration files (currently ~7+, including
  amendments like the M4.3b-frontend `bond_details.series_code` add) are squashed into a single
  initial-schema migration before the first production deploy
- Hosting target is chosen and the app is deployed
- A real Resend domain is configured for production email
- Backup / restore for the production DB is documented

**Status (2026-06-17):** the Q8a property/vehicle revaluation helper (`lib/revaluation.ts`), the Q12
fee cash→quantity helper (`lib/feeQuantity.ts`), the Q14c matured-TD redeploy helper (`lib/rollover.ts`
+ `CreateTimeDepositDialog` prefill), the migration squash (ADR-0031 baseline), the hosting target
(Fly preview, ADR-0030), and whole-household backup/restore (epic #52, ADR-0036) are all shipped.
**PDF export (#187) was pivoted to M8** (re-scoped to a downloadable/emailable *polished* report,
blocked on #204): 3 of 4 group Homes were stubs, so a PDF now would snapshot a half-finished report —
polish-then-snapshot is the correct order. The **remaining M6 done-when item is the group-Home
dashboard-polish epic (#204)**; a production Resend domain is prod-gated and moves to M7; "production
DB backup/restore documented" is a short ops note still owed.

## Milestone 7 — Productization / beta — ✅ shipped

**Goal:** make Balances trustable by real households, not richer in domain features. The bet: a large,
safe surface has shipped with **zero external feedback** (preview-only, OAuth Testing mode, no prod) —
real usage, not more building, drives M8.

**Done when:**
- **Self-hosting (#116)** lands — the bus-factor answer and a `1.0.0` blocker (ADR-0033). Prioritized
  over any net-new feature.
- A non-disposable environment exists (hosted beta or self-host), distinct from `preview`.
- **Onboarding (#158)** is resolved — invite-vs-found-household at first sign-in is irreversible
  (one household, no leave/switch, ADR-0017); needs its own grill-with-docs + ADR before build.
- A production Resend domain is configured (carried from M6).
- Marketing landing + docs site (#93) ships, or is consciously deferred with a trigger.

Opens at `v0.7.0-alpha.1` — the minor bump marks the milestone boundary (ADR-0033; "milestone =
minor" convention).

**Status (2026-07-03):** self-hosting (#116), onboarding (#158), and the non-disposable-environment
bullet (self-host satisfies "hosted beta or self-host" — no separate hosted beta needed) are all done.
#93 landing/docs is consciously deferred to near-RC (trigger: app domain costs), satisfying its own
"or deferred" clause. The one bullet that doesn't literally close — **a production Resend domain** —
is gated on a production launch that is itself now **deferred indefinitely** (2026-07-02; ADR-0033
amendment unpinned first-prod from any version or date). Rather than hold M7 open on a decision that's
been shelved, that bullet carries forward with prod: it lands whenever prod actually unparks, tracked
via #218 (prod Neon isolation) and #299's remaining GDPR scope — not its own milestone. M7 closes here.

## Milestone 8 — Next domain features

**Status (2026-07-03):** now the active milestone — M7 closed above.

**Goal:** the next wave of domain capability, **prioritized by real-user feedback from M7**, not
pre-specified here. Candidates currently parked in the backlog (#66 per-bond coupon disposition, #145
expected-coupon projection, #69 component tests, etc.) compete on observed signal. Future ADRs
(passkeys, offline support, push notifications, Apple OAuth) remain incremental enhancements.

- **PDF export (#187)** — pivoted here from M6 (2026-06-17). Re-scoped to a downloadable/emailable
  *polished* monthly report, building on the now-parity group Homes (epic #204); deferred so the PDF
  snapshots a finished dashboard, not a half-built one.
- **Riding along, not domain features** — small M7 leftovers that don't gate anything, moved here so
  M7 could close clean: **#299** (privacy policy — preview-scoped note + breach runbook, not blocked
  on prod) and **#163** (email hosted-wordmark raster).
