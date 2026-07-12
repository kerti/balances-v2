-- Manual monthly inflation rates (ADR-0048). Household-scoped; `rate` is an
-- annualized (YoY) percentage for that month (may be negative — deflation).
-- year_month is the identity (one rate per month) — to change it, delete and
-- recreate; UpdateInflationRate edits only the rate. Feeds Fund Resilience.

-- name: CreateInflationRate :one
INSERT INTO inflation_rates (
    household_id, year_month, rate, created_by, updated_by
) VALUES (
    $1, $2, $3, $4, $4
)
RETURNING *;

-- name: ListInflationRatesByHousehold :many
SELECT *
FROM inflation_rates
WHERE household_id = $1 AND deleted_at IS NULL
ORDER BY year_month DESC;

-- name: GetInflationRateByID :one
SELECT *
FROM inflation_rates
WHERE id = $1 AND household_id = $2 AND deleted_at IS NULL;

-- name: UpdateInflationRate :one
UPDATE inflation_rates
SET rate       = $3,
    updated_by = $4,
    updated_at = now()
WHERE id = $1 AND household_id = $2 AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteInflationRate :execrows
UPDATE inflation_rates
SET deleted_at = now(),
    updated_by = $3,
    updated_at = now()
WHERE id = $1 AND household_id = $2 AND deleted_at IS NULL;

-- name: ListInflationRatesForExport :many
SELECT * FROM inflation_rates
WHERE household_id = sqlc.arg(household_id)
  AND (deleted_at IS NULL OR sqlc.arg(include_deleted)::bool)
ORDER BY year_month, id;
