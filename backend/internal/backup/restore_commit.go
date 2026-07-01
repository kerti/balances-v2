package backup

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kerti/balances-v2/backend/internal/db"
)

// Commit performs the destructive restore (ADR-0036): in one all-or-nothing
// transaction it wipes the caller's current Household entirely, then loads the
// validated backup verbatim — adopting the backup's Household UUID. Any failure
// rolls the whole thing back, leaving the caller's data exactly as it was
// ("nothing was changed"). The wipe includes the caller's sessions, so a
// successful restore forces re-login; the returned restored UUID is the caller's
// original id in the backup, which the handler re-issues a session against.
//
// For a local (password) caller the credential is carried across the wipe inside
// this transaction (ADR-0039): the bootstrap row's password hash is stashed
// before the wipe and re-inserted against the restored UUID after load, so the
// hash moves DB-row→DB-row and is never read from or written to the backup file.
// A Google caller has no local credential and re-links by google_sub, so nothing
// is carried.
//
// Callers MUST Parse + Validate the envelope first — Commit trusts that the
// graph is internally consistent and that the caller belongs to the backup.
func Commit(ctx context.Context, pool *pgxpool.Pool, env *Envelope, caller Caller) (uuid.UUID, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return uuid.Nil, fmt.Errorf("restore: begin tx: %w", err)
	}
	// Rollback is a no-op once Commit succeeds; on any early return it undoes the
	// whole wipe+load so the caller's data is left untouched.
	defer func() { _ = tx.Rollback(ctx) }()

	q := db.New(tx)

	// Stash the local caller's password hash before the wipe deletes it. A Google
	// caller (or a local caller who somehow lacks a credential row) carries
	// nothing — pgx.ErrNoRows is not an error here.
	var stashedHash *string
	if caller.GoogleSub == nil {
		cred, e := q.GetLocalCredentialByUserID(ctx, caller.UserID)
		switch {
		case e == nil:
			stashedHash = &cred.PasswordHash
		case errors.Is(e, pgx.ErrNoRows):
			// no credential to carry
		default:
			return uuid.Nil, fmt.Errorf("restore: stash credential: %w", e)
		}
	}

	if err := wipeHousehold(ctx, tx, caller.HouseholdID); err != nil {
		return uuid.Nil, err
	}
	if err := loadHousehold(ctx, tx, &env.Household); err != nil {
		return uuid.Nil, err
	}

	// The caller's restored (backup's original) UUID. Validate already proved the
	// match, so a miss here is defensive.
	restoredID, ok := matchCaller(env, caller)
	if !ok {
		return uuid.Nil, ErrNotMemberOfBackup
	}

	// Re-bind the carried credential to the restored UUID so a local restorer keeps
	// their password (DB-row→DB-row, nothing from the file — ADR-0039).
	if stashedHash != nil {
		if _, err := q.UpsertLocalCredential(ctx, db.UpsertLocalCredentialParams{
			UserID:       restoredID,
			PasswordHash: *stashedHash,
		}); err != nil {
			return uuid.Nil, fmt.Errorf("restore: re-bind credential: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, fmt.Errorf("restore: commit tx: %w", err)
	}
	return restoredID, nil
}

// wipeDeletes empties one Household in strict child→parent order. The schema has
// no ON DELETE CASCADE rules, so every dependent row is removed explicitly
// before its parent; tables without a household_id column are scoped through
// their parent. Derived/cached and invitation/session rows are not part of a
// backup but reference the Household or its users, so they are wiped too. The
// lone UPDATE breaks the non-deferrable households<->users FK cycle: the
// Household's audit columns point at users we are about to delete. $1 is the
// Household id.
var wipeDeletes = []string{
	// History: snapshots + the investment ledger (reference positions + users).
	`DELETE FROM asset_snapshots         WHERE asset_id       IN (SELECT id FROM assets      WHERE household_id = $1)`,
	`DELETE FROM liability_snapshots     WHERE liability_id   IN (SELECT id FROM liabilities WHERE household_id = $1)`,
	`DELETE FROM receivable_snapshots    WHERE receivable_id  IN (SELECT id FROM receivables WHERE household_id = $1)`,
	`DELETE FROM investment_snapshots    WHERE investment_id  IN (SELECT id FROM investments WHERE household_id = $1)`,
	`DELETE FROM investment_transactions WHERE investment_id  IN (SELECT id FROM investments WHERE household_id = $1)`,
	// Position detail tables (1:1 with their position).
	`DELETE FROM bank_account_details    WHERE asset_id       IN (SELECT id FROM assets      WHERE household_id = $1)`,
	`DELETE FROM property_details        WHERE asset_id       IN (SELECT id FROM assets      WHERE household_id = $1)`,
	`DELETE FROM vehicle_details         WHERE asset_id       IN (SELECT id FROM assets      WHERE household_id = $1)`,
	`DELETE FROM stock_details           WHERE investment_id  IN (SELECT id FROM investments WHERE household_id = $1)`,
	`DELETE FROM mutual_fund_details     WHERE investment_id  IN (SELECT id FROM investments WHERE household_id = $1)`,
	`DELETE FROM bond_details            WHERE investment_id  IN (SELECT id FROM investments WHERE household_id = $1)`,
	`DELETE FROM gold_details            WHERE investment_id  IN (SELECT id FROM investments WHERE household_id = $1)`,
	`DELETE FROM time_deposit_details    WHERE investment_id  IN (SELECT id FROM investments WHERE household_id = $1)`,
	// Not in the backup, but they reference the Household/users and would block it.
	`DELETE FROM monthly_reports         WHERE household_id = $1`,
	`DELETE FROM household_invitations   WHERE household_id = $1`,
	// Standalone flows.
	`DELETE FROM income                  WHERE household_id = $1`,
	`DELETE FROM fx_rates                WHERE household_id = $1`,
	// Positions (now free of detail/history/invitation references).
	`DELETE FROM assets                  WHERE household_id = $1`,
	`DELETE FROM investments             WHERE household_id = $1`,
	`DELETE FROM liabilities             WHERE household_id = $1`,
	`DELETE FROM receivables             WHERE household_id = $1`,
	// Tags (referenced by positions, now gone).
	`DELETE FROM tags                    WHERE household_id = $1`,
	// Null the Household's audit columns so deleting its users no longer violates
	// the households<->users FK, then drop sessions → users → the Household.
	`UPDATE households SET created_by = NULL, updated_by = NULL WHERE id = $1`,
	`DELETE FROM sessions                WHERE user_id        IN (SELECT id FROM users WHERE household_id = $1)`,
	`DELETE FROM users                   WHERE household_id = $1`,
	`DELETE FROM households              WHERE id = $1`,
}

func wipeHousehold(ctx context.Context, tx pgx.Tx, hid uuid.UUID) error {
	for _, stmt := range wipeDeletes {
		if _, err := tx.Exec(ctx, stmt, hid); err != nil {
			return fmt.Errorf("restore wipe: %w", err)
		}
	}
	return nil
}

// loadHousehold inserts the backup's Household verbatim, parents-before-children
// (ADR-0036). Each section is round-tripped through json_populate_recordset,
// which maps JSON keys to columns by name and yields rows in the table's column
// order — so a verbatim row goes in with no hand-written column lists to drift
// against the schema. A bulk INSERT ... SELECT lets Postgres check foreign keys
// at statement end, which is what lets the investments self-reference
// (rolled_from_investment_id) load in one shot regardless of row order.
func loadHousehold(ctx context.Context, tx pgx.Tx, d *HouseholdData) error {
	// The households<->users FK cycle is non-deferrable, so insert the Household
	// with its audit columns nulled, add the users, then backfill the audit.
	if err := insertHousehold(ctx, tx, &d.Household); err != nil {
		return err
	}
	if err := insertSection(ctx, tx, "users", d.Users); err != nil {
		return err
	}
	if err := backfillHouseholdAudit(ctx, tx, &d.Household); err != nil {
		return err
	}

	sections := []struct {
		table string
		rows  any
	}{
		{"tags", d.Tags},
		{"assets", d.Assets},
		{"investments", d.Investments},
		{"liabilities", d.Liabilities},
		{"receivables", d.Receivables},
		{"bank_account_details", d.BankAccounts},
		{"property_details", d.Properties},
		{"vehicle_details", d.Vehicles},
		{"stock_details", d.Stocks},
		{"mutual_fund_details", d.MutualFunds},
		{"bond_details", d.Bonds},
		{"gold_details", d.Golds},
		{"time_deposit_details", d.TimeDeposits},
		{"asset_snapshots", d.AssetSnapshots},
		{"liability_snapshots", d.LiabilitySnapshots},
		{"receivable_snapshots", d.ReceivableSnapshots},
		{"investment_snapshots", d.InvestmentSnapshots},
		{"investment_transactions", d.InvestmentTransactions},
		{"income", d.Income},
		{"fx_rates", d.FxRates},
	}
	for _, s := range sections {
		if err := insertSection(ctx, tx, s.table, s.rows); err != nil {
			return err
		}
	}
	return nil
}

// insertSection bulk-inserts one backup section verbatim. table is a fixed
// internal identifier (never user input), so interpolating it into the
// json_populate_recordset target type is safe.
func insertSection(ctx context.Context, tx pgx.Tx, table string, rows any) error {
	raw, err := json.Marshal(rows)
	if err != nil {
		return fmt.Errorf("restore load %s: marshal: %w", table, err)
	}
	// An absent/empty section marshals to "null" or "[]"; json_populate_recordset
	// rejects a JSON scalar, and there is nothing to insert anyway.
	if s := string(raw); s == "null" || s == "[]" {
		return nil
	}
	stmt := fmt.Sprintf(
		"INSERT INTO %s SELECT * FROM json_populate_recordset(NULL::%s, $1::json)", table, table)
	if _, err := tx.Exec(ctx, stmt, raw); err != nil {
		return fmt.Errorf("restore load %s: %w", table, err)
	}
	return nil
}

// insertHousehold inserts the Household row with its created_by/updated_by audit
// columns deliberately omitted (left NULL); they are backfilled by
// backfillHouseholdAudit once the users land, because the households<->users FK
// cycle is non-deferrable.
func insertHousehold(ctx context.Context, tx pgx.Tx, h *db.Household) error {
	raw, err := json.Marshal(h)
	if err != nil {
		return fmt.Errorf("restore load household: marshal: %w", err)
	}
	const stmt = `
		INSERT INTO households
			(id, display_name, reporting_currency, created_at, updated_at, deleted_at, multi_currency_enabled)
		SELECT id, display_name, reporting_currency, created_at, updated_at, deleted_at, multi_currency_enabled
		FROM json_populate_record(NULL::households, $1::json)`
	if _, err := tx.Exec(ctx, stmt, raw); err != nil {
		return fmt.Errorf("restore load household: %w", err)
	}
	return nil
}

// backfillHouseholdAudit sets the Household's audit columns from the backup, now
// that its users exist to satisfy the FK.
func backfillHouseholdAudit(ctx context.Context, tx pgx.Tx, h *db.Household) error {
	raw, err := json.Marshal(h)
	if err != nil {
		return fmt.Errorf("restore household audit: marshal: %w", err)
	}
	const stmt = `
		UPDATE households AS t
		SET created_by = s.created_by, updated_by = s.updated_by
		FROM json_populate_record(NULL::households, $1::json) AS s
		WHERE t.id = s.id`
	if _, err := tx.Exec(ctx, stmt, raw); err != nil {
		return fmt.Errorf("restore household audit: %w", err)
	}
	return nil
}
