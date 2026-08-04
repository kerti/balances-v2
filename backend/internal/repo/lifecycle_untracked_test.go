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

// `untracked` is the exit-side declaration of a Tracking Change (ADR-0053 §3):
// the Position left the Household's books without being sold, paid off,
// collected or lost. It is the one terminal status *every* group defines,
// Investment included — ADR-0053 §5 amends ADR-0052 §5 there, because a
// departing member's portfolio did not lose its value.
//
// The Investment case is the load-bearing one, and it is why this test exists
// rather than a line in the settlement-matrix test: ADR-0052 §6 makes every
// Investment terminal status demand a Sell or Maturity, and `untracked` must be
// exempt. Nothing was sold, so no Transaction can express it; without the
// exemption the status would be unreachable for every subtype and the whole exit
// side of the term would be dead code.
//
// covers: INV-LIFECYCLE-09, INV-LIFECYCLE-03
func TestUpdateInvestmentLifecycle_UntrackedNeedsNoSettlement(t *testing.T) {
	r, ctx := investmentRepoFor(t)

	qty := decimal.RequireFromString("100")
	price := decimal.RequireFromString("9500")
	stock := stockWithHolding(ctx, t, r, "Departing stock", qty, price)
	id := stock.Investment.ID

	// No InvestmentSettlement — the terminate dialog will not offer one for this
	// status (#595), and the raw API must not demand one either.
	termDate := day(2026, time.March, 15)
	row, err := r.UpdateInvestmentLifecycle(ctx, id, repo.LifecycleParams{
		Status:       repo.StatusUntracked,
		TerminatedAt: &termDate,
	}, nil)
	if err != nil {
		t.Fatalf("terminate as untracked: %v — the status must be exempt from the ADR-0052 §6 settlement capture", err)
	}
	if row.Status != repo.StatusUntracked {
		t.Errorf("status: got %q, want %q", row.Status, repo.StatusUntracked)
	}
	if row.TerminatedAt == nil {
		t.Error("terminated_at: got nil, want the termination date (the biconditional applies unchanged)")
	}

	// Nothing was sold, so the ledger must be untouched — only the funding Buy.
	txns, err := r.ListInvestmentTransactions(ctx, id)
	if err != nil {
		t.Fatalf("ListInvestmentTransactions: %v", err)
	}
	for _, txn := range txns {
		if txn.TransactionType != "buy" {
			t.Errorf("unexpected %q transaction: an untracked departure settles nothing", txn.TransactionType)
		}
	}

	// The 0-value close snapshot rule (INV-LIFECYCLE-03) is group-wide and knows
	// nothing about the status, so it must fire here too — the engine's exit-side
	// term reads exactly that snapshot as the departing `now`.
	snaps, err := r.ListInvestmentSnapshots(ctx, id)
	if err != nil {
		t.Fatalf("ListInvestmentSnapshots: %v", err)
	}
	var closed bool
	for _, s := range snaps {
		if s.YearMonth.Equal(ym(2026, time.March)) && s.Amount.IsZero() {
			closed = true
		}
	}
	if !closed {
		t.Error("no 0-value close snapshot at the termination month")
	}
}

// The group-level enum must admit `untracked` in all four groups — it is the
// only terminal status that spans them — and the status/terminated_at
// biconditional (INV-LIFECYCLE-01) still governs it, since it is a terminal
// status like any other.
//
// covers: INV-LIFECYCLE-09
func TestPositionLifecycle_UntrackedAcceptedByEveryGroup(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)
	alice := testutil.CreateHouseholdWithUser(t, q, "Alice")
	ctx := identity.WithUser(context.Background(), alice)

	ar := repo.NewAssetRepo(tdb.Pool)
	lr := repo.NewLiabilityRepo(tdb.Pool)
	rr := repo.NewReceivableRepo(tdb.Pool)
	termDate := day(2026, time.March, 15)

	acct, err := ar.CreateBankAccount(ctx, repo.CreateBankAccountParams{
		DisplayName: "Departing account", OwnershipType: "joint", NativeCurrency: "IDR",
		BankName: "Bank", AccountNumber: "111", AccountType: "savings",
	})
	if err != nil {
		t.Fatalf("CreateBankAccount: %v", err)
	}
	liab, err := lr.CreateLiability(ctx, repo.CreateLiabilityParams{
		DisplayName: "Departing loan", Subtype: "personal", OwnershipType: "joint",
		NativeCurrency: "IDR", CounterpartyName: "Bank",
	})
	if err != nil {
		t.Fatalf("CreateLiability: %v", err)
	}
	recv, err := rr.CreateReceivable(ctx, repo.CreateReceivableParams{
		DisplayName: "Departing IOU", OwnershipType: "joint", NativeCurrency: "IDR",
		CounterpartyName: "A friend",
	})
	if err != nil {
		t.Fatalf("CreateReceivable: %v", err)
	}

	terminate := func(p repo.LifecycleParams) error {
		if _, err := ar.UpdateAssetLifecycle(ctx, acct.Asset.ID, p); err != nil {
			return err
		}
		if _, err := lr.UpdateLiabilityLifecycle(ctx, liab.ID, p); err != nil {
			return err
		}
		_, err := rr.UpdateReceivableLifecycle(ctx, recv.ID, p)
		return err
	}

	if err := terminate(repo.LifecycleParams{
		Status: repo.StatusUntracked, TerminatedAt: &termDate,
	}); err != nil {
		t.Fatalf("terminate as untracked: %v — every group defines the status", err)
	}
	// The biconditional is untouched by the new status: terminal means dated.
	if err := terminate(repo.LifecycleParams{Status: repo.StatusUntracked}); err == nil {
		t.Error("untracked with no termination date was accepted; the biconditional must reject it")
	}
}
