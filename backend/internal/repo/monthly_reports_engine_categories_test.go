package repo

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

// TestEngine_IncomeCategoriesAndReturnSubtypes drives every arm of the two
// per-category accumulators (earnedIncomeAmounts.add / investmentReturnAmounts.add):
// one income row per earned-income category and one investment position per
// subtype, each with a Jan→Feb value change. January is the suppressed baseline;
// the income statement (incl. both accumulators) is computed for February.
// covers: INV-FINANCE-14
func TestEngine_IncomeCategoriesAndReturnSubtypes(t *testing.T) {
	jan, feb := ym(2026, time.January), ym(2026, time.February)

	// One investment position per subtype, each +10 Jan→Feb. With no
	// transactions the whole delta is return, attributed by subtype.
	subtypes := []string{"stock", "mutual_fund", "bond", "gold", "time_deposit"}
	var positions []reportPosition
	var snapshots []reportSnapshot
	for _, st := range subtypes {
		id := uuid.New()
		positions = append(positions, reportPosition{
			id: id, group: groupInvestment, subtype: st, ownershipType: "joint",
		})
		snapshots = append(snapshots,
			reportSnapshot{positionID: id, yearMonth: jan, amount: dec("100")},
			reportSnapshot{positionID: id, yearMonth: feb, amount: dec("110")},
		)
	}

	// One income row per earned-income category, distinct amounts in February.
	catAmt := map[string]string{
		"salary":           "10",
		"business_income":  "20",
		"rental_income":    "30",
		"gift":             "40",
		"tax_refund":       "50",
		"insurance_payout": "60",
		"other":            "70",
	}
	var income []reportIncome
	for cat, amt := range catAmt {
		income = append(income, reportIncome{
			yearMonth: feb, amount: dec(amt), category: cat, ownershipType: "joint",
		})
	}

	r := findMonth(t, generateMonthlyReports(reportEngineInput{
		positions:    positions,
		snapshots:    snapshots,
		income:       income,
		currentMonth: feb,
	}), feb)

	ei := r.earnedIncome
	if !ei.salary.Equal(dec("10")) || !ei.business.Equal(dec("20")) ||
		!ei.rental.Equal(dec("30")) || !ei.gift.Equal(dec("40")) ||
		!ei.taxRefund.Equal(dec("50")) || !ei.insurance.Equal(dec("60")) ||
		!ei.other.Equal(dec("70")) {
		t.Errorf("earned income per-category mismatch: %+v", ei)
	}
	if !ei.total.Equal(dec("280")) { // 10+20+30+40+50+60+70
		t.Errorf("earned income total: got %s, want 280", ei.total)
	}

	if r.investmentReturn == nil {
		t.Fatal("investmentReturn is nil on a non-baseline month")
	}
	ir := *r.investmentReturn
	if !ir.stock.Equal(dec("10")) || !ir.mutualFund.Equal(dec("10")) ||
		!ir.bond.Equal(dec("10")) || !ir.gold.Equal(dec("10")) ||
		!ir.timeDeposit.Equal(dec("10")) {
		t.Errorf("investment return per-subtype mismatch: %+v", ir)
	}
	if !ir.total.Equal(dec("50")) { // 5 subtypes × 10
		t.Errorf("investment return total: got %s, want 50", ir.total)
	}
}
