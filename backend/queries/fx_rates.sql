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

-- ListUsedForeignCurrencies: every currency a live position's non-deleted
-- snapshot is recorded in, minus the household's reporting currency. The same
-- inputs the report engine converts (ListXSnapshotsForReport), so this is the
-- full set a rate can apply to - not only those currently missing one
-- (a user adds a rate for another month). Feeds the New-rate currency select.
-- name: ListUsedForeignCurrencies :many
SELECT currency::text AS currency FROM (
    SELECT s.currency FROM asset_snapshots s
        JOIN assets a ON a.id = s.asset_id
        WHERE a.household_id = $1 AND a.deleted_at IS NULL AND s.deleted_at IS NULL
    UNION
    SELECT s.currency FROM liability_snapshots s
        JOIN liabilities l ON l.id = s.liability_id
        WHERE l.household_id = $1 AND l.deleted_at IS NULL AND s.deleted_at IS NULL
    UNION
    SELECT s.currency FROM receivable_snapshots s
        JOIN receivables rc ON rc.id = s.receivable_id
        WHERE rc.household_id = $1 AND rc.deleted_at IS NULL AND s.deleted_at IS NULL
    UNION
    SELECT s.currency FROM investment_snapshots s
        JOIN investments i ON i.id = s.investment_id
        WHERE i.household_id = $1 AND i.deleted_at IS NULL AND s.deleted_at IS NULL
) used
WHERE currency <> (SELECT h.reporting_currency FROM households h WHERE h.id = $1)
ORDER BY currency;
