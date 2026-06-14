package repo_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/kerti/balances-v2/backend/internal/auth"
	"github.com/kerti/balances-v2/backend/internal/db"
	"github.com/kerti/balances-v2/backend/internal/repo"
	"github.com/kerti/balances-v2/backend/internal/testutil"
)

// TestPositionLifecycle_UpdateAndValidation covers the M4.6 dedicated terminate
// action across all four position groups: the happy-path flip persists status +
// terminated_at + note, the shared validatePositionLifecycle rejects the three
// invalid shapes (unknown status, terminal-without-date, active-with-date) plus
// cross-group statuses, the correction path back to active clears the date, and
// bob cannot terminate alice's position.
// covers: INV-TENANCY-08
func TestPositionLifecycle_UpdateAndValidation(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)

	aliceUser := testutil.CreateHouseholdWithUser(t, q, "Alice")
	bobUser := testutil.CreateHouseholdWithUser(t, q, "Bob")
	if aliceUser.HouseholdID == bobUser.HouseholdID {
		t.Fatalf("fixture: alice and bob ended up in the same household")
	}
	aliceCtx := auth.WithUser(context.Background(), aliceUser)
	bobCtx := auth.WithUser(context.Background(), bobUser)

	assetRepo := repo.NewAssetRepo(tdb.Pool)
	liabRepo := repo.NewLiabilityRepo(tdb.Pool)
	recvRepo := repo.NewReceivableRepo(tdb.Pool)
	invRepo := repo.NewInvestmentRepo(tdb.Pool)

	account, err := assetRepo.CreateBankAccount(aliceCtx, repo.CreateBankAccountParams{
		DisplayName: "Alice BCA", OwnershipType: "joint", NativeCurrency: "IDR",
		BankName: "BCA", AccountNumber: "111", AccountType: "savings",
	})
	if err != nil {
		t.Fatalf("CreateBankAccount: %v", err)
	}
	liability, err := liabRepo.CreateLiability(aliceCtx, repo.CreateLiabilityParams{
		DisplayName: "Alice KPR", Subtype: "institutional", OwnershipType: "joint",
		NativeCurrency: "IDR", CounterpartyName: "Bank BCA",
	})
	if err != nil {
		t.Fatalf("CreateLiability: %v", err)
	}
	receivable, err := recvRepo.CreateReceivable(aliceCtx, repo.CreateReceivableParams{
		DisplayName: "Loan to friend", OwnershipType: "joint",
		NativeCurrency: "IDR", CounterpartyName: "Carol",
	})
	if err != nil {
		t.Fatalf("CreateReceivable: %v", err)
	}
	stock, err := invRepo.CreateStock(aliceCtx, repo.CreateStockParams{
		DisplayName: "BBCA", OwnershipType: "joint", NativeCurrency: "IDR",
		Ticker: "BBCA", Exchange: "IDX",
		RiskProfile: "medium",
	})
	if err != nil {
		t.Fatalf("CreateStock: %v", err)
	}

	termDate := time.Date(2026, time.May, 25, 0, 0, 0, 0, time.UTC)
	note := "closed per statement"

	t.Run("alice terminates bank account (sold)", func(t *testing.T) {
		row, err := assetRepo.UpdateAssetLifecycle(aliceCtx, account.Asset.ID, repo.LifecycleParams{
			Status: "sold", TerminatedAt: &termDate, TerminationNote: &note,
		})
		if err != nil {
			t.Fatalf("UpdateAssetLifecycle: %v", err)
		}
		if row.Status != "sold" || row.TerminatedAt == nil || !row.TerminatedAt.Equal(termDate) {
			t.Fatalf("status/date not persisted: %+v", row)
		}
		got, err := assetRepo.GetBankAccount(aliceCtx, account.Asset.ID)
		if err != nil {
			t.Fatalf("GetBankAccount: %v", err)
		}
		if got.Asset.Status != "sold" || got.Asset.TerminationNote == nil || *got.Asset.TerminationNote != note {
			t.Fatalf("Get reflects wrong lifecycle: %+v", got.Asset)
		}
	})

	t.Run("alice terminates liability (paid_off)", func(t *testing.T) {
		row, err := liabRepo.UpdateLiabilityLifecycle(aliceCtx, liability.ID, repo.LifecycleParams{
			Status: "paid_off", TerminatedAt: &termDate,
		})
		if err != nil {
			t.Fatalf("UpdateLiabilityLifecycle: %v", err)
		}
		if row.Status != "paid_off" || row.TerminatedAt == nil {
			t.Fatalf("status/date not persisted: %+v", row)
		}
	})

	t.Run("alice terminates receivable (collected)", func(t *testing.T) {
		row, err := recvRepo.UpdateReceivableLifecycle(aliceCtx, receivable.ID, repo.LifecycleParams{
			Status: "collected", TerminatedAt: &termDate,
		})
		if err != nil {
			t.Fatalf("UpdateReceivableLifecycle: %v", err)
		}
		if row.Status != "collected" {
			t.Fatalf("status not persisted: %+v", row)
		}
	})

	t.Run("alice terminates stock (sold)", func(t *testing.T) {
		row, err := invRepo.UpdateInvestmentLifecycle(aliceCtx, stock.Investment.ID, repo.LifecycleParams{
			Status: "sold", TerminatedAt: &termDate,
		})
		if err != nil {
			t.Fatalf("UpdateInvestmentLifecycle: %v", err)
		}
		if row.Status != "sold" {
			t.Fatalf("status not persisted: %+v", row)
		}
	})

	t.Run("terminal status without date is rejected", func(t *testing.T) {
		_, err := liabRepo.UpdateLiabilityLifecycle(aliceCtx, liability.ID, repo.LifecycleParams{
			Status: "forgiven", TerminatedAt: nil,
		})
		if !errors.Is(err, repo.ErrInvalidLifecycle) {
			t.Fatalf("want ErrInvalidLifecycle, got %v", err)
		}
	})

	t.Run("active status with date is rejected", func(t *testing.T) {
		_, err := assetRepo.UpdateAssetLifecycle(aliceCtx, account.Asset.ID, repo.LifecycleParams{
			Status: "active", TerminatedAt: &termDate,
		})
		if !errors.Is(err, repo.ErrInvalidLifecycle) {
			t.Fatalf("want ErrInvalidLifecycle, got %v", err)
		}
	})

	t.Run("unknown status is rejected", func(t *testing.T) {
		_, err := assetRepo.UpdateAssetLifecycle(aliceCtx, account.Asset.ID, repo.LifecycleParams{
			Status: "frozen", TerminatedAt: &termDate,
		})
		if !errors.Is(err, repo.ErrInvalidLifecycle) {
			t.Fatalf("want ErrInvalidLifecycle, got %v", err)
		}
	})

	t.Run("cross-group status is rejected (asset cannot be paid_off)", func(t *testing.T) {
		_, err := assetRepo.UpdateAssetLifecycle(aliceCtx, account.Asset.ID, repo.LifecycleParams{
			Status: "paid_off", TerminatedAt: &termDate,
		})
		if !errors.Is(err, repo.ErrInvalidLifecycle) {
			t.Fatalf("want ErrInvalidLifecycle, got %v", err)
		}
	})

	t.Run("correction back to active clears terminated_at", func(t *testing.T) {
		row, err := assetRepo.UpdateAssetLifecycle(aliceCtx, account.Asset.ID, repo.LifecycleParams{
			Status: "active", TerminatedAt: nil, TerminationNote: nil,
		})
		if err != nil {
			t.Fatalf("UpdateAssetLifecycle back to active: %v", err)
		}
		if row.Status != "active" || row.TerminatedAt != nil {
			t.Fatalf("active row still carries a date: %+v", row)
		}
	})

	t.Run("bob cannot terminate alice's asset", func(t *testing.T) {
		_, err := assetRepo.UpdateAssetLifecycle(bobCtx, account.Asset.ID, repo.LifecycleParams{
			Status: "sold", TerminatedAt: &termDate,
		})
		if !errors.Is(err, repo.ErrNotFound) {
			t.Fatalf("want ErrNotFound, got %v", err)
		}
	})

	// The asset path above covers every branch of the shared helper, but coverage
	// is per-function: each Update*Lifecycle has its own validate-error return and
	// its own ErrNoRows→ErrNotFound mapping. Exercise both on the other three
	// groups so those branches aren't left to the asset stand-in.
	t.Run("bob cannot terminate alice's liability", func(t *testing.T) {
		_, err := liabRepo.UpdateLiabilityLifecycle(bobCtx, liability.ID, repo.LifecycleParams{
			Status: "paid_off", TerminatedAt: &termDate,
		})
		if !errors.Is(err, repo.ErrNotFound) {
			t.Fatalf("want ErrNotFound, got %v", err)
		}
	})

	t.Run("receivable rejects cross-group status", func(t *testing.T) {
		_, err := recvRepo.UpdateReceivableLifecycle(aliceCtx, receivable.ID, repo.LifecycleParams{
			Status: "sold", TerminatedAt: &termDate, // sold is an asset/investment status
		})
		if !errors.Is(err, repo.ErrInvalidLifecycle) {
			t.Fatalf("want ErrInvalidLifecycle, got %v", err)
		}
	})

	t.Run("bob cannot terminate alice's receivable", func(t *testing.T) {
		_, err := recvRepo.UpdateReceivableLifecycle(bobCtx, receivable.ID, repo.LifecycleParams{
			Status: "collected", TerminatedAt: &termDate,
		})
		if !errors.Is(err, repo.ErrNotFound) {
			t.Fatalf("want ErrNotFound, got %v", err)
		}
	})

	t.Run("investment rejects cross-group status", func(t *testing.T) {
		_, err := invRepo.UpdateInvestmentLifecycle(aliceCtx, stock.Investment.ID, repo.LifecycleParams{
			Status: "paid_off", TerminatedAt: &termDate, // paid_off is a liability status
		})
		if !errors.Is(err, repo.ErrInvalidLifecycle) {
			t.Fatalf("want ErrInvalidLifecycle, got %v", err)
		}
	})

	t.Run("bob cannot terminate alice's stock", func(t *testing.T) {
		_, err := invRepo.UpdateInvestmentLifecycle(bobCtx, stock.Investment.ID, repo.LifecycleParams{
			Status: "sold", TerminatedAt: &termDate,
		})
		if !errors.Is(err, repo.ErrNotFound) {
			t.Fatalf("want ErrNotFound, got %v", err)
		}
	})
}

// TestInvestmentTransaction_MaturityFlipsStatus verifies the M4.6 hard guard:
// a Maturity transaction atomically flips the investment to 'matured' with
// terminated_at = the maturity date, and any further transaction on the now
// terminal position is rejected with ErrPositionNotActive (ADR-0009).
func TestInvestmentTransaction_MaturityFlipsStatus(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)
	aliceUser := testutil.CreateHouseholdWithUser(t, q, "Alice")
	aliceCtx := auth.WithUser(context.Background(), aliceUser)
	r := repo.NewInvestmentRepo(tdb.Pool)

	couponRate, _ := decimal.NewFromString("6.25")
	bond, err := r.CreateBond(aliceCtx, repo.CreateBondParams{
		DisplayName: "ORI024", OwnershipType: "joint", NativeCurrency: "IDR",
		BondType: "govt_primary", Issuer: "Republik Indonesia",
		CouponRate:      couponRate,
		CouponFrequency: "monthly",
		MaturityDate:    time.Date(2029, time.October, 15, 0, 0, 0, 0, time.UTC),
		RiskProfile:     "medium",
	})
	if err != nil {
		t.Fatalf("CreateBond: %v", err)
	}

	couponAmount := decimal.NewFromInt(52_000)
	if _, err := r.CreateInvestmentTransaction(aliceCtx, repo.CreateInvestmentTransactionParams{
		InvestmentID:    bond.Investment.ID,
		TransactionType: repo.TxnTypeCoupon,
		TransactionDate: time.Date(2026, time.April, 15, 0, 0, 0, 0, time.UTC),
		Currency:        "IDR",
		Amount:          &couponAmount,
	}); err != nil {
		t.Fatalf("coupon on active bond should succeed: %v", err)
	}

	matDate := time.Date(2029, time.October, 15, 0, 0, 0, 0, time.UTC)
	principal := decimal.NewFromInt(10_000_000)
	interest := decimal.NewFromInt(52_000)
	cashOut := repo.DispositionCashOut
	if _, err := r.CreateInvestmentTransaction(aliceCtx, repo.CreateInvestmentTransactionParams{
		InvestmentID:         bond.Investment.ID,
		TransactionType:      repo.TxnTypeMaturity,
		TransactionDate:      matDate,
		Currency:             "IDR",
		PrincipalAmount:      &principal,
		InterestAmount:       &interest,
		PrincipalDisposition: &cashOut,
		InterestDisposition:  &cashOut,
	}); err != nil {
		t.Fatalf("maturity on active bond should succeed: %v", err)
	}

	got, err := r.GetBond(aliceCtx, bond.Investment.ID)
	if err != nil {
		t.Fatalf("GetBond: %v", err)
	}
	if got.Investment.Status != "matured" {
		t.Fatalf("status not flipped to matured: %q", got.Investment.Status)
	}
	if got.Investment.TerminatedAt == nil || !got.Investment.TerminatedAt.Equal(matDate) {
		t.Fatalf("terminated_at not set to maturity date: %+v", got.Investment.TerminatedAt)
	}

	// A further transaction on the matured bond is rejected.
	_, err = r.CreateInvestmentTransaction(aliceCtx, repo.CreateInvestmentTransactionParams{
		InvestmentID:    bond.Investment.ID,
		TransactionType: repo.TxnTypeCoupon,
		TransactionDate: time.Date(2029, time.November, 15, 0, 0, 0, 0, time.UTC),
		Currency:        "IDR",
		Amount:          &couponAmount,
	})
	if !errors.Is(err, repo.ErrPositionNotActive) {
		t.Fatalf("want ErrPositionNotActive after maturity, got %v", err)
	}
}

// TestInvestmentLifecycle_CloseSnapshot covers #25's generalization of the
// truthful 0-value close snapshot to manual termination: terminating a position
// writes a 0 close at the termination month (in the subtype's shape), and the
// un-terminate correction affordance (ADR-0009) removes it so a reactivated
// position carries forward its last real value rather than reading 0.
func TestInvestmentLifecycle_CloseSnapshot(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)
	aliceUser := testutil.CreateHouseholdWithUser(t, q, "Alice")
	aliceCtx := auth.WithUser(context.Background(), aliceUser)
	r := repo.NewInvestmentRepo(tdb.Pool)

	stock, err := r.CreateStock(aliceCtx, repo.CreateStockParams{
		DisplayName: "BBCA", OwnershipType: "joint", NativeCurrency: "IDR",
		Ticker: "BBCA", Exchange: "IDX", RiskProfile: "medium",
	})
	if err != nil {
		t.Fatalf("CreateStock: %v", err)
	}

	// A real January snapshot (quantity/price shape) worth 500.
	qty, ppu := decimal.NewFromInt(10), decimal.NewFromInt(50)
	jan := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	if _, err := r.CreateInvestmentSnapshot(aliceCtx, repo.CreateInvestmentSnapshotParams{
		InvestmentID: stock.Investment.ID, YearMonth: jan, Amount: decimal.NewFromInt(500),
		Currency: "IDR", Quantity: &qty, PricePerUnit: &ppu,
	}); err != nil {
		t.Fatalf("CreateInvestmentSnapshot: %v", err)
	}

	termDate := time.Date(2026, time.February, 20, 0, 0, 0, 0, time.UTC)
	termMonth := time.Date(2026, time.February, 1, 0, 0, 0, 0, time.UTC)

	t.Run("termination writes 0-value close snapshot in subtype shape", func(t *testing.T) {
		if _, err := r.UpdateInvestmentLifecycle(aliceCtx, stock.Investment.ID, repo.LifecycleParams{
			Status: "sold", TerminatedAt: &termDate,
		}); err != nil {
			t.Fatalf("UpdateInvestmentLifecycle sold: %v", err)
		}
		snaps, err := r.ListInvestmentSnapshots(aliceCtx, stock.Investment.ID)
		if err != nil {
			t.Fatalf("ListInvestmentSnapshots: %v", err)
		}
		var closeSnap *db.InvestmentSnapshot
		for i := range snaps {
			if snaps[i].YearMonth.Equal(termMonth) {
				closeSnap = &snaps[i]
			}
		}
		if closeSnap == nil {
			t.Fatalf("no close snapshot at termination month; got %d snapshots", len(snaps))
		}
		if !closeSnap.Amount.IsZero() {
			t.Errorf("close Amount: got %s, want 0", closeSnap.Amount)
		}
		// Stock shape: quantity + price present (0), accrued nil.
		if closeSnap.Quantity == nil || !closeSnap.Quantity.IsZero() || closeSnap.PricePerUnit == nil || !closeSnap.PricePerUnit.IsZero() {
			t.Errorf("close quantity/price: got q=%v p=%v, want 0/0", closeSnap.Quantity, closeSnap.PricePerUnit)
		}
		if closeSnap.AccruedInterest != nil {
			t.Errorf("close AccruedInterest should be nil for stock shape; got %v", closeSnap.AccruedInterest)
		}
	})

	t.Run("un-terminate removes the close snapshot", func(t *testing.T) {
		if _, err := r.UpdateInvestmentLifecycle(aliceCtx, stock.Investment.ID, repo.LifecycleParams{
			Status: "active", TerminatedAt: nil,
		}); err != nil {
			t.Fatalf("UpdateInvestmentLifecycle un-terminate: %v", err)
		}
		snaps, err := r.ListInvestmentSnapshots(aliceCtx, stock.Investment.ID)
		if err != nil {
			t.Fatalf("ListInvestmentSnapshots: %v", err)
		}
		if len(snaps) != 1 {
			t.Fatalf("after un-terminate: got %d snapshots, want 1 (only January)", len(snaps))
		}
		if !snaps[0].YearMonth.Equal(jan) || !snaps[0].Amount.Equal(decimal.NewFromInt(500)) {
			t.Errorf("surviving snapshot: got %s=%s, want Jan=500", snaps[0].YearMonth.Format("2006-01"), snaps[0].Amount)
		}
	})
}
