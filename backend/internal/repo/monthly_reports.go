package repo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/kerti/balances-v2/backend/internal/db"
)

// ptr returns a pointer to a decimal, for the nullable income-statement
// columns that the engine always computes a value for.
func ptr(d decimal.Decimal) *decimal.Decimal { return &d }

// MonthlyReportRepo serves the materialized monthly net-worth report (ADR-0006).
// Reads are lazy: ListReports / GetReport regenerate the household's rows when
// the inputs are newer than what's materialized, then return the cached rows.
// The compute itself lives in the pure engine (monthly_reports_engine.go); this
// type only fetches inputs, decides staleness, and upserts.
type MonthlyReportRepo struct {
	pool *pgxpool.Pool
	q    *db.Queries
}

func NewMonthlyReportRepo(pool *pgxpool.Pool) *MonthlyReportRepo {
	return &MonthlyReportRepo{pool: pool, q: db.New(pool)}
}

// ListReports refreshes the household's reports if stale, then returns them
// ascending by month.
func (r *MonthlyReportRepo) ListReports(ctx context.Context) ([]db.MonthlyReport, error) {
	uid, hid, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	if err := r.refresh(ctx, uid, hid); err != nil {
		return nil, err
	}
	rows, err := r.q.ListMonthlyReports(ctx, hid)
	if err != nil {
		return nil, fmt.Errorf("list monthly reports: %w", err)
	}
	if rows == nil {
		return []db.MonthlyReport{}, nil
	}
	return rows, nil
}

// ReportingCurrency returns the household's reporting currency, which the
// dashboard needs to format the (single-currency, slice-1) aggregates.
func (r *MonthlyReportRepo) ReportingCurrency(ctx context.Context) (string, error) {
	_, hid, err := currentUser(ctx)
	if err != nil {
		return "", err
	}
	hh, err := r.q.GetHouseholdByID(ctx, hid)
	if err != nil {
		return "", fmt.Errorf("get household: %w", err)
	}
	return hh.ReportingCurrency, nil
}

// GetReport refreshes if stale, then returns a single month (ErrNotFound when
// the month is outside the reportable range).
func (r *MonthlyReportRepo) GetReport(ctx context.Context, yearMonth time.Time) (*db.MonthlyReport, error) {
	uid, hid, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	if err := r.refresh(ctx, uid, hid); err != nil {
		return nil, err
	}
	row, err := r.q.GetMonthlyReport(ctx, db.GetMonthlyReportParams{
		HouseholdID: hid,
		YearMonth:   monthFromIndex(monthIndex(yearMonth)),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get monthly report: %w", err)
	}
	return &row, nil
}

// refresh regenerates and upserts the household's reports when the materialized
// rows are stale or the month range changed.
func (r *MonthlyReportRepo) refresh(ctx context.Context, uid, hid uuid.UUID) error {
	currentMonth, err := r.currentMonth(ctx, uid)
	if err != nil {
		return err
	}
	in, err := r.loadEngineInput(ctx, hid, currentMonth)
	if err != nil {
		return err
	}
	reports := generateMonthlyReports(in)
	if len(reports) == 0 {
		return nil // no snapshot data — nothing to materialize
	}
	first := reports[0].yearMonth
	last := reports[len(reports)-1].yearMonth

	watermark, err := r.q.MaxReportInputUpdatedAt(ctx, db.MaxReportInputUpdatedAtParams{
		HouseholdID: hid,
		YearMonth:   last,
	})
	if err != nil {
		return fmt.Errorf("report staleness watermark: %w", err)
	}
	existing, err := r.q.ListMonthlyReports(ctx, hid)
	if err != nil {
		return fmt.Errorf("list monthly reports: %w", err)
	}
	if !needsRegen(reports, existing, watermark) {
		return nil
	}
	return r.writeReports(ctx, hid, first, last, reports)
}

// needsRegen is the coarse-but-correct slice-1 staleness check: regenerate the
// whole household when the materialized month set differs from the engine's or
// any materialized row predates the input watermark (ADR-0006 conservative
// rule — over-regenerates cheaply for one household, never serves stale).
func needsRegen(reports []monthlyReportData, existing []db.MonthlyReport, watermark pgtype.Timestamptz) bool {
	if len(existing) != len(reports) {
		return true
	}
	have := make(map[int]pgtype.Timestamptz, len(existing))
	for _, e := range existing {
		have[monthIndex(e.YearMonth)] = e.GeneratedAt
	}
	for _, rep := range reports {
		gen, ok := have[monthIndex(rep.yearMonth)]
		if !ok {
			return true
		}
		if !gen.Valid || gen.Time.Before(watermark.Time) {
			return true
		}
	}
	return false
}

// writeReports prunes out-of-range cache rows and upserts every generated month
// in one transaction.
func (r *MonthlyReportRepo) writeReports(ctx context.Context, hid uuid.UUID, first, last time.Time, reports []monthlyReportData) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin report write: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := r.q.WithTx(tx)

	if err := qtx.DeleteMonthlyReportsOutsideRange(ctx, db.DeleteMonthlyReportsOutsideRangeParams{
		HouseholdID: hid,
		YearMonth:   first,
		YearMonth_2: last,
	}); err != nil {
		return fmt.Errorf("prune monthly reports: %w", err)
	}
	for _, rep := range reports {
		params, err := buildUpsertParams(hid, rep)
		if err != nil {
			return err
		}
		if _, err := qtx.UpsertMonthlyReport(ctx, params); err != nil {
			return fmt.Errorf("upsert monthly report: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit report write: %w", err)
	}
	return nil
}

// writeReport upserts a single month without pruning — the per-month rebuild
// path (RebuildMonth), where neighbouring cached months must stay intact.
func (r *MonthlyReportRepo) writeReport(ctx context.Context, hid uuid.UUID, rep monthlyReportData) error {
	params, err := buildUpsertParams(hid, rep)
	if err != nil {
		return err
	}
	if _, err := r.q.UpsertMonthlyReport(ctx, params); err != nil {
		return fmt.Errorf("upsert monthly report: %w", err)
	}
	return nil
}

// buildUpsertParams marshals one generated month into upsert params. The
// income-statement columns are always-computed except on the first-month
// baseline, where investment_return/asset_value_change/living_expenses stay nil
// (ADR-0006).
func buildUpsertParams(hid uuid.UUID, rep monthlyReportData) (db.UpsertMonthlyReportParams, error) {
	ub, err := json.Marshal(rep.userBreakdowns)
	if err != nil {
		return db.UpsertMonthlyReportParams{}, fmt.Errorf("marshal user breakdowns: %w", err)
	}
	stale, err := json.Marshal(rep.stalePositions)
	if err != nil {
		return db.UpsertMonthlyReportParams{}, fmt.Errorf("marshal stale positions: %w", err)
	}
	fxUsed, err := json.Marshal(rep.fxRatesUsed)
	if err != nil {
		return db.UpsertMonthlyReportParams{}, fmt.Errorf("marshal fx rates used: %w", err)
	}
	missingFx, err := json.Marshal(rep.missingFx)
	if err != nil {
		return db.UpsertMonthlyReportParams{}, fmt.Errorf("marshal missing fx: %w", err)
	}
	params := db.UpsertMonthlyReportParams{
		HouseholdID:           hid,
		YearMonth:             rep.yearMonth,
		NwTotal:               rep.nwTotal,
		NwAssets:              rep.nwAssets,
		NwLiabilities:         rep.nwLiabilities,
		NwReceivables:         rep.nwReceivables,
		NwInvestments:         rep.nwInvestments,
		EarnedIncomeTotal:     ptr(rep.earnedIncome.total),
		EarnedIncomeSalary:    ptr(rep.earnedIncome.salary),
		EarnedIncomeBusiness:  ptr(rep.earnedIncome.business),
		EarnedIncomeRental:    ptr(rep.earnedIncome.rental),
		EarnedIncomeGift:      ptr(rep.earnedIncome.gift),
		EarnedIncomeTaxRefund: ptr(rep.earnedIncome.taxRefund),
		EarnedIncomeInsurance: ptr(rep.earnedIncome.insurance),
		EarnedIncomeOther:     ptr(rep.earnedIncome.other),
		AssetValueChange:      rep.assetValueChange, // nil on baseline
		DerivedLivingExpenses: rep.livingExpenses,   // nil on baseline
		UserBreakdowns:        ub,
		StalePositions:        stale,
		FxRatesUsed:           fxUsed,
		MissingFx:             missingFx,
	}
	if rep.investmentReturn != nil { // suppressed on the baseline month
		params.InvestmentReturnTotal = ptr(rep.investmentReturn.total)
		params.InvestmentReturnStock = ptr(rep.investmentReturn.stock)
		params.InvestmentReturnMutualFund = ptr(rep.investmentReturn.mutualFund)
		params.InvestmentReturnBond = ptr(rep.investmentReturn.bond)
		params.InvestmentReturnGold = ptr(rep.investmentReturn.gold)
		params.InvestmentReturnTimeDeposit = ptr(rep.investmentReturn.timeDeposit)
	}
	return params, nil
}

// RebuildAll forces a full regeneration of the household's reports, ignoring the
// staleness watermark (ADR-0006 manual rebuild, household scope). The escape
// hatch for changes the data-driven watermark can't see — engine code changes
// and FX corrections that should propagate across history. Mirrors refresh's
// no-data behaviour: with no inputs there's nothing to materialize.
func (r *MonthlyReportRepo) RebuildAll(ctx context.Context) error {
	_, hid, err := currentUser(ctx)
	if err != nil {
		return err
	}
	reports, err := r.generate(ctx, hid)
	if err != nil {
		return err
	}
	if len(reports) == 0 {
		return nil
	}
	return r.writeReports(ctx, hid, reports[0].yearMonth, reports[len(reports)-1].yearMonth, reports)
}

// RebuildMonth forces regeneration of a single month, ignoring staleness
// (ADR-0006 manual rebuild, per-month scope — surgical fixes). Carry-forward
// means the recompute still reads every input ≤ M; only the one row is
// rewritten, so neighbouring cached months are untouched. ErrNotFound when the
// month falls outside the reportable range.
func (r *MonthlyReportRepo) RebuildMonth(ctx context.Context, yearMonth time.Time) error {
	_, hid, err := currentUser(ctx)
	if err != nil {
		return err
	}
	reports, err := r.generate(ctx, hid)
	if err != nil {
		return err
	}
	target := monthFromIndex(monthIndex(yearMonth))
	for _, rep := range reports {
		if rep.yearMonth.Equal(target) {
			return r.writeReport(ctx, hid, rep)
		}
	}
	return ErrNotFound
}

// generate loads the engine input for the household's current month and runs
// the pure engine. Shared by the two rebuild paths.
func (r *MonthlyReportRepo) generate(ctx context.Context, hid uuid.UUID) ([]monthlyReportData, error) {
	uid, _, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	currentMonth, err := r.currentMonth(ctx, uid)
	if err != nil {
		return nil, err
	}
	in, err := r.loadEngineInput(ctx, hid, currentMonth)
	if err != nil {
		return nil, err
	}
	return generateMonthlyReports(in), nil
}

// currentMonth is the first of the month containing now() in the requesting
// user's time zone (ADR-0006). A bad/unknown zone falls back to UTC rather than
// failing the dashboard.
func (r *MonthlyReportRepo) currentMonth(ctx context.Context, uid uuid.UUID) (time.Time, error) {
	u, err := r.q.GetUserByID(ctx, uid)
	if err != nil {
		return time.Time{}, fmt.Errorf("load user for time zone: %w", err)
	}
	loc, err := time.LoadLocation(u.TimeZone)
	if err != nil {
		loc = time.UTC
	}
	now := time.Now().In(loc)
	return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC), nil
}

// loadEngineInput fetches the household's positions, snapshots, and members and
// shapes them for the pure engine.
func (r *MonthlyReportRepo) loadEngineInput(ctx context.Context, hid uuid.UUID, currentMonth time.Time) (reportEngineInput, error) {
	in := reportEngineInput{currentMonth: currentMonth}

	assets, err := r.q.ListAssetsForReport(ctx, hid)
	if err != nil {
		return in, fmt.Errorf("list assets for report: %w", err)
	}
	for _, a := range assets {
		in.positions = append(in.positions, reportPosition{
			id: a.ID, name: a.DisplayName, group: groupAsset, subtype: a.Subtype, ownershipType: a.OwnershipType,
			soleOwnerID: a.SoleOwnerUserID, terminatedAt: a.TerminatedAt,
		})
	}
	liabilities, err := r.q.ListLiabilitiesForReport(ctx, hid)
	if err != nil {
		return in, fmt.Errorf("list liabilities for report: %w", err)
	}
	for _, l := range liabilities {
		in.positions = append(in.positions, reportPosition{
			id: l.ID, name: l.DisplayName, group: groupLiability, subtype: l.Subtype, ownershipType: l.OwnershipType,
			soleOwnerID: l.SoleOwnerUserID, terminatedAt: l.TerminatedAt,
		})
	}
	receivables, err := r.q.ListReceivablesForReport(ctx, hid)
	if err != nil {
		return in, fmt.Errorf("list receivables for report: %w", err)
	}
	for _, rc := range receivables {
		in.positions = append(in.positions, reportPosition{
			id: rc.ID, name: rc.DisplayName, group: groupReceivable, ownershipType: rc.OwnershipType,
			soleOwnerID: rc.SoleOwnerUserID, terminatedAt: rc.TerminatedAt,
		})
	}
	investments, err := r.q.ListInvestmentsForReport(ctx, hid)
	if err != nil {
		return in, fmt.Errorf("list investments for report: %w", err)
	}
	for _, i := range investments {
		p := reportPosition{
			id: i.ID, name: i.DisplayName, group: groupInvestment, subtype: i.Subtype, ownershipType: i.OwnershipType,
			soleOwnerID: i.SoleOwnerUserID, terminatedAt: i.TerminatedAt,
			rolledFrom: i.RolledFromInvestmentID,
		}
		// TimeDeposit placement cash_in (issue #27): the principal + placement
		// date feed the engine's synthetic flow so the placement month nets to 0
		// return. Other subtypes record a real Buy instead, so these stay nil.
		// A rolled-over TD is funded by its predecessor's maturity instead — the
		// engine routes that terminal value here as cash_in, so it takes no
		// synthetic TdPrincipal placement (which would understate the rolled-in
		// interest and double-count against the rollover cash_in).
		if i.Subtype == "time_deposit" && i.RolledFromInvestmentID == nil &&
			i.TdPrincipal != nil && i.TdPlacementDate != nil {
			p.placementAmount = i.TdPrincipal
			p.placementMonth = i.TdPlacementDate
			p.currency = i.NativeCurrency
		}
		in.positions = append(in.positions, p)
	}

	asnaps, err := r.q.ListAssetSnapshotsForReport(ctx, hid)
	if err != nil {
		return in, fmt.Errorf("list asset snapshots for report: %w", err)
	}
	for _, s := range asnaps {
		in.snapshots = append(in.snapshots, reportSnapshot{positionID: s.PositionID, yearMonth: s.YearMonth, amount: s.Amount, currency: s.Currency})
	}
	lsnaps, err := r.q.ListLiabilitySnapshotsForReport(ctx, hid)
	if err != nil {
		return in, fmt.Errorf("list liability snapshots for report: %w", err)
	}
	for _, s := range lsnaps {
		in.snapshots = append(in.snapshots, reportSnapshot{positionID: s.PositionID, yearMonth: s.YearMonth, amount: s.Amount, currency: s.Currency})
	}
	rsnaps, err := r.q.ListReceivableSnapshotsForReport(ctx, hid)
	if err != nil {
		return in, fmt.Errorf("list receivable snapshots for report: %w", err)
	}
	for _, s := range rsnaps {
		in.snapshots = append(in.snapshots, reportSnapshot{positionID: s.PositionID, yearMonth: s.YearMonth, amount: s.Amount, currency: s.Currency})
	}
	isnaps, err := r.q.ListInvestmentSnapshotsForReport(ctx, hid)
	if err != nil {
		return in, fmt.Errorf("list investment snapshots for report: %w", err)
	}
	for _, s := range isnaps {
		in.snapshots = append(in.snapshots, reportSnapshot{positionID: s.PositionID, yearMonth: s.YearMonth, amount: s.Amount, currency: s.Currency})
	}

	incomes, err := r.q.ListIncomeForReport(ctx, hid)
	if err != nil {
		return in, fmt.Errorf("list income for report: %w", err)
	}
	for _, inc := range incomes {
		in.income = append(in.income, reportIncome{
			yearMonth: inc.Date, amount: inc.Amount, currency: inc.Currency, category: inc.Category,
			ownershipType: inc.OwnershipType, soleOwnerID: inc.SoleOwnerUserID,
		})
	}

	txns, err := r.q.ListInvestmentTransactionsForReport(ctx, hid)
	if err != nil {
		return in, fmt.Errorf("list investment transactions for report: %w", err)
	}
	for _, t := range txns {
		in.transactions = append(in.transactions, reportTransaction{
			investmentID: t.InvestmentID, yearMonth: t.TransactionDate, currency: t.Currency, txnType: t.TransactionType,
			amount: t.Amount, quantity: t.Quantity,
			principalAmount: t.PrincipalAmount, interestAmount: t.InterestAmount,
			principalDisposition: t.PrincipalDisposition, interestDisposition: t.InterestDisposition,
		})
	}

	users, err := r.q.ListUsersByHousehold(ctx, hid)
	if err != nil {
		return in, fmt.Errorf("list household members for report: %w", err)
	}
	for _, u := range users {
		in.members = append(in.members, u.ID)
	}

	hh, err := r.q.GetHouseholdByID(ctx, hid)
	if err != nil {
		return in, fmt.Errorf("get household for report: %w", err)
	}
	in.reportingCurrency = hh.ReportingCurrency
	in.multiCurrency = hh.MultiCurrencyEnabled

	rates, err := r.q.ListFxRatesForReport(ctx, hid)
	if err != nil {
		return in, fmt.Errorf("list fx rates for report: %w", err)
	}
	for _, fr := range rates {
		in.fxRates = append(in.fxRates, reportFxRate{currency: fr.Currency, yearMonth: fr.YearMonth, rate: fr.Rate})
	}
	return in, nil
}
