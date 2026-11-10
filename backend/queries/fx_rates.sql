-- Manual monthly FX rates (ADR-0002). Household-scoped; `rate` is reporting-
-- currency units per 1 unit of `currency`. year_month + currency are the
-- identity (one rate per month per currency) — to change those, delete and
-- recreate; UpdateFxRate edits only the rate.

-- name: CreateFxRate :one
INSERT INTO fx_rates (
    household_id, year_month, currency, rate, created_by, updated_by
) VALUES (
    $1, $2, $3, $4, $5, $5
)
RETURNING *;

-- name: ListFxRatesByHousehold :many
SELECT *
FROM fx_rates
WHERE household_id = $1 AND deleted_at IS NULL
ORDER BY year_month DESC, currency ASC;

-- name: GetFxRateByID :one
SELECT *
FROM fx_rates
WHERE id = $1 AND household_id = $2 AND deleted_at IS NULL;

-- name: UpdateFxRate :one
UPDATE fx_rates
SET rate       = $3,
    updated_by = $4,
    updated_at = now()
WHERE id = $1 AND household_id = $2 AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteFxRate :execrows
UPDATE fx_rates
SET deleted_at = now(),
    updated_by = $3,
    updated_at = now()
WHERE id = $1 AND household_id = $2 AND deleted_at IS NULL;

-- RestoreFxRate undoes a delete (ADR-0058 §2). A unique_violation on the live
-- (household, year_month, currency) index means the rate was re-entered meanwhile.
-- name: RestoreFxRate :execrows
UPDATE fx_rates
SET deleted_at = NULL,
    updated_by = $3,
    updated_at = now()
WHERE id = $1 AND household_id = $2 AND deleted_at IS NOT NULL;
