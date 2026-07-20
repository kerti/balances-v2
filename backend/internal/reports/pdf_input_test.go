package reports

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/kerti/balances-v2/backend/internal/db"
	"github.com/kerti/balances-v2/backend/internal/repo"
)

func ym(y int, m time.Month) time.Time { return time.Date(y, m, 1, 0, 0, 0, 0, time.UTC) }

func dec(s string) decimal.Decimal { return decimal.RequireFromString(s) }

func decp(s string) *decimal.Decimal { d := dec(s); return &d }

func TestDeltaFrom(t *testing.T) {
	row := &db.MonthlyReport{YearMonth: ym(2026, time.June), NwTotal: dec("1100")}

	t.Run("nil prev → nil", func(t *testing.T) {
		if deltaFrom(row, nil) != nil {
			t.Error("want nil when prev absent")
		}
	})

	t.Run("zero prev NW → nil (percentage undefined)", func(t *testing.T) {
		prev := &db.MonthlyReport{YearMonth: ym(2026, time.May), NwTotal: decimal.Zero}
		if deltaFrom(row, prev) != nil {
			t.Error("want nil when prev net worth is zero")
		}
	})

	t.Run("computes signed change and percent", func(t *testing.T) {
		prev := &db.MonthlyReport{YearMonth: ym(2026, time.May), NwTotal: dec("1000")}
		d := deltaFrom(row, prev)
		if d == nil {
			t.Fatal("want a delta")
		}
		if d.Amount != "100" {
			t.Errorf("amount: got %q, want 100", d.Amount)
		}
		if d.Percent != 10 {
			t.Errorf("percent: got %v, want 10", d.Percent)
		}
		if !d.Prev.Equal(ym(2026, time.May)) {
			t.Errorf("prev month: got %v", d.Prev)
		}
	})

	t.Run("negative change", func(t *testing.T) {
		down := &db.MonthlyReport{YearMonth: ym(2026, time.June), NwTotal: dec("800")}
		prev := &db.MonthlyReport{YearMonth: ym(2026, time.May), NwTotal: dec("1000")}
		d := deltaFrom(down, prev)
		if d.Amount != "-200" || d.Percent != -20 {
			t.Errorf("got amount=%q percent=%v, want -200 / -20", d.Amount, d.Percent)
		}
	})
}

func TestBuildDeltaPicksImmediatelyPrecedingMonth(t *testing.T) {
	row := &db.MonthlyReport{YearMonth: ym(2026, time.June), NwTotal: dec("1200")}
	series := []db.MonthlyReport{
		{YearMonth: ym(2026, time.March), NwTotal: dec("900")},
		{YearMonth: ym(2026, time.May), NwTotal: dec("1000")}, // most recent before June
		{YearMonth: ym(2026, time.June), NwTotal: dec("1200")},
		{YearMonth: ym(2026, time.July), NwTotal: dec("1300")}, // later — ignored
	}
	d := buildDelta(row, series)
	if d == nil || !d.Prev.Equal(ym(2026, time.May)) {
		t.Fatalf("buildDelta: want prev=May 2026, got %v", d)
	}
	if d.Amount != "200" {
		t.Errorf("amount: got %q, want 200", d.Amount)
	}
}

func TestBuildYoY(t *testing.T) {
	row := &db.MonthlyReport{YearMonth: ym(2026, time.June), NwTotal: dec("1200")}

	t.Run("matches same month a year earlier", func(t *testing.T) {
		series := []db.MonthlyReport{
			{YearMonth: ym(2025, time.June), NwTotal: dec("1000")},
			{YearMonth: ym(2026, time.May), NwTotal: dec("1150")},
		}
		d := buildYoY(row, series)
		if d == nil || !d.Prev.Equal(ym(2025, time.June)) {
			t.Fatalf("buildYoY: want prev=June 2025, got %v", d)
		}
		if d.Amount != "200" {
			t.Errorf("amount: got %q, want 200", d.Amount)
		}
	})

	t.Run("nil when year-ago month absent", func(t *testing.T) {
		series := []db.MonthlyReport{{YearMonth: ym(2026, time.May), NwTotal: dec("1150")}}
		if buildYoY(row, series) != nil {
			t.Error("want nil when no month a year earlier")
		}
	})
}

func TestBuildFxRatesSortedByCurrency(t *testing.T) {
	fx := buildFxRates([]byte(`{"USD":"15500","EUR":"16800","SGD":"11500"}`))
	if len(fx) != 3 {
		t.Fatalf("got %d rates, want 3", len(fx))
	}
	if fx[0].Currency != "EUR" || fx[1].Currency != "SGD" || fx[2].Currency != "USD" {
		t.Errorf("not sorted by currency: %+v", fx)
	}
	if fx[0].Rate != "16800" {
		t.Errorf("EUR rate: got %q", fx[0].Rate)
	}
	// Malformed JSON → empty, not panic.
	if got := buildFxRates([]byte(`not json`)); len(got) != 0 {
		t.Errorf("bad JSON: got %d rates, want 0", len(got))
	}
}

func TestBuildTrendAscendingCappedAt12(t *testing.T) {
	var series []db.MonthlyReport
	for i := 0; i < 15; i++ { // 15 months, out of order (descending)
		series = append(series, db.MonthlyReport{
			YearMonth: ym(2026, time.January).AddDate(0, -i, 0),
			NwTotal:   decimal.NewFromInt(int64(1000 + i)),
		})
	}
	trend := buildTrend(series, ym(2026, time.January))
	if len(trend) != 12 {
		t.Fatalf("trend length: got %d, want 12 (capped)", len(trend))
	}
	// Must be ascending by month.
	for i := 1; i < len(trend); i++ {
		if trend[i-1].Label == "" {
			t.Fatal("empty trend label")
		}
	}
	// Last point is the reported month (Jan 2026), net worth 1000.
	if trend[len(trend)-1].NetWorth != 1000 {
		t.Errorf("last trend NW: got %v, want 1000", trend[len(trend)-1].NetWorth)
	}
}

// A report for an older month must chart the 12 months ending at that month, not
// the latest 12 months on record — the trend must never run past the reported
// month into its future.
func TestBuildTrendEndsAtReportingMonth(t *testing.T) {
	var series []db.MonthlyReport
	// 18 consecutive months, Jan 2025 .. Jun 2026, NW = month index.
	for i := 0; i < 18; i++ {
		series = append(series, db.MonthlyReport{
			YearMonth: ym(2025, time.January).AddDate(0, i, 0),
			NwTotal:   decimal.NewFromInt(int64(i)),
		})
	}
	// Report for Mar 2026 (index 14) while Apr–Jun 2026 also exist.
	trend := buildTrend(series, ym(2026, time.March))
	if len(trend) != 12 {
		t.Fatalf("trend length: got %d, want 12", len(trend))
	}
	last := trend[len(trend)-1]
	if last.Label != "Mar 26" {
		t.Errorf("last label: got %q, want Mar 26 (reported month, not latest)", last.Label)
	}
	if last.NetWorth != 14 {
		t.Errorf("last NW: got %v, want 14 (Mar 2026, not Jun 2026)", last.NetWorth)
	}
	// Window is the 12 months ending at Mar 2026 → starts Apr 2025 (index 3).
	if trend[0].Label != "Apr 25" || trend[0].NetWorth != 3 {
		t.Errorf("first point: got %q/%v, want Apr 25/3", trend[0].Label, trend[0].NetWorth)
	}
}

func TestBuildPDFInputHidesZeroPositions(t *testing.T) {
	row := &db.MonthlyReport{YearMonth: ym(2026, time.June), NwTotal: dec("900")}
	positions := []repo.PositionDetail{
		{Name: "Everyday", Group: "asset", Subtype: "bank_account", OwnershipType: "joint",
			NativeCurrency: "IDR", NativeAmount: dec("900"), Amount: dec("900")},
		{Name: "Drained", Group: "asset", Subtype: "bank_account", OwnershipType: "joint",
			NativeCurrency: "IDR", NativeAmount: decimal.Zero, Amount: decimal.Zero},
		{Name: "Paid-off loan", Group: "liability", Subtype: "personal", OwnershipType: "joint",
			NativeCurrency: "IDR", NativeAmount: decimal.Zero, Amount: decimal.Zero},
	}
	in := buildPDFInput(row, positions, nil, nil, nil, decimal.Zero, "IDR", "en-GB", "dev")
	if len(in.Positions) != 1 {
		t.Fatalf("positions: got %d, want 1 (two zero rows dropped)", len(in.Positions))
	}
	if in.Positions[0].Name != "Everyday" {
		t.Errorf("kept position: got %q, want Everyday", in.Positions[0].Name)
	}
	// Net worth is untouched by the filter — zeros never contributed to it.
	if in.NetWorth != "900" {
		t.Errorf("net worth: got %q, want 900", in.NetWorth)
	}
	// Build version is plumbed through to the footer slot (#414).
	if in.Version != "dev" {
		t.Errorf("version: got %q, want dev", in.Version)
	}
}

func TestBuildCashFlow(t *testing.T) {
	alice := uuid.New()
	names := map[uuid.UUID]string{alice: "Alice"}

	t.Run("nil on baseline month (no derived expenses)", func(t *testing.T) {
		row := &db.MonthlyReport{DerivedLivingExpenses: nil}
		if buildCashFlow(row, names, "Joint") != nil {
			t.Error("want nil when DerivedLivingExpenses is nil")
		}
	})

	t.Run("members by earned income, net = income − expenses", func(t *testing.T) {
		row := &db.MonthlyReport{
			DerivedLivingExpenses: decp("90000000"),
			EarnedIncomeTotal:     decp("75000000"),
			UserBreakdowns: []byte(`{
				"` + alice.String() + `":{"earned_income":"40000000"},
				"joint":{"earned_income":"35000000"},
				"` + uuid.New().String() + `":{"earned_income":"0"}
			}`),
		}
		cf := buildCashFlow(row, names, "Joint")
		if cf == nil {
			t.Fatal("want a cash flow")
		}
		// Zero-income member dropped; two remain, sorted by amount desc.
		if len(cf.Members) != 2 {
			t.Fatalf("members: got %d, want 2", len(cf.Members))
		}
		if cf.Members[0].Label != "Alice" || cf.Members[0].Amount != "40000000" {
			t.Errorf("member[0]: got %+v, want Alice/40000000", cf.Members[0])
		}
		if cf.Members[1].Label != "Joint" {
			t.Errorf("member[1] label: got %q, want Joint", cf.Members[1].Label)
		}
		if cf.Income != "75000000" || cf.Expenses != "90000000" || cf.Net != "-15000000" {
			t.Errorf("income/expenses/net: got %q/%q/%q", cf.Income, cf.Expenses, cf.Net)
		}
	})

	// covers: INV-FINANCE-26
	t.Run("active/passive decompose income by source; coupons ride separately", func(t *testing.T) {
		row := &db.MonthlyReport{
			DerivedLivingExpenses: decp("30000000"),
			EarnedIncomeTotal:     decp("60000000"),
			// Active sources
			EarnedIncomeSalary:    decp("40000000"),
			EarnedIncomeBusiness:  decp("5000000"),
			EarnedIncomeGift:      decp("1000000"),
			EarnedIncomeTaxRefund: decp("500000"),
			EarnedIncomeInsurance: decp("500000"),
			EarnedIncomeOther:     decp("1000000"),
			// Passive sources
			EarnedIncomeRental:   decp("8000000"),
			EarnedIncomePension:  decp("3000000"),
			EarnedIncomeInterest: decp("1000000"),
			// Coupon cash rides inside investment return, not earned income
			PassiveCouponCash: decp("2000000"),
			UserBreakdowns:    []byte(`{}`),
		}
		cf := buildCashFlow(row, names, "Joint")
		if cf == nil {
			t.Fatal("want a cash flow")
		}
		if cf.Active != "48000000" { // 40M+5M+1M+0.5M+0.5M+1M
			t.Errorf("active: got %q, want 48000000", cf.Active)
		}
		if cf.Passive != "12000000" { // 8M+3M+1M
			t.Errorf("passive: got %q, want 12000000", cf.Passive)
		}
		// Active + Passive == Income (the by-source decomposition is exact).
		if got := dec(cf.Active).Add(dec(cf.Passive)).String(); got != cf.Income {
			t.Errorf("active+passive = %q, want == income %q", got, cf.Income)
		}
		// Coupons are additive: reported, but not folded into Income or Net.
		if cf.Coupons != "2000000" {
			t.Errorf("coupons: got %q, want 2000000", cf.Coupons)
		}
		if cf.Income != "60000000" || cf.Net != "30000000" {
			t.Errorf("income/net: got %q/%q, want 60000000/30000000", cf.Income, cf.Net)
		}
	})

	t.Run("coupons empty string when zero", func(t *testing.T) {
		row := &db.MonthlyReport{
			DerivedLivingExpenses: decp("100"),
			EarnedIncomeTotal:     decp("0"),
			UserBreakdowns:        []byte(`{}`),
		}
		cf := buildCashFlow(row, names, "Joint")
		if cf.Coupons != "" {
			t.Errorf("coupons: got %q, want empty", cf.Coupons)
		}
		if cf.Active != "0" || cf.Passive != "0" {
			t.Errorf("active/passive: got %q/%q, want 0/0", cf.Active, cf.Passive)
		}
	})

	t.Run("nil earned-income total treated as zero", func(t *testing.T) {
		row := &db.MonthlyReport{
			DerivedLivingExpenses: decp("500"),
			EarnedIncomeTotal:     nil,
			UserBreakdowns:        []byte(`{}`),
		}
		cf := buildCashFlow(row, names, "Joint")
		if cf.Income != "0" || cf.Net != "-500" {
			t.Errorf("got income=%q net=%q, want 0 / -500", cf.Income, cf.Net)
		}
	})
}
