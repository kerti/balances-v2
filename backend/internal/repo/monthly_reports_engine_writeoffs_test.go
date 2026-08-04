package repo

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Pure unit tests for the Write-Off term and the two rules that surround it
// (ADR-0052): asset-value-change stops absorbing the termination month, and an
// Investment terminated without recorded proceeds raises an advisory.
//
// The shared fixture shape is #585's: one bank account as the cash plug plus
// positions settled in consecutive months, so a correct engine leaves the
// derived Living Expenses residual at exactly 0 in every non-baseline month.
// A residual that moves is the defect these tests guard.

func writeOffAmountFor(t *testing.T, r monthlyReportData, id uuid.UUID) (decimal.Decimal, bool) {
	t.Helper()
	for _, w := range r.writeOffPositions {
		if w.ID == id {
			return w.Amount, true
		}
	}
	return decimal.Zero, false
}

func unsettledIDs(r monthlyReportData) []uuid.UUID {
	out := make([]uuid.UUID, 0, len(r.unsettledTerminations))
	for _, u := range r.unsettledTerminations {
		out = append(out, u.ID)
	}
	return out
}

// A disposed Asset, a forgiven Liability and a written-off Receivable each move
// net worth with no cash leg. Without the Write-Off term the whole value lands
// in the residual — and, per ADR-0052's timing analysis, lands there again with
// the opposite sign the month after. The term must leave the residual at 0 in
// both the termination month and the one following, for all three groups, with
// the sign following the effect on net worth (a forgiven debt is positive).
// covers: INV-FINANCE-33, INV-FINANCE-06, INV-FINANCE-05
func TestEngine_WriteOffAbsorbsNonCashTerminations(t *testing.T) {
	bank, veh, debt, loan := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	feb, mar, apr := ym(2026, time.February), ym(2026, time.March), ym(2026, time.April)

	in := reportEngineInput{
		positions: []reportPosition{
			{id: bank, group: groupAsset, subtype: "bank_account", status: "active", ownershipType: "joint"},
			{id: veh, group: groupAsset, subtype: "vehicle", status: "disposed", ownershipType: "joint", terminatedAt: &feb},
			{id: debt, group: groupLiability, subtype: "personal", status: "forgiven", ownershipType: "joint", terminatedAt: &mar},
			{id: loan, group: groupReceivable, status: "written_off", ownershipType: "joint", terminatedAt: &apr},
		},
		snapshots: []reportSnapshot{
			// Jan baseline: everything on the book. NW = 1000 + 200 + 150 − 300 = 1050.
			{positionID: bank, yearMonth: ym(2026, time.January), amount: dec("1000")},
			{positionID: veh, yearMonth: ym(2026, time.January), amount: dec("200")},
			{positionID: debt, yearMonth: ym(2026, time.January), amount: dec("300")},
			{positionID: loan, yearMonth: ym(2026, time.January), amount: dec("150")},
			// Each termination writes the 0-value close snapshot in its own month
			// (#585). The bank never moves: no cash settled any of these.
			{positionID: veh, yearMonth: feb, amount: dec("0")},
			{positionID: debt, yearMonth: mar, amount: dec("0")},
			{positionID: loan, yearMonth: apr, amount: dec("0")},
		},
		currentMonth: ym(2026, time.May),
	}
	reports := generateMonthlyReports(in)

	jan := findMonth(t, reports, ym(2026, time.January))
	if jan.writeOffs != nil {
		t.Errorf("Jan baseline write-offs: got %v, want nil (derived lines suppressed)", jan.writeOffs)
	}

	cases := []struct {
		month    time.Month
		writeOff string
		nw       string
		who      uuid.UUID // constituent expected on the line ("" for none)
	}{
		{time.February, "-200", "850", veh}, // disposed vehicle: NW down, term negative
		{time.March, "300", "1150", debt},   // forgiven debt: NW up, term positive
		{time.April, "-150", "1000", loan},  // written-off receivable: NW down
		{time.May, "0", "1000", uuid.Nil},   // nothing left to settle
	}
	for _, c := range cases {
		r := findMonth(t, reports, ym(2026, c.month))
		if !r.nwTotal.Equal(dec(c.nw)) {
			t.Errorf("%s nwTotal: got %s, want %s", c.month, r.nwTotal, c.nw)
		}
		if r.writeOffs == nil || !r.writeOffs.Equal(dec(c.writeOff)) {
			t.Errorf("%s write-offs: got %v, want %s", c.month, r.writeOffs, c.writeOff)
		}
		if r.livingExpenses == nil || !r.livingExpenses.IsZero() {
			t.Errorf("%s living expenses: got %v, want 0 — a non-cash termination must not read as spending",
				c.month, r.livingExpenses)
		}
		if c.who == uuid.Nil {
			if len(r.writeOffPositions) != 0 {
				t.Errorf("%s write-off constituents: got %d, want 0", c.month, len(r.writeOffPositions))
			}
			continue
		}
		amt, ok := writeOffAmountFor(t, r, c.who)
		if !ok || !amt.Equal(dec(c.writeOff)) {
			t.Errorf("%s constituent for the terminated position: got %s (present=%v), want %s",
				c.month, amt, ok, c.writeOff)
		}
	}

	// The identity closes month over month, including the Write-Off term.
	for i := 1; i < len(reports); i++ {
		prev, cur := reports[i-1], reports[i]
		deltaNW := cur.nwTotal.Sub(prev.nwTotal)
		if rhs := identityRHS(cur); !deltaNW.Equal(rhs) {
			t.Errorf("%s identity broken: ΔNW=%s != earned+return+assetΔ+writeOffs+tracking−expenses=%s",
				cur.yearMonth.Format("2006-01"), deltaNW, rhs)
		}
	}
}

// A cash-settled termination is not a write-off: `sold`, `paid_off` and
// `collected` all had cash come back, which the bank snapshot already records.
// Booking a write-off for them would double-count the settlement.
// covers: INV-FINANCE-33
func TestEngine_CashSettledTerminationsBookNoWriteOff(t *testing.T) {
	bank, veh, debt, loan := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	feb := ym(2026, time.February)

	in := reportEngineInput{
		positions: []reportPosition{
			{id: bank, group: groupAsset, subtype: "bank_account", status: "active", ownershipType: "joint"},
			{id: veh, group: groupAsset, subtype: "vehicle", status: "sold", ownershipType: "joint", terminatedAt: &feb},
			{id: debt, group: groupLiability, subtype: "personal", status: "paid_off", ownershipType: "joint", terminatedAt: &feb},
			{id: loan, group: groupReceivable, status: "collected", ownershipType: "joint", terminatedAt: &feb},
		},
		snapshots: []reportSnapshot{
			// Jan: NW = 1000 + 200 + 150 − 300 = 1050.
			{positionID: bank, yearMonth: ym(2026, time.January), amount: dec("1000")},
			{positionID: veh, yearMonth: ym(2026, time.January), amount: dec("200")},
			{positionID: debt, yearMonth: ym(2026, time.January), amount: dec("300")},
			{positionID: loan, yearMonth: ym(2026, time.January), amount: dec("150")},
			// Feb: all three settle in cash — vehicle proceeds + loan collected in,
			// debt paid off out. Bank 1000 + 200 + 150 − 300 = 1050. NW unchanged.
			{positionID: bank, yearMonth: feb, amount: dec("1050")},
			{positionID: veh, yearMonth: feb, amount: dec("0")},
			{positionID: debt, yearMonth: feb, amount: dec("0")},
			{positionID: loan, yearMonth: feb, amount: dec("0")},
		},
		currentMonth: feb,
	}
	reports := generateMonthlyReports(in)
	r := findMonth(t, reports, feb)

	if r.writeOffs == nil || !r.writeOffs.IsZero() {
		t.Errorf("Feb write-offs: got %v, want 0 — cash-settled statuses take no write-off", r.writeOffs)
	}
	if len(r.writeOffPositions) != 0 {
		t.Errorf("Feb write-off constituents: got %d, want 0", len(r.writeOffPositions))
	}
	if r.livingExpenses == nil || !r.livingExpenses.IsZero() {
		t.Errorf("Feb living expenses: got %v, want 0", r.livingExpenses)
	}
}

// A termination-month value change is never a mark change (ADR-0052 §3). A
// property sold for cash drops to its 0-value close snapshot in the termination
// month; if asset-value-change absorbed that, it would read as depreciation and
// understate the residual by the whole sale value. The carve-out must be
// surgical: a property the household still holds keeps depreciating normally in
// the same month.
// covers: INV-FINANCE-34, INV-FINANCE-10, INV-FINANCE-06
func TestEngine_SoldPropertyIsNotDepreciation(t *testing.T) {
	bank, sold, held := uuid.New(), uuid.New(), uuid.New()
	feb := ym(2026, time.February)

	in := reportEngineInput{
		positions: []reportPosition{
			{id: bank, group: groupAsset, subtype: "bank_account", status: "active", ownershipType: "joint"},
			{id: sold, group: groupAsset, subtype: "property", status: "sold", ownershipType: "joint", terminatedAt: &feb},
			{id: held, group: groupAsset, subtype: "property", status: "active", ownershipType: "joint"},
		},
		snapshots: []reportSnapshot{
			// Jan: NW = 1000 + 500 + 300 = 1800.
			{positionID: bank, yearMonth: ym(2026, time.January), amount: dec("1000")},
			{positionID: sold, yearMonth: ym(2026, time.January), amount: dec("500")},
			{positionID: held, yearMonth: ym(2026, time.January), amount: dec("300")},
			// Feb: the sale settles into the bank (1000 → 1500) as the property drops
			// to its close snapshot; the retained property marks down 300 → 290.
			// NW = 1500 + 290 = 1790, so ΔNW = −10 — the retained mark, nothing else.
			{positionID: bank, yearMonth: feb, amount: dec("1500")},
			{positionID: sold, yearMonth: feb, amount: dec("0")},
			{positionID: held, yearMonth: feb, amount: dec("290")},
		},
		currentMonth: feb,
	}
	reports := generateMonthlyReports(in)
	r := findMonth(t, reports, feb)

	if r.assetValueChange == nil || !r.assetValueChange.Equal(dec("-10")) {
		t.Fatalf("Feb asset value change: %v, want -10 (retained property's mark only; the sale is not depreciation)",
			r.assetValueChange)
	}
	if r.writeOffs == nil || !r.writeOffs.IsZero() {
		t.Errorf("Feb write-offs: got %v, want 0 (a `sold` property settled in cash)", r.writeOffs)
	}
	if r.livingExpenses == nil || !r.livingExpenses.IsZero() {
		t.Errorf("Feb living expenses: got %v, want 0 — the residual must not be understated by the sale value",
			r.livingExpenses)
	}
}

// Write-Offs is one signed term, so a month holding both a forgiven debt and a
// written-off receivable can net to zero on the line. The constituent list is
// what keeps that from reading as "nothing happened" (ADR-0052 §4).
// covers: INV-FINANCE-33
func TestEngine_WriteOffConstituentsSurviveNettingToZero(t *testing.T) {
	bank, debt, loan := uuid.New(), uuid.New(), uuid.New()
	feb := ym(2026, time.February)

	in := reportEngineInput{
		positions: []reportPosition{
			{id: bank, group: groupAsset, subtype: "bank_account", status: "active", ownershipType: "joint"},
			{id: debt, group: groupLiability, subtype: "personal", status: "forgiven", ownershipType: "joint", terminatedAt: &feb},
			{id: loan, group: groupReceivable, status: "written_off", ownershipType: "joint", terminatedAt: &feb},
		},
		snapshots: []reportSnapshot{
			{positionID: bank, yearMonth: ym(2026, time.January), amount: dec("1000")},
			{positionID: debt, yearMonth: ym(2026, time.January), amount: dec("200")},
			{positionID: loan, yearMonth: ym(2026, time.January), amount: dec("200")},
			{positionID: debt, yearMonth: feb, amount: dec("0")},
			{positionID: loan, yearMonth: feb, amount: dec("0")},
		},
		currentMonth: feb,
	}
	r := findMonth(t, generateMonthlyReports(in), feb)

	if r.writeOffs == nil || !r.writeOffs.IsZero() {
		t.Fatalf("Feb write-offs: got %v, want 0 (+200 forgiven, −200 written off)", r.writeOffs)
	}
	if len(r.writeOffPositions) != 2 {
		t.Fatalf("Feb write-off constituents: got %d, want 2 — a netted line must still itemise", len(r.writeOffPositions))
	}
	if amt, ok := writeOffAmountFor(t, r, debt); !ok || !amt.Equal(dec("200")) {
		t.Errorf("forgiven liability constituent: got %s (present=%v), want +200", amt, ok)
	}
	if amt, ok := writeOffAmountFor(t, r, loan); !ok || !amt.Equal(dec("-200")) {
		t.Errorf("written-off receivable constituent: got %s (present=%v), want -200", amt, ok)
	}
}

// A Position whose currency has no rate at or before M is excluded from net
// worth; the Write-Off term must skip it too, or it would book a movement
// against a ΔNW that never happened (ADR-0052 §4).
// covers: INV-FINANCE-33, INV-FINANCE-16
func TestEngine_WriteOffSkipsUnconvertibleCurrency(t *testing.T) {
	bank, loan := uuid.New(), uuid.New()
	feb := ym(2026, time.February)

	in := reportEngineInput{
		positions: []reportPosition{
			{id: bank, group: groupAsset, subtype: "bank_account", status: "active", ownershipType: "joint"},
			{id: loan, group: groupReceivable, status: "written_off", ownershipType: "joint", terminatedAt: &feb},
		},
		snapshots: []reportSnapshot{
			{positionID: bank, yearMonth: ym(2026, time.January), amount: dec("1000"), currency: "IDR"},
			{positionID: loan, yearMonth: ym(2026, time.January), amount: dec("150"), currency: "USD"},
			{positionID: loan, yearMonth: feb, amount: dec("0"), currency: "USD"},
		},
		reportingCurrency: "IDR",
		multiCurrency:     true, // ...but no USD rate exists at any month
		currentMonth:      feb,
	}
	r := findMonth(t, generateMonthlyReports(in), feb)

	if r.writeOffs == nil || !r.writeOffs.IsZero() {
		t.Errorf("Feb write-offs: got %v, want 0 — an unconvertible position is skipped in both passes", r.writeOffs)
	}
	if r.livingExpenses == nil || !r.livingExpenses.IsZero() {
		t.Errorf("Feb living expenses: got %v, want 0", r.livingExpenses)
	}
}

// An Investment terminated with no proceeds recorded raises the advisory — the
// restore-from-backup / import / raw-API path that can hand a household bad data
// with no way to notice (ADR-0052 §7). A deliberate 0-proceeds Sell IS the
// modelled investment write-off (§5), so it settles the advisory rather than
// tripping it forever; the amount is never inspected.
// covers: INV-FINANCE-35
func TestEngine_UnsettledTerminationAdvisory(t *testing.T) {
	bank, noTxn, zeroSell, realSell := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	feb := ym(2026, time.February)
	zero, four := dec("0"), dec("400")

	in := reportEngineInput{
		positions: []reportPosition{
			{id: bank, group: groupAsset, subtype: "bank_account", status: "active", ownershipType: "joint"},
			{id: noTxn, group: groupInvestment, subtype: "stock", ownershipType: "joint", terminatedAt: &feb},
			{id: zeroSell, group: groupInvestment, subtype: "stock", ownershipType: "joint", terminatedAt: &feb},
			{id: realSell, group: groupInvestment, subtype: "stock", ownershipType: "joint", terminatedAt: &feb},
		},
		snapshots: []reportSnapshot{
			{positionID: bank, yearMonth: ym(2026, time.January), amount: dec("1000")},
			{positionID: noTxn, yearMonth: ym(2026, time.January), amount: four},
			{positionID: zeroSell, yearMonth: ym(2026, time.January), amount: four},
			{positionID: realSell, yearMonth: ym(2026, time.January), amount: four},
			{positionID: noTxn, yearMonth: feb, amount: zero},
			{positionID: zeroSell, yearMonth: feb, amount: zero},
			{positionID: realSell, yearMonth: feb, amount: zero},
		},
		transactions: []reportTransaction{
			{investmentID: zeroSell, yearMonth: feb, txnType: "sell", amount: &zero},
			{investmentID: realSell, yearMonth: feb, txnType: "sell", amount: &four},
		},
		currentMonth: ym(2026, time.March),
	}
	reports := generateMonthlyReports(in)

	got := unsettledIDs(findMonth(t, reports, feb))
	if len(got) != 1 || got[0] != noTxn {
		t.Fatalf("Feb unsettled terminations: got %v, want exactly the proceeds-less investment %v", got, noTxn)
	}
	if r := findMonth(t, reports, feb); r.unsettledTerminations[0].Reason != reasonNoProceeds {
		t.Errorf("advisory reason: got %q, want %q", r.unsettledTerminations[0].Reason, reasonNoProceeds)
	}
	// The advisory belongs to the termination month, like every other per-month
	// payload — neither the month before nor the month after carries it.
	for _, m := range []time.Month{time.January, time.March} {
		if ids := unsettledIDs(findMonth(t, reports, ym(2026, m))); len(ids) != 0 {
			t.Errorf("%s unsettled terminations: got %v, want none", m, ids)
		}
	}
}
