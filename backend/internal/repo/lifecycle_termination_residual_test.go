package repo_test

import (
	"context"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/kerti/balances-v2/backend/internal/db"
	"github.com/kerti/balances-v2/backend/internal/identity"
	"github.com/kerti/balances-v2/backend/internal/repo"
	"github.com/kerti/balances-v2/backend/internal/testutil"
)

// TestPositionLifecycle_CashSettledTerminationReconciles is the read-side proof
// of the timing defect in #576, end to end: repo write → engine → report row.
//
// `terminatedBefore` is `idx > monthIndex(terminated_at)`, so a terminated
// position contributes through its termination month at its last carried value
// and drops out at month+1. The cash leg, however, lands in the month the bank
// actually moved. Without a 0-value close snapshot the two legs sit in different
// months and the derived-Living-Expenses residual is wrong in both, by equal and
// opposite amounts — for a liability paid off at the end of month M:
//
//	M    : bank −20, liability still 20   ⇒ ΔNW = −20  ⇒ residual overstated by 20
//	M+1  : liability suppressed           ⇒ ΔNW = +20  ⇒ residual understated by 20
//
// The fixture settles one Asset, one Liability and one Receivable in three
// consecutive months against a single bank account that absorbs every cash leg,
// so net worth is flat across the whole timeline. A close snapshot missing in any
// group breaks the residual in that group's termination month *and* the month
// after — which is what the per-month assertion below covers.
//
// Every position is a cash subtype (no property/vehicle), so asset-value-change
// stays 0 throughout: the interaction between a close snapshot and the
// asset-value-change loop is ADR-0052 §3, and #586's problem.
//
// covers: INV-FINANCE-06, INV-FINANCE-05, INV-LIFECYCLE-03
func TestPositionLifecycle_CashSettledTerminationReconciles(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)

	alice := testutil.CreateHouseholdWithUser(t, q, "Alice")
	ctx := identity.WithUser(context.Background(), alice)

	ar := repo.NewAssetRepo(tdb.Pool)
	lr := repo.NewLiabilityRepo(tdb.Pool)
	rr := repo.NewReceivableRepo(tdb.Pool)

	jan := ymUTC(2026, time.January)
	feb := ymUTC(2026, time.February)
	mar := ymUTC(2026, time.March)
	apr := ymUTC(2026, time.April)
	may := ymUTC(2026, time.May)

	rows := func(month time.Time, amount int64) []repo.ImportSnapshotRow {
		return []repo.ImportSnapshotRow{{
			YearMonth: month, Amount: decimal.NewFromInt(amount), Currency: "IDR",
		}}
	}

	// The cash plug: every settlement below moves money in or out of this one
	// account, so ΔNW is 0 in every month and the residual must be too.
	cash, err := ar.CreateBankAccount(ctx, repo.CreateBankAccountParams{
		DisplayName: "Everyday", OwnershipType: "joint", NativeCurrency: "IDR",
		BankName: "Bank", AccountNumber: "111", AccountType: "savings",
	})
	if err != nil {
		t.Fatalf("CreateBankAccount (cash): %v", err)
	}
	cashID := cash.Asset.ID

	// A second account, closed in February — its 200 moves to the cash account.
	closing, err := ar.CreateBankAccount(ctx, repo.CreateBankAccountParams{
		DisplayName: "Old Savings", OwnershipType: "joint", NativeCurrency: "IDR",
		BankName: "Bank", AccountNumber: "222", AccountType: "savings",
	})
	if err != nil {
		t.Fatalf("CreateBankAccount (closing): %v", err)
	}
	closingID := closing.Asset.ID

	liab, err := lr.CreateLiability(ctx, repo.CreateLiabilityParams{
		DisplayName: "Car Loan", Subtype: "personal", OwnershipType: "joint",
		NativeCurrency: "IDR", CounterpartyName: "Bank",
	})
	if err != nil {
		t.Fatalf("CreateLiability: %v", err)
	}
	rec, err := rr.CreateReceivable(ctx, repo.CreateReceivableParams{
		DisplayName: "Loan to Carol", OwnershipType: "joint",
		NativeCurrency: "IDR", CounterpartyName: "Carol",
	})
	if err != nil {
		t.Fatalf("CreateReceivable: %v", err)
	}

	// Jan baseline: cash 1000 + old savings 200 + receivable 60 − loan 100 = 1160.
	// Feb: old savings closed, its 200 landing in cash.
	// Mar: the 100 loan paid off out of cash.
	// Apr: the 60 receivable collected into cash.
	// Net worth is 1160 in every month.
	for _, s := range []struct {
		name  string
		apply func() error
	}{
		{"cash", func() error {
			_, err := ar.ImportAssetSnapshots(ctx, cashID, []repo.ImportSnapshotRow{
				{YearMonth: jan, Amount: decimal.NewFromInt(1000), Currency: "IDR"},
				{YearMonth: feb, Amount: decimal.NewFromInt(1200), Currency: "IDR"},
				{YearMonth: mar, Amount: decimal.NewFromInt(1100), Currency: "IDR"},
				{YearMonth: apr, Amount: decimal.NewFromInt(1160), Currency: "IDR"},
			}, false)
			return err
		}},
		{"old savings", func() error {
			_, err := ar.ImportAssetSnapshots(ctx, closingID, rows(jan, 200), false)
			return err
		}},
		{"loan", func() error {
			_, err := lr.ImportLiabilitySnapshots(ctx, liab.ID, []repo.ImportSnapshotRow{
				{YearMonth: jan, Amount: decimal.NewFromInt(100), Currency: "IDR"},
				{YearMonth: feb, Amount: decimal.NewFromInt(100), Currency: "IDR"},
			}, false)
			return err
		}},
		{"receivable", func() error {
			_, err := rr.ImportReceivableSnapshots(ctx, rec.ID, []repo.ImportSnapshotRow{
				{YearMonth: jan, Amount: decimal.NewFromInt(60), Currency: "IDR"},
				{YearMonth: feb, Amount: decimal.NewFromInt(60), Currency: "IDR"},
				{YearMonth: mar, Amount: decimal.NewFromInt(60), Currency: "IDR"},
			}, false)
			return err
		}},
	} {
		if err := s.apply(); err != nil {
			t.Fatalf("seed %s snapshots: %v", s.name, err)
		}
	}

	day := func(m time.Month, d int) time.Time {
		return time.Date(2026, m, d, 0, 0, 0, 0, time.UTC)
	}
	febClose, marPaid, aprCollected := day(time.February, 20), day(time.March, 15), day(time.April, 10)

	if _, err := ar.UpdateAssetLifecycle(ctx, closingID, repo.LifecycleParams{
		Status: "closed", TerminatedAt: &febClose,
	}); err != nil {
		t.Fatalf("close old savings: %v", err)
	}
	if _, err := lr.UpdateLiabilityLifecycle(ctx, liab.ID, repo.LifecycleParams{
		Status: "paid_off", TerminatedAt: &marPaid,
	}); err != nil {
		t.Fatalf("pay off loan: %v", err)
	}
	if _, err := rr.UpdateReceivableLifecycle(ctx, rec.ID, repo.LifecycleParams{
		Status: "collected", TerminatedAt: &aprCollected,
	}); err != nil {
		t.Fatalf("collect receivable: %v", err)
	}

	reports, err := repo.NewMonthlyReportRepo(tdb.Pool).ListReports(ctx)
	if err != nil {
		t.Fatalf("ListReports: %v", err)
	}

	// Jan is the baseline — derived lines are nil there by design. Feb..May covers
	// each termination month and the month after it.
	for _, month := range []time.Time{feb, mar, apr, may} {
		row := mustMonth(t, reports, month)
		label := month.Format("2006-01")

		if !row.NwTotal.Equal(decimal.NewFromInt(1160)) {
			t.Errorf("%s net worth: got %s, want 1160", label, row.NwTotal)
		}
		if row.AssetValueChange == nil || !row.AssetValueChange.IsZero() {
			t.Errorf("%s asset value change: got %v, want 0 (no property/vehicle)", label, row.AssetValueChange)
		}
		if row.DerivedLivingExpenses == nil {
			t.Fatalf("%s derived living expenses is nil; want 0", label)
		}
		if !row.DerivedLivingExpenses.IsZero() {
			t.Errorf("%s derived living expenses: got %s, want 0 — a settlement's two legs "+
				"landed in different months", label, row.DerivedLivingExpenses)
		}
	}
}
