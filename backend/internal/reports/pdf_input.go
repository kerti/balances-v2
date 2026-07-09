package reports

import (
	"encoding/json"
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
func buildPDFInput(row *db.MonthlyReport, positions []repo.PositionDetail, series []db.MonthlyReport, members []db.User, currency, locale string) pdf.Input {
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
		ReportingCurrency: currency,
		NetWorth:          row.NwTotal.String(),
		Delta:             buildDelta(row, series),
		YoY:               buildYoY(row, series),
		Positions:         pos,
		CashFlow:          buildCashFlow(row, nameByID, joint),
		FxRates:           buildFxRates(row.FxRatesUsed),
		Trend:             buildTrend(series, row.YearMonth),
	}
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
	return &pdf.CashFlow{
		Members:  members,
		Income:   income.String(),
		Expenses: expenses.String(),
		Net:      income.Sub(expenses).String(),
	}
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
