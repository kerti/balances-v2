package repo_test

import (
	"context"
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

// snapView is the group-agnostic slice of a snapshot the assertions below read.
type snapView struct {
	id     uuid.UUID
	month  time.Time
	amount decimal.Decimal
}

// closeSnapshotGroup adapts one position group onto the shared scenario. The
// four differ only in which repo methods name the group and, for Investment, in
// the quantity/price shape its snapshots must carry.
type closeSnapshotGroup struct {
	name      string
	terminal  string // a cash-settled terminal status for this group
	table     string
	fkCol     string
	create    func(t *testing.T) uuid.UUID
	addSnap   func(t *testing.T, posID uuid.UUID, month time.Time, amount int64)
	list      func(t *testing.T, posID uuid.UUID) []snapView
	lifecycle func(posID uuid.UUID, p repo.LifecycleParams) error
}

// TestPositionLifecycle_CloseSnapshotAllGroups is the write-side guarantee
// ADR-0052 §1–2 generalised from Investment to every group: a terminal flip
// writes a truthful 0-value close snapshot at the termination month, displacing
// (never overwriting) whatever the user recorded there, and the un-terminate
// correction hands the displaced row back — unless the user recorded a fresh
// value at that month while the position was terminated, in which case theirs
// wins.
//
// Asset/Liability/Receivable wrote no close snapshot at all before this, which
// is the timing defect in #576: the cash leg landed in the termination month and
// the position's drop in the month after. Investment had the snapshot but
// overwrote the user's row in place (#25), so un-terminate left the month empty
// rather than restoring a value.
//
// covers: INV-LIFECYCLE-03, INV-LIFECYCLE-04
func TestPositionLifecycle_CloseSnapshotAllGroups(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)

	alice := testutil.CreateHouseholdWithUser(t, q, "Alice")
	ctx := identity.WithUser(context.Background(), alice)

	ar := repo.NewAssetRepo(tdb.Pool)
	lr := repo.NewLiabilityRepo(tdb.Pool)
	rr := repo.NewReceivableRepo(tdb.Pool)
	ir := repo.NewInvestmentRepo(tdb.Pool)

	imp := func(t *testing.T, month time.Time, amount int64) []repo.ImportSnapshotRow {
		t.Helper()
		return []repo.ImportSnapshotRow{{
			YearMonth: month, Amount: decimal.NewFromInt(amount), Currency: "IDR",
		}}
	}

	groups := []closeSnapshotGroup{
		{
			name:     "asset",
			terminal: "closed",
			table:    "asset_snapshots",
			fkCol:    "asset_id",
			create: func(t *testing.T) uuid.UUID {
				t.Helper()
				acct, err := ar.CreateBankAccount(ctx, repo.CreateBankAccountParams{
					DisplayName: "Everyday", OwnershipType: "joint", NativeCurrency: "IDR",
					BankName: "Bank", AccountNumber: "111", AccountType: "savings",
				})
				if err != nil {
					t.Fatalf("CreateBankAccount: %v", err)
				}
				return acct.Asset.ID
			},
			addSnap: func(t *testing.T, posID uuid.UUID, month time.Time, amount int64) {
				t.Helper()
				if _, err := ar.ImportAssetSnapshots(ctx, posID, imp(t, month, amount), false); err != nil {
					t.Fatalf("ImportAssetSnapshots: %v", err)
				}
			},
			list: func(t *testing.T, posID uuid.UUID) []snapView {
				t.Helper()
				rows, err := ar.ListAssetSnapshots(ctx, posID)
				if err != nil {
					t.Fatalf("ListAssetSnapshots: %v", err)
				}
				out := make([]snapView, 0, len(rows))
				for _, s := range rows {
					out = append(out, snapView{id: s.ID, month: s.YearMonth, amount: s.Amount})
				}
				return out
			},
			lifecycle: func(posID uuid.UUID, p repo.LifecycleParams) error {
				_, err := ar.UpdateAssetLifecycle(ctx, posID, p)
				return err
			},
		},
		{
			name:     "liability",
			terminal: "paid_off",
			table:    "liability_snapshots",
			fkCol:    "liability_id",
			create: func(t *testing.T) uuid.UUID {
				t.Helper()
				liab, err := lr.CreateLiability(ctx, repo.CreateLiabilityParams{
					DisplayName: "Car Loan", Subtype: "personal", OwnershipType: "joint",
					NativeCurrency: "IDR", CounterpartyName: "Bank",
				})
				if err != nil {
					t.Fatalf("CreateLiability: %v", err)
				}
				return liab.ID
			},
			addSnap: func(t *testing.T, posID uuid.UUID, month time.Time, amount int64) {
				t.Helper()
				if _, err := lr.ImportLiabilitySnapshots(ctx, posID, imp(t, month, amount), false); err != nil {
					t.Fatalf("ImportLiabilitySnapshots: %v", err)
				}
			},
			list: func(t *testing.T, posID uuid.UUID) []snapView {
				t.Helper()
				rows, err := lr.ListLiabilitySnapshots(ctx, posID)
				if err != nil {
					t.Fatalf("ListLiabilitySnapshots: %v", err)
				}
				out := make([]snapView, 0, len(rows))
				for _, s := range rows {
					out = append(out, snapView{id: s.ID, month: s.YearMonth, amount: s.Amount})
				}
				return out
			},
			lifecycle: func(posID uuid.UUID, p repo.LifecycleParams) error {
				_, err := lr.UpdateLiabilityLifecycle(ctx, posID, p)
				return err
			},
		},
		{
			name:     "receivable",
			terminal: "collected",
			table:    "receivable_snapshots",
			fkCol:    "receivable_id",
			create: func(t *testing.T) uuid.UUID {
				t.Helper()
				rec, err := rr.CreateReceivable(ctx, repo.CreateReceivableParams{
					DisplayName: "Loan to Carol", OwnershipType: "joint",
					NativeCurrency: "IDR", CounterpartyName: "Carol",
				})
				if err != nil {
					t.Fatalf("CreateReceivable: %v", err)
				}
				return rec.ID
			},
			addSnap: func(t *testing.T, posID uuid.UUID, month time.Time, amount int64) {
				t.Helper()
				if _, err := rr.ImportReceivableSnapshots(ctx, posID, imp(t, month, amount), false); err != nil {
					t.Fatalf("ImportReceivableSnapshots: %v", err)
				}
			},
			list: func(t *testing.T, posID uuid.UUID) []snapView {
				t.Helper()
				rows, err := rr.ListReceivableSnapshots(ctx, posID)
				if err != nil {
					t.Fatalf("ListReceivableSnapshots: %v", err)
				}
				out := make([]snapView, 0, len(rows))
				for _, s := range rows {
					out = append(out, snapView{id: s.ID, month: s.YearMonth, amount: s.Amount})
				}
				return out
			},
			lifecycle: func(posID uuid.UUID, p repo.LifecycleParams) error {
				_, err := rr.UpdateReceivableLifecycle(ctx, posID, p)
				return err
			},
		},
		{
			name:     "investment",
			terminal: "sold",
			table:    "investment_snapshots",
			fkCol:    "investment_id",
			create: func(t *testing.T) uuid.UUID {
				t.Helper()
				stock, err := ir.CreateStock(ctx, repo.CreateStockParams{
					DisplayName: "BBCA", OwnershipType: "joint", NativeCurrency: "IDR",
					Ticker: "BBCA", Exchange: "IDX", RiskProfile: "medium",
				})
				if err != nil {
					t.Fatalf("CreateStock: %v", err)
				}
				return stock.Investment.ID
			},
			addSnap: func(t *testing.T, posID uuid.UUID, month time.Time, amount int64) {
				t.Helper()
				qty := decimal.NewFromInt(1)
				price := decimal.NewFromInt(amount)
				if _, err := ir.ImportInvestmentSnapshots(ctx, posID, []repo.ImportInvestmentSnapshotRow{{
					YearMonth: month, Amount: decimal.NewFromInt(amount), Currency: "IDR",
					Quantity: &qty, PricePerUnit: &price,
				}}, false); err != nil {
					t.Fatalf("ImportInvestmentSnapshots: %v", err)
				}
			},
			list: func(t *testing.T, posID uuid.UUID) []snapView {
				t.Helper()
				rows, err := ir.ListInvestmentSnapshots(ctx, posID)
				if err != nil {
					t.Fatalf("ListInvestmentSnapshots: %v", err)
				}
				out := make([]snapView, 0, len(rows))
				for _, s := range rows {
					out = append(out, snapView{id: s.ID, month: s.YearMonth, amount: s.Amount})
				}
				return out
			},
			lifecycle: func(posID uuid.UUID, p repo.LifecycleParams) error {
				_, err := ir.UpdateInvestmentLifecycle(ctx, posID, p, nil)
				return err
			},
		},
	}

	jan, mar := ymUTC(2026, time.January), ymUTC(2026, time.March)
	termDate := time.Date(2026, time.March, 15, 0, 0, 0, 0, time.UTC)
	note := "settled in full"

	for _, g := range groups {
		t.Run(g.name, func(t *testing.T) {
			posID := g.create(t)
			g.addSnap(t, posID, jan, 20)
			g.addSnap(t, posID, mar, 25)

			// ----- terminate: the March 25 is archived, a 0 close takes its place
			if err := g.lifecycle(posID, repo.LifecycleParams{
				Status: g.terminal, TerminatedAt: &termDate, TerminationNote: &note,
			}); err != nil {
				t.Fatalf("terminate: %v", err)
			}
			assertSnapshots(t, g.list(t, posID), map[time.Time]int64{jan: 20, mar: 0})
			if n := countArchivedSnapshots(t, tdb.Pool, g.table, g.fkCol, posID); n != 1 {
				t.Fatalf("archived rows after terminate: got %d, want 1 (the displaced March 25)", n)
			}

			// ----- re-asserting the same terminal flip refreshes the close row in
			// place instead of displacing it. If it displaced, the archived March 25
			// would lose the created_at pairing that un-terminate restores it by, and
			// repeated flips would pile up tombstones.
			if err := g.lifecycle(posID, repo.LifecycleParams{
				Status: g.terminal, TerminatedAt: &termDate, TerminationNote: &note,
			}); err != nil {
				t.Fatalf("re-assert terminal flip: %v", err)
			}
			assertSnapshots(t, g.list(t, posID), map[time.Time]int64{jan: 20, mar: 0})
			if n := countArchivedSnapshots(t, tdb.Pool, g.table, g.fkCol, posID); n != 1 {
				t.Fatalf("archived rows after re-asserting the flip: got %d, want 1", n)
			}

			// ----- un-terminate: the archived March 25 comes back, not 0 and not a hole
			if err := g.lifecycle(posID, repo.LifecycleParams{Status: repo.StatusActive}); err != nil {
				t.Fatalf("un-terminate: %v", err)
			}
			assertSnapshots(t, g.list(t, posID), map[time.Time]int64{jan: 20, mar: 25})
			if n := countArchivedSnapshots(t, tdb.Pool, g.table, g.fkCol, posID); n != 1 {
				t.Fatalf("archived rows after un-terminate: got %d, want 1 (the dropped 0 close)", n)
			}

			// ----- collision: a value recorded while terminated wins over the archive.
			// Snapshots, unlike investment transactions, are not blocked on a
			// terminated position, so this is a reachable state — the import path
			// upserts straight over the live close row.
			if err := g.lifecycle(posID, repo.LifecycleParams{
				Status: g.terminal, TerminatedAt: &termDate, TerminationNote: &note,
			}); err != nil {
				t.Fatalf("re-terminate: %v", err)
			}
			g.addSnap(t, posID, mar, 30)
			if err := g.lifecycle(posID, repo.LifecycleParams{Status: repo.StatusActive}); err != nil {
				t.Fatalf("un-terminate after collision: %v", err)
			}
			assertSnapshots(t, g.list(t, posID), map[time.Time]int64{jan: 20, mar: 30})
			// Two archived rows: the first cycle's 0 close, and the March 25 the
			// second cycle displaced — which stays archived, since the user's 30
			// occupies the month.
			if n := countArchivedSnapshots(t, tdb.Pool, g.table, g.fkCol, posID); n != 2 {
				t.Fatalf("archived rows after collision: got %d, want 2", n)
			}
		})
	}
}

// assertSnapshots pins the exact set of live snapshots: month → amount.
func assertSnapshots(t *testing.T, got []snapView, want map[time.Time]int64) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("live snapshots: got %d, want %d (%v)", len(got), len(want), describeSnaps(got))
	}
	for _, s := range got {
		w, ok := want[s.month]
		if !ok {
			t.Fatalf("unexpected live snapshot at %s (%v)", s.month.Format("2006-01"), describeSnaps(got))
		}
		if !s.amount.Equal(decimal.NewFromInt(w)) {
			t.Errorf("live snapshot at %s: got %s, want %d", s.month.Format("2006-01"), s.amount, w)
		}
	}
}

func describeSnaps(snaps []snapView) []string {
	out := make([]string, 0, len(snaps))
	for _, s := range snaps {
		out = append(out, s.month.Format("2006-01")+"="+s.amount.String())
	}
	return out
}

// countArchivedSnapshots reads the tombstoned rows straight from the table.
// Every repo list path filters `deleted_at IS NULL`, so displacement is only
// observable in raw SQL — and "the archived row still exists" is exactly the
// guarantee that makes un-terminate non-destructive.
func countArchivedSnapshots(t *testing.T, pool *pgxpool.Pool, table, fkCol string, posID uuid.UUID) int {
	t.Helper()
	var n int
	err := pool.QueryRow(context.Background(),
		"SELECT count(*) FROM "+table+" WHERE "+fkCol+" = $1 AND deleted_at IS NOT NULL", posID).Scan(&n)
	if err != nil {
		t.Fatalf("count archived %s: %v", table, err)
	}
	return n
}
