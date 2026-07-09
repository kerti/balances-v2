-- All snapshot queries verify the parent asset belongs to the requesting
-- Household. This is belt + suspenders on top of the application-layer
-- tenancy middleware: even if a handler forgets to filter, SQL will not
-- expose or mutate snapshots from another Household.

-- name: CreateAssetSnapshot :one
WITH owned_asset AS (
    SELECT a.id AS aid
    FROM assets a
    WHERE a.id = $1 AND a.household_id = sqlc.arg('household_id')::uuid AND a.deleted_at IS NULL
)
INSERT INTO asset_snapshots (
    asset_id, year_month, amount, currency, as_of_date, description,
    created_by, updated_by
)
SELECT owned_asset.aid, $2, $3, $4, $5, $6, $7, $7
FROM owned_asset
RETURNING *;

-- name: ListAssetSnapshotsForAsset :many
SELECT s.*
FROM asset_snapshots s
JOIN assets a ON a.id = s.asset_id
WHERE s.asset_id = $1
  AND a.household_id = $2
  AND a.deleted_at IS NULL
  AND s.deleted_at IS NULL
ORDER BY s.year_month DESC;

-- name: GetAssetSnapshotByID :one
SELECT s.*
FROM asset_snapshots s
JOIN assets a ON a.id = s.asset_id
WHERE s.id = $1
  AND a.household_id = $2
  AND a.deleted_at IS NULL
  AND s.deleted_at IS NULL;

-- name: UpdateAssetSnapshot :one
UPDATE asset_snapshots s
SET amount      = $3,
    currency    = $4,
    as_of_date  = $5,
    description = $6,
    updated_by  = $7,
    updated_at  = now()
FROM assets a
WHERE s.id = $1
  AND s.asset_id = a.id
  AND a.household_id = $2
  AND a.deleted_at IS NULL
  AND s.deleted_at IS NULL
RETURNING s.*;

-- Batch fetch of the most-recent snapshot per asset, for list views.
-- Postgres DISTINCT ON keeps the first row per asset_id given the ORDER BY,
-- which is asset_id then year_month DESC, so we get the latest valid snapshot.
-- name: ListLatestSnapshotsByAssetIDs :many
SELECT DISTINCT ON (asset_id) *
FROM asset_snapshots
WHERE asset_id = ANY($1::uuid[]) AND deleted_at IS NULL
ORDER BY asset_id, year_month DESC;

-- Full ascending value series per asset, for the Assets Home time graphs
-- (epic #204). Ascending order is what AssetTimeSeries' carry-forward sampling
-- relies on; mirrors ListInvestmentSnapshotsByInvestmentIDs.
-- name: ListAssetSnapshotsByAssetIDs :many
SELECT *
FROM asset_snapshots
WHERE asset_id = ANY($1::uuid[]) AND deleted_at IS NULL
ORDER BY asset_id, year_month;

-- name: SoftDeleteAssetSnapshot :execrows
UPDATE asset_snapshots s
SET deleted_at = now(),
    updated_by = $3,
    updated_at = now()
FROM assets a
WHERE s.id = $1
  AND s.asset_id = a.id
  AND a.household_id = $2
  AND a.deleted_at IS NULL
  AND s.deleted_at IS NULL;

-- GetAssetForImport returns the display name + native currency of an owned
-- asset. Doubles as the ownership/existence check for the snapshot importer:
-- ErrNoRows means the asset doesn't exist in this household (or is deleted),
-- which the repo maps to ErrNotFound -> 404.
-- name: GetAssetForImport :one
SELECT a.display_name, a.native_currency
FROM assets a
WHERE a.id = $1 AND a.household_id = $2 AND a.deleted_at IS NULL;

-- ListEligibleAssetIDsForMonth returns the ids of the household's assets that
-- may legitimately hold a snapshot for the given target month (ADR-0046
-- month-aware eligibility): owned, not deleted, and either still active or
-- terminated in the target month or later. An asset terminated *before* the
-- target month never appears — its forward contribution is already frozen by
-- the carry-forward rule. There is deliberately no created_at guard: created_at
-- is a record timestamp, not economic existence, and gating on it would block
-- legitimate backfill of pre-creation months (the importer) and onboarding
-- (enter last month for an account added today) — the per-position dialog has
-- no such guard either.
-- name: ListEligibleAssetIDsForMonth :many
SELECT id
FROM assets
WHERE household_id = sqlc.arg('household_id')::uuid
  AND deleted_at IS NULL
  AND (terminated_at IS NULL OR terminated_at >= sqlc.arg('year_month')::date);

-- ListEligibleAssetsForMonth is ListEligibleAssetIDsForMonth plus the display
-- fields the bulk monthly-entry list needs (ADR-0046): name, native currency,
-- and subtype so the entry view can group by type (bank account / property /
-- vehicle). Ordered by subtype then display name so rows arrive pre-grouped —
-- the subtype strings sort bank_account < property < vehicle, the order the
-- entry view presents.
-- name: ListEligibleAssetsForMonth :many
SELECT id, display_name, native_currency, subtype, ownership_type, sole_owner_user_id
FROM assets
WHERE household_id = sqlc.arg('household_id')::uuid
  AND deleted_at IS NULL
  AND (terminated_at IS NULL OR terminated_at >= sqlc.arg('year_month')::date)
ORDER BY subtype, display_name;

-- ListLatestSnapshotsByAssetIDsAsOfMonth returns, per asset, the most-recent
-- snapshot at or before the target month — the carry-forward prefill for the
-- entry list. Month-bounded so a value entered ahead of the target does not
-- leak backwards as the prefill.
-- name: ListLatestSnapshotsByAssetIDsAsOfMonth :many
SELECT DISTINCT ON (asset_id) asset_id, amount, year_month
FROM asset_snapshots
WHERE asset_id = ANY(sqlc.arg('asset_ids')::uuid[])
  AND deleted_at IS NULL
  AND year_month <= sqlc.arg('year_month')::date
ORDER BY asset_id, year_month DESC;

-- UpsertAssetSnapshot inserts a snapshot or, when one already exists for the
-- (asset_id, year_month) pair, overwrites it (last-write-wins) — the importer
-- needs idempotent re-runs of a multi-year backfill. ON CONFLICT targets the
-- partial unique index, so its predicate (deleted_at IS NULL) is repeated.
-- created_by is only set on insert; updated_by always.
-- name: UpsertAssetSnapshot :one
WITH owned_asset AS (
    SELECT a.id AS aid
    FROM assets a
    WHERE a.id = $1 AND a.household_id = sqlc.arg('household_id')::uuid AND a.deleted_at IS NULL
)
INSERT INTO asset_snapshots (
    asset_id, year_month, amount, currency, as_of_date, description,
    created_by, updated_by
)
SELECT owned_asset.aid, $2, $3, $4, $5, $6, $7, $7
FROM owned_asset
ON CONFLICT (asset_id, year_month) WHERE deleted_at IS NULL
DO UPDATE SET
    amount      = EXCLUDED.amount,
    currency    = EXCLUDED.currency,
    as_of_date  = EXCLUDED.as_of_date,
    description = EXCLUDED.description,
    updated_by  = EXCLUDED.updated_by,
    updated_at  = now()
RETURNING *;
