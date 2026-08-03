package repo_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/kerti/balances-v2/backend/internal/db"
	"github.com/kerti/balances-v2/backend/internal/identity"
	"github.com/kerti/balances-v2/backend/internal/repo"
	"github.com/kerti/balances-v2/backend/internal/testutil"
)

// stockWithHolding builds a Stock carrying one January snapshot of qty×price,
// funded by a matching Buy so the ledger's held quantity agrees with it.
func stockWithHolding(ctx context.Context, t *testing.T, r *repo.InvestmentRepo, name string, qty, price decimal.Decimal) *repo.Stock {
	t.Helper()
	amount := qty.Mul(price)
	stock, err := r.CreateStockWithSnapshotsAndLedger(ctx, repo.CreateStockParams{
		DisplayName:    name,
		OwnershipType:  "joint",
		NativeCurrency: "IDR",
		RiskProfile:    "medium",
		Ticker:         "BBCA",
		Exchange:       "IDX",
	}, nil, []repo.ImportInvestmentSnapshotRow{
		{YearMonth: ym(2026, time.January), Amount: amount, Currency: "IDR", Quantity: &qty, PricePerUnit: &price},
	}, []repo.ImportTransactionRow{
		{
			TransactionType: "buy",
			TransactionDate: day(2026, time.January, 5),
			Currency:        "IDR",
			Amount:          &amount,
			Quantity:        &qty,
			PricePerUnit:    &price,
		},
	})
	if err != nil {
		t.Fatalf("CreateStockWithSnapshotsAndLedger: %v", err)
	}
	return stock
}

// TestUpdateInvestmentLifecycle_SettlementWritesSellTransaction is the
// capture-at-source guarantee of ADR-0052 §6: terminating an Investment through
// the lifecycle action writes its terminal Transaction in the SAME database
// transaction as the status flip, so the position can never be left holding
// nothing with no record of where its value went (issue #587).
//
// covers: INV-LIFECYCLE-08
func TestUpdateInvestmentLifecycle_SettlementWritesSellTransaction(t *testing.T) {
	r, ctx := investmentRepoFor(t)

	qty := decimal.RequireFromString("100")
	price := decimal.RequireFromString("9500")
	stock := stockWithHolding(ctx, t, r, "Settled stock", qty, price)
	id := stock.Investment.ID

	termDate := day(2026, time.March, 15)
	if _, err := r.UpdateInvestmentLifecycle(ctx, id, repo.LifecycleParams{
		Status:       "sold",
		TerminatedAt: &termDate,
	}, &repo.InvestmentSettlement{
		Quantity:     ptrDec("100"),
		PricePerUnit: ptrDec("11000"),
	}); err != nil {
		t.Fatalf("terminate with settlement: %v", err)
	}

	txns, err := r.ListInvestmentTransactions(ctx, id)
	if err != nil {
		t.Fatalf("ListInvestmentTransactions: %v", err)
	}
	var sells int
	for _, txn := range txns {
		if txn.TransactionType != "sell" {
			continue
		}
		sells++
		if got, want := txn.TransactionDate, termDate; !got.Equal(want) {
			t.Errorf("sell date: got %s, want %s", got.Format("2006-01-02"), want.Format("2006-01-02"))
		}
		// amount is derived, never taken from the caller: quantity × price is the
		// one figure the return formula reads as cash_in (ADR-0008).
		if want := decimal.RequireFromString("1100000"); txn.Amount == nil || !txn.Amount.Equal(want) {
			t.Errorf("sell amount: got %v, want %s", txn.Amount, want)
		}
	}
	if sells != 1 {
		t.Fatalf("got %d sell transactions, want exactly 1", sells)
	}
}

// TestUpdateInvestmentLifecycle_SettlementWritesMaturityTransaction pins the
// other half of the subtype matrix (ADR-0052 §6): a TimeDeposit has no Sell in
// its transaction matrix, so its settlement is a Maturity — and both legs are
// dispositioned cash_out, because a rollover links a successor Investment that
// only the Maturity dialog can create (issue #27).
//
// covers: INV-LIFECYCLE-08
func TestUpdateInvestmentLifecycle_SettlementWritesMaturityTransaction(t *testing.T) {
	r, ctx := investmentRepoFor(t)

	td, err := r.CreateTimeDepositWithSnapshotsAndLedger(ctx, repo.CreateTimeDepositParams{
		DisplayName:    "Settled deposit",
		OwnershipType:  "joint",
		NativeCurrency: "IDR",
		RiskProfile:    "low",
		BankName:       "Test Bank",
		Principal:      decimal.RequireFromString("100000000"),
		InterestRate:   decimal.RequireFromString("4.5"),
		TermMonths:     12,
		PlacementDate:  day(2026, time.January, 10),
		MaturityDate:   day(2027, time.January, 10),
		RolloverPolicy: "no_rollover",
	}, nil, nil, nil)
	if err != nil {
		t.Fatalf("CreateTimeDepositWithSnapshotsAndLedger: %v", err)
	}
	id := td.Investment.ID

	termDate := day(2027, time.January, 10)
	if _, err := r.UpdateInvestmentLifecycle(ctx, id, repo.LifecycleParams{
		Status:       "matured",
		TerminatedAt: &termDate,
	}, &repo.InvestmentSettlement{
		PrincipalAmount: ptrDec("100000000"),
		InterestAmount:  ptrDec("4500000"),
	}); err != nil {
		t.Fatalf("terminate with settlement: %v", err)
	}

	txns, err := r.ListInvestmentTransactions(ctx, id)
	if err != nil {
		t.Fatalf("ListInvestmentTransactions: %v", err)
	}
	if len(txns) != 1 {
		t.Fatalf("got %d transactions, want exactly 1 (the settling maturity)", len(txns))
	}
	m := txns[0]
	if m.TransactionType != "maturity" {
		t.Fatalf("settlement type: got %q, want maturity", m.TransactionType)
	}
	if want := decimal.RequireFromString("100000000"); m.PrincipalAmount == nil || !m.PrincipalAmount.Equal(want) {
		t.Errorf("principal_amount: got %v, want %s", m.PrincipalAmount, want)
	}
	if want := decimal.RequireFromString("4500000"); m.InterestAmount == nil || !m.InterestAmount.Equal(want) {
		t.Errorf("interest_amount: got %v, want %s", m.InterestAmount, want)
	}
	for label, d := range map[string]*string{
		"principal_disposition": m.PrincipalDisposition,
		"interest_disposition":  m.InterestDisposition,
	} {
		if d == nil || *d != "cash_out" {
			t.Errorf("%s: got %v, want cash_out", label, d)
		}
	}
}

// TestUpdateInvestmentLifecycle_SettlementRejectsUnsupportedPair guards the
// matrix from the raw API. The terminate dialog narrows its status dropdown to
// the settleable pairs, but the group-level lifecycle enum stays wide (a
// household may already hold a position on one of these statuses and must keep
// being able to edit it), so a direct caller can still ask for a combination no
// transaction can express. That must be a clean rejection, not a silent hole.
//
// covers: INV-LIFECYCLE-08
func TestUpdateInvestmentLifecycle_SettlementRejectsUnsupportedPair(t *testing.T) {
	r, ctx := investmentRepoFor(t)

	qty := decimal.RequireFromString("100")
	price := decimal.RequireFromString("9500")
	stock := stockWithHolding(ctx, t, r, "Unsettleable stock", qty, price)
	id := stock.Investment.ID

	termDate := day(2026, time.March, 15)
	// A Stock cannot mature — 'matured' is offered by the group-level enum but
	// has no transaction that expresses it.
	_, err := r.UpdateInvestmentLifecycle(ctx, id, repo.LifecycleParams{
		Status:       "matured",
		TerminatedAt: &termDate,
	}, &repo.InvestmentSettlement{
		PrincipalAmount: ptrDec("100"),
		InterestAmount:  ptrDec("0"),
	})
	if !errors.Is(err, repo.ErrInvalidLifecycle) {
		t.Fatalf("want ErrInvalidLifecycle, got %v", err)
	}

	// The rejection rolls back: the position is untouched, still active.
	inv, err := r.GetStock(ctx, id)
	if err != nil {
		t.Fatalf("GetStock: %v", err)
	}
	if inv.Investment.Status != "active" {
		t.Errorf("status after rejected settlement: got %q, want active", inv.Investment.Status)
	}
}

// TestUpdateInvestmentLifecycle_RefusesUnsettleableTerminalStatus closes the
// combination at the API, not just in the dialog (ADR-0052 §6). The group-level
// enum offers `sold` and `matured` to all five subtypes, but a Stock cannot
// mature and a TimeDeposit cannot be sold — no Transaction expresses either, so
// letting one be created would manufacture exactly the unsettleable position
// #587 exists to prevent, with no way to record where the money went.
//
// Refused with or without a settlement attached: it is the *status* that has no
// meaning for the subtype, not the payload.
//
// covers: INV-LIFECYCLE-08
func TestUpdateInvestmentLifecycle_RefusesUnsettleableTerminalStatus(t *testing.T) {
	r, ctx := investmentRepoFor(t)

	qty := decimal.RequireFromString("100")
	price := decimal.RequireFromString("9500")
	stock := stockWithHolding(ctx, t, r, "Never-maturing stock", qty, price)
	id := stock.Investment.ID
	termDate := day(2026, time.March, 15)

	_, err := r.UpdateInvestmentLifecycle(ctx, id, repo.LifecycleParams{
		Status: "matured", TerminatedAt: &termDate,
	}, nil)
	if !errors.Is(err, repo.ErrInvalidLifecycle) {
		t.Fatalf("bare flip to matured: want ErrInvalidLifecycle, got %v", err)
	}

	// The supported pair still goes through untouched.
	if _, err := r.UpdateInvestmentLifecycle(ctx, id, repo.LifecycleParams{
		Status: "sold", TerminatedAt: &termDate,
	}, nil); err != nil {
		t.Fatalf("flip to sold: %v", err)
	}

	// And reactivating is never blocked — it is the way back for a position that
	// arrived on an unsupported status via restore or import.
	if _, err := r.UpdateInvestmentLifecycle(ctx, id, repo.LifecycleParams{
		Status: "active", TerminatedAt: nil,
	}, nil); err != nil {
		t.Fatalf("reactivate: %v", err)
	}
}

// TestUpdateInvestmentLifecycle_AllowsEditingAnAlreadyUnsettleableStatus is the
// deliberate hole in the rule above. A restore (ADR-0036 writes rows directly)
// can land a position on a pair the matrix rejects; refusing every write to it
// would strand its date and note as uneditable. Only the *transition into* an
// unsupported pair is refused, never a re-assertion of one already recorded.
//
// covers: INV-LIFECYCLE-08
func TestUpdateInvestmentLifecycle_AllowsEditingAnAlreadyUnsettleableStatus(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)
	alice := testutil.CreateHouseholdWithUser(t, q, "Alice")
	r := repo.NewInvestmentRepo(tdb.Pool)
	ctx := identity.WithUser(context.Background(), alice)

	qty := decimal.RequireFromString("100")
	price := decimal.RequireFromString("9500")
	stock := stockWithHolding(ctx, t, r, "Restored oddity", qty, price)
	id := stock.Investment.ID

	// Simulate what a restore writes: straight to the row, bypassing the repo.
	if _, err := tdb.Pool.Exec(context.Background(),
		"UPDATE investments SET status = 'matured', terminated_at = $2 WHERE id = $1",
		id, day(2026, time.March, 15)); err != nil {
		t.Fatalf("seed unsettleable status: %v", err)
	}

	corrected := day(2026, time.March, 20)
	note := "corrected date"
	if _, err := r.UpdateInvestmentLifecycle(ctx, id, repo.LifecycleParams{
		Status: "matured", TerminatedAt: &corrected, TerminationNote: &note,
	}, nil); err != nil {
		t.Fatalf("editing an already-unsettleable position must stay possible: %v", err)
	}
}

// TestUpdateInvestmentLifecycle_SettlementIsAtomicWithTheFlip is the guarantee
// that made capture-at-source worth doing in the repo rather than as two calls
// from the dialog (ADR-0052 §6): the status flip and the terminal Transaction
// are one database transaction, so a failure after the Transaction is inserted
// still leaves the position active and the ledger clean. Two half-applied API
// calls could not promise that.
//
// The failure is injected with the #575 lock idiom: a well-formed flip cannot be
// made to fail on demand, but it can be made to *wait*. A second session holds
// the termination-month snapshot row, so the close-snapshot archive — which runs
// after the settlement insert — blocks until the caller's deadline aborts it.
//
// covers: INV-LIFECYCLE-08
func TestUpdateInvestmentLifecycle_SettlementIsAtomicWithTheFlip(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)
	alice := testutil.CreateHouseholdWithUser(t, q, "Alice")
	r := repo.NewInvestmentRepo(tdb.Pool)
	ctx := identity.WithUser(context.Background(), alice)

	qty := decimal.RequireFromString("100")
	price := decimal.RequireFromString("9500")
	amount := qty.Mul(price)
	stock, err := r.CreateStockWithSnapshotsAndLedger(ctx, repo.CreateStockParams{
		DisplayName:    "Atomic stock",
		OwnershipType:  "joint",
		NativeCurrency: "IDR",
		RiskProfile:    "medium",
		Ticker:         "BBCA",
		Exchange:       "IDX",
	}, nil, []repo.ImportInvestmentSnapshotRow{
		// A snapshot AT the termination month, so the close-snapshot pass has a
		// row to archive — that UPDATE is what the blocker below stalls.
		{YearMonth: ym(2026, time.March), Amount: amount, Currency: "IDR", Quantity: &qty, PricePerUnit: &price},
	}, nil)
	if err != nil {
		t.Fatalf("CreateStockWithSnapshotsAndLedger: %v", err)
	}
	id := stock.Investment.ID

	release := lockRow(t, tdb.Pool, "investment_snapshots",
		anyRowID(t, tdb.Pool, "investment_snapshots", "investment_id", id))
	defer release()

	deadlined, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()

	termDate := day(2026, time.March, 15)
	if _, err := r.UpdateInvestmentLifecycle(deadlined, id, repo.LifecycleParams{
		Status:       "sold",
		TerminatedAt: &termDate,
	}, &repo.InvestmentSettlement{
		Quantity:     ptrDec("100"),
		PricePerUnit: ptrDec("11000"),
	}); err == nil {
		t.Fatal("terminate under a held lock returned nil error; the blocking branch was never reached")
	}

	// The blocker still holds only the snapshot row, so these reads are free.
	// Both halves rolled back. Read raw: a repo getter would re-derive
	// visibility and could agree by accident.
	var status string
	if err := tdb.Pool.QueryRow(context.Background(),
		"SELECT status FROM investments WHERE id = $1", id).Scan(&status); err != nil {
		t.Fatalf("read status: %v", err)
	}
	if status != "active" {
		t.Errorf("status after rolled-back terminate: got %q, want active", status)
	}

	var sells int
	if err := tdb.Pool.QueryRow(context.Background(),
		"SELECT count(*) FROM investment_transactions WHERE investment_id = $1 AND transaction_type = 'sell'",
		id).Scan(&sells); err != nil {
		t.Fatalf("count sells: %v", err)
	}
	if sells != 0 {
		t.Errorf("settlement transactions after rolled-back terminate: got %d, want 0", sells)
	}
}

// TestUpdateInvestmentLifecycle_SettlementRejectsReassertion stops a second sale
// being booked when a termination is merely edited. The dialog only offers the
// settlement block on the active → terminal edge, and correcting a termination
// date or note afterwards must not re-run the capture.
//
// Deliberately NOT deduplicated by "a sale already exists this month": several
// partial Sells in one month are legitimate, so the position's own status is the
// only honest signal that a termination has already been settled.
//
// covers: INV-LIFECYCLE-08
func TestUpdateInvestmentLifecycle_SettlementRejectsReassertion(t *testing.T) {
	r, ctx := investmentRepoFor(t)

	qty := decimal.RequireFromString("100")
	price := decimal.RequireFromString("9500")
	stock := stockWithHolding(ctx, t, r, "Re-asserted stock", qty, price)
	id := stock.Investment.ID

	termDate := day(2026, time.March, 15)
	settlement := &repo.InvestmentSettlement{
		Quantity:     ptrDec("100"),
		PricePerUnit: ptrDec("11000"),
	}
	if _, err := r.UpdateInvestmentLifecycle(ctx, id, repo.LifecycleParams{
		Status: "sold", TerminatedAt: &termDate,
	}, settlement); err != nil {
		t.Fatalf("first terminate: %v", err)
	}

	// Correcting the date on an already-terminated position, settlement attached.
	corrected := day(2026, time.March, 20)
	_, err := r.UpdateInvestmentLifecycle(ctx, id, repo.LifecycleParams{
		Status: "sold", TerminatedAt: &corrected,
	}, settlement)
	if !errors.Is(err, repo.ErrPositionNotActive) {
		t.Fatalf("want ErrPositionNotActive, got %v", err)
	}

	txns, err := r.ListInvestmentTransactions(ctx, id)
	if err != nil {
		t.Fatalf("ListInvestmentTransactions: %v", err)
	}
	var sells int
	for _, txn := range txns {
		if txn.TransactionType == "sell" {
			sells++
		}
	}
	if sells != 1 {
		t.Fatalf("got %d sell transactions, want exactly 1", sells)
	}
}

// TestUpdateInvestmentLifecycle_WriteOffEscapeSettlesTheAdvisory is ADR-0052 §5
// end to end: repo write → engine → report row. An Investment that genuinely
// lost its value is terminated with a **0-proceeds** Sell — not with nothing —
// because a total loss is a truthful negative Investment Return, and booking the
// 0-proceeds Sell is what tells the report the value's fate is known.
//
// The control is the second position, terminated with no settlement at all: it
// must still trip #586's advisory in the same month, so the assertion proves the
// escape is what clears it rather than the advisory being broken.
//
// covers: INV-LIFECYCLE-08, INV-FINANCE-35
func TestUpdateInvestmentLifecycle_WriteOffEscapeSettlesTheAdvisory(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)
	alice := testutil.CreateHouseholdWithUser(t, q, "Alice")
	r := repo.NewInvestmentRepo(tdb.Pool)
	ctx := identity.WithUser(context.Background(), alice)

	qty := decimal.RequireFromString("100")
	price := decimal.RequireFromString("10")
	writtenOff := stockWithHolding(ctx, t, r, "Collapsed stock", qty, price)
	unsettled := stockWithHolding(ctx, t, r, "Silently gone stock", qty, price)

	termDate := day(2026, time.February, 20)
	// The escape: quantity still leaves the position, at a price of 0.
	if _, err := r.UpdateInvestmentLifecycle(ctx, writtenOff.Investment.ID, repo.LifecycleParams{
		Status: "sold", TerminatedAt: &termDate,
	}, &repo.InvestmentSettlement{
		Quantity:     ptrDec("100"),
		PricePerUnit: ptrDec("0"),
	}); err != nil {
		t.Fatalf("write-off terminate: %v", err)
	}
	// The control: terminated with nothing recorded, the restore/import shape.
	if _, err := r.UpdateInvestmentLifecycle(ctx, unsettled.Investment.ID, repo.LifecycleParams{
		Status: "sold", TerminatedAt: &termDate,
	}, nil); err != nil {
		t.Fatalf("unsettled terminate: %v", err)
	}

	txns, err := r.ListInvestmentTransactions(ctx, writtenOff.Investment.ID)
	if err != nil {
		t.Fatalf("ListInvestmentTransactions: %v", err)
	}
	var zeroSells int
	for _, txn := range txns {
		if txn.TransactionType == "sell" && txn.Amount != nil && txn.Amount.IsZero() {
			zeroSells++
		}
	}
	if zeroSells != 1 {
		t.Fatalf("got %d zero-proceeds sells, want exactly 1 — the write-off escape writes a Sell, not nothing", zeroSells)
	}

	reports, err := repo.NewMonthlyReportRepo(tdb.Pool).ListReports(ctx)
	if err != nil {
		t.Fatalf("ListReports: %v", err)
	}
	feb := mustMonth(t, reports, ym(2026, time.February))

	var advisory []struct {
		PositionID uuid.UUID `json:"position_id"`
		Reason     string    `json:"reason"`
	}
	if err := json.Unmarshal(feb.UnsettledTerminations, &advisory); err != nil {
		t.Fatalf("unmarshal unsettled_terminations: %v", err)
	}
	if len(advisory) != 1 {
		t.Fatalf("Feb advisory: got %d entries, want exactly 1 (the unsettled control)", len(advisory))
	}
	if advisory[0].PositionID != unsettled.Investment.ID {
		t.Errorf("Feb advisory names %v, want the unsettled control %v — the write-off escape must clear it",
			advisory[0].PositionID, unsettled.Investment.ID)
	}

	// Both positions carried 1000 into February and left holding nothing, and no
	// cash came back for either — a truthful total loss on both.
	if feb.InvestmentReturnTotal == nil {
		t.Fatal("Feb investment return is nil")
	}
	if want := decimal.NewFromInt(-2000); !feb.InvestmentReturnTotal.Equal(want) {
		t.Errorf("Feb investment return: got %s, want %s", feb.InvestmentReturnTotal, want)
	}
}
