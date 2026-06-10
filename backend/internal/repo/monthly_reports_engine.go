package repo

import (
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// This file is the pure compute core of the materialized monthly report
// (ADR-0006 / ADR-0012). It takes plain in-memory inputs and derives one
// report per month — no DB, no context, no I/O — so the rules (carry-forward,
// lifecycle suppression, per-user/Joint attribution, stale flagging, the
// comprehensive-income identity, and FX conversion) are unit-testable without
// a container. The MonthlyReportRepo fetches the inputs, calls this, and
// upserts the result.
//
// Slice-3 scope: net worth + group breakdowns + the income statement +
// multi-currency conversion. Each monetary amount is converted from its native
// currency to the household's reporting currency at the report month's rate
// (latest <= M, carry-forward). When multi-currency is off the converter is a
// no-op and the engine behaves exactly as the single-currency slices.

const jointKey = "joint"

type positionGroup int

const (
	groupAsset positionGroup = iota
	groupLiability
	groupReceivable
	groupInvestment
)

// String is the wire name for the group, used by the stale-position payload so
// the frontend can resolve a position's detail-page route (#50).
func (g positionGroup) String() string {
	switch g {
	case groupAsset:
		return "asset"
	case groupLiability:
		return "liability"
	case groupReceivable:
		return "receivable"
	case groupInvestment:
		return "investment"
	}
	return ""
}

// reportPosition is the lifecycle + ownership + subtype metadata the engine
// needs. terminatedAt nil => active (migration 00012's biconditional CHECK).
// subtype distinguishes property/vehicle from bank_account (asset value change)
// and carries the investment subtype (per-subtype return).
type reportPosition struct {
	id            uuid.UUID
	name          string // display_name, surfaced in the stale-position drill-down (#50)
	group         positionGroup
	subtype       string
	ownershipType string // "sole" | "joint"
	soleOwnerID   *uuid.UUID
	terminatedAt  *time.Time
	// Time-deposit placement (issue #27). A TD records no Buy, so the engine
	// synthesizes a placement cash_in to cancel the 0→principal snapshot jump
	// in the placement month — otherwise it reads as pure return. nil for every
	// other subtype. currency is the native currency the cash_in is booked in.
	placementAmount *decimal.Decimal
	placementMonth  *time.Time
	currency        string
	// Predecessor TD when this position was opened by rolling over a matured TD
	// (issue #27 rollover). nil for fresh placements. The rollover's funding is
	// the predecessor's maturity terminal value, routed here as a cash_in, so a
	// rolled TD takes no synthetic TdPrincipal placement.
	rolledFrom *uuid.UUID
}

// reportSnapshot is one monthly observation in its native currency.
type reportSnapshot struct {
	positionID uuid.UUID
	yearMonth  time.Time
	amount     decimal.Decimal
	currency   string
}

type reportIncome struct {
	yearMonth     time.Time
	amount        decimal.Decimal
	currency      string
	category      string
	ownershipType string
	soleOwnerID   *uuid.UUID
}

type reportTransaction struct {
	investmentID         uuid.UUID
	yearMonth            time.Time
	currency             string
	txnType              string
	amount               *decimal.Decimal
	quantity             *decimal.Decimal
	principalAmount      *decimal.Decimal
	interestAmount       *decimal.Decimal
	principalDisposition *string
	interestDisposition  *string
}

// reportFxRate is one manual rate: reporting-currency units per 1 unit of
// currency, for the month-end of yearMonth (ADR-0002).
type reportFxRate struct {
	currency  string
	yearMonth time.Time
	rate      decimal.Decimal
}

type reportEngineInput struct {
	positions         []reportPosition
	snapshots         []reportSnapshot
	income            []reportIncome
	transactions      []reportTransaction
	fxRates           []reportFxRate
	members           []uuid.UUID
	reportingCurrency string
	multiCurrency     bool
	currentMonth      time.Time
}

type userBreakdown struct {
	NW               decimal.Decimal `json:"nw"`
	EarnedIncome     decimal.Decimal `json:"earned_income"`
	InvestmentReturn decimal.Decimal `json:"investment_return"`
}

type earnedIncomeAmounts struct {
	total, salary, business, rental, gift, taxRefund, insurance, other decimal.Decimal
}

func (e *earnedIncomeAmounts) add(category string, v decimal.Decimal) {
	e.total = e.total.Add(v)
	switch category {
	case "salary":
		e.salary = e.salary.Add(v)
	case "business_income":
		e.business = e.business.Add(v)
	case "rental_income":
		e.rental = e.rental.Add(v)
	case "gift":
		e.gift = e.gift.Add(v)
	case "tax_refund":
		e.taxRefund = e.taxRefund.Add(v)
	case "insurance_payout":
		e.insurance = e.insurance.Add(v)
	case "other":
		e.other = e.other.Add(v)
	}
}

type investmentReturnAmounts struct {
	total, stock, mutualFund, bond, gold, timeDeposit decimal.Decimal
}

func (r *investmentReturnAmounts) add(subtype string, v decimal.Decimal) {
	r.total = r.total.Add(v)
	switch subtype {
	case "stock":
		r.stock = r.stock.Add(v)
	case "mutual_fund":
		r.mutualFund = r.mutualFund.Add(v)
	case "bond":
		r.bond = r.bond.Add(v)
	case "gold":
		r.gold = r.gold.Add(v)
	case "time_deposit":
		r.timeDeposit = r.timeDeposit.Add(v)
	}
}

// missingFxEntry names a position (or, for a flow, nil) and the currency that
// could not be converted because no rate existed at or before its month.
type missingFxEntry struct {
	PositionID *uuid.UUID `json:"position_id"`
	Currency   string     `json:"currency"`
}

// stalePosition is one position whose value in a month was carried forward from
// an earlier snapshot rather than recorded fresh (#50). It carries enough for
// the dashboard to label the position and deep-link to its detail page:
// group+subtype map to a route, name is the display label, and lastMonth is the
// month the carried-forward snapshot was actually recorded.
type stalePosition struct {
	ID        uuid.UUID `json:"position_id"`
	Name      string    `json:"name"`
	Group     string    `json:"group"`
	Subtype   string    `json:"subtype"`
	LastMonth time.Time `json:"last_month"`
}

// monthlyReportData is one generated month, pre-serialisation. Income-statement
// pointers are nil on the first-month baseline (ADR-0006).
type monthlyReportData struct {
	yearMonth        time.Time
	nwTotal          decimal.Decimal
	nwAssets         decimal.Decimal
	nwLiabilities    decimal.Decimal // positive magnitude; subtracted into nwTotal
	nwReceivables    decimal.Decimal
	nwInvestments    decimal.Decimal
	earnedIncome     earnedIncomeAmounts
	investmentReturn *investmentReturnAmounts // nil on baseline
	assetValueChange *decimal.Decimal         // nil on baseline
	livingExpenses   *decimal.Decimal         // nil on baseline
	userBreakdowns   map[string]userBreakdown
	stalePositions   []stalePosition
	missingFx        []missingFxEntry
	fxRatesUsed      map[string]decimal.Decimal
	missingSeen      map[string]bool // dedup helper, not serialised
}

type monthAmount struct {
	idx      int
	amount   decimal.Decimal
	currency string // snapshot's native currency; unused for fx-rate entries
}

type cashFlow struct{ in, out decimal.Decimal }

func monthIndex(t time.Time) int {
	y, m, _ := t.Date()
	return y*12 + int(m) - 1
}

func monthFromIndex(i int) time.Time {
	return time.Date(i/12, time.Month(i%12+1), 1, 0, 0, 0, 0, time.UTC)
}

func ownerKey(ownershipType string, soleOwnerID *uuid.UUID) string {
	if ownershipType == "sole" && soleOwnerID != nil {
		return soleOwnerID.String()
	}
	return jointKey
}

func decOrZero(d *decimal.Decimal) decimal.Decimal {
	if d == nil {
		return decimal.Zero
	}
	return *d
}

// fxConverter converts native amounts to the reporting currency. When multi is
// false (single-currency household) it is a no-op: amounts pass through and no
// conversion can fail.
type fxConverter struct {
	reporting string
	multi     bool
	rates     map[string][]monthAmount // currency -> sorted (monthIdx, rate)
}

func newFxConverter(in reportEngineInput) fxConverter {
	rates := make(map[string][]monthAmount)
	if in.multiCurrency {
		for _, r := range in.fxRates {
			rates[r.currency] = append(rates[r.currency], monthAmount{idx: monthIndex(r.yearMonth), amount: r.rate})
		}
		for _, rs := range rates {
			sort.Slice(rs, func(i, j int) bool { return rs[i].idx < rs[j].idx })
		}
	}
	return fxConverter{reporting: in.reportingCurrency, multi: in.multiCurrency, rates: rates}
}

// convert returns the amount in reporting currency, the rate applied (1 for the
// reporting currency or when multi-currency is off), and ok=false when a
// foreign currency has no rate at or before idx.
func (fx fxConverter) convert(amount decimal.Decimal, currency string, idx int) (converted, rate decimal.Decimal, ok bool) {
	if !fx.multi || currency == "" || currency == fx.reporting {
		return amount, decimal.NewFromInt(1), true
	}
	r, found := latestAtOrBefore(fx.rates[currency], idx)
	if !found {
		return decimal.Zero, decimal.Zero, false
	}
	return amount.Mul(r.amount), r.amount, true
}

// foreign reports whether a currency is a non-reporting currency under an
// active multi-currency setting (i.e. worth recording in fx_rates_used).
func (fx fxConverter) foreign(currency string) bool {
	return fx.multi && currency != "" && currency != fx.reporting
}

func generateMonthlyReports(in reportEngineInput) []monthlyReportData {
	if len(in.snapshots) == 0 {
		return nil
	}
	fx := newFxConverter(in)

	byPos := make(map[uuid.UUID][]monthAmount, len(in.positions))
	var minIdx, maxIdx int
	for i, s := range in.snapshots {
		si := monthIndex(s.yearMonth)
		byPos[s.positionID] = append(byPos[s.positionID], monthAmount{idx: si, amount: s.amount, currency: s.currency})
		if i == 0 || si < minIdx {
			minIdx = si
		}
		if i == 0 || si > maxIdx {
			maxIdx = si
		}
	}
	for _, ss := range byPos {
		sort.Slice(ss, func(i, j int) bool { return ss[i].idx < ss[j].idx })
	}

	incomeByMonth := make(map[int][]reportIncome)
	for _, inc := range in.income {
		incomeByMonth[monthIndex(inc.yearMonth)] = append(incomeByMonth[monthIndex(inc.yearMonth)], inc)
	}

	// Convert transaction cash flows at the transaction's own month (= the
	// report month they affect). Unconvertible currencies are dropped and
	// flagged per month so the report still generates.
	cashByPos := make(map[uuid.UUID]map[int]cashFlow)
	txnMissing := make(map[int]map[string]bool)
	txnRates := make(map[int]map[string]decimal.Decimal)
	for _, t := range in.transactions {
		raw := transactionCashFlows(t)
		if raw.in.IsZero() && raw.out.IsZero() {
			continue
		}
		mi := monthIndex(t.yearMonth)
		inC, rate, okIn := fx.convert(raw.in, t.currency, mi)
		outC, _, okOut := fx.convert(raw.out, t.currency, mi)
		if !okIn || !okOut {
			if txnMissing[mi] == nil {
				txnMissing[mi] = make(map[string]bool)
			}
			txnMissing[mi][t.currency] = true
			continue
		}
		if fx.foreign(t.currency) {
			if txnRates[mi] == nil {
				txnRates[mi] = make(map[string]decimal.Decimal)
			}
			txnRates[mi][t.currency] = rate
		}
		m := cashByPos[t.investmentID]
		if m == nil {
			m = make(map[int]cashFlow)
			cashByPos[t.investmentID] = m
		}
		c := m[mi]
		c.in = c.in.Add(inC)
		c.out = c.out.Add(outC)
		m[mi] = c
	}

	// Synthetic time-deposit placement cash_in (issue #27). Booked exactly like
	// a Buy: in the placement month it cancels the snapshot's 0→principal jump so
	// the deployed capital nets to 0 return, leaving only interest as yield. Same
	// FX handling as real transactions — an unconvertible currency is flagged and
	// the flow dropped so the report still generates.
	for _, p := range in.positions {
		if p.placementAmount == nil || p.placementMonth == nil {
			continue
		}
		mi := monthIndex(*p.placementMonth)
		inC, rate, ok := fx.convert(*p.placementAmount, p.currency, mi)
		if !ok {
			if txnMissing[mi] == nil {
				txnMissing[mi] = make(map[string]bool)
			}
			txnMissing[mi][p.currency] = true
			continue
		}
		if fx.foreign(p.currency) {
			if txnRates[mi] == nil {
				txnRates[mi] = make(map[string]decimal.Decimal)
			}
			txnRates[mi][p.currency] = rate
		}
		m := cashByPos[p.id]
		if m == nil {
			m = make(map[int]cashFlow)
			cashByPos[p.id] = m
		}
		c := m[mi]
		c.in = c.in.Add(inC)
		m[mi] = c
	}

	// Rollover funding cash_in (issue #27 rollover). A rolled TD takes no
	// TdPrincipal placement; instead the terminal value that rolled out of its
	// predecessor's maturity (the rolled_to_new portions) enters here as a
	// cash_in, in the maturity month (= the successor's placement month). This is
	// the inflow leg that matches the maturity's cash_out outflow leg, so the
	// rolled capital nets to 0 return on both TDs and only genuine new interest
	// shows as yield. Same FX handling as the placement flow above.
	successorOf := make(map[uuid.UUID]uuid.UUID) // predecessor id -> successor id
	for _, p := range in.positions {
		if p.rolledFrom != nil {
			successorOf[*p.rolledFrom] = p.id
		}
	}
	for _, t := range in.transactions {
		if t.txnType != "maturity" {
			continue
		}
		succ, ok := successorOf[t.investmentID]
		if !ok {
			continue
		}
		var rolled decimal.Decimal
		if t.principalDisposition != nil && *t.principalDisposition == "rolled_to_new" {
			rolled = rolled.Add(decOrZero(t.principalAmount))
		}
		if t.interestDisposition != nil && *t.interestDisposition == "rolled_to_new" {
			rolled = rolled.Add(decOrZero(t.interestAmount))
		}
		if rolled.IsZero() {
			continue
		}
		mi := monthIndex(t.yearMonth)
		inC, rate, ok := fx.convert(rolled, t.currency, mi)
		if !ok {
			if txnMissing[mi] == nil {
				txnMissing[mi] = make(map[string]bool)
			}
			txnMissing[mi][t.currency] = true
			continue
		}
		if fx.foreign(t.currency) {
			if txnRates[mi] == nil {
				txnRates[mi] = make(map[string]decimal.Decimal)
			}
			txnRates[mi][t.currency] = rate
		}
		m := cashByPos[succ]
		if m == nil {
			m = make(map[int]cashFlow)
			cashByPos[succ] = m
		}
		c := m[mi]
		c.in = c.in.Add(inC)
		m[mi] = c
	}

	lastIdx := maxIdx
	if ci := monthIndex(in.currentMonth); ci > lastIdx {
		lastIdx = ci
	}
	positions := sortedPositions(in.positions)

	var prevNwTotal decimal.Decimal
	out := make([]monthlyReportData, 0, lastIdx-minIdx+1)
	for idx := minIdx; idx <= lastIdx; idx++ {
		baseline := idx == minIdx
		m := monthlyReportData{
			yearMonth:      monthFromIndex(idx),
			userBreakdowns: make(map[string]userBreakdown, len(in.members)+1),
			stalePositions: []stalePosition{},
			missingFx:      []missingFxEntry{},
			fxRatesUsed:    make(map[string]decimal.Decimal),
			missingSeen:    make(map[string]bool),
		}
		for _, u := range in.members {
			m.userBreakdowns[u.String()] = userBreakdown{}
		}
		m.userBreakdowns[jointKey] = userBreakdown{}

		// Merge transaction-currency rates/missing for this month.
		for cur, rate := range txnRates[idx] {
			m.fxRatesUsed[cur] = rate
		}
		for cur := range txnMissing[idx] {
			m.addMissingFx(nil, cur)
		}

		// ----- earned income (always; converted) -------------------------
		for _, inc := range incomeByMonth[idx] {
			conv, rate, ok := fx.convert(inc.amount, inc.currency, idx)
			if !ok {
				m.addMissingFx(nil, inc.currency)
				continue
			}
			m.recordRate(fx, inc.currency, rate)
			m.earnedIncome.add(inc.category, conv)
			key := ownerKey(inc.ownershipType, inc.soleOwnerID)
			b := m.userBreakdowns[key]
			b.EarnedIncome = b.EarnedIncome.Add(conv)
			m.userBreakdowns[key] = b
		}

		// ----- net worth (always; carry-forward, converted) --------------
		for _, p := range positions {
			if terminatedBefore(p, idx) {
				continue
			}
			carried, ok := latestAtOrBefore(byPos[p.id], idx)
			if !ok {
				continue
			}
			v, rate, cok := fx.convert(carried.amount, carried.currency, idx)
			if !cok {
				m.addMissingFx(&p.id, carried.currency)
				continue
			}
			m.recordRate(fx, carried.currency, rate)
			if carried.idx < idx {
				m.stalePositions = append(m.stalePositions, stalePosition{
					ID:        p.id,
					Name:      p.name,
					Group:     p.group.String(),
					Subtype:   p.subtype,
					LastMonth: monthFromIndex(carried.idx),
				})
			}
			switch p.group {
			case groupAsset:
				m.nwAssets = m.nwAssets.Add(v)
			case groupLiability:
				m.nwLiabilities = m.nwLiabilities.Add(v)
			case groupReceivable:
				m.nwReceivables = m.nwReceivables.Add(v)
			case groupInvestment:
				m.nwInvestments = m.nwInvestments.Add(v)
			}
			signed := v
			if p.group == groupLiability {
				signed = v.Neg()
			}
			key := ownerKey(p.ownershipType, p.soleOwnerID)
			b := m.userBreakdowns[key]
			b.NW = b.NW.Add(signed)
			m.userBreakdowns[key] = b
		}
		m.nwTotal = m.nwAssets.Add(m.nwReceivables).Add(m.nwInvestments).Sub(m.nwLiabilities)

		// ----- income statement (suppressed on baseline) -----------------
		if !baseline {
			ret := &investmentReturnAmounts{}
			for _, p := range positions {
				if p.group != groupInvestment || terminatedBefore(p, idx) {
					continue
				}
				now, okNow := fx.carried(byPos[p.id], idx)
				prev, okPrev := fx.carried(byPos[p.id], idx-1)
				if !okNow || !okPrev {
					continue // currency unconvertible — flagged in the NW pass
				}
				cf := cashByPos[p.id][idx]
				r := now.Sub(prev).Add(cf.out).Sub(cf.in)
				ret.add(p.subtype, r)
				key := ownerKey(p.ownershipType, p.soleOwnerID)
				b := m.userBreakdowns[key]
				b.InvestmentReturn = b.InvestmentReturn.Add(r)
				m.userBreakdowns[key] = b
			}
			m.investmentReturn = ret

			avc := decimal.Zero
			for _, p := range positions {
				if p.group != groupAsset || terminatedBefore(p, idx) {
					continue
				}
				if p.subtype != "property" && p.subtype != "vehicle" {
					continue
				}
				now, okNow := fx.carried(byPos[p.id], idx)
				prev, okPrev := fx.carried(byPos[p.id], idx-1)
				if !okNow || !okPrev {
					continue
				}
				avc = avc.Add(now.Sub(prev))
			}
			m.assetValueChange = &avc

			deltaNW := m.nwTotal.Sub(prevNwTotal)
			exp := m.earnedIncome.total.Add(ret.total).Add(avc).Sub(deltaNW)
			m.livingExpenses = &exp
		}

		prevNwTotal = m.nwTotal
		out = append(out, m)
	}
	return out
}

// carried converts the most recent snapshot with month <= idx; (0,true) when
// none exists (contributes nothing), (0,false) when one exists but its currency
// has no rate at idx.
func (fx fxConverter) carried(ss []monthAmount, idx int) (decimal.Decimal, bool) {
	c, ok := latestAtOrBefore(ss, idx)
	if !ok {
		return decimal.Zero, true
	}
	v, _, cok := fx.convert(c.amount, c.currency, idx)
	return v, cok
}

// recordRate notes a foreign rate applied this month (fx_rates_used audit).
func (m *monthlyReportData) recordRate(fx fxConverter, currency string, rate decimal.Decimal) {
	if fx.foreign(currency) {
		m.fxRatesUsed[currency] = rate
	}
}

// addMissingFx records a (position, currency) that could not be converted,
// deduplicated within the month.
func (m *monthlyReportData) addMissingFx(positionID *uuid.UUID, currency string) {
	key := currency
	if positionID != nil {
		key = positionID.String() + "|" + currency
	}
	if m.missingSeen[key] {
		return
	}
	m.missingSeen[key] = true
	m.missingFx = append(m.missingFx, missingFxEntry{PositionID: positionID, Currency: currency})
}

func transactionCashFlows(t reportTransaction) cashFlow {
	switch t.txnType {
	case "buy":
		return cashFlow{in: decOrZero(t.amount)}
	case "sell", "coupon", "dividend", "distribution":
		return cashFlow{out: decOrZero(t.amount)}
	case "fee":
		if t.quantity == nil {
			return cashFlow{in: decOrZero(t.amount)}
		}
		return cashFlow{}
	case "maturity":
		// The matured TD's terminal value leaves the position regardless of where
		// it lands — paid out to the bank (cash_out) or rolled into a successor TD
		// (rolled_to_new). Either disposition is an outflow that offsets the
		// snapshot's drop to 0; booking both as cash_out keeps a rollover from
		// reading as a phantom loss of the matured principal.
		return cashFlow{out: decOrZero(t.principalAmount).Add(decOrZero(t.interestAmount))}
	}
	return cashFlow{}
}

func terminatedBefore(p reportPosition, idx int) bool {
	return p.terminatedAt != nil && idx > monthIndex(*p.terminatedAt)
}

func sortedPositions(ps []reportPosition) []reportPosition {
	out := make([]reportPosition, len(ps))
	copy(out, ps)
	sort.Slice(out, func(i, j int) bool { return out[i].id.String() < out[j].id.String() })
	return out
}

func latestAtOrBefore(ss []monthAmount, idx int) (monthAmount, bool) {
	var found monthAmount
	ok := false
	for k := range ss {
		if ss[k].idx <= idx {
			found = ss[k]
			ok = true
		} else {
			break
		}
	}
	return found, ok
}
