package repo

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
)

// GroupBreakdownMonth is one month of a single Position group's converted
// value, as the overview hubs (Assets / Investments / Liabilities) chart it.
// Total is the very nw_* figure the dashboard shows for that group and month;
// BySubtype and ByRisk are partitions of it. MissingFx lists the group's
// Positions left out of Total for want of an exchange rate (ADR-0002).
type GroupBreakdownMonth struct {
	YearMonth time.Time                  `json:"year_month"`
	Total     decimal.Decimal            `json:"total"`
	BySubtype map[string]decimal.Decimal `json:"by_subtype"`
	// ByRisk is set for the investment group only (every Investment carries one
	// risk profile, so it partitions Total just as BySubtype does).
	ByRisk    map[string]decimal.Decimal `json:"by_risk,omitempty"`
	MissingFx []missingFxEntry           `json:"missing_fx"`
}

type GroupBreakdown struct {
	Group             string                `json:"group"`
	ReportingCurrency string                `json:"reporting_currency"`
	Months            []GroupBreakdownMonth `json:"months"`
}

// GroupBreakdown returns one group's month-by-month converted value from the
// same engine pass that materializes the dashboard's reports (INV-FINANCE-39),
// so a hub total can never drift from the dashboard's group total. The group is
// its wire name — "asset", "liability" or "investment"; anything else is
// ErrNotFound. Nothing here is persisted.
func (r *MonthlyReportRepo) GroupBreakdown(ctx context.Context, group string) (*GroupBreakdown, error) {
	g, ok := hubGroup(group)
	if !ok {
		return nil, ErrNotFound
	}
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
	return buildGroupBreakdown(reports, g, currency), nil
}

func hubGroup(name string) (positionGroup, bool) {
	switch name {
	case "asset":
		return groupAsset, true
	case "liability":
		return groupLiability, true
	case "investment":
		return groupInvestment, true
	}
	return 0, false
}

// buildGroupBreakdown projects generated months onto one group. The engine's
// range starts at the household's first snapshot of any group, so months before
// the group's first contributing Position (or first excluded one) are dropped:
// they are all zero and would only stretch the hub's charts leftwards.
func buildGroupBreakdown(reports []monthlyReportData, g positionGroup, currency string) *GroupBreakdown {
	out := &GroupBreakdown{Group: g.String(), ReportingCurrency: currency, Months: []GroupBreakdownMonth{}}
	started := false
	for _, m := range reports {
		gv := m.groupValue[g]
		missing := m.groupMissingFx[g]
		if !started && (gv == nil || gv.count == 0) && len(missing) == 0 {
			continue
		}
		started = true
		month := GroupBreakdownMonth{
			YearMonth: m.yearMonth,
			Total:     groupTotal(m, g),
			BySubtype: map[string]decimal.Decimal{},
			MissingFx: append([]missingFxEntry{}, missing...),
		}
		if gv != nil {
			for subtype, v := range gv.bySubtype {
				month.BySubtype[subtype] = v
			}
		}
		if g == groupInvestment {
			month.ByRisk = map[string]decimal.Decimal{
				"low":    m.investmentValue.low,
				"medium": m.investmentValue.medium,
				"high":   m.investmentValue.high,
			}
		}
		out.Months = append(out.Months, month)
	}
	return out
}

// groupTotal reads the month's stored-and-dashboarded figure for the group.
func groupTotal(m monthlyReportData, g positionGroup) decimal.Decimal {
	switch g {
	case groupAsset:
		return m.nwAssets
	case groupLiability:
		return m.nwLiabilities
	case groupReceivable:
		return m.nwReceivables
	default:
		return m.nwInvestments
	}
}
