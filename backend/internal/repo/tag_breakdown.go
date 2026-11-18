package repo

import (
	"bytes"
	"context"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// TagBreakdownEntry is one Tag's converted value for the served month. A nil
// TagID is the Untagged bucket. Holdings is assets + receivables + investments;
// Liabilities is a positive magnitude, kept apart so the report can show it as
// its own slice (ADR-0028). Holdings − Liabilities is the tag's net.
type TagBreakdownEntry struct {
	TagID       *uuid.UUID      `json:"tag_id"`
	Holdings    decimal.Decimal `json:"holdings"`
	Liabilities decimal.Decimal `json:"liabilities"`
}

// TagMissingFxEntry names a Position left out of the Tags figures because its
// currency has no rate at or before the month (ADR-0002: exclude and warn).
// TagID is the Tag it would have landed in (nil = Untagged).
type TagMissingFxEntry struct {
	PositionID uuid.UUID  `json:"position_id"`
	Currency   string     `json:"currency"`
	TagID      *uuid.UUID `json:"tag_id"`
}

// TagBreakdown is the Tags report: one month, in the household reporting
// currency, cut from the same engine pass that writes the dashboard's reports
// (INV-FINANCE-40). YearMonth is nil for a household with no snapshot data.
type TagBreakdown struct {
	ReportingCurrency string                     `json:"reporting_currency"`
	YearMonth         *time.Time                 `json:"year_month"`
	Tags              []TagBreakdownEntry        `json:"tags"`
	MissingFx         []TagMissingFxEntry        `json:"missing_fx"`
	FxRatesUsed       map[string]decimal.Decimal `json:"fx_rates_used"`
}

// TagBreakdown serves the Tags report for the latest generated month — the month
// the dashboard selects by default — so that month's Σ(holdings − liabilities)
// across every Tag, Untagged included, is the dashboard's nw_total
// (INV-FINANCE-40). Positions without a rate are excluded and listed. Nothing
// here is persisted. When multi-currency is off the engine passes native amounts
// through unconverted, exactly as it does for the dashboard.
func (r *MonthlyReportRepo) TagBreakdown(ctx context.Context) (*TagBreakdown, error) {
	uid, hid, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	reports, err := r.refresh(ctx, uid, hid)
	if err != nil {
		return nil, err
	}
	currency, err := r.ReportingCurrency(ctx)
	if err != nil {
		return nil, err
	}
	return buildTagBreakdown(reports, currency), nil
}

// buildTagBreakdown projects the latest generated month onto the Tags report.
func buildTagBreakdown(reports []monthlyReportData, currency string) *TagBreakdown {
	out := &TagBreakdown{
		ReportingCurrency: currency,
		Tags:              []TagBreakdownEntry{},
		MissingFx:         []TagMissingFxEntry{},
		FxRatesUsed:       map[string]decimal.Decimal{},
	}
	if len(reports) == 0 {
		return out
	}
	m := reports[len(reports)-1]
	ym := m.yearMonth
	out.YearMonth = &ym
	for key, v := range m.tagValue {
		e := TagBreakdownEntry{Holdings: v.holdings, Liabilities: v.liabilities}
		if key != uuid.Nil {
			id := key
			e.TagID = &id
		}
		out.Tags = append(out.Tags, e)
	}
	sortTagEntries(out.Tags)
	out.MissingFx = append(out.MissingFx, m.tagMissingFx...)
	for cur, rate := range m.tagFxRatesUsed {
		out.FxRatesUsed[cur] = rate
	}
	return out
}

// sortTagEntries orders by tag id with Untagged last, so the payload is stable.
func sortTagEntries(es []TagBreakdownEntry) {
	sort.Slice(es, func(i, j int) bool {
		a, b := es[i].TagID, es[j].TagID
		if a == nil || b == nil {
			return a != nil && b == nil
		}
		return bytes.Compare(a[:], b[:]) < 0
	})
}
