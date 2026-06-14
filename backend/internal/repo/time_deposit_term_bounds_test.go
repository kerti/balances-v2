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

// newTermBoundsTD spins up a household + investment repo and a single time
// deposit with the term [2026-01-15, 2027-01-15], the fixture every subtest
// below confines its snapshots and transactions against (issue #62).
func newTermBoundsTD(t *testing.T) (*repo.InvestmentRepo, context.Context, *repo.TimeDeposit) {
	t.Helper()
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)
	user := testutil.CreateHouseholdWithUser(t, q, "Alice")
	ctx := auth.WithUser(context.Background(), user)
	r := repo.NewInvestmentRepo(tdb.Pool)

	rate, _ := decimal.NewFromString("5.5")
	td, err := r.CreateTimeDeposit(ctx, repo.CreateTimeDepositParams{
		DisplayName:    "BCA TD",
		OwnershipType:  "joint",
		NativeCurrency: "IDR",
		RiskProfile:    "medium",
		BankName:       "BCA",
		Principal:      decimal.NewFromInt(50_000_000),
		InterestRate:   rate,
		TermMonths:     12,
		PlacementDate:  time.Date(2026, time.January, 15, 0, 0, 0, 0, time.UTC),
		MaturityDate:   time.Date(2027, time.January, 15, 0, 0, 0, 0, time.UTC),
		RolloverPolicy: "auto_renew_principal",
	})
	if err != nil {
		t.Fatalf("CreateTimeDeposit: %v", err)
	}
	return r, ctx, td
}

// TestTimeDeposit_RejectsInvertedTerm guards the maturity-after-placement rule
// (issue #62) on both create and update — the app-layer companion to the DB
// CHECK in migration 00004.
//
// covers: INV-BONDS-03
func TestTimeDeposit_RejectsInvertedTerm(t *testing.T) {
	r, ctx, td := newTermBoundsTD(t)

	rate, _ := decimal.NewFromString("5.5")
	base := repo.CreateTimeDepositParams{
		DisplayName: "Inverted", OwnershipType: "joint", NativeCurrency: "IDR",
		RiskProfile: "medium", BankName: "BCA", Principal: decimal.NewFromInt(1_000),
		InterestRate: rate, TermMonths: 12, RolloverPolicy: "no_rollover",
	}

	t.Run("maturity equal to placement is rejected", func(t *testing.T) {
		p := base
		p.PlacementDate = time.Date(2026, time.January, 15, 0, 0, 0, 0, time.UTC)
		p.MaturityDate = p.PlacementDate
		if _, err := r.CreateTimeDeposit(ctx, p); !errors.Is(err, repo.ErrInvalidDepositTerm) {
			t.Fatalf("create equal term: want ErrInvalidDepositTerm, got %v", err)
		}
	})

	t.Run("maturity before placement is rejected", func(t *testing.T) {
		p := base
		p.PlacementDate = time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC)
		p.MaturityDate = time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC)
		if _, err := r.CreateTimeDeposit(ctx, p); !errors.Is(err, repo.ErrInvalidDepositTerm) {
			t.Fatalf("create inverted term: want ErrInvalidDepositTerm, got %v", err)
		}
	})

	t.Run("update to inverted term is rejected", func(t *testing.T) {
		_, err := r.UpdateTimeDeposit(ctx, td.Investment.ID, repo.UpdateTimeDepositParams{
			DisplayName: "BCA TD", OwnershipType: "joint", RiskProfile: "medium",
			BankName: "BCA", Principal: decimal.NewFromInt(50_000_000), InterestRate: rate,
			TermMonths: 12, RolloverPolicy: "auto_renew_principal",
			PlacementDate: time.Date(2027, time.January, 15, 0, 0, 0, 0, time.UTC),
			MaturityDate:  time.Date(2026, time.January, 15, 0, 0, 0, 0, time.UTC),
		})
		if !errors.Is(err, repo.ErrInvalidDepositTerm) {
			t.Fatalf("update inverted term: want ErrInvalidDepositTerm, got %v", err)
		}
	})
}

// TestTimeDepositSnapshot_ConfinedToTerm verifies a deposit's snapshots are held
// inside the term at month granularity (issue #62): a placement on the 15th
// still admits that whole month, the maturity month is admitted, and the months
// on either side are rejected.
// covers: INV-BONDS-03
func TestTimeDepositSnapshot_ConfinedToTerm(t *testing.T) {
	r, ctx, td := newTermBoundsTD(t)

	accrued := decimal.NewFromInt(100_000)
	create := func(ym time.Time) (*db.InvestmentSnapshot, error) {
		return r.CreateInvestmentSnapshot(ctx, repo.CreateInvestmentSnapshotParams{
			InvestmentID:    td.Investment.ID,
			YearMonth:       ym,
			Amount:          decimal.NewFromInt(50_100_000),
			Currency:        "IDR",
			AccruedInterest: &accrued,
		})
	}

	t.Run("month before placement is rejected", func(t *testing.T) {
		if _, err := create(time.Date(2025, time.December, 1, 0, 0, 0, 0, time.UTC)); !errors.Is(err, repo.ErrOutsideDepositTerm) {
			t.Fatalf("pre-term snapshot: want ErrOutsideDepositTerm, got %v", err)
		}
	})

	t.Run("placement month is admitted despite mid-month placement", func(t *testing.T) {
		snap, err := create(time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC))
		if err != nil {
			t.Fatalf("placement-month snapshot: %v", err)
		}
		if err := r.DeleteInvestmentSnapshot(ctx, snap.ID); err != nil {
			t.Fatalf("cleanup: %v", err)
		}
	})

	t.Run("maturity month is admitted", func(t *testing.T) {
		if _, err := create(time.Date(2027, time.January, 1, 0, 0, 0, 0, time.UTC)); err != nil {
			t.Fatalf("maturity-month snapshot: %v", err)
		}
	})

	t.Run("month after maturity is rejected", func(t *testing.T) {
		if _, err := create(time.Date(2027, time.February, 1, 0, 0, 0, 0, time.UTC)); !errors.Is(err, repo.ErrOutsideDepositTerm) {
			t.Fatalf("post-term snapshot: want ErrOutsideDepositTerm, got %v", err)
		}
	})
}

// TestTimeDepositMaturity_ConfinedToTerm verifies the terminal Maturity event is
// bound to the term to the day (issue #62): a payout dated after maturity_date
// or before placement_date is rejected, the maturity date itself is accepted.
// covers: INV-BONDS-03
func TestTimeDepositMaturity_ConfinedToTerm(t *testing.T) {
	r, ctx, td := newTermBoundsTD(t)

	cashOut := repo.DispositionCashOut
	principal := decimal.NewFromInt(50_000_000)
	interest := decimal.NewFromInt(2_750_000)
	mature := func(date time.Time) error {
		_, err := r.CreateInvestmentTransaction(ctx, repo.CreateInvestmentTransactionParams{
			InvestmentID:         td.Investment.ID,
			TransactionType:      repo.TxnTypeMaturity,
			TransactionDate:      date,
			Currency:             "IDR",
			PrincipalAmount:      &principal,
			InterestAmount:       &interest,
			PrincipalDisposition: &cashOut,
			InterestDisposition:  &cashOut,
		})
		return err
	}

	t.Run("maturity after maturity_date is rejected", func(t *testing.T) {
		if err := mature(time.Date(2027, time.January, 16, 0, 0, 0, 0, time.UTC)); !errors.Is(err, repo.ErrOutsideDepositTerm) {
			t.Fatalf("late maturity: want ErrOutsideDepositTerm, got %v", err)
		}
	})

	t.Run("maturity before placement is rejected", func(t *testing.T) {
		if err := mature(time.Date(2026, time.January, 14, 0, 0, 0, 0, time.UTC)); !errors.Is(err, repo.ErrOutsideDepositTerm) {
			t.Fatalf("early maturity: want ErrOutsideDepositTerm, got %v", err)
		}
	})

	t.Run("maturity on maturity_date is accepted", func(t *testing.T) {
		if err := mature(time.Date(2027, time.January, 15, 0, 0, 0, 0, time.UTC)); err != nil {
			t.Fatalf("on-time maturity: %v", err)
		}
	})
}

// TestTimeDeposit_TermEditCannotStrandHistory verifies a term edit is refused
// when it would leave an existing snapshot outside the new window — the user
// must fix the offending entry first (issue #62).
// covers: INV-BONDS-03
func TestTimeDeposit_TermEditCannotStrandHistory(t *testing.T) {
	r, ctx, td := newTermBoundsTD(t)

	accrued := decimal.NewFromInt(100_000)
	if _, err := r.CreateInvestmentSnapshot(ctx, repo.CreateInvestmentSnapshotParams{
		InvestmentID:    td.Investment.ID,
		YearMonth:       time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC),
		Amount:          decimal.NewFromInt(50_100_000),
		Currency:        "IDR",
		AccruedInterest: &accrued,
	}); err != nil {
		t.Fatalf("CreateInvestmentSnapshot: %v", err)
	}

	rate, _ := decimal.NewFromString("5.5")
	// Shrink the term to start in April — the March snapshot would be stranded.
	_, err := r.UpdateTimeDeposit(ctx, td.Investment.ID, repo.UpdateTimeDepositParams{
		DisplayName: "BCA TD", OwnershipType: "joint", RiskProfile: "medium",
		BankName: "BCA", Principal: decimal.NewFromInt(50_000_000), InterestRate: rate,
		TermMonths: 9, RolloverPolicy: "auto_renew_principal",
		PlacementDate: time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC),
		MaturityDate:  time.Date(2027, time.January, 15, 0, 0, 0, 0, time.UTC),
	})
	if !errors.Is(err, repo.ErrOutsideDepositTerm) {
		t.Fatalf("stranding term edit: want ErrOutsideDepositTerm, got %v", err)
	}
}
