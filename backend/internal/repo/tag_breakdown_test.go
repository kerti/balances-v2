package repo

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Pure tests for the Tags report's converted breakdown (INV-FINANCE-40). The
// breakdown is projected from the very months generateMonthlyReports returns, so
// every test checks the projection against the report's own nw_total.

func tagEntry(t *testing.T, b *TagBreakdown, id *uuid.UUID) TagBreakdownEntry {
	t.Helper()
	for _, e := range b.Tags {
		if (e.TagID == nil && id == nil) || (e.TagID != nil && id != nil && *e.TagID == *id) {
			return e
		}
	}
	t.Fatalf("no tag entry for %v in %+v", id, b.Tags)
	return TagBreakdownEntry{}
}

// tagsNet is Σ holdings − Σ liabilities across every entry, Untagged included.
func tagsNet(b *TagBreakdown) decimal.Decimal {
	net := decimal.Zero
	for _, e := range b.Tags {
		net = net.Add(e.Holdings).Sub(e.Liabilities)
	}
	return net
}

// One tag holding IDR and USD positions yields ONE converted value; liabilities
// stay a separate positive magnitude; the Untagged bucket carries its own; and
// across all tags the net is exactly the report's nw_total.
// covers: INV-FINANCE-40
func TestTagBreakdown_ConvertsAcrossCurrenciesAndEqualsNetWorth(t *testing.T) {
	tagA, tagB := uuid.New(), uuid.New()
	idrAcct, usdAcct, debt, recv, untaggedInv := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	jan, mar := ym(2026, time.January), ym(2026, time.March)
	in := reportEngineInput{
		reportingCurrency: "IDR",
		multiCurrency:     true,
		positions: []reportPosition{
			{id: idrAcct, group: groupAsset, subtype: "bank_account", ownershipType: "joint", tagID: &tagA},
			{id: usdAcct, group: groupAsset, subtype: "bank_account", ownershipType: "joint", tagID: &tagA},
			{id: debt, group: groupLiability, subtype: "personal", ownershipType: "joint", tagID: &tagA},
			{id: recv, group: groupReceivable, ownershipType: "joint", tagID: &tagB},
			{id: untaggedInv, group: groupInvestment, subtype: "stock", riskProfile: "high", ownershipType: "joint"},
		},
		snapshots: []reportSnapshot{
			{positionID: idrAcct, yearMonth: jan, amount: dec("1000000"), currency: "IDR"},
			{positionID: usdAcct, yearMonth: jan, amount: dec("100"), currency: "USD"},
			{positionID: debt, yearMonth: jan, amount: dec("400000"), currency: "IDR"},
			{positionID: recv, yearMonth: jan, amount: dec("50"), currency: "IDR"},
			{positionID: untaggedInv, yearMonth: jan, amount: dec("10"), currency: "USD"},
			{positionID: usdAcct, yearMonth: mar, amount: dec("120"), currency: "USD"},
		},
		fxRates: []reportFxRate{
			{currency: "USD", yearMonth: jan, rate: dec("15000")},
			{currency: "USD", yearMonth: mar, rate: dec("16000")},
		},
		currentMonth: mar,
	}
	reports := generateMonthlyReports(in)
	b := buildTagBreakdown(reports, "IDR")

	if b.ReportingCurrency != "IDR" || b.YearMonth == nil || !b.YearMonth.Equal(mar) {
		t.Fatalf("header: currency %s, month %v; want IDR / March", b.ReportingCurrency, b.YearMonth)
	}
	a := tagEntry(t, b, &tagA)
	// 1,000,000 + 120 × 16,000: IDR and USD positions fold into ONE figure, and
	// the carried-forward USD rate does not matter because March has its own.
	if !a.Holdings.Equal(dec("2920000")) {
		t.Errorf("tag A holdings %s, want 2920000", a.Holdings)
	}
	if !a.Liabilities.Equal(dec("400000")) {
		t.Errorf("tag A liabilities %s, want 400000 (positive, not netted into holdings)", a.Liabilities)
	}
	if bb := tagEntry(t, b, &tagB); !bb.Holdings.Equal(dec("50")) || !bb.Liabilities.IsZero() {
		t.Errorf("tag B %+v, want holdings 50 / liabilities 0", bb)
	}
	// The untagged investment: 10 USD carried forward at March's 16,000.
	if u := tagEntry(t, b, nil); !u.Holdings.Equal(dec("160000")) || !u.Liabilities.IsZero() {
		t.Errorf("Untagged %+v, want holdings 160000", u)
	}
	if len(b.Tags) != 3 {
		t.Errorf("got %d entries, want 3 (A, B, Untagged): %+v", len(b.Tags), b.Tags)
	}
	// Untagged sorts last.
	if b.Tags[len(b.Tags)-1].TagID != nil {
		t.Errorf("Untagged is not last: %+v", b.Tags)
	}

	nw := findMonth(t, reports, mar).nwTotal
	if got := tagsNet(b); !got.Equal(nw) {
		t.Errorf("Σ(holdings − liabilities) = %s, nw_total = %s", got, nw)
	}
	if !nw.Equal(dec("2680050")) {
		t.Errorf("nw_total %s, want 2680050 (fixture sanity)", nw)
	}
	if len(b.MissingFx) != 0 {
		t.Errorf("unexpected missing_fx %+v", b.MissingFx)
	}
	if got := b.FxRatesUsed["USD"]; !got.Equal(dec("16000")) || len(b.FxRatesUsed) != 1 {
		t.Errorf("fx_rates_used %v, want only USD=16000", b.FxRatesUsed)
	}
}

// A USD position with no rate at or before the month is excluded from its tag's
// figures — never summed 1:1 — and listed with the tag it would have landed in.
// The net still equals nw_total, which excludes it too. A rate-less position of
// the Untagged bucket is listed with a nil tag.
// covers: INV-FINANCE-40, INV-FINANCE-16
func TestTagBreakdown_MissingFxExcludedAndNamed(t *testing.T) {
	tagA := uuid.New()
	idr, usd, usdDebt, usdUntagged := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	jan := ym(2026, time.January)
	in := reportEngineInput{
		reportingCurrency: "IDR",
		multiCurrency:     true,
		positions: []reportPosition{
			{id: idr, group: groupAsset, subtype: "bank_account", ownershipType: "joint", tagID: &tagA},
			{id: usd, group: groupAsset, subtype: "bank_account", ownershipType: "joint", tagID: &tagA},
			{id: usdDebt, group: groupLiability, subtype: "personal", ownershipType: "joint", tagID: &tagA},
			{id: usdUntagged, group: groupAsset, subtype: "bank_account", ownershipType: "joint"},
		},
		snapshots: []reportSnapshot{
			{positionID: idr, yearMonth: jan, amount: dec("500"), currency: "IDR"},
			{positionID: usd, yearMonth: jan, amount: dec("10"), currency: "USD"},
			{positionID: usdDebt, yearMonth: jan, amount: dec("7"), currency: "USD"},
			{positionID: usdUntagged, yearMonth: jan, amount: dec("3"), currency: "USD"},
		},
		// No USD rate at all.
		currentMonth: jan,
	}
	reports := generateMonthlyReports(in)
	b := buildTagBreakdown(reports, "IDR")

	a := tagEntry(t, b, &tagA)
	if !a.Holdings.Equal(dec("500")) || !a.Liabilities.IsZero() {
		t.Errorf("tag A %+v, want holdings 500 / liabilities 0 (USD excluded, not 510 / 7)", a)
	}
	if len(b.Tags) != 1 {
		t.Errorf("entries %+v, want only tag A (the Untagged position is entirely unconvertible)", b.Tags)
	}
	if got, nw := tagsNet(b), findMonth(t, reports, jan).nwTotal; !got.Equal(nw) || !nw.Equal(dec("500")) {
		t.Errorf("net %s vs nw_total %s, want both 500", got, nw)
	}
	if len(b.MissingFx) != 3 {
		t.Fatalf("missing_fx %+v, want the three USD positions", b.MissingFx)
	}
	wantTag := map[uuid.UUID]*uuid.UUID{usd: &tagA, usdDebt: &tagA, usdUntagged: nil}
	for _, mf := range b.MissingFx {
		want, ok := wantTag[mf.PositionID]
		if !ok || mf.Currency != "USD" {
			t.Errorf("unexpected missing_fx entry %+v", mf)
			continue
		}
		if (want == nil) != (mf.TagID == nil) || (want != nil && *want != *mf.TagID) {
			t.Errorf("missing_fx %v carries tag %v, want %v", mf.PositionID, mf.TagID, want)
		}
	}
	if len(b.FxRatesUsed) != 0 {
		t.Errorf("fx_rates_used %v, want none (no rate applied)", b.FxRatesUsed)
	}
}

// Zero, negative and terminated positions at a month boundary: the served month
// is the engine's LAST (a snapshot dated after the current month stretches the
// range, and the dashboard defaults to that last report too), a terminated
// position drops out after its termination month, an overdrawn account is summed
// (not clamped), and a tag whose only position is zero still lists as zero.
// covers: INV-FINANCE-40, INV-FINANCE-05
func TestTagBreakdown_LatestMonthTerminationZeroAndNegative(t *testing.T) {
	tagA, tagZ := uuid.New(), uuid.New()
	closed, overdrawn, zero := uuid.New(), uuid.New(), uuid.New()
	jan, feb, mar := ym(2026, time.January), ym(2026, time.February), ym(2026, time.March)
	term := feb
	in := reportEngineInput{
		positions: []reportPosition{
			{id: closed, group: groupAsset, subtype: "bank_account", ownershipType: "joint", tagID: &tagA, terminatedAt: &term, status: "closed"},
			{id: overdrawn, group: groupAsset, subtype: "bank_account", ownershipType: "joint", tagID: &tagA},
			{id: zero, group: groupAsset, subtype: "bank_account", ownershipType: "joint", tagID: &tagZ},
		},
		snapshots: []reportSnapshot{
			{positionID: closed, yearMonth: jan, amount: dec("300")},
			{positionID: closed, yearMonth: feb, amount: dec("0")},
			{positionID: overdrawn, yearMonth: jan, amount: dec("-50.25")},
			{positionID: zero, yearMonth: jan, amount: dec("0")},
			// A snapshot dated March while the current month is February.
			{positionID: overdrawn, yearMonth: mar, amount: dec("-80.5")},
		},
		currentMonth: feb,
	}
	reports := generateMonthlyReports(in)
	b := buildTagBreakdown(reports, "IDR")

	if b.YearMonth == nil || !b.YearMonth.Equal(mar) {
		t.Fatalf("served month %v, want March (the last generated, the dashboard default)", b.YearMonth)
	}
	if a := tagEntry(t, b, &tagA); !a.Holdings.Equal(dec("-80.5")) {
		t.Errorf("tag A holdings %s, want -80.5 (closed account gone, overdraft summed)", a.Holdings)
	}
	if z := tagEntry(t, b, &tagZ); !z.Holdings.IsZero() || !z.Liabilities.IsZero() {
		t.Errorf("tag Z %+v, want zero", z)
	}
	if got, nw := tagsNet(b), findMonth(t, reports, mar).nwTotal; !got.Equal(nw) {
		t.Errorf("net %s != nw_total %s", got, nw)
	}
}

// With multi-currency off the engine passes native amounts through unconverted
// and can never miss a rate; the Tags report does exactly what the dashboard
// does — no missing_fx, no rates line.
// covers: INV-FINANCE-40
func TestTagBreakdown_MultiCurrencyOffPassesThrough(t *testing.T) {
	tagA := uuid.New()
	a1, a2 := uuid.New(), uuid.New()
	jan := ym(2026, time.January)
	in := reportEngineInput{
		reportingCurrency: "IDR",
		multiCurrency:     false,
		positions: []reportPosition{
			{id: a1, group: groupAsset, subtype: "bank_account", ownershipType: "joint", tagID: &tagA},
			{id: a2, group: groupAsset, subtype: "bank_account", ownershipType: "joint", tagID: &tagA},
		},
		snapshots: []reportSnapshot{
			{positionID: a1, yearMonth: jan, amount: dec("100"), currency: "IDR"},
			{positionID: a2, yearMonth: jan, amount: dec("5"), currency: "USD"},
		},
		currentMonth: jan,
	}
	reports := generateMonthlyReports(in)
	b := buildTagBreakdown(reports, "IDR")

	if a := tagEntry(t, b, &tagA); !a.Holdings.Equal(dec("105")) {
		t.Errorf("holdings %s, want 105 (native amounts summed, as the dashboard does)", a.Holdings)
	}
	if len(b.MissingFx) != 0 || len(b.FxRatesUsed) != 0 {
		t.Errorf("missing_fx %v / fx_rates_used %v, want none", b.MissingFx, b.FxRatesUsed)
	}
	if got, nw := tagsNet(b), findMonth(t, reports, jan).nwTotal; !got.Equal(nw) {
		t.Errorf("net %s != nw_total %s", got, nw)
	}
}

// No snapshot data: no month, empty (non-nil) collections so the wire shape is
// stable.
func TestTagBreakdown_EmptyHousehold(t *testing.T) {
	b := buildTagBreakdown(nil, "IDR")
	if b.YearMonth != nil || b.Tags == nil || len(b.Tags) != 0 || b.MissingFx == nil || b.FxRatesUsed == nil {
		t.Errorf("empty breakdown %+v, want nil month and empty non-nil collections", b)
	}
	if b.ReportingCurrency != "IDR" {
		t.Errorf("reporting currency %q, want IDR", b.ReportingCurrency)
	}
}
