-- Backup export reads (ADR-0036, issue #174).
--
-- Every query is scoped to one Household and takes an include_deleted flag:
--   full fidelity  -> include_deleted = true  (carry soft-deleted rows verbatim)
--   compacted      -> include_deleted = false (live rows only)
-- Detail tables (1:1 with their position, no own deleted_at/household_id) and
-- snapshot/transaction tables are scoped by joining their parent on
-- household_id; their liveness follows the parent's deleted_at too, so a
-- compacted backup never carries orphaned history of a deleted position.
-- Deterministic ORDER BY keeps exports stable (round-trip + test friendliness).

-- name: GetHouseholdForExport :one
SELECT * FROM households WHERE id = sqlc.arg(household_id);

-- name: ListUsersForExport :many
SELECT * FROM users
WHERE household_id = sqlc.arg(household_id)
  AND (deleted_at IS NULL OR sqlc.arg(include_deleted)::bool)
ORDER BY created_at, id;

-- name: ListTagsForExport :many
SELECT * FROM tags
WHERE household_id = sqlc.arg(household_id)
  AND (deleted_at IS NULL OR sqlc.arg(include_deleted)::bool)
ORDER BY created_at, id;

-- name: ListAssetsForExport :many
SELECT * FROM assets
WHERE household_id = sqlc.arg(household_id)
  AND (deleted_at IS NULL OR sqlc.arg(include_deleted)::bool)
ORDER BY created_at, id;

-- name: ListLiabilitiesForExport :many
SELECT * FROM liabilities
WHERE household_id = sqlc.arg(household_id)
  AND (deleted_at IS NULL OR sqlc.arg(include_deleted)::bool)
ORDER BY created_at, id;

-- name: ListReceivablesForExport :many
SELECT * FROM receivables
WHERE household_id = sqlc.arg(household_id)
  AND (deleted_at IS NULL OR sqlc.arg(include_deleted)::bool)
ORDER BY created_at, id;

-- name: ListInvestmentsForExport :many
SELECT * FROM investments
WHERE household_id = sqlc.arg(household_id)
  AND (deleted_at IS NULL OR sqlc.arg(include_deleted)::bool)
ORDER BY created_at, id;

-- name: ListIncomeForExport :many
SELECT * FROM income
WHERE household_id = sqlc.arg(household_id)
  AND (deleted_at IS NULL OR sqlc.arg(include_deleted)::bool)
ORDER BY date, id;

-- name: ListFxRatesForExport :many
SELECT * FROM fx_rates
WHERE household_id = sqlc.arg(household_id)
  AND (deleted_at IS NULL OR sqlc.arg(include_deleted)::bool)
ORDER BY year_month, currency, id;

-- ----- Asset detail tables (scoped via assets) ----------------------------

-- name: ListBankAccountsForExport :many
SELECT d.* FROM bank_account_details d
JOIN assets a ON a.id = d.asset_id
WHERE a.household_id = sqlc.arg(household_id)
  AND (a.deleted_at IS NULL OR sqlc.arg(include_deleted)::bool)
ORDER BY d.asset_id;

-- name: ListPropertiesForExport :many
SELECT d.* FROM property_details d
JOIN assets a ON a.id = d.asset_id
WHERE a.household_id = sqlc.arg(household_id)
  AND (a.deleted_at IS NULL OR sqlc.arg(include_deleted)::bool)
ORDER BY d.asset_id;

-- name: ListVehiclesForExport :many
SELECT d.* FROM vehicle_details d
JOIN assets a ON a.id = d.asset_id
WHERE a.household_id = sqlc.arg(household_id)
  AND (a.deleted_at IS NULL OR sqlc.arg(include_deleted)::bool)
ORDER BY d.asset_id;

-- ----- Investment detail tables (scoped via investments) ------------------

-- name: ListStocksForExport :many
SELECT d.* FROM stock_details d
JOIN investments i ON i.id = d.investment_id
WHERE i.household_id = sqlc.arg(household_id)
  AND (i.deleted_at IS NULL OR sqlc.arg(include_deleted)::bool)
ORDER BY d.investment_id;

-- name: ListMutualFundsForExport :many
SELECT d.* FROM mutual_fund_details d
JOIN investments i ON i.id = d.investment_id
WHERE i.household_id = sqlc.arg(household_id)
  AND (i.deleted_at IS NULL OR sqlc.arg(include_deleted)::bool)
ORDER BY d.investment_id;

-- name: ListBondsForExport :many
SELECT d.* FROM bond_details d
JOIN investments i ON i.id = d.investment_id
WHERE i.household_id = sqlc.arg(household_id)
  AND (i.deleted_at IS NULL OR sqlc.arg(include_deleted)::bool)
ORDER BY d.investment_id;

-- name: ListGoldsForExport :many
SELECT d.* FROM gold_details d
JOIN investments i ON i.id = d.investment_id
WHERE i.household_id = sqlc.arg(household_id)
  AND (i.deleted_at IS NULL OR sqlc.arg(include_deleted)::bool)
ORDER BY d.investment_id;

-- name: ListTimeDepositsForExport :many
SELECT d.* FROM time_deposit_details d
JOIN investments i ON i.id = d.investment_id
WHERE i.household_id = sqlc.arg(household_id)
  AND (i.deleted_at IS NULL OR sqlc.arg(include_deleted)::bool)
ORDER BY d.investment_id;

-- ----- Snapshots + ledger (scoped via parent; liveness follows parent) -----

-- name: ListAssetSnapshotsForExport :many
SELECT s.* FROM asset_snapshots s
JOIN assets a ON a.id = s.asset_id
WHERE a.household_id = sqlc.arg(household_id)
  AND (sqlc.arg(include_deleted)::bool OR (s.deleted_at IS NULL AND a.deleted_at IS NULL))
ORDER BY s.asset_id, s.year_month, s.id;

-- name: ListLiabilitySnapshotsForExport :many
SELECT s.* FROM liability_snapshots s
JOIN liabilities l ON l.id = s.liability_id
WHERE l.household_id = sqlc.arg(household_id)
  AND (sqlc.arg(include_deleted)::bool OR (s.deleted_at IS NULL AND l.deleted_at IS NULL))
ORDER BY s.liability_id, s.year_month, s.id;

-- name: ListReceivableSnapshotsForExport :many
SELECT s.* FROM receivable_snapshots s
JOIN receivables r ON r.id = s.receivable_id
WHERE r.household_id = sqlc.arg(household_id)
  AND (sqlc.arg(include_deleted)::bool OR (s.deleted_at IS NULL AND r.deleted_at IS NULL))
ORDER BY s.receivable_id, s.year_month, s.id;

-- name: ListInvestmentSnapshotsForExport :many
SELECT s.* FROM investment_snapshots s
JOIN investments i ON i.id = s.investment_id
WHERE i.household_id = sqlc.arg(household_id)
  AND (sqlc.arg(include_deleted)::bool OR (s.deleted_at IS NULL AND i.deleted_at IS NULL))
ORDER BY s.investment_id, s.year_month, s.id;

-- name: ListInvestmentTransactionsForExport :many
SELECT t.* FROM investment_transactions t
JOIN investments i ON i.id = t.investment_id
WHERE i.household_id = sqlc.arg(household_id)
  AND (sqlc.arg(include_deleted)::bool OR (t.deleted_at IS NULL AND i.deleted_at IS NULL))
ORDER BY t.investment_id, t.transaction_date, t.id;
