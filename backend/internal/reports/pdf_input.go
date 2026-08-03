package reports

import (
	"encoding/json"
	"math"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/kerti/balances-v2/backend/internal/db"
	"github.com/kerti/balances-v2/backend/internal/repo"
	"github.com/kerti/balances-v2/backend/internal/reports/pdf"
)

// userBreakdownJSON mirrors the engine's per-user breakdown JSONB (nw +
// earned_income + investment_return). decimal.Decimal unmarshals both quoted
// and unquoted numbers, so it tolerates either marshaling.
type userBreakdownJSON struct {
	NW               decimal.Decimal `json:"nw"`
	EarnedIncome     decimal.Decimal `json:"earned_income"`
	InvestmentReturn decimal.Decimal `json:"investment_return"`
}

// buildPDFInput assembles the renderer's input from the aggregate report, the
// itemized position detail, the report series (for the trend), and household
// members (for owner + per-member labels). All numbers already come resolved to
// the reporting currency by the report engine (ADR-0045).
func buildPDFInput(row *db.MonthlyReport, positions []repo.PositionDetail, series []db.MonthlyReport, members []db.User, inflation []db.InflationRate, assumedAnnualInflation decimal.Decimal, currency, locale, appVersion string) pdf.Input {
	nameByID := map[uuid.UUID]string{}
	for _, m := range members {
		name := m.DisplayName
		if m.Nickname != nil && *m.Nickname != "" {
			name = *m.Nickname
		}
		nameByID[m.ID] = name
	}
	joint := pdf.JointLabel(locale)

	ownerLabel := func(ownership string, sole *uuid.UUID) string {
		if ownership == "joint" || sole == nil {
			return joint
		}
		if n, ok := nameByID[*sole]; ok {
			return n
		}
		return joint
	}

	pos := make([]pdf.Position, 0, len(positions))
	for _, p := range positions {
		// Hide exact-zero positions (drained accounts, sold-out holdings, paid-off
		// debts): they contribute nothing to any subtotal or net worth, so every
		// figure is unchanged, and a household statement reads cleaner without empty
		// line items. A position to retire for good is terminated (already excluded);
		// near-zero real balances are kept.
		if p.Amount.IsZero() {
			continue
		}
		pos = append(pos, pdf.Position{
			Group:          p.Group,
			Subtype:        p.Subtype,
			Name:           p.Name,
			OwnerLabel:     ownerLabel(p.OwnershipType, p.SoleOwnerID),
			NativeCurrency: p.NativeCurrency,
			NativeAmount:   p.NativeAmount.String(),
			Amount:         p.Amount.String(),
			Stale:          p.Stale,
		})
	}

	return pdf.Input{
		YearMonth:         row.YearMonth,
		Locale:            locale,
		Version:           appVersion,
		ReportingCurrency: currency,
		NetWorth:          row.NwTotal.String(),
		Delta:             buildDelta(row, series),
		YoY:               buildYoY(row, series),
		Positions:         pos,
		CashFlow:          buildCashFlow(row, nameByID, joint),
		WriteOffs:         buildWriteOffs(row),
		Unsettled:         buildUnsettled(row.UnsettledTerminations),
		FxRates:           buildFxRates(row.FxRatesUsed),
		Trend:             buildTrend(series, row.YearMonth),
		Stats:             buildStats(row, positions, series, inflation, assumedAnnualInflation),
		InvestmentPerf:    buildInvestmentPerformance(row, series),
	}
}

// riskKeys / subtypeKeys fix the row order of the two investment-performance
// partitions (ADR-0048 amendment). Both are complete: every Investment has one
// risk_profile and one subtype.
var (
	riskKeys    = []string{"low", "medium", "high"}
	subtypeKeys = []string{"stock", "mutual_fund", "bond", "gold", "time_deposit"}
)

// bucketReturn / bucketValue read one bucket's materialized monthly return and
// closing invested value off a report row, keyed by the bucket token. Return is
// nil on the baseline month (no flow); value is set on every month. The "total"
// value base is nw_investments (always present), so it is returned as a non-nil
// pointer.
func bucketReturn(r *db.MonthlyReport, key string) *decimal.Decimal {
	switch key {
	case "total":
		return r.InvestmentReturnTotal
	case "low":
		return r.InvestmentReturnLow
	case "medium":
		return r.InvestmentReturnMedium
	case "high":
		return r.InvestmentReturnHigh
	case "stock":
		return r.InvestmentReturnStock
	case "mutual_fund":
		return r.InvestmentReturnMutualFund
	case "bond":
		return r.InvestmentReturnBond
	case "gold":
		return r.InvestmentReturnGold
	case "time_deposit":
		return r.InvestmentReturnTimeDeposit
	}
	return nil
}

func bucketValue(r *db.MonthlyReport, key string) *decimal.Decimal {
	switch key {
	case "total":
		return &r.NwInvestments
	case "low":
		return r.InvestmentValueLow
	case "medium":
		return r.InvestmentValueMedium
	case "high":
		return r.InvestmentValueHigh
	case "stock":
		return r.InvestmentValueStock
	case "mutual_fund":
		return r.InvestmentValueMutualFund
	case "bond":
		return r.InvestmentValueBond
	case "gold":
		return r.InvestmentValueGold
	case "time_deposit":
		return r.InvestmentValueTimeDeposit
	}
	return nil
}

// buildInvestmentPerformance derives the investment-performance block (ADR-0048
// amendment): each bucket's this-month return rate (return ÷ prior-month opening
// invested value) beside its trailing-12 geometric compound. Computed at render
// time from the series, never materialized — the same seam as the four ratios.
// Suppressed (Defined=false) when the reported month holds no investments.
func buildInvestmentPerformance(row *db.MonthlyReport, series []db.MonthlyReport) pdf.InvestmentPerf {
	if !row.NwInvestments.IsPositive() {
		return pdf.InvestmentPerf{}
	}
	byMonth := make(map[string]*db.MonthlyReport, len(series))
	for i := range series {
		byMonth[series[i].YearMonth.Format("2006-01")] = &series[i]
	}
	prevOf := func(r *db.MonthlyReport) *db.MonthlyReport {
		return byMonth[r.YearMonth.AddDate(0, -1, 0).Format("2006-01")]
	}
	upto := row.YearMonth
	lo := upto.AddDate(0, -11, 0)

	// monthRate is a bucket's return over its opening (prior-month) invested value,
	// defined only when that base is positive and a return exists (INV-FINANCE-30).
	monthRate := func(r *db.MonthlyReport, key string) (float64, bool) {
		ret := bucketReturn(r, key)
		prev := prevOf(r)
		if ret == nil || prev == nil {
			return 0, false
		}
		base := bucketValue(prev, key)
		if base == nil || !base.IsPositive() {
			return 0, false
		}
		f, _ := ret.Div(*base).Float64()
		return f, true
	}

	row1 := func(key string) pdf.PerfRow {
		out := pdf.PerfRow{Key: key}
		// This month: rate + the underlying return amount for context.
		if ret := bucketReturn(row, key); ret != nil {
			out.Month.Amount = ret.String()
		}
		if r, ok := monthRate(row, key); ok {
			out.Month.Defined = true
			out.Month.Percent = r * 100
		}
		// Trailing-12: geometric compound Π(1+rₘ)−1 over in-window months with a
		// defined base; a month with a zero/absent base contributes no factor
		// (INV-FINANCE-31). Undefined when no month in the window qualifies.
		product := 1.0
		n := 0
		for i := range series {
			s := &series[i]
			if s.YearMonth.Before(lo) || s.YearMonth.After(upto) {
				continue
			}
			if r, ok := monthRate(s, key); ok {
				product *= 1 + r
				n++
			}
		}
		if n > 0 {
			out.Trailing.Defined = true
			out.Trailing.Percent = (product - 1) * 100
		}
		return out
	}

	perf := pdf.InvestmentPerf{Defined: true, Total: row1("total")}
	for _, k := range riskKeys {
		perf.ByRisk = append(perf.ByRisk, row1(k))
	}
	for _, k := range subtypeKeys {
		perf.ByType = append(perf.ByType, row1(k))
	}

	// Placement: new money in as a share of the opening pool. Trailing-12 is an
	// arithmetic average, NOT the geometric compound used for return — placement
	// is a flow you average to a typical month, not a compounding growth rate
	// (ADR-0048 amendment / INV-FINANCE-32). % over the window is Σplacement /
	// Σopening-pool (avg/avg); the amount is Σplacement / n.
	if row.InvestmentPlacement != nil {
		pr := pdf.PerfRow{Key: "placement"}
		pr.Month.Amount = row.InvestmentPlacement.String()
		if prev := prevOf(row); prev != nil && prev.NwInvestments.IsPositive() {
			f, _ := row.InvestmentPlacement.Div(prev.NwInvestments).Float64()
			pr.Month.Defined = true
			pr.Month.Percent = f * 100
		}
		sumPlace, sumPool, n := decimal.Zero, decimal.Zero, 0
		for i := range series {
			s := &series[i]
			if s.YearMonth.Before(lo) || s.YearMonth.After(upto) || s.InvestmentPlacement == nil {
				continue
			}
			prev := prevOf(s)
			if prev == nil || !prev.NwInvestments.IsPositive() {
				continue
			}
			sumPlace = sumPlace.Add(*s.InvestmentPlacement)
			sumPool = sumPool.Add(prev.NwInvestments)
			n++
		}
		if n > 0 {
			pr.Trailing.Amount = sumPlace.Div(decimal.NewFromInt(int64(n))).String()
			if sumPool.IsPositive() {
				f, _ := sumPlace.Div(sumPool).Float64()
				pr.Trailing.Defined = true
				pr.Trailing.Percent = f * 100
			}
		}
		perf.HasPlacement = true
		perf.Placement = pr
	}
	return perf
}

// resilienceHorizonMonths caps the Fund Resilience depletion simulation at ~100
// years; a pool that survives it is reported as "indefinite" (ADR-0048).
const resilienceHorizonMonths = 1200

// buildStats derives the financial-health panel (ADR-0048): four ratios computed
// at render time from the report series + positions + inflation, never
// materialized. Flow inputs are a trailing-12-month average (the reported month
// and the eleven preceding calendar months, fewer when history is shorter);
// stocks (net worth, investments, cash) are read at the reported month as-is. A
// ratio whose inputs are unavailable stays undefined (rendered as an em-dash).
func buildStats(row *db.MonthlyReport, positions []repo.PositionDetail, series []db.MonthlyReport, inflation []db.InflationRate, assumedAnnualInflation decimal.Decimal) pdf.Stats {
	upto := row.YearMonth
	lo := upto.AddDate(0, -11, 0)
	inWindow := func(t time.Time) bool { return !t.Before(lo) && !t.After(upto) }

	// Trailing-12 flow sums over the months in the window that carry flow data
	// (the baseline month has no derived expenses/income and is skipped).
	var incomeSum, expSum, passiveCashSum, totalPassiveSum decimal.Decimal
	var gSum float64 // Σ monthly investment-return rate, for the pool's growth g
	var gN, flowN int
	for i := range series {
		s := &series[i]
		if !inWindow(s.YearMonth) || s.DerivedLivingExpenses == nil {
			continue
		}
		flowN++
		// Passive *cash* income excludes the pool's own return g — the projection
		// already models that as growth, so counting it here too would double-count
		// it (ADR-0048). It is realized external cash that left the pool: Rental +
		// Pension + bank/deposit Interest, plus paid-out bond coupons (pays_out
		// disposition, #476). The coupon slice sits inside InvestmentReturnTotal
		// (the domain keeps coupon yield in investment return), so it is added to
		// passive cash here and removed from own-return g below — the two-scope
		// split that guards the double-count (INV-FINANCE-25). Accruing coupons
		// never enter passiveCouponCash and stay in g, mark-to-market, untouched.
		// Routine-only income (ADR-0048 amendment / INV-FINANCE-19,-21,-24): the
		// ratios judge the household against income it relies on, so every income
		// term is the materialized routine subtotal, not the all-regularity total.
		// A one-off (severance, THR, insurance payout) is excluded here while still
		// counting toward net worth, the income statement, and living expenses.
		couponCash := decOr(s.PassiveCouponCash)
		passiveCash := decOr(s.EarnedIncomeRentalRoutine).
			Add(decOr(s.EarnedIncomePensionRoutine)).
			Add(decOr(s.EarnedIncomeInterestRoutine)).
			Add(couponCash)
		// Own return excludes the paid-out coupon slice: it is passive cash, not
		// pool growth. The Passive-Income numerator is unchanged by the split
		// (passiveCash gains couponCash, ownReturn loses it), but the resilience
		// draw-offset gains it and g no longer compounds it (INV-FINANCE-25).
		ownReturn := decOr(s.InvestmentReturnTotal).Sub(couponCash)
		incomeSum = incomeSum.Add(decOr(s.EarnedIncomeTotalRoutine))
		expSum = expSum.Add(*s.DerivedLivingExpenses)
		passiveCashSum = passiveCashSum.Add(passiveCash)
		totalPassiveSum = totalPassiveSum.Add(passiveCash.Add(ownReturn))
		if s.NwInvestments.IsPositive() {
			r, _ := ownReturn.Div(s.NwInvestments).Float64()
			gSum += r
			gN++
		}
	}

	hundred := decimal.NewFromInt(100)
	var st pdf.Stats

	// Cash-Flow (savings rate) = (Income − LivingExpenses) / Income.
	if flowN > 0 && incomeSum.IsPositive() {
		pct, _ := incomeSum.Sub(expSum).Div(incomeSum).Mul(hundred).Float64()
		st.CashFlow = pdf.Ratio{Defined: true, Percent: pct}
	}
	// Passive-Income = TotalPassiveIncome / LivingExpenses (≥100% ⇒ FI).
	if flowN > 0 && expSum.IsPositive() {
		pct, _ := totalPassiveSum.Div(expSum).Mul(hundred).Float64()
		st.PassiveIncome = pdf.Ratio{Defined: true, Percent: pct}
	}
	// Instant-Liquidity = bank cash / total investments (a ceiling gauge). Bank
	// cash is same-day-accessible Assets only; the denominator is the reported
	// month's investment value (a stock), so this is defined even on the baseline.
	if row.NwInvestments.IsPositive() {
		bank := decimal.Zero
		for i := range positions {
			p := &positions[i]
			if p.Group == "asset" && p.Subtype == "bank_account" {
				bank = bank.Add(p.Amount)
			}
		}
		pct, _ := bank.Div(row.NwInvestments).Mul(hundred).Float64()
		st.InstantLiquidity = pdf.Ratio{Defined: true, Percent: pct}
	}
	// Fund Resilience: depletion projection of the investment pool (needs
	// investments > 0 and at least one flow month).
	if row.NwInvestments.IsPositive() && flowN > 0 {
		n := decimal.NewFromInt(int64(flowN))
		p0, _ := row.NwInvestments.Float64()
		e0, _ := expSum.Div(n).Float64()
		pi0, _ := passiveCashSum.Div(n).Float64()
		g := 0.0
		if gN > 0 {
			g = gSum / float64(gN)
		}
		st.Resilience = simulateResilience(p0, e0, pi0, g, monthlyInflation(inflation, assumedAnnualInflation, lo, upto))
	}
	// Reproducibility inputs: the trailing-12 operands behind the two flow ratios,
	// as per-month averages (sum/n; ratio is sum/sum == avg/avg). Defined exactly
	// when the flow ratios are (≥1 flow month in the window). See ADR-0048.
	if flowN > 0 {
		n := decimal.NewFromInt(int64(flowN))
		st.Inputs = pdf.StatInputs{
			Defined:     true,
			AvgIncome:   incomeSum.Div(n).String(),
			AvgExpenses: expSum.Div(n).String(),
			AvgPassive:  totalPassiveSum.Div(n).String(),
			Months:      flowN,
		}
	}
	return st
}

// monthlyInflation converts the household's inflation assumption to a monthly
// rate for the Fund Resilience projection. Stored annualized figures within the
// trailing-12 window are averaged to an effective annual rate; with none, the
// assumed_annual_inflation setting is used. Either way the annual percentage is
// converted once as (1 + a)^(1/12) − 1 (ADR-0048).
func monthlyInflation(rates []db.InflationRate, assumedAnnual decimal.Decimal, lo, upto time.Time) float64 {
	sum := decimal.Zero
	n := 0
	for i := range rates {
		if r := &rates[i]; !r.YearMonth.Before(lo) && !r.YearMonth.After(upto) {
			sum = sum.Add(r.Rate)
			n++
		}
	}
	annual := assumedAnnual
	if n > 0 {
		annual = sum.Div(decimal.NewFromInt(int64(n)))
	}
	a, _ := annual.Float64()
	return math.Pow(1+a/100, 1.0/12) - 1
}

// simulateResilience projects the investment pool forward month by month until
// it depletes, or reports Indefinite past the horizon. Each month the pool grows
// by g and is drawn down by living expenses net of continuing passive cash
// income; both the expense and the passive-income legs inflate at i (ADR-0048).
func simulateResilience(pool, expenses, passiveCash, g, i float64) pdf.Resilience {
	for m := 1; m <= resilienceHorizonMonths; m++ {
		pool = pool*(1+g) - (expenses - passiveCash)
		if pool <= 0 {
			return pdf.Resilience{Defined: true, Months: m}
		}
		expenses *= 1 + i
		passiveCash *= 1 + i
	}
	return pdf.Resilience{Defined: true, Indefinite: true}
}

// decOr dereferences an optional report figure, treating a nil (absent) column
// as zero.
func decOr(d *decimal.Decimal) decimal.Decimal {
	if d == nil {
		return decimal.Zero
	}
	return *d
}

// buildDelta computes the month-over-month net-worth change: against the report
// immediately preceding row's month (handles gaps in the series).
func buildDelta(row *db.MonthlyReport, series []db.MonthlyReport) *pdf.Delta {
	var prev *db.MonthlyReport
	for i := range series {
		r := &series[i]
		if r.YearMonth.Before(row.YearMonth) && (prev == nil || r.YearMonth.After(prev.YearMonth)) {
			prev = r
		}
	}
	return deltaFrom(row, prev)
}

// buildYoY computes the year-over-year change: against the same month one year
// earlier. Nil when that month isn't in the reported range.
func buildYoY(row *db.MonthlyReport, series []db.MonthlyReport) *pdf.Delta {
	target := row.YearMonth.AddDate(-1, 0, 0)
	for i := range series {
		s := &series[i]
		if s.YearMonth.Year() == target.Year() && s.YearMonth.Month() == target.Month() {
			return deltaFrom(row, s)
		}
	}
	return nil
}

// deltaFrom builds a Delta from row against prev. Nil when prev is absent or its
// net worth is zero (percentage undefined).
func deltaFrom(row, prev *db.MonthlyReport) *pdf.Delta {
	if prev == nil || prev.NwTotal.IsZero() {
		return nil
	}
	change := row.NwTotal.Sub(prev.NwTotal)
	pct, _ := change.Div(prev.NwTotal).Mul(decimal.NewFromInt(100)).Float64()
	return &pdf.Delta{Amount: change.String(), Percent: pct, Prev: prev.YearMonth}
}

// buildCashFlow reduces the income statement to cash terms (earned income by
// member − living expenses). Nil on the baseline month, where derived expenses
// don't exist yet.
func buildCashFlow(row *db.MonthlyReport, nameByID map[uuid.UUID]string, joint string) *pdf.CashFlow {
	if row.DerivedLivingExpenses == nil {
		return nil
	}
	var breakdowns map[string]userBreakdownJSON
	_ = json.Unmarshal(row.UserBreakdowns, &breakdowns)

	type memAmt struct {
		label string
		amt   decimal.Decimal
	}
	var mem []memAmt
	for key, b := range breakdowns {
		if b.EarnedIncome.IsZero() {
			continue
		}
		label := joint
		if key != "joint" {
			if id, err := uuid.Parse(key); err == nil {
				if n, ok := nameByID[id]; ok {
					label = n
				}
			}
		}
		mem = append(mem, memAmt{label: label, amt: b.EarnedIncome})
	}
	sort.SliceStable(mem, func(i, j int) bool { return mem[i].amt.GreaterThan(mem[j].amt) })

	members := make([]pdf.CashMember, 0, len(mem))
	for _, m := range mem {
		members = append(members, pdf.CashMember{Label: m.label, Amount: m.amt.String()})
	}

	income := decimal.Zero
	if row.EarnedIncomeTotal != nil {
		income = *row.EarnedIncomeTotal
	}
	expenses := *row.DerivedLivingExpenses

	// By-source decomposition of this month's earned income (single-month total,
	// incl. one-offs). Active + Passive == Income by construction (the engine's
	// source columns sum to EarnedIncomeTotal). See ADR-0048 PR2 amendment.
	active := decOr(row.EarnedIncomeSalary).
		Add(decOr(row.EarnedIncomeBusiness)).
		Add(decOr(row.EarnedIncomeGift)).
		Add(decOr(row.EarnedIncomeTaxRefund)).
		Add(decOr(row.EarnedIncomeInsurance)).
		Add(decOr(row.EarnedIncomeOther))
	passive := decOr(row.EarnedIncomeRental).
		Add(decOr(row.EarnedIncomePension)).
		Add(decOr(row.EarnedIncomeInterest))

	// Paid-out bond-coupon cash is passive cash that lives inside investment
	// return, not earned income (INV-FINANCE-25). Surface it as a separate
	// additive line, never folded into Income/Net; blank when zero.
	coupons := ""
	if c := decOr(row.PassiveCouponCash); c.IsPositive() {
		coupons = c.String()
	}

	return &pdf.CashFlow{
		Members:  members,
		Income:   income.String(),
		Active:   active.String(),
		Passive:  passive.String(),
		Coupons:  coupons,
		Expenses: expenses.String(),
		Net:      income.Sub(expenses).String(),
	}
}

// writeOffPositionJSON / unsettledTerminationJSON mirror the engine's two
// termination payloads (ADR-0052). Same tolerance as userBreakdownJSON: the
// decimal unmarshals whether the amount was written quoted or bare.
type writeOffPositionJSON struct {
	Name   string          `json:"name"`
	Amount decimal.Decimal `json:"amount"`
}

type unsettledTerminationJSON struct {
	Name string `json:"name"`
}

// buildWriteOffs renders the month's Write-Off line only when something was
// actually written off. Nil on the baseline (no derived lines) and nil when the
// constituents are empty — a 0 total with no Positions behind it is noise, and
// the section is meant to explain net-worth movement that nothing else does.
// The total comes from the materialized column rather than re-summing the items,
// so what prints is the figure the identity balanced against.
func buildWriteOffs(row *db.MonthlyReport) *pdf.WriteOffs {
	if row.WriteOffs == nil {
		return nil
	}
	var items []writeOffPositionJSON
	_ = json.Unmarshal(row.WriteOffPositions, &items)
	if len(items) == 0 {
		return nil
	}
	// Largest absolute movement first: the line a household should read is the one
	// that moved their net worth most, regardless of direction.
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].Amount.Abs().GreaterThan(items[j].Amount.Abs())
	})
	out := make([]pdf.WriteOffItem, 0, len(items))
	for _, it := range items {
		out = append(out, pdf.WriteOffItem{Label: it.Name, Amount: it.Amount.String()})
	}
	return &pdf.WriteOffs{Total: row.WriteOffs.String(), Items: out}
}

func buildUnsettled(raw []byte) []pdf.UnsettledTermination {
	var rows []unsettledTerminationJSON
	_ = json.Unmarshal(raw, &rows)
	out := make([]pdf.UnsettledTermination, 0, len(rows))
	for _, r := range rows {
		out = append(out, pdf.UnsettledTermination{Label: r.Name})
	}
	return out
}

func buildFxRates(raw []byte) []pdf.FxRate {
	var fxMap map[string]string
	_ = json.Unmarshal(raw, &fxMap)
	fx := make([]pdf.FxRate, 0, len(fxMap))
	for cur, rate := range fxMap {
		fx = append(fx, pdf.FxRate{Currency: cur, Rate: rate})
	}
	sort.Slice(fx, func(i, j int) bool { return fx[i].Currency < fx[j].Currency })
	return fx
}

// buildTrend takes the 12 months of net worth ending at the reported month
// (inclusive), ascending, for the trend line. Months after the reported month
// are excluded so the trend runs up to the report, not to the latest data on
// record — a report for an older month must not chart into its future.
func buildTrend(series []db.MonthlyReport, upto time.Time) []pdf.TrendPoint {
	s := make([]db.MonthlyReport, 0, len(series))
	for _, r := range series {
		if !r.YearMonth.After(upto) {
			s = append(s, r)
		}
	}
	sort.Slice(s, func(i, j int) bool { return s[i].YearMonth.Before(s[j].YearMonth) })
	if len(s) > 12 {
		s = s[len(s)-12:]
	}
	trend := make([]pdf.TrendPoint, 0, len(s))
	for _, r := range s {
		f, _ := r.NwTotal.Float64()
		trend = append(trend, pdf.TrendPoint{Label: r.YearMonth.Format("Jan 06"), NetWorth: f})
	}
	return trend
}
