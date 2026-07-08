package repo

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

// Pure unit tests for generatePositionDetail — the itemized per-position
// breakdown behind the PDF export (ADR-0045). Reuses ym/dec from
// monthly_reports_engine_test.go.

func findDetail(t *testing.T, out []PositionDetail, id uuid.UUID) PositionDetail {
	t.Helper()
	for _, d := range out {
		if d.ID == id {
			return d
		}
	}
	t.Fatalf("no position detail for %s", id)
	return PositionDetail{}
}

// A fresh snapshot at the target month is not stale, and the itemized value
// matches what the aggregate pass would sum for that position.
// covers: INV-FINANCE-18
func TestPositionDetail_Fresh(t *testing.T) {
	a := uuid.New()
	in := reportEngineInput{
		positions: []reportPosition{{id: a, name: "Checking", group: groupAsset, subtype: "bank_account", ownershipType: "joint", currency: "USD"}},
		snapshots: []reportSnapshot{{positionID: a, yearMonth: ym(2026, time.March), amount: dec("300"), currency: "USD"}},
	}
	out := generatePositionDetail(in, ym(2026, time.March))
	if len(out) != 1 {
		t.Fatalf("got %d positions, want 1", len(out))
	}
	d := findDetail(t, out, a)
	if d.Stale {
		t.Errorf("fresh snapshot flagged stale")
	}
	if !d.Amount.Equal(dec("300")) {
		t.Errorf("amount: got %s, want 300", d.Amount)
	}
	if d.Group != "asset" || d.Subtype != "bank_account" {
		t.Errorf("group/subtype: got %s/%s", d.Group, d.Subtype)
	}
}

// A gap month carries the latest snapshot <= target and flags it stale, with
// StaleMonth naming the month it was actually recorded — same rule as
// INV-FINANCE-03 for the aggregate report.
// covers: INV-FINANCE-18
func TestPositionDetail_CarriedForwardIsStale(t *testing.T) {
	a := uuid.New()
	in := reportEngineInput{
		positions: []reportPosition{{id: a, group: groupAsset, ownershipType: "joint"}},
		snapshots: []reportSnapshot{{positionID: a, yearMonth: ym(2026, time.January), amount: dec("100")}},
	}
	out := generatePositionDetail(in, ym(2026, time.March))
	d := findDetail(t, out, a)
	if !d.Stale {
		t.Errorf("carried-forward position not flagged stale")
	}
	if d.StaleMonth == nil || monthIndex(*d.StaleMonth) != monthIndex(ym(2026, time.January)) {
		t.Errorf("stale month: got %v, want January", d.StaleMonth)
	}
	if !d.Amount.Equal(dec("100")) {
		t.Errorf("amount: got %s, want 100 (carried)", d.Amount)
	}
}

// A position with no snapshot at or before the target month is excluded
// entirely — same birth-month rule as INV-FINANCE-04.
// covers: INV-FINANCE-18
func TestPositionDetail_ExcludesBeforeBirth(t *testing.T) {
	a := uuid.New()
	in := reportEngineInput{
		positions: []reportPosition{{id: a, group: groupAsset, ownershipType: "joint"}},
		snapshots: []reportSnapshot{{positionID: a, yearMonth: ym(2026, time.March), amount: dec("500")}},
	}
	out := generatePositionDetail(in, ym(2026, time.January))
	if len(out) != 0 {
		t.Fatalf("got %d positions, want 0 (position born after target month)", len(out))
	}
}

// A position terminated before the target month is excluded — same rule as
// INV-FINANCE-05.
// covers: INV-FINANCE-18
func TestPositionDetail_ExcludesTerminated(t *testing.T) {
	a := uuid.New()
	feb := ym(2026, time.February)
	in := reportEngineInput{
		positions: []reportPosition{{id: a, group: groupAsset, ownershipType: "joint", terminatedAt: &feb}},
		snapshots: []reportSnapshot{{positionID: a, yearMonth: ym(2026, time.January), amount: dec("100")}},
	}
	if out := generatePositionDetail(in, feb); len(out) != 1 {
		t.Errorf("termination month: got %d positions, want 1 (still contributes)", len(out))
	}
	if out := generatePositionDetail(in, ym(2026, time.March)); len(out) != 0 {
		t.Errorf("post-termination month: got %d positions, want 0", len(out))
	}
}

// A foreign amount with no rate at or before the target month is excluded
// (never summed 1:1) — same rule as INV-FINANCE-16.
// covers: INV-FINANCE-18
func TestPositionDetail_ExcludesMissingFx(t *testing.T) {
	a := uuid.New()
	in := reportEngineInput{
		positions:         []reportPosition{{id: a, group: groupAsset, ownershipType: "joint"}},
		snapshots:         []reportSnapshot{{positionID: a, yearMonth: ym(2026, time.January), amount: dec("100"), currency: "EUR"}},
		reportingCurrency: "USD",
		multiCurrency:     true,
		// No fxRates entry for EUR at all.
	}
	out := generatePositionDetail(in, ym(2026, time.January))
	if len(out) != 0 {
		t.Fatalf("got %d positions, want 0 (unconvertible currency excluded)", len(out))
	}
}

// A foreign amount converts at the latest rate <= target month, same as the
// aggregate pass (INV-FINANCE-15).
// covers: INV-FINANCE-18
func TestPositionDetail_ConvertsForeignCurrency(t *testing.T) {
	a := uuid.New()
	in := reportEngineInput{
		positions:         []reportPosition{{id: a, group: groupAsset, ownershipType: "joint"}},
		snapshots:         []reportSnapshot{{positionID: a, yearMonth: ym(2026, time.January), amount: dec("100"), currency: "EUR"}},
		fxRates:           []reportFxRate{{currency: "EUR", yearMonth: ym(2026, time.January), rate: dec("1.1")}},
		reportingCurrency: "USD",
		multiCurrency:     true,
	}
	out := generatePositionDetail(in, ym(2026, time.January))
	d := findDetail(t, out, a)
	if d.NativeCurrency != "EUR" || !d.NativeAmount.Equal(dec("100")) {
		t.Errorf("native: got %s %s, want EUR 100", d.NativeAmount, d.NativeCurrency)
	}
	if !d.Amount.Equal(dec("110")) {
		t.Errorf("converted amount: got %s, want 110", d.Amount)
	}
}

// Summing itemized details per group reproduces the aggregate report's nw_*
// totals for the same month — the core guarantee of INV-FINANCE-18: the
// itemized endpoint and the aggregate report can never silently diverge
// because they share the same resolution function.
func TestPositionDetail_SumsMatchAggregateReport(t *testing.T) {
	assetA, assetB, liab, inv := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	in := reportEngineInput{
		positions: []reportPosition{
			{id: assetA, group: groupAsset, subtype: "bank_account", ownershipType: "joint"},
			{id: assetB, group: groupAsset, subtype: "property", ownershipType: "joint"},
			{id: liab, group: groupLiability, ownershipType: "joint"},
			{id: inv, group: groupInvestment, subtype: "stock", ownershipType: "joint"},
		},
		snapshots: []reportSnapshot{
			{positionID: assetA, yearMonth: ym(2026, time.January), amount: dec("100")},
			{positionID: assetB, yearMonth: ym(2026, time.February), amount: dec("2000")},
			{positionID: liab, yearMonth: ym(2026, time.January), amount: dec("50")},
			{positionID: inv, yearMonth: ym(2026, time.January), amount: dec("300")},
		},
		currentMonth: ym(2026, time.March),
	}

	reports := generateMonthlyReports(in)
	agg := findMonth(t, reports, ym(2026, time.March))
	detail := generatePositionDetail(in, ym(2026, time.March))

	sumAssets, sumLiab, sumInv := dec("0"), dec("0"), dec("0")
	for _, d := range detail {
		switch d.Group {
		case "asset":
			sumAssets = sumAssets.Add(d.Amount)
		case "liability":
			sumLiab = sumLiab.Add(d.Amount)
		case "investment":
			sumInv = sumInv.Add(d.Amount)
		}
	}
	if !sumAssets.Equal(agg.nwAssets) {
		t.Errorf("summed assets: got %s, want %s (aggregate nwAssets)", sumAssets, agg.nwAssets)
	}
	if !sumLiab.Equal(agg.nwLiabilities) {
		t.Errorf("summed liabilities: got %s, want %s (aggregate nwLiabilities)", sumLiab, agg.nwLiabilities)
	}
	if !sumInv.Equal(agg.nwInvestments) {
		t.Errorf("summed investments: got %s, want %s (aggregate nwInvestments)", sumInv, agg.nwInvestments)
	}
}
