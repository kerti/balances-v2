package repo

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Pure tests for the overview hubs' per-group breakdown (INV-FINANCE-39). The
// breakdown is projected from the very months generateMonthlyReports returns, so
// every test checks the projection against the report's own nw_* figure.

func findBreakdownMonth(t *testing.T, b *GroupBreakdown, m time.Time) GroupBreakdownMonth {
	t.Helper()
	for _, bm := range b.Months {
		if bm.YearMonth.Equal(m) {
			return bm
		}
	}
	t.Fatalf("no breakdown month %s", m.Format("2006-01"))
	return GroupBreakdownMonth{}
}

func sumDecimals(m map[string]decimal.Decimal) decimal.Decimal {
	s := decimal.Zero
	for _, v := range m {
		s = s.Add(v)
	}
	return s
}

// An IDR bank account + a USD bank account + a USD property: each month's hub
// total is exactly the report's nw_assets, converted at that month's rate, and
// the subtypes partition it.
// covers: INV-FINANCE-39
func TestGroupBreakdown_TotalEqualsDashboardAssets(t *testing.T) {
	idr, usdBank, usdProp := uuid.New(), uuid.New(), uuid.New()
	jan, feb, mar := ym(2026, time.January), ym(2026, time.February), ym(2026, time.March)
	in := reportEngineInput{
		reportingCurrency: "IDR",
		multiCurrency:     true,
		positions: []reportPosition{
			{id: idr, group: groupAsset, subtype: "bank_account", ownershipType: "joint"},
			{id: usdBank, group: groupAsset, subtype: "bank_account", ownershipType: "joint"},
			{id: usdProp, group: groupAsset, subtype: "property", ownershipType: "joint"},
		},
		snapshots: []reportSnapshot{
			{positionID: idr, yearMonth: jan, amount: dec("1000000"), currency: "IDR"},
			{positionID: usdBank, yearMonth: jan, amount: dec("100"), currency: "USD"},
			{positionID: usdProp, yearMonth: feb, amount: dec("2000.5"), currency: "USD"},
			{positionID: usdBank, yearMonth: mar, amount: dec("120"), currency: "USD"},
		},
		fxRates: []reportFxRate{
			{currency: "USD", yearMonth: jan, rate: dec("15000")},
			{currency: "USD", yearMonth: mar, rate: dec("16000")},
		},
		currentMonth: mar,
	}
	reports := generateMonthlyReports(in)
	b := buildGroupBreakdown(reports, groupAsset, "IDR")

	want := map[time.Time]struct{ total, bank, prop string }{
		// Jan: 1,000,000 + 100 × 15,000
		jan: {"2500000", "2500000", "0"},
		// Feb carries Jan's bank balances, rate carries forward (15,000); the
		// property is 2,000.5 × 15,000 — an exact decimal, no float rounding.
		feb: {"32507500", "2500000", "30007500"},
		// Mar: bank = 1,000,000 + 120 × 16,000; the property carries
		// 2,000.5 × 16,000 at the new rate.
		mar: {"34928000", "2920000", "32008000"},
	}

	for m, w := range want {
		bm := findBreakdownMonth(t, b, m)
		rep := findMonth(t, reports, m)
		if !bm.Total.Equal(rep.nwAssets) {
			t.Errorf("%s: hub total %s != report nw_assets %s", m.Format("2006-01"), bm.Total, rep.nwAssets)
		}
		if !bm.Total.Equal(dec(w.total)) {
			t.Errorf("%s: total got %s, want %s", m.Format("2006-01"), bm.Total, w.total)
		}
		if !bm.BySubtype["bank_account"].Equal(dec(w.bank)) {
			t.Errorf("%s: bank_account got %s, want %s", m.Format("2006-01"), bm.BySubtype["bank_account"], w.bank)
		}
		if !bm.BySubtype["property"].Equal(dec(w.prop)) {
			t.Errorf("%s: property got %s, want %s", m.Format("2006-01"), bm.BySubtype["property"], w.prop)
		}
		if got := sumDecimals(bm.BySubtype); !got.Equal(bm.Total) {
			t.Errorf("%s: subtypes sum to %s, total is %s", m.Format("2006-01"), got, bm.Total)
		}
		if len(bm.MissingFx) != 0 {
			t.Errorf("%s: unexpected missing_fx %v", m.Format("2006-01"), bm.MissingFx)
		}
	}
}

// A USD position in a month with no rate at or before it is left out of the hub
// total and named in missing_fx — never counted 1:1 — and exactly as the
// dashboard's report does. The warning is scoped to the group: a USD liability
// missing the same rate does not appear on the assets breakdown.
// covers: INV-FINANCE-39, INV-FINANCE-16
func TestGroupBreakdown_MissingFxExcludedAndScopedToGroup(t *testing.T) {
	idr, usd, usdDebt := uuid.New(), uuid.New(), uuid.New()
	jan, feb, mar := ym(2026, time.January), ym(2026, time.February), ym(2026, time.March)
	in := reportEngineInput{
		reportingCurrency: "IDR",
		multiCurrency:     true,
		positions: []reportPosition{
			{id: idr, group: groupAsset, subtype: "bank_account", ownershipType: "joint"},
			{id: usd, group: groupAsset, subtype: "bank_account", ownershipType: "joint"},
			{id: usdDebt, group: groupLiability, subtype: "personal", ownershipType: "joint"},
		},
		snapshots: []reportSnapshot{
			{positionID: idr, yearMonth: jan, amount: dec("500"), currency: "IDR"},
			{positionID: usd, yearMonth: jan, amount: dec("10"), currency: "USD"},
			{positionID: usdDebt, yearMonth: jan, amount: dec("7"), currency: "USD"},
		},
		// The first USD rate is March, so Jan and Feb have no rate at or before.
		fxRates:      []reportFxRate{{currency: "USD", yearMonth: mar, rate: dec("100")}},
		currentMonth: mar,
	}
	reports := generateMonthlyReports(in)
	assets := buildGroupBreakdown(reports, groupAsset, "IDR")
	debts := buildGroupBreakdown(reports, groupLiability, "IDR")

	for _, m := range []time.Time{jan, feb} {
		bm := findBreakdownMonth(t, assets, m)
		if !bm.Total.Equal(dec("500")) {
			t.Errorf("%s: total %s, want 500 (USD excluded, not summed 1:1 as 510)", m.Format("2006-01"), bm.Total)
		}
		if !bm.Total.Equal(findMonth(t, reports, m).nwAssets) {
			t.Errorf("%s: hub total differs from the report's nw_assets", m.Format("2006-01"))
		}
		if len(bm.MissingFx) != 1 || bm.MissingFx[0].Currency != "USD" ||
			bm.MissingFx[0].PositionID == nil || *bm.MissingFx[0].PositionID != usd {
			t.Errorf("%s: missing_fx %+v, want exactly the USD asset", m.Format("2006-01"), bm.MissingFx)
		}
		dm := findBreakdownMonth(t, debts, m)
		if !dm.Total.IsZero() {
			t.Errorf("%s: liabilities total %s, want 0 (its only position is unconvertible)", m.Format("2006-01"), dm.Total)
		}
		if len(dm.MissingFx) != 1 || *dm.MissingFx[0].PositionID != usdDebt {
			t.Errorf("%s: liabilities missing_fx %+v, want exactly the USD liability", m.Format("2006-01"), dm.MissingFx)
		}
	}

	// Once the rate exists the position is counted and the warning clears.
	bm := findBreakdownMonth(t, assets, mar)
	if !bm.Total.Equal(dec("1500")) { // 500 + 10 × 100
		t.Errorf("Mar: total %s, want 1500", bm.Total)
	}
	if len(bm.MissingFx) != 0 {
		t.Errorf("Mar: missing_fx %+v, want none once the rate exists", bm.MissingFx)
	}
}

// The engine's range starts at the household's first snapshot of any group; a
// group's breakdown starts at its own first contributing month, and a group with
// no positions has no months at all.
// covers: INV-FINANCE-39
func TestGroupBreakdown_StartsAtGroupsFirstMonthAndEmptyGroup(t *testing.T) {
	asset, debt := uuid.New(), uuid.New()
	jan, mar := ym(2026, time.January), ym(2026, time.March)
	in := reportEngineInput{
		positions: []reportPosition{
			{id: asset, group: groupAsset, subtype: "bank_account", ownershipType: "joint"},
			{id: debt, group: groupLiability, subtype: "institutional", ownershipType: "joint"},
		},
		snapshots: []reportSnapshot{
			{positionID: asset, yearMonth: jan, amount: dec("100")},
			{positionID: debt, yearMonth: mar, amount: dec("40")},
		},
		currentMonth: mar,
	}
	reports := generateMonthlyReports(in)

	debts := buildGroupBreakdown(reports, groupLiability, "IDR")
	if len(debts.Months) != 1 || !debts.Months[0].YearMonth.Equal(mar) {
		t.Fatalf("liabilities months: got %+v, want only March", debts.Months)
	}
	if !debts.Months[0].BySubtype["institutional"].Equal(dec("40")) {
		t.Errorf("institutional: got %s, want 40 (positive magnitude)", debts.Months[0].BySubtype["institutional"])
	}

	none := buildGroupBreakdown(reports, groupInvestment, "IDR")
	if len(none.Months) != 0 || none.Months == nil {
		t.Errorf("investment months: got %v, want an empty (non-nil) list", none.Months)
	}
}

// Month boundary + lifecycle: a position terminated in February contributes its
// termination-month (0-value close) and nothing after, and a negative balance
// (overdraft) is summed, not clamped.
// covers: INV-FINANCE-39, INV-FINANCE-05
func TestGroupBreakdown_TerminationAndNegative(t *testing.T) {
	closed, overdrawn := uuid.New(), uuid.New()
	jan, feb, mar := ym(2026, time.January), ym(2026, time.February), ym(2026, time.March)
	term := feb
	in := reportEngineInput{
		positions: []reportPosition{
			{id: closed, group: groupAsset, subtype: "bank_account", ownershipType: "joint", terminatedAt: &term, status: "closed"},
			{id: overdrawn, group: groupAsset, subtype: "bank_account", ownershipType: "joint"},
		},
		snapshots: []reportSnapshot{
			{positionID: closed, yearMonth: jan, amount: dec("300")},
			{positionID: closed, yearMonth: feb, amount: dec("0")},
			{positionID: overdrawn, yearMonth: jan, amount: dec("-50.25")},
		},
		currentMonth: mar,
	}
	reports := generateMonthlyReports(in)
	b := buildGroupBreakdown(reports, groupAsset, "IDR")

	for m, want := range map[time.Time]string{jan: "249.75", feb: "-50.25", mar: "-50.25"} {
		bm := findBreakdownMonth(t, b, m)
		if !bm.Total.Equal(dec(want)) {
			t.Errorf("%s: total %s, want %s", m.Format("2006-01"), bm.Total, want)
		}
		if !bm.Total.Equal(findMonth(t, reports, m).nwAssets) {
			t.Errorf("%s: hub total differs from the report's nw_assets", m.Format("2006-01"))
		}
	}
}

// Investments carry the risk partition as well as the subtype one; both sum to
// the report's nw_investments.
// covers: INV-FINANCE-39, INV-FINANCE-29
func TestGroupBreakdown_InvestmentSubtypeAndRisk(t *testing.T) {
	stock, bond, gold := uuid.New(), uuid.New(), uuid.New()
	jan := ym(2026, time.January)
	in := reportEngineInput{
		reportingCurrency: "IDR",
		multiCurrency:     true,
		positions: []reportPosition{
			{id: stock, group: groupInvestment, subtype: "stock", riskProfile: "high", ownershipType: "joint"},
			{id: bond, group: groupInvestment, subtype: "bond", riskProfile: "low", ownershipType: "joint"},
			{id: gold, group: groupInvestment, subtype: "gold", riskProfile: "low", ownershipType: "joint"},
		},
		snapshots: []reportSnapshot{
			{positionID: stock, yearMonth: jan, amount: dec("10"), currency: "USD"},
			{positionID: bond, yearMonth: jan, amount: dec("200"), currency: "IDR"},
			{positionID: gold, yearMonth: jan, amount: dec("300"), currency: "IDR"},
		},
		fxRates:      []reportFxRate{{currency: "USD", yearMonth: jan, rate: dec("1500")}},
		currentMonth: jan,
	}
	reports := generateMonthlyReports(in)
	b := buildGroupBreakdown(reports, groupInvestment, "IDR")
	bm := findBreakdownMonth(t, b, jan)

	if !bm.Total.Equal(dec("15500")) || !bm.Total.Equal(findMonth(t, reports, jan).nwInvestments) {
		t.Errorf("total %s, want 15500 == nw_investments", bm.Total)
	}
	if !bm.BySubtype["stock"].Equal(dec("15000")) || !bm.BySubtype["bond"].Equal(dec("200")) || !bm.BySubtype["gold"].Equal(dec("300")) {
		t.Errorf("by_subtype %v", bm.BySubtype)
	}
	if !bm.ByRisk["high"].Equal(dec("15000")) || !bm.ByRisk["low"].Equal(dec("500")) || !bm.ByRisk["medium"].IsZero() {
		t.Errorf("by_risk %v", bm.ByRisk)
	}
	if !sumDecimals(bm.ByRisk).Equal(bm.Total) || !sumDecimals(bm.BySubtype).Equal(bm.Total) {
		t.Errorf("partitions do not sum to total %s", bm.Total)
	}
	// Assets/liabilities never carry a risk split.
	if got := buildGroupBreakdown(reports, groupAsset, "IDR"); len(got.Months) != 0 {
		t.Errorf("asset breakdown of an investments-only household has months: %v", got.Months)
	}
}

func TestHubGroup(t *testing.T) {
	for _, name := range []string{"asset", "liability", "investment"} {
		if _, ok := hubGroup(name); !ok {
			t.Errorf("hubGroup(%q) not recognised", name)
		}
	}
	for _, name := range []string{"", "receivable", "assets", "bogus"} {
		if _, ok := hubGroup(name); ok {
			t.Errorf("hubGroup(%q) should be rejected", name)
		}
	}
}
