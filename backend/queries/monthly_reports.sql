-- Materialized monthly net-worth reports (ADR-0006 / ADR-0012).
--
-- The report row is a regenerable cache keyed by (household_id, year_month);
-- UpsertMonthlyReport writes in place on regeneration. The *ForReport queries
-- below feed the pure engine in internal/repo (monthly_reports_engine.go):
-- one lightweight position fetch + one flat snapshot fetch per group, from
-- which the engine derives every month with carry-forward in Go.
--
-- Slice-1 scope: only the net-worth columns + user_breakdowns + stale_positions
-- are written; the income-statement columns and fx JSON land in M5 slices 2-3.

-- name: ListMonthlyReports :many
SELECT *
FROM monthly_reports
WHERE household_id = $1
ORDER BY year_month;

-- name: GetMonthlyReport :one
SELECT *
FROM monthly_reports
WHERE household_id = $1 AND year_month = $2;

-- Drop cache rows for months no longer in range (e.g. the earliest snapshots
-- were deleted, shrinking the report window). Keeps the materialized set in
-- sync with the engine's month range on regeneration.
-- name: DeleteMonthlyReportsOutsideRange :exec
DELETE FROM monthly_reports
WHERE household_id = $1 AND (year_month < $2 OR year_month > $3);

-- name: UpsertMonthlyReport :one
INSERT INTO monthly_reports (
    household_id, year_month, generated_at,
    nw_total, nw_assets, nw_liabilities, nw_receivables, nw_investments,
    earned_income_total, earned_income_salary, earned_income_business,
    earned_income_rental, earned_income_gift, earned_income_tax_refund,
    earned_income_insurance, earned_income_other,
    investment_return_total, investment_return_stock, investment_return_mutual_fund,
    investment_return_bond, investment_return_gold, investment_return_time_deposit,
    asset_value_change, derived_living_expenses,
    user_breakdowns, stale_positions, fx_rates_used, missing_fx
) VALUES (
    $1, $2, now(),
    $3, $4, $5, $6, $7,
    $8, $9, $10, $11, $12, $13, $14, $15,
    $16, $17, $18, $19, $20, $21,
    $22, $23,
    $24, $25, $26, $27
)
ON CONFLICT (household_id, year_month) DO UPDATE SET
    generated_at                   = now(),
    nw_total                       = EXCLUDED.nw_total,
    nw_assets                      = EXCLUDED.nw_assets,
    nw_liabilities                 = EXCLUDED.nw_liabilities,
    nw_receivables                 = EXCLUDED.nw_receivables,
    nw_investments                 = EXCLUDED.nw_investments,
    earned_income_total            = EXCLUDED.earned_income_total,
    earned_income_salary           = EXCLUDED.earned_income_salary,
    earned_income_business         = EXCLUDED.earned_income_business,
    earned_income_rental           = EXCLUDED.earned_income_rental,
    earned_income_gift             = EXCLUDED.earned_income_gift,
    earned_income_tax_refund       = EXCLUDED.earned_income_tax_refund,
    earned_income_insurance        = EXCLUDED.earned_income_insurance,
    earned_income_other            = EXCLUDED.earned_income_other,
    investment_return_total        = EXCLUDED.investment_return_total,
    investment_return_stock        = EXCLUDED.investment_return_stock,
    investment_return_mutual_fund  = EXCLUDED.investment_return_mutual_fund,
    investment_return_bond         = EXCLUDED.investment_return_bond,
    investment_return_gold         = EXCLUDED.investment_return_gold,
    investment_return_time_deposit = EXCLUDED.investment_return_time_deposit,
    asset_value_change             = EXCLUDED.asset_value_change,
    derived_living_expenses        = EXCLUDED.derived_living_expenses,
    user_breakdowns                = EXCLUDED.user_breakdowns,
    stale_positions                = EXCLUDED.stale_positions,
    fx_rates_used                  = EXCLUDED.fx_rates_used,
    missing_fx                     = EXCLUDED.missing_fx
RETURNING *;

-- Staleness watermark: the newest updated_at across every input that feeds
-- month <= $2 for household $1 (ADR-0006 conservative <=M rule). A report row
-- is stale when its generated_at predates this value. Snapshot subqueries do
-- not filter deleted_at so soft-deletes count; parent tables are household-wide
-- (metadata is timeless). Detail tables and `users` are deliberately excluded
-- (ADR-0006). Income + investment_transactions joined the set in slice 2;
-- fx_rates in slice 3.
-- name: MaxReportInputUpdatedAt :one
SELECT COALESCE(GREATEST(
    (SELECT MAX(s.updated_at) FROM asset_snapshots s
        JOIN assets a ON a.id = s.asset_id
        WHERE a.household_id = $1 AND s.year_month <= $2),
    (SELECT MAX(s.updated_at) FROM liability_snapshots s
        JOIN liabilities l ON l.id = s.liability_id
        WHERE l.household_id = $1 AND s.year_month <= $2),
    (SELECT MAX(s.updated_at) FROM receivable_snapshots s
        JOIN receivables rc ON rc.id = s.receivable_id
        WHERE rc.household_id = $1 AND s.year_month <= $2),
    (SELECT MAX(s.updated_at) FROM investment_snapshots s
        JOIN investments i ON i.id = s.investment_id
        WHERE i.household_id = $1 AND s.year_month <= $2),
    (SELECT MAX(updated_at) FROM income
        WHERE household_id = $1 AND date <= $2),
    (SELECT MAX(t.updated_at) FROM investment_transactions t
        JOIN investments i ON i.id = t.investment_id
        WHERE i.household_id = $1 AND t.transaction_date <= $2),
    (SELECT MAX(updated_at) FROM fx_rates
        WHERE household_id = $1 AND year_month <= $2),
    (SELECT MAX(updated_at) FROM assets       WHERE household_id = $1),
    (SELECT MAX(updated_at) FROM liabilities  WHERE household_id = $1),
    (SELECT MAX(updated_at) FROM receivables  WHERE household_id = $1),
    (SELECT MAX(updated_at) FROM investments  WHERE household_id = $1),
    (SELECT MAX(updated_at) FROM households   WHERE id = $1)
), to_timestamp(0))::timestamptz AS max_updated_at;

-- ----- engine inputs: positions (lifecycle + ownership) -------------------
-- terminated_at NULL => active (biconditional CHECK, migration 00012); the
-- engine needs only terminated_at for month-granular lifecycle suppression
-- plus ownership for the per-user / Joint breakdown.

-- display_name + subtype let the report name unrecorded positions in the
-- dashboard drill-down (issue #50): the stale-position list carries enough to
-- render a label and deep-link to the position's detail page.
-- name: ListAssetsForReport :many
SELECT id, display_name, subtype, ownership_type, sole_owner_user_id, terminated_at
FROM assets
WHERE household_id = $1 AND deleted_at IS NULL;

-- name: ListLiabilitiesForReport :many
SELECT id, display_name, subtype, ownership_type, sole_owner_user_id, terminated_at
FROM liabilities
WHERE household_id = $1 AND deleted_at IS NULL;

-- name: ListReceivablesForReport :many
SELECT id, display_name, ownership_type, sole_owner_user_id, terminated_at
FROM receivables
WHERE household_id = $1 AND deleted_at IS NULL;

-- native_currency + the time_deposit placement fields let the engine synthesize
-- a placement cash_in for TDs (issue #27): a TD records no Buy, so without it
-- the placement-month snapshot 0→principal reads as pure return. td.principal /
-- td.placement_date are NULL for non-TD subtypes.
-- name: ListInvestmentsForReport :many
SELECT i.id, i.display_name, i.subtype, i.ownership_type, i.sole_owner_user_id, i.terminated_at,
       i.native_currency, i.rolled_from_investment_id,
       td.principal AS td_principal, td.placement_date AS td_placement_date
FROM investments i
LEFT JOIN time_deposit_details td ON td.investment_id = i.id
WHERE i.household_id = $1 AND i.deleted_at IS NULL;

-- ----- engine inputs: income + investment transactions (slice 2) ----------

-- name: ListIncomeForReport :many
SELECT date, amount, currency, category, ownership_type, sole_owner_user_id
FROM income
WHERE household_id = $1 AND deleted_at IS NULL;

-- Transaction cash-flow fields the engine maps to cash_in/cash_out (ADR-0008).
-- price_per_unit is omitted — it doesn't enter the return formula.
-- name: ListInvestmentTransactionsForReport :many
SELECT t.investment_id, t.transaction_date, t.transaction_type, t.currency,
       t.amount, t.quantity,
       t.principal_amount, t.interest_amount,
       t.principal_disposition, t.interest_disposition
FROM investment_transactions t
JOIN investments i ON i.id = t.investment_id
WHERE i.household_id = $1 AND i.deleted_at IS NULL AND t.deleted_at IS NULL
ORDER BY t.investment_id, t.transaction_date;

-- ----- engine inputs: flat snapshots (position_id, year_month, amount) -----
-- All non-deleted snapshots for the household, ordered for the engine's
-- per-position carry-forward scan.

-- name: ListAssetSnapshotsForReport :many
SELECT s.asset_id AS position_id, s.year_month, s.amount, s.currency
FROM asset_snapshots s
JOIN assets a ON a.id = s.asset_id
WHERE a.household_id = $1 AND a.deleted_at IS NULL AND s.deleted_at IS NULL
ORDER BY s.asset_id, s.year_month;

-- name: ListLiabilitySnapshotsForReport :many
SELECT s.liability_id AS position_id, s.year_month, s.amount, s.currency
FROM liability_snapshots s
JOIN liabilities l ON l.id = s.liability_id
WHERE l.household_id = $1 AND l.deleted_at IS NULL AND s.deleted_at IS NULL
ORDER BY s.liability_id, s.year_month;

-- name: ListReceivableSnapshotsForReport :many
SELECT s.receivable_id AS position_id, s.year_month, s.amount, s.currency
FROM receivable_snapshots s
JOIN receivables rc ON rc.id = s.receivable_id
WHERE rc.household_id = $1 AND rc.deleted_at IS NULL AND s.deleted_at IS NULL
ORDER BY s.receivable_id, s.year_month;

-- name: ListInvestmentSnapshotsForReport :many
SELECT s.investment_id AS position_id, s.year_month, s.amount, s.currency
FROM investment_snapshots s
JOIN investments i ON i.id = s.investment_id
WHERE i.household_id = $1 AND i.deleted_at IS NULL AND s.deleted_at IS NULL
ORDER BY s.investment_id, s.year_month;

-- All non-deleted FX rates for the household; the engine indexes them by
-- currency and applies the latest with year_month <= M (carry-forward).
-- name: ListFxRatesForReport :many
SELECT currency, year_month, rate
FROM fx_rates
WHERE household_id = $1 AND deleted_at IS NULL
ORDER BY currency, year_month;
