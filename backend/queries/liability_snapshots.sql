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
