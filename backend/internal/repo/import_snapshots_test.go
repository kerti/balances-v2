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

// TestAssetRepo_ImportAssetSnapshots covers the bulk-import path: dry-run writes
// nothing, commit inserts, re-import upserts by month (last-write-wins, no
// duplicate), and a cross-household import is rejected as ErrNotFound.
//
// covers: INV-SNAPSHOTS-04
func TestAssetRepo_ImportAssetSnapshots(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)

	alice := testutil.CreateHouseholdWithUser(t, q, "Alice")
	bob := testutil.CreateHouseholdWithUser(t, q, "Bob")
	aliceCtx := auth.WithUser(context.Background(), alice)
	bobCtx := auth.WithUser(context.Background(), bob)

	r := repo.NewAssetRepo(tdb.Pool)

	acct, err := r.CreateBankAccount(aliceCtx, repo.CreateBankAccountParams{
		DisplayName:    "Alice BCA",
		OwnershipType:  "joint",
		NativeCurrency: "IDR",
		BankName:       "BCA",
		AccountNumber:  "111",
		AccountType:    "savings",
	})
	if err != nil {
		t.Fatalf("CreateBankAccount: %v", err)
	}
	aid := acct.Asset.ID

	ym := func(y int, m time.Month) time.Time {
		return time.Date(y, m, 1, 0, 0, 0, 0, time.UTC)
	}
	rows := []repo.ImportSnapshotRow{
		{YearMonth: ym(2015, time.January), Amount: decimal.NewFromInt(100), Currency: "IDR"},
		{YearMonth: ym(2015, time.February), Amount: decimal.NewFromInt(200), Currency: "IDR"},
	}

	t.Run("dry-run classifies but writes nothing", func(t *testing.T) {
		res, err := r.ImportAssetSnapshots(aliceCtx, aid, rows, true)
		if err != nil {
			t.Fatalf("dry-run: %v", err)
		}
		if res.ToInsert != 2 || res.ToUpdate != 0 {
			t.Errorf("counts = %d/%d, want 2/0", res.ToInsert, res.ToUpdate)
		}
		snaps, _ := r.ListAssetSnapshots(aliceCtx, aid)
		if len(snaps) != 0 {
			t.Errorf("dry-run wrote %d snapshots; want 0", len(snaps))
		}
	})

	t.Run("commit inserts all rows", func(t *testing.T) {
		res, err := r.ImportAssetSnapshots(aliceCtx, aid, rows, false)
		if err != nil {
			t.Fatalf("commit: %v", err)
		}
		if res.ToInsert != 2 || res.ToUpdate != 0 {
			t.Errorf("counts = %d/%d, want 2/0", res.ToInsert, res.ToUpdate)
		}
		snaps, _ := r.ListAssetSnapshots(aliceCtx, aid)
		if len(snaps) != 2 {
			t.Fatalf("after commit got %d snapshots; want 2", len(snaps))
		}
	})

	t.Run("re-import upserts by month, last-write-wins", func(t *testing.T) {
		updated := []repo.ImportSnapshotRow{
			{YearMonth: ym(2015, time.January), Amount: decimal.NewFromInt(999), Currency: "IDR"}, // overwrite
			{YearMonth: ym(2015, time.March), Amount: decimal.NewFromInt(300), Currency: "IDR"},   // new
		}
		res, err := r.ImportAssetSnapshots(aliceCtx, aid, updated, false)
		if err != nil {
			t.Fatalf("re-import: %v", err)
		}
		if res.ToInsert != 1 || res.ToUpdate != 1 {
			t.Errorf("counts = %d/%d, want 1/1", res.ToInsert, res.ToUpdate)
		}
		snaps, _ := r.ListAssetSnapshots(aliceCtx, aid)
		if len(snaps) != 3 {
			t.Fatalf("got %d snapshots; want 3 (Jan/Feb/Mar, no dup)", len(snaps))
		}
		for _, s := range snaps {
			if s.YearMonth.Equal(ym(2015, time.January)) && !s.Amount.Equal(decimal.NewFromInt(999)) {
				t.Errorf("Jan amount = %s, want 999 (overwritten)", s.Amount)
			}
		}
	})

	t.Run("bob cannot import into alice's asset", func(t *testing.T) {
		_, err := r.ImportAssetSnapshots(bobCtx, aid, rows, false)
		if !errors.Is(err, repo.ErrNotFound) {
			t.Errorf("bob import err = %v, want ErrNotFound", err)
		}
		snaps, _ := r.ListAssetSnapshots(aliceCtx, aid)
		if len(snaps) != 3 {
			t.Errorf("after bob's attempt got %d snapshots; want 3 unchanged", len(snaps))
		}
	})
}

// TestLiabilityRepo_ImportLiabilitySnapshots mirrors the asset import coverage
// for the Liability group (dry-run / commit / upsert-by-month / tenancy).
// covers: INV-SNAPSHOTS-04
func TestLiabilityRepo_ImportLiabilitySnapshots(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)

	alice := testutil.CreateHouseholdWithUser(t, q, "Alice")
	bob := testutil.CreateHouseholdWithUser(t, q, "Bob")
	aliceCtx := auth.WithUser(context.Background(), alice)
	bobCtx := auth.WithUser(context.Background(), bob)

	r := repo.NewLiabilityRepo(tdb.Pool)

	liab, err := r.CreateLiability(aliceCtx, repo.CreateLiabilityParams{
		DisplayName:      "Alice Car Loan",
		Subtype:          "personal",
		OwnershipType:    "joint",
		NativeCurrency:   "IDR",
		CounterpartyName: "Uncle Bob",
	})
	if err != nil {
		t.Fatalf("CreateLiability: %v", err)
	}
	lid := liab.ID

	ym := func(y int, m time.Month) time.Time {
		return time.Date(y, m, 1, 0, 0, 0, 0, time.UTC)
	}
	rows := []repo.ImportSnapshotRow{
		{YearMonth: ym(2015, time.January), Amount: decimal.NewFromInt(100), Currency: "IDR"},
		{YearMonth: ym(2015, time.February), Amount: decimal.NewFromInt(200), Currency: "IDR"},
	}

	t.Run("dry-run classifies but writes nothing", func(t *testing.T) {
		res, err := r.ImportLiabilitySnapshots(aliceCtx, lid, rows, true)
		if err != nil {
			t.Fatalf("dry-run: %v", err)
		}
		if res.ToInsert != 2 || res.ToUpdate != 0 {
			t.Errorf("counts = %d/%d, want 2/0", res.ToInsert, res.ToUpdate)
		}
		snaps, _ := r.ListLiabilitySnapshots(aliceCtx, lid)
		if len(snaps) != 0 {
			t.Errorf("dry-run wrote %d snapshots; want 0", len(snaps))
		}
	})

	t.Run("commit inserts all rows", func(t *testing.T) {
		res, err := r.ImportLiabilitySnapshots(aliceCtx, lid, rows, false)
		if err != nil {
			t.Fatalf("commit: %v", err)
		}
		if res.ToInsert != 2 || res.ToUpdate != 0 {
			t.Errorf("counts = %d/%d, want 2/0", res.ToInsert, res.ToUpdate)
		}
		snaps, _ := r.ListLiabilitySnapshots(aliceCtx, lid)
		if len(snaps) != 2 {
			t.Fatalf("after commit got %d snapshots; want 2", len(snaps))
		}
	})

	t.Run("re-import upserts by month, last-write-wins", func(t *testing.T) {
		updated := []repo.ImportSnapshotRow{
			{YearMonth: ym(2015, time.January), Amount: decimal.NewFromInt(999), Currency: "IDR"},
			{YearMonth: ym(2015, time.March), Amount: decimal.NewFromInt(300), Currency: "IDR"},
		}
		res, err := r.ImportLiabilitySnapshots(aliceCtx, lid, updated, false)
		if err != nil {
			t.Fatalf("re-import: %v", err)
		}
		if res.ToInsert != 1 || res.ToUpdate != 1 {
			t.Errorf("counts = %d/%d, want 1/1", res.ToInsert, res.ToUpdate)
		}
		snaps, _ := r.ListLiabilitySnapshots(aliceCtx, lid)
		if len(snaps) != 3 {
			t.Fatalf("got %d snapshots; want 3 (Jan/Feb/Mar, no dup)", len(snaps))
		}
		for _, s := range snaps {
			if s.YearMonth.Equal(ym(2015, time.January)) && !s.Amount.Equal(decimal.NewFromInt(999)) {
				t.Errorf("Jan amount = %s, want 999 (overwritten)", s.Amount)
			}
		}
	})

	t.Run("bob cannot import into alice's liability", func(t *testing.T) {
		_, err := r.ImportLiabilitySnapshots(bobCtx, lid, rows, false)
		if !errors.Is(err, repo.ErrNotFound) {
			t.Errorf("bob import err = %v, want ErrNotFound", err)
		}
		snaps, _ := r.ListLiabilitySnapshots(aliceCtx, lid)
		if len(snaps) != 3 {
			t.Errorf("after bob's attempt got %d snapshots; want 3 unchanged", len(snaps))
		}
	})
}

// TestReceivableRepo_ImportReceivableSnapshots mirrors the asset import coverage
// for the Receivable group.
// covers: INV-SNAPSHOTS-04
func TestReceivableRepo_ImportReceivableSnapshots(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)

	alice := testutil.CreateHouseholdWithUser(t, q, "Alice")
	bob := testutil.CreateHouseholdWithUser(t, q, "Bob")
	aliceCtx := auth.WithUser(context.Background(), alice)
	bobCtx := auth.WithUser(context.Background(), bob)

	r := repo.NewReceivableRepo(tdb.Pool)

	rec, err := r.CreateReceivable(aliceCtx, repo.CreateReceivableParams{
		DisplayName:      "Loan to Carol",
		OwnershipType:    "joint",
		NativeCurrency:   "IDR",
		CounterpartyName: "Carol",
	})
	if err != nil {
		t.Fatalf("CreateReceivable: %v", err)
	}
	rid := rec.ID

	ym := func(y int, m time.Month) time.Time {
		return time.Date(y, m, 1, 0, 0, 0, 0, time.UTC)
	}
	rows := []repo.ImportSnapshotRow{
		{YearMonth: ym(2015, time.January), Amount: decimal.NewFromInt(100), Currency: "IDR"},
		{YearMonth: ym(2015, time.February), Amount: decimal.NewFromInt(200), Currency: "IDR"},
	}

	t.Run("dry-run classifies but writes nothing", func(t *testing.T) {
		res, err := r.ImportReceivableSnapshots(aliceCtx, rid, rows, true)
		if err != nil {
			t.Fatalf("dry-run: %v", err)
		}
		if res.ToInsert != 2 || res.ToUpdate != 0 {
			t.Errorf("counts = %d/%d, want 2/0", res.ToInsert, res.ToUpdate)
		}
		snaps, _ := r.ListReceivableSnapshots(aliceCtx, rid)
		if len(snaps) != 0 {
			t.Errorf("dry-run wrote %d snapshots; want 0", len(snaps))
		}
	})

	t.Run("commit inserts all rows", func(t *testing.T) {
		res, err := r.ImportReceivableSnapshots(aliceCtx, rid, rows, false)
		if err != nil {
			t.Fatalf("commit: %v", err)
		}
		if res.ToInsert != 2 || res.ToUpdate != 0 {
			t.Errorf("counts = %d/%d, want 2/0", res.ToInsert, res.ToUpdate)
		}
		snaps, _ := r.ListReceivableSnapshots(aliceCtx, rid)
		if len(snaps) != 2 {
			t.Fatalf("after commit got %d snapshots; want 2", len(snaps))
		}
	})

	t.Run("re-import upserts by month, last-write-wins", func(t *testing.T) {
		updated := []repo.ImportSnapshotRow{
			{YearMonth: ym(2015, time.January), Amount: decimal.NewFromInt(999), Currency: "IDR"},
			{YearMonth: ym(2015, time.March), Amount: decimal.NewFromInt(300), Currency: "IDR"},
		}
		res, err := r.ImportReceivableSnapshots(aliceCtx, rid, updated, false)
		if err != nil {
			t.Fatalf("re-import: %v", err)
		}
		if res.ToInsert != 1 || res.ToUpdate != 1 {
			t.Errorf("counts = %d/%d, want 1/1", res.ToInsert, res.ToUpdate)
		}
		snaps, _ := r.ListReceivableSnapshots(aliceCtx, rid)
		if len(snaps) != 3 {
			t.Fatalf("got %d snapshots; want 3 (Jan/Feb/Mar, no dup)", len(snaps))
		}
		for _, s := range snaps {
			if s.YearMonth.Equal(ym(2015, time.January)) && !s.Amount.Equal(decimal.NewFromInt(999)) {
				t.Errorf("Jan amount = %s, want 999 (overwritten)", s.Amount)
			}
		}
	})

	t.Run("bob cannot import into alice's receivable", func(t *testing.T) {
		_, err := r.ImportReceivableSnapshots(bobCtx, rid, rows, false)
		if !errors.Is(err, repo.ErrNotFound) {
			t.Errorf("bob import err = %v, want ErrNotFound", err)
		}
		snaps, _ := r.ListReceivableSnapshots(aliceCtx, rid)
		if len(snaps) != 3 {
			t.Errorf("after bob's attempt got %d snapshots; want 3 unchanged", len(snaps))
		}
	})
}

// TestInvestmentRepo_ImportInvestmentSnapshots covers both investment snapshot
// shapes: quantity-price (stock) and accrued-interest (bond), plus tenancy.
// covers: INV-SNAPSHOTS-04
func TestInvestmentRepo_ImportInvestmentSnapshots(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)

	alice := testutil.CreateHouseholdWithUser(t, q, "Alice")
	bob := testutil.CreateHouseholdWithUser(t, q, "Bob")
	aliceCtx := auth.WithUser(context.Background(), alice)
	bobCtx := auth.WithUser(context.Background(), bob)

	r := repo.NewInvestmentRepo(tdb.Pool)

	ym := func(y int, m time.Month) time.Time {
		return time.Date(y, m, 1, 0, 0, 0, 0, time.UTC)
	}
	dec := func(s string) *decimal.Decimal { d, _ := decimal.NewFromString(s); return &d }

	t.Run("quantity-price shape (stock)", func(t *testing.T) {
		stock, err := r.CreateStock(aliceCtx, repo.CreateStockParams{
			DisplayName:    "BBCA",
			OwnershipType:  "joint",
			NativeCurrency: "IDR",
			RiskProfile:    "medium",
			Ticker:         "BBCA",
			Exchange:       "IDX",
		})
		if err != nil {
			t.Fatalf("CreateStock: %v", err)
		}
		id := stock.Investment.ID
		rows := []repo.ImportInvestmentSnapshotRow{
			{YearMonth: ym(2015, time.January), Amount: decimal.NewFromInt(850000), Currency: "IDR", Quantity: dec("100"), PricePerUnit: dec("8500")},
			{YearMonth: ym(2015, time.February), Amount: decimal.NewFromInt(900000), Currency: "IDR", Quantity: dec("100"), PricePerUnit: dec("9000")},
		}

		res, err := r.ImportInvestmentSnapshots(aliceCtx, id, rows, true)
		if err != nil {
			t.Fatalf("dry-run: %v", err)
		}
		if res.ToInsert != 2 || res.ToUpdate != 0 {
			t.Errorf("dry-run counts = %d/%d, want 2/0", res.ToInsert, res.ToUpdate)
		}
		if snaps, _ := r.ListInvestmentSnapshots(aliceCtx, id); len(snaps) != 0 {
			t.Errorf("dry-run wrote %d; want 0", len(snaps))
		}

		if _, err := r.ImportInvestmentSnapshots(aliceCtx, id, rows, false); err != nil {
			t.Fatalf("commit: %v", err)
		}
		snaps, _ := r.ListInvestmentSnapshots(aliceCtx, id)
		if len(snaps) != 2 {
			t.Fatalf("after commit got %d; want 2", len(snaps))
		}
		// Shape persisted: quantity + price set, accrued nil.
		for _, s := range snaps {
			if s.Quantity == nil || s.PricePerUnit == nil || s.AccruedInterest != nil {
				t.Errorf("stock snapshot shape wrong: qty=%v price=%v accrued=%v", s.Quantity, s.PricePerUnit, s.AccruedInterest)
			}
		}

		// Re-import overwrites Jan (last-write-wins), adds Mar.
		updated := []repo.ImportInvestmentSnapshotRow{
			{YearMonth: ym(2015, time.January), Amount: decimal.NewFromInt(999000), Currency: "IDR", Quantity: dec("100"), PricePerUnit: dec("9990")},
			{YearMonth: ym(2015, time.March), Amount: decimal.NewFromInt(1000000), Currency: "IDR", Quantity: dec("100"), PricePerUnit: dec("10000")},
		}
		res, err = r.ImportInvestmentSnapshots(aliceCtx, id, updated, false)
		if err != nil {
			t.Fatalf("re-import: %v", err)
		}
		if res.ToInsert != 1 || res.ToUpdate != 1 {
			t.Errorf("re-import counts = %d/%d, want 1/1", res.ToInsert, res.ToUpdate)
		}
		snaps, _ = r.ListInvestmentSnapshots(aliceCtx, id)
		if len(snaps) != 3 {
			t.Fatalf("got %d; want 3 (no dup)", len(snaps))
		}

		if _, err := r.ImportInvestmentSnapshots(bobCtx, id, rows, false); !errors.Is(err, repo.ErrNotFound) {
			t.Errorf("bob import err = %v, want ErrNotFound", err)
		}
	})

	t.Run("accrued-interest shape (bond)", func(t *testing.T) {
		couponRate, _ := decimal.NewFromString("6.25")
		bond, err := r.CreateBond(aliceCtx, repo.CreateBondParams{
			DisplayName:     "ORI024",
			OwnershipType:   "joint",
			NativeCurrency:  "IDR",
			RiskProfile:     "medium",
			BondType:        "govt_primary",
			Issuer:          "Republik Indonesia",
			FaceValue:       decPtr(decimal.NewFromInt(10_000_000)),
			CouponRate:      couponRate,
			CouponFrequency: "monthly",
			MaturityDate:    time.Date(2029, time.October, 15, 0, 0, 0, 0, time.UTC),
		})
		if err != nil {
			t.Fatalf("CreateBond: %v", err)
		}
		id := bond.Investment.ID
		rows := []repo.ImportInvestmentSnapshotRow{
			{YearMonth: ym(2015, time.January), Amount: decimal.NewFromInt(10_250_000), Currency: "IDR", AccruedInterest: dec("250000")},
			{YearMonth: ym(2015, time.February), Amount: decimal.NewFromInt(10_000_000), Currency: "IDR", AccruedInterest: dec("0")},
		}

		if _, err := r.ImportInvestmentSnapshots(aliceCtx, id, rows, false); err != nil {
			t.Fatalf("commit: %v", err)
		}
		snaps, _ := r.ListInvestmentSnapshots(aliceCtx, id)
		if len(snaps) != 2 {
			t.Fatalf("after commit got %d; want 2", len(snaps))
		}
		for _, s := range snaps {
			if s.AccruedInterest == nil || s.Quantity != nil || s.PricePerUnit != nil {
				t.Errorf("bond snapshot shape wrong: accrued=%v qty=%v price=%v", s.AccruedInterest, s.Quantity, s.PricePerUnit)
			}
		}
	})

	t.Run("wrong shape rejected (accrued row on a stock)", func(t *testing.T) {
		stock, err := r.CreateStock(aliceCtx, repo.CreateStockParams{
			DisplayName: "TLKM", OwnershipType: "joint", NativeCurrency: "IDR", Ticker: "TLKM", Exchange: "IDX",
			RiskProfile: "medium",
		})
		if err != nil {
			t.Fatalf("CreateStock: %v", err)
		}
		bad := []repo.ImportInvestmentSnapshotRow{
			{YearMonth: ym(2016, time.January), Amount: decimal.NewFromInt(100), Currency: "IDR", AccruedInterest: dec("5")},
		}
		if _, err := r.ImportInvestmentSnapshots(aliceCtx, stock.Investment.ID, bad, false); !errors.Is(err, repo.ErrInvalidSnapshotShape) {
			t.Errorf("err = %v, want ErrInvalidSnapshotShape", err)
		}
		if snaps, _ := r.ListInvestmentSnapshots(aliceCtx, stock.Investment.ID); len(snaps) != 0 {
			t.Errorf("rejected import still wrote %d snapshots; want 0", len(snaps))
		}
	})
}
