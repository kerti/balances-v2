package repo_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/kerti/balances-v2/backend/internal/db"
	"github.com/kerti/balances-v2/backend/internal/identity"
	"github.com/kerti/balances-v2/backend/internal/repo"
	"github.com/kerti/balances-v2/backend/internal/testutil"
)

// Replacing a snapshot archives, never overwrites (ADR-0058 §1). These tests
// drive every write that changes an existing snapshot's values (Edit, bulk
// save, import) across all four snapshot groups and all three shapes, and pin
// the same properties for each: one live row per month holding the new values,
// the earlier row kept as a soft-deleted row with `supersedes` still null,
// identical values writing nothing, and a failure midway leaving nothing
// changed.

var (
	replSep = time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC)
	replOct = time.Date(2026, time.October, 1, 0, 0, 0, 0, time.UTC)
	replNov = time.Date(2026, time.November, 1, 0, 0, 0, 0, time.UTC)
)

func rd(s string) decimal.Decimal { return decimal.RequireFromString(s) }

func dayOf(y int, m time.Month, d int) *time.Time {
	t := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	return &t
}

// replaceGroup is one snapshot shape's adapter onto the shared scenarios. Amount
// is the only varying value; the investment adapters derive their shape columns
// from it (quantity = amount, price 1; accrued interest fixed) so the stored
// amount equals the argument.
type replaceGroup struct {
	name, table, fkCol string
	newPosition        func(t *testing.T, ctx context.Context) uuid.UUID
	create             func(t *testing.T, ctx context.Context, pos uuid.UUID, month time.Time, amt decimal.Decimal) uuid.UUID
	edit               func(ctx context.Context, snapID uuid.UUID, amt decimal.Decimal, currency string, asOf *time.Time) (live uuid.UUID, replaced *uuid.UUID, err error)
	bulk               func(ctx context.Context, pos uuid.UUID, month time.Time, amt decimal.Decimal, asOf *time.Time) (repo.BulkResult, []repo.BulkSnapshotRowError, error)
	imp                func(ctx context.Context, pos uuid.UUID, rows []replaceImportRow, dry bool) (*repo.ImportResult, error)
}

type replaceImportRow struct {
	month time.Time
	amt   decimal.Decimal
	asOf  *time.Time
}

func replaceGroups(pool *pgxpool.Pool) []replaceGroup {
	ar := repo.NewAssetRepo(pool)
	lr := repo.NewLiabilityRepo(pool)
	rr := repo.NewReceivableRepo(pool)
	ir := repo.NewInvestmentRepo(pool)

	flatRows := func(rows []replaceImportRow) []repo.ImportSnapshotRow {
		out := make([]repo.ImportSnapshotRow, len(rows))
		for i, r := range rows {
			out[i] = repo.ImportSnapshotRow{YearMonth: r.month, Amount: r.amt, Currency: "IDR", AsOfDate: r.asOf}
		}
		return out
	}
	stockRows := func(rows []replaceImportRow) []repo.ImportInvestmentSnapshotRow {
		out := make([]repo.ImportInvestmentSnapshotRow, len(rows))
		for i, r := range rows {
			q, p := r.amt, decimal.NewFromInt(1)
			out[i] = repo.ImportInvestmentSnapshotRow{YearMonth: r.month, Amount: r.amt, Currency: "IDR", Quantity: &q, PricePerUnit: &p, AsOfDate: r.asOf}
		}
		return out
	}
	bondRows := func(rows []replaceImportRow) []repo.ImportInvestmentSnapshotRow {
		out := make([]repo.ImportInvestmentSnapshotRow, len(rows))
		for i, r := range rows {
			a := decimal.NewFromInt(1)
			out[i] = repo.ImportInvestmentSnapshotRow{YearMonth: r.month, Amount: r.amt, Currency: "IDR", AccruedInterest: &a, AsOfDate: r.asOf}
		}
		return out
	}
	return []replaceGroup{
		{
			name: "asset", table: "asset_snapshots", fkCol: "asset_id",
			newPosition: func(t *testing.T, ctx context.Context) uuid.UUID {
				a, err := ar.CreateBankAccount(ctx, repo.CreateBankAccountParams{
					DisplayName: "Checking", OwnershipType: "joint", NativeCurrency: "IDR",
					BankName: "BCA", AccountNumber: "1", AccountType: "savings",
				})
				if err != nil {
					t.Fatalf("CreateBankAccount: %v", err)
				}
				return a.Asset.ID
			},
			create: func(t *testing.T, ctx context.Context, pos uuid.UUID, month time.Time, amt decimal.Decimal) uuid.UUID {
				s, err := ar.CreateAssetSnapshot(ctx, repo.CreateAssetSnapshotParams{AssetID: pos, YearMonth: month, Amount: amt, Currency: "IDR"})
				if err != nil {
					t.Fatalf("CreateAssetSnapshot: %v", err)
				}
				return s.ID
			},
			edit: func(ctx context.Context, id uuid.UUID, amt decimal.Decimal, cur string, asOf *time.Time) (uuid.UUID, *uuid.UUID, error) {
				w, err := ar.UpdateAssetSnapshot(ctx, repo.UpdateAssetSnapshotParams{SnapshotID: id, Amount: amt, Currency: cur, AsOfDate: asOf})
				if err != nil {
					return uuid.Nil, nil, err
				}
				return w.ID, w.ReplacedID, nil
			},
			bulk: func(ctx context.Context, pos uuid.UUID, month time.Time, amt decimal.Decimal, asOf *time.Time) (repo.BulkResult, []repo.BulkSnapshotRowError, error) {
				return ar.BulkUpsertAssetSnapshots(ctx, repo.BulkUpsertAssetSnapshotsParams{
					YearMonth: month, AsOfDate: asOf,
					Rows: []repo.BulkAssetSnapshotRow{{AssetID: pos, Amount: amt, Currency: "IDR"}},
				})
			},
			imp: func(ctx context.Context, pos uuid.UUID, rows []replaceImportRow, dry bool) (*repo.ImportResult, error) {
				return ar.ImportAssetSnapshots(ctx, pos, flatRows(rows), dry)
			},
		},
		{
			name: "liability", table: "liability_snapshots", fkCol: "liability_id",
			newPosition: func(t *testing.T, ctx context.Context) uuid.UUID {
				l, err := lr.CreateLiability(ctx, repo.CreateLiabilityParams{
					DisplayName: "Mortgage", Subtype: "institutional", OwnershipType: "joint",
					NativeCurrency: "IDR", CounterpartyName: "Bank",
				})
				if err != nil {
					t.Fatalf("CreateLiability: %v", err)
				}
				return l.ID
			},
			create: func(t *testing.T, ctx context.Context, pos uuid.UUID, month time.Time, amt decimal.Decimal) uuid.UUID {
				s, err := lr.CreateLiabilitySnapshot(ctx, repo.CreateLiabilitySnapshotParams{LiabilityID: pos, YearMonth: month, Amount: amt, Currency: "IDR"})
				if err != nil {
					t.Fatalf("CreateLiabilitySnapshot: %v", err)
				}
				return s.ID
			},
			edit: func(ctx context.Context, id uuid.UUID, amt decimal.Decimal, cur string, asOf *time.Time) (uuid.UUID, *uuid.UUID, error) {
				w, err := lr.UpdateLiabilitySnapshot(ctx, repo.UpdateLiabilitySnapshotParams{SnapshotID: id, Amount: amt, Currency: cur, AsOfDate: asOf})
				if err != nil {
					return uuid.Nil, nil, err
				}
				return w.ID, w.ReplacedID, nil
			},
			bulk: func(ctx context.Context, pos uuid.UUID, month time.Time, amt decimal.Decimal, asOf *time.Time) (repo.BulkResult, []repo.BulkSnapshotRowError, error) {
				return lr.BulkUpsertLiabilitySnapshots(ctx, repo.BulkUpsertLiabilitySnapshotsParams{
					YearMonth: month, AsOfDate: asOf,
					Rows: []repo.BulkLiabilitySnapshotRow{{LiabilityID: pos, Amount: amt, Currency: "IDR"}},
				})
			},
			imp: func(ctx context.Context, pos uuid.UUID, rows []replaceImportRow, dry bool) (*repo.ImportResult, error) {
				return lr.ImportLiabilitySnapshots(ctx, pos, flatRows(rows), dry)
			},
		},
		{
			name: "receivable", table: "receivable_snapshots", fkCol: "receivable_id",
			newPosition: func(t *testing.T, ctx context.Context) uuid.UUID {
				v, err := rr.CreateReceivable(ctx, repo.CreateReceivableParams{
					DisplayName: "Loan", OwnershipType: "joint", NativeCurrency: "IDR", CounterpartyName: "Bob",
				})
				if err != nil {
					t.Fatalf("CreateReceivable: %v", err)
				}
				return v.ID
			},
			create: func(t *testing.T, ctx context.Context, pos uuid.UUID, month time.Time, amt decimal.Decimal) uuid.UUID {
				s, err := rr.CreateReceivableSnapshot(ctx, repo.CreateReceivableSnapshotParams{ReceivableID: pos, YearMonth: month, Amount: amt, Currency: "IDR"})
				if err != nil {
					t.Fatalf("CreateReceivableSnapshot: %v", err)
				}
				return s.ID
			},
			edit: func(ctx context.Context, id uuid.UUID, amt decimal.Decimal, cur string, asOf *time.Time) (uuid.UUID, *uuid.UUID, error) {
				w, err := rr.UpdateReceivableSnapshot(ctx, repo.UpdateReceivableSnapshotParams{SnapshotID: id, Amount: amt, Currency: cur, AsOfDate: asOf})
				if err != nil {
					return uuid.Nil, nil, err
				}
				return w.ID, w.ReplacedID, nil
			},
			bulk: func(ctx context.Context, pos uuid.UUID, month time.Time, amt decimal.Decimal, asOf *time.Time) (repo.BulkResult, []repo.BulkSnapshotRowError, error) {
				return rr.BulkUpsertReceivableSnapshots(ctx, repo.BulkUpsertReceivableSnapshotsParams{
					YearMonth: month, AsOfDate: asOf,
					Rows: []repo.BulkReceivableSnapshotRow{{ReceivableID: pos, Amount: amt, Currency: "IDR"}},
				})
			},
			imp: func(ctx context.Context, pos uuid.UUID, rows []replaceImportRow, dry bool) (*repo.ImportResult, error) {
				return rr.ImportReceivableSnapshots(ctx, pos, flatRows(rows), dry)
			},
		},
		{
			name: "investment quantity-price", table: "investment_snapshots", fkCol: "investment_id",
			newPosition: func(t *testing.T, ctx context.Context) uuid.UUID {
				s, err := ir.CreateStock(ctx, repo.CreateStockParams{
					DisplayName: "BBCA", OwnershipType: "joint", NativeCurrency: "IDR",
					RiskProfile: "medium", Ticker: "BBCA", Exchange: "IDX",
				})
				if err != nil {
					t.Fatalf("CreateStock: %v", err)
				}
				return s.Investment.ID
			},
			create: func(t *testing.T, ctx context.Context, pos uuid.UUID, month time.Time, amt decimal.Decimal) uuid.UUID {
				q, p := amt, decimal.NewFromInt(1)
				s, err := ir.CreateInvestmentSnapshot(ctx, repo.CreateInvestmentSnapshotParams{
					InvestmentID: pos, YearMonth: month, Amount: amt, Currency: "IDR", Quantity: &q, PricePerUnit: &p,
				})
				if err != nil {
					t.Fatalf("CreateInvestmentSnapshot: %v", err)
				}
				return s.ID
			},
			edit: func(ctx context.Context, id uuid.UUID, amt decimal.Decimal, cur string, asOf *time.Time) (uuid.UUID, *uuid.UUID, error) {
				q, p := amt, decimal.NewFromInt(1)
				w, err := ir.UpdateInvestmentSnapshot(ctx, repo.UpdateInvestmentSnapshotParams{
					SnapshotID: id, Amount: amt, Currency: cur, Quantity: &q, PricePerUnit: &p, AsOfDate: asOf,
				})
				if err != nil {
					return uuid.Nil, nil, err
				}
				return w.ID, w.ReplacedID, nil
			},
			bulk: func(ctx context.Context, pos uuid.UUID, month time.Time, amt decimal.Decimal, asOf *time.Time) (repo.BulkResult, []repo.BulkSnapshotRowError, error) {
				return ir.BulkUpsertInvestmentSnapshots(ctx, repo.BulkUpsertInvestmentSnapshotsParams{
					YearMonth: month, AsOfDate: asOf,
					Rows: []repo.BulkInvestmentSnapshotRow{{InvestmentID: pos, Quantity: amt, PricePerUnit: decimal.NewFromInt(1), Currency: "IDR"}},
				})
			},
			imp: func(ctx context.Context, pos uuid.UUID, rows []replaceImportRow, dry bool) (*repo.ImportResult, error) {
				return ir.ImportInvestmentSnapshots(ctx, pos, stockRows(rows), dry)
			},
		},
		{
			name: "investment accrued", table: "investment_snapshots", fkCol: "investment_id",
			newPosition: func(t *testing.T, ctx context.Context) uuid.UUID {
				b, err := ir.CreateBond(ctx, repo.CreateBondParams{
					DisplayName: "ORI024", OwnershipType: "joint", NativeCurrency: "IDR", RiskProfile: "low",
					BondType: "secondary_market", Issuer: "Govt", CouponRate: decimal.RequireFromString("0.06"),
					CouponFrequency: "monthly", CouponDisposition: "accrues",
					MaturityDate: time.Date(2030, time.January, 1, 0, 0, 0, 0, time.UTC),
				})
				if err != nil {
					t.Fatalf("CreateBond: %v", err)
				}
				return b.Investment.ID
			},
			create: func(t *testing.T, ctx context.Context, pos uuid.UUID, month time.Time, amt decimal.Decimal) uuid.UUID {
				a := decimal.NewFromInt(1)
				s, err := ir.CreateInvestmentSnapshot(ctx, repo.CreateInvestmentSnapshotParams{
					InvestmentID: pos, YearMonth: month, Amount: amt, Currency: "IDR", AccruedInterest: &a,
				})
				if err != nil {
					t.Fatalf("CreateInvestmentSnapshot: %v", err)
				}
				return s.ID
			},
			edit: func(ctx context.Context, id uuid.UUID, amt decimal.Decimal, cur string, asOf *time.Time) (uuid.UUID, *uuid.UUID, error) {
				a := decimal.NewFromInt(1)
				w, err := ir.UpdateInvestmentSnapshot(ctx, repo.UpdateInvestmentSnapshotParams{
					SnapshotID: id, Amount: amt, Currency: cur, AccruedInterest: &a, AsOfDate: asOf,
				})
				if err != nil {
					return uuid.Nil, nil, err
				}
				return w.ID, w.ReplacedID, nil
			},
			bulk: func(ctx context.Context, pos uuid.UUID, month time.Time, amt decimal.Decimal, asOf *time.Time) (repo.BulkResult, []repo.BulkSnapshotRowError, error) {
				return ir.BulkUpsertInvestmentAccruedSnapshots(ctx, repo.BulkUpsertInvestmentAccruedSnapshotsParams{
					YearMonth: month, AsOfDate: asOf,
					Rows: []repo.BulkInvestmentAccruedSnapshotRow{{InvestmentID: pos, Amount: amt, AccruedInterest: dp(1), Currency: "IDR"}},
				})
			},
			imp: func(ctx context.Context, pos uuid.UUID, rows []replaceImportRow, dry bool) (*repo.ImportResult, error) {
				return ir.ImportInvestmentSnapshots(ctx, pos, bondRows(rows), dry)
			},
		},
	}
}

// snapRows reads every row of a position's month straight from the table, live
// and archived alike, so a test can tell "archived" from "gone".
type snapRow struct {
	ID         uuid.UUID
	Amount     decimal.Decimal
	Currency   string
	Archived   bool
	Supersedes *uuid.UUID
}

func readMonth(t *testing.T, pool *pgxpool.Pool, g replaceGroup, pos uuid.UUID, month time.Time) []snapRow {
	t.Helper()
	rows, err := pool.Query(context.Background(),
		"SELECT id, amount, currency, deleted_at IS NOT NULL, supersedes FROM "+g.table+
			" WHERE "+g.fkCol+" = $1 AND year_month = $2 ORDER BY created_at, id", pos, month)
	if err != nil {
		t.Fatalf("read %s: %v", g.table, err)
	}
	defer rows.Close()
	var out []snapRow
	for rows.Next() {
		var r snapRow
		if err := rows.Scan(&r.ID, &r.Amount, &r.Currency, &r.Archived, &r.Supersedes); err != nil {
			t.Fatalf("scan: %v", err)
		}
		out = append(out, r)
	}
	return out
}

func rawCount(t *testing.T, pool *pgxpool.Pool, g replaceGroup, pos uuid.UUID, where string) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(),
		"SELECT count(*) FROM "+g.table+" WHERE "+g.fkCol+" = $1 AND "+where, pos).Scan(&n); err != nil {
		t.Fatalf("count %s: %v", g.table, err)
	}
	return n
}

// requireOneLive pins the month's exact state: one live row with the given id
// and amount, `archived` soft-deleted rows, and `supersedes` null on all of them.
func requireOneLive(t *testing.T, pool *pgxpool.Pool, g replaceGroup, pos uuid.UUID, month time.Time, wantID *uuid.UUID, wantAmt string, archived int) snapRow {
	t.Helper()
	rows := readMonth(t, pool, g, pos, month)
	var live []snapRow
	arch := 0
	for _, r := range rows {
		if r.Supersedes != nil {
			t.Errorf("%s: row %s has supersedes=%s; a replace must leave it null", month.Format("2006-01"), r.ID, r.Supersedes)
		}
		if r.Archived {
			arch++
		} else {
			live = append(live, r)
		}
	}
	if len(live) != 1 {
		t.Fatalf("%s: %d live rows, want exactly 1 (%+v)", month.Format("2006-01"), len(live), rows)
	}
	if arch != archived {
		t.Fatalf("%s: %d archived rows, want %d (%+v)", month.Format("2006-01"), arch, archived, rows)
	}
	if !live[0].Amount.Equal(rd(wantAmt)) {
		t.Fatalf("%s: live amount %s, want %s", month.Format("2006-01"), live[0].Amount, wantAmt)
	}
	if wantID != nil && live[0].ID != *wantID {
		t.Fatalf("%s: live id %s, want %s (nothing should have been rewritten)", month.Format("2006-01"), live[0].ID, *wantID)
	}
	return live[0]
}

// covers: INV-SNAPSHOTS-11
func TestSnapshotReplace_Edit_AllGroups(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)
	alice := testutil.CreateHouseholdWithUser(t, q, "Alice")
	bob := testutil.CreateHouseholdWithUser(t, q, "Bob")
	aliceCtx := identity.WithUser(context.Background(), alice)
	bobCtx := identity.WithUser(context.Background(), bob)

	for _, g := range replaceGroups(tdb.Pool) {
		t.Run(g.name, func(t *testing.T) {
			t.Run("edit archives the original and leaves one live row", func(t *testing.T) {
				pos := g.newPosition(t, aliceCtx)
				orig := g.create(t, aliceCtx, pos, replOct, rd("100"))

				live, replaced, err := g.edit(aliceCtx, orig, rd("150"), "IDR", nil)
				if err != nil {
					t.Fatalf("edit: %v", err)
				}
				if live == orig {
					t.Fatalf("the snapshot id did not change on edit (ADR-0058 §1)")
				}
				if replaced == nil || *replaced != orig {
					t.Fatalf("replaced_id = %v, want %s", replaced, orig)
				}
				requireOneLive(t, tdb.Pool, g, pos, replOct, &live, "150", 1)
				// The archived row still holds the value the month had.
				for _, r := range readMonth(t, tdb.Pool, g, pos, replOct) {
					if r.ID == orig && (!r.Archived || !r.Amount.Equal(rd("100"))) {
						t.Errorf("original = %+v, want archived with amount 100", r)
					}
				}
			})

			t.Run("an edit that changes nothing writes nothing", func(t *testing.T) {
				pos := g.newPosition(t, aliceCtx)
				orig := g.create(t, aliceCtx, pos, replOct, rd("1000000.1234"))

				// Same value spelled with a different scale: compared as a
				// decimal, never as a string.
				live, replaced, err := g.edit(aliceCtx, orig, rd("1000000.12340000"), "IDR", nil)
				if err != nil {
					t.Fatalf("edit: %v", err)
				}
				if live != orig || replaced != nil {
					t.Fatalf("identical edit: live=%s replaced=%v, want %s / nil", live, replaced, orig)
				}
				requireOneLive(t, tdb.Pool, g, pos, replOct, &orig, "1000000.1234", 0)
			})

			t.Run("zero, negative and a second currency are real changes", func(t *testing.T) {
				pos := g.newPosition(t, aliceCtx)
				id := g.create(t, aliceCtx, pos, replOct, rd("100"))

				steps := []struct {
					amt string
					cur string
				}{{"0", "IDR"}, {"-5.5", "IDR"}, {"-5.5", "USD"}}
				for i, s := range steps {
					next, replaced, err := g.edit(aliceCtx, id, rd(s.amt), s.cur, nil)
					if err != nil {
						t.Fatalf("step %d edit: %v", i, err)
					}
					if replaced == nil || *replaced != id {
						t.Fatalf("step %d: replaced_id = %v, want %s", i, replaced, id)
					}
					id = next
					row := requireOneLive(t, tdb.Pool, g, pos, replOct, &id, s.amt, i+1)
					if row.Currency != s.cur {
						t.Fatalf("step %d: currency %s, want %s", i, row.Currency, s.cur)
					}
				}
				// Re-saving what it just became is not a change.
				if _, replaced, err := g.edit(aliceCtx, id, rd("-5.5"), "USD", nil); err != nil || replaced != nil {
					t.Fatalf("identical re-save: replaced=%v err=%v", replaced, err)
				}
			})

			t.Run("a failure midway leaves nothing changed", func(t *testing.T) {
				pos := g.newPosition(t, aliceCtx)
				orig := g.create(t, aliceCtx, pos, replOct, rd("100"))

				// Last day of the month is inside the month and succeeds ...
				ok, _, err := g.edit(aliceCtx, orig, rd("110"), "IDR", dayOf(2026, time.October, 31))
				if err != nil {
					t.Fatalf("as_of on the month's last day: %v", err)
				}
				// ... the first of the next month trips the CHECK after the
				// archive step has already run inside the transaction.
				_, _, err = g.edit(aliceCtx, ok, rd("120"), "IDR", dayOf(2026, time.November, 1))
				if !errors.Is(err, repo.ErrSnapshotDateOutsideMonth) {
					t.Fatalf("err = %v, want ErrSnapshotDateOutsideMonth", err)
				}
				requireOneLive(t, tdb.Pool, g, pos, replOct, &ok, "110", 1)
			})

			t.Run("another household cannot edit it", func(t *testing.T) {
				pos := g.newPosition(t, aliceCtx)
				orig := g.create(t, aliceCtx, pos, replOct, rd("100"))

				if _, _, err := g.edit(bobCtx, orig, rd("999"), "IDR", nil); !errors.Is(err, repo.ErrNotFound) {
					t.Fatalf("bob edit err = %v, want ErrNotFound", err)
				}
				requireOneLive(t, tdb.Pool, g, pos, replOct, &orig, "100", 0)
			})

			t.Run("a soft-deleted snapshot cannot be edited", func(t *testing.T) {
				pos := g.newPosition(t, aliceCtx)
				orig := g.create(t, aliceCtx, pos, replOct, rd("100"))
				if _, err := tdb.Pool.Exec(context.Background(),
					"UPDATE "+g.table+" SET deleted_at = now() WHERE id = $1", orig); err != nil {
					t.Fatalf("soft delete: %v", err)
				}
				if _, _, err := g.edit(aliceCtx, orig, rd("999"), "IDR", nil); !errors.Is(err, repo.ErrNotFound) {
					t.Fatalf("edit of a deleted snapshot err = %v, want ErrNotFound", err)
				}
				if n := rawCount(t, tdb.Pool, g, pos, "deleted_at IS NULL"); n != 0 {
					t.Fatalf("%d live rows appeared, want 0", n)
				}
			})
		})
	}
}

// covers: INV-SNAPSHOTS-11
func TestSnapshotReplace_Bulk_AllGroups(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)
	alice := testutil.CreateHouseholdWithUser(t, q, "Alice")
	bob := testutil.CreateHouseholdWithUser(t, q, "Bob")
	aliceCtx := identity.WithUser(context.Background(), alice)
	bobCtx := identity.WithUser(context.Background(), bob)

	for _, g := range replaceGroups(tdb.Pool) {
		t.Run(g.name, func(t *testing.T) {
			t.Run("a bulk save over an existing month archives it, untouched months write nothing", func(t *testing.T) {
				pos := g.newPosition(t, aliceCtx)
				sep := g.create(t, aliceCtx, pos, replSep, rd("50"))
				g.create(t, aliceCtx, pos, replOct, rd("100"))

				res, rowErrs, err := g.bulk(aliceCtx, pos, replOct, rd("150"), nil)
				if err != nil || len(rowErrs) != 0 {
					t.Fatalf("bulk: res=%+v rowErrs=%+v err=%v", res, rowErrs, err)
				}
				if res != (repo.BulkResult{Written: 1, Replaced: 1}) {
					t.Fatalf("bulk result = %+v, want written=1 replaced=1", res)
				}
				requireOneLive(t, tdb.Pool, g, pos, replOct, nil, "150", 1)
				requireOneLive(t, tdb.Pool, g, pos, replSep, &sep, "50", 0)
			})

			t.Run("a new month is written without replacing anything", func(t *testing.T) {
				pos := g.newPosition(t, aliceCtx)
				g.create(t, aliceCtx, pos, replOct, rd("100"))

				res, _, err := g.bulk(aliceCtx, pos, replNov, rd("120"), nil)
				if err != nil {
					t.Fatalf("bulk: %v", err)
				}
				if res != (repo.BulkResult{Written: 1}) {
					t.Fatalf("bulk result = %+v, want written=1 replaced=0", res)
				}
				requireOneLive(t, tdb.Pool, g, pos, replNov, nil, "120", 0)
			})

			t.Run("identical values are not written", func(t *testing.T) {
				pos := g.newPosition(t, aliceCtx)
				orig := g.create(t, aliceCtx, pos, replOct, rd("100"))

				res, _, err := g.bulk(aliceCtx, pos, replOct, rd("100.0000"), nil)
				if err != nil {
					t.Fatalf("bulk: %v", err)
				}
				if res != (repo.BulkResult{Unchanged: 1}) {
					t.Fatalf("bulk result = %+v, want unchanged=1 only", res)
				}
				requireOneLive(t, tdb.Pool, g, pos, replOct, &orig, "100", 0)
			})

			t.Run("a month that only has a soft-deleted row is inserted, not replaced", func(t *testing.T) {
				pos := g.newPosition(t, aliceCtx)
				gone := g.create(t, aliceCtx, pos, replOct, rd("100"))
				if _, err := tdb.Pool.Exec(context.Background(),
					"UPDATE "+g.table+" SET deleted_at = now() WHERE id = $1", gone); err != nil {
					t.Fatalf("soft delete: %v", err)
				}
				res, _, err := g.bulk(aliceCtx, pos, replOct, rd("150"), nil)
				if err != nil {
					t.Fatalf("bulk: %v", err)
				}
				if res != (repo.BulkResult{Written: 1}) {
					t.Fatalf("bulk result = %+v, want written=1 replaced=0", res)
				}
				requireOneLive(t, tdb.Pool, g, pos, replOct, nil, "150", 1)
			})

			t.Run("a failure midway leaves nothing changed", func(t *testing.T) {
				pos := g.newPosition(t, aliceCtx)
				orig := g.create(t, aliceCtx, pos, replOct, rd("100"))

				// The batch's as-of date falls outside the target month: the row
				// is archived first, then its insert trips the CHECK.
				_, _, err := g.bulk(aliceCtx, pos, replOct, rd("150"), dayOf(2026, time.November, 1))
				if !errors.Is(err, repo.ErrSnapshotDateOutsideMonth) {
					t.Fatalf("err = %v, want ErrSnapshotDateOutsideMonth", err)
				}
				requireOneLive(t, tdb.Pool, g, pos, replOct, &orig, "100", 0)
			})

			t.Run("another household's position is rejected and untouched", func(t *testing.T) {
				pos := g.newPosition(t, aliceCtx)
				orig := g.create(t, aliceCtx, pos, replOct, rd("100"))

				res, rowErrs, err := g.bulk(bobCtx, pos, replOct, rd("999"), nil)
				if err != nil || len(rowErrs) != 1 || rowErrs[0].Reason != repo.BulkRowIneligible || res.Written != 0 {
					t.Fatalf("bob bulk: res=%+v rowErrs=%+v err=%v, want one ineligible row", res, rowErrs, err)
				}
				requireOneLive(t, tdb.Pool, g, pos, replOct, &orig, "100", 0)
			})
		})
	}
}

// covers: INV-SNAPSHOTS-11
func TestSnapshotReplace_Import_AllGroups(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)
	alice := testutil.CreateHouseholdWithUser(t, q, "Alice")
	bob := testutil.CreateHouseholdWithUser(t, q, "Bob")
	aliceCtx := identity.WithUser(context.Background(), alice)
	bobCtx := identity.WithUser(context.Background(), bob)

	for _, g := range replaceGroups(tdb.Pool) {
		t.Run(g.name, func(t *testing.T) {
			pos := g.newPosition(t, aliceCtx)
			sep := g.create(t, aliceCtx, pos, replSep, rd("50"))
			g.create(t, aliceCtx, pos, replOct, rd("100"))
			rows := []replaceImportRow{
				{replSep, rd("50"), nil},  // identical
				{replOct, rd("150"), nil}, // changed
				{replNov, rd("120"), nil}, // new
			}
			want := repo.ImportResult{ToInsert: 1, ToUpdate: 1, Unchanged: 1}

			// Dry-run classifies and writes nothing.
			res, err := g.imp(aliceCtx, pos, rows, true)
			if err != nil || *res != want {
				t.Fatalf("dry-run: res=%+v err=%v, want %+v", res, err, want)
			}
			if n := rawCount(t, tdb.Pool, g, pos, "deleted_at IS NOT NULL"); n != 0 {
				t.Fatalf("dry-run archived %d rows", n)
			}

			// A failure midway rolls the whole batch back, including the
			// replacement that had already run.
			bad := []replaceImportRow{
				{replOct, rd("999"), nil},
				{replNov, rd("999"), dayOf(2026, time.December, 1)},
			}
			if _, err := g.imp(aliceCtx, pos, bad, false); !errors.Is(err, repo.ErrSnapshotDateOutsideMonth) {
				t.Fatalf("bad import err = %v, want ErrSnapshotDateOutsideMonth", err)
			}
			requireOneLive(t, tdb.Pool, g, pos, replOct, nil, "100", 0)
			if n := rawCount(t, tdb.Pool, g, pos, "year_month = '2026-11-01'"); n != 0 {
				t.Fatalf("a rolled-back import left %d November rows", n)
			}

			// Commit: replace Oct, insert Nov, leave Sep alone.
			res, err = g.imp(aliceCtx, pos, rows, false)
			if err != nil || *res != want {
				t.Fatalf("commit: res=%+v err=%v, want %+v", res, err, want)
			}
			requireOneLive(t, tdb.Pool, g, pos, replOct, nil, "150", 1)
			requireOneLive(t, tdb.Pool, g, pos, replNov, nil, "120", 0)
			requireOneLive(t, tdb.Pool, g, pos, replSep, &sep, "50", 0)

			// Re-importing identical data creates no archived rows.
			archived := rawCount(t, tdb.Pool, g, pos, "deleted_at IS NOT NULL")
			res, err = g.imp(aliceCtx, pos, rows, false)
			if err != nil || *res != (repo.ImportResult{Unchanged: 3}) {
				t.Fatalf("re-import: res=%+v err=%v, want unchanged=3", res, err)
			}
			if n := rawCount(t, tdb.Pool, g, pos, "deleted_at IS NOT NULL"); n != archived {
				t.Fatalf("re-import archived rows: %d -> %d", archived, n)
			}

			// Another household's import is rejected and writes nothing.
			if _, err := g.imp(bobCtx, pos, rows, false); !errors.Is(err, repo.ErrNotFound) {
				t.Fatalf("bob import err = %v, want ErrNotFound", err)
			}
			if n := rawCount(t, tdb.Pool, g, pos, "deleted_at IS NOT NULL"); n != archived {
				t.Fatalf("bob's import archived rows: %d -> %d", archived, n)
			}
		})
	}
}

// Quantity and price are part of "the same values": a stock edit that keeps the
// total but changes the split is a replace, not a no-op.
//
// covers: INV-SNAPSHOTS-11
func TestSnapshotReplace_Investment_QuantityPriceAreValues(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)
	alice := testutil.CreateHouseholdWithUser(t, q, "Alice")
	ctx := identity.WithUser(context.Background(), alice)
	ir := repo.NewInvestmentRepo(tdb.Pool)

	stock, err := ir.CreateStock(ctx, repo.CreateStockParams{
		DisplayName: "BBCA", OwnershipType: "joint", NativeCurrency: "IDR",
		RiskProfile: "medium", Ticker: "BBCA", Exchange: "IDX",
	})
	if err != nil {
		t.Fatalf("CreateStock: %v", err)
	}
	id := stock.Investment.ID
	q10, p10 := rd("10"), rd("10")
	snap, err := ir.CreateInvestmentSnapshot(ctx, repo.CreateInvestmentSnapshotParams{
		InvestmentID: id, YearMonth: replOct, Amount: rd("100"), Currency: "IDR", Quantity: &q10, PricePerUnit: &p10,
	})
	if err != nil {
		t.Fatalf("CreateInvestmentSnapshot: %v", err)
	}

	q5, p20 := rd("5"), rd("20")
	w, err := ir.UpdateInvestmentSnapshot(ctx, repo.UpdateInvestmentSnapshotParams{
		SnapshotID: snap.ID, Amount: rd("100"), Currency: "IDR", Quantity: &q5, PricePerUnit: &p20,
	})
	if err != nil {
		t.Fatalf("UpdateInvestmentSnapshot: %v", err)
	}
	if w.ReplacedID == nil || *w.ReplacedID != snap.ID {
		t.Fatalf("replaced_id = %v, want %s: 10 x 10 and 5 x 20 are different readings", w.ReplacedID, snap.ID)
	}
	if w.Quantity == nil || !w.Quantity.Equal(q5) || w.PricePerUnit == nil || !w.PricePerUnit.Equal(p20) {
		t.Fatalf("live row = %v x %v, want 5 x 20", w.Quantity, w.PricePerUnit)
	}
}

// The staleness watermark is MAX(updated_at) over every snapshot row, deleted
// ones included, so archiving the old row and inserting the new one still moves
// it: the month's report regenerates on the next read, and an edit that wrote
// nothing leaves the cached report alone.
//
// covers: INV-SNAPSHOTS-11, INV-STALENESS-01
func TestSnapshotReplace_ReportsRegenerate(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)
	alice := testutil.CreateHouseholdWithUser(t, q, "Alice")
	ctx := identity.WithUser(context.Background(), alice)
	ar := repo.NewAssetRepo(tdb.Pool)
	reports := repo.NewMonthlyReportRepo(tdb.Pool)

	acct := createAsset(t, q, alice.HouseholdID, &alice.ID, nil, "joint")
	snap := createAssetSnapshot(t, q, alice.HouseholdID, acct, replOct, "100")

	read := func() db.MonthlyReport {
		t.Helper()
		rows, err := reports.ListReports(ctx)
		if err != nil {
			t.Fatalf("ListReports: %v", err)
		}
		return mustMonth(t, rows, replOct)
	}
	before := read()
	if !before.NwAssets.Equal(rd("100")) {
		t.Fatalf("baseline nw_assets = %s, want 100", before.NwAssets)
	}

	same, err := ar.UpdateAssetSnapshot(ctx, repo.UpdateAssetSnapshotParams{SnapshotID: snap, Amount: rd("100"), Currency: "IDR"})
	if err != nil || same.ReplacedID != nil {
		t.Fatalf("identical edit: err=%v", err)
	}
	if got := read(); !got.GeneratedAt.Time.Equal(before.GeneratedAt.Time) {
		t.Errorf("an edit that wrote nothing regenerated the report: %s -> %s", before.GeneratedAt.Time, got.GeneratedAt.Time)
	}

	if _, err := ar.UpdateAssetSnapshot(ctx, repo.UpdateAssetSnapshotParams{SnapshotID: snap, Amount: rd("150"), Currency: "IDR"}); err != nil {
		t.Fatalf("replace: %v", err)
	}
	after := read()
	if !after.NwAssets.Equal(rd("150")) {
		t.Errorf("nw_assets after replace = %s, want 150 (the archived 100 must not count)", after.NwAssets)
	}
	if !after.GeneratedAt.Time.After(before.GeneratedAt.Time) {
		t.Errorf("report was not regenerated after the replace")
	}
}
