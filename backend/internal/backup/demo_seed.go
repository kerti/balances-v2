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

// demoMonths returns the 12 calendar months ending at the current month,
// oldest first, so every reset lands its snapshot trail in the trailing year
// regardless of when the cron actually runs (issue: demo data going stale).
func demoMonths() [12]time.Time {
	now := time.Now().UTC()
	var months [12]time.Time
	for i := 0; i < 12; i++ {
		offset := 11 - i
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

// demoSeries produces a 12-point deterministic value curve — a steady
// monthly trend plus a small sinusoidal wobble — so seeded charts show
// motion instead of a flat line, without pulling in real randomness.
func demoSeries(base, monthlyGrowth, wobble float64) [12]float64 {
	var vals [12]float64
	v := base
	for i := 0; i < 12; i++ {
		vals[i] = v * (1 + wobble*math.Sin(float64(i)*1.3))
		v *= 1 + monthlyGrowth
	}
	return vals
}

func demoDecimal(f float64) decimal.Decimal {
	return decimal.NewFromFloat(f).Round(2)
}

// seedDemoData wipes-then-rebuilds the shared demo Household's entire toy
// dataset (ADR-0041, #217): at least two Positions per Asset/Investment
// subtype plus Liabilities, Receivables, Income, and Tags, each carrying a
// full trailing-12-month snapshot trail — enough spread that a first-time
// visitor can see every position type, tag breakdown, and income category in
// action rather than an empty-feeling dashboard.
func seedDemoData(ctx context.Context, pool *pgxpool.Pool, ownerID, member2ID uuid.UUID) error {
	tags, err := seedDemoTags(ctx, pool)
	if err != nil {
		return err
	}
	if err := seedDemoAssets(ctx, pool, ownerID, member2ID, tags); err != nil {
		return err
	}
	if err := seedDemoInvestments(ctx, pool, ownerID, member2ID, tags); err != nil {
		return err
	}
	if err := seedDemoLiabilitiesAndReceivables(ctx, pool, ownerID, member2ID, tags); err != nil {
		return err
	}
	if err := seedDemoIncome(ctx, pool, ownerID, member2ID); err != nil {
		return err
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

// seedAssetSnapshots backfills a 12-month value trail (amount-only, ADR-0022
// snapshot shape) for a Bank Account / Property / Vehicle Position.
func seedAssetSnapshots(ctx context.Context, assets *repo.AssetRepo, assetID uuid.UUID, base, growth, wobble float64) error {
	series := demoSeries(base, growth, wobble)
	for i, ym := range demoMonths() {
		if _, err := assets.CreateAssetSnapshot(ctx, repo.CreateAssetSnapshotParams{
			AssetID:   assetID,
			YearMonth: ym,
			Amount:    demoDecimal(series[i]),
			Currency:  demoCurrency,
		}); err != nil {
			return fmt.Errorf("demo reset: seed asset snapshot: %w", err)
		}
	}
	return nil
}

// seedMarketSnapshots backfills a 12-month quantity+price trail for a Stock /
// MutualFund / Gold Position — quantity held steady (no seeded Buy/Sell
// ledger), price wobbling to show market movement.
func seedMarketSnapshots(ctx context.Context, investments *repo.InvestmentRepo, investmentID uuid.UUID, qty decimal.Decimal, basePrice, priceGrowth, wobble float64) error {
	prices := demoSeries(basePrice, priceGrowth, wobble)
	for i, ym := range demoMonths() {
		price := demoDecimal(prices[i])
		q := qty
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
	return nil
}

// seedInterestBearingSnapshots backfills a 12-month total-value+accrued trail
// for a Bond / TimeDeposit Position (the "dirty total value" shape, ADR-0022).
// resetEveryMonths cycles accrued back to near-zero on a coupon schedule
// (secondary-market bonds that accrue); 0 means it grows monotonically for
// the whole window instead (a time deposit accruing toward its single
// maturity payout). accrualStep 0 keeps accrued pinned at zero throughout
// (a pays-out govt bond, where the coupon lands in the bank, not the
// instrument).
func seedInterestBearingSnapshots(ctx context.Context, investments *repo.InvestmentRepo, investmentID uuid.UUID, principalBase, principalGrowth, principalWobble, accrualStep float64, resetEveryMonths int) error {
	principals := demoSeries(principalBase, principalGrowth, principalWobble)
	for i, ym := range demoMonths() {
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
			Currency:        demoCurrency,
			AccruedInterest: &accrued,
		}); err != nil {
			return fmt.Errorf("demo reset: seed interest-bearing snapshot: %w", err)
		}
	}
	return nil
}

// seedLiabilitySnapshots backfills a 12-month balance trail for a Liability.
func seedLiabilitySnapshots(ctx context.Context, liabilities *repo.LiabilityRepo, liabilityID uuid.UUID, base, growth, wobble float64) error {
	series := demoSeries(base, growth, wobble)
	for i, ym := range demoMonths() {
		if _, err := liabilities.CreateLiabilitySnapshot(ctx, repo.CreateLiabilitySnapshotParams{
			LiabilityID: liabilityID,
			YearMonth:   ym,
			Amount:      demoDecimal(series[i]),
			Currency:    demoCurrency,
		}); err != nil {
			return fmt.Errorf("demo reset: seed liability snapshot: %w", err)
		}
	}
	return nil
}

// seedReceivableSnapshots backfills a 12-month balance trail for a Receivable.
func seedReceivableSnapshots(ctx context.Context, receivables *repo.ReceivableRepo, receivableID uuid.UUID, base, growth, wobble float64) error {
	series := demoSeries(base, growth, wobble)
	for i, ym := range demoMonths() {
		if _, err := receivables.CreateReceivableSnapshot(ctx, repo.CreateReceivableSnapshotParams{
			ReceivableID: receivableID,
			YearMonth:    ym,
			Amount:       demoDecimal(series[i]),
			Currency:     demoCurrency,
		}); err != nil {
			return fmt.Errorf("demo reset: seed receivable snapshot: %w", err)
		}
	}
	return nil
}

// seedDemoAssets seeds two Positions each of BankAccount, Property, and
// Vehicle — the three Asset subtypes (ADR-0022) — with a full snapshot trail
// and a mix of sole/joint ownership and tag assignment.
func seedDemoAssets(ctx context.Context, pool *pgxpool.Pool, ownerID, member2ID uuid.UUID, tags map[string]uuid.UUID) error {
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
		return fmt.Errorf("demo reset: seed bank account (checking): %w", err)
	}
	if err := seedAssetSnapshots(ctx, assets, checking.Asset.ID, 15_000_000, 0.01, 0.03); err != nil {
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
	if err := seedAssetSnapshots(ctx, assets, savings.Asset.ID, 45_000_000, 0.006, 0.015); err != nil {
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
		AcquisitionDate:        &acqLong,
		AcquisitionCost:        decimalPtr(950_000_000),
		AnnualAppreciationRate: decimalPtr(0.04),
	})
	if err != nil {
		return fmt.Errorf("demo reset: seed property (family home): %w", err)
	}
	if err := seedAssetSnapshots(ctx, assets, familyHome.Asset.ID, 1_200_000_000, 0.003, 0.008); err != nil {
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
		AcquisitionDate:        &acqLong,
		AcquisitionCost:        decimalPtr(700_000_000),
		AnnualAppreciationRate: decimalPtr(0.035),
	})
	if err != nil {
		return fmt.Errorf("demo reset: seed property (rental): %w", err)
	}
	if err := seedAssetSnapshots(ctx, assets, rental.Asset.ID, 800_000_000, 0.004, 0.008); err != nil {
		return err
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
		AnnualDepreciationRate: decimalPtr(0.08),
	})
	if err != nil {
		return fmt.Errorf("demo reset: seed vehicle (car): %w", err)
	}
	if err := seedAssetSnapshots(ctx, assets, car.Asset.ID, 250_000_000, -0.004, 0.01); err != nil {
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
		Year:                   int32Ptr(2023),
		PlateNumber:            strPtr("B 5678 DEM"),
		AnnualDepreciationRate: decimalPtr(0.1),
	})
	if err != nil {
		return fmt.Errorf("demo reset: seed vehicle (bike): %w", err)
	}
	if err := seedAssetSnapshots(ctx, assets, bike.Asset.ID, 25_000_000, -0.01, 0.02); err != nil {
		return err
	}

	return nil
}

// seedDemoInvestments seeds two Positions each of the five Investment
// subtypes (Stock, MutualFund, Bond, Gold, TimeDeposit — ADR-0022) with a
// full snapshot trail.
func seedDemoInvestments(ctx context.Context, pool *pgxpool.Pool, ownerID, member2ID uuid.UUID, tags map[string]uuid.UUID) error {
	investments := repo.NewInvestmentRepo(pool)
	tagRepo := repo.NewTagRepo(pool)
	months := demoMonths()
	earliest := months[0]

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
	if err := seedMarketSnapshots(ctx, investments, stock1.Investment.ID, decimal.RequireFromString("100"), 9_500, 0.012, 0.04); err != nil {
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
	if err := seedMarketSnapshots(ctx, investments, stock2.Investment.ID, decimal.RequireFromString("200"), 5_200, 0.008, 0.05); err != nil {
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
	if err := seedMarketSnapshots(ctx, investments, mf1.Investment.ID, decimal.RequireFromString("1000"), 1_450, 0.01, 0.03); err != nil {
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
	if err := seedMarketSnapshots(ctx, investments, mf2.Investment.ID, decimal.RequireFromString("5000"), 1_050, 0.002, 0.005); err != nil {
		return err
	}

	bondPlacement := dayIn(earliest, 10)
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
		FaceValue:         decimalPtr(50_000_000),
		PlacementDate:     &bondPlacement,
	})
	if err != nil {
		return fmt.Errorf("demo reset: seed bond (ORI024): %w", err)
	}
	if err := seedInterestBearingSnapshots(ctx, investments, bond1.Investment.ID, 50_000_000, 0, 0, 0, 1); err != nil {
		return err
	}

	bond2Maturity := dayIn(months[11], 1).AddDate(4, 0, 0)
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
	if err := seedInterestBearingSnapshots(ctx, investments, bond2.Investment.ID, 40_000_000, 0.002, 0.01, 250_000, 6); err != nil {
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
	if err := seedMarketSnapshots(ctx, investments, gold1.Investment.ID, decimal.RequireFromString("50"), 1_150_000, 0.008, 0.02); err != nil {
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
	if err := seedMarketSnapshots(ctx, investments, gold2.Investment.ID, decimal.RequireFromString("20"), 1_150_000, 0.008, 0.02); err != nil {
		return err
	}

	td1Placement := earliest
	td1Maturity := td1Placement.AddDate(2, 0, 0)
	td1, err := investments.CreateTimeDeposit(ctx, repo.CreateTimeDepositParams{
		DisplayName:     "Mandiri Time Deposit",
		OwnershipType:   "sole",
		SoleOwnerUserID: &ownerID,
		NativeCurrency:  demoCurrency,
		RiskProfile:     "low",
		BankName:        "Bank Mandiri",
		Principal:       decimal.RequireFromString("100000000"),
		InterestRate:    decimal.RequireFromString("5.5"),
		TermMonths:      24,
		PlacementDate:   td1Placement,
		MaturityDate:    td1Maturity,
		RolloverPolicy:  "auto_renew_with_interest",
	})
	if err != nil {
		return fmt.Errorf("demo reset: seed time deposit (Mandiri): %w", err)
	}
	if err := seedInterestBearingSnapshots(ctx, investments, td1.Investment.ID, 100_000_000, 0, 0, 45_000, 0); err != nil {
		return err
	}
	if err := tagRepo.AssignTag(ctx, repo.TagGroupInvestment, td1.Investment.ID, tagPtr(tags, "Education")); err != nil {
		return fmt.Errorf("demo reset: tag Mandiri time deposit: %w", err)
	}

	td2Placement := earliest
	td2Maturity := td2Placement.AddDate(0, 18, 0)
	td2, err := investments.CreateTimeDeposit(ctx, repo.CreateTimeDepositParams{
		DisplayName:    "BCA Time Deposit",
		OwnershipType:  "joint",
		NativeCurrency: demoCurrency,
		RiskProfile:    "low",
		BankName:       "Bank BCA",
		Principal:      decimal.RequireFromString("200000000"),
		InterestRate:   decimal.RequireFromString("6"),
		TermMonths:     18,
		PlacementDate:  td2Placement,
		MaturityDate:   td2Maturity,
		RolloverPolicy: "no_rollover",
	})
	if err != nil {
		return fmt.Errorf("demo reset: seed time deposit (BCA): %w", err)
	}
	if err := seedInterestBearingSnapshots(ctx, investments, td2.Investment.ID, 200_000_000, 0, 0, 90_000, 0); err != nil {
		return err
	}

	return nil
}

// seedDemoLiabilitiesAndReceivables seeds two Liabilities (one personal, one
// institutional) and two Receivables, each with a snapshot trail.
func seedDemoLiabilitiesAndReceivables(ctx context.Context, pool *pgxpool.Pool, ownerID, member2ID uuid.UUID, tags map[string]uuid.UUID) error {
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
		Principal:        decimalPtr(20_000_000),
		StartDate:        &familyLoanStart,
	})
	if err != nil {
		return fmt.Errorf("demo reset: seed liability (family loan): %w", err)
	}
	if err := seedLiabilitySnapshots(ctx, liabilities, familyLoan.ID, 20_000_000, -0.01, 0); err != nil {
		return err
	}

	mortgageStart := earliest.AddDate(-2, 0, 0)
	mortgageMaturity := mortgageStart.AddDate(15, 0, 0)
	mortgage, err := liabilities.CreateLiability(ctx, repo.CreateLiabilityParams{
		DisplayName:      "Home Mortgage",
		Subtype:          "institutional",
		OwnershipType:    "joint",
		NativeCurrency:   demoCurrency,
		CounterpartyName: "Bank Mandiri",
		Principal:        decimalPtr(900_000_000),
		InterestRate:     decimalPtr(8.5),
		TermMonths:       int32Ptr(180),
		StartDate:        &mortgageStart,
		MaturityDate:     &mortgageMaturity,
	})
	if err != nil {
		return fmt.Errorf("demo reset: seed liability (mortgage): %w", err)
	}
	if err := seedLiabilitySnapshots(ctx, liabilities, mortgage.ID, 860_000_000, -0.002, 0); err != nil {
		return err
	}
	if err := tagRepo.AssignTag(ctx, repo.TagGroupLiability, mortgage.ID, tagPtr(tags, "Big Ticket")); err != nil {
		return fmt.Errorf("demo reset: tag mortgage: %w", err)
	}

	friendDue := dayIn(months[11], 20).AddDate(0, 3, 0)
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
	if err := seedReceivableSnapshots(ctx, receivables, friendLoan.ID, 5_000_000, -0.02, 0); err != nil {
		return err
	}

	invoiceDue := dayIn(months[11], 20).AddDate(0, 1, 0)
	invoice, err := receivables.CreateReceivable(ctx, repo.CreateReceivableParams{
		DisplayName:      "Freelance Invoice Pending",
		OwnershipType:    "sole",
		SoleOwnerUserID:  &member2ID,
		NativeCurrency:   demoCurrency,
		CounterpartyName: "Client Co",
		DueDate:          &invoiceDue,
	})
	if err != nil {
		return fmt.Errorf("demo reset: seed receivable (invoice): %w", err)
	}
	if err := seedReceivableSnapshots(ctx, receivables, invoice.ID, 4_000_000, 0.01, 0.05); err != nil {
		return err
	}

	return nil
}

// seedDemoIncome seeds a routine monthly Salary across the trailing year plus
// a spread of incidental Income events across every other category (ADR-0008)
// so the income-statement view has real category variety.
func seedDemoIncome(ctx context.Context, pool *pgxpool.Pool, ownerID, member2ID uuid.UUID) error {
	income := repo.NewIncomeRepo(pool)
	months := demoMonths()

	for i, ym := range months {
		desc := "Monthly salary"
		amount := demoDecimal(25_000_000 + float64(i)*150_000)
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
	}

	incidentals := []struct {
		monthIdx    int
		category    string
		description string
		amount      float64
		ownership   string
		soleID      *uuid.UUID
	}{
		{1, "business_income", "Side consulting project", 8_000_000, "sole", &ownerID},
		{3, "rental_income", "Spare room rental income", 3_500_000, "joint", nil},
		{5, "gift", "Wedding gift from relatives", 5_000_000, "joint", nil},
		{7, "tax_refund", "Annual tax refund", 2_200_000, "sole", &member2ID},
		{9, "insurance_payout", "Health insurance reimbursement", 4_500_000, "sole", &ownerID},
		{10, "other", "Marketplace resale", 1_200_000, "sole", &member2ID},
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
