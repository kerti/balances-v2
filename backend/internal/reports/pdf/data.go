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
//
// Active/Passive decompose Income by source on a single-month total basis (incl.
// one-offs) — Active+Passive == Income. This deliberately differs from the
// statistics panel's passive ratio (trailing-12, routine-only): the drill-down
// answers "where did this month's income come from", the ratio judges reliance.
// Coupons is paid-out bond-coupon cash — passive cash that rides inside
// InvestmentReturn (not EarnedIncome), so it prints as a separate additive line
// and is not summed into Income or Net. See ADR-0048.
type CashFlow struct {
	Members  []CashMember
	Income   string // total earned income, decimal string
	Active   string // active income (salary/business/gift/tax-refund/insurance/other), decimal string
	Passive  string // passive income (rental/pension/interest), decimal string
	Coupons  string // paid-out bond-coupon cash; "" when zero, additive line, not in Income/Net
	Expenses string // derived living expenses, decimal string
	Net      string // Income − Expenses, decimal string
}

// WriteOffItem is one Position behind the month's Write-Off line, with its
// signed contribution to net worth: a forgiven Liability reads positive, a
// disposed Asset or written-off Receivable negative.
type WriteOffItem struct {
	Label  string
	Amount string // signed decimal string
}

// WriteOffs is the month's Write-Off line: value that left the book with no cash
// settling it (ADR-0052). Nil when the month had none, which is the usual case —
// the section renders only when something was actually written off.
//
// Items exists because Write-Offs is ONE signed term, not a gains/losses pair: a
// month holding both a forgiven debt and a written-off receivable can net toward
// zero on Total, and the itemisation is what keeps that from reading as "nothing
// happened".
type WriteOffs struct {
	Total string // signed decimal string
	Items []WriteOffItem
}

// TrackingChangeItem is one Position behind the month's Tracking Changes line,
// with its signed contribution to net worth: a Position onboarded as already
// owned reads positive, one that left the books untracked negative — and the
// signs invert for a Liability, whose arrival lowers net worth.
type TrackingChangeItem struct {
	Label  string
	Amount string // signed decimal string
}

// TrackingChanges is the month's Tracking Changes line: value that crossed the
// edge of the book without being earned, spent or invested (ADR-0053). Nil when
// nothing crossed, which is the usual case — the section renders only when a
// Household actually declared one.
//
// Items exists for the same reason WriteOffs has them: this is ONE signed term
// covering both directions, so a month where one Position arrived and another
// left can net toward zero on Total, and only the itemisation shows that two
// real things happened (ADR-0052 §4, inherited by ADR-0053 §1).
type TrackingChanges struct {
	Total string // signed decimal string
	Items []TrackingChangeItem
}

// UnsettledTermination is one Investment terminated with no recorded proceeds —
// a data-quality advisory, part of no figure (ADR-0052 §7). Reachable only
// through a path that bypasses the terminate dialog: restore-from-backup,
// import, or the raw API.
type UnsettledTermination struct {
	Label string
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

// Stats is the financial-health panel (ADR-0048, #412): four household-health
// ratios derived at render time from the report series, positions, inflation
// and the assumed-inflation setting — never materialized. The zero value is
// all-undefined, which renders the reserved em-dash panel unchanged.
type Stats struct {
	CashFlow         Ratio      // savings rate; may be negative
	PassiveIncome    Ratio      // total passive income ÷ living expenses; market-sensitive, may be negative
	InstantLiquidity Ratio      // bank cash ÷ total investments; a ceiling gauge
	Resilience       Resilience // investment-pool depletion runway
	Inputs           StatInputs // the trailing-12 operands behind the two flow ratios
}

// StatInputs is the trailing-12 (routine-income) operands the two flow ratios
// divide, surfaced so the owner can reproduce the percentages by hand:
// Cash-Flow = (AvgIncome − AvgExpenses) / AvgIncome; Passive-Income =
// AvgPassive / AvgExpenses. Averages, not sums — sum/sum equals avg/avg, so the
// ratio is identical while the figures read as "typical month". Undefined on the
// baseline (no flow month in the window), where the flow ratios are undefined too.
type StatInputs struct {
	Defined     bool
	AvgIncome   string // avg monthly earned income, regular only (routine), decimal string
	AvgExpenses string // avg monthly derived living expenses (estimated), decimal string
	AvgPassive  string // avg monthly total passive income (passive cash + investment return)
	Months      int    // flow months averaged over (≤12)
}

// Ratio is one percentage indicator. Defined=false renders an em-dash + the
// undefined note (inputs unavailable — see ADR-0048's edge states); Percent is
// only meaningful when Defined.
type Ratio struct {
	Defined bool
	Percent float64
}

// InvestmentPerf is the investment-performance block (ADR-0048 amendment):
// investment return read as a *rate* (return ÷ opening invested capital), three
// cuts — Total, by risk profile, by instrument type — each a this-month rate
// beside its trailing-12-month compound. Derived at render time from the report
// series; never materialized. Defined=false suppresses the block (the household
// held no investments in the reported month).
type InvestmentPerf struct {
	Defined bool
	Total   PerfRow
	ByRisk  []PerfRow // low, medium, high — undefined-rate rows kept (labelled "—")
	ByType  []PerfRow // stock, mutual_fund, bond, gold, time_deposit

	// Placement is new money deployed into investments (Buys + fresh TD
	// placements, excl. rollovers/fees), as a share of the opening invested pool —
	// "how much of the pool's growth was new money vs return". Month and Trailing
	// each carry the % (headline) and the underlying amount (Month = this month,
	// Trailing = 12-month average). HasPlacement is false on a baseline reported
	// month (no flow), which suppresses the row.
	HasPlacement bool
	Placement    PerfRow
}

// PerfRow is one investment-performance line for a bucket: its this-month rate
// and its trailing-12 compound rate. Key is the bucket token ("total", a risk
// level, or a subtype) that copy resolves to a localized label.
type PerfRow struct {
	Key      string
	Month    Rate // this month: return ÷ prior-month opening value
	Trailing Rate // trailing-12 geometric compound of in-window monthly rates
}

// Rate is one investment-return rate cell. Defined=false renders "—" (opening
// base zero or absent — INV-FINANCE-30). Amount is the underlying reporting-
// currency return for context ("" when not shown, e.g. the trailing cell).
type Rate struct {
	Defined bool
	Percent float64
	Amount  string
}

// Resilience is the Fund Resilience runway: how many months the investment pool
// would last if active income stopped, or Indefinite when it never depletes
// within the ~100-year horizon (financial independence reached).
type Resilience struct {
	Defined    bool
	Indefinite bool
	Months     int
}

// Input is everything the renderer needs for one month's report. Assembled by
// the reports handler from the aggregate report, position detail, the report
// series, and household members.
type Input struct {
	YearMonth         time.Time
	Locale            string // BCP-47, from the authenticated user's preference
	Version           string // app build tag for the footer (#414); "" hides the suffix
	ReportingCurrency string
	NetWorth          string // reporting-currency decimal string
	Delta             *Delta // month-over-month; nil when no prior month
	YoY               *Delta // year-over-year; nil when no month a year earlier
	Positions         []Position
	CashFlow          *CashFlow  // nil on the baseline month
	WriteOffs         *WriteOffs // nil when the month wrote nothing off (ADR-0052)
	// TrackingChanges is the value that entered or left the books by declaration
	// rather than by a flow; nil when nothing crossed the edge (ADR-0053).
	TrackingChanges *TrackingChanges
	// Unsettled is the proceeds-less-termination advisory; empty in the normal
	// case, rendered as a footnote beside the stale-position one.
	Unsettled      []UnsettledTermination
	FxRates        []FxRate
	Trend          []TrendPoint
	Stats          Stats          // financial-health panel (ADR-0048); zero value = reserved em-dash panel
	InvestmentPerf InvestmentPerf // investment-performance rates (ADR-0048 amendment); Defined=false suppresses
}
