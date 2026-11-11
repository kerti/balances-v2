package repo_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/kerti/balances-v2/backend/internal/db"
	"github.com/kerti/balances-v2/backend/internal/identity"
	"github.com/kerti/balances-v2/backend/internal/repo"
	"github.com/kerti/balances-v2/backend/internal/testutil"
)

// A month whose file row equals the live snapshot is "unchanged" (#829): the
// importer counts it separately in the preview and the commit, and never
// writes, archives or touches it. These tests pin that with raw COUNT(*)s and
// the row ids/timestamps, not the result struct alone, so a classification that
// merely *reports* unchanged while still rewriting would fail.

type rowStamp struct {
	ID        uuid.UUID
	UpdatedAt time.Time
}

// liveStamps returns the live row id + updated_at per month, straight from the table.
func liveStamps(t *testing.T, tdb *testutil.TestDB, g replaceGroup, pos uuid.UUID) map[string]rowStamp {
	t.Helper()
	rows, err := tdb.Pool.Query(context.Background(),
		"SELECT to_char(year_month, 'YYYY-MM'), id, updated_at FROM "+g.table+
			" WHERE "+g.fkCol+" = $1 AND deleted_at IS NULL", pos)
	if err != nil {
		t.Fatalf("live stamps: %v", err)
	}
	defer rows.Close()
	out := map[string]rowStamp{}
	for rows.Next() {
		var ym string
		var s rowStamp
		if err := rows.Scan(&ym, &s.ID, &s.UpdatedAt); err != nil {
			t.Fatalf("scan: %v", err)
		}
		out[ym] = s
	}
	return out
}

// covers: INV-IMPORT-10
func TestImportSnapshots_UnchangedRowsAreNeverWritten_AllGroups(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)
	alice := testutil.CreateHouseholdWithUser(t, q, "Alice")
	aliceCtx := identity.WithUser(context.Background(), alice)

	months := []time.Time{
		time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, time.May, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC),
	}

	for _, g := range replaceGroups(tdb.Pool) {
		t.Run(g.name, func(t *testing.T) {
			pos := g.newPosition(t, aliceCtx)
			for i, m := range months {
				g.create(t, aliceCtx, pos, m, rd("100").Add(decimal.NewFromInt(int64(10*i))))
			}
			before := liveStamps(t, tdb, g, pos)
			totalBefore := rawCount(t, tdb.Pool, g, pos, "true")

			// The file as the export wrote it, but with every amount spelled at
			// a different scale: "100.00" is the stored 100.0000, not a change.
			same := func() []replaceImportRow {
				rows := make([]replaceImportRow, len(months))
				for i, m := range months {
					rows[i] = replaceImportRow{month: m, amt: rd("100.00").Add(decimal.NewFromInt(int64(10 * i)))}
				}
				return rows
			}

			// Re-importing the position's own data: all unchanged, preview and commit.
			for _, dry := range []bool{true, false} {
				res, err := g.imp(aliceCtx, pos, same(), dry)
				if err != nil || *res != (repo.ImportResult{Unchanged: 5}) {
					t.Fatalf("dry=%v: res=%+v err=%v, want unchanged=5 only", dry, res, err)
				}
			}
			if n := rawCount(t, tdb.Pool, g, pos, "true"); n != totalBefore {
				t.Fatalf("rows %d -> %d: an unchanged import wrote something", totalBefore, n)
			}
			if n := rawCount(t, tdb.Pool, g, pos, "deleted_at IS NOT NULL"); n != 0 {
				t.Fatalf("an unchanged import archived %d rows", n)
			}
			for ym, s := range liveStamps(t, tdb, g, pos) {
				if s != before[ym] {
					t.Fatalf("%s: live row %+v, was %+v — an unchanged month was touched", ym, s, before[ym])
				}
			}

			// Change one month: 1 replaced, 4 unchanged, and only that month moves.
			rows := same()
			rows[2].amt = rd("999")
			want := repo.ImportResult{ToUpdate: 1, Unchanged: 4}
			if res, err := g.imp(aliceCtx, pos, rows, true); err != nil || *res != want {
				t.Fatalf("preview: res=%+v err=%v, want %+v", res, err, want)
			}
			if n := rawCount(t, tdb.Pool, g, pos, "true"); n != totalBefore {
				t.Fatalf("the preview wrote rows: %d -> %d", totalBefore, n)
			}
			if res, err := g.imp(aliceCtx, pos, rows, false); err != nil || *res != want {
				t.Fatalf("commit: res=%+v err=%v, want %+v", res, err, want)
			}
			if n := rawCount(t, tdb.Pool, g, pos, "true"); n != totalBefore+1 {
				t.Fatalf("rows %d -> %d, want exactly one new row (the replacement)", totalBefore, n)
			}
			if n := rawCount(t, tdb.Pool, g, pos, "deleted_at IS NOT NULL"); n != 1 {
				t.Fatalf("archived rows = %d, want only the replaced month's old row", n)
			}
			after := liveStamps(t, tdb, g, pos)
			for i, m := range months {
				ym := m.Format("2006-01")
				if i == 2 {
					if after[ym].ID == before[ym].ID {
						t.Fatalf("%s: the changed month kept its old live row", ym)
					}
					continue
				}
				if after[ym] != before[ym] {
					t.Fatalf("%s: unchanged month touched: %+v -> %+v", ym, before[ym], after[ym])
				}
			}
			requireOneLive(t, tdb.Pool, g, pos, months[2], nil, "999", 1)
		})
	}
}

// A month whose only row is archived has no live snapshot to compare with: the
// file row is a new month, not an unchanged or replaced one.
// covers: INV-IMPORT-10
func TestImportSnapshots_ArchivedMonthIsNew_AllGroups(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)
	alice := testutil.CreateHouseholdWithUser(t, q, "Alice")
	aliceCtx := identity.WithUser(context.Background(), alice)

	for _, g := range replaceGroups(tdb.Pool) {
		t.Run(g.name, func(t *testing.T) {
			pos := g.newPosition(t, aliceCtx)
			gone := g.create(t, aliceCtx, pos, replOct, rd("100"))
			if _, err := tdb.Pool.Exec(context.Background(),
				"UPDATE "+g.table+" SET deleted_at = now() WHERE id = $1", gone); err != nil {
				t.Fatalf("archive: %v", err)
			}

			rows := []replaceImportRow{{month: replOct, amt: rd("100")}}
			res, err := g.imp(aliceCtx, pos, rows, false)
			if err != nil || *res != (repo.ImportResult{ToInsert: 1}) {
				t.Fatalf("res=%+v err=%v, want one new month", res, err)
			}
			requireOneLive(t, tdb.Pool, g, pos, replOct, nil, "100", 1)
		})
	}
}

func noteOf(s string) *string { return &s }

func dateOf(d int) *time.Time {
	t := time.Date(2026, time.October, d, 0, 0, 0, 0, time.UTC)
	return &t
}

// Every stored field is part of "equal": currency, statement date and note
// decide it as much as the amount, and an absent note is the same as an empty
// one (the importer never produces "", the Edit dialog never stores it).
// covers: INV-IMPORT-10
func TestImportSnapshots_UnchangedComparesEveryField_Flat(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)
	alice := testutil.CreateHouseholdWithUser(t, q, "Alice")
	aliceCtx := identity.WithUser(context.Background(), alice)
	ar := repo.NewAssetRepo(tdb.Pool)

	acct, err := ar.CreateBankAccount(aliceCtx, repo.CreateBankAccountParams{
		DisplayName: "Checking", OwnershipType: "joint", NativeCurrency: "IDR",
		BankName: "BCA", AccountNumber: "1", AccountType: "savings",
	})
	if err != nil {
		t.Fatalf("CreateBankAccount: %v", err)
	}
	pos := acct.Asset.ID

	seed := func(desc *string, asOf *time.Time) {
		t.Helper()
		if _, err := ar.CreateAssetSnapshot(aliceCtx, repo.CreateAssetSnapshotParams{
			AssetID: pos, YearMonth: replOct, Amount: rd("1500.5"), Currency: "IDR", AsOfDate: asOf, Description: desc,
		}); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	row := func(amt, cur string, desc *string, asOf *time.Time) []repo.ImportSnapshotRow {
		return []repo.ImportSnapshotRow{{YearMonth: replOct, Amount: rd(amt), Currency: cur, AsOfDate: asOf, Description: desc}}
	}
	check := func(label string, rows []repo.ImportSnapshotRow, want repo.ImportResult) {
		t.Helper()
		res, err := ar.ImportAssetSnapshots(aliceCtx, pos, rows, true)
		if err != nil || *res != want {
			t.Fatalf("%s: res=%+v err=%v, want %+v", label, res, err, want)
		}
	}
	unchanged := repo.ImportResult{Unchanged: 1}
	replaced := repo.ImportResult{ToUpdate: 1}

	// A fully populated month.
	seed(noteOf("rent"), dateOf(15))
	check("identical, amount at another scale", row("1500.5000", "IDR", noteOf("rent"), dateOf(15)), unchanged)
	check("different amount", row("1500.51", "IDR", noteOf("rent"), dateOf(15)), replaced)
	check("different currency", row("1500.5", "USD", noteOf("rent"), dateOf(15)), replaced)
	check("different note", row("1500.5", "IDR", noteOf("Rent"), dateOf(15)), replaced)
	check("note removed", row("1500.5", "IDR", nil, dateOf(15)), replaced)
	check("different statement date", row("1500.5", "IDR", noteOf("rent"), dateOf(16)), replaced)
	check("statement date removed", row("1500.5", "IDR", noteOf("rent"), nil), replaced)

	// A bare month: no note, no statement date.
	if _, err := tdb.Pool.Exec(context.Background(),
		"DELETE FROM asset_snapshots WHERE asset_id = $1", pos); err != nil {
		t.Fatalf("clear: %v", err)
	}
	seed(nil, nil)
	check("bare, identical", row("1500.5", "IDR", nil, nil), unchanged)
	check("bare, empty note equals no note", row("1500.5", "IDR", noteOf(""), nil), unchanged)
	check("bare, note added", row("1500.5", "IDR", noteOf("rent"), nil), replaced)
	check("bare, statement date added", row("1500.5", "IDR", nil, dateOf(15)), replaced)
}

// Quantity and price per unit are fields of their own: the same total split
// differently is a replacement, and the same split spelled at another scale is
// not.
// covers: INV-IMPORT-10
func TestImportSnapshots_UnchangedComparesQuantityAndPrice(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)
	alice := testutil.CreateHouseholdWithUser(t, q, "Alice")
	aliceCtx := identity.WithUser(context.Background(), alice)
	ir := repo.NewInvestmentRepo(tdb.Pool)

	stock, err := ir.CreateStock(aliceCtx, repo.CreateStockParams{
		DisplayName: "BBCA", OwnershipType: "joint", NativeCurrency: "IDR",
		RiskProfile: "medium", Ticker: "BBCA", Exchange: "IDX",
	})
	if err != nil {
		t.Fatalf("CreateStock: %v", err)
	}
	pos := stock.Investment.ID
	q10, p10 := rd("10.5"), rd("1000")
	if _, err := ir.CreateInvestmentSnapshot(aliceCtx, repo.CreateInvestmentSnapshotParams{
		InvestmentID: pos, YearMonth: replOct, Amount: rd("10500"), Currency: "IDR",
		Quantity: &q10, PricePerUnit: &p10, Description: noteOf("lot A"),
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	row := func(amt, qty, price string, desc *string) []repo.ImportInvestmentSnapshotRow {
		qd, pd := rd(qty), rd(price)
		return []repo.ImportInvestmentSnapshotRow{{
			YearMonth: replOct, Amount: rd(amt), Currency: "IDR", Quantity: &qd, PricePerUnit: &pd, Description: desc,
		}}
	}
	check := func(label string, rows []repo.ImportInvestmentSnapshotRow, want repo.ImportResult) {
		t.Helper()
		res, err := ir.ImportInvestmentSnapshots(aliceCtx, pos, rows, true)
		if err != nil || *res != want {
			t.Fatalf("%s: res=%+v err=%v, want %+v", label, res, err, want)
		}
	}

	check("same split at another scale", row("10500.00", "10.50000000", "1000.00", noteOf("lot A")), repo.ImportResult{Unchanged: 1})
	check("same total, different split", row("10500", "21", "500", noteOf("lot A")), repo.ImportResult{ToUpdate: 1})
	check("same quantity, different price", row("10501.5", "10.5", "1000.1428", noteOf("lot A")), repo.ImportResult{ToUpdate: 1})
	check("different note", row("10500", "10.5", "1000", nil), repo.ImportResult{ToUpdate: 1})
}
