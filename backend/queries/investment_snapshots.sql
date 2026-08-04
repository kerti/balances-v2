-- All snapshot queries verify the parent investment belongs to the requesting
-- Household. This is belt + suspenders on top of the application-layer
-- tenancy middleware: even if a handler forgets to filter, SQL will not
-- expose or mutate snapshots from another Household. The XOR shape
-- (quantity+price vs accrued_interest) is enforced at the column level by
-- the table's CHECK constraint and at the subtype level by the repository
-- (per ADR-0022).

-- name: CreateInvestmentSnapshot :one
WITH owned_investment AS (
    SELECT i.id AS iid
    FROM investments i
    WHERE i.id = $1 AND i.household_id = sqlc.arg('household_id')::uuid AND i.deleted_at IS NULL
)
INSERT INTO investment_snapshots (
    investment_id, year_month, amount, currency,
    quantity, price_per_unit, accrued_interest,
    as_of_date, description,
    created_by, updated_by, supersedes
)
SELECT owned_investment.iid, $2, $3, $4, $5, $6, $7, $8, $9, $10, $10, sqlc.narg('supersedes')
FROM owned_investment
RETURNING *;

-- name: ListInvestmentSnapshotsForInvestment :many
SELECT s.*
FROM investment_snapshots s
JOIN investments i ON i.id = s.investment_id
WHERE s.investment_id = $1
  AND i.household_id = $2
  AND i.deleted_at IS NULL
  AND s.deleted_at IS NULL
ORDER BY s.year_month DESC;

-- name: GetInvestmentSnapshotByID :one
SELECT s.*
FROM investment_snapshots s
JOIN investments i ON i.id = s.investment_id
WHERE s.id = $1
  AND i.household_id = $2
  AND i.deleted_at IS NULL
  AND s.deleted_at IS NULL;

-- name: UpdateInvestmentSnapshot :one
UPDATE investment_snapshots s
SET amount           = $3,
    currency         = $4,
    quantity         = $5,
    price_per_unit   = $6,
    accrued_interest = $7,
    as_of_date       = $8,
    description      = $9,
    updated_by       = $10,
    updated_at       = now()
FROM investments i
WHERE s.id = $1
  AND s.investment_id = i.id
  AND i.household_id = $2
  AND i.deleted_at IS NULL
  AND s.deleted_at IS NULL
RETURNING s.*;

-- Batch fetch of the most-recent snapshot per investment, for list views.
-- name: ListLatestInvestmentSnapshotsByInvestmentIDs :many
SELECT DISTINCT ON (investment_id) *
FROM investment_snapshots
WHERE investment_id = ANY($1::uuid[]) AND deleted_at IS NULL
ORDER BY investment_id, year_month DESC;

-- Batch fetch of every (non-deleted) snapshot across many investments, for the
-- list/home time-graph value + cost series (issue #22). Household-scoped IDs
-- supplied by the caller (mirrors ListLatestInvestmentSnapshotsByInvestmentIDs).
-- Ascending by year_month so the repo can build each series in order.
-- name: ListInvestmentSnapshotsByInvestmentIDs :many
SELECT *
FROM investment_snapshots
WHERE investment_id = ANY($1::uuid[]) AND deleted_at IS NULL
ORDER BY investment_id, year_month;

-- name: SoftDeleteInvestmentSnapshot :execrows
UPDATE investment_snapshots s
SET deleted_at = now(),
    updated_by = $3,
    updated_at = now()
FROM investments i
WHERE s.id = $1
  AND s.investment_id = i.id
  AND i.household_id = $2
  AND i.deleted_at IS NULL
  AND s.deleted_at IS NULL;

-- CascadeSoftDeleteInvestmentSnapshots tombstones every live snapshot of one
-- investment — see CascadeSoftDeleteAssetSnapshots for the ordering contract
-- and why the already-deleted child is left alone (INV-SOFT-DELETE-05).
-- Investment is the one group with a second child table; the cascade is only
-- complete alongside CascadeSoftDeleteInvestmentTransactions.
-- name: CascadeSoftDeleteInvestmentSnapshots :execrows
UPDATE investment_snapshots s
SET deleted_at = now(),
    updated_by = $3,
    updated_at = now()
FROM investments i
WHERE s.investment_id = $1
  AND s.investment_id = i.id
  AND i.household_id = $2
  AND i.deleted_at IS NULL
  AND s.deleted_at IS NULL;

-- UpsertInvestmentSnapshot inserts a snapshot or, when one already exists for
-- the (investment_id, year_month) pair, overwrites it (last-write-wins) — the
-- importer needs idempotent re-runs of a multi-year backfill. ON CONFLICT
-- targets the partial unique index, so its predicate (deleted_at IS NULL) is
-- repeated. The repo validates the value-column shape against the parent's
-- subtype before calling this; the DB CHECK is the final backstop.
-- created_by is only set on insert; updated_by always.
-- name: UpsertInvestmentSnapshot :one
WITH owned_investment AS (
    SELECT i.id AS iid
    FROM investments i
    WHERE i.id = $1 AND i.household_id = sqlc.arg('household_id')::uuid AND i.deleted_at IS NULL
)
INSERT INTO investment_snapshots (
    investment_id, year_month, amount, currency,
    quantity, price_per_unit, accrued_interest,
    as_of_date, description,
    created_by, updated_by
)
SELECT owned_investment.iid, $2, $3, $4, $5, $6, $7, $8, $9, $10, $10
FROM owned_investment
ON CONFLICT (investment_id, year_month) WHERE deleted_at IS NULL
DO UPDATE SET
    amount           = EXCLUDED.amount,
    currency         = EXCLUDED.currency,
    quantity         = EXCLUDED.quantity,
    price_per_unit   = EXCLUDED.price_per_unit,
    accrued_interest = EXCLUDED.accrued_interest,
    as_of_date       = EXCLUDED.as_of_date,
    description      = EXCLUDED.description,
    updated_by       = EXCLUDED.updated_by,
    updated_at       = now()
RETURNING *;

-- ----- bulk monthly-entry, qty×price shape (ADR-0046, #423) ------------------
--
-- Stock/MutualFund/Gold only: the three subtypes whose snapshots take the
-- quantity+price_per_unit branch of investment_snapshot_shape. Bond/TimeDeposit
-- (the accrued branch) are a separate slice (#424) with their own queries.
-- Eligibility mirrors the asset rule: owned, not deleted, and either still
-- active or terminated in the target month or later. created_at is deliberately
-- not gated (a record timestamp, not economic existence — see the asset notes).

-- name: ListEligibleQtyPriceInvestmentIDsForMonth :many
SELECT id
FROM investments
WHERE household_id = sqlc.arg('household_id')::uuid
  AND deleted_at IS NULL
  AND subtype IN ('stock', 'mutual_fund', 'gold')
  AND (terminated_at IS NULL OR terminated_at >= sqlc.arg('year_month')::date);

-- ListEligibleQtyPriceInvestmentsForMonth is the ID query plus the display
-- fields the entry list needs (ADR-0046): name, native currency, and subtype so
-- the entry view groups by type. Ordered by subtype then name so rows arrive
-- pre-grouped; the entry view re-orders subtypes to its own preference.
-- name: ListEligibleQtyPriceInvestmentsForMonth :many
SELECT id, display_name, native_currency, subtype, ownership_type, sole_owner_user_id
FROM investments
WHERE household_id = sqlc.arg('household_id')::uuid
  AND deleted_at IS NULL
  AND subtype IN ('stock', 'mutual_fund', 'gold')
  AND (terminated_at IS NULL OR terminated_at >= sqlc.arg('year_month')::date)
ORDER BY subtype, display_name;

-- ListLatestQtyPriceSnapshotsByInvestmentIDsAsOfMonth returns, per investment,
-- the most-recent snapshot at or before the target month — the carry-forward
-- prefill for the entry list. Carries quantity + price_per_unit (the two
-- tab-stops), month-bounded so a value entered ahead of the target does not leak
-- backwards as the prefill.
-- name: ListLatestQtyPriceSnapshotsByInvestmentIDsAsOfMonth :many
SELECT DISTINCT ON (investment_id) investment_id, quantity, price_per_unit, year_month
FROM investment_snapshots
WHERE investment_id = ANY(sqlc.arg('investment_ids')::uuid[])
  AND deleted_at IS NULL
  AND year_month <= sqlc.arg('year_month')::date
ORDER BY investment_id, year_month DESC;

-- ----- bulk monthly-entry, accrued shape (ADR-0046, #424) --------------------
--
-- Bond/TimeDeposit only: the two subtypes whose snapshots take the
-- accrued_interest branch of investment_snapshot_shape (accrued_interest set,
-- quantity/price null). Stock/MutualFund/Gold (the qty×price branch) are the
-- separate #423 slice above. A row carries the total value (amount) and the
-- accrued-interest component — the same two figures the per-position accrued
-- dialog takes — so, unlike qty×price, `amount` is entered directly (a bond's
-- total value already *is* its snapshot amount), not derived. Eligibility
-- mirrors the qty×price/asset rule (owned, not deleted, active or terminated in
-- the target month or later); created_at is deliberately not gated (a record
-- timestamp, not economic existence). A time deposit's snapshots are further
-- confined to its term window (placement month..maturity month, issue #62) — the
-- same repo-layer bound the per-position form applies — so an out-of-term month
-- excludes it from the list and rejects a hand-crafted write row. The
-- coupon_disposition of a bond drives only the entry list's per-row accrued
-- default (empty vs 0), so it is joined in for the list; time deposits have no
-- bond_details row and default to 0.

-- name: ListEligibleAccruedInvestmentIDsForMonth :many
SELECT i.id
FROM investments i
LEFT JOIN time_deposit_details td ON td.investment_id = i.id
WHERE i.household_id = sqlc.arg('household_id')::uuid
  AND i.deleted_at IS NULL
  AND i.subtype IN ('bond', 'time_deposit')
  AND (i.terminated_at IS NULL OR i.terminated_at >= sqlc.arg('year_month')::date)
  AND (
    i.subtype <> 'time_deposit'
    OR (date_trunc('month', td.placement_date)::date <= sqlc.arg('year_month')::date
        AND date_trunc('month', td.maturity_date)::date >= sqlc.arg('year_month')::date)
  );

-- ListEligibleAccruedInvestmentsForMonth is the ID query plus the display fields
-- the entry list needs (ADR-0046), and each bond's coupon_disposition (via a
-- LEFT JOIN on bond_details) so the entry view can seed the accrued default the
-- per-position form uses (accrues → forced entry, pays_out/time-deposit → 0).
-- A time deposit has no bond_details row, so coupon_disposition is NULL — the
-- repo treats NULL as pays_out. Time deposits are also confined to their term
-- window (same predicate as the ID query). Ordered by subtype then name so rows
-- arrive pre-grouped.
-- name: ListEligibleAccruedInvestmentsForMonth :many
SELECT i.id, i.display_name, i.native_currency, i.subtype, i.ownership_type,
       i.sole_owner_user_id, bd.coupon_disposition
FROM investments i
LEFT JOIN bond_details bd ON bd.investment_id = i.id
LEFT JOIN time_deposit_details td ON td.investment_id = i.id
WHERE i.household_id = sqlc.arg('household_id')::uuid
  AND i.deleted_at IS NULL
  AND i.subtype IN ('bond', 'time_deposit')
  AND (i.terminated_at IS NULL OR i.terminated_at >= sqlc.arg('year_month')::date)
  AND (
    i.subtype <> 'time_deposit'
    OR (date_trunc('month', td.placement_date)::date <= sqlc.arg('year_month')::date
        AND date_trunc('month', td.maturity_date)::date >= sqlc.arg('year_month')::date)
  )
ORDER BY i.subtype, i.display_name;

-- ListLatestAccruedSnapshotsByInvestmentIDsAsOfMonth returns, per investment,
-- the most-recent snapshot at or before the target month — the carry-forward
-- prefill for the accrued entry list. Carries amount (the total value tab-stop)
-- and accrued_interest (the second tab-stop), month-bounded so a value entered
-- ahead of the target does not leak backwards as the prefill.
-- name: ListLatestAccruedSnapshotsByInvestmentIDsAsOfMonth :many
SELECT DISTINCT ON (investment_id) investment_id, amount, accrued_interest, year_month
FROM investment_snapshots
WHERE investment_id = ANY(sqlc.arg('investment_ids')::uuid[])
  AND deleted_at IS NULL
  AND year_month <= sqlc.arg('year_month')::date
ORDER BY investment_id, year_month DESC;

-- Close-snapshot displacement (ADR-0052 §2) — see asset_snapshots.sql for the
-- mechanism and the reasoning behind each guard. Investment is retrofitted onto
-- it: #25 wrote its close via UpsertInvestmentSnapshot, which overwrote the
-- user's termination-month row in place.

-- name: GetInvestmentSnapshotAtMonth :one
SELECT s.*
FROM investment_snapshots s
JOIN investments i ON i.id = s.investment_id
WHERE s.investment_id = sqlc.arg('investment_id')
  AND s.year_month = sqlc.arg('year_month')::date
  AND i.household_id = sqlc.arg('household_id')::uuid
  AND i.deleted_at IS NULL
  AND s.deleted_at IS NULL;

-- name: RestoreInvestmentSnapshot :execrows
UPDATE investment_snapshots s
SET deleted_at = NULL,
    updated_by = sqlc.arg('updated_by'),
    updated_at = now()
FROM investments i
WHERE s.id = sqlc.arg('id')
  AND s.investment_id = i.id
  AND i.household_id = sqlc.arg('household_id')::uuid
  AND i.deleted_at IS NULL
  AND s.deleted_at IS NOT NULL;
