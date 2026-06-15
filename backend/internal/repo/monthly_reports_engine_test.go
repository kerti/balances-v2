package repo

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/kerti/balances-v2/backend/internal/db"
)

// Pure unit tests for the monthly-report engine — no DB. These carry the
// net-worth rule coverage (carry-forward, gaps, birth, lifecycle suppression,
// per-user/Joint attribution incl. liability subtraction, stale flagging); the
// repo integration test only checks the plumbing.

func ym(y int, m time.Month) time.Time { return time.Date(y, m, 1, 0, 0, 0, 0, time.UTC) }
func dec(s string) decimal.Decimal     { return decimal.RequireFromString(s) }
func strp(s string) *string            { return &s }

func findMonth(t *testing.T, reports []monthlyReportData, m time.Time) monthlyReportData {
	t.Helper()
	for _, r := range reports {
		if monthIndex(r.yearMonth) == monthIndex(m) {
			return r
		}
	}
	t.Fatalf("no report generated for %s", m.Format("2006-01"))
	return monthlyReportData{}
}

func contains(stale []stalePosition, id uuid.UUID) bool {
	for _, x := range stale {
		if x.ID == id {
			return true
		}
	}
	return false
}

// Carry-forward covers the current month, a mid-history gap, and a never-yet-
// snapshotted (birth) month with one rule: latest snapshot <= M, stale-flagged
// when older than M.
// covers: INV-FINANCE-03
func TestEngine_CarryForward(t *testing.T) {
	a := uuid.New()
	in := reportEngineInput{
		positions: []reportPosition{{id: a, group: groupAsset, ownershipType: "joint"}},
		snapshots: []reportSnapshot{
			{positionID: a, yearMonth: ym(2026, time.January), amount: dec("100")},
			{positionID: a, yearMonth: ym(2026, time.March), amount: dec("300")},
		},
		currentMonth: ym(2026, time.April),
	}
	reports := generateMonthlyReports(in)
	if len(reports) != 4 {
		t.Fatalf("got %d months, want 4 (Jan..Apr)", len(reports))
	}

	cases := []struct {
		month time.Month
		nw    string
		stale bool
	}{
		{time.January, "100", false}, // fresh
		{time.February, "100", true}, // gap → carries Jan
		{time.March, "300", false},   // fresh
		{time.April, "300", true},    // current → carries Mar
	}
	for _, c := range cases {
		r := findMonth(t, reports, ym(2026, c.month))
		if !r.nwTotal.Equal(dec(c.nw)) {
			t.Errorf("%s nwTotal: got %s, want %s", c.month, r.nwTotal, c.nw)
		}
		if got := contains(r.stalePositions, a); got != c.stale {
			t.Errorf("%s stale(a): got %v, want %v", c.month, got, c.stale)
		}
	}
}

// A position born in February contributes nothing to January.
// covers: INV-FINANCE-04
func TestEngine_BirthMonth(t *testing.T) {
	a := uuid.New()
	in := reportEngineInput{
		positions:    []reportPosition{{id: a, group: groupAsset, ownershipType: "joint"}},
		snapshots:    []reportSnapshot{{positionID: a, yearMonth: ym(2026, time.February), amount: dec("500")}},
		currentMonth: ym(2026, time.February),
	}
	reports := generateMonthlyReports(in)
	// First month with data is February — January is never generated.
	if len(reports) != 1 {
		t.Fatalf("got %d months, want 1 (Feb only)", len(reports))
	}
	if mi := monthIndex(reports[0].yearMonth); mi != monthIndex(ym(2026, time.February)) {
		t.Fatalf("first month is not February")
	}
}

// A terminated position contributes through its termination month, then drops.
// covers: INV-FINANCE-05
func TestEngine_TerminatedSuppression(t *testing.T) {
	a := uuid.New()
	feb := ym(2026, time.February)
	in := reportEngineInput{
		positions:    []reportPosition{{id: a, group: groupAsset, ownershipType: "joint", terminatedAt: &feb}},
		snapshots:    []reportSnapshot{{positionID: a, yearMonth: ym(2026, time.January), amount: dec("100")}},
		currentMonth: ym(2026, time.April),
	}
	reports := generateMonthlyReports(in)

	if r := findMonth(t, reports, ym(2026, time.February)); !r.nwTotal.Equal(dec("100")) {
		t.Errorf("Feb (termination month) nwTotal: got %s, want 100", r.nwTotal)
	}
	if r := findMonth(t, reports, ym(2026, time.March)); !r.nwTotal.Equal(dec("0")) {
		t.Errorf("Mar (post-termination) nwTotal: got %s, want 0", r.nwTotal)
	}
}

// Group sums and the per-user / Joint breakdown (with liability subtraction)
// reconcile with the total.
// covers: INV-FINANCE-01, INV-FINANCE-02, INV-ATTRIBUTION-01, INV-ATTRIBUTION-02
func TestEngine_GroupsAndBreakdown(t *testing.T) {
	alice, bob := uuid.New(), uuid.New()
	a1, a2 := uuid.New(), uuid.New()
	rcv, inv, liab := uuid.New(), uuid.New(), uuid.New()

	jan := ym(2026, time.January)
	in := reportEngineInput{
		members: []uuid.UUID{alice, bob},
		positions: []reportPosition{
			{id: a1, group: groupAsset, ownershipType: "joint"},
			{id: a2, group: groupAsset, ownershipType: "sole", soleOwnerID: &alice},
			{id: rcv, group: groupReceivable, ownershipType: "joint"},
			{id: inv, group: groupInvestment, ownershipType: "joint"},
			{id: liab, group: groupLiability, ownershipType: "sole", soleOwnerID: &bob},
		},
		snapshots: []reportSnapshot{
			{positionID: a1, yearMonth: jan, amount: dec("1000")},
			{positionID: a2, yearMonth: jan, amount: dec("600")},
			{positionID: rcv, yearMonth: jan, amount: dec("50")},
			{positionID: inv, yearMonth: jan, amount: dec("300")},
			{positionID: liab, yearMonth: jan, amount: dec("200")},
		},
		currentMonth: jan,
	}
	r := findMonth(t, generateMonthlyReports(in), jan)

	checks := map[string]struct{ got, want decimal.Decimal }{
		"nwAssets":      {r.nwAssets, dec("1600")},
		"nwReceivables": {r.nwReceivables, dec("50")},
		"nwInvestments": {r.nwInvestments, dec("300")},
		"nwLiabilities": {r.nwLiabilities, dec("200")},                // positive magnitude
		"nwTotal":       {r.nwTotal, dec("1750")},                     // 1600+50+300-200
		"joint":         {r.userBreakdowns[jointKey].NW, dec("1350")}, // 1000+50+300
		"alice":         {r.userBreakdowns[alice.String()].NW, dec("600")},
		"bob":           {r.userBreakdowns[bob.String()].NW, dec("-200")}, // liability subtracts
	}
	for name, c := range checks {
		if !c.got.Equal(c.want) {
			t.Errorf("%s: got %s, want %s", name, c.got, c.want)
		}
	}

	// Breakdown buckets reconcile with the total.
	sum := r.userBreakdowns[jointKey].NW.
		Add(r.userBreakdowns[alice.String()].NW).
		Add(r.userBreakdowns[bob.String()].NW)
	if !sum.Equal(r.nwTotal) {
		t.Errorf("breakdown sum %s != nwTotal %s", sum, r.nwTotal)
	}
}

// Membership seeds the bucket set: a member who owns nothing still gets a
// zero bucket, and sole vs joint income routes by ownerKey just like net worth.
// covers: INV-ATTRIBUTION-03, INV-ATTRIBUTION-01, INV-ATTRIBUTION-02
func TestEngine_AttributionMembershipAndIncomeRouting(t *testing.T) {
	alice, bob, carol := uuid.New(), uuid.New(), uuid.New()
	jan := ym(2026, time.January)
	anchor := uuid.New() // a joint asset to establish the month range
	in := reportEngineInput{
		members: []uuid.UUID{alice, bob, carol}, // carol owns nothing
		positions: []reportPosition{
			{id: anchor, group: groupAsset, ownershipType: "joint"},
		},
		snapshots: []reportSnapshot{
			{positionID: anchor, yearMonth: jan, amount: dec("10")},
		},
		income: []reportIncome{
			{yearMonth: jan, amount: dec("100"), category: "salary", ownershipType: "sole", soleOwnerID: &bob},
			{yearMonth: jan, amount: dec("40"), category: "salary", ownershipType: "joint"},
		},
		currentMonth: jan,
	}
	r := findMonth(t, generateMonthlyReports(in), jan)

	// Carol owns nothing but is a member → present with a zero bucket.
	carolB, ok := r.userBreakdowns[carol.String()]
	if !ok {
		t.Fatalf("carol bucket missing: membership did not seed the bucket set")
	}
	if !carolB.NW.Equal(dec("0")) || !carolB.EarnedIncome.Equal(dec("0")) {
		t.Errorf("carol bucket non-zero: NW=%s income=%s", carolB.NW, carolB.EarnedIncome)
	}

	// Sole income routes to its owner; joint income to the joint bucket; no leak.
	checks := map[string]struct{ got, want decimal.Decimal }{
		"bob income":   {r.userBreakdowns[bob.String()].EarnedIncome, dec("100")},
		"joint income": {r.userBreakdowns[jointKey].EarnedIncome, dec("40")},
		"alice income": {r.userBreakdowns[alice.String()].EarnedIncome, dec("0")},
	}
	for name, c := range checks {
		if !c.got.Equal(c.want) {
			t.Errorf("%s: got %s, want %s", name, c.got, c.want)
		}
	}
	if !r.earnedIncome.total.Equal(dec("140")) {
		t.Errorf("earned income total: got %s, want 140", r.earnedIncome.total)
	}
}

// A sole row with a nil soleOwnerID is malformed; ownerKey degrades it to the
// joint bucket rather than panicking or dropping the value from the total.
// covers: INV-ATTRIBUTION-04
func TestEngine_AttributionMalformedSoleDegradesToJoint(t *testing.T) {
	alice := uuid.New()
	orphan := uuid.New()
	jan := ym(2026, time.January)
	in := reportEngineInput{
		members: []uuid.UUID{alice},
		positions: []reportPosition{
			// sole ownership but no owner id — the malformed row.
			{id: orphan, group: groupAsset, ownershipType: "sole", soleOwnerID: nil},
		},
		snapshots: []reportSnapshot{
			{positionID: orphan, yearMonth: jan, amount: dec("500")},
		},
		currentMonth: jan,
	}
	r := findMonth(t, generateMonthlyReports(in), jan)

	if !r.userBreakdowns[jointKey].NW.Equal(dec("500")) {
		t.Errorf("malformed sole did not degrade to joint: joint NW=%s, want 500", r.userBreakdowns[jointKey].NW)
	}
	if !r.userBreakdowns[alice.String()].NW.Equal(dec("0")) {
		t.Errorf("malformed sole leaked into a member bucket: alice NW=%s, want 0", r.userBreakdowns[alice.String()].NW)
	}
	// Value stays in the household total — reconciliation holds.
	if !r.nwTotal.Equal(dec("500")) {
		t.Errorf("malformed sole dropped from total: nwTotal=%s, want 500", r.nwTotal)
	}
}

// No snapshot data → nothing to report.
// covers: INV-FINANCE-04
func TestEngine_EmptyIsNil(t *testing.T) {
	if got := generateMonthlyReports(reportEngineInput{currentMonth: ym(2026, time.January)}); got != nil {
		t.Errorf("empty input: got %v, want nil", got)
	}
}

// The transaction → cash mapping (ADR-0008): unit-deducting fees move no cash;
// cash fees and Buys move cash in. A maturity always books its full terminal
// value (principal + interest) as cash_out of the matured TD regardless of
// disposition — the value leaves the position whether paid out or rolled over.
// The rollover's matching cash_in into the successor TD is a separate engine
// pass (issue #27 rollover), not part of this per-transaction mapping.
// covers: INV-FINANCE-09
func TestEngine_TransactionCashFlows(t *testing.T) {
	amt := func(s string) *decimal.Decimal { v := dec(s); return &v }
	str := func(s string) *string { return &s }
	cases := []struct {
		name    string
		txn     reportTransaction
		in, out string
	}{
		{"buy", reportTransaction{txnType: "buy", amount: amt("350")}, "350", "0"},
		{"sell", reportTransaction{txnType: "sell", amount: amt("400")}, "0", "400"},
		{"coupon", reportTransaction{txnType: "coupon", amount: amt("50")}, "0", "50"},
		{"dividend", reportTransaction{txnType: "dividend", amount: amt("30")}, "0", "30"},
		{"distribution", reportTransaction{txnType: "distribution", amount: amt("20")}, "0", "20"},
		{"fee cash", reportTransaction{txnType: "fee", amount: amt("10")}, "10", "0"},
		{"fee unit-deducted", reportTransaction{txnType: "fee", amount: amt("10"), quantity: amt("2")}, "0", "0"},
		{"maturity both cash_out", reportTransaction{txnType: "maturity", principalAmount: amt("1000"), interestAmount: amt("55"), principalDisposition: str("cash_out"), interestDisposition: str("cash_out")}, "0", "1055"},
		{"maturity both rolled", reportTransaction{txnType: "maturity", principalAmount: amt("1000"), interestAmount: amt("55"), principalDisposition: str("rolled_to_new"), interestDisposition: str("rolled_to_new")}, "0", "1055"},
		{"maturity P rolled / I cash", reportTransaction{txnType: "maturity", principalAmount: amt("1000"), interestAmount: amt("55"), principalDisposition: str("rolled_to_new"), interestDisposition: str("cash_out")}, "0", "1055"},
	}
	for _, c := range cases {
		cf := transactionCashFlows(c.txn)
		if !cf.in.Equal(dec(c.in)) || !cf.out.Equal(dec(c.out)) {
			t.Errorf("%s: got in=%s out=%s, want in=%s out=%s", c.name, cf.in, cf.out, c.in, c.out)
		}
	}
}

// The income statement: earned income by category, investment return,
// property/vehicle asset value change isolated from the residual, and the
// comprehensive-income identity closing. Baseline month suppresses the derived
// lines. Vehicle depreciation must land in asset_value_change, not expenses.
// covers: INV-FINANCE-06, INV-FINANCE-07, INV-FINANCE-10
func TestEngine_IncomeStatement(t *testing.T) {
	bank, veh, stock := uuid.New(), uuid.New(), uuid.New()
	in := reportEngineInput{
		positions: []reportPosition{
			{id: bank, group: groupAsset, subtype: "bank_account", ownershipType: "joint"},
			{id: veh, group: groupAsset, subtype: "vehicle", ownershipType: "joint"},
			{id: stock, group: groupInvestment, subtype: "stock", ownershipType: "joint"},
		},
		snapshots: []reportSnapshot{
			{positionID: bank, yearMonth: ym(2026, time.January), amount: dec("1000")},
			{positionID: bank, yearMonth: ym(2026, time.February), amount: dec("900")},
			{positionID: veh, yearMonth: ym(2026, time.January), amount: dec("200")},
			{positionID: veh, yearMonth: ym(2026, time.February), amount: dec("197")},
			{positionID: stock, yearMonth: ym(2026, time.January), amount: dec("500")},
			{positionID: stock, yearMonth: ym(2026, time.February), amount: dec("560")},
		},
		income: []reportIncome{
			{yearMonth: ym(2026, time.February), amount: dec("50"), category: "salary", ownershipType: "joint"},
		},
		currentMonth: ym(2026, time.February),
	}
	reports := generateMonthlyReports(in)
	jan := findMonth(t, reports, ym(2026, time.January))
	feb := findMonth(t, reports, ym(2026, time.February))

	// Baseline (Jan): derived lines suppressed; earned income still present (0).
	if jan.investmentReturn != nil || jan.assetValueChange != nil || jan.livingExpenses != nil {
		t.Errorf("Jan baseline: derived lines should be nil, got return=%v assetΔ=%v exp=%v",
			jan.investmentReturn, jan.assetValueChange, jan.livingExpenses)
	}
	if !jan.earnedIncome.total.Equal(dec("0")) {
		t.Errorf("Jan earned income: got %s, want 0", jan.earnedIncome.total)
	}

	// Feb income statement.
	if !feb.earnedIncome.salary.Equal(dec("50")) || !feb.earnedIncome.total.Equal(dec("50")) {
		t.Errorf("Feb earned income: got total=%s salary=%s, want 50/50", feb.earnedIncome.total, feb.earnedIncome.salary)
	}
	if feb.investmentReturn == nil || !feb.investmentReturn.total.Equal(dec("60")) || !feb.investmentReturn.stock.Equal(dec("60")) {
		t.Fatalf("Feb investment return: %+v, want total/stock=60", feb.investmentReturn)
	}
	if feb.assetValueChange == nil || !feb.assetValueChange.Equal(dec("-3")) {
		t.Fatalf("Feb asset value change: %v, want -3 (vehicle only; bank excluded)", feb.assetValueChange)
	}
	if feb.livingExpenses == nil || !feb.livingExpenses.Equal(dec("150")) {
		t.Fatalf("Feb living expenses: %v, want 150 (cash residual, depreciation excluded)", feb.livingExpenses)
	}
	// Identity closes: ΔNW == earned + return + assetΔ − expenses.
	deltaNW := feb.nwTotal.Sub(jan.nwTotal)
	rhs := feb.earnedIncome.total.Add(feb.investmentReturn.total).Add(*feb.assetValueChange).Sub(*feb.livingExpenses)
	if !deltaNW.Equal(rhs) {
		t.Errorf("identity broken: ΔNW=%s != earned+return+assetΔ−expenses=%s", deltaNW, rhs)
	}
}

// Multi-currency: a foreign holding is converted to the reporting currency at
// the month's rate, and the rate is recorded in fx_rates_used.
// covers: INV-FINANCE-15
func TestEngine_FxConversion(t *testing.T) {
	usdAcct := uuid.New()
	in := reportEngineInput{
		reportingCurrency: "IDR",
		multiCurrency:     true,
		positions:         []reportPosition{{id: usdAcct, group: groupAsset, subtype: "bank_account", ownershipType: "joint"}},
		snapshots:         []reportSnapshot{{positionID: usdAcct, yearMonth: ym(2026, time.January), amount: dec("100"), currency: "USD"}},
		fxRates:           []reportFxRate{{currency: "USD", yearMonth: ym(2026, time.January), rate: dec("16000")}},
		currentMonth:      ym(2026, time.January),
	}
	r := findMonth(t, generateMonthlyReports(in), ym(2026, time.January))
	if !r.nwTotal.Equal(dec("1600000")) { // 100 USD × 16000
		t.Errorf("nwTotal: got %s, want 1600000", r.nwTotal)
	}
	if got := r.fxRatesUsed["USD"]; !got.Equal(dec("16000")) {
		t.Errorf("fx_rates_used[USD]: got %s, want 16000", got)
	}
	if len(r.missingFx) != 0 {
		t.Errorf("missingFx: got %v, want empty", r.missingFx)
	}
}

// A rate carries forward to later months that have no rate of their own.
// covers: INV-FINANCE-15
func TestEngine_FxCarryForward(t *testing.T) {
	usdAcct := uuid.New()
	in := reportEngineInput{
		reportingCurrency: "IDR",
		multiCurrency:     true,
		positions:         []reportPosition{{id: usdAcct, group: groupAsset, subtype: "bank_account", ownershipType: "joint"}},
		snapshots: []reportSnapshot{
			{positionID: usdAcct, yearMonth: ym(2026, time.January), amount: dec("100"), currency: "USD"},
			{positionID: usdAcct, yearMonth: ym(2026, time.February), amount: dec("100"), currency: "USD"},
		},
		fxRates:      []reportFxRate{{currency: "USD", yearMonth: ym(2026, time.January), rate: dec("16000")}}, // Jan only
		currentMonth: ym(2026, time.February),
	}
	feb := findMonth(t, generateMonthlyReports(in), ym(2026, time.February))
	if !feb.nwTotal.Equal(dec("1600000")) { // carries Jan's rate
		t.Errorf("Feb nwTotal: got %s, want 1600000 (Jan rate carried)", feb.nwTotal)
	}
}

// A foreign currency with no rate at or before the month excludes those
// positions from net worth and records them in missing_fx — never 1:1.
// covers: INV-FINANCE-16
func TestEngine_FxMissingRate(t *testing.T) {
	usdAcct, idrAcct := uuid.New(), uuid.New()
	in := reportEngineInput{
		reportingCurrency: "IDR",
		multiCurrency:     true,
		positions: []reportPosition{
			{id: usdAcct, group: groupAsset, subtype: "bank_account", ownershipType: "joint"},
			{id: idrAcct, group: groupAsset, subtype: "bank_account", ownershipType: "joint"},
		},
		snapshots: []reportSnapshot{
			{positionID: usdAcct, yearMonth: ym(2026, time.January), amount: dec("100"), currency: "USD"},
			{positionID: idrAcct, yearMonth: ym(2026, time.January), amount: dec("500"), currency: "IDR"},
		},
		// no USD rate
		currentMonth: ym(2026, time.January),
	}
	r := findMonth(t, generateMonthlyReports(in), ym(2026, time.January))
	if !r.nwTotal.Equal(dec("500")) { // only the IDR account counts
		t.Errorf("nwTotal: got %s, want 500 (USD excluded)", r.nwTotal)
	}
	if len(r.missingFx) != 1 || r.missingFx[0].Currency != "USD" || r.missingFx[0].PositionID == nil || *r.missingFx[0].PositionID != usdAcct {
		t.Errorf("missingFx: got %+v, want one USD entry for the usd account", r.missingFx)
	}
}

// Regression: with multi-currency off the converter is a no-op — amounts sum at
// face value, no missing_fx, no fx_rates_used (the single-currency path).
// covers: INV-FINANCE-17
func TestEngine_FxOffPathUnchanged(t *testing.T) {
	acct := uuid.New()
	in := reportEngineInput{
		reportingCurrency: "IDR",
		multiCurrency:     false, // off
		positions:         []reportPosition{{id: acct, group: groupAsset, subtype: "bank_account", ownershipType: "joint"}},
		snapshots:         []reportSnapshot{{positionID: acct, yearMonth: ym(2026, time.January), amount: dec("100"), currency: "USD"}},
		fxRates:           []reportFxRate{{currency: "USD", yearMonth: ym(2026, time.January), rate: dec("16000")}},
		currentMonth:      ym(2026, time.January),
	}
	r := findMonth(t, generateMonthlyReports(in), ym(2026, time.January))
	if !r.nwTotal.Equal(dec("100")) { // face value, no conversion
		t.Errorf("nwTotal: got %s, want 100 (off path, no conversion)", r.nwTotal)
	}
	if len(r.missingFx) != 0 || len(r.fxRatesUsed) != 0 {
		t.Errorf("off path should not produce missingFx (%v) or fxRatesUsed (%v)", r.missingFx, r.fxRatesUsed)
	}
}

// Investment return counts the transaction cash flow: a Buy reduces return by
// the cash put in, so a holding that rose 400 on 300 of new cash returns 100.
// covers: INV-FINANCE-08
func TestEngine_InvestmentReturnWithCashFlow(t *testing.T) {
	stock := uuid.New()
	buy := dec("300")
	in := reportEngineInput{
		positions: []reportPosition{{id: stock, group: groupInvestment, subtype: "stock", ownershipType: "joint"}},
		snapshots: []reportSnapshot{
			{positionID: stock, yearMonth: ym(2026, time.January), amount: dec("500")},
			{positionID: stock, yearMonth: ym(2026, time.February), amount: dec("500")},
			{positionID: stock, yearMonth: ym(2026, time.March), amount: dec("900")},
		},
		transactions: []reportTransaction{
			{investmentID: stock, yearMonth: ym(2026, time.March), txnType: "buy", amount: &buy},
		},
		currentMonth: ym(2026, time.March),
	}
	mar := findMonth(t, generateMonthlyReports(in), ym(2026, time.March))
	if mar.investmentReturn == nil || !mar.investmentReturn.total.Equal(dec("100")) {
		t.Fatalf("Mar return: %+v, want 100 (Δsnapshot 400 − cashIn 300)", mar.investmentReturn)
	}
}

// Maturity with both portions cashed out (#25): on truthful data — a 0-value
// close snapshot at the maturity month plus the cash_out transaction — the
// engine books interest only, and the position leaves no net-worth bubble.
// This is the worked example from #25: principal 100, interest 5.
// covers: INV-FINANCE-11
func TestEngine_MaturityCashOutBooksInterestOnly(t *testing.T) {
	td := uuid.New()
	feb := ym(2026, time.February)
	principal, interest := dec("100"), dec("5")
	in := reportEngineInput{
		positions: []reportPosition{
			{id: td, group: groupInvestment, subtype: "time_deposit", ownershipType: "joint", terminatedAt: &feb},
		},
		snapshots: []reportSnapshot{
			{positionID: td, yearMonth: ym(2026, time.January), amount: dec("100")},
			{positionID: td, yearMonth: feb, amount: dec("0")}, // truthful close
		},
		transactions: []reportTransaction{
			{investmentID: td, yearMonth: feb, txnType: "maturity",
				principalAmount: &principal, interestAmount: &interest,
				principalDisposition: strp("cash_out"), interestDisposition: strp("cash_out")},
		},
		currentMonth: feb,
	}
	reports := generateMonthlyReports(in)
	jan := findMonth(t, reports, ym(2026, time.January))
	m := findMonth(t, reports, feb)

	// Return = (0 − 100) + 105 cash_out = 5 (interest only, no principal).
	if m.investmentReturn == nil || !m.investmentReturn.total.Equal(dec("5")) || !m.investmentReturn.timeDeposit.Equal(dec("5")) {
		t.Fatalf("maturity return: %+v, want total/time_deposit=5 (interest only)", m.investmentReturn)
	}
	// No NW bubble: the matured position contributes 0 in the maturity month.
	if !m.nwInvestments.Equal(dec("0")) {
		t.Errorf("maturity month nwInvestments: got %s, want 0 (no bubble)", m.nwInvestments)
	}
	if !jan.nwTotal.Equal(dec("100")) {
		t.Errorf("Jan nwTotal: got %s, want 100", jan.nwTotal)
	}
}

// Rolled TimeDeposit (#25/#27 rollover): the old position closes to 0 and the
// new position carries the rolled principal + interest. The maturity books the
// terminal value as a cash_out of the old TD; the engine routes the matching
// cash_in into the rolled-from successor. The two legs cancel across the
// rollover, so the combined return is interest only and the maturity month shows
// no double-counted principal in net worth. Crucially this holds even when the
// old TD's last snapshot under-accrues the final interest (see the 90→0 close
// below), unlike the old fragile model that relied on the closing snapshot
// equalling the full terminal value.
// covers: INV-FINANCE-12
func TestEngine_MaturityRolledNoDoubleCount(t *testing.T) {
	oldTD, newTD := uuid.New(), uuid.New()
	feb := ym(2026, time.February)
	principal, interest := dec("100"), dec("5")
	in := reportEngineInput{
		positions: []reportPosition{
			{id: oldTD, group: groupInvestment, subtype: "time_deposit", ownershipType: "joint", terminatedAt: &feb},
			{id: newTD, group: groupInvestment, subtype: "time_deposit", ownershipType: "joint", rolledFrom: &oldTD},
		},
		snapshots: []reportSnapshot{
			{positionID: oldTD, yearMonth: ym(2026, time.January), amount: dec("90")}, // under-accrued: final interest not yet in the snapshot
			{positionID: oldTD, yearMonth: feb, amount: dec("0")},                     // old closes
			{positionID: newTD, yearMonth: feb, amount: dec("105")},                   // rolled principal + interest
		},
		transactions: []reportTransaction{
			{investmentID: oldTD, yearMonth: feb, txnType: "maturity",
				principalAmount: &principal, interestAmount: &interest,
				principalDisposition: strp("rolled_to_new"), interestDisposition: strp("rolled_to_new")},
		},
		currentMonth: feb,
	}
	reports := generateMonthlyReports(in)
	jan := findMonth(t, reports, ym(2026, time.January))
	m := findMonth(t, reports, feb)

	// old: (0−90) + 105 cash_out = 15 (the final accrual the snapshot missed);
	// new: (105−0) − 105 rollover cash_in = 0; total = 15 (interest only).
	if m.investmentReturn == nil || !m.investmentReturn.total.Equal(dec("15")) {
		t.Fatalf("rolled return: %+v, want total=15", m.investmentReturn)
	}
	// No double-count: NW investments = 0 (old) + 105 (new) = 105, not 205.
	if !m.nwInvestments.Equal(dec("105")) {
		t.Errorf("rolled month nwInvestments: got %s, want 105 (old→0, new 105)", m.nwInvestments)
	}
	if !jan.nwTotal.Equal(dec("90")) {
		t.Errorf("Jan nwTotal: got %s, want 90", jan.nwTotal)
	}
}

// Sold position (#25): a manual Sell + terminate now writes the same truthful
// 0-value close snapshot, so the engine books the realized gain only (not the
// returned principal) and leaves no net-worth bubble.
// covers: INV-FINANCE-11
func TestEngine_SoldTerminationBooksGainOnly(t *testing.T) {
	stock := uuid.New()
	feb := ym(2026, time.February)
	proceeds := dec("560")
	in := reportEngineInput{
		positions: []reportPosition{
			{id: stock, group: groupInvestment, subtype: "stock", ownershipType: "joint", terminatedAt: &feb},
		},
		snapshots: []reportSnapshot{
			{positionID: stock, yearMonth: ym(2026, time.January), amount: dec("500")},
			{positionID: stock, yearMonth: feb, amount: dec("0")}, // truthful close
		},
		transactions: []reportTransaction{
			{investmentID: stock, yearMonth: feb, txnType: "sell", amount: &proceeds},
		},
		currentMonth: feb,
	}
	m := findMonth(t, generateMonthlyReports(in), feb)

	// Return = (0 − 500) + 560 = 60 (realized gain only).
	if m.investmentReturn == nil || !m.investmentReturn.total.Equal(dec("60")) || !m.investmentReturn.stock.Equal(dec("60")) {
		t.Fatalf("sold return: %+v, want total/stock=60", m.investmentReturn)
	}
	if !m.nwInvestments.Equal(dec("0")) {
		t.Errorf("sold month nwInvestments: got %s, want 0 (no bubble)", m.nwInvestments)
	}
}

// TimeDeposit placement (#27): a TD records no Buy, so the engine synthesizes a
// placement cash_in from principal + placement_date. In the placement month the
// 0→principal snapshot jump is cancelled → 0 return (capital deployed, not
// earned); later months book only accrued interest. A bank account establishes
// the Dec baseline so the Jan placement month is a computed (non-baseline) one.
// covers: INV-FINANCE-13
func TestEngine_TimeDepositPlacementBooksZero(t *testing.T) {
	bank, td := uuid.New(), uuid.New()
	jan := ym(2026, time.January)
	principal := dec("1000")
	in := reportEngineInput{
		positions: []reportPosition{
			{id: bank, group: groupAsset, subtype: "bank_account", ownershipType: "joint"},
			{id: td, group: groupInvestment, subtype: "time_deposit", ownershipType: "joint",
				placementAmount: &principal, placementMonth: &jan, currency: "IDR"},
		},
		snapshots: []reportSnapshot{
			{positionID: bank, yearMonth: ym(2025, time.December), amount: dec("10000")},
			{positionID: td, yearMonth: jan, amount: dec("1000")},                     // placed
			{positionID: td, yearMonth: ym(2026, time.February), amount: dec("1005")}, // +5 accrued
		},
		currentMonth: ym(2026, time.February),
	}
	reports := generateMonthlyReports(in)
	janR := findMonth(t, reports, jan)
	feb := findMonth(t, reports, ym(2026, time.February))

	// Jan: (1000 − 0) − cash_in 1000 = 0 (deployed capital, not return).
	if janR.investmentReturn == nil || !janR.investmentReturn.timeDeposit.Equal(dec("0")) {
		t.Fatalf("Jan TD return: %+v, want 0 (placement nets to 0)", janR.investmentReturn)
	}
	// Feb: (1005 − 1000) − 0 = 5 (accrued interest only).
	if !feb.investmentReturn.timeDeposit.Equal(dec("5")) {
		t.Fatalf("Feb TD return: %+v, want 5 (accrued interest)", feb.investmentReturn)
	}
}

// Bond placement (#27): a govt_primary bond now carries a Buy at placement (like
// secondary-market bonds always did), whose cash_in cancels the 0→nominal
// snapshot jump → 0 return in the placement month. No engine change is needed —
// the existing Buy cash-flow path handles it; this pins the behaviour.
// covers: INV-FINANCE-13
func TestEngine_BondPlacementBuyBooksZero(t *testing.T) {
	bank, bond := uuid.New(), uuid.New()
	jan := ym(2026, time.January)
	buy := dec("10000000")
	in := reportEngineInput{
		positions: []reportPosition{
			{id: bank, group: groupAsset, subtype: "bank_account", ownershipType: "joint"},
			{id: bond, group: groupInvestment, subtype: "bond", ownershipType: "joint"},
		},
		snapshots: []reportSnapshot{
			{positionID: bank, yearMonth: ym(2025, time.December), amount: dec("10000000")},
			{positionID: bond, yearMonth: jan, amount: dec("10000000")},
		},
		transactions: []reportTransaction{
			{investmentID: bond, yearMonth: jan, txnType: "buy", amount: &buy},
		},
		currentMonth: jan,
	}
	janR := findMonth(t, generateMonthlyReports(in), jan)
	if janR.investmentReturn == nil || !janR.investmentReturn.bond.Equal(dec("0")) {
		t.Fatalf("Jan bond return: %+v, want 0 (placement Buy cancels snapshot)", janR.investmentReturn)
	}
}

// Multi-tranche bond (#27): two Buys in consecutive months (20M then +80M) with
// snapshots tracking the nominal. Each tranche month's cash_in cancels its
// snapshot step, so every placement/top-up month nets to 0 — the case the issue
// calls "unfixable by inference" without a real per-tranche Buy.
// covers: INV-FINANCE-13
func TestEngine_BondTwoTranchePlacementsBookZero(t *testing.T) {
	bank, bond := uuid.New(), uuid.New()
	jan, feb := ym(2026, time.January), ym(2026, time.February)
	buy1, buy2 := dec("20000000"), dec("80000000")
	in := reportEngineInput{
		positions: []reportPosition{
			{id: bank, group: groupAsset, subtype: "bank_account", ownershipType: "joint"},
			{id: bond, group: groupInvestment, subtype: "bond", ownershipType: "joint"},
		},
		snapshots: []reportSnapshot{
			{positionID: bank, yearMonth: ym(2025, time.December), amount: dec("100000000")},
			{positionID: bond, yearMonth: jan, amount: dec("20000000")},  // tranche 1
			{positionID: bond, yearMonth: feb, amount: dec("100000000")}, // + tranche 2
		},
		transactions: []reportTransaction{
			{investmentID: bond, yearMonth: jan, txnType: "buy", amount: &buy1},
			{investmentID: bond, yearMonth: feb, txnType: "buy", amount: &buy2},
		},
		currentMonth: feb,
	}
	reports := generateMonthlyReports(in)
	janR := findMonth(t, reports, jan)
	febR := findMonth(t, reports, feb)
	if janR.investmentReturn == nil || !janR.investmentReturn.bond.Equal(dec("0")) {
		t.Fatalf("Jan bond return: %+v, want 0 (tranche 1 nets to 0)", janR.investmentReturn)
	}
	if !febR.investmentReturn.bond.Equal(dec("0")) {
		t.Fatalf("Feb bond return: %+v, want 0 (tranche 2 nets to 0)", febR.investmentReturn)
	}
}

// outstandingFaceFromLedger derives held nominal across tranches and sells (#27):
// (Σ buy_qty − Σ sell_qty) × 1,000,000.
func TestOutstandingFaceFromLedger(t *testing.T) {
	q := func(s string) *decimal.Decimal { d := dec(s); return &d }
	ledger := []db.InvestmentTransaction{
		{TransactionType: "buy", Quantity: q("20")},  // 20M
		{TransactionType: "buy", Quantity: q("80")},  // +80M → 100M
		{TransactionType: "sell", Quantity: q("30")}, // −30M → 70M
		{TransactionType: "coupon"},                  // no quantity — ignored
	}
	if got := outstandingFaceFromLedger(ledger); !got.Equal(dec("70000000")) {
		t.Fatalf("outstanding face: got %s, want 70000000", got)
	}
}
