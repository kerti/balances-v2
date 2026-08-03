-- All snapshot queries verify the parent liability belongs to the requesting
-- Household. This is belt + suspenders on top of the application-layer
-- tenancy middleware: even if a handler forgets to filter, SQL will not
-- expose or mutate snapshots from another Household.

-- name: CreateLiabilitySnapshot :one
WITH owned_liability AS (
    SELECT l.id AS lid
    FROM liabilities l
    WHERE l.id = $1 AND l.household_id = sqlc.arg('household_id')::uuid AND l.deleted_at IS NULL
)
INSERT INTO liability_snapshots (
    liability_id, year_month, amount, currency, as_of_date, description,
    created_by, updated_by
)
SELECT owned_liability.lid, $2, $3, $4, $5, $6, $7, $7
FROM owned_liability
RETURNING *;

-- name: ListLiabilitySnapshotsForLiability :many
SELECT s.*
FROM liability_snapshots s
JOIN liabilities l ON l.id = s.liability_id
WHERE s.liability_id = $1
  AND l.household_id = $2
  AND l.deleted_at IS NULL
  AND s.deleted_at IS NULL
ORDER BY s.year_month DESC;

-- name: GetLiabilitySnapshotByID :one
SELECT s.*
FROM liability_snapshots s
JOIN liabilities l ON l.id = s.liability_id
WHERE s.id = $1
  AND l.household_id = $2
  AND l.deleted_at IS NULL
  AND s.deleted_at IS NULL;

-- name: UpdateLiabilitySnapshot :one
UPDATE liability_snapshots s
SET amount      = $3,
    currency    = $4,
    as_of_date  = $5,
    description = $6,
    updated_by  = $7,
    updated_at  = now()
FROM liabilities l
WHERE s.id = $1
  AND s.liability_id = l.id
  AND l.household_id = $2
  AND l.deleted_at IS NULL
  AND s.deleted_at IS NULL
RETURNING s.*;

-- Batch fetch of the most-recent snapshot per liability, for list views.
-- name: ListLatestLiabilitySnapshotsByLiabilityIDs :many
SELECT DISTINCT ON (liability_id) *
FROM liability_snapshots
WHERE liability_id = ANY($1::uuid[]) AND deleted_at IS NULL
ORDER BY liability_id, year_month DESC;

-- Full ascending value series per liability, for the Liabilities Home time
-- graphs (epic #204). Ascending order is what LiabilityTimeSeries' carry-
-- forward sampling relies on; mirrors ListAssetSnapshotsByAssetIDs.
-- name: ListLiabilitySnapshotsByLiabilityIDs :many
SELECT *
FROM liability_snapshots
WHERE liability_id = ANY($1::uuid[]) AND deleted_at IS NULL
ORDER BY liability_id, year_month;

-- name: SoftDeleteLiabilitySnapshot :execrows
UPDATE liability_snapshots s
SET deleted_at = now(),
    updated_by = $3,
    updated_at = now()
FROM liabilities l
WHERE s.id = $1
  AND s.liability_id = l.id
  AND l.household_id = $2
  AND l.deleted_at IS NULL
  AND s.deleted_at IS NULL;

-- CascadeSoftDeleteLiabilitySnapshots tombstones every live snapshot of one
-- liability — see CascadeSoftDeleteAssetSnapshots for the ordering contract
-- and why the already-deleted child is left alone (INV-SOFT-DELETE-05).
-- name: CascadeSoftDeleteLiabilitySnapshots :execrows
UPDATE liability_snapshots s
SET deleted_at = now(),
    updated_by = $3,
    updated_at = now()
FROM liabilities l
WHERE s.liability_id = $1
  AND s.liability_id = l.id
  AND l.household_id = $2
  AND l.deleted_at IS NULL
  AND s.deleted_at IS NULL;

-- GetLiabilityForImport returns the display name + native currency of an owned
-- liability. Doubles as the ownership/existence check for the snapshot
-- importer: ErrNoRows means the liability doesn't exist in this household (or
-- is deleted), which the repo maps to ErrNotFound -> 404.
-- name: GetLiabilityForImport :one
SELECT l.display_name, l.native_currency
FROM liabilities l
WHERE l.id = $1 AND l.household_id = $2 AND l.deleted_at IS NULL;

-- UpsertLiabilitySnapshot inserts a snapshot or, when one already exists for
-- the (liability_id, year_month) pair, overwrites it (last-write-wins) — the
-- importer needs idempotent re-runs of a multi-year backfill. ON CONFLICT
-- targets the partial unique index, so its predicate (deleted_at IS NULL) is
-- repeated. created_by is only set on insert; updated_by always.
-- name: UpsertLiabilitySnapshot :one
WITH owned_liability AS (
    SELECT l.id AS lid
    FROM liabilities l
    WHERE l.id = $1 AND l.household_id = sqlc.arg('household_id')::uuid AND l.deleted_at IS NULL
)
INSERT INTO liability_snapshots (
    liability_id, year_month, amount, currency, as_of_date, description,
    created_by, updated_by
)
SELECT owned_liability.lid, $2, $3, $4, $5, $6, $7, $7
FROM owned_liability
ON CONFLICT (liability_id, year_month) WHERE deleted_at IS NULL
DO UPDATE SET
    amount      = EXCLUDED.amount,
    currency    = EXCLUDED.currency,
    as_of_date  = EXCLUDED.as_of_date,
    description = EXCLUDED.description,
    updated_by  = EXCLUDED.updated_by,
    updated_at  = now()
RETURNING *;

-- ListEligibleLiabilityIDsForMonth returns the ids of the household's
-- liabilities that may legitimately hold a snapshot for the given target month
-- (ADR-0046 month-aware eligibility): owned, not deleted, and either still
-- active or terminated in the target month or later. A liability terminated
-- *before* the target month never appears — its forward contribution is already
-- frozen by the carry-forward rule. No created_at guard, for the same reason as
-- the Asset entry list: created_at is a record timestamp, not economic
-- existence, and gating on it would block legitimate backfill/onboarding.
-- name: ListEligibleLiabilityIDsForMonth :many
SELECT id
FROM liabilities
WHERE household_id = sqlc.arg('household_id')::uuid
  AND deleted_at IS NULL
  AND (terminated_at IS NULL OR terminated_at >= sqlc.arg('year_month')::date);

-- ListEligibleLiabilitiesForMonth is ListEligibleLiabilityIDsForMonth plus the
-- display fields the bulk monthly-entry list needs (ADR-0046): name, native
-- currency, and subtype so the entry view can group by type (personal /
-- institutional). Ordered by subtype then display name so rows arrive
-- pre-grouped.
-- name: ListEligibleLiabilitiesForMonth :many
SELECT id, display_name, native_currency, subtype, ownership_type, sole_owner_user_id
FROM liabilities
WHERE household_id = sqlc.arg('household_id')::uuid
  AND deleted_at IS NULL
  AND (terminated_at IS NULL OR terminated_at >= sqlc.arg('year_month')::date)
ORDER BY subtype, display_name;

-- ListLatestSnapshotsByLiabilityIDsAsOfMonth returns, per liability, the
-- most-recent snapshot at or before the target month — the carry-forward
-- prefill for the entry list. Month-bounded so a value entered ahead of the
-- target does not leak backwards as the prefill.
-- name: ListLatestSnapshotsByLiabilityIDsAsOfMonth :many
SELECT DISTINCT ON (liability_id) liability_id, amount, year_month
FROM liability_snapshots
WHERE liability_id = ANY(sqlc.arg('liability_ids')::uuid[])
  AND deleted_at IS NULL
  AND year_month <= sqlc.arg('year_month')::date
ORDER BY liability_id, year_month DESC;

-- Close-snapshot displacement (ADR-0052 §2) — see asset_snapshots.sql for the
-- mechanism and the reasoning behind each guard.

-- name: GetLiabilitySnapshotAtMonth :one
SELECT s.*
FROM liability_snapshots s
JOIN liabilities l ON l.id = s.liability_id
WHERE s.liability_id = sqlc.arg('liability_id')
  AND s.year_month = sqlc.arg('year_month')::date
  AND l.household_id = sqlc.arg('household_id')::uuid
  AND l.deleted_at IS NULL
  AND s.deleted_at IS NULL;

-- name: GetArchivedLiabilitySnapshotAtMonth :one
SELECT s.*
FROM liability_snapshots s
JOIN liabilities l ON l.id = s.liability_id
WHERE s.liability_id = sqlc.arg('liability_id')
  AND s.year_month = sqlc.arg('year_month')::date
  AND l.household_id = sqlc.arg('household_id')::uuid
  AND l.deleted_at IS NULL
  AND s.deleted_at = sqlc.arg('archived_at')::timestamptz
  AND s.amount <> 0
ORDER BY s.created_at DESC
LIMIT 1;

-- name: RestoreLiabilitySnapshot :execrows
UPDATE liability_snapshots s
SET deleted_at = NULL,
    updated_by = sqlc.arg('updated_by'),
    updated_at = now()
FROM liabilities l
WHERE s.id = sqlc.arg('id')
  AND s.liability_id = l.id
  AND l.household_id = sqlc.arg('household_id')::uuid
  AND l.deleted_at IS NULL
  AND s.deleted_at IS NOT NULL;
