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

// The bulk monthly-entry repo layer (ADR-0046) is a per-table parallel
// implementation across the five groups. These integration tests drive each
// group's ListXEntryRows (carry-forward prefill assembly) and
// BulkUpsertXSnapshots (dirty-only atomic upsert, re-entry overwrite, and the
// ineligible-row rejection that writes nothing) against a real DB — the repo
// paths the handler tests exercise at runtime but Go's per-package coverage
// does not attribute back here.

var (
	priorMonth  = time.Date(2026, time.May, 1, 0, 0, 0, 0, time.UTC)
	targetMonth = time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC)
)

func di(n int64) decimal.Decimal { return decimal.NewFromInt(n) }

// covers: INV-SNAPSHOTS-06
// covers: INV-SNAPSHOTS-07
func TestAssetRepo_BulkMonthlyEntry(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)
	user := testutil.CreateHouseholdWithUser(t, q, "Alice")
	ctx := identity.WithUser(context.Background(), user)
	r := repo.NewAssetRepo(tdb.Pool)

	acct, err := r.CreateBankAccount(ctx, repo.CreateBankAccountParams{
		DisplayName:    "Alice Savings",
		OwnershipType:  "joint",
		NativeCurrency: "IDR",
		BankName:       "BCA",
		AccountNumber:  "111",
		AccountType:    "savings",
	})
	if err != nil {
		t.Fatalf("CreateBankAccount: %v", err)
	}
	id := acct.Asset.ID

	// No history yet: one eligible row, nil prefill.
	rows, err := r.ListAssetEntryRows(ctx, targetMonth)
	if err != nil {
		t.Fatalf("ListAssetEntryRows: %v", err)
	}
	if len(rows) != 1 || rows[0].AssetID != id {
		t.Fatalf("entry rows: want 1 for %s, got %+v", id, rows)
	}
	if rows[0].PrefillAmount != nil {
		t.Errorf("prefill should be nil with no history, got %v", rows[0].PrefillAmount)
	}

	// A prior-month snapshot carries forward as the prefill.
	if _, err := r.CreateAssetSnapshot(ctx, repo.CreateAssetSnapshotParams{
		AssetID: id, YearMonth: priorMonth, Amount: di(1_000_000), Currency: "IDR",
	}); err != nil {
		t.Fatalf("CreateAssetSnapshot: %v", err)
	}
	rows, _ = r.ListAssetEntryRows(ctx, targetMonth)
	if rows[0].PrefillAmount == nil || !rows[0].PrefillAmount.Equal(di(1_000_000)) {
		t.Fatalf("carry-forward prefill: want 1000000, got %v", rows[0].PrefillAmount)
	}
	if rows[0].CarriedFrom == nil || !rows[0].CarriedFrom.Equal(priorMonth) {
		t.Errorf("carried-from: want %s, got %v", priorMonth, rows[0].CarriedFrom)
	}

	// Happy bulk save: one dirty row, written and committed.
	n, rowErrs, err := r.BulkUpsertAssetSnapshots(ctx, repo.BulkUpsertAssetSnapshotsParams{
		YearMonth: targetMonth,
		Rows:      []repo.BulkAssetSnapshotRow{{AssetID: id, Amount: di(1_200_000), Currency: "IDR"}},
	})
	if err != nil || len(rowErrs) != 0 || n != 1 {
		t.Fatalf("bulk upsert: want (1, [], nil), got (%d, %+v, %v)", n, rowErrs, err)
	}
	rows, _ = r.ListAssetEntryRows(ctx, targetMonth)
	if !rows[0].PrefillAmount.Equal(di(1_200_000)) || !rows[0].CarriedFrom.Equal(targetMonth) {
		t.Errorf("post-write prefill: want 1200000@%s, got %v@%v", targetMonth, rows[0].PrefillAmount, rows[0].CarriedFrom)
	}

	// Re-entry upserts in place rather than duplicating.
	n, _, err = r.BulkUpsertAssetSnapshots(ctx, repo.BulkUpsertAssetSnapshotsParams{
		YearMonth: targetMonth,
		Rows:      []repo.BulkAssetSnapshotRow{{AssetID: id, Amount: di(1_300_000), Currency: "IDR"}},
	})
	if err != nil || n != 1 {
		t.Fatalf("re-entry upsert: want (1, nil), got (%d, %v)", n, err)
	}
	rows, _ = r.ListAssetEntryRows(ctx, targetMonth)
	if !rows[0].PrefillAmount.Equal(di(1_300_000)) {
		t.Errorf("re-entry overwrite: want 1300000, got %v", rows[0].PrefillAmount)
	}

	// An ineligible row (unknown id) rejects the whole batch, nothing written.
	n, rowErrs, err = r.BulkUpsertAssetSnapshots(ctx, repo.BulkUpsertAssetSnapshotsParams{
		YearMonth: targetMonth,
		Rows:      []repo.BulkAssetSnapshotRow{{AssetID: uuid.New(), Amount: di(1), Currency: "IDR"}},
	})
	if err != nil || n != 0 || len(rowErrs) != 1 || rowErrs[0].Reason != repo.BulkRowIneligible {
		t.Fatalf("ineligible row: want (0, [ineligible], nil), got (%d, %+v, %v)", n, rowErrs, err)
	}

	// Empty batch is a no-op.
	if n, _, err := r.BulkUpsertAssetSnapshots(ctx, repo.BulkUpsertAssetSnapshotsParams{YearMonth: targetMonth}); err != nil || n != 0 {
		t.Errorf("empty batch: want (0, nil), got (%d, %v)", n, err)
	}
}

// covers: INV-SNAPSHOTS-06
// covers: INV-SNAPSHOTS-07
func TestLiabilityRepo_BulkMonthlyEntry(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)
	user := testutil.CreateHouseholdWithUser(t, q, "Alice")
	ctx := identity.WithUser(context.Background(), user)
	r := repo.NewLiabilityRepo(tdb.Pool)

	liab, err := r.CreateLiability(ctx, repo.CreateLiabilityParams{
		DisplayName:      "Mortgage",
		Subtype:          "institutional",
		OwnershipType:    "joint",
		NativeCurrency:   "IDR",
		CounterpartyName: "Bank",
	})
	if err != nil {
		t.Fatalf("CreateLiability: %v", err)
	}
	id := liab.ID

	rows, err := r.ListLiabilityEntryRows(ctx, targetMonth)
	if err != nil || len(rows) != 1 || rows[0].LiabilityID != id {
		t.Fatalf("entry rows: want 1 for %s, got %+v (err %v)", id, rows, err)
	}
	if rows[0].PrefillAmount != nil {
		t.Errorf("prefill should be nil with no history, got %v", rows[0].PrefillAmount)
	}

	if _, err := r.CreateLiabilitySnapshot(ctx, repo.CreateLiabilitySnapshotParams{
		LiabilityID: id, YearMonth: priorMonth, Amount: di(500_000_000), Currency: "IDR",
	}); err != nil {
		t.Fatalf("CreateLiabilitySnapshot: %v", err)
	}
	rows, _ = r.ListLiabilityEntryRows(ctx, targetMonth)
	if rows[0].PrefillAmount == nil || !rows[0].PrefillAmount.Equal(di(500_000_000)) {
		t.Fatalf("carry-forward prefill: want 500000000, got %v", rows[0].PrefillAmount)
	}

	n, rowErrs, err := r.BulkUpsertLiabilitySnapshots(ctx, repo.BulkUpsertLiabilitySnapshotsParams{
		YearMonth: targetMonth,
		Rows:      []repo.BulkLiabilitySnapshotRow{{LiabilityID: id, Amount: di(495_000_000), Currency: "IDR"}},
	})
	if err != nil || len(rowErrs) != 0 || n != 1 {
		t.Fatalf("bulk upsert: want (1, [], nil), got (%d, %+v, %v)", n, rowErrs, err)
	}
	rows, _ = r.ListLiabilityEntryRows(ctx, targetMonth)
	if !rows[0].PrefillAmount.Equal(di(495_000_000)) {
		t.Errorf("post-write prefill: want 495000000, got %v", rows[0].PrefillAmount)
	}

	// Re-entry overwrites.
	if _, _, err := r.BulkUpsertLiabilitySnapshots(ctx, repo.BulkUpsertLiabilitySnapshotsParams{
		YearMonth: targetMonth,
		Rows:      []repo.BulkLiabilitySnapshotRow{{LiabilityID: id, Amount: di(490_000_000), Currency: "IDR"}},
	}); err != nil {
		t.Fatalf("re-entry: %v", err)
	}
	rows, _ = r.ListLiabilityEntryRows(ctx, targetMonth)
	if !rows[0].PrefillAmount.Equal(di(490_000_000)) {
		t.Errorf("re-entry overwrite: want 490000000, got %v", rows[0].PrefillAmount)
	}

	n, rowErrs, err = r.BulkUpsertLiabilitySnapshots(ctx, repo.BulkUpsertLiabilitySnapshotsParams{
		YearMonth: targetMonth,
		Rows:      []repo.BulkLiabilitySnapshotRow{{LiabilityID: uuid.New(), Amount: di(1), Currency: "IDR"}},
	})
	if err != nil || n != 0 || len(rowErrs) != 1 || rowErrs[0].Reason != repo.BulkRowIneligible {
		t.Fatalf("ineligible row: want (0, [ineligible], nil), got (%d, %+v, %v)", n, rowErrs, err)
	}
}

// covers: INV-SNAPSHOTS-06
// covers: INV-SNAPSHOTS-07
func TestReceivableRepo_BulkMonthlyEntry(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)
	user := testutil.CreateHouseholdWithUser(t, q, "Alice")
	ctx := identity.WithUser(context.Background(), user)
	r := repo.NewReceivableRepo(tdb.Pool)

	recv, err := r.CreateReceivable(ctx, repo.CreateReceivableParams{
		DisplayName:      "Loan to Bob",
		OwnershipType:    "joint",
		NativeCurrency:   "IDR",
		CounterpartyName: "Bob",
	})
	if err != nil {
		t.Fatalf("CreateReceivable: %v", err)
	}
	id := recv.ID

	rows, err := r.ListReceivableEntryRows(ctx, targetMonth)
	if err != nil || len(rows) != 1 || rows[0].ReceivableID != id {
		t.Fatalf("entry rows: want 1 for %s, got %+v (err %v)", id, rows, err)
	}

	if _, err := r.CreateReceivableSnapshot(ctx, repo.CreateReceivableSnapshotParams{
		ReceivableID: id, YearMonth: priorMonth, Amount: di(5_000_000), Currency: "IDR",
	}); err != nil {
		t.Fatalf("CreateReceivableSnapshot: %v", err)
	}
	rows, _ = r.ListReceivableEntryRows(ctx, targetMonth)
	if rows[0].PrefillAmount == nil || !rows[0].PrefillAmount.Equal(di(5_000_000)) {
		t.Fatalf("carry-forward prefill: want 5000000, got %v", rows[0].PrefillAmount)
	}

	n, rowErrs, err := r.BulkUpsertReceivableSnapshots(ctx, repo.BulkUpsertReceivableSnapshotsParams{
		YearMonth: targetMonth,
		Rows:      []repo.BulkReceivableSnapshotRow{{ReceivableID: id, Amount: di(4_500_000), Currency: "IDR"}},
	})
	if err != nil || len(rowErrs) != 0 || n != 1 {
		t.Fatalf("bulk upsert: want (1, [], nil), got (%d, %+v, %v)", n, rowErrs, err)
	}
	rows, _ = r.ListReceivableEntryRows(ctx, targetMonth)
	if !rows[0].PrefillAmount.Equal(di(4_500_000)) {
		t.Errorf("post-write prefill: want 4500000, got %v", rows[0].PrefillAmount)
	}

	n, rowErrs, err = r.BulkUpsertReceivableSnapshots(ctx, repo.BulkUpsertReceivableSnapshotsParams{
		YearMonth: targetMonth,
		Rows:      []repo.BulkReceivableSnapshotRow{{ReceivableID: uuid.New(), Amount: di(1), Currency: "IDR"}},
	})
	if err != nil || n != 0 || len(rowErrs) != 1 || rowErrs[0].Reason != repo.BulkRowIneligible {
		t.Fatalf("ineligible row: want (0, [ineligible], nil), got (%d, %+v, %v)", n, rowErrs, err)
	}
}

// covers: INV-SNAPSHOTS-08
func TestInvestmentRepo_BulkMonthlyEntry_QtyPrice(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)
	user := testutil.CreateHouseholdWithUser(t, q, "Alice")
	ctx := identity.WithUser(context.Background(), user)
	r := repo.NewInvestmentRepo(tdb.Pool)

	stock, err := r.CreateStock(ctx, repo.CreateStockParams{
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

	rows, err := r.ListInvestmentEntryRows(ctx, targetMonth)
	if err != nil || len(rows) != 1 || rows[0].InvestmentID != id {
		t.Fatalf("entry rows: want 1 for %s, got %+v (err %v)", id, rows, err)
	}
	if rows[0].PrefillQuantity != nil {
		t.Errorf("prefill quantity should be nil with no history, got %v", rows[0].PrefillQuantity)
	}

	qty, price := di(100), di(9_500)
	if _, err := r.CreateInvestmentSnapshot(ctx, repo.CreateInvestmentSnapshotParams{
		InvestmentID: id, YearMonth: priorMonth, Amount: di(950_000), Currency: "IDR",
		Quantity: &qty, PricePerUnit: &price,
	}); err != nil {
		t.Fatalf("CreateInvestmentSnapshot: %v", err)
	}
	rows, _ = r.ListInvestmentEntryRows(ctx, targetMonth)
	if rows[0].PrefillQuantity == nil || !rows[0].PrefillQuantity.Equal(qty) ||
		rows[0].PrefillPrice == nil || !rows[0].PrefillPrice.Equal(price) {
		t.Fatalf("carry-forward prefill: want qty 100 @ 9500, got %v @ %v", rows[0].PrefillQuantity, rows[0].PrefillPrice)
	}

	// Happy save: amount derived server-side from qty × price.
	n, rowErrs, err := r.BulkUpsertInvestmentSnapshots(ctx, repo.BulkUpsertInvestmentSnapshotsParams{
		YearMonth: targetMonth,
		Rows:      []repo.BulkInvestmentSnapshotRow{{InvestmentID: id, Quantity: di(120), PricePerUnit: di(10_000), Currency: "IDR"}},
	})
	if err != nil || len(rowErrs) != 0 || n != 1 {
		t.Fatalf("bulk upsert: want (1, [], nil), got (%d, %+v, %v)", n, rowErrs, err)
	}
	rows, _ = r.ListInvestmentEntryRows(ctx, targetMonth)
	if !rows[0].PrefillQuantity.Equal(di(120)) || !rows[0].PrefillPrice.Equal(di(10_000)) {
		t.Errorf("post-write prefill: want 120 @ 10000, got %v @ %v", rows[0].PrefillQuantity, rows[0].PrefillPrice)
	}

	n, rowErrs, err = r.BulkUpsertInvestmentSnapshots(ctx, repo.BulkUpsertInvestmentSnapshotsParams{
		YearMonth: targetMonth,
		Rows:      []repo.BulkInvestmentSnapshotRow{{InvestmentID: uuid.New(), Quantity: di(1), PricePerUnit: di(1), Currency: "IDR"}},
	})
	if err != nil || n != 0 || len(rowErrs) != 1 || rowErrs[0].Reason != repo.BulkRowIneligible {
		t.Fatalf("ineligible row: want (0, [ineligible], nil), got (%d, %+v, %v)", n, rowErrs, err)
	}
}

// covers: INV-SNAPSHOTS-09
func TestInvestmentRepo_BulkMonthlyEntry_Accrued(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)
	user := testutil.CreateHouseholdWithUser(t, q, "Alice")
	ctx := identity.WithUser(context.Background(), user)
	r := repo.NewInvestmentRepo(tdb.Pool)

	bond, err := r.CreateBond(ctx, repo.CreateBondParams{
		DisplayName:       "ORI024",
		OwnershipType:     "joint",
		NativeCurrency:    "IDR",
		RiskProfile:       "low",
		BondType:          "secondary_market",
		Issuer:            "Govt",
		CouponRate:        decimal.RequireFromString("0.06"),
		CouponFrequency:   "monthly",
		CouponDisposition: "accrues",
		MaturityDate:      time.Date(2030, time.January, 1, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("CreateBond: %v", err)
	}
	id := bond.Investment.ID

	rows, err := r.ListInvestmentAccruedEntryRows(ctx, targetMonth)
	if err != nil || len(rows) != 1 || rows[0].InvestmentID != id {
		t.Fatalf("entry rows: want 1 for %s, got %+v (err %v)", id, rows, err)
	}
	if rows[0].CouponDisposition == nil || *rows[0].CouponDisposition != "accrues" {
		t.Errorf("coupon disposition: want accrues, got %v", rows[0].CouponDisposition)
	}

	amt, accr := di(10_000_000), di(50_000)
	if _, err := r.CreateInvestmentSnapshot(ctx, repo.CreateInvestmentSnapshotParams{
		InvestmentID: id, YearMonth: priorMonth, Amount: amt, Currency: "IDR", AccruedInterest: &accr,
	}); err != nil {
		t.Fatalf("CreateInvestmentSnapshot: %v", err)
	}
	rows, _ = r.ListInvestmentAccruedEntryRows(ctx, targetMonth)
	if rows[0].PrefillAmount == nil || !rows[0].PrefillAmount.Equal(amt) ||
		rows[0].PrefillAccruedInterest == nil || !rows[0].PrefillAccruedInterest.Equal(accr) {
		t.Fatalf("carry-forward prefill: want 10000000 / 50000, got %v / %v", rows[0].PrefillAmount, rows[0].PrefillAccruedInterest)
	}

	n, rowErrs, err := r.BulkUpsertInvestmentAccruedSnapshots(ctx, repo.BulkUpsertInvestmentAccruedSnapshotsParams{
		YearMonth: targetMonth,
		Rows:      []repo.BulkInvestmentAccruedSnapshotRow{{InvestmentID: id, Amount: di(10_100_000), AccruedInterest: di(60_000), Currency: "IDR"}},
	})
	if err != nil || len(rowErrs) != 0 || n != 1 {
		t.Fatalf("bulk upsert: want (1, [], nil), got (%d, %+v, %v)", n, rowErrs, err)
	}
	rows, _ = r.ListInvestmentAccruedEntryRows(ctx, targetMonth)
	if !rows[0].PrefillAmount.Equal(di(10_100_000)) || !rows[0].PrefillAccruedInterest.Equal(di(60_000)) {
		t.Errorf("post-write prefill: want 10100000 / 60000, got %v / %v", rows[0].PrefillAmount, rows[0].PrefillAccruedInterest)
	}

	n, rowErrs, err = r.BulkUpsertInvestmentAccruedSnapshots(ctx, repo.BulkUpsertInvestmentAccruedSnapshotsParams{
		YearMonth: targetMonth,
		Rows:      []repo.BulkInvestmentAccruedSnapshotRow{{InvestmentID: uuid.New(), Amount: di(1), AccruedInterest: di(0), Currency: "IDR"}},
	})
	if err != nil || n != 0 || len(rowErrs) != 1 || rowErrs[0].Reason != repo.BulkRowIneligible {
		t.Fatalf("ineligible row: want (0, [ineligible], nil), got (%d, %+v, %v)", n, rowErrs, err)
	}
}
