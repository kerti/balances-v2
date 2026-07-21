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

// investment_placement counts new bank-sourced money into investments — Buys +
// fresh TD placements — and MUST exclude TD rollover funding (rolled_to_new,
// internal recycling that never touches the bank), else an auto-renewing TD reads
// as a huge recurring placement.
// covers: INV-FINANCE-32
func TestEngine_PlacementExcludesRollover(t *testing.T) {
	bank, stock := uuid.New(), uuid.New()
	freshTD := uuid.New()
	oldTD, newTD := uuid.New(), uuid.New()
	feb := ym(2026, time.February)
	principal, interest := dec("100"), dec("5")
	tdPrincipal := dec("300")
	buyAmt := dec("200")
	in := reportEngineInput{
		positions: []reportPosition{
			{id: bank, group: groupAsset, subtype: "bank_account", ownershipType: "joint"},
			{id: stock, group: groupInvestment, subtype: "stock", riskProfile: "medium", ownershipType: "joint"},
			// A fresh TD placed in Feb (real bank money → counts).
			{id: freshTD, group: groupInvestment, subtype: "time_deposit", riskProfile: "low", ownershipType: "joint",
				placementAmount: &tdPrincipal, placementMonth: &feb, currency: "IDR"},
			// A rolled-over TD: old matures into new (recycled → must NOT count).
			{id: oldTD, group: groupInvestment, subtype: "time_deposit", riskProfile: "low", ownershipType: "joint", terminatedAt: &feb},
			{id: newTD, group: groupInvestment, subtype: "time_deposit", riskProfile: "low", ownershipType: "joint", rolledFrom: &oldTD},
		},
		snapshots: []reportSnapshot{
			{positionID: bank, yearMonth: ym(2026, time.January), amount: dec("5000")},
			{positionID: bank, yearMonth: feb, amount: dec("5000")},
			{positionID: stock, yearMonth: ym(2026, time.January), amount: dec("500")},
			{positionID: stock, yearMonth: feb, amount: dec("700")}, // +200 from the Buy below
			{positionID: freshTD, yearMonth: feb, amount: dec("300")},
			{positionID: oldTD, yearMonth: ym(2026, time.January), amount: dec("100")},
			{positionID: oldTD, yearMonth: feb, amount: dec("0")},
			{positionID: newTD, yearMonth: feb, amount: dec("105")},
		},
		transactions: []reportTransaction{
			{investmentID: stock, yearMonth: feb, txnType: "buy", amount: &buyAmt}, // a 200 Buy
			{investmentID: oldTD, yearMonth: feb, txnType: "maturity",
				principalAmount: &principal, interestAmount: &interest,
				principalDisposition: strp("rolled_to_new"), interestDisposition: strp("rolled_to_new")},
		},
		currentMonth: feb,
	}
	reports := generateMonthlyReports(in)
	jan := findMonth(t, reports, ym(2026, time.January))
	feb2 := findMonth(t, reports, feb)

	// Baseline: no flow, placement suppressed.
	if jan.investmentPlacement != nil {
		t.Errorf("Jan baseline: investmentPlacement should be nil, got %v", jan.investmentPlacement)
	}
	// Feb placement = 200 (Buy) + 300 (fresh TD) = 500. The 105 rollover funding is
	// excluded.
	if feb2.investmentPlacement == nil || !feb2.investmentPlacement.Equal(dec("500")) {
		t.Fatalf("Feb placement: %v, want 500 (Buy 200 + fresh TD 300; rollover 105 excluded)", feb2.investmentPlacement)
	}
}
