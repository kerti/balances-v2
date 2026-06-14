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

// Repo-level cover for the create-from-list investment seed (issue #90). The
// handler suite drives the happy paths end-to-end; these reach the two branches
// the HTTP layer can't: the snapshot-shape backstop (the importer parses by
// endpoint subtype, so a mismatched shape never arrives over HTTP) and the
// returned subtype aggregate.

func investmentRepoFor(t *testing.T) (*repo.InvestmentRepo, context.Context) {
	t.Helper()
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)
	alice := testutil.CreateHouseholdWithUser(t, q, "Alice")
	return repo.NewInvestmentRepo(tdb.Pool), auth.WithUser(context.Background(), alice)
}

// covers: INV-IMPORT-03
func TestCreateStockWithSnapshotsAndLedger_Aggregate(t *testing.T) {
	r, ctx := investmentRepoFor(t)
	qty := decimal.RequireFromString("100")
	price := decimal.RequireFromString("9500")
	stock, err := r.CreateStockWithSnapshotsAndLedger(ctx, repo.CreateStockParams{
		DisplayName:    "Seeded stock",
		OwnershipType:  "joint",
		NativeCurrency: "IDR",
		RiskProfile:    "medium",
		Ticker:         "BBCA",
		Exchange:       "IDX",
	}, nil, []repo.ImportInvestmentSnapshotRow{
		{YearMonth: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), Amount: qty.Mul(price), Currency: "IDR", Quantity: &qty, PricePerUnit: &price},
	}, []repo.ImportTransactionRow{
		{TransactionType: "buy", TransactionDate: time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC), Currency: "IDR", Amount: ptrDec("950000"), Quantity: &qty, PricePerUnit: &price},
	})
	if err != nil {
		t.Fatalf("CreateStockWithSnapshotsAndLedger: %v", err)
	}
	if stock.Details.Ticker != "BBCA" || stock.Investment.Subtype != "stock" {
		t.Errorf("aggregate not populated: %+v", stock)
	}
}

// A snapshot row carrying the wrong value-shape for the subtype (accrued_interest
// on a quantity-price stock) is rejected by the seed's shape backstop, rolling
// the whole create back.
// covers: INV-IMPORT-02
func TestCreateStockWithSnapshotsAndLedger_RejectsMismatchedSnapshotShape(t *testing.T) {
	r, ctx := investmentRepoFor(t)
	accrued := decimal.RequireFromString("1000")
	_, err := r.CreateStockWithSnapshotsAndLedger(ctx, repo.CreateStockParams{
		DisplayName:    "Bad-shape stock",
		OwnershipType:  "joint",
		NativeCurrency: "IDR",
		RiskProfile:    "medium",
		Ticker:         "BBCA",
		Exchange:       "IDX",
	}, nil, []repo.ImportInvestmentSnapshotRow{
		// accrued_interest belongs to bond/time_deposit, not stock.
		{YearMonth: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), Amount: decimal.RequireFromString("100"), Currency: "IDR", AccruedInterest: &accrued},
	}, nil)
	if err == nil {
		t.Fatal("want a shape-validation error, got nil")
	}
	list, err := r.ListStocks(ctx)
	if err != nil {
		t.Fatalf("ListStocks: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("rejected create left %d stocks behind (not rolled back)", len(list))
	}
}

func ptrDec(s string) *decimal.Decimal {
	d := decimal.RequireFromString(s)
	return &d
}

func ym(y int, m time.Month) time.Time { return time.Date(y, m, 1, 0, 0, 0, 0, time.UTC) }
func day(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// accruedSnap is one bond/time-deposit shaped seed snapshot (amount + accrued).
func accruedSnap(month time.Month, amount, accrued string) repo.ImportInvestmentSnapshotRow {
	return repo.ImportInvestmentSnapshotRow{
		YearMonth: ym(2030, month), Amount: decimal.RequireFromString(amount),
		Currency: "IDR", AccruedInterest: ptrDec(accrued),
	}
}

// TestCreateInvestmentWithSnapshotsAndLedger_PerSubtype drives each subtype's
// repo create-with-history method directly (the handler suite's cross-package
// exercise doesn't count toward this package's coverage). Each seeds a snapshot
// and a subtype-legal ledger.
// covers: INV-IMPORT-03
func TestCreateInvestmentWithSnapshotsAndLedger_PerSubtype(t *testing.T) {
	qty := decimal.RequireFromString("100")
	price := decimal.RequireFromString("9500")
	qpSnap := repo.ImportInvestmentSnapshotRow{YearMonth: ym(2026, 1), Amount: qty.Mul(price), Currency: "IDR", Quantity: &qty, PricePerUnit: &price}
	buy := repo.ImportTransactionRow{TransactionType: "buy", TransactionDate: day(2026, 1, 5), Currency: "IDR", Amount: ptrDec("950000"), Quantity: &qty, PricePerUnit: &price}

	t.Run("mutual fund (buy + distribution)", func(t *testing.T) {
		r, ctx := investmentRepoFor(t)
		mf, err := r.CreateMutualFundWithSnapshotsAndLedger(ctx, repo.CreateMutualFundParams{
			DisplayName: "MF", OwnershipType: "joint", NativeCurrency: "IDR", RiskProfile: "medium",
			FundCode: "BNI-AM", FundType: "money_market",
		}, nil, []repo.ImportInvestmentSnapshotRow{qpSnap}, []repo.ImportTransactionRow{
			buy, {TransactionType: "distribution", TransactionDate: day(2026, 3, 1), Currency: "IDR", Amount: ptrDec("40000")},
		})
		if err != nil {
			t.Fatalf("CreateMutualFundWithSnapshotsAndLedger: %v", err)
		}
		if mf.Details.FundCode != "BNI-AM" {
			t.Errorf("details not populated: %+v", mf.Details)
		}
	})

	t.Run("gold (buy + fee)", func(t *testing.T) {
		r, ctx := investmentRepoFor(t)
		if _, err := r.CreateGoldWithSnapshotsAndLedger(ctx, repo.CreateGoldParams{
			DisplayName: "Gold", OwnershipType: "joint", NativeCurrency: "IDR", RiskProfile: "medium",
			Form: "bar", Purity: decimal.RequireFromString("0.9999"),
		}, nil, []repo.ImportInvestmentSnapshotRow{qpSnap}, []repo.ImportTransactionRow{
			buy, {TransactionType: "fee", TransactionDate: day(2026, 2, 1), Currency: "IDR", Amount: ptrDec("5000")},
		}); err != nil {
			t.Fatalf("CreateGoldWithSnapshotsAndLedger: %v", err)
		}
	})
}

// TestCreateBondWithSnapshotsAndLedger_Maturity covers the terminal seed at the
// repo level: a Maturity row (applied last) matures the bond and the 0-value
// close snapshot overwrites the seeded snapshot in the maturity month. The
// placement Buy is seeded once — the bond seed never auto-seeds it.
// covers: INV-LIFECYCLE-02, INV-LIFECYCLE-03
func TestCreateBondWithSnapshotsAndLedger_Maturity(t *testing.T) {
	r, ctx := investmentRepoFor(t)
	bond, err := r.CreateBondWithSnapshotsAndLedger(ctx, repo.CreateBondParams{
		DisplayName: "Matured bond", OwnershipType: "joint", NativeCurrency: "IDR", RiskProfile: "medium",
		BondType: "govt_primary", Issuer: "Govt", CouponRate: decimal.RequireFromString("6.25"),
		CouponFrequency: "monthly", MaturityDate: day(2030, 1, 1),
	}, nil,
		// A pre-maturity reading in the maturity month (2030-01) — overwritten by the 0 close.
		[]repo.ImportInvestmentSnapshotRow{accruedSnap(time.January, "10500000", "0")},
		[]repo.ImportTransactionRow{
			// Maturity listed first; seedLedger applies it last.
			{TransactionType: "maturity", TransactionDate: day(2030, 1, 1), Currency: "IDR",
				PrincipalAmount: ptrDec("10000000"), InterestAmount: ptrDec("600000"),
				PrincipalDisposition: ptrStrLit("cash_out"), InterestDisposition: ptrStrLit("cash_out")},
			{TransactionType: "coupon", TransactionDate: day(2029, 6, 1), Currency: "IDR", Amount: ptrDec("52083")},
		})
	if err != nil {
		t.Fatalf("CreateBondWithSnapshotsAndLedger: %v", err)
	}

	got, err := r.GetBond(ctx, bond.Investment.ID)
	if err != nil {
		t.Fatalf("GetBond: %v", err)
	}
	if got.Investment.Status != repo.StatusMatured || got.Investment.TerminatedAt == nil {
		t.Errorf("bond not matured: status=%q terminated=%v", got.Investment.Status, got.Investment.TerminatedAt)
	}
	snaps, err := r.ListInvestmentSnapshots(ctx, bond.Investment.ID)
	if err != nil {
		t.Fatalf("ListInvestmentSnapshots: %v", err)
	}
	if len(snaps) != 1 || !snaps[0].Amount.IsZero() {
		t.Errorf("want one 0-value close snapshot, got %+v", snaps)
	}
	txns, err := r.ListInvestmentTransactions(ctx, bond.Investment.ID)
	if err != nil {
		t.Fatalf("ListInvestmentTransactions: %v", err)
	}
	if len(txns) != 2 {
		t.Errorf("want 2 ledger rows (coupon + maturity), got %d", len(txns))
	}
}

// TestCreateTimeDepositWithSnapshotsAndLedger_Maturity: a TD's only ledger type
// is Maturity, so a matured deposit round-trips with its single Maturity row.
// covers: INV-LIFECYCLE-02
func TestCreateTimeDepositWithSnapshotsAndLedger_Maturity(t *testing.T) {
	r, ctx := investmentRepoFor(t)
	td, err := r.CreateTimeDepositWithSnapshotsAndLedger(ctx, repo.CreateTimeDepositParams{
		DisplayName: "Matured TD", OwnershipType: "joint", NativeCurrency: "IDR", RiskProfile: "medium",
		BankName: "BCA", Principal: decimal.RequireFromString("100000000"), InterestRate: decimal.RequireFromString("4.5"),
		TermMonths: 12, PlacementDate: day(2026, 1, 1), MaturityDate: day(2027, 1, 1), RolloverPolicy: "no_rollover",
	}, nil, nil, []repo.ImportTransactionRow{
		{TransactionType: "maturity", TransactionDate: day(2027, 1, 1), Currency: "IDR",
			PrincipalAmount: ptrDec("100000000"), InterestAmount: ptrDec("4500000"),
			PrincipalDisposition: ptrStrLit("cash_out"), InterestDisposition: ptrStrLit("cash_out")},
	})
	if err != nil {
		t.Fatalf("CreateTimeDepositWithSnapshotsAndLedger: %v", err)
	}
	got, err := r.GetTimeDeposit(ctx, td.Investment.ID)
	if err != nil {
		t.Fatalf("GetTimeDeposit: %v", err)
	}
	if got.Investment.Status != repo.StatusMatured {
		t.Errorf("status: want matured, got %q", got.Investment.Status)
	}
}

// TestCreateTimeDepositWithSnapshotsAndLedger_TermBounds: the seed path confines
// a deposit's snapshots and Maturity to its term, and rejects an inverted term
// (issue #62) — the import counterpart of the manual-path guards.
func TestCreateTimeDepositWithSnapshotsAndLedger_TermBounds(t *testing.T) {
	r, ctx := investmentRepoFor(t)
	accrued := decimal.RequireFromString("100000")
	base := func() repo.CreateTimeDepositParams {
		return repo.CreateTimeDepositParams{
			DisplayName: "Seeded TD", OwnershipType: "joint", NativeCurrency: "IDR", RiskProfile: "medium",
			BankName: "BCA", Principal: decimal.RequireFromString("100000000"), InterestRate: decimal.RequireFromString("4.5"),
			TermMonths: 12, PlacementDate: day(2026, 1, 15), MaturityDate: day(2027, 1, 15), RolloverPolicy: "no_rollover",
		}
	}

	t.Run("snapshot before the term rolls the seed back", func(t *testing.T) {
		_, err := r.CreateTimeDepositWithSnapshotsAndLedger(ctx, base(), nil,
			[]repo.ImportInvestmentSnapshotRow{
				{YearMonth: ym(2025, time.December), Amount: decimal.RequireFromString("100000000"), Currency: "IDR", AccruedInterest: &accrued},
			}, nil)
		if !errors.Is(err, repo.ErrOutsideDepositTerm) {
			t.Fatalf("pre-term seed snapshot: want ErrOutsideDepositTerm, got %v", err)
		}
	})

	t.Run("maturity after the term rolls the seed back", func(t *testing.T) {
		_, err := r.CreateTimeDepositWithSnapshotsAndLedger(ctx, base(), nil, nil,
			[]repo.ImportTransactionRow{
				{TransactionType: "maturity", TransactionDate: day(2027, 2, 1), Currency: "IDR",
					PrincipalAmount: ptrDec("100000000"), InterestAmount: ptrDec("4500000"),
					PrincipalDisposition: ptrStrLit("cash_out"), InterestDisposition: ptrStrLit("cash_out")},
			})
		if !errors.Is(err, repo.ErrOutsideDepositTerm) {
			t.Fatalf("post-term seed maturity: want ErrOutsideDepositTerm, got %v", err)
		}
	})

	t.Run("inverted term is rejected before any seeding", func(t *testing.T) {
		p := base()
		p.PlacementDate, p.MaturityDate = day(2027, 1, 15), day(2026, 1, 15)
		if _, err := r.CreateTimeDepositWithSnapshotsAndLedger(ctx, p, nil, nil, nil); !errors.Is(err, repo.ErrInvalidDepositTerm) {
			t.Fatalf("inverted seed term: want ErrInvalidDepositTerm, got %v", err)
		}
	})
}

// TestInvestmentRepo_LookupAndTagSeed covers the owner-email + tag-name lookups
// and a tag actually assigned through the seed.
// covers: INV-IMPORT-04
func TestInvestmentRepo_LookupAndTagSeed(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)
	alice := testutil.CreateHouseholdWithUser(t, q, "Alice")
	ctx := auth.WithUser(context.Background(), alice)
	r := repo.NewInvestmentRepo(tdb.Pool)
	tag, err := repo.NewTagRepo(tdb.Pool).CreateTag(ctx, "Brokerage", "#22c55e")
	if err != nil {
		t.Fatalf("CreateTag: %v", err)
	}

	id, found, err := r.LookupUserIDByEmail(ctx, alice.Email)
	if err != nil || !found || id != alice.ID {
		t.Fatalf("LookupUserIDByEmail: id=%v found=%v err=%v", id, found, err)
	}
	if _, found, _ := r.LookupUserIDByEmail(ctx, "nobody@example.com"); found {
		t.Error("unknown email should not be found")
	}
	tagID, err := r.LookupTagIDByName(ctx, "Brokerage")
	if err != nil || tagID == nil || *tagID != tag.ID {
		t.Fatalf("LookupTagIDByName: %v / %v", tagID, err)
	}
	if got, _ := r.LookupTagIDByName(ctx, "No Such Tag"); got != nil {
		t.Errorf("unknown tag should resolve to nil, got %v", got)
	}
	if got, _ := r.LookupTagIDByName(ctx, "  "); got != nil {
		t.Errorf("blank tag should resolve to nil, got %v", got)
	}

	stock, err := r.CreateStockWithSnapshotsAndLedger(ctx, repo.CreateStockParams{
		DisplayName: "Tagged", OwnershipType: "sole", SoleOwnerUserID: &id,
		NativeCurrency: "IDR", RiskProfile: "medium", Ticker: "BBCA", Exchange: "IDX",
	}, tagID, nil, nil)
	if err != nil {
		t.Fatalf("CreateStockWithSnapshotsAndLedger: %v", err)
	}
	if stock.Investment.TagID == nil || *stock.Investment.TagID != tag.ID {
		t.Errorf("tag not assigned through the seed: %v", stock.Investment.TagID)
	}
}

func ptrStrLit(s string) *string { return &s }

// TestValidateSeedTransaction unit-tests the exported seed validator (pure, no
// DB): the subtype→type matrix and the ADR-0023 column-combo shape.
// covers: INV-COST-BASIS-04
func TestValidateSeedTransaction(t *testing.T) {
	qty := decimal.RequireFromString("100")
	price := decimal.RequireFromString("9500")
	amt := decimal.RequireFromString("950000")

	t.Run("valid stock buy", func(t *testing.T) {
		err := repo.ValidateSeedTransaction("stock", repo.CreateInvestmentTransactionParams{
			TransactionType: "buy", Amount: &amt, Quantity: &qty, PricePerUnit: &price,
		})
		if err != nil {
			t.Fatalf("want nil, got %v", err)
		}
	})

	t.Run("coupon off the stock matrix", func(t *testing.T) {
		err := repo.ValidateSeedTransaction("stock", repo.CreateInvestmentTransactionParams{
			TransactionType: "coupon", Amount: &amt,
		})
		if !errors.Is(err, repo.ErrInvalidTransactionType) {
			t.Fatalf("want ErrInvalidTransactionType, got %v", err)
		}
	})

	t.Run("maturity missing dispositions is a shape error", func(t *testing.T) {
		err := repo.ValidateSeedTransaction("bond", repo.CreateInvestmentTransactionParams{
			TransactionType: "maturity", PrincipalAmount: &amt, InterestAmount: &amt,
		})
		if !errors.Is(err, repo.ErrInvalidTransactionShape) {
			t.Fatalf("want ErrInvalidTransactionShape, got %v", err)
		}
	})

	t.Run("valid bond maturity", func(t *testing.T) {
		err := repo.ValidateSeedTransaction("bond", repo.CreateInvestmentTransactionParams{
			TransactionType: "maturity", PrincipalAmount: &amt, InterestAmount: &amt,
			PrincipalDisposition: ptrStrLit("cash_out"), InterestDisposition: ptrStrLit("rolled_to_new"),
		})
		if err != nil {
			t.Fatalf("want nil, got %v", err)
		}
	})
}
