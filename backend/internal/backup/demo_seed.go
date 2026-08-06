package backup

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/kerti/balances-v2/backend/internal/repo"
)

// demoCurrency is the single native/reporting currency every seeded Position
// and Income event uses — matches the "IDR" reporting currency the demo
// Household is created with (resetDemoHousehold), so no FX rate seeding is
// needed for the dashboard to render cleanly.
const demoCurrency = "IDR"

// demoExtraHistoryMonths precedes the trailing 12-month window most seeding
// helpers used to cover exclusively. The dashboard's year-over-year figure
// (Trend in DashboardScreen.tsx) looks up a report whose year_month starts
// with "<selected year - 1>-<month>" — that report doesn't exist in a bare
// 12-month trail, so the YoY line silently never rendered. 27 months total
// (15 extra + 12) gives it something to compare against.
const demoExtraHistoryMonths = 15
const demoMonthCount = 12 + demoExtraHistoryMonths

// demoMonths returns demoMonthCount consecutive calendar months ending at the
// current month, oldest first, so every reset lands its snapshot trail in a
// fixed window relative to "now" regardless of when the cron actually runs.
func demoMonths() []time.Time {
	now := time.Now().UTC()
	months := make([]time.Time, demoMonthCount)
	for i := 0; i < demoMonthCount; i++ {
		offset := demoMonthCount - 1 - i
		total := now.Year()*12 + int(now.Month()) - 1 - offset
		y := total / 12
		m := total%12 + 1
		months[i] = time.Date(y, time.Month(m), 1, 0, 0, 0, 0, time.UTC)
	}
	return months
}

// dayIn anchors a specific day-of-month onto a year_month value, for fields
// that want a real calendar date (income dates, placement/start dates) rather
// than a month bucket.
func dayIn(ym time.Time, day int) time.Time {
	return time.Date(ym.Year(), ym.Month(), day, 0, 0, 0, 0, time.UTC)
}

// demoPhaseStep is the golden angle (radians) — spacing each seeded
// position's wobble phase by this amount decorrelates them from each other
// (see demoSeries) instead of leaving every position peaking/dipping in the
// same calendar month.
const demoPhaseStep = 2.399963229728653

// demoSeries produces an n-point deterministic value curve — a steady
// monthly trend plus a small sinusoidal wobble — so seeded charts show
// motion instead of a flat line, without pulling in real randomness.
// phaseIdx shifts the wobble's phase per position (multiplied by
// demoPhaseStep): every position calling this with the same phaseIdx=0 would
// all use sin(i*1.3), so a diversified portfolio's noise would sum instead of
// averaging out in the aggregate net-worth total — the dashboard's headline
// chart visibly dipped every couple of months as a result. Each call site
// passes its own distinct index instead.
func demoSeries(base, monthlyGrowth, wobble float64, phaseIdx, n int) []float64 {
	phase := float64(phaseIdx) * demoPhaseStep
	vals := make([]float64, n)
	v := base
	for i := 0; i < n; i++ {
		vals[i] = v * (1 + wobble*math.Sin(float64(i)*1.3+phase))
		v *= 1 + monthlyGrowth
	}
	return vals
}

func demoDecimal(f float64) decimal.Decimal {
	return decimal.NewFromFloat(f).Round(2)
}

// demoRevaluationSeries projects a base value forward by a signed annual rate,
// compounded monthly exactly as the app's own revaluation helper does
// (frontend/src/lib/revaluation.ts): value[i] = base * (1 + annualRatePct/100)^(i/12).
// A positive rate appreciates (property), a negative rate depreciates (vehicle),
// so a seeded property/vehicle trail actually tracks the
// annual_appreciation_rate / annual_depreciation_rate the position advertises,
// rather than an unrelated sinusoidal curve. annualRatePct is a percent (4 = 4%
// per year), matching how those rate fields are stored and displayed.
func demoRevaluationSeries(base, annualRatePct float64, n int) []float64 {
	vals := make([]float64, n)
	factor := 1 + annualRatePct/100
	for i := 0; i < n; i++ {
		vals[i] = base * math.Pow(factor, float64(i)/12)
	}
	return vals
}

// demoStepDownSeries models an irregular paydown/settlement: it starts at base
// and subtracts one whole-nominal payment at each month listed in payments,
// holding flat in between — so a receivable/loan reads as real sporadic
// repayments (IDR 50,000 here, a USD 1 there) rather than a smooth curve. The
// value in month i is base − nominal × (payment months ≤ i). Payment months
// beyond the series length are ignored; the schedule must not drive the balance
// negative (base ≥ nominal × len(payments)).
func demoStepDownSeries(base, nominal float64, payments []int, n int) []float64 {
	vals := make([]float64, n)
	for i := 0; i < n; i++ {
		var paid int
		for _, p := range payments {
			if p <= i {
				paid++
			}
		}
		vals[i] = base - nominal*float64(paid)
	}
	return vals
}

// demoTrade is one Buy (deltaQty > 0) or Sell (deltaQty < 0) in a market
// position's ledger, at month index deltaQty into demoMonths. The snapshot
// quantity a position holds in month i is the running sum of every trade with
// monthIdx <= i (cumulativeQty) — so the snapshot value always equals the
// ledger's net quantity times that month's price, and the engine's per-month
// return (INV-FINANCE-08, ΔSnapshot + cash_out − cash_in) collapses to pure
// price appreciation on the previously-held lot with zero phantom loss (#497).
type demoTrade struct {
	monthIdx int
	deltaQty float64
}

// cumulativeQty is the net quantity held after every trade dated at or before
// uptoIdx — the snapshot quantity seedMarketSnapshots stamps for that month.
func cumulativeQty(trades []demoTrade, uptoIdx int) float64 {
	var q float64
	for _, t := range trades {
		if t.monthIdx <= uptoIdx {
			q += t.deltaQty
		}
	}
	return q
}

// demoCheckingOpening is the Everyday Checking balance in the baseline (oldest)
// month. The baseline books no Living Expenses — the engine has no prior month
// to diff against — so this is a free starting position; every later month's
// balance is derived from it by the cash-flow recurrence in checkingSeries.
const demoCheckingOpening = 12_000_000.0

// demoExpenseSeries is the household's chosen monthly living-expenses curve — a
// modest base with gentle lifestyle creep (tracking the salary drift) plus a
// mild seasonal swing, always well under monthly income so the checking plug
// trends upward. Because the checking account is seeded as the reconciling plug
// (checkingSeries), the engine's *derived* Living Expenses come out equal to
// this series (± a small foreign-currency revaluation leak), rather than an
// arbitrary residual (#497, INV-FINANCE-28).
func demoExpenseSeries() []float64 {
	e := make([]float64, demoMonthCount)
	for i := range e {
		e[i] = 5_200_000.0 + 25_000.0*float64(i) + 350_000.0*math.Sin(float64(i)*1.05)
	}
	return e
}

// demoCashLedger accumulates the household's monthly IDR cash flows as the rest
// of the demo is seeded, so the Everyday Checking balance can be seeded last as
// the reconciling plug (seedDemoCheckingPlug). Every cash movement a real
// household routes through its bank is recorded here: income in, net investment
// purchases out, transfers into the other bank/savings series, debt principal
// paid, money lent, and the chosen living expenses. Foreign-currency positions
// are deliberately *not* recorded — their IDR value carries an FX revaluation
// that isn't cash — so they leak a small (kept tiny) wobble into the engine's
// derived expenses instead of the checking balance (see seedDemoData).
type demoCashLedger struct {
	income       []float64 // earned income landing in the bank
	investNetIn  []float64 // engine cash_in − cash_out: +Buy/placement, −Sell/Dividend/Distribution/Coupon
	savingsDelta []float64 // month-over-month change in the other (IDR) bank/savings series
	recvDelta    []float64 // month-over-month change in (IDR) receivables — positive = money lent
	liabDelta    []float64 // month-over-month change in (IDR) liability magnitude — negative = paid down
	expenses     []float64 // chosen living-expenses series
}

func newDemoCashLedger() *demoCashLedger {
	n := demoMonthCount
	return &demoCashLedger{
		income:       make([]float64, n),
		investNetIn:  make([]float64, n),
		savingsDelta: make([]float64, n),
		recvDelta:    make([]float64, n),
		liabDelta:    make([]float64, n),
		expenses:     demoExpenseSeries(),
	}
}

func (l *demoCashLedger) addIncome(monthIdx int, amt float64) { l.income[monthIdx] += amt }

// addInvest records a signed investment cash flow: positive for a Buy (cash out
// of the bank, into the position), negative for a payout the household receives
// (Sell/Dividend/Distribution/Coupon). Mirrors the engine's transactionCashFlows
// sign convention so the checking plug funds exactly what the return math books.
func (l *demoCashLedger) addInvest(monthIdx int, cashIn float64) { l.investNetIn[monthIdx] += cashIn }

// recordSeriesDelta feeds a seeded IDR position's month-over-month changes into
// one of the delta buckets, so the checking plug accounts for the cash those
// changes represent (a savings deposit, debt paid down, money lent/repaid).
func recordSeriesDelta(dst, series []float64) {
	for i := 1; i < len(series); i++ {
		dst[i] += series[i] - series[i-1]
	}
}

// checkingSeries derives the Everyday Checking balance for every month from the
// opening balance and the accumulated cash flows, forcing the engine's derived
// Living Expenses to equal demoExpenseSeries. The recurrence is the cash-flow
// rearrangement of the engine's comprehensive-income identity (INV-FINANCE-28):
//
//	livingExpenses = income − Δbank − Δreceivables + Δliabilities − netInvestCashIn
//
// solved for Δchecking (Δbank restricted to this account, the plug):
//
//	Δchecking = income − Δsavings − Δreceivables + Δliabilities − netInvestCashIn − expenses
func (l *demoCashLedger) checkingSeries() []float64 {
	n := len(l.expenses)
	c := make([]float64, n)
	c[0] = demoCheckingOpening
	for i := 1; i < n; i++ {
		c[i] = c[i-1] + l.income[i] - l.savingsDelta[i] - l.recvDelta[i] + l.liabDelta[i] - l.investNetIn[i] - l.expenses[i]
	}
	return c
}

// seedDemoData wipes-then-rebuilds the shared demo Household's entire toy
// dataset (ADR-0041, #217): at least two Positions per Asset/Investment
// subtype plus Liabilities, Receivables, Income, and Tags, each carrying a
// multi-year snapshot trail — enough spread that a first-time visitor can see
// every position type, tag breakdown, income category, and a year-over-year
// net-worth comparison in action rather than an empty-feeling dashboard.
func seedDemoData(ctx context.Context, pool *pgxpool.Pool, ownerID, member2ID uuid.UUID) error {
	// The demo is seeded as one coherent household cash flow: every position's
	// cash movements accumulate in ledger, and the Everyday Checking account is
	// seeded last as the reconciling plug (seedDemoCheckingPlug) so the engine's
	// derived Living Expenses stay positive and equal to demoExpenseSeries every
	// month (#497, INV-FINANCE-28) rather than an arbitrary residual. Order
	// matters: checking must be seeded after every other cash flow is recorded.
	ledger := newDemoCashLedger()
	tags, err := seedDemoTags(ctx, pool)
	if err != nil {
		return err
	}
	if err := seedDemoFxRates(ctx, pool); err != nil {
		return err
	}
	checkingID, err := seedDemoAssets(ctx, pool, ownerID, member2ID, tags, ledger)
	if err != nil {
		return err
	}
	if err := seedDemoInvestments(ctx, pool, ownerID, member2ID, tags, ledger); err != nil {
		return err
	}
	if err := seedDemoLiabilitiesAndReceivables(ctx, pool, ownerID, member2ID, tags, ledger); err != nil {
		return err
	}
	if err := seedDemoIncome(ctx, pool, ownerID, member2ID, ledger); err != nil {
		return err
	}
	if err := seedDemoInflation(ctx, pool); err != nil {
		return err
	}
	if err := seedDemoCheckingPlug(ctx, pool, checkingID, ledger); err != nil {
		return err
	}
	return nil
}

// seedDemoCheckingPlug seeds the Everyday Checking snapshot trail from the
// accumulated cash ledger (checkingSeries) — the reconciling balance that makes
// the household's income, spending, investment purchases and payouts tally
// (#497, INV-FINANCE-28). Seeded last, after every other cash flow is recorded.
func seedDemoCheckingPlug(ctx context.Context, pool *pgxpool.Pool, assetID uuid.UUID, ledger *demoCashLedger) error {
	assets := repo.NewAssetRepo(pool)
	series := ledger.checkingSeries()
	for i, ym := range demoMonths() {
		if _, err := assets.CreateAssetSnapshot(ctx, repo.CreateAssetSnapshotParams{
			AssetID:   assetID,
			YearMonth: ym,
			Amount:    demoDecimal(series[i]),
			Currency:  demoCurrency,
		}); err != nil {
			return fmt.Errorf("demo reset: seed checking plug snapshot: %w", err)
		}
	}
	return nil
}

// seedDemoInflation seeds a handful of recent monthly inflation figures
// (ADR-0048) — annualized YoY percentages around recent Indonesian CPI — so the
// statistics panel's Fund Resilience runs off stored figures rather than only
// the assumed_annual_inflation fallback. Seeded across the trailing-12 window
// the projection averages, oldest months left empty to exercise the fallback too.
func seedDemoInflation(ctx context.Context, pool *pgxpool.Pool) error {
	rates := repo.NewInflationRateRepo(pool)
	months := demoMonths()
	// Plausible recent YoY CPI, most-recent last.
	pct := []string{"3.4", "3.1", "2.9", "2.8", "3.0", "3.2", "3.3", "2.9", "2.7", "2.6", "2.8", "3.0"}
	recent := months[len(months)-len(pct):]
	for i, ym := range recent {
		if _, err := rates.CreateInflationRate(ctx, repo.CreateInflationRateParams{
			YearMonth: ym,
			Rate:      decimal.RequireFromString(pct[i]),
		}); err != nil {
			return fmt.Errorf("demo reset: seed inflation rate: %w", err)
		}
	}
	return nil
}

// seedDemoTags creates a handful of household-defined Tags (ADR-0028)
// spanning the fixed swatch palette, so the Tag-breakdown report has more
// than one slice to show. Not every seeded Position gets one — a few stay
// Untagged so that bucket renders too.
func seedDemoTags(ctx context.Context, pool *pgxpool.Pool) (map[string]uuid.UUID, error) {
	tagRepo := repo.NewTagRepo(pool)
	defs := []struct{ name, color string }{
		{"Emergency Fund", "#3b82f6"},
		{"Retirement", "#10b981"},
		{"Education", "#a855f7"},
		{"Short-term Goals", "#f97316"},
		{"Big Ticket", "#ef4444"},
	}
	tags := make(map[string]uuid.UUID, len(defs))
	for _, d := range defs {
		t, err := tagRepo.CreateTag(ctx, d.name, d.color)
		if err != nil {
			return nil, fmt.Errorf("demo reset: seed tag %q: %w", d.name, err)
		}
		tags[d.name] = t.ID
	}
	return tags, nil
}

// seedDemoFxRates seeds the USD rate the household's multi-currency toggle
// (flipped on in resetDemoHousehold) needs to convert the few USD-denominated
// Positions below into the IDR-reporting nw_total. Rate resolution is
// carry-forward — latestAtOrBefore, ADR-0002 — so one row dated at or before
// a month covers every later month; three rows here just give the Settings
// "Exchange rates" card some gentle movement to display rather than a single
// static number.
func seedDemoFxRates(ctx context.Context, pool *pgxpool.Pool) error {
	fxRates := repo.NewFxRateRepo(pool)
	months := demoMonths()
	rates := []struct {
		monthIdx int
		rate     float64
	}{
		{0, 15_800},
		{13, 16_200},
		{len(months) - 1, 16_450},
	}
	for _, r := range rates {
		if _, err := fxRates.CreateFxRate(ctx, repo.CreateFxRateParams{
			YearMonth: months[r.monthIdx],
			Currency:  "USD",
			Rate:      demoDecimal(r.rate),
		}); err != nil {
			return fmt.Errorf("demo reset: seed USD fx rate: %w", err)
		}
	}
	return nil
}

// seedAssetSnapshots backfills the full demoMonths value trail (amount-only,
// ADR-0022 snapshot shape) for a Bank Account / Property / Vehicle Position.
// currency must match the Position's own native_currency — a snapshot tagged
// with the wrong currency either skips FX conversion entirely (if it
// happens to equal the reporting currency) or converts at the wrong rate.
func seedAssetSnapshots(ctx context.Context, assets *repo.AssetRepo, assetID uuid.UUID, base, growth, wobble float64, phaseIdx int, currency string) ([]float64, error) {
	months := demoMonths()
	series := demoSeries(base, growth, wobble, phaseIdx, len(months))
	for i, ym := range months {
		if _, err := assets.CreateAssetSnapshot(ctx, repo.CreateAssetSnapshotParams{
			AssetID:   assetID,
			YearMonth: ym,
			Amount:    demoDecimal(series[i]),
			Currency:  currency,
		}); err != nil {
			return nil, fmt.Errorf("demo reset: seed asset snapshot: %w", err)
		}
	}
	return series, nil
}

// seedRevaluationSnapshots backfills a Property / Vehicle value trail as a
// clean annual-rate revaluation (demoRevaluationSeries) rather than the
// sinusoidal demoSeries — so the trail matches the position's advertised
// annual_appreciation_rate / annual_depreciation_rate. Property/vehicle
// revaluation is pure mark-to-market: it cancels out of the engine's
// living-expenses identity (avc − avc), so unlike bank/savings it feeds no cash
// ledger. annualRatePct is signed (positive appreciates, negative depreciates).
func seedRevaluationSnapshots(ctx context.Context, assets *repo.AssetRepo, assetID uuid.UUID, base, annualRatePct float64, currency string) error {
	months := demoMonths()
	series := demoRevaluationSeries(base, annualRatePct, len(months))
	for i, ym := range months {
		if _, err := assets.CreateAssetSnapshot(ctx, repo.CreateAssetSnapshotParams{
			AssetID:   assetID,
			YearMonth: ym,
			Amount:    demoDecimal(series[i]),
			Currency:  currency,
		}); err != nil {
			return fmt.Errorf("demo reset: seed revaluation snapshot: %w", err)
		}
	}
	return nil
}

// seedMarketPosition seeds a Stock / MutualFund / Gold Position's full trail —
// snapshots and the Buy/Sell ledger that backs them — from a single trade
// schedule, so the snapshot quantity each month equals the ledger's net
// quantity (cumulativeQty) and their values can never drift apart (#497). Every
// trade also records its cash flow into ledger so the checking plug funds it.
// The first trade should be dated at month 0 (the acquisition, a baseline month
// that books no return per INV-FINANCE-23) so the position is present across the
// whole history rather than appearing mid-chart. prices is the shared price
// series (the same one passed to seedMarketSnapshots), so a Buy at month i
// prices at exactly that month's snapshot price. currency must match the
// Position's own native_currency (see seedAssetSnapshots).
func seedMarketPosition(ctx context.Context, investments *repo.InvestmentRepo, investmentID uuid.UUID, trades []demoTrade, prices []float64, currency string, ledger *demoCashLedger) error {
	if err := seedMarketSnapshots(ctx, investments, investmentID, trades, prices, currency); err != nil {
		return err
	}
	for _, t := range trades {
		txnType := repo.TxnTypeBuy
		qty := t.deltaQty
		if qty < 0 {
			txnType = repo.TxnTypeSell
			qty = -qty
		}
		if err := seedInvestmentTrade(ctx, investments, investmentID, txnType, demoMonths(), t.monthIdx, qty, prices, ledger); err != nil {
			return err
		}
	}
	return nil
}

// seedMarketSnapshots backfills the full demoMonths quantity+price trail for a
// Stock / MutualFund / Gold Position, given a precomputed price series of the
// same length. The quantity held in month i is the running sum of every trade
// dated at or before it (cumulativeQty) — so the snapshot always equals the
// ledger's net Buy/Sell quantity times that month's price, the frontend's
// reconcileQuantity check holds by construction, and the engine's per-month
// return collapses to pure price appreciation (INV-FINANCE-08, #497). currency
// must match the Position's own native_currency (see seedAssetSnapshots).
func seedMarketSnapshots(ctx context.Context, investments *repo.InvestmentRepo, investmentID uuid.UUID, trades []demoTrade, prices []float64, currency string) error {
	for i, ym := range demoMonths() {
		price := demoDecimal(prices[i])
		q := demoDecimal(cumulativeQty(trades, i))
		if _, err := investments.CreateInvestmentSnapshot(ctx, repo.CreateInvestmentSnapshotParams{
			InvestmentID: investmentID,
			YearMonth:    ym,
			Amount:       q.Mul(price),
			Currency:     currency,
			Quantity:     &q,
			PricePerUnit: &price,
		}); err != nil {
			return fmt.Errorf("demo reset: seed market snapshot: %w", err)
		}
	}
	return nil
}

// seedInvestmentTrade records a Buy/Sell transaction priced off the same
// series seedMarketSnapshots (or seedInterestBearingSnapshots) used for that
// position's month i, so the recorded trade price matches what the chart
// shows for that month. months/prices must be the same slice the caller used
// to seed that position's snapshots (the full demoMonths for Stock/
// MutualFund/Gold, or demoRecentMonths for Bond/TimeDeposit).
func seedInvestmentTrade(ctx context.Context, investments *repo.InvestmentRepo, investmentID uuid.UUID, txnType string, months []time.Time, monthIdx int, quantity float64, prices []float64, ledger *demoCashLedger) error {
	qty := demoDecimal(quantity)
	price := demoDecimal(prices[monthIdx])
	amount := qty.Mul(price)
	date := dayIn(months[monthIdx], 5)
	if _, err := investments.CreateInvestmentTransaction(ctx, repo.CreateInvestmentTransactionParams{
		InvestmentID:    investmentID,
		TransactionType: txnType,
		TransactionDate: date,
		Currency:        demoCurrency,
		Amount:          &amount,
		Quantity:        &qty,
		PricePerUnit:    &price,
	}); err != nil {
		return fmt.Errorf("demo reset: seed %s trade: %w", txnType, err)
	}
	// A Buy is cash out of the bank into the position, a Sell is cash back in.
	cashIn, _ := amount.Float64()
	if txnType == repo.TxnTypeSell {
		cashIn = -cashIn
	}
	ledger.addInvest(monthIdx, cashIn)
	return nil
}

// seedInvestmentCashEvent records an amount-only ledger entry — Dividend,
// Distribution, Coupon, or Fee — none of which carry Quantity/PricePerUnit
// (ADR-0009's transaction-shape rules). months follows the same convention as
// seedInvestmentTrade.
func seedInvestmentCashEvent(ctx context.Context, investments *repo.InvestmentRepo, investmentID uuid.UUID, txnType string, months []time.Time, monthIdx, day int, amount float64, ledger *demoCashLedger) error {
	amt := demoDecimal(amount)
	date := dayIn(months[monthIdx], day)
	if _, err := investments.CreateInvestmentTransaction(ctx, repo.CreateInvestmentTransactionParams{
		InvestmentID:    investmentID,
		TransactionType: txnType,
		TransactionDate: date,
		Currency:        demoCurrency,
		Amount:          &amt,
	}); err != nil {
		return fmt.Errorf("demo reset: seed %s: %w", txnType, err)
	}
	// Dividend/Distribution/Coupon are cash the household receives (cash_out of
	// the position → into the bank); a quantity-less Fee is a capitalised cost
	// paid from the bank (cash_in, matching the engine's transactionCashFlows).
	amtF, _ := amt.Float64()
	if txnType == repo.TxnTypeFee {
		ledger.addInvest(monthIdx, amtF)
	} else {
		ledger.addInvest(monthIdx, -amtF)
	}
	return nil
}

// seedInterestBearingSnapshots backfills a total-value+accrued trail (the
// "dirty total value" shape, ADR-0022) across the given months — Bond/
// TimeDeposit Positions pass demoRecentMonths since they're seeded as if
// placed 12 months ago, not backfilled across the full history window.
// resetEveryMonths cycles accrued back to near-zero on a coupon schedule
// (secondary-market bonds that accrue); 0 means it grows monotonically for
// the whole window instead (a time deposit accruing toward its single
// maturity payout). accrualStep 0 keeps accrued pinned at zero throughout
// (a pays-out govt bond, where the coupon lands in the bank, not the
// instrument). currency must match the Position's own native_currency (see
// seedAssetSnapshots).
func seedInterestBearingSnapshots(ctx context.Context, investments *repo.InvestmentRepo, investmentID uuid.UUID, principalBase, principalGrowth, principalWobble, accrualStep float64, resetEveryMonths, phaseIdx int, months []time.Time, currency string) error {
	principals := demoSeries(principalBase, principalGrowth, principalWobble, phaseIdx, len(months))
	for i, ym := range months {
		var accruedF float64
		if resetEveryMonths > 0 {
			accruedF = accrualStep * float64(i%resetEveryMonths)
		} else {
			accruedF = accrualStep * float64(i+1)
		}
		accrued := demoDecimal(accruedF)
		amount := demoDecimal(principals[i]).Add(accrued)
		if _, err := investments.CreateInvestmentSnapshot(ctx, repo.CreateInvestmentSnapshotParams{
			InvestmentID:    investmentID,
			YearMonth:       ym,
			Amount:          amount,
			Currency:        currency,
			AccruedInterest: &accrued,
		}); err != nil {
			return fmt.Errorf("demo reset: seed interest-bearing snapshot: %w", err)
		}
	}
	return nil
}

// seedLiabilitySnapshots backfills the full demoMonths balance trail for a
// Liability. currency must match the Position's own native_currency (see
// seedAssetSnapshots).
func seedLiabilitySnapshots(ctx context.Context, liabilities *repo.LiabilityRepo, liabilityID uuid.UUID, series []float64, currency string) error {
	for i, ym := range demoMonths() {
		if _, err := liabilities.CreateLiabilitySnapshot(ctx, repo.CreateLiabilitySnapshotParams{
			LiabilityID: liabilityID,
			YearMonth:   ym,
			Amount:      demoDecimal(series[i]),
			Currency:    currency,
		}); err != nil {
			return fmt.Errorf("demo reset: seed liability snapshot: %w", err)
		}
	}
	return nil
}

// seedReceivableSnapshots backfills the full demoMonths balance trail for a
// Receivable. currency must match the Position's own native_currency (see
// seedAssetSnapshots).
func seedReceivableSnapshots(ctx context.Context, receivables *repo.ReceivableRepo, receivableID uuid.UUID, series []float64, currency string) error {
	for i, ym := range demoMonths() {
		if _, err := receivables.CreateReceivableSnapshot(ctx, repo.CreateReceivableSnapshotParams{
			ReceivableID: receivableID,
			YearMonth:    ym,
			Amount:       demoDecimal(series[i]),
			Currency:     currency,
		}); err != nil {
			return fmt.Errorf("demo reset: seed receivable snapshot: %w", err)
		}
	}
	return nil
}

// seedDemoAssets seeds two Positions each of BankAccount, Property, and
// Vehicle — the three Asset subtypes (ADR-0022) — with a full snapshot trail
// and a mix of sole/joint ownership and tag assignment. Bases are tuned for a
// modest ~400M IDR household net worth (not a wealthy one). Property and
// vehicle trails revalue at realistic annual rates (seedRevaluationSnapshots,
// matching each position's advertised appreciation/depreciation rate) rather
// than a boosted curve — the growth story now comes from the coherent cash
// flow and modest, believable revaluation, not an inflated appreciation rate.
// The Everyday Checking account is the one
// exception: it is NOT seeded here but returned to the caller and seeded last
// as the household cash plug (seedDemoCheckingPlug), and the Joint Savings
// series is recorded into the ledger so the plug funds its deposits.
func seedDemoAssets(ctx context.Context, pool *pgxpool.Pool, ownerID, member2ID uuid.UUID, tags map[string]uuid.UUID, ledger *demoCashLedger) (uuid.UUID, error) {
	assets := repo.NewAssetRepo(pool)
	tagRepo := repo.NewTagRepo(pool)
	months := demoMonths()
	acqLong := months[0].AddDate(-6, 0, 0)

	checking, err := assets.CreateBankAccount(ctx, repo.CreateBankAccountParams{
		DisplayName:     "Everyday Checking",
		OwnershipType:   "sole",
		SoleOwnerUserID: &ownerID,
		NativeCurrency:  demoCurrency,
		BankName:        "Demo Bank",
		AccountNumber:   "1234567890",
		AccountType:     "savings",
	})
	if err != nil {
		return uuid.Nil, fmt.Errorf("demo reset: seed bank account (checking): %w", err)
	}
	// Checking snapshots are seeded last by seedDemoCheckingPlug — it is the
	// reconciling cash balance derived from every other position's flows.

	savings, err := assets.CreateBankAccount(ctx, repo.CreateBankAccountParams{
		DisplayName:    "Joint Savings",
		OwnershipType:  "joint",
		NativeCurrency: demoCurrency,
		BankName:       "Demo Bank",
		AccountNumber:  "9876543210",
		AccountType:    "savings",
	})
	if err != nil {
		return uuid.Nil, fmt.Errorf("demo reset: seed bank account (savings): %w", err)
	}
	savingsSeries, err := seedAssetSnapshots(ctx, assets, savings.Asset.ID, 9_000_000, 0.03, 0.015, 1, demoCurrency)
	if err != nil {
		return uuid.Nil, err
	}
	// Savings deposits are real cash out of checking — record so the plug funds them.
	recordSeriesDelta(ledger.savingsDelta, savingsSeries)
	if err := tagRepo.AssignTag(ctx, repo.TagGroupAsset, savings.Asset.ID, tagPtr(tags, "Emergency Fund")); err != nil {
		return uuid.Nil, fmt.Errorf("demo reset: tag joint savings: %w", err)
	}

	familyHome, err := assets.CreateProperty(ctx, repo.CreatePropertyParams{
		DisplayName:            "Family Home",
		OwnershipType:          "joint",
		NativeCurrency:         demoCurrency,
		PropertyType:           "house",
		Address:                strPtr("Jl. Demo Raya No. 1"),
		AcquisitionDate:        &acqLong,
		AcquisitionCost:        decimalPtr(200_000_000),
		AnnualAppreciationRate: decimalPtr(4), // 4%/yr (≈ 200M acquisition compounded over the 6-year hold)
	})
	if err != nil {
		return uuid.Nil, fmt.Errorf("demo reset: seed property (family home): %w", err)
	}
	// Property/vehicle value changes are revaluation, not cash — they cancel out
	// of the engine's living-expenses identity, so their series feed no ledger.
	// The trail compounds the advertised annual rate rather than wobbling.
	if err := seedRevaluationSnapshots(ctx, assets, familyHome.Asset.ID, 250_000_000, 4, demoCurrency); err != nil {
		return uuid.Nil, err
	}
	if err := tagRepo.AssignTag(ctx, repo.TagGroupAsset, familyHome.Asset.ID, tagPtr(tags, "Big Ticket")); err != nil {
		return uuid.Nil, fmt.Errorf("demo reset: tag family home: %w", err)
	}

	rental, err := assets.CreateProperty(ctx, repo.CreatePropertyParams{
		DisplayName:            "Rental Apartment",
		OwnershipType:          "sole",
		SoleOwnerUserID:        &ownerID,
		NativeCurrency:         demoCurrency,
		PropertyType:           "apartment",
		Address:                strPtr("Jl. Demo Kedua No. 12"),
		AcquisitionDate:        &acqLong,
		AcquisitionCost:        decimalPtr(145_000_000),
		AnnualAppreciationRate: decimalPtr(3.5), // 3.5%/yr
	})
	if err != nil {
		return uuid.Nil, fmt.Errorf("demo reset: seed property (rental): %w", err)
	}
	if err := seedRevaluationSnapshots(ctx, assets, rental.Asset.ID, 165_000_000, 3.5, demoCurrency); err != nil {
		return uuid.Nil, err
	}

	car, err := assets.CreateVehicle(ctx, repo.CreateVehicleParams{
		DisplayName:            "Family Car",
		OwnershipType:          "joint",
		NativeCurrency:         demoCurrency,
		VehicleType:            "car",
		Make:                   strPtr("Toyota"),
		Model:                  strPtr("Demo Sedan"),
		Year:                   int32Ptr(2022),
		PlateNumber:            strPtr("B 1234 DEM"),
		AnnualDepreciationRate: decimalPtr(8), // 8%/yr — stored unsigned; the trail declines
	})
	if err != nil {
		return uuid.Nil, fmt.Errorf("demo reset: seed vehicle (car): %w", err)
	}
	if err := seedRevaluationSnapshots(ctx, assets, car.Asset.ID, 50_000_000, -8, demoCurrency); err != nil {
		return uuid.Nil, err
	}
	if err := tagRepo.AssignTag(ctx, repo.TagGroupAsset, car.Asset.ID, tagPtr(tags, "Big Ticket")); err != nil {
		return uuid.Nil, fmt.Errorf("demo reset: tag family car: %w", err)
	}

	bike, err := assets.CreateVehicle(ctx, repo.CreateVehicleParams{
		DisplayName:            "Motorbike",
		OwnershipType:          "sole",
		SoleOwnerUserID:        &member2ID,
		NativeCurrency:         demoCurrency,
		VehicleType:            "motorcycle",
		Make:                   strPtr("Demo Moto"),
		Model:                  strPtr("Scoot 125"),
		Year:                   int32Ptr(2023),
		PlateNumber:            strPtr("B 5678 DEM"),
		AnnualDepreciationRate: decimalPtr(10), // 10%/yr — bikes shed value faster than cars
	})
	if err != nil {
		return uuid.Nil, fmt.Errorf("demo reset: seed vehicle (bike): %w", err)
	}
	if err := seedRevaluationSnapshots(ctx, assets, bike.Asset.ID, 5_000_000, -10, demoCurrency); err != nil {
		return uuid.Nil, err
	}

	// USD Savings — a foreign-currency account (common in Indonesia for
	// travel/hedging savings), one of the "select few" USD positions
	// exercising multi-currency conversion (seedDemoFxRates provides the
	// rate). native_currency USD, snapshot amounts are USD, not IDR.
	usdSavings, err := assets.CreateBankAccount(ctx, repo.CreateBankAccountParams{
		DisplayName:     "USD Savings",
		OwnershipType:   "sole",
		SoleOwnerUserID: &member2ID,
		NativeCurrency:  "USD",
		BankName:        "Demo Bank",
		AccountNumber:   "1122334455",
		AccountType:     "savings",
	})
	if err != nil {
		return uuid.Nil, fmt.Errorf("demo reset: seed bank account (USD savings): %w", err)
	}
	// USD Savings is deliberately kept small and left out of the cash ledger: its
	// IDR value carries an FX revaluation that isn't cash, so its monthly change
	// leaks a small (kept tiny) wobble into the engine's derived expenses rather
	// than distorting the IDR-reconciled checking plug.
	if _, err := seedAssetSnapshots(ctx, assets, usdSavings.Asset.ID, 600, 0.02, 0.01, 20, "USD"); err != nil {
		return uuid.Nil, err
	}

	return checking.Asset.ID, nil
}

// seedDemoInvestments seeds two Positions each of the five Investment
// subtypes (Stock, MutualFund, Bond, Gold, TimeDeposit — ADR-0022) with a
// full snapshot trail across the full history window (each holds an opening
// lot from month 0, so no position "appears" mid-chart with a discontinuous
// jump in the net-worth series) that then accumulates as later Buys land.
// Stock/MutualFund/Gold prices are kept at realistic market levels
// (rescaling a real per-share/per-gram price would look wrong to anyone who
// knows it) — the modest-household rescale instead shrinks the quantity
// held.
func seedDemoInvestments(ctx context.Context, pool *pgxpool.Pool, ownerID, member2ID uuid.UUID, tags map[string]uuid.UUID, ledger *demoCashLedger) error {
	investments := repo.NewInvestmentRepo(pool)
	tagRepo := repo.NewTagRepo(pool)
	months := demoMonths()

	stock1, err := investments.CreateStock(ctx, repo.CreateStockParams{
		DisplayName:     "Bank Central Asia",
		OwnershipType:   "sole",
		SoleOwnerUserID: &ownerID,
		NativeCurrency:  demoCurrency,
		RiskProfile:     "high",
		Ticker:          "BBCA",
		Exchange:        "IDX",
	})
	if err != nil {
		return fmt.Errorf("demo reset: seed stock (BBCA): %w", err)
	}
	stock1Prices := demoSeries(9_500, 0.012, 0.04, 6, len(months))
	// Ledger: an opening lot (month 0, the acquisition) plus follow-on Buys
	// sprinkled across the trail, netting to 20 shares — snapshot quantity steps
	// with each Buy so the value tracks the ledger (#497). A Dividend and a
	// small capitalized custody Fee round out a realistic cost basis.
	if err := seedMarketPosition(ctx, investments, stock1.Investment.ID, []demoTrade{{0, 10}, {5, 6}, {17, 4}}, stock1Prices, demoCurrency, ledger); err != nil {
		return err
	}
	if err := seedInvestmentCashEvent(ctx, investments, stock1.Investment.ID, repo.TxnTypeDividend, months, 13, 15, 100_000, ledger); err != nil {
		return err
	}
	if err := seedInvestmentCashEvent(ctx, investments, stock1.Investment.ID, repo.TxnTypeFee, months, 22, 20, 15_000, ledger); err != nil {
		return err
	}

	stock2, err := investments.CreateStock(ctx, repo.CreateStockParams{
		DisplayName:    "Astra International",
		OwnershipType:  "joint",
		NativeCurrency: demoCurrency,
		RiskProfile:    "high",
		Ticker:         "ASII",
		Exchange:       "IDX",
	})
	if err != nil {
		return fmt.Errorf("demo reset: seed stock (ASII): %w", err)
	}
	stock2Prices := demoSeries(5_200, 0.008, 0.05, 7, len(months))
	// Ledger: an opening lot and one later top-up Buy netting to 40 shares, plus
	// a Dividend.
	if err := seedMarketPosition(ctx, investments, stock2.Investment.ID, []demoTrade{{0, 25}, {9, 15}}, stock2Prices, demoCurrency, ledger); err != nil {
		return err
	}
	if err := seedInvestmentCashEvent(ctx, investments, stock2.Investment.ID, repo.TxnTypeDividend, months, 20, 15, 130_000, ledger); err != nil {
		return err
	}
	if err := tagRepo.AssignTag(ctx, repo.TagGroupInvestment, stock2.Investment.ID, tagPtr(tags, "Short-term Goals")); err != nil {
		return fmt.Errorf("demo reset: tag ASII: %w", err)
	}

	mf1, err := investments.CreateMutualFund(ctx, repo.CreateMutualFundParams{
		DisplayName:     "Equity Growth Fund",
		OwnershipType:   "sole",
		SoleOwnerUserID: &member2ID,
		NativeCurrency:  demoCurrency,
		RiskProfile:     "high",
		FundCode:        "EGF01",
		FundManager:     strPtr("Demo Asset Management"),
		FundType:        "equity",
	})
	if err != nil {
		return fmt.Errorf("demo reset: seed mutual fund (equity growth): %w", err)
	}
	mf1Prices := demoSeries(1_450, 0.01, 0.03, 8, len(months))
	// Ledger: an opening lot, two accumulating Buys and a partial profit-take
	// Sell netting to 200 units, plus a Distribution.
	if err := seedMarketPosition(ctx, investments, mf1.Investment.ID, []demoTrade{{0, 100}, {4, 80}, {11, 40}, {20, -20}}, mf1Prices, demoCurrency, ledger); err != nil {
		return err
	}
	if err := seedInvestmentCashEvent(ctx, investments, mf1.Investment.ID, repo.TxnTypeDistribution, months, 15, 10, 60_000, ledger); err != nil {
		return err
	}
	if err := tagRepo.AssignTag(ctx, repo.TagGroupInvestment, mf1.Investment.ID, tagPtr(tags, "Retirement")); err != nil {
		return fmt.Errorf("demo reset: tag equity growth fund: %w", err)
	}

	mf2, err := investments.CreateMutualFund(ctx, repo.CreateMutualFundParams{
		DisplayName:    "Money Market Fund",
		OwnershipType:  "joint",
		NativeCurrency: demoCurrency,
		RiskProfile:    "low",
		FundCode:       "MMF01",
		FundManager:    strPtr("Demo Asset Management"),
		FundType:       "money_market",
	})
	if err != nil {
		return fmt.Errorf("demo reset: seed mutual fund (money market): %w", err)
	}
	mf2Prices := demoSeries(1_050, 0.002, 0.005, 9, len(months))
	// Ledger: an opening lot and one top-up Buy netting to 1,000 units, plus two
	// Distributions (money-market funds pay out routinely).
	if err := seedMarketPosition(ctx, investments, mf2.Investment.ID, []demoTrade{{0, 600}, {7, 400}}, mf2Prices, demoCurrency, ledger); err != nil {
		return err
	}
	if err := seedInvestmentCashEvent(ctx, investments, mf2.Investment.ID, repo.TxnTypeDistribution, months, 12, 10, 40_000, ledger); err != nil {
		return err
	}
	if err := seedInvestmentCashEvent(ctx, investments, mf2.Investment.ID, repo.TxnTypeDistribution, months, 24, 10, 40_000, ledger); err != nil {
		return err
	}

	bondPlacement := dayIn(months[0], 10)
	bondMaturity := bondPlacement.AddDate(3, 0, 0)
	bond1, err := investments.CreateBond(ctx, repo.CreateBondParams{
		DisplayName:       "ORI024 Retail Bond",
		OwnershipType:     "sole",
		SoleOwnerUserID:   &ownerID,
		NativeCurrency:    demoCurrency,
		RiskProfile:       "low",
		BondType:          "govt_primary",
		SeriesCode:        strPtr("ORI024"),
		Issuer:            "Government of Indonesia",
		CouponRate:        decimal.RequireFromString("6.25"),
		CouponFrequency:   "monthly",
		CouponDisposition: repo.CouponDispositionPaysOut,
		MaturityDate:      bondMaturity,
		FaceValue:         decimalPtr(10_000_000),
		PlacementDate:     &bondPlacement,
	})
	if err != nil {
		return fmt.Errorf("demo reset: seed bond (ORI024): %w", err)
	}
	if err := seedInterestBearingSnapshots(ctx, investments, bond1.Investment.ID, 10_000_000, 0, 0, 0, 1, 10, months, demoCurrency); err != nil {
		return err
	}
	// CreateBond already seeded the placement Buy (govt_primary auto-seed).
	// CouponDisposition is pays_out with a monthly frequency, matching the
	// accrued-interest snapshot resetting to 0 every month — so the cash
	// actually paid out each month belongs in the ledger too. One Coupon per
	// month since placement: face * annual rate / 12.
	bond1MonthlyCoupon := 10_000_000 * 6.25 / 100 / 12
	for i := 1; i < len(months); i++ {
		if err := seedInvestmentCashEvent(ctx, investments, bond1.Investment.ID, repo.TxnTypeCoupon, months, i, 10, bond1MonthlyCoupon, ledger); err != nil {
			return err
		}
	}

	bond2Maturity := dayIn(months[len(months)-1], 1).AddDate(4, 0, 0)
	bond2, err := investments.CreateBond(ctx, repo.CreateBondParams{
		DisplayName:       "Astra Sedaya Corporate Bond",
		OwnershipType:     "joint",
		NativeCurrency:    demoCurrency,
		RiskProfile:       "medium",
		BondType:          "secondary_market",
		Issuer:            "PT Astra Sedaya Finance",
		CouponRate:        decimal.RequireFromString("7.5"),
		CouponFrequency:   "semi_annual",
		CouponDisposition: "accrues",
		MaturityDate:      bond2Maturity,
	})
	if err != nil {
		return fmt.Errorf("demo reset: seed bond (Astra Sedaya): %w", err)
	}
	if err := seedInterestBearingSnapshots(ctx, investments, bond2.Investment.ID, 8_000_000, 0.002, 0.01, 50_000, 6, 11, months, demoCurrency); err != nil {
		return err
	}
	// secondary_market bonds aren't auto-seeded a placement Buy (CreateBond
	// only does that for govt_primary) — bought on the secondary market at a
	// slight premium over the 8M principal, plus a brokerage Fee.
	bond2Qty := decimal.RequireFromString("8")
	bond2Price := decimal.RequireFromString("1010000")
	bond2Amount := bond2Qty.Mul(bond2Price)
	if _, err := investments.CreateInvestmentTransaction(ctx, repo.CreateInvestmentTransactionParams{
		InvestmentID:    bond2.Investment.ID,
		TransactionType: repo.TxnTypeBuy,
		TransactionDate: months[0],
		Currency:        demoCurrency,
		Amount:          &bond2Amount,
		Quantity:        &bond2Qty,
		PricePerUnit:    &bond2Price,
	}); err != nil {
		return fmt.Errorf("demo reset: seed bond (Astra Sedaya) placement buy: %w", err)
	}
	// The placement Buy and brokerage Fee both land in the baseline month, which
	// books no Living Expenses, so they need no checking funding (passing ledger
	// records them harmlessly at index 0, which the plug recurrence skips).
	if err := seedInvestmentCashEvent(ctx, investments, bond2.Investment.ID, repo.TxnTypeFee, months, 0, 1, 30_000, ledger); err != nil {
		return err
	}
	if err := tagRepo.AssignTag(ctx, repo.TagGroupInvestment, bond2.Investment.ID, tagPtr(tags, "Short-term Goals")); err != nil {
		return fmt.Errorf("demo reset: tag Astra Sedaya bond: %w", err)
	}

	gold1, err := investments.CreateGold(ctx, repo.CreateGoldParams{
		DisplayName:     "Antam Gold Bar",
		OwnershipType:   "sole",
		SoleOwnerUserID: &ownerID,
		NativeCurrency:  demoCurrency,
		RiskProfile:     "medium",
		Form:            "bar",
		Purity:          decimal.RequireFromString("0.999"),
	})
	if err != nil {
		return fmt.Errorf("demo reset: seed gold (bar): %w", err)
	}
	gold1Prices := demoSeries(1_150_000, 0.008, 0.02, 12, len(months))
	// Ledger: an opening lot and two accumulating Buys netting to 10 grams
	// (the household's largest recurring cash outlays), plus a storage/insurance
	// Fee (Gold has no income transaction type).
	if err := seedMarketPosition(ctx, investments, gold1.Investment.ID, []demoTrade{{0, 5}, {8, 3}, {17, 2}}, gold1Prices, demoCurrency, ledger); err != nil {
		return err
	}
	if err := seedInvestmentCashEvent(ctx, investments, gold1.Investment.ID, repo.TxnTypeFee, months, 21, 25, 8_000, ledger); err != nil {
		return err
	}
	if err := tagRepo.AssignTag(ctx, repo.TagGroupInvestment, gold1.Investment.ID, tagPtr(tags, "Retirement")); err != nil {
		return fmt.Errorf("demo reset: tag gold bar: %w", err)
	}

	gold2, err := investments.CreateGold(ctx, repo.CreateGoldParams{
		DisplayName:    "Digital Gold Savings",
		OwnershipType:  "joint",
		NativeCurrency: demoCurrency,
		RiskProfile:    "medium",
		Form:           "digital",
		Purity:         decimal.RequireFromString("0.999"),
	})
	if err != nil {
		return fmt.Errorf("demo reset: seed gold (digital): %w", err)
	}
	gold2Prices := demoSeries(1_150_000, 0.008, 0.02, 13, len(months))
	// Ledger: an opening lot and one later top-up Buy netting to 4 grams.
	if err := seedMarketPosition(ctx, investments, gold2.Investment.ID, []demoTrade{{0, 2}, {13, 2}}, gold2Prices, demoCurrency, ledger); err != nil {
		return err
	}

	// Placed at months[0] like the assets above (full history, no appear-from-
	// nowhere jump); TermMonths is longer than the original 12-months-only
	// design needed, so a term anchored demoExtraHistoryMonths further back
	// still matures in the future.
	td1Placement := months[0]
	td1Maturity := td1Placement.AddDate(3, 0, 0)
	td1, err := investments.CreateTimeDeposit(ctx, repo.CreateTimeDepositParams{
		DisplayName:     "Mandiri Time Deposit",
		OwnershipType:   "sole",
		SoleOwnerUserID: &ownerID,
		NativeCurrency:  demoCurrency,
		RiskProfile:     "low",
		BankName:        "Bank Mandiri",
		Principal:       decimal.RequireFromString("20000000"),
		InterestRate:    decimal.RequireFromString("5.5"),
		TermMonths:      36,
		PlacementDate:   td1Placement,
		MaturityDate:    td1Maturity,
		RolloverPolicy:  "auto_renew_with_interest",
	})
	if err != nil {
		return fmt.Errorf("demo reset: seed time deposit (Mandiri): %w", err)
	}
	if err := seedInterestBearingSnapshots(ctx, investments, td1.Investment.ID, 20_000_000, 0, 0, 9_000, 0, 14, months, demoCurrency); err != nil {
		return err
	}
	if err := tagRepo.AssignTag(ctx, repo.TagGroupInvestment, td1.Investment.ID, tagPtr(tags, "Education")); err != nil {
		return fmt.Errorf("demo reset: tag Mandiri time deposit: %w", err)
	}

	// USD-denominated — Indonesian banks commonly offer USD time deposits at a
	// much lower rate than IDR ones (interest rate parity), which this reflects
	// (2.5% vs the IDR TDs' 5.5%/6%). One of the "select few" USD positions
	// exercising multi-currency conversion.
	td2Placement := months[0]
	td2Maturity := td2Placement.AddDate(0, 33, 0)
	td2, err := investments.CreateTimeDeposit(ctx, repo.CreateTimeDepositParams{
		DisplayName:    "BCA Time Deposit",
		OwnershipType:  "joint",
		NativeCurrency: "USD",
		RiskProfile:    "low",
		BankName:       "Bank BCA",
		Principal:      decimal.RequireFromString("2500"),
		InterestRate:   decimal.RequireFromString("2.5"),
		TermMonths:     33,
		PlacementDate:  td2Placement,
		MaturityDate:   td2Maturity,
		RolloverPolicy: "no_rollover",
	})
	if err != nil {
		return fmt.Errorf("demo reset: seed time deposit (BCA): %w", err)
	}
	if err := seedInterestBearingSnapshots(ctx, investments, td2.Investment.ID, 2_500, 0, 0, 5, 0, 15, months, "USD"); err != nil {
		return err
	}

	return nil
}

// seedDemoLiabilitiesAndReceivables seeds two Liabilities (one personal, one
// institutional) and two Receivables, each with a snapshot trail. Debt paydown
// and receivable movements are real cash out of / into the bank, so their IDR
// series changes are recorded into the ledger (the checking plug funds them);
// paydown rates are kept realistic rather than boosted, since debt paydown is
// now a genuine monthly cash outflow the checking balance must cover, not a
// free net-worth boost. The USD receivable is left out of the ledger (its IDR
// change carries FX revaluation that isn't cash — see seedDemoAssets).
func seedDemoLiabilitiesAndReceivables(ctx context.Context, pool *pgxpool.Pool, ownerID, member2ID uuid.UUID, tags map[string]uuid.UUID, ledger *demoCashLedger) error {
	liabilities := repo.NewLiabilityRepo(pool)
	receivables := repo.NewReceivableRepo(pool)
	tagRepo := repo.NewTagRepo(pool)
	months := demoMonths()
	earliest := months[0]

	familyLoanStart := dayIn(earliest, 5)
	familyLoan, err := liabilities.CreateLiability(ctx, repo.CreateLiabilityParams{
		DisplayName:      "Loan from Parents",
		Subtype:          "personal",
		OwnershipType:    "sole",
		SoleOwnerUserID:  &ownerID,
		NativeCurrency:   demoCurrency,
		CounterpartyName: "Parents",
		Principal:        decimalPtr(4_000_000),
		StartDate:        &familyLoanStart,
	})
	if err != nil {
		return fmt.Errorf("demo reset: seed liability (family loan): %w", err)
	}
	// Repaid to the parents in sporadic IDR 250,000 chunks — flat between
	// payments, not a smooth curve. 11 payments over the trail net 2.75M off the
	// 4M principal, leaving 1.25M still owed at the end.
	familyLoanSeries := demoStepDownSeries(4_000_000, 250_000, []int{1, 4, 5, 9, 12, 16, 17, 20, 23, 24, 26}, len(months))
	if err := seedLiabilitySnapshots(ctx, liabilities, familyLoan.ID, familyLoanSeries, demoCurrency); err != nil {
		return err
	}
	recordSeriesDelta(ledger.liabDelta, familyLoanSeries)

	mortgageStart := earliest.AddDate(-2, 0, 0)
	mortgageMaturity := mortgageStart.AddDate(15, 0, 0)
	mortgage, err := liabilities.CreateLiability(ctx, repo.CreateLiabilityParams{
		DisplayName:      "Home Mortgage",
		Subtype:          "institutional",
		OwnershipType:    "joint",
		NativeCurrency:   demoCurrency,
		CounterpartyName: "Bank Mandiri",
		Principal:        decimalPtr(185_000_000),
		InterestRate:     decimalPtr(8.5),
		TermMonths:       int32Ptr(180),
		StartDate:        &mortgageStart,
		MaturityDate:     &mortgageMaturity,
	})
	if err != nil {
		return fmt.Errorf("demo reset: seed liability (mortgage): %w", err)
	}
	// A realistic principal paydown (~0.4%/mo) — this is now a genuine monthly
	// cash outflow funded from checking, not a free net-worth boost, so it can't
	// run at the boosted rate an independent snapshot curve once used. Kept as a
	// smooth amortising curve (an institutional mortgage pays a fixed schedule),
	// unlike the sporadic personal loans/receivables around it.
	mortgageSeries := demoSeries(175_000_000, -0.004, 0, 17, len(months))
	if err := seedLiabilitySnapshots(ctx, liabilities, mortgage.ID, mortgageSeries, demoCurrency); err != nil {
		return err
	}
	recordSeriesDelta(ledger.liabDelta, mortgageSeries)
	if err := tagRepo.AssignTag(ctx, repo.TagGroupLiability, mortgage.ID, tagPtr(tags, "Big Ticket")); err != nil {
		return fmt.Errorf("demo reset: tag mortgage: %w", err)
	}

	friendDue := dayIn(months[len(months)-1], 20).AddDate(0, 3, 0)
	friendLoan, err := receivables.CreateReceivable(ctx, repo.CreateReceivableParams{
		DisplayName:      "Money Lent to a Friend",
		OwnershipType:    "sole",
		SoleOwnerUserID:  &ownerID,
		NativeCurrency:   demoCurrency,
		CounterpartyName: "Budi",
		DueDate:          &friendDue,
	})
	if err != nil {
		return fmt.Errorf("demo reset: seed receivable (friend): %w", err)
	}
	// The friend repays in sporadic IDR 50,000 instalments — flat most months,
	// a payment here and there. 10 payments net 500k off the 1M lent, leaving
	// 500k outstanding at the due date.
	friendLoanSeries := demoStepDownSeries(1_000_000, 50_000, []int{2, 5, 6, 10, 13, 14, 18, 21, 22, 25}, len(months))
	if err := seedReceivableSnapshots(ctx, receivables, friendLoan.ID, friendLoanSeries, demoCurrency); err != nil {
		return err
	}
	// Repayments trickle back into the bank — a small positive cash flow.
	recordSeriesDelta(ledger.recvDelta, friendLoanSeries)

	// USD-denominated — an overseas freelance client paying in USD, plausible
	// for the "sole" freelancer member. One of the "select few" USD positions
	// exercising multi-currency conversion.
	invoiceDue := dayIn(months[len(months)-1], 20).AddDate(0, 1, 0)
	invoice, err := receivables.CreateReceivable(ctx, repo.CreateReceivableParams{
		DisplayName:      "Freelance Invoice Pending",
		OwnershipType:    "sole",
		SoleOwnerUserID:  &member2ID,
		NativeCurrency:   "USD",
		CounterpartyName: "Client Co",
		DueDate:          &invoiceDue,
	})
	if err != nil {
		return fmt.Errorf("demo reset: seed receivable (invoice): %w", err)
	}
	// The client settles the invoice in sporadic USD 1 instalments — flat
	// between payments, not a smooth curve. Left out of the cash ledger like the
	// other USD positions (its IDR change carries FX revaluation that isn't cash).
	invoiceSeries := demoStepDownSeries(50, 1, []int{1, 3, 4, 8, 11, 12, 16, 19, 20, 24, 25}, len(months))
	if err := seedReceivableSnapshots(ctx, receivables, invoice.ID, invoiceSeries, "USD"); err != nil {
		return err
	}

	return nil
}

// seedDemoIncome seeds a routine monthly Salary across the full history plus
// a spread of incidental Income events across every other category (ADR-0008)
// so the income-statement view has real category variety. Amounts are scaled
// to match the modest ~400M IDR household seedDemoAssets targets, rather than
// the much larger salary a 2B-net-worth household would imply.
func seedDemoIncome(ctx context.Context, pool *pgxpool.Pool, ownerID, member2ID uuid.UUID, ledger *demoCashLedger) error {
	income := repo.NewIncomeRepo(pool)
	months := demoMonths()

	for i, ym := range months {
		desc := "Monthly salary"
		amountF := 5_000_000 + float64(i)*30_000
		amount := demoDecimal(amountF)
		if _, err := income.CreateIncome(ctx, repo.CreateIncomeParams{
			Date:            dayIn(ym, 25),
			Amount:          amount,
			Currency:        demoCurrency,
			Category:        "salary",
			Description:     &desc,
			OwnershipType:   "sole",
			SoleOwnerUserID: &ownerID,
			Regularity:      "routine",
		}); err != nil {
			return fmt.Errorf("demo reset: seed salary income: %w", err)
		}
		ledger.addIncome(i, amountF)
	}

	// A routine monthly Pension for the second member (ADR-0048) — a passive
	// *cash* stream, so the statistics panel's Passive-Income Ratio and Fund
	// Resilience read meaningfully rather than off a single incidental rental.
	pensionDesc := "Monthly pension"
	for i, ym := range months {
		if _, err := income.CreateIncome(ctx, repo.CreateIncomeParams{
			Date:            dayIn(ym, 3),
			Amount:          demoDecimal(2_500_000),
			Currency:        demoCurrency,
			Category:        "pension",
			Description:     &pensionDesc,
			OwnershipType:   "sole",
			SoleOwnerUserID: &member2ID,
			Regularity:      "routine",
		}); err != nil {
			return fmt.Errorf("demo reset: seed pension income: %w", err)
		}
		ledger.addIncome(i, 2_500_000)
	}

	// A monthly bank Interest for the household (ADR-0048 slice 5), incidental:
	// real-world current/savings rates are negligible, and the interest is
	// credited back into the account that earned it rather than paid out, so it
	// is not income the household relies on. Rent and pension carry the passive
	// scope in the demo; interest is here for the category breakdown and as the
	// bookkeeping counterpart to the balance it accrues into.
	interestDesc := "Savings account interest"
	for i, ym := range months {
		if _, err := income.CreateIncome(ctx, repo.CreateIncomeParams{
			Date:          dayIn(ym, 25),
			Amount:        demoDecimal(300_000),
			Currency:      demoCurrency,
			Category:      "interest",
			Description:   &interestDesc,
			OwnershipType: "joint",
			Regularity:    "incidental",
		}); err != nil {
			return fmt.Errorf("demo reset: seed interest income: %w", err)
		}
		ledger.addIncome(i, 300_000)
	}

	// monthIdx values land in the most recent 12 months (demoExtraHistoryMonths
	// offset) — incidental one-offs read oddly scattered across years of
	// backfilled history, so they're kept recent like the investment ledger.
	incidentals := []struct {
		monthIdx    int
		category    string
		description string
		amount      float64
		ownership   string
		soleID      *uuid.UUID
	}{
		{demoExtraHistoryMonths + 1, "business_income", "Side consulting project", 1_600_000, "sole", &ownerID},
		{demoExtraHistoryMonths + 3, "rental_income", "Spare room rental income", 700_000, "joint", nil},
		{demoExtraHistoryMonths + 5, "gift", "Wedding gift from relatives", 1_000_000, "joint", nil},
		{demoExtraHistoryMonths + 7, "tax_refund", "Annual tax refund", 450_000, "sole", &member2ID},
		{demoExtraHistoryMonths + 9, "insurance_payout", "Health insurance reimbursement", 900_000, "sole", &ownerID},
		{demoExtraHistoryMonths + 10, "other", "Marketplace resale", 250_000, "sole", &member2ID},
	}
	for _, inc := range incidentals {
		desc := inc.description
		if _, err := income.CreateIncome(ctx, repo.CreateIncomeParams{
			Date:            dayIn(months[inc.monthIdx], 15),
			Amount:          demoDecimal(inc.amount),
			Currency:        demoCurrency,
			Category:        inc.category,
			Description:     &desc,
			OwnershipType:   inc.ownership,
			SoleOwnerUserID: inc.soleID,
			Regularity:      "incidental",
		}); err != nil {
			return fmt.Errorf("demo reset: seed incidental income (%s): %w", inc.category, err)
		}
		ledger.addIncome(inc.monthIdx, inc.amount)
	}

	return nil
}

func tagPtr(tags map[string]uuid.UUID, name string) *uuid.UUID {
	id := tags[name]
	return &id
}

func strPtr(s string) *string { return &s }

func int32Ptr(i int32) *int32 { return &i }

func decimalPtr(f float64) *decimal.Decimal {
	d := demoDecimal(f)
	return &d
}
