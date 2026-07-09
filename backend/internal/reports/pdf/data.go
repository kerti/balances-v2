package pdf

import "time"

// Position is one itemized holding at the reported month, already resolved to
// the reporting currency by the report engine (repo.PositionDetail). Native
// amount/currency are carried so foreign holdings can show both.
type Position struct {
	Group          string // asset | liability | investment | receivable
	Subtype        string // "" for receivable (no subtype)
	Name           string
	OwnerLabel     string // resolved member nickname/name, or the Joint label
	NativeCurrency string
	NativeAmount   string // decimal string
	Amount         string // reporting-currency decimal string
	Stale          bool   // value carried forward from an earlier month
}

// CashMember is one household member's (or the Joint bucket's) earned income
// for the month, in the reporting currency.
type CashMember struct {
	Label  string
	Amount string // decimal string
}

// CashFlow is the month's income statement reduced to cash terms: earned income
// in (by member) minus living expenses out. Nil on the first reported month
// (no prior month → no derived expenses).
type CashFlow struct {
	Members  []CashMember
	Income   string // total earned income, decimal string
	Expenses string // derived living expenses, decimal string
	Net      string // Income − Expenses, decimal string
}

// FxRate is one currency's exchange rate into the reporting currency, as used
// by the engine for this month.
type FxRate struct {
	Currency string
	Rate     string // decimal string
}

// TrendPoint is one month on the 12-month net-worth trend line.
type TrendPoint struct {
	Label    string  // short month label, e.g. "Jun 26"
	NetWorth float64 // reporting-currency net worth
}

// Delta is the month-over-month change in net worth against the immediately
// preceding reported month. Nil when there is no prior month.
type Delta struct {
	Amount  string    // signed reporting-currency change (current − previous)
	Percent float64   // signed percentage change
	Prev    time.Time // the month compared against
}

// Input is everything the renderer needs for one month's report. Assembled by
// the reports handler from the aggregate report, position detail, the report
// series, and household members.
type Input struct {
	YearMonth         time.Time
	Locale            string // BCP-47, from the authenticated user's preference
	ReportingCurrency string
	NetWorth          string // reporting-currency decimal string
	Delta             *Delta // month-over-month; nil when no prior month
	YoY               *Delta // year-over-year; nil when no month a year earlier
	Positions         []Position
	CashFlow          *CashFlow // nil on the baseline month
	FxRates           []FxRate
	Trend             []TrendPoint
}
