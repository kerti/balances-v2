package repo

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Pure unit tests for the Tracking Change term in both directions (ADR-0053):
// a Position arriving over the edge of the book at its first Snapshot, and one
// departing over it at a termination.
//
// The fixture shape is the Write-Off suite's: one bank account as the cash plug
// plus positions crossing the edge in consecutive months, so a correct engine
// leaves the derived Living Expenses residual at exactly 0 in every non-baseline
// month. A residual that moves is the defect these tests guard.

func trackingChangeAmountFor(t *testing.T, r monthlyReportData, id uuid.UUID) (decimal.Decimal, bool) {
	t.Helper()
	for _, tc := range r.trackingChangePositions {
		if tc.ID == id {
			return tc.Amount, true
		}
	}
	return decimal.Zero, false
}

// identityRHS is the whole comprehensive-income identity as the engine derives
// it, so a test can assert ΔNW == RHS without restating the term list.
func identityRHS(r monthlyReportData) decimal.Decimal {
	return r.earnedIncome.total.
		Add(r.investmentReturn.total).
		Add(*r.assetValueChange).
		Add(*r.writeOffs).
		Add(*r.trackingChanges).
		Sub(*r.livingExpenses)
}

// A Position the Household already owned enters net worth at its first Snapshot
// with no counterpart flow. Before the Tracking Change term that whole amount
// landed in the residual, reading as a huge *negative* month's spending (#591).
// All four groups are affected on the way in — Investment included, whose
// birth-month return suppression (INV-FINANCE-23) was never protection — and the
// sign follows the effect on net worth, so an arriving Liability is negative.
// covers: INV-FINANCE-36, INV-FINANCE-06, INV-FINANCE-38
func TestEngine_NewlyTrackedArrivalAbsorbedByTrackingChange(t *testing.T) {
	bank, house, mortgage, iou, fund := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	jan := ym(2026, time.January)

	in := reportEngineInput{
		positions: []reportPosition{
			{id: bank, group: groupAsset, subtype: "bank_account", status: StatusActive, entryType: EntryTypeAcquired, ownershipType: "joint"},
			{id: house, group: groupAsset, subtype: "property", status: StatusActive, entryType: EntryTypeNewlyTracked, ownershipType: "joint"},
			{id: mortgage, group: groupLiability, subtype: "mortgage", status: StatusActive, entryType: EntryTypeNewlyTracked, ownershipType: "joint"},
			{id: iou, group: groupReceivable, status: StatusActive, entryType: EntryTypeNewlyTracked, ownershipType: "joint"},
			{id: fund, group: groupInvestment, subtype: "mutual_fund", riskProfile: "medium", status: StatusActive, entryType: EntryTypeNewlyTracked, ownershipType: "joint"},
		},
		snapshots: []reportSnapshot{
			// Jan baseline: only the household's own cash. NW = 1000.
			{positionID: bank, yearMonth: jan, amount: dec("1000")},
			// One arrival per month, each at its first Snapshot. The bank never
			// moves: none of these was funded from tracked wealth.
			{positionID: house, yearMonth: ym(2026, time.February), amount: dec("500")},
			{positionID: mortgage, yearMonth: ym(2026, time.March), amount: dec("300")},
			{positionID: iou, yearMonth: ym(2026, time.April), amount: dec("150")},
			{positionID: fund, yearMonth: ym(2026, time.May), amount: dec("400")},
		},
		currentMonth: ym(2026, time.June),
	}
	reports := generateMonthlyReports(in)

	baseline := findMonth(t, reports, jan)
	if baseline.trackingChanges != nil {
		t.Errorf("Jan baseline tracking changes: got %v, want nil (derived lines suppressed)", baseline.trackingChanges)
	}

	cases := []struct {
		month    time.Month
		tracking string
		nw       string
		who      uuid.UUID // constituent expected on the line (uuid.Nil for none)
	}{
		{time.February, "500", "1500", house},  // property arrives: NW up
		{time.March, "-300", "1200", mortgage}, // mortgage arrives: NW down
		{time.April, "150", "1350", iou},       // receivable arrives: NW up
		{time.May, "400", "1750", fund},        // investment arrives: NW up
		{time.June, "0", "1750", uuid.Nil},     // nothing crossed the edge
	}
	for _, c := range cases {
		r := findMonth(t, reports, ym(2026, c.month))
		if !r.nwTotal.Equal(dec(c.nw)) {
			t.Errorf("%s nwTotal: got %s, want %s", c.month, r.nwTotal, c.nw)
		}
		if r.trackingChanges == nil || !r.trackingChanges.Equal(dec(c.tracking)) {
			t.Errorf("%s tracking changes: got %v, want %s", c.month, r.trackingChanges, c.tracking)
		}
		if r.livingExpenses == nil || !r.livingExpenses.IsZero() {
			t.Errorf("%s living expenses: got %v, want 0 — the books' coverage moving must not read as spending",
				c.month, r.livingExpenses)
		}
		if c.who == uuid.Nil {
			if len(r.trackingChangePositions) != 0 {
				t.Errorf("%s tracking-change constituents: got %d, want 0", c.month, len(r.trackingChangePositions))
			}
			continue
		}
		amt, ok := trackingChangeAmountFor(t, r, c.who)
		if !ok || !amt.Equal(dec(c.tracking)) {
			t.Errorf("%s constituent for the arriving position: got %s (present=%v), want %s",
				c.month, amt, ok, c.tracking)
		}
	}

	// No arrival reaches Earned Income or the per-owner breakdown (ADR-0053 §6).
	for _, r := range reports[1:] {
		if !r.earnedIncome.total.IsZero() {
			t.Errorf("%s earned income: got %s, want 0 — a Tracking Change is never income",
				r.yearMonth.Format("2006-01"), r.earnedIncome.total)
		}
	}

	for i := 1; i < len(reports); i++ {
		prev, cur := reports[i-1], reports[i]
		if deltaNW, rhs := cur.nwTotal.Sub(prev.nwTotal), identityRHS(cur); !deltaNW.Equal(rhs) {
			t.Errorf("%s identity broken: ΔNW=%s != income+return+assetΔ+writeOffs+tracking−expenses=%s",
				cur.yearMonth.Format("2006-01"), deltaNW, rhs)
		}
	}
}

// The whole design turns on the term being *declared*, never inferred. An
// acquisition funded from tracked wealth and an arrival over the books' edge
// present to the engine as the same thing — a first Snapshot with no prior
// value. `acquired` (the column DEFAULT, and what an unset value normalises to)
// must therefore book nothing, and declaring the same acquisition
// `newly_tracked` must visibly break it, by exactly the amount a blanket
// birth-month term would have broken every acquisition by.
// covers: INV-FINANCE-36, INV-FINANCE-23
func TestEngine_AcquiredEntryBooksNoTrackingChange(t *testing.T) {
	feb := ym(2026, time.February)

	// One bank account funds a second one: 400 moves out of the old and into the
	// new, so net worth does not move at all.
	build := func(entryType string) []monthlyReportData {
		old, fresh := uuid.New(), uuid.New()
		return generateMonthlyReports(reportEngineInput{
			positions: []reportPosition{
				{id: old, group: groupAsset, subtype: "bank_account", status: StatusActive, entryType: EntryTypeAcquired, ownershipType: "joint"},
				{id: fresh, group: groupAsset, subtype: "bank_account", status: StatusActive, entryType: entryType, ownershipType: "joint"},
			},
			snapshots: []reportSnapshot{
				{positionID: old, yearMonth: ym(2026, time.January), amount: dec("1000")},
				{positionID: old, yearMonth: feb, amount: dec("600")},
				{positionID: fresh, yearMonth: feb, amount: dec("400")},
			},
			currentMonth: feb,
		})
	}

	// The empty string is what every create path that predates ADR-0053 supplies;
	// it must behave exactly like an explicit `acquired`.
	for _, entryType := range []string{EntryTypeAcquired, ""} {
		r := findMonth(t, build(entryType), feb)
		if r.trackingChanges == nil || !r.trackingChanges.IsZero() {
			t.Errorf("entry_type=%q tracking changes: got %v, want 0 — a funded acquisition crosses no edge",
				entryType, r.trackingChanges)
		}
		if len(r.trackingChangePositions) != 0 {
			t.Errorf("entry_type=%q constituents: got %d, want 0", entryType, len(r.trackingChangePositions))
		}
		if r.livingExpenses == nil || !r.livingExpenses.IsZero() {
			t.Errorf("entry_type=%q living expenses: got %v, want 0", entryType, r.livingExpenses)
		}
	}

	// Mis-declared: the same acquisition marked `newly_tracked` books the full
	// value and throws the residual out by it. This is what a birth-month term
	// applied blanket would do to every acquisition in the book.
	r := findMonth(t, build(EntryTypeNewlyTracked), feb)
	if r.trackingChanges == nil || !r.trackingChanges.Equal(dec("400")) {
		t.Fatalf("mis-declared tracking changes: got %v, want 400", r.trackingChanges)
	}
	if r.livingExpenses == nil || !r.livingExpenses.Equal(dec("400")) {
		t.Errorf("mis-declared living expenses: got %v, want 400 — the declaration is load-bearing, and a wrong one is visible",
			r.livingExpenses)
	}
}

// The exit side: `untracked` is the one terminal status every group defines,
// Investment included (ADR-0053 §5 amending ADR-0052 §5). The departing value
// must be carried once — by the Tracking Change term — which means that month's
// Investment Return must exclude it, and the unsettled-termination advisory must
// not fire on it.
// covers: INV-FINANCE-37, INV-FINANCE-06, INV-FINANCE-35
func TestEngine_UntrackedDepartureAbsorbedByTrackingChange(t *testing.T) {
	bank, house, mortgage, iou, fund := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	jan := ym(2026, time.January)
	feb, mar, apr, may := ym(2026, time.February), ym(2026, time.March), ym(2026, time.April), ym(2026, time.May)

	in := reportEngineInput{
		positions: []reportPosition{
			{id: bank, group: groupAsset, subtype: "bank_account", status: StatusActive, entryType: EntryTypeAcquired, ownershipType: "joint"},
			{id: house, group: groupAsset, subtype: "property", status: StatusUntracked, entryType: EntryTypeAcquired, ownershipType: "joint", terminatedAt: &feb},
			{id: mortgage, group: groupLiability, subtype: "mortgage", status: StatusUntracked, entryType: EntryTypeAcquired, ownershipType: "joint", terminatedAt: &mar},
			{id: iou, group: groupReceivable, status: StatusUntracked, entryType: EntryTypeAcquired, ownershipType: "joint", terminatedAt: &apr},
			{id: fund, group: groupInvestment, subtype: "mutual_fund", riskProfile: "medium", status: StatusUntracked, entryType: EntryTypeAcquired, ownershipType: "joint", terminatedAt: &may},
		},
		snapshots: []reportSnapshot{
			// Jan: everything on the book. NW = 1000 + 500 + 150 + 400 − 300 = 1750.
			{positionID: bank, yearMonth: jan, amount: dec("1000")},
			{positionID: house, yearMonth: jan, amount: dec("500")},
			{positionID: mortgage, yearMonth: jan, amount: dec("300")},
			{positionID: iou, yearMonth: jan, amount: dec("150")},
			{positionID: fund, yearMonth: jan, amount: dec("400")},
			// Each departure writes the 0-value close snapshot in its own month.
			// The bank never moves: nothing was sold, so no cash came back.
			{positionID: house, yearMonth: feb, amount: dec("0")},
			{positionID: mortgage, yearMonth: mar, amount: dec("0")},
			{positionID: iou, yearMonth: apr, amount: dec("0")},
			{positionID: fund, yearMonth: may, amount: dec("0")},
		},
		currentMonth: may,
	}
	reports := generateMonthlyReports(in)

	cases := []struct {
		month    time.Month
		tracking string
		nw       string
		who      uuid.UUID
	}{
		{time.February, "-500", "1250", house}, // property leaves: NW down
		{time.March, "300", "1550", mortgage},  // mortgage leaves: NW up
		{time.April, "-150", "1400", iou},      // receivable leaves: NW down
		{time.May, "-400", "1000", fund},       // portfolio leaves: NW down
	}
	for _, c := range cases {
		r := findMonth(t, reports, ym(2026, c.month))
		if !r.nwTotal.Equal(dec(c.nw)) {
			t.Errorf("%s nwTotal: got %s, want %s", c.month, r.nwTotal, c.nw)
		}
		if r.trackingChanges == nil || !r.trackingChanges.Equal(dec(c.tracking)) {
			t.Errorf("%s tracking changes: got %v, want %s", c.month, r.trackingChanges, c.tracking)
		}
		if r.writeOffs == nil || !r.writeOffs.IsZero() {
			t.Errorf("%s write-offs: got %v, want 0 — `untracked` is a Tracking Change, not a write-off", c.month, r.writeOffs)
		}
		if r.livingExpenses == nil || !r.livingExpenses.IsZero() {
			t.Errorf("%s living expenses: got %v, want 0 — a departure over the books' edge must not read as spending",
				c.month, r.livingExpenses)
		}
		amt, ok := trackingChangeAmountFor(t, r, c.who)
		if !ok || !amt.Equal(dec(c.tracking)) {
			t.Errorf("%s constituent for the departing position: got %s (present=%v), want %s",
				c.month, amt, ok, c.tracking)
		}
	}

	// The departing portfolio did not lose its value: booking it as a large
	// negative Investment Return is the falsehood #576 complained about, and
	// would double-count against the Tracking Change.
	mayReport := findMonth(t, reports, may)
	if !mayReport.investmentReturn.total.IsZero() {
		t.Errorf("May investment return: got %s, want 0 — an `untracked` departure is not a loss",
			mayReport.investmentReturn.total)
	}
	// Nothing was sold, so there are no proceeds to look for. Without the
	// exemption every departing Investment would trip an advisory nothing clears.
	if len(mayReport.unsettledTerminations) != 0 {
		t.Errorf("May unsettled terminations: got %d, want 0 — `untracked` needs no settlement",
			len(mayReport.unsettledTerminations))
	}

	for i := 1; i < len(reports); i++ {
		prev, cur := reports[i-1], reports[i]
		if deltaNW, rhs := cur.nwTotal.Sub(prev.nwTotal), identityRHS(cur); !deltaNW.Equal(rhs) {
			t.Errorf("%s identity broken: ΔNW=%s != income+return+assetΔ+writeOffs+tracking−expenses=%s",
				cur.yearMonth.Format("2006-01"), deltaNW, rhs)
		}
	}
}

// One signed term covers both directions, so a month holding an arrival and a
// departure of equal size nets to zero on the line. The constituent list is what
// keeps that from reading as "nothing happened" (ADR-0053 §1).
// covers: INV-FINANCE-38
func TestEngine_TrackingChangeConstituentsSurviveNettingToZero(t *testing.T) {
	bank, arriving, leaving := uuid.New(), uuid.New(), uuid.New()
	feb := ym(2026, time.February)

	in := reportEngineInput{
		positions: []reportPosition{
			{id: bank, group: groupAsset, subtype: "bank_account", status: StatusActive, entryType: EntryTypeAcquired, ownershipType: "joint"},
			{id: arriving, group: groupAsset, subtype: "bank_account", status: StatusActive, entryType: EntryTypeNewlyTracked, ownershipType: "joint"},
			{id: leaving, group: groupReceivable, status: StatusUntracked, entryType: EntryTypeAcquired, ownershipType: "joint", terminatedAt: &feb},
		},
		snapshots: []reportSnapshot{
			{positionID: bank, yearMonth: ym(2026, time.January), amount: dec("1000")},
			{positionID: leaving, yearMonth: ym(2026, time.January), amount: dec("300")},
			{positionID: arriving, yearMonth: feb, amount: dec("300")},
			{positionID: leaving, yearMonth: feb, amount: dec("0")},
		},
		currentMonth: feb,
	}
	r := findMonth(t, generateMonthlyReports(in), feb)

	if r.trackingChanges == nil || !r.trackingChanges.IsZero() {
		t.Fatalf("Feb tracking changes: got %v, want 0 (an arrival and a departure of equal size)", r.trackingChanges)
	}
	if len(r.trackingChangePositions) != 2 {
		t.Fatalf("Feb constituents: got %d, want 2 — a netted line must still itemise", len(r.trackingChangePositions))
	}
	if amt, ok := trackingChangeAmountFor(t, r, arriving); !ok || !amt.Equal(dec("300")) {
		t.Errorf("arriving constituent: got %s (present=%v), want 300", amt, ok)
	}
	if amt, ok := trackingChangeAmountFor(t, r, leaving); !ok || !amt.Equal(dec("-300")) {
		t.Errorf("departing constituent: got %s (present=%v), want -300", amt, ok)
	}
	if r.livingExpenses == nil || !r.livingExpenses.IsZero() {
		t.Errorf("Feb living expenses: got %v, want 0", r.livingExpenses)
	}
}

// A household that onboards everything at once already had no defect: the
// engine suppresses the income statement wholesale in its first Snapshot month
// (ADR-0006). #591 is precisely the *post-baseline* birth, and the baseline must
// stay untouched — which is also why `acquired` is the right default.
// covers: INV-FINANCE-36, INV-FINANCE-07
func TestEngine_NewlyTrackedInBaselineMonthBooksNothing(t *testing.T) {
	bank, house := uuid.New(), uuid.New()
	jan, feb := ym(2026, time.January), ym(2026, time.February)

	in := reportEngineInput{
		positions: []reportPosition{
			{id: bank, group: groupAsset, subtype: "bank_account", status: StatusActive, entryType: EntryTypeNewlyTracked, ownershipType: "joint"},
			{id: house, group: groupAsset, subtype: "property", status: StatusActive, entryType: EntryTypeNewlyTracked, ownershipType: "joint"},
		},
		snapshots: []reportSnapshot{
			{positionID: bank, yearMonth: jan, amount: dec("1000")},
			{positionID: house, yearMonth: jan, amount: dec("500")},
			{positionID: bank, yearMonth: feb, amount: dec("1000")},
			{positionID: house, yearMonth: feb, amount: dec("500")},
		},
		currentMonth: feb,
	}
	reports := generateMonthlyReports(in)

	baseline := findMonth(t, reports, jan)
	if baseline.trackingChanges != nil || baseline.livingExpenses != nil {
		t.Errorf("Jan baseline: tracking=%v expenses=%v, want both nil", baseline.trackingChanges, baseline.livingExpenses)
	}
	if len(baseline.trackingChangePositions) != 0 {
		t.Errorf("Jan baseline constituents: got %d, want 0", len(baseline.trackingChangePositions))
	}
	// February holds no first Snapshot, so nothing crosses the edge there either.
	next := findMonth(t, reports, feb)
	if next.trackingChanges == nil || !next.trackingChanges.IsZero() {
		t.Errorf("Feb tracking changes: got %v, want 0", next.trackingChanges)
	}
}

// The term runs through the same FX-converted carried values the net-worth pass
// uses, so it cancels ΔNW structurally rather than coincidentally: an arrival in
// a foreign currency is booked at the same rate its net-worth contribution is,
// and one whose currency has no rate at or before its month drops out of both.
// covers: INV-FINANCE-36, INV-FINANCE-15, INV-FINANCE-16
func TestEngine_TrackingChangeUsesTheSameFxAsNetWorth(t *testing.T) {
	bank, usdAcct, gbpAcct := uuid.New(), uuid.New(), uuid.New()
	jan, feb := ym(2026, time.January), ym(2026, time.February)

	in := reportEngineInput{
		positions: []reportPosition{
			{id: bank, group: groupAsset, subtype: "bank_account", status: StatusActive, entryType: EntryTypeAcquired, ownershipType: "joint"},
			{id: usdAcct, group: groupAsset, subtype: "bank_account", status: StatusActive, entryType: EntryTypeNewlyTracked, ownershipType: "joint"},
			// No GBP rate is ever recorded, so this one is excluded from net worth
			// and must be excluded from the term too.
			{id: gbpAcct, group: groupAsset, subtype: "bank_account", status: StatusActive, entryType: EntryTypeNewlyTracked, ownershipType: "joint"},
		},
		snapshots: []reportSnapshot{
			{positionID: bank, yearMonth: jan, amount: dec("1000"), currency: "IDR"},
			{positionID: usdAcct, yearMonth: feb, amount: dec("100"), currency: "USD"},
			{positionID: gbpAcct, yearMonth: feb, amount: dec("50"), currency: "GBP"},
		},
		fxRates: []reportFxRate{
			// Carried forward from January — the same rate the net-worth pass picks.
			{currency: "USD", yearMonth: jan, rate: dec("16000")},
		},
		reportingCurrency: "IDR",
		multiCurrency:     true,
		currentMonth:      feb,
	}
	r := findMonth(t, generateMonthlyReports(in), feb)

	if want := dec("1600000"); r.trackingChanges == nil || !r.trackingChanges.Equal(want) {
		t.Fatalf("Feb tracking changes: got %v, want %s (100 USD at the carried-forward rate; the GBP arrival has no rate)",
			r.trackingChanges, want)
	}
	if len(r.trackingChangePositions) != 1 {
		t.Errorf("Feb constituents: got %d, want 1 — an unconvertible arrival is not itemised", len(r.trackingChangePositions))
	}
	if _, ok := trackingChangeAmountFor(t, r, gbpAcct); ok {
		t.Error("the unconvertible GBP arrival was itemised; it must drop out of the term exactly as it drops out of net worth")
	}
	if r.livingExpenses == nil || !r.livingExpenses.IsZero() {
		t.Errorf("Feb living expenses: got %v, want 0", r.livingExpenses)
	}
}
