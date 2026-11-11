package repo_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/kerti/balances-v2/backend/internal/db"
	"github.com/kerti/balances-v2/backend/internal/identity"
	"github.com/kerti/balances-v2/backend/internal/repo"
	"github.com/kerti/balances-v2/backend/internal/testutil"
)

func ptr[T any](v T) *T { return &v }

func matchIDs(ms []repo.PositionMatch) []uuid.UUID {
	out := make([]uuid.UUID, len(ms))
	for i, m := range ms {
		out[i] = m.ID
	}
	return out
}

func wantOnly(t *testing.T, got []repo.PositionMatch, want ...uuid.UUID) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("want matches %v, got %v", want, matchIDs(got))
	}
	for i := range want {
		if got[i].ID != want[i] {
			t.Fatalf("want matches %v, got %v", want, matchIDs(got))
		}
	}
}

// TestFindAssetImportMatches covers the duplicate-position lookup behind the
// create-import preview (issue #828): same subtype + case-insensitive trimmed
// name (+ account number for a bank account), terminated included, scoped to
// the caller's household, soft-deleted excluded.
// covers: INV-IMPORT-08
func TestFindAssetImportMatches(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)

	alice := testutil.CreateHouseholdWithUser(t, q, "Alice")
	bob := testutil.CreateHouseholdWithUser(t, q, "Bob")
	aliceCtx := identity.WithUser(context.Background(), alice)
	bobCtx := identity.WithUser(context.Background(), bob)
	assets := repo.NewAssetRepo(tdb.Pool)

	newAccount := func(ctx context.Context, name, number string) uuid.UUID {
		t.Helper()
		acct, err := assets.CreateBankAccount(ctx, repo.CreateBankAccountParams{
			DisplayName: name, OwnershipType: "joint", NativeCurrency: "IDR",
			BankName: "BCA", AccountNumber: number, AccountType: "savings",
		})
		if err != nil {
			t.Fatalf("CreateBankAccount(%q): %v", name, err)
		}
		return acct.Asset.ID
	}

	salary := newAccount(aliceCtx, "Salary", "111")
	newAccount(aliceCtx, "Salary", "222")              // same name, other number
	newAccount(aliceCtx, "Holiday fund", "111")        // same number, other name
	bobSalary := newAccount(bobCtx, "Salary", "111")   // other household
	closed := newAccount(aliceCtx, "Old savings", "9") // terminated below
	gone := newAccount(aliceCtx, "Gone", "7")          // soft-deleted below

	termDate := time.Date(2026, time.May, 25, 0, 0, 0, 0, time.UTC)
	if _, err := assets.UpdateAssetLifecycle(aliceCtx, closed, repo.LifecycleParams{Status: "closed", TerminatedAt: &termDate}); err != nil {
		t.Fatalf("UpdateAssetLifecycle: %v", err)
	}
	if err := assets.DeleteBankAccount(aliceCtx, gone); err != nil {
		t.Fatalf("DeleteBankAccount: %v", err)
	}
	// The soft delete must leave the row in place — otherwise the
	// "excluded" case below would pass for the wrong reason.
	var rows int
	if err := tdb.Pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM assets WHERE id = $1 AND deleted_at IS NOT NULL`, gone).Scan(&rows); err != nil || rows != 1 {
		t.Fatalf("want the deleted account kept as a tombstone, got count=%d err=%v", rows, err)
	}

	t.Run("name and number match", func(t *testing.T) {
		got, err := assets.FindImportMatches(aliceCtx, "bank_account", "Salary", ptr("111"))
		if err != nil {
			t.Fatalf("FindImportMatches: %v", err)
		}
		wantOnly(t, got, salary)
		m := got[0]
		if m.Name != "Salary" || m.Status != "active" || m.Group != "asset" || m.Subtype != "bank_account" {
			t.Errorf("match shape: %+v", m)
		}
	})

	t.Run("case-insensitive and trimmed", func(t *testing.T) {
		got, err := assets.FindImportMatches(aliceCtx, "bank_account", "  sALARY\t ", ptr(" 111 "))
		if err != nil {
			t.Fatalf("FindImportMatches: %v", err)
		}
		wantOnly(t, got, salary)
	})

	t.Run("a different account number is not a match", func(t *testing.T) {
		got, err := assets.FindImportMatches(aliceCtx, "bank_account", "Salary", ptr("333"))
		if err != nil {
			t.Fatalf("FindImportMatches: %v", err)
		}
		wantOnly(t, got)
	})

	t.Run("a different name is not a match", func(t *testing.T) {
		got, err := assets.FindImportMatches(aliceCtx, "bank_account", "Salary 2", ptr("111"))
		if err != nil {
			t.Fatalf("FindImportMatches: %v", err)
		}
		wantOnly(t, got)
	})

	t.Run("a terminated account still matches", func(t *testing.T) {
		got, err := assets.FindImportMatches(aliceCtx, "bank_account", "Old savings", ptr("9"))
		if err != nil {
			t.Fatalf("FindImportMatches: %v", err)
		}
		wantOnly(t, got, closed)
		if got[0].Status != "closed" {
			t.Errorf("want status closed, got %q", got[0].Status)
		}
	})

	t.Run("a soft-deleted account does not match", func(t *testing.T) {
		got, err := assets.FindImportMatches(aliceCtx, "bank_account", "Gone", ptr("7"))
		if err != nil {
			t.Fatalf("FindImportMatches: %v", err)
		}
		wantOnly(t, got)
	})

	t.Run("another household's account never matches, and each side sees only its own", func(t *testing.T) {
		got, err := assets.FindImportMatches(bobCtx, "bank_account", "Salary", ptr("111"))
		if err != nil {
			t.Fatalf("FindImportMatches: %v", err)
		}
		wantOnly(t, got, bobSalary)

		none, err := assets.FindImportMatches(bobCtx, "bank_account", "Old savings", ptr("9"))
		if err != nil {
			t.Fatalf("FindImportMatches: %v", err)
		}
		wantOnly(t, none)
	})

	t.Run("an identifier-less subtype matches on name alone, within its subtype", func(t *testing.T) {
		house, err := assets.CreateProperty(aliceCtx, repo.CreatePropertyParams{
			DisplayName: "Salary", OwnershipType: "joint", NativeCurrency: "IDR", PropertyType: "house",
		})
		if err != nil {
			t.Fatalf("CreateProperty: %v", err)
		}
		got, err := assets.FindImportMatches(aliceCtx, "property", "salary", nil)
		if err != nil {
			t.Fatalf("FindImportMatches: %v", err)
		}
		wantOnly(t, got, house.Asset.ID)
		if got[0].Subtype != "property" {
			t.Errorf("want subtype property, got %q", got[0].Subtype)
		}

		// The two bank accounts also called "Salary" must not leak across subtypes.
		vehicles, err := assets.FindImportMatches(aliceCtx, "vehicle", "Salary", nil)
		if err != nil {
			t.Fatalf("FindImportMatches: %v", err)
		}
		wantOnly(t, vehicles)
	})

	t.Run("no household on the context is an error, not an unscoped read", func(t *testing.T) {
		if _, err := assets.FindImportMatches(context.Background(), "bank_account", "Salary", ptr("111")); err == nil {
			t.Fatal("want an error without an authenticated user")
		}
	})
}

// TestFindInvestmentImportMatches is the investment half: name within a
// subtype, terminated included, household-scoped, soft-deleted excluded.
// covers: INV-IMPORT-08
func TestFindInvestmentImportMatches(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)

	alice := testutil.CreateHouseholdWithUser(t, q, "Alice")
	bob := testutil.CreateHouseholdWithUser(t, q, "Bob")
	aliceCtx := identity.WithUser(context.Background(), alice)
	bobCtx := identity.WithUser(context.Background(), bob)
	inv := repo.NewInvestmentRepo(tdb.Pool)

	newStock := func(ctx context.Context, name string) uuid.UUID {
		t.Helper()
		s, err := inv.CreateStock(ctx, repo.CreateStockParams{
			DisplayName: name, OwnershipType: "joint", NativeCurrency: "IDR",
			RiskProfile: "medium", Ticker: "BBCA", Exchange: "IDX",
		})
		if err != nil {
			t.Fatalf("CreateStock(%q): %v", name, err)
		}
		return s.Investment.ID
	}

	bbca := newStock(aliceCtx, "BBCA")
	sold := newStock(aliceCtx, "TLKM")
	gone := newStock(aliceCtx, "ASII")
	bobBBCA := newStock(bobCtx, "BBCA")
	fund, err := inv.CreateMutualFund(aliceCtx, repo.CreateMutualFundParams{
		DisplayName: "BBCA", OwnershipType: "joint", NativeCurrency: "IDR",
		RiskProfile: "low", FundCode: "F1", FundType: "equity",
	})
	if err != nil {
		t.Fatalf("CreateMutualFund: %v", err)
	}

	termDate := time.Date(2026, time.May, 25, 0, 0, 0, 0, time.UTC)
	if _, err := inv.UpdateInvestmentLifecycle(aliceCtx, sold, repo.LifecycleParams{Status: "sold", TerminatedAt: &termDate}, nil); err != nil {
		t.Fatalf("UpdateInvestmentLifecycle: %v", err)
	}
	if err := inv.DeleteStock(aliceCtx, gone); err != nil {
		t.Fatalf("DeleteStock: %v", err)
	}
	var rows int
	if err := tdb.Pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM investments WHERE id = $1 AND deleted_at IS NOT NULL`, gone).Scan(&rows); err != nil || rows != 1 {
		t.Fatalf("want the deleted stock kept as a tombstone, got count=%d err=%v", rows, err)
	}

	t.Run("same subtype and name, case-insensitive and trimmed", func(t *testing.T) {
		got, err := inv.FindImportMatches(aliceCtx, "stock", " bbca ")
		if err != nil {
			t.Fatalf("FindImportMatches: %v", err)
		}
		wantOnly(t, got, bbca)
		if got[0].Group != "investment" || got[0].Subtype != "stock" {
			t.Errorf("match shape: %+v", got[0])
		}
	})

	t.Run("the same name under another subtype is found only under that subtype", func(t *testing.T) {
		got, err := inv.FindImportMatches(aliceCtx, "mutual_fund", "BBCA")
		if err != nil {
			t.Fatalf("FindImportMatches: %v", err)
		}
		wantOnly(t, got, fund.Investment.ID)
	})

	t.Run("a different name is not a match", func(t *testing.T) {
		got, err := inv.FindImportMatches(aliceCtx, "stock", "BBRI")
		if err != nil {
			t.Fatalf("FindImportMatches: %v", err)
		}
		wantOnly(t, got)
	})

	t.Run("a sold position still matches", func(t *testing.T) {
		got, err := inv.FindImportMatches(aliceCtx, "stock", "TLKM")
		if err != nil {
			t.Fatalf("FindImportMatches: %v", err)
		}
		wantOnly(t, got, sold)
		if got[0].Status != "sold" {
			t.Errorf("want status sold, got %q", got[0].Status)
		}
	})

	t.Run("a soft-deleted position does not match", func(t *testing.T) {
		got, err := inv.FindImportMatches(aliceCtx, "stock", "ASII")
		if err != nil {
			t.Fatalf("FindImportMatches: %v", err)
		}
		wantOnly(t, got)
	})

	t.Run("households are isolated", func(t *testing.T) {
		got, err := inv.FindImportMatches(bobCtx, "stock", "BBCA")
		if err != nil {
			t.Fatalf("FindImportMatches: %v", err)
		}
		wantOnly(t, got, bobBBCA)
		none, err := inv.FindImportMatches(bobCtx, "stock", "TLKM")
		if err != nil {
			t.Fatalf("FindImportMatches: %v", err)
		}
		wantOnly(t, none)
	})
}

// TestFindLiabilityAndReceivableImportMatches covers the two remaining groups:
// liability by subtype + name, receivable by name alone.
// covers: INV-IMPORT-08
func TestFindLiabilityAndReceivableImportMatches(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)

	alice := testutil.CreateHouseholdWithUser(t, q, "Alice")
	bob := testutil.CreateHouseholdWithUser(t, q, "Bob")
	aliceCtx := identity.WithUser(context.Background(), alice)
	bobCtx := identity.WithUser(context.Background(), bob)
	liabs := repo.NewLiabilityRepo(tdb.Pool)
	recvs := repo.NewReceivableRepo(tdb.Pool)

	termDate := time.Date(2026, time.May, 25, 0, 0, 0, 0, time.UTC)

	kpr, err := liabs.CreateLiability(aliceCtx, repo.CreateLiabilityParams{
		DisplayName: "KPR", Subtype: "institutional", OwnershipType: "joint",
		NativeCurrency: "IDR", CounterpartyName: "Bank",
	})
	if err != nil {
		t.Fatalf("CreateLiability: %v", err)
	}
	paid, err := liabs.CreateLiability(aliceCtx, repo.CreateLiabilityParams{
		DisplayName: "Car loan", Subtype: "institutional", OwnershipType: "joint",
		NativeCurrency: "IDR", CounterpartyName: "Bank",
	})
	if err != nil {
		t.Fatalf("CreateLiability: %v", err)
	}
	if _, err := liabs.UpdateLiabilityLifecycle(aliceCtx, paid.ID, repo.LifecycleParams{Status: "paid_off", TerminatedAt: &termDate}); err != nil {
		t.Fatalf("UpdateLiabilityLifecycle: %v", err)
	}
	if _, err := liabs.CreateLiability(bobCtx, repo.CreateLiabilityParams{
		DisplayName: "KPR", Subtype: "institutional", OwnershipType: "joint",
		NativeCurrency: "IDR", CounterpartyName: "Bank",
	}); err != nil {
		t.Fatalf("CreateLiability(bob): %v", err)
	}

	t.Run("liability: subtype + name, case-insensitive", func(t *testing.T) {
		got, err := liabs.FindImportMatches(aliceCtx, "institutional", " kpr ")
		if err != nil {
			t.Fatalf("FindImportMatches: %v", err)
		}
		wantOnly(t, got, kpr.ID)
		if got[0].Group != "liability" || got[0].Subtype != "institutional" {
			t.Errorf("match shape: %+v", got[0])
		}
	})

	t.Run("liability: another subtype, or another household, is not a match", func(t *testing.T) {
		got, err := liabs.FindImportMatches(aliceCtx, "personal", "KPR")
		if err != nil {
			t.Fatalf("FindImportMatches: %v", err)
		}
		wantOnly(t, got)
		// Bob has his own KPR; alice's id must not appear in his results.
		bobGot, err := liabs.FindImportMatches(bobCtx, "institutional", "KPR")
		if err != nil {
			t.Fatalf("FindImportMatches: %v", err)
		}
		if len(bobGot) != 1 || bobGot[0].ID == kpr.ID {
			t.Fatalf("bob must see only his own KPR, got %v", matchIDs(bobGot))
		}
	})

	t.Run("liability: a paid-off one still matches", func(t *testing.T) {
		got, err := liabs.FindImportMatches(aliceCtx, "institutional", "Car loan")
		if err != nil {
			t.Fatalf("FindImportMatches: %v", err)
		}
		wantOnly(t, got, paid.ID)
		if got[0].Status != "paid_off" {
			t.Errorf("want status paid_off, got %q", got[0].Status)
		}
	})

	loan, err := recvs.CreateReceivable(aliceCtx, repo.CreateReceivableParams{
		DisplayName: "Loan to friend", OwnershipType: "joint", NativeCurrency: "IDR", CounterpartyName: "Carol",
	})
	if err != nil {
		t.Fatalf("CreateReceivable: %v", err)
	}
	collected, err := recvs.CreateReceivable(aliceCtx, repo.CreateReceivableParams{
		DisplayName: "Old IOU", OwnershipType: "joint", NativeCurrency: "IDR", CounterpartyName: "Dan",
	})
	if err != nil {
		t.Fatalf("CreateReceivable: %v", err)
	}
	if _, err := recvs.UpdateReceivableLifecycle(aliceCtx, collected.ID, repo.LifecycleParams{Status: "collected", TerminatedAt: &termDate}); err != nil {
		t.Fatalf("UpdateReceivableLifecycle: %v", err)
	}
	gone, err := recvs.CreateReceivable(aliceCtx, repo.CreateReceivableParams{
		DisplayName: "Forgotten", OwnershipType: "joint", NativeCurrency: "IDR", CounterpartyName: "Eve",
	})
	if err != nil {
		t.Fatalf("CreateReceivable: %v", err)
	}
	if err := recvs.DeleteReceivable(aliceCtx, gone.ID); err != nil {
		t.Fatalf("DeleteReceivable: %v", err)
	}

	t.Run("receivable: name alone, case-insensitive", func(t *testing.T) {
		got, err := recvs.FindImportMatches(aliceCtx, "  LOAN TO FRIEND")
		if err != nil {
			t.Fatalf("FindImportMatches: %v", err)
		}
		wantOnly(t, got, loan.ID)
		if got[0].Group != "receivable" || got[0].Subtype != "" {
			t.Errorf("match shape: %+v", got[0])
		}
	})

	t.Run("receivable: collected matches, deleted and cross-household do not", func(t *testing.T) {
		got, err := recvs.FindImportMatches(aliceCtx, "Old IOU")
		if err != nil {
			t.Fatalf("FindImportMatches: %v", err)
		}
		wantOnly(t, got, collected.ID)

		for _, name := range []string{"Forgotten", "No such"} {
			none, err := recvs.FindImportMatches(aliceCtx, name)
			if err != nil {
				t.Fatalf("FindImportMatches: %v", err)
			}
			wantOnly(t, none)
		}
		cross, err := recvs.FindImportMatches(bobCtx, "Loan to friend")
		if err != nil {
			t.Fatalf("FindImportMatches: %v", err)
		}
		wantOnly(t, cross)
	})
}
