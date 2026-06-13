package repo_test

import (
	"context"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/kerti/balances-v2/backend/internal/auth"
	"github.com/kerti/balances-v2/backend/internal/db"
	"github.com/kerti/balances-v2/backend/internal/repo"
	"github.com/kerti/balances-v2/backend/internal/testutil"
)

// TestUpdateMaturityTransaction_SyncsTerminationAndCloseSnapshot covers issue
// #58: editing a Maturity transaction's date must drag the position's
// terminated_at and the 0-value close snapshot along with it. The reported case
// was a mistyped year — the transaction updated but the position's termination
// date (and, by extension, its close-snapshot month) stayed stale.
func TestUpdateMaturityTransaction_SyncsTerminationAndCloseSnapshot(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)

	user := testutil.CreateHouseholdWithUser(t, q, "Alice")
	ctx := auth.WithUser(context.Background(), user)
	r := repo.NewInvestmentRepo(tdb.Pool)

	interestRate, _ := decimal.NewFromString("5.5")
	td, err := r.CreateTimeDeposit(ctx, repo.CreateTimeDepositParams{
		DisplayName:    "BCA TD",
		OwnershipType:  "joint",
		NativeCurrency: "IDR",
		RiskProfile:    "medium",
		BankName:       "BCA",
		Principal:      decimal.NewFromInt(50_000_000),
		InterestRate:   interestRate,
		TermMonths:     12,
		PlacementDate:  time.Date(2026, time.January, 15, 0, 0, 0, 0, time.UTC),
		MaturityDate:   time.Date(2027, time.January, 15, 0, 0, 0, 0, time.UTC),
		RolloverPolicy: "auto_renew_principal",
	})
	if err != nil {
		t.Fatalf("CreateTimeDeposit: %v", err)
	}

	cashOut := repo.DispositionCashOut
	principal := decimal.NewFromInt(50_000_000)
	interest := decimal.NewFromInt(2_750_000)

	// Mistyped year: maturity entered as 2028 instead of 2027.
	wrongDate := time.Date(2028, time.January, 15, 0, 0, 0, 0, time.UTC)
	mat, err := r.CreateInvestmentTransaction(ctx, repo.CreateInvestmentTransactionParams{
		InvestmentID:         td.Investment.ID,
		TransactionType:      repo.TxnTypeMaturity,
		TransactionDate:      wrongDate,
		Currency:             "IDR",
		PrincipalAmount:      &principal,
		InterestAmount:       &interest,
		PrincipalDisposition: &cashOut,
		InterestDisposition:  &cashOut,
	})
	if err != nil {
		t.Fatalf("CreateInvestmentTransaction maturity: %v", err)
	}

	// Sanity: create flipped the position and wrote the close at the wrong month.
	got, err := r.GetTimeDeposit(ctx, td.Investment.ID)
	if err != nil {
		t.Fatalf("GetTimeDeposit: %v", err)
	}
	if got.Investment.TerminatedAt == nil || !got.Investment.TerminatedAt.Equal(wrongDate) {
		t.Fatalf("pre-edit terminated_at: got %v, want %s", got.Investment.TerminatedAt, wrongDate.Format("2006-01-02"))
	}

	// ----- Correct the year (crosses a month boundary) -----
	rightDate := time.Date(2027, time.January, 15, 0, 0, 0, 0, time.UTC)
	if _, err := r.UpdateInvestmentTransaction(ctx, repo.UpdateInvestmentTransactionParams{
		TransactionID:        mat.ID,
		TransactionDate:      rightDate,
		Currency:             "IDR",
		PrincipalAmount:      &principal,
		InterestAmount:       &interest,
		PrincipalDisposition: &cashOut,
		InterestDisposition:  &cashOut,
	}); err != nil {
		t.Fatalf("UpdateInvestmentTransaction: %v", err)
	}

	got, err = r.GetTimeDeposit(ctx, td.Investment.ID)
	if err != nil {
		t.Fatalf("GetTimeDeposit after edit: %v", err)
	}
	if got.Investment.Status != repo.StatusMatured {
		t.Errorf("status after edit: got %q, want %q", got.Investment.Status, repo.StatusMatured)
	}
	if got.Investment.TerminatedAt == nil || !got.Investment.TerminatedAt.Equal(rightDate) {
		t.Errorf("terminated_at after edit: got %v, want %s", got.Investment.TerminatedAt, rightDate.Format("2006-01-02"))
	}

	// The close snapshot must have moved to the corrected month, leaving exactly
	// one 0-value close at 2027-01 and nothing at the stale 2028-01.
	snaps, err := r.ListInvestmentSnapshots(ctx, td.Investment.ID)
	if err != nil {
		t.Fatalf("ListInvestmentSnapshots: %v", err)
	}
	if len(snaps) != 1 {
		t.Fatalf("snapshots after edit: got %d, want 1", len(snaps))
	}
	wantYM := time.Date(2027, time.January, 1, 0, 0, 0, 0, time.UTC)
	if !snaps[0].YearMonth.Equal(wantYM) {
		t.Errorf("close snapshot month: got %s, want %s", snaps[0].YearMonth.Format("2006-01-02"), wantYM.Format("2006-01-02"))
	}
	if !snaps[0].Amount.IsZero() {
		t.Errorf("close snapshot amount: got %s, want 0", snaps[0].Amount.String())
	}
}

// TestUpdateMaturityTransaction_SameMonthEdit verifies a within-month date
// correction moves terminated_at but does not duplicate or strand the close
// snapshot (the relocation is skipped; the upsert just refreshes it in place).
func TestUpdateMaturityTransaction_SameMonthEdit(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)

	user := testutil.CreateHouseholdWithUser(t, q, "Alice")
	ctx := auth.WithUser(context.Background(), user)
	r := repo.NewInvestmentRepo(tdb.Pool)

	interestRate, _ := decimal.NewFromString("4.0")
	td, err := r.CreateTimeDeposit(ctx, repo.CreateTimeDepositParams{
		DisplayName:    "Mandiri TD",
		OwnershipType:  "joint",
		NativeCurrency: "IDR",
		RiskProfile:    "low",
		BankName:       "Mandiri",
		Principal:      decimal.NewFromInt(20_000_000),
		InterestRate:   interestRate,
		TermMonths:     6,
		PlacementDate:  time.Date(2026, time.January, 10, 0, 0, 0, 0, time.UTC),
		MaturityDate:   time.Date(2026, time.July, 10, 0, 0, 0, 0, time.UTC),
		RolloverPolicy: "no_rollover",
	})
	if err != nil {
		t.Fatalf("CreateTimeDeposit: %v", err)
	}

	cashOut := repo.DispositionCashOut
	principal := decimal.NewFromInt(20_000_000)
	interest := decimal.NewFromInt(400_000)

	mat, err := r.CreateInvestmentTransaction(ctx, repo.CreateInvestmentTransactionParams{
		InvestmentID:         td.Investment.ID,
		TransactionType:      repo.TxnTypeMaturity,
		TransactionDate:      time.Date(2026, time.July, 8, 0, 0, 0, 0, time.UTC),
		Currency:             "IDR",
		PrincipalAmount:      &principal,
		InterestAmount:       &interest,
		PrincipalDisposition: &cashOut,
		InterestDisposition:  &cashOut,
	})
	if err != nil {
		t.Fatalf("CreateInvestmentTransaction maturity: %v", err)
	}

	corrected := time.Date(2026, time.July, 10, 0, 0, 0, 0, time.UTC)
	if _, err := r.UpdateInvestmentTransaction(ctx, repo.UpdateInvestmentTransactionParams{
		TransactionID:        mat.ID,
		TransactionDate:      corrected,
		Currency:             "IDR",
		PrincipalAmount:      &principal,
		InterestAmount:       &interest,
		PrincipalDisposition: &cashOut,
		InterestDisposition:  &cashOut,
	}); err != nil {
		t.Fatalf("UpdateInvestmentTransaction: %v", err)
	}

	got, err := r.GetTimeDeposit(ctx, td.Investment.ID)
	if err != nil {
		t.Fatalf("GetTimeDeposit: %v", err)
	}
	if got.Investment.TerminatedAt == nil || !got.Investment.TerminatedAt.Equal(corrected) {
		t.Errorf("terminated_at: got %v, want %s", got.Investment.TerminatedAt, corrected.Format("2006-01-02"))
	}

	snaps, err := r.ListInvestmentSnapshots(ctx, td.Investment.ID)
	if err != nil {
		t.Fatalf("ListInvestmentSnapshots: %v", err)
	}
	if len(snaps) != 1 {
		t.Fatalf("snapshots after same-month edit: got %d, want 1", len(snaps))
	}
	wantYM := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	if !snaps[0].YearMonth.Equal(wantYM) {
		t.Errorf("close snapshot month: got %s, want %s", snaps[0].YearMonth.Format("2006-01-02"), wantYM.Format("2006-01-02"))
	}
	// The statement date (as_of_date) must follow the corrected maturity date,
	// even though the month is unchanged — the close snapshot stands in for the
	// realized maturity event, so it should read the day it actually matured.
	if snaps[0].AsOfDate == nil || !snaps[0].AsOfDate.Equal(corrected) {
		t.Errorf("close snapshot as_of_date: got %v, want %s", snaps[0].AsOfDate, corrected.Format("2006-01-02"))
	}
}
