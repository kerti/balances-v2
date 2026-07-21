package repo

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

// The engine materializes investment return and closing value split two ways —
// by risk profile and by instrument type — and both partitions reconcile to the
// same totals (return -> investment_return_total, value -> nw_investments),
// because every Investment has exactly one risk_profile and one subtype.
// covers: INV-FINANCE-29
func TestEngine_InvestmentPerformanceBreakdown(t *testing.T) {
	bank, stockLow, fundHigh := uuid.New(), uuid.New(), uuid.New()
	in := reportEngineInput{
		positions: []reportPosition{
			{id: bank, group: groupAsset, subtype: "bank_account", ownershipType: "joint"},
			{id: stockLow, group: groupInvestment, subtype: "stock", riskProfile: "low", ownershipType: "joint"},
			{id: fundHigh, group: groupInvestment, subtype: "mutual_fund", riskProfile: "high", ownershipType: "joint"},
		},
		snapshots: []reportSnapshot{
			{positionID: bank, yearMonth: ym(2026, time.January), amount: dec("1000")},
			{positionID: bank, yearMonth: ym(2026, time.February), amount: dec("1000")},
			// Both investments are born in Jan (baseline) and appreciate in Feb.
			{positionID: stockLow, yearMonth: ym(2026, time.January), amount: dec("500")},
			{positionID: stockLow, yearMonth: ym(2026, time.February), amount: dec("560")}, // +60
			{positionID: fundHigh, yearMonth: ym(2026, time.January), amount: dec("300")},
			{positionID: fundHigh, yearMonth: ym(2026, time.February), amount: dec("330")}, // +30
		},
		currentMonth: ym(2026, time.February),
	}
	reports := generateMonthlyReports(in)
	jan := findMonth(t, reports, ym(2026, time.January))
	feb := findMonth(t, reports, ym(2026, time.February))

	// Baseline (Jan): return suppressed, but closing value is set on every month
	// (it seeds Feb's opening rate base).
	if jan.investmentReturn != nil {
		t.Errorf("Jan baseline: investmentReturn should be nil, got %+v", jan.investmentReturn)
	}
	if !jan.investmentValue.stock.Equal(dec("500")) || !jan.investmentValue.low.Equal(dec("500")) {
		t.Errorf("Jan closing value: stock=%s low=%s, want 500/500", jan.investmentValue.stock, jan.investmentValue.low)
	}
	if !jan.investmentValue.mutualFund.Equal(dec("300")) || !jan.investmentValue.high.Equal(dec("300")) {
		t.Errorf("Jan closing value: mutualFund=%s high=%s, want 300/300", jan.investmentValue.mutualFund, jan.investmentValue.high)
	}

	// Feb return by risk: low 60, high 30, medium 0.
	if r := feb.investmentReturn; r == nil ||
		!r.low.Equal(dec("60")) || !r.high.Equal(dec("30")) || !r.medium.Equal(dec("0")) {
		t.Fatalf("Feb return by risk: %+v, want low=60 high=30 medium=0", feb.investmentReturn)
	}
	// Return reconciles across the risk partition (INV-FINANCE-29).
	if riskSum := feb.investmentReturn.low.Add(feb.investmentReturn.medium).Add(feb.investmentReturn.high); !riskSum.Equal(feb.investmentReturn.total) {
		t.Errorf("return risk partition: Σ=%s != total=%s", riskSum, feb.investmentReturn.total)
	}

	// Feb closing value both partitions reconcile to nw_investments (890).
	if !feb.nwInvestments.Equal(dec("890")) {
		t.Fatalf("Feb nwInvestments: %s, want 890", feb.nwInvestments)
	}
	v := feb.investmentValue
	subtypeSum := v.stock.Add(v.mutualFund).Add(v.bond).Add(v.gold).Add(v.timeDeposit)
	riskSum := v.low.Add(v.medium).Add(v.high)
	if !subtypeSum.Equal(feb.nwInvestments) {
		t.Errorf("value subtype partition: Σ=%s != nwInvestments=%s", subtypeSum, feb.nwInvestments)
	}
	if !riskSum.Equal(feb.nwInvestments) {
		t.Errorf("value risk partition: Σ=%s != nwInvestments=%s", riskSum, feb.nwInvestments)
	}
}
