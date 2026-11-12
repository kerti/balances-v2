package backup

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/kerti/balances-v2/backend/internal/repo"
)

// demoCurrency is the native/reporting currency of almost every seeded Position
// and Income event — the "IDR" the demo Household is created with
// (resetDemoHousehold). A few Positions are USD (multi-currency is switched on
// there), converted through the USD rate table.
const demoCurrency = "IDR"

// demoMonthCount is the length of the seeded history: six years, so the
// dashboard's 1Y / 3Y / 5Y / All range chips all show (a chip shows only when
// the history is longer than the chip) and its year-over-year figure always has
// a month a year back to compare with.
const demoMonthCount = 72

// demoTimeZone is the demo users' time zone. The report engine buckets "now"
// into the user's local month, and the 19:00 UTC reset cron is already the next
// month here on every month end — so the seeder must bucket in the same zone,
// or the demo's current month comes out empty.
const demoTimeZone = "Asia/Jakarta"

// demoMonthsAt returns demoMonthCount consecutive calendar months ending at the
// month containing now (in demoTimeZone), oldest first, so every reset lands
// its snapshot trail in a fixed window relative to "now" regardless of when the
// cron actually runs.
func demoMonthsAt(now time.Time) []time.Time {
	loc, err := time.LoadLocation(demoTimeZone)
	if err != nil {
		loc = time.UTC // mirrors the report engine's fallback, keeping the two in step
	}
	now = now.In(loc)
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

// demoSlot maps window month i onto the year-row calendar the tables use: which
// row (oldest first, so the current year is the last) and which calendar month.
// The window ends in the current month, so the current year is the year of its
// last month and a month's row is how many years that is before it. This is the
// whole trick that keeps seasonality on real calendar months while the window
// rolls forward nightly: a table cell is addressed by "N years ago, in March",
// never by "k months from the start".
func demoSlot(months []time.Time, i int) (row, mon int) {
	yearsBack := months[len(months)-1].Year() - months[i].Year()
	return demoYearRows - 1 - yearsBack, int(months[i].Month()) - 1
}

func (g *demoGrid) at(months []time.Time, i int) int64 {
	row, mon := demoSlot(months, i)
	return g[row][mon]
}

// money reads the cell for window month i as an amount in the given currency:
// whole units for IDR, cents for USD (the tables' two unit conventions).
func (g *demoGrid) money(months []time.Time, i int, currency string) decimal.Decimal {
	if currency == "USD" {
		return decimal.New(g.at(months, i), -2)
	}
	return decimal.NewFromInt(g.at(months, i))
}

// demoRecorded is how many of the window's months a position has a snapshot
// for: all of them, less the trailing months demoNotUpdated says it is behind by.
func demoRecorded(months []time.Time, name string) int {
	return len(months) - demoNotUpdated[name]
}

// demoPlace says where an event sits relative to the seeded window.
type demoPlace int

const (
	demoBeforeWindow demoPlace = iota
	demoInWindow
	demoAfterNow // a future cell of the current year's row: not seeded yet
)

// locate finds w in the window: its month index when inside, otherwise whether
// it falls before the first month or after the current one.
func (w demoWhen) locate(months []time.Time) (int, demoPlace) {
	target := time.Date(months[len(months)-1].Year()-w.yearsBack, w.month, 1, 0, 0, 0, 0, time.UTC)
	switch {
	case target.Before(months[0]):
		return 0, demoBeforeWindow
	case target.After(months[len(months)-1]):
		return 0, demoAfterNow
	}
	first := months[0]
	return (target.Year()-first.Year())*12 + int(target.Month()) - int(first.Month()), demoInWindow
}

// date is w anchored on a day of the month, for fields that want a calendar
// date. Unlike locate it does not care where the window is, so it also serves
// dates before the window (a loan's start) and after it (a maturity).
func (w demoWhen) date(months []time.Time, day int) time.Time {
	return time.Date(months[len(months)-1].Year()-w.yearsBack, w.month, day, 0, 0, 0, 0, time.UTC)
}

// demoDeal is one Buy (qty > 0) or Sell (qty < 0) to record at window month idx.
type demoDeal struct {
	idx int
	qty int64
}

// demoHoldings resolves a trade list against the window. held[i] is the net
// quantity at the end of window month i — the snapshot quantity — and deals are
// the Buy/Sell rows to record. A position is already held when the window opens,
// so every trade before the window (and any in its first month) folds into one
// opening lot at month 0, which books no return because the first month is the
// baseline (INV-FINANCE-23); a trade after "now" is not seeded yet. Because the
// snapshot quantity is the running sum of the very deals recorded, the snapshot
// always equals the ledger's net Buy/Sell quantity and the engine's per-month
// return is pure price movement (INV-FINANCE-08, #497).
func demoHoldings(months []time.Time, trades []demoTrade) (held []int64, deals []demoDeal) {
	net := make([]int64, len(months))
	for _, t := range trades {
		idx, place := t.when.locate(months)
		switch place {
		case demoAfterNow:
			continue
		case demoBeforeWindow:
			idx = 0
		}
		net[idx] += t.qty
	}
	held = make([]int64, len(months))
	var run int64
	for i, q := range net {
		run += q
		held[i] = run
		if q != 0 {
			deals = append(deals, demoDeal{idx: i, qty: q})
		}
	}
	return held, deals
}

// seedDemoData wipes-then-rebuilds the shared demo Household's entire toy
// dataset (ADR-0041, #217): at least two Positions per Asset/Investment
// subtype plus Liabilities, Receivables, Income, and Tags, each carrying a
// six-year hand-written history (demo_seed_data.go) — enough spread that a
// first-time visitor can see every position type, tag breakdown, income
// category, a year-over-year comparison and every chart range in action rather
// than an empty-feeling dashboard.
func seedDemoData(ctx context.Context, pool *pgxpool.Pool, ownerID, member2ID uuid.UUID) error {
	return seedDemoDataAt(ctx, pool, ownerID, member2ID, time.Now())
}

// seedDemoDataAt is seedDemoData with "now" injected, so tests can seed the
// window at fixed dates (a January and a December exercise both ends of the
// table calendar).
func seedDemoDataAt(ctx context.Context, pool *pgxpool.Pool, ownerID, member2ID uuid.UUID, now time.Time) error {
	months := demoMonthsAt(now)
	tags, err := seedDemoTags(ctx, pool)
	if err != nil {
		return err
	}
	if err := seedDemoFxRates(ctx, pool, months); err != nil {
		return err
	}
	if err := seedDemoAssets(ctx, pool, months, ownerID, member2ID, tags); err != nil {
		return err
	}
	if err := seedDemoInvestments(ctx, pool, months, ownerID, member2ID, tags); err != nil {
		return err
	}
	if err := seedDemoLiabilitiesAndReceivables(ctx, pool, months, ownerID, member2ID, tags); err != nil {
		return err
	}
	if err := seedDemoIncome(ctx, pool, months, ownerID, member2ID); err != nil {
		return err
	}
	return seedDemoInflation(ctx, pool, months)
}

// seedDemoInflation seeds the monthly annualised CPI figures (ADR-0048) the
// statistics panel's Fund Resilience averages over the trailing 12 months, so
// it runs off stored figures rather than only the assumed_annual_inflation
// fallback. Only the last three years are entered, leaving the older months
// empty to exercise the fallback too.
func seedDemoInflation(ctx context.Context, pool *pgxpool.Pool, months []time.Time) error {
	rates := repo.NewInflationRateRepo(pool)
	for i, ym := range months {
		hundredths := demoInflation.at(months, i)
		if hundredths == 0 {
			continue
		}
		if _, err := rates.CreateInflationRate(ctx, repo.CreateInflationRateParams{
			YearMonth: ym,
			Rate:      decimal.New(hundredths, -2),
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
// Positions into the IDR-reporting nw_total — one entered rate per month, so the
// Settings "Exchange rates" card has a real history to show.
func seedDemoFxRates(ctx context.Context, pool *pgxpool.Pool, months []time.Time) error {
	fxRates := repo.NewFxRateRepo(pool)
	for i, ym := range months {
		if _, err := fxRates.CreateFxRate(ctx, repo.CreateFxRateParams{
			YearMonth: ym,
			Currency:  "USD",
			Rate:      demoUSDRate.money(months, i, demoCurrency),
		}); err != nil {
			return fmt.Errorf("demo reset: seed USD fx rate: %w", err)
		}
	}
	return nil
}

// seedAssetSnapshots writes one amount-only snapshot (ADR-0022) per window
// month from a table. currency must match the Position's own native_currency —
// a snapshot tagged with the wrong currency either skips FX conversion entirely
// (if it happens to equal the reporting currency) or converts at the wrong rate.
func seedAssetSnapshots(ctx context.Context, assets *repo.AssetRepo, assetID uuid.UUID, months []time.Time, g *demoGrid, currency string, recorded int) error {
	for i, ym := range months[:recorded] {
		if _, err := assets.CreateAssetSnapshot(ctx, repo.CreateAssetSnapshotParams{
			AssetID:   assetID,
			YearMonth: ym,
			Amount:    g.money(months, i, currency),
			Currency:  currency,
		}); err != nil {
			return fmt.Errorf("demo reset: seed asset snapshot: %w", err)
		}
	}
	return nil
}

// seedMarketPosition seeds a Stock / MutualFund / Gold Position's full trail —
// snapshots and the Buy/Sell ledger that backs them — from its trade schedule
// (demoHoldings), so the snapshot quantity each month equals the ledger's net
// quantity and their values can never drift apart (#497). A Buy or Sell is
// priced at the month's snapshot price, so the recorded trade matches what the
// chart shows. Months before the first holding get no snapshot: a position
// bought mid-history simply starts there. Dividends, distributions and fees
// follow from the position's cash events.
func seedMarketPosition(ctx context.Context, investments *repo.InvestmentRepo, investmentID uuid.UUID, months []time.Time, p *demoMarket, recorded int) error {
	held, deals := demoHoldings(months, p.trades)
	for i, ym := range months[:recorded] {
		if held[i] == 0 {
			continue
		}
		price := p.prices.money(months, i, demoCurrency)
		q := decimal.NewFromInt(held[i])
		if _, err := investments.CreateInvestmentSnapshot(ctx, repo.CreateInvestmentSnapshotParams{
			InvestmentID: investmentID,
			YearMonth:    ym,
			Amount:       q.Mul(price),
			Currency:     demoCurrency,
			Quantity:     &q,
			PricePerUnit: &price,
		}); err != nil {
			return fmt.Errorf("demo reset: seed market snapshot: %w", err)
		}
	}
	for _, d := range deals {
		txnType, qty := repo.TxnTypeBuy, decimal.NewFromInt(d.qty)
		if d.qty < 0 {
			txnType, qty = repo.TxnTypeSell, qty.Neg()
		}
		price := p.prices.money(months, d.idx, demoCurrency)
		amount := qty.Mul(price)
		if _, err := investments.CreateInvestmentTransaction(ctx, repo.CreateInvestmentTransactionParams{
			InvestmentID:    investmentID,
			TransactionType: txnType,
			TransactionDate: dayIn(months[d.idx], 5),
			Currency:        demoCurrency,
			Amount:          &amount,
			Quantity:        &qty,
			PricePerUnit:    &price,
		}); err != nil {
			return fmt.Errorf("demo reset: seed %s trade: %w", txnType, err)
		}
	}
	for _, ev := range p.cash {
		idx, place := ev.when.locate(months)
		if place != demoInWindow {
			continue
		}
		if err := seedInvestmentCashEvent(ctx, investments, investmentID, ev.txn, dayIn(months[idx], ev.day), ev.amount); err != nil {
			return err
		}
	}
	return nil
}

// seedClosedMarketPosition seeds a stock that was later sold in full: its trail
// up to the month before the sale, then the lifecycle flip to "sold" with the
// whole remaining holding settled at that month's price — the Terminate dialog's
// path, which writes the terminal Sell and the zero close snapshot in one
// transaction (ADR-0052 §1, §6).
func seedClosedMarketPosition(ctx context.Context, investments *repo.InvestmentRepo, investmentID uuid.UUID, months []time.Time, p *demoMarket) error {
	closeIdx, place := p.closing.when.locate(months)
	if place != demoInWindow {
		return fmt.Errorf("demo reset: a closing is outside the history window")
	}
	if err := seedMarketPosition(ctx, investments, investmentID, months, p, closeIdx); err != nil {
		return err
	}
	held, _ := demoHoldings(months, p.trades)
	qty := decimal.NewFromInt(held[closeIdx])
	price := p.prices.money(months, closeIdx, demoCurrency)
	soldAt := p.closing.when.date(months, p.closing.day)
	if _, err := investments.UpdateInvestmentLifecycle(ctx, investmentID, repo.LifecycleParams{
		Status:          repo.StatusSold,
		TerminatedAt:    &soldAt,
		TerminationNote: &p.closing.note,
	}, &repo.InvestmentSettlement{Quantity: &qty, PricePerUnit: &price}); err != nil {
		return fmt.Errorf("demo reset: sell the closed position: %w", err)
	}
	return nil
}

// seedMaturedDeposit creates a time deposit placed on d.placed for d.termMonths
// and records it up to the month before its maturity date (the table is zero
// from then on). It returns the deposit and its maturity date for the caller to
// settle — rolled over or paid out.
func seedMaturedDeposit(ctx context.Context, investments *repo.InvestmentRepo, months []time.Time, d *demoMaturedDeposit, params repo.CreateTimeDepositParams) (*repo.TimeDeposit, time.Time, error) {
	placement := d.placed.date(months, 12)
	maturity := placement.AddDate(0, d.termMonths, 0)
	params.Principal = decimal.NewFromInt(d.principal)
	params.TermMonths = int32(d.termMonths)
	params.PlacementDate = placement
	params.MaturityDate = maturity
	td, err := investments.CreateTimeDeposit(ctx, params)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("demo reset: seed time deposit (%s): %w", params.DisplayName, err)
	}
	if err := seedInterestBearingSnapshots(ctx, investments, td.Investment.ID, months, &d.value, nil, params.Principal, demoCurrency); err != nil {
		return nil, time.Time{}, err
	}
	return td, maturity, nil
}

// seedInvestmentCashEvent records an amount-only ledger entry — Dividend,
// Distribution, Coupon, or Fee — none of which carry Quantity/PricePerUnit
// (ADR-0009's transaction-shape rules).
func seedInvestmentCashEvent(ctx context.Context, investments *repo.InvestmentRepo, investmentID uuid.UUID, txnType string, date time.Time, amount int64) error {
	amt := decimal.NewFromInt(amount)
	if _, err := investments.CreateInvestmentTransaction(ctx, repo.CreateInvestmentTransactionParams{
		InvestmentID:    investmentID,
		TransactionType: txnType,
		TransactionDate: date,
		Currency:        demoCurrency,
		Amount:          &amt,
	}); err != nil {
		return fmt.Errorf("demo reset: seed %s: %w", txnType, err)
	}
	return nil
}

// seedCouponCash records one Coupon per non-zero month of a coupon table.
func seedCouponCash(ctx context.Context, investments *repo.InvestmentRepo, investmentID uuid.UUID, months []time.Time, coupons *demoGrid, day int) error {
	for i, ym := range months {
		if c := coupons.at(months, i); c > 0 {
			if err := seedInvestmentCashEvent(ctx, investments, investmentID, repo.TxnTypeCoupon, dayIn(ym, day), c); err != nil {
				return err
			}
		}
	}
	return nil
}

// seedInterestBearingSnapshots writes a total-value+accrued trail (the "dirty
// total value" shape, ADR-0022) for a Bond or TimeDeposit: one snapshot per
// month in which total is non-zero (zero = not placed yet), with accrued read
// from the accrued table, or derived as total − principal when accrued is nil
// (a time deposit's whole gain over its principal, INV-SNAPSHOTS-17). currency
// must match the Position's own native_currency (see seedAssetSnapshots).
func seedInterestBearingSnapshots(ctx context.Context, investments *repo.InvestmentRepo, investmentID uuid.UUID, months []time.Time, total, accruedTable *demoGrid, principal decimal.Decimal, currency string) error {
	for i, ym := range months {
		if total.at(months, i) == 0 {
			continue
		}
		amount := total.money(months, i, currency)
		accrued := amount.Sub(principal)
		if accruedTable != nil {
			accrued = accruedTable.money(months, i, currency)
		}
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

// seedLiabilitySnapshots backfills the full window's balance trail for a
// Liability or Receivable from a table. currency must match the Position's own
// native_currency (see seedAssetSnapshots).
func seedLiabilitySnapshots(ctx context.Context, liabilities *repo.LiabilityRepo, liabilityID uuid.UUID, months []time.Time, g *demoGrid, currency string) error {
	for i, ym := range months {
		if _, err := liabilities.CreateLiabilitySnapshot(ctx, repo.CreateLiabilitySnapshotParams{
			LiabilityID: liabilityID,
			YearMonth:   ym,
			Amount:      g.money(months, i, currency),
			Currency:    currency,
		}); err != nil {
			return fmt.Errorf("demo reset: seed liability snapshot: %w", err)
		}
	}
	return nil
}

func seedReceivableSnapshots(ctx context.Context, receivables *repo.ReceivableRepo, receivableID uuid.UUID, months []time.Time, g *demoGrid, currency string, recorded int) error {
	for i, ym := range months[:recorded] {
		if _, err := receivables.CreateReceivableSnapshot(ctx, repo.CreateReceivableSnapshotParams{
			ReceivableID: receivableID,
			YearMonth:    ym,
			Amount:       g.money(months, i, currency),
			Currency:     currency,
		}); err != nil {
			return fmt.Errorf("demo reset: seed receivable snapshot: %w", err)
		}
	}
	return nil
}

// seedDemoAssets seeds two Positions each of BankAccount, Property, and
// Vehicle — the three Asset subtypes (ADR-0022) — plus a USD account, each with
// the full snapshot trail from its table and a mix of sole/joint ownership and
// tag assignment. Property values step at an appraisal (the Family Home each
// January, the Rental Apartment each July); vehicles depreciate a little each
// month. Property/vehicle purchases are not seeded: both exist when the window
// opens, so no month's living expenses has to absorb an acquisition.
func seedDemoAssets(ctx context.Context, pool *pgxpool.Pool, months []time.Time, ownerID, member2ID uuid.UUID, tags map[string]uuid.UUID) error {
	assets := repo.NewAssetRepo(pool)
	tagRepo := repo.NewTagRepo(pool)
	nowYear := months[len(months)-1].Year()
	acquired := at(12, time.March).date(months, 1)

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
		return fmt.Errorf("demo reset: seed bank account (checking): %w", err)
	}
	if err := seedAssetSnapshots(ctx, assets, checking.Asset.ID, months, &demoChecking, demoCurrency, len(months)); err != nil {
		return err
	}

	savings, err := assets.CreateBankAccount(ctx, repo.CreateBankAccountParams{
		DisplayName:    "Joint Savings",
		OwnershipType:  "joint",
		NativeCurrency: demoCurrency,
		BankName:       "Demo Bank",
		AccountNumber:  "9876543210",
		AccountType:    "savings",
	})
	if err != nil {
		return fmt.Errorf("demo reset: seed bank account (savings): %w", err)
	}
	if err := seedAssetSnapshots(ctx, assets, savings.Asset.ID, months, &demoJointSavings, demoCurrency, len(months)); err != nil {
		return err
	}
	if err := tagRepo.AssignTag(ctx, repo.TagGroupAsset, savings.Asset.ID, tagPtr(tags, "Emergency Fund")); err != nil {
		return fmt.Errorf("demo reset: tag joint savings: %w", err)
	}

	familyHome, err := assets.CreateProperty(ctx, repo.CreatePropertyParams{
		DisplayName:            "Family Home",
		OwnershipType:          "joint",
		NativeCurrency:         demoCurrency,
		PropertyType:           "house",
		Address:                strPtr("Jl. Demo Raya No. 1"),
		AcquisitionDate:        &acquired,
		AcquisitionCost:        decimalPtr("150000000"),
		AnnualAppreciationRate: decimalPtr("4"), // 4%/yr, matching the appraisals' long-run drift
	})
	if err != nil {
		return fmt.Errorf("demo reset: seed property (family home): %w", err)
	}
	if err := seedAssetSnapshots(ctx, assets, familyHome.Asset.ID, months, &demoFamilyHome, demoCurrency, len(months)); err != nil {
		return err
	}
	if err := tagRepo.AssignTag(ctx, repo.TagGroupAsset, familyHome.Asset.ID, tagPtr(tags, "Big Ticket")); err != nil {
		return fmt.Errorf("demo reset: tag family home: %w", err)
	}

	rental, err := assets.CreateProperty(ctx, repo.CreatePropertyParams{
		DisplayName:            "Rental Apartment",
		OwnershipType:          "sole",
		SoleOwnerUserID:        &ownerID,
		NativeCurrency:         demoCurrency,
		PropertyType:           "apartment",
		Address:                strPtr("Jl. Demo Kedua No. 12"),
		AcquisitionDate:        &acquired,
		AcquisitionCost:        decimalPtr("100000000"),
		AnnualAppreciationRate: decimalPtr("3.5"),
	})
	if err != nil {
		return fmt.Errorf("demo reset: seed property (rental): %w", err)
	}
	if err := seedAssetSnapshots(ctx, assets, rental.Asset.ID, months, &demoRentalApartment, demoCurrency, len(months)); err != nil {
		return err
	}

	car, err := assets.CreateVehicle(ctx, repo.CreateVehicleParams{
		DisplayName:            "Family Car",
		OwnershipType:          "joint",
		NativeCurrency:         demoCurrency,
		VehicleType:            "car",
		Make:                   strPtr("Toyota"),
		Model:                  strPtr("Demo Sedan"),
		Year:                   int32Ptr(int32(nowYear - 10)), // already 4 years old when the history opens
		PlateNumber:            strPtr("B 1234 DEM"),
		AnnualDepreciationRate: decimalPtr("8"), // 8%/yr — stored unsigned; the trail declines
	})
	if err != nil {
		return fmt.Errorf("demo reset: seed vehicle (car): %w", err)
	}
	if err := seedAssetSnapshots(ctx, assets, car.Asset.ID, months, &demoFamilyCar, demoCurrency, demoRecorded(months, "Family Car")); err != nil {
		return err
	}
	if err := tagRepo.AssignTag(ctx, repo.TagGroupAsset, car.Asset.ID, tagPtr(tags, "Big Ticket")); err != nil {
		return fmt.Errorf("demo reset: tag family car: %w", err)
	}

	bike, err := assets.CreateVehicle(ctx, repo.CreateVehicleParams{
		DisplayName:            "Motorbike",
		OwnershipType:          "sole",
		SoleOwnerUserID:        &member2ID,
		NativeCurrency:         demoCurrency,
		VehicleType:            "motorcycle",
		Make:                   strPtr("Demo Moto"),
		Model:                  strPtr("Scoot 125"),
		Year:                   int32Ptr(int32(nowYear - 7)),
		PlateNumber:            strPtr("B 5678 DEM"),
		AnnualDepreciationRate: decimalPtr("10"), // 10%/yr — bikes shed value faster than cars
	})
	if err != nil {
		return fmt.Errorf("demo reset: seed vehicle (bike): %w", err)
	}
	if err := seedAssetSnapshots(ctx, assets, bike.Asset.ID, months, &demoMotorbike, demoCurrency, demoRecorded(months, "Motorbike")); err != nil {
		return err
	}

	// Old Hatchback: sold "3 years ago". It is recorded up to the month before
	// the sale and then closed through the lifecycle flip the Terminate dialog
	// makes (ADR-0052), which writes the zero close snapshot in the sale month; the
	// proceeds are not a transaction, they simply land in the bank.
	hatch, err := assets.CreateVehicle(ctx, repo.CreateVehicleParams{
		DisplayName:            "Old Hatchback",
		OwnershipType:          "sole",
		SoleOwnerUserID:        &ownerID,
		NativeCurrency:         demoCurrency,
		VehicleType:            "car",
		Make:                   strPtr("Demo Motors"),
		Model:                  strPtr("City Hatch"),
		Year:                   int32Ptr(int32(nowYear - 12)),
		PlateNumber:            strPtr("B 9012 DEM"),
		AnnualDepreciationRate: decimalPtr("9"),
	})
	if err != nil {
		return fmt.Errorf("demo reset: seed vehicle (old hatchback): %w", err)
	}
	soldIdx, place := demoOldHatchback.closing.when.locate(months)
	if place != demoInWindow {
		return fmt.Errorf("demo reset: the Old Hatchback sale is outside the history window")
	}
	if err := seedAssetSnapshots(ctx, assets, hatch.Asset.ID, months, &demoOldHatchback.value, demoCurrency, soldIdx); err != nil {
		return err
	}
	soldAt := demoOldHatchback.closing.when.date(months, demoOldHatchback.closing.day)
	if _, err := assets.UpdateAssetLifecycle(ctx, hatch.Asset.ID, repo.LifecycleParams{
		Status:          "sold",
		TerminatedAt:    &soldAt,
		TerminationNote: &demoOldHatchback.closing.note,
	}); err != nil {
		return fmt.Errorf("demo reset: sell the old hatchback: %w", err)
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
		return fmt.Errorf("demo reset: seed bank account (USD savings): %w", err)
	}
	return seedAssetSnapshots(ctx, assets, usdSavings.Asset.ID, months, &demoUSDSavings, "USD", len(months))
}

// seedDemoInvestments seeds two Positions each of the five Investment
// subtypes (Stock, MutualFund, Bond, Gold, TimeDeposit — ADR-0022). The
// market positions (stocks, funds, gold) and the first of each other kind exist
// when the window opens and accumulate as later Buys land; a few were bought
// mid-history (Digital Gold, ORI024, the Astra Sedaya bond and the two time
// deposits) and start there. Stock/MutualFund/Gold prices sit at realistic
// market levels (rescaling a real per-share/per-gram price would look wrong to
// anyone who knows it) — the modest-household rescale shrinks the quantity held.
func seedDemoInvestments(ctx context.Context, pool *pgxpool.Pool, months []time.Time, ownerID, member2ID uuid.UUID, tags map[string]uuid.UUID) error {
	investments := repo.NewInvestmentRepo(pool)
	tagRepo := repo.NewTagRepo(pool)

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
	if err := seedMarketPosition(ctx, investments, stock1.Investment.ID, months, &demoBBCA, len(months)); err != nil {
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
	if err := seedMarketPosition(ctx, investments, stock2.Investment.ID, months, &demoASII, len(months)); err != nil {
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
	if err := seedMarketPosition(ctx, investments, mf1.Investment.ID, months, &demoEquityGrowthFund, len(months)); err != nil {
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
	if err := seedMarketPosition(ctx, investments, mf2.Investment.ID, months, &demoMoneyMarketFund, len(months)); err != nil {
		return err
	}

	// ORI024: a government-primary bond. CreateBond seeds the placement Buy
	// itself (govt_primary + face value + placement date); the coupon pays out
	// to the bank each month, so the instrument's accrued component is zero.
	oriPlacement := demoORI.placed.date(months, 10)
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
		MaturityDate:      demoORI.matures.date(months, 10),
		FaceValue:         decimalPtr("10000000"),
		PlacementDate:     &oriPlacement,
	})
	if err != nil {
		return fmt.Errorf("demo reset: seed bond (ORI024): %w", err)
	}
	oriIdx, place := demoORI.placed.locate(months)
	if place != demoInWindow {
		return fmt.Errorf("demo reset: ORI024 placement is outside the history window")
	}
	var oriValue, noAccrued demoGrid // par, no accrued — from the placement month on
	for i := oriIdx; i < len(months); i++ {
		row, mon := demoSlot(months, i)
		oriValue[row][mon] = demoORIFace
	}
	if err := seedInterestBearingSnapshots(ctx, investments, bond1.Investment.ID, months, &oriValue, &noAccrued, decimal.Zero, demoCurrency); err != nil {
		return err
	}
	if err := seedCouponCash(ctx, investments, bond1.Investment.ID, months, &demoORI.coupons, 10); err != nil {
		return err
	}

	// Astra Sedaya: a secondary-market corporate bond (not auto-seeded a
	// placement Buy) bought at a slight premium, with a brokerage Fee and a
	// semi-annual coupon; accrued builds between coupons and resets at each.
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
		MaturityDate:      at(-2, time.June).date(months, 1),
	})
	if err != nil {
		return fmt.Errorf("demo reset: seed bond (Astra Sedaya): %w", err)
	}
	if err := seedInterestBearingSnapshots(ctx, investments, bond2.Investment.ID, months, &demoAstraBond.value, &demoAstraBond.accrued, decimal.Zero, demoCurrency); err != nil {
		return err
	}
	bond2Qty := decimal.RequireFromString("8")
	bond2Price := decimal.RequireFromString("1010000")
	bond2Amount := bond2Qty.Mul(bond2Price)
	if _, err := investments.CreateInvestmentTransaction(ctx, repo.CreateInvestmentTransactionParams{
		InvestmentID:    bond2.Investment.ID,
		TransactionType: repo.TxnTypeBuy,
		TransactionDate: demoAstraBond.placed.date(months, 8),
		Currency:        demoCurrency,
		Amount:          &bond2Amount,
		Quantity:        &bond2Qty,
		PricePerUnit:    &bond2Price,
	}); err != nil {
		return fmt.Errorf("demo reset: seed bond (Astra Sedaya) placement buy: %w", err)
	}
	if err := seedInvestmentCashEvent(ctx, investments, bond2.Investment.ID, repo.TxnTypeFee, demoAstraBond.placed.date(months, 9), 30_000); err != nil {
		return err
	}
	if err := seedCouponCash(ctx, investments, bond2.Investment.ID, months, &demoAstraBond.coupons, 20); err != nil {
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
	if err := seedMarketPosition(ctx, investments, gold1.Investment.ID, months, &demoGoldBar, len(months)); err != nil {
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
	if err := seedMarketPosition(ctx, investments, gold2.Investment.ID, months, &demoDigitalGold, demoRecorded(months, "Digital Gold Savings")); err != nil {
		return err
	}

	// TLKM: a stock held from before the history opens and sold in full "2
	// years ago". seedClosedMarketPosition records it up to the month before and
	// then closes it through the lifecycle flip, which writes the terminal Sell and
	// the zero close snapshot together.
	stock3, err := investments.CreateStock(ctx, repo.CreateStockParams{
		DisplayName:     "Telkom Indonesia",
		OwnershipType:   "sole",
		SoleOwnerUserID: &member2ID,
		NativeCurrency:  demoCurrency,
		RiskProfile:     "medium",
		Ticker:          "TLKM",
		Exchange:        "IDX",
	})
	if err != nil {
		return fmt.Errorf("demo reset: seed stock (TLKM): %w", err)
	}
	if err := seedClosedMarketPosition(ctx, investments, stock3.Investment.ID, months, &demoTLKM); err != nil {
		return err
	}

	// BNI: a 36-month deposit that matured "2 years ago" and rolled over — the
	// maturity dialog's path: a Maturity transaction with both legs rolled_to_new
	// (which flips the deposit to matured and writes its close snapshot), then a
	// successor deposit holding principal + interest, linked back to it.
	bni, bniMaturity, err := seedMaturedDeposit(ctx, investments, months, &demoBNIDeposit, repo.CreateTimeDepositParams{
		DisplayName:    "BNI Time Deposit",
		OwnershipType:  "joint",
		NativeCurrency: demoCurrency,
		RiskProfile:    "low",
		BankName:       "Bank BNI",
		InterestRate:   decimal.RequireFromString("5"),
		RolloverPolicy: "auto_renew_with_interest",
	})
	if err != nil {
		return err
	}
	rolledPrincipal := decimal.NewFromInt(demoBNIDeposit.principal)
	rolledInterest := decimal.NewFromInt(demoBNIDeposit.interest)
	rolled := repo.DispositionRolledToNew
	if _, err := investments.CreateInvestmentTransaction(ctx, repo.CreateInvestmentTransactionParams{
		InvestmentID:         bni.Investment.ID,
		TransactionType:      repo.TxnTypeMaturity,
		TransactionDate:      bniMaturity,
		Currency:             demoCurrency,
		PrincipalAmount:      &rolledPrincipal,
		InterestAmount:       &rolledInterest,
		PrincipalDisposition: &rolled,
		InterestDisposition:  &rolled,
	}); err != nil {
		return fmt.Errorf("demo reset: record BNI maturity: %w", err)
	}
	renewedPrincipal := rolledPrincipal.Add(rolledInterest)
	renewed, err := investments.CreateTimeDeposit(ctx, repo.CreateTimeDepositParams{
		DisplayName:            "BNI Time Deposit (renewed)",
		OwnershipType:          "joint",
		NativeCurrency:         demoCurrency,
		RiskProfile:            "low",
		BankName:               "Bank BNI",
		Principal:              renewedPrincipal,
		InterestRate:           decimal.RequireFromString("5"),
		TermMonths:             int32(demoBNIRenewal.termMonths),
		PlacementDate:          bniMaturity,
		MaturityDate:           bniMaturity.AddDate(0, demoBNIRenewal.termMonths, 0),
		RolloverPolicy:         "auto_renew_with_interest",
		RolledFromInvestmentID: &bni.Investment.ID,
	})
	if err != nil {
		return fmt.Errorf("demo reset: seed BNI renewal: %w", err)
	}
	if err := seedInterestBearingSnapshots(ctx, investments, renewed.Investment.ID, months, &demoBNIRenewal.value, nil, renewedPrincipal, demoCurrency); err != nil {
		return err
	}

	// Danamon: a 12-month deposit that matured "3 years ago" and was paid out —
	// the Terminate dialog's path: the lifecycle flip to matured with a settlement,
	// which writes the Maturity transaction (both legs cash_out) and the close
	// snapshot in one transaction.
	danamon, danamonMaturity, err := seedMaturedDeposit(ctx, investments, months, &demoDanamonDeposit, repo.CreateTimeDepositParams{
		DisplayName:     "Danamon Time Deposit",
		OwnershipType:   "sole",
		SoleOwnerUserID: &member2ID,
		NativeCurrency:  demoCurrency,
		RiskProfile:     "low",
		BankName:        "Bank Danamon",
		InterestRate:    decimal.RequireFromString("5.25"),
		RolloverPolicy:  "no_rollover",
	})
	if err != nil {
		return err
	}
	paidPrincipal := decimal.NewFromInt(demoDanamonDeposit.principal)
	paidInterest := decimal.NewFromInt(demoDanamonDeposit.interest)
	paidNote := "Paid out to the bank at maturity"
	if _, err := investments.UpdateInvestmentLifecycle(ctx, danamon.Investment.ID, repo.LifecycleParams{
		Status:          repo.StatusMatured,
		TerminatedAt:    &danamonMaturity,
		TerminationNote: &paidNote,
	}, &repo.InvestmentSettlement{PrincipalAmount: &paidPrincipal, InterestAmount: &paidInterest}); err != nil {
		return fmt.Errorf("demo reset: settle Danamon maturity: %w", err)
	}

	// Mandiri: placed mid-history for a 36-month term that is still running,
	// since a time deposit's snapshots must fall inside its term.
	td1Placement := demoMandiriTD.placed.date(months, 12)
	td1Principal := decimal.RequireFromString("20000000")
	td1, err := investments.CreateTimeDeposit(ctx, repo.CreateTimeDepositParams{
		DisplayName:     "Mandiri Time Deposit",
		OwnershipType:   "sole",
		SoleOwnerUserID: &ownerID,
		NativeCurrency:  demoCurrency,
		RiskProfile:     "low",
		BankName:        "Bank Mandiri",
		Principal:       td1Principal,
		InterestRate:    decimal.RequireFromString("5.5"),
		TermMonths:      int32(demoMandiriTD.termMonths),
		PlacementDate:   td1Placement,
		MaturityDate:    td1Placement.AddDate(0, demoMandiriTD.termMonths, 0),
		RolloverPolicy:  "auto_renew_with_interest",
	})
	if err != nil {
		return fmt.Errorf("demo reset: seed time deposit (Mandiri): %w", err)
	}
	if err := seedInterestBearingSnapshots(ctx, investments, td1.Investment.ID, months, &demoMandiriTD.value, nil, td1Principal, demoCurrency); err != nil {
		return err
	}
	if err := tagRepo.AssignTag(ctx, repo.TagGroupInvestment, td1.Investment.ID, tagPtr(tags, "Education")); err != nil {
		return fmt.Errorf("demo reset: tag Mandiri time deposit: %w", err)
	}

	// USD-denominated — Indonesian banks commonly offer USD time deposits at a
	// much lower rate than IDR ones (interest rate parity), which this reflects
	// (2.5% vs the IDR TD's 5.5%). One of the "select few" USD positions
	// exercising multi-currency conversion; the principal came out of USD Savings.
	td2Placement := demoBCATD.placed.date(months, 12)
	td2Principal := decimal.RequireFromString("1500")
	td2, err := investments.CreateTimeDeposit(ctx, repo.CreateTimeDepositParams{
		DisplayName:    "BCA Time Deposit",
		OwnershipType:  "joint",
		NativeCurrency: "USD",
		RiskProfile:    "low",
		BankName:       "Bank BCA",
		Principal:      td2Principal,
		InterestRate:   decimal.RequireFromString("2.5"),
		TermMonths:     int32(demoBCATD.termMonths),
		PlacementDate:  td2Placement,
		MaturityDate:   td2Placement.AddDate(0, demoBCATD.termMonths, 0),
		RolloverPolicy: "no_rollover",
	})
	if err != nil {
		return fmt.Errorf("demo reset: seed time deposit (BCA): %w", err)
	}
	return seedInterestBearingSnapshots(ctx, investments, td2.Investment.ID, months, &demoBCATD.value, nil, td2Principal, "USD")
}

// seedDemoLiabilitiesAndReceivables seeds two Liabilities (one personal, one
// institutional) and two Receivables, each with the snapshot trail from its
// table. The mortgage amortises on a fixed instalment; the personal loan, the
// loan to a friend and the USD invoice move in sporadic chunks, flat in between.
func seedDemoLiabilitiesAndReceivables(ctx context.Context, pool *pgxpool.Pool, months []time.Time, ownerID, member2ID uuid.UUID, tags map[string]uuid.UUID) error {
	liabilities := repo.NewLiabilityRepo(pool)
	receivables := repo.NewReceivableRepo(pool)
	tagRepo := repo.NewTagRepo(pool)

	familyLoanStart := at(7, time.November).date(months, 5)
	familyLoan, err := liabilities.CreateLiability(ctx, repo.CreateLiabilityParams{
		DisplayName:      "Loan from Parents",
		Subtype:          "personal",
		OwnershipType:    "sole",
		SoleOwnerUserID:  &ownerID,
		NativeCurrency:   demoCurrency,
		CounterpartyName: "Parents",
		Principal:        decimalPtr("12000000"),
		StartDate:        &familyLoanStart,
	})
	if err != nil {
		return fmt.Errorf("demo reset: seed liability (family loan): %w", err)
	}
	if err := seedLiabilitySnapshots(ctx, liabilities, familyLoan.ID, months, &demoParentsLoan, demoCurrency); err != nil {
		return err
	}

	mortgageStart := at(7, time.June).date(months, 5)
	mortgageMaturity := mortgageStart.AddDate(15, 0, 0)
	mortgage, err := liabilities.CreateLiability(ctx, repo.CreateLiabilityParams{
		DisplayName:      "Home Mortgage",
		Subtype:          "institutional",
		OwnershipType:    "joint",
		NativeCurrency:   demoCurrency,
		CounterpartyName: "Bank Mandiri",
		Principal:        decimalPtr("185000000"),
		InterestRate:     decimalPtr("8.5"),
		TermMonths:       int32Ptr(180),
		StartDate:        &mortgageStart,
		MaturityDate:     &mortgageMaturity,
	})
	if err != nil {
		return fmt.Errorf("demo reset: seed liability (mortgage): %w", err)
	}
	if err := seedLiabilitySnapshots(ctx, liabilities, mortgage.ID, months, &demoMortgage, demoCurrency); err != nil {
		return err
	}
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
	if err := seedReceivableSnapshots(ctx, receivables, friendLoan.ID, months, &demoFriendLoan, demoCurrency, demoRecorded(months, "Money Lent to a Friend")); err != nil {
		return err
	}

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
	return seedReceivableSnapshots(ctx, receivables, invoice.ID, months, &demoFreelanceInvoice, "USD", len(months))
}

// ownership resolves a demoOwner to the (ownership_type, sole owner) pair the
// create params take.
func (o demoOwner) ownership(ownerID, member2ID uuid.UUID) (string, *uuid.UUID) {
	switch o {
	case demoOwnerSole:
		return "sole", &ownerID
	case demoOwnerMember2:
		return "sole", &member2ID
	}
	return "joint", nil
}

// seedDemoIncome seeds the routine monthly Salary and Pension across the full
// history, a monthly bank Interest, and the dated events (THR, year-end bonus
// and incidental one-offs across every other category — ADR-0008) so the
// income-statement view has real category variety.
func seedDemoIncome(ctx context.Context, pool *pgxpool.Pool, months []time.Time, ownerID, member2ID uuid.UUID) error {
	income := repo.NewIncomeRepo(pool)

	monthly := []struct {
		grid        *demoGrid
		day         int
		category    string
		description string
		owner       demoOwner
		regularity  string
	}{
		{&demoSalary, 25, "salary", "Monthly salary", demoOwnerSole, "routine"},
		// A routine Pension for the second member (ADR-0048) — a passive *cash*
		// stream, so the statistics panel's Passive-Income Ratio and Fund
		// Resilience read meaningfully rather than off a single incidental rental.
		{&demoPension, 3, "pension", "Monthly pension", demoOwnerMember2, "routine"},
		// Interest is incidental: real-world current/savings rates are negligible,
		// and it is credited back into the account that earned it rather than paid
		// out, so it is not income the household relies on. Rent and pension carry
		// the passive scope in the demo; interest is here for the category
		// breakdown and as the bookkeeping counterpart to the balance it accrues into.
		{&demoSavingsInterest, 25, "interest", "Savings account interest", demoOwnerJoint, "incidental"},
	}
	for _, m := range monthly {
		ownership, sole := m.owner.ownership(ownerID, member2ID)
		for i, ym := range months {
			desc := m.description
			if _, err := income.CreateIncome(ctx, repo.CreateIncomeParams{
				Date:            dayIn(ym, m.day),
				Amount:          m.grid.money(months, i, demoCurrency),
				Currency:        demoCurrency,
				Category:        m.category,
				Description:     &desc,
				OwnershipType:   ownership,
				SoleOwnerUserID: sole,
				Regularity:      m.regularity,
			}); err != nil {
				return fmt.Errorf("demo reset: seed %s income: %w", m.category, err)
			}
		}
	}

	for _, ev := range demoIncomeEvents {
		idx, place := ev.when.locate(months)
		if place != demoInWindow {
			continue
		}
		desc := ev.description
		ownership, sole := ev.owner.ownership(ownerID, member2ID)
		if _, err := income.CreateIncome(ctx, repo.CreateIncomeParams{
			Date:            dayIn(months[idx], ev.day),
			Amount:          decimal.NewFromInt(ev.amount),
			Currency:        demoCurrency,
			Category:        ev.category,
			Description:     &desc,
			OwnershipType:   ownership,
			SoleOwnerUserID: sole,
			Regularity:      ev.regularity,
		}); err != nil {
			return fmt.Errorf("demo reset: seed income event (%s): %w", ev.category, err)
		}
	}
	return nil
}

func tagPtr(tags map[string]uuid.UUID, name string) *uuid.UUID {
	id := tags[name]
	return &id
}

func strPtr(s string) *string { return &s }

func int32Ptr(i int32) *int32 { return &i }

func decimalPtr(s string) *decimal.Decimal {
	d := decimal.RequireFromString(s)
	return &d
}
