-- All snapshot queries verify the parent receivable belongs to the requesting
-- Household. This is belt + suspenders on top of the application-layer
-- tenancy middleware: even if a handler forgets to filter, SQL will not
-- expose or mutate snapshots from another Household.

-- name: CreateReceivableSnapshot :one
WITH owned_receivable AS (
    SELECT r.id AS rid
    FROM receivables r
    WHERE r.id = $1 AND r.household_id = sqlc.arg('household_id')::uuid AND r.deleted_at IS NULL
)
INSERT INTO receivable_snapshots (
    receivable_id, year_month, amount, currency, as_of_date, description,
    created_by, updated_by
)
SELECT owned_receivable.rid, $2, $3, $4, $5, $6, $7, $7
FROM owned_receivable
RETURNING *;

-- name: ListReceivableSnapshotsForReceivable :many
SELECT s.*
FROM receivable_snapshots s
JOIN receivables r ON r.id = s.receivable_id
WHERE s.receivable_id = $1
  AND r.household_id = $2
  AND r.deleted_at IS NULL
  AND s.deleted_at IS NULL
ORDER BY s.year_month DESC;

-- name: GetReceivableSnapshotByID :one
SELECT s.*
FROM receivable_snapshots s
JOIN receivables r ON r.id = s.receivable_id
WHERE s.id = $1
  AND r.household_id = $2
  AND r.deleted_at IS NULL
  AND s.deleted_at IS NULL;

-- name: UpdateReceivableSnapshot :one
UPDATE receivable_snapshots s
SET amount      = $3,
    currency    = $4,
    as_of_date  = $5,
    description = $6,
    updated_by  = $7,
    updated_at  = now()
FROM receivables r
WHERE s.id = $1
  AND s.receivable_id = r.id
  AND r.household_id = $2
  AND r.deleted_at IS NULL
  AND s.deleted_at IS NULL
RETURNING s.*;

-- Batch fetch of the most-recent snapshot per receivable, for list views.
-- name: ListLatestReceivableSnapshotsByReceivableIDs :many
SELECT DISTINCT ON (receivable_id) *
FROM receivable_snapshots
WHERE receivable_id = ANY($1::uuid[]) AND deleted_at IS NULL
ORDER BY receivable_id, year_month DESC;

-- Full ascending value series per receivable, for the Receivables list
-- total-over-time chart (epic #204). Ascending order is what
-- ReceivableTimeSeries' carry-forward sampling relies on; mirrors
-- ListAssetSnapshotsByAssetIDs.
-- name: ListReceivableSnapshotsByReceivableIDs :many
SELECT *
FROM receivable_snapshots
WHERE receivable_id = ANY($1::uuid[]) AND deleted_at IS NULL
ORDER BY receivable_id, year_month;

-- name: SoftDeleteReceivableSnapshot :execrows
UPDATE receivable_snapshots s
SET deleted_at = now(),
    updated_by = $3,
    updated_at = now()
FROM receivables r
WHERE s.id = $1
  AND s.receivable_id = r.id
  AND r.household_id = $2
  AND r.deleted_at IS NULL
  AND s.deleted_at IS NULL;

-- GetReceivableForImport returns the display name + native currency of an
-- owned receivable. Doubles as the ownership/existence check for the snapshot
-- importer: ErrNoRows means the receivable doesn't exist in this household (or
-- is deleted), which the repo maps to ErrNotFound -> 404.
-- name: GetReceivableForImport :one
SELECT r.display_name, r.native_currency
FROM receivables r
WHERE r.id = $1 AND r.household_id = $2 AND r.deleted_at IS NULL;

-- UpsertReceivableSnapshot inserts a snapshot or, when one already exists for
-- the (receivable_id, year_month) pair, overwrites it (last-write-wins) — the
-- importer needs idempotent re-runs of a multi-year backfill. ON CONFLICT
-- targets the partial unique index, so its predicate (deleted_at IS NULL) is
-- repeated. created_by is only set on insert; updated_by always.
-- name: UpsertReceivableSnapshot :one
WITH owned_receivable AS (
    SELECT r.id AS rid
    FROM receivables r
    WHERE r.id = $1 AND r.household_id = sqlc.arg('household_id')::uuid AND r.deleted_at IS NULL
)
INSERT INTO receivable_snapshots (
    receivable_id, year_month, amount, currency, as_of_date, description,
    created_by, updated_by
)
SELECT owned_receivable.rid, $2, $3, $4, $5, $6, $7, $7
FROM owned_receivable
ON CONFLICT (receivable_id, year_month) WHERE deleted_at IS NULL
DO UPDATE SET
    amount      = EXCLUDED.amount,
    currency    = EXCLUDED.currency,
    as_of_date  = EXCLUDED.as_of_date,
    description = EXCLUDED.description,
    updated_by  = EXCLUDED.updated_by,
    updated_at  = now()
RETURNING *;
