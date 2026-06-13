package repo

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/kerti/balances-v2/backend/internal/db"
)

// Create-from-list import for the Investment group (issue #90) — the heaviest
// slice. Unlike the asset/liability/receivable groups (snapshots only, #88/#89),
// an investment workbook also carries a Transactions ledger, so the commit path
// seeds three things atomically: the position + its subtype details, its snapshot
// history, and its transaction ledger. Per the #51 decision this is create-only
// seeding — transactions are applied solely when creating a new position here,
// never bulk-imported into an existing one.
//
// Maturity-in-seed follows decision (b) from #90: a Maturity row produces a
// matured position with its 0-value close snapshot, reproducing the terminal
// behavior of CreateInvestmentTransaction. seedLedger applies the Maturity row
// last so the seed write-order matches the conceptual terminal-event model and
// stays correct if it is ever routed through the status-guarded insert. (The
// seed uses the unguarded db query directly, so intra-ledger order is not itself
// load-bearing today; the load-bearing ordering is snapshots-before-ledger, so
// the 0 close overwrites any seeded snapshot in the maturity month — see
// createInvestmentWithHistory.)

// ImportTransactionRow is one ledger transaction to seed on a create-from-list
// import, the repo-side counterpart of snapshotimport.ParsedTransaction. The
// value columns are the ADR-0023 union; which are non-nil depends on the type
// (already validated against the subtype matrix + shape in the import flow).
type ImportTransactionRow struct {
	TransactionType      string
	TransactionDate      time.Time
	Currency             string
	Description          *string
	Amount               *decimal.Decimal
	Quantity             *decimal.Decimal
	PricePerUnit         *decimal.Decimal
	PrincipalAmount      *decimal.Decimal
	InterestAmount       *decimal.Decimal
	PrincipalDisposition *string
	InterestDisposition  *string
}

// LookupUserIDByEmail resolves a household member's email to their user id — the
// Investment-group mirror of AssetRepo.LookupUserIDByEmail (the sole_owner Detail
// convention). Case-insensitive on a trimmed email; found is false when no member
// matches (a per-field error, not a 404). Scoped to the caller's household.
func (r *InvestmentRepo) LookupUserIDByEmail(ctx context.Context, email string) (id uuid.UUID, found bool, err error) {
	_, hid, err := currentUser(ctx)
	if err != nil {
		return uuid.Nil, false, err
	}
	users, err := r.q.ListUsersByHousehold(ctx, hid)
	if err != nil {
		return uuid.Nil, false, fmt.Errorf("lookup user by email: %w", err)
	}
	want := strings.ToLower(strings.TrimSpace(email))
	for _, u := range users {
		if strings.ToLower(u.Email) == want {
			return u.ID, true, nil
		}
	}
	return uuid.Nil, false, nil
}

// LookupTagIDByName resolves a Tag name to its id within the caller's household —
// the inverse of the Detail-sheet tag convention. Returns (nil, nil) when no Tag
// matches (an unmatched tag is left unassigned, never an error) or the name is
// blank. Exact match on a trimmed name (round-trips an export verbatim).
func (r *InvestmentRepo) LookupTagIDByName(ctx context.Context, name string) (*uuid.UUID, error) {
	_, hid, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	want := strings.TrimSpace(name)
	if want == "" {
		return nil, nil
	}
	tags, err := r.q.ListTagsByHousehold(ctx, hid)
	if err != nil {
		return nil, fmt.Errorf("lookup tag by name: %w", err)
	}
	for _, tg := range tags {
		if tg.Name == want {
			tid := tg.ID
			return &tid, nil
		}
	}
	return nil, nil
}

// CreateStockWithSnapshotsAndLedger creates a stock + its details, seeds the
// snapshot history, and seeds the transaction ledger — all in one transaction
// (all-or-nothing). It is the commit path of the stock create-from-list import.
func (r *InvestmentRepo) CreateStockWithSnapshotsAndLedger(ctx context.Context, p CreateStockParams, tagID *uuid.UUID, snaps []ImportInvestmentSnapshotRow, ledger []ImportTransactionRow) (*Stock, error) {
	var out *Stock
	err := r.createInvestmentWithHistory(ctx, "stock", tagID, snaps, ledger, termBounds{}, func(ctx context.Context, qtx *db.Queries, user, hid uuid.UUID) (db.Investment, error) {
		inv, err := qtx.CreateInvestment(ctx, db.CreateInvestmentParams{
			HouseholdID:     hid,
			DisplayName:     p.DisplayName,
			Description:     p.Description,
			Subtype:         "stock",
			OwnershipType:   p.OwnershipType,
			SoleOwnerUserID: p.SoleOwnerUserID,
			NativeCurrency:  p.NativeCurrency,
			RiskProfile:     p.RiskProfile,
			CreatedBy:       &user,
		})
		if err != nil {
			return db.Investment{}, fmt.Errorf("create investment: %w", err)
		}
		details, err := qtx.CreateStockDetails(ctx, db.CreateStockDetailsParams{
			InvestmentID: inv.ID,
			Ticker:       p.Ticker,
			Exchange:     p.Exchange,
		})
		if err != nil {
			return db.Investment{}, fmt.Errorf("create stock_details: %w", err)
		}
		out = &Stock{Investment: inv, Details: details}
		return inv, nil
	})
	if err != nil {
		return nil, err
	}
	out.Investment.TagID = tagID // reflect the assignment in the returned aggregate
	return out, nil
}

// CreateMutualFundWithSnapshotsAndLedger — see CreateStockWithSnapshotsAndLedger.
func (r *InvestmentRepo) CreateMutualFundWithSnapshotsAndLedger(ctx context.Context, p CreateMutualFundParams, tagID *uuid.UUID, snaps []ImportInvestmentSnapshotRow, ledger []ImportTransactionRow) (*MutualFund, error) {
	var out *MutualFund
	err := r.createInvestmentWithHistory(ctx, "mutual_fund", tagID, snaps, ledger, termBounds{}, func(ctx context.Context, qtx *db.Queries, user, hid uuid.UUID) (db.Investment, error) {
		inv, err := qtx.CreateInvestment(ctx, db.CreateInvestmentParams{
			HouseholdID:     hid,
			DisplayName:     p.DisplayName,
			Description:     p.Description,
			Subtype:         "mutual_fund",
			OwnershipType:   p.OwnershipType,
			SoleOwnerUserID: p.SoleOwnerUserID,
			NativeCurrency:  p.NativeCurrency,
			RiskProfile:     p.RiskProfile,
			CreatedBy:       &user,
		})
		if err != nil {
			return db.Investment{}, fmt.Errorf("create investment: %w", err)
		}
		details, err := qtx.CreateMutualFundDetails(ctx, db.CreateMutualFundDetailsParams{
			InvestmentID: inv.ID,
			FundCode:     p.FundCode,
			FundManager:  p.FundManager,
			FundType:     p.FundType,
		})
		if err != nil {
			return db.Investment{}, fmt.Errorf("create mutual_fund_details: %w", err)
		}
		out = &MutualFund{Investment: inv, Details: details}
		return inv, nil
	})
	if err != nil {
		return nil, err
	}
	out.Investment.TagID = tagID // reflect the assignment in the returned aggregate
	return out, nil
}

// CreateGoldWithSnapshotsAndLedger — see CreateStockWithSnapshotsAndLedger.
func (r *InvestmentRepo) CreateGoldWithSnapshotsAndLedger(ctx context.Context, p CreateGoldParams, tagID *uuid.UUID, snaps []ImportInvestmentSnapshotRow, ledger []ImportTransactionRow) (*Gold, error) {
	var out *Gold
	err := r.createInvestmentWithHistory(ctx, "gold", tagID, snaps, ledger, termBounds{}, func(ctx context.Context, qtx *db.Queries, user, hid uuid.UUID) (db.Investment, error) {
		inv, err := qtx.CreateInvestment(ctx, db.CreateInvestmentParams{
			HouseholdID:     hid,
			DisplayName:     p.DisplayName,
			Description:     p.Description,
			Subtype:         "gold",
			OwnershipType:   p.OwnershipType,
			SoleOwnerUserID: p.SoleOwnerUserID,
			NativeCurrency:  p.NativeCurrency,
			RiskProfile:     p.RiskProfile,
			CreatedBy:       &user,
		})
		if err != nil {
			return db.Investment{}, fmt.Errorf("create investment: %w", err)
		}
		details, err := qtx.CreateGoldDetails(ctx, db.CreateGoldDetailsParams{
			InvestmentID: inv.ID,
			Form:         p.Form,
			Purity:       p.Purity,
		})
		if err != nil {
			return db.Investment{}, fmt.Errorf("create gold_details: %w", err)
		}
		out = &Gold{Investment: inv, Details: details}
		return inv, nil
	})
	if err != nil {
		return nil, err
	}
	out.Investment.TagID = tagID // reflect the assignment in the returned aggregate
	return out, nil
}

// CreateBondWithSnapshotsAndLedger seeds a bond from an import. Unlike CreateBond
// it does NOT auto-seed a placement Buy for govt_primary bonds — the uploaded
// ledger already carries every transaction (including any placement Buy), so
// re-seeding it here would double the cost basis.
func (r *InvestmentRepo) CreateBondWithSnapshotsAndLedger(ctx context.Context, p CreateBondParams, tagID *uuid.UUID, snaps []ImportInvestmentSnapshotRow, ledger []ImportTransactionRow) (*Bond, error) {
	var out *Bond
	err := r.createInvestmentWithHistory(ctx, "bond", tagID, snaps, ledger, termBounds{}, func(ctx context.Context, qtx *db.Queries, user, hid uuid.UUID) (db.Investment, error) {
		inv, err := qtx.CreateInvestment(ctx, db.CreateInvestmentParams{
			HouseholdID:     hid,
			DisplayName:     p.DisplayName,
			Description:     p.Description,
			Subtype:         "bond",
			OwnershipType:   p.OwnershipType,
			SoleOwnerUserID: p.SoleOwnerUserID,
			NativeCurrency:  p.NativeCurrency,
			RiskProfile:     p.RiskProfile,
			CreatedBy:       &user,
		})
		if err != nil {
			return db.Investment{}, fmt.Errorf("create investment: %w", err)
		}
		details, err := qtx.CreateBondDetails(ctx, db.CreateBondDetailsParams{
			InvestmentID:    inv.ID,
			BondType:        p.BondType,
			SeriesCode:      p.SeriesCode,
			Issuer:          p.Issuer,
			CouponRate:      p.CouponRate,
			CouponFrequency: p.CouponFrequency,
			MaturityDate:    p.MaturityDate,
		})
		if err != nil {
			return db.Investment{}, fmt.Errorf("create bond_details: %w", err)
		}
		out = &Bond{Investment: inv, Details: details}
		return inv, nil
	})
	if err != nil {
		return nil, err
	}
	out.Investment.TagID = tagID // reflect the assignment in the returned aggregate
	return out, nil
}

// CreateTimeDepositWithSnapshotsAndLedger — see CreateStockWithSnapshotsAndLedger.
// A TimeDeposit's only allowed ledger type is Maturity (the subtype matrix), so a
// matured deposit round-trips with its single Maturity row applied here.
func (r *InvestmentRepo) CreateTimeDepositWithSnapshotsAndLedger(ctx context.Context, p CreateTimeDepositParams, tagID *uuid.UUID, snaps []ImportInvestmentSnapshotRow, ledger []ImportTransactionRow) (*TimeDeposit, error) {
	if !p.MaturityDate.After(p.PlacementDate) {
		return nil, ErrInvalidDepositTerm
	}
	var out *TimeDeposit
	bounds := termBounds{placement: p.PlacementDate, maturity: p.MaturityDate}
	err := r.createInvestmentWithHistory(ctx, "time_deposit", tagID, snaps, ledger, bounds, func(ctx context.Context, qtx *db.Queries, user, hid uuid.UUID) (db.Investment, error) {
		inv, err := qtx.CreateInvestment(ctx, db.CreateInvestmentParams{
			HouseholdID:     hid,
			DisplayName:     p.DisplayName,
			Description:     p.Description,
			Subtype:         "time_deposit",
			OwnershipType:   p.OwnershipType,
			SoleOwnerUserID: p.SoleOwnerUserID,
			NativeCurrency:  p.NativeCurrency,
			RiskProfile:     p.RiskProfile,
			CreatedBy:       &user,
		})
		if err != nil {
			return db.Investment{}, fmt.Errorf("create investment: %w", err)
		}
		details, err := qtx.CreateTimeDepositDetails(ctx, db.CreateTimeDepositDetailsParams{
			InvestmentID:   inv.ID,
			BankName:       p.BankName,
			Principal:      p.Principal,
			InterestRate:   p.InterestRate,
			TermMonths:     p.TermMonths,
			PlacementDate:  p.PlacementDate,
			MaturityDate:   p.MaturityDate,
			RolloverPolicy: p.RolloverPolicy,
		})
		if err != nil {
			return db.Investment{}, fmt.Errorf("create time_deposit_details: %w", err)
		}
		out = &TimeDeposit{Investment: inv, Details: details}
		return inv, nil
	})
	if err != nil {
		return nil, err
	}
	out.Investment.TagID = tagID // reflect the assignment in the returned aggregate
	return out, nil
}

// createInvestmentWithHistory runs the shared create-from-list commit: it opens
// one transaction, lets createCore create the core investment + subtype details
// (returning the row + its native currency), assigns the resolved tag, seeds the
// snapshots, and seeds the ledger — atomically. A mid-batch failure rolls the
// whole thing back, so a position is never left half-seeded.
func (r *InvestmentRepo) createInvestmentWithHistory(
	ctx context.Context,
	subtype string,
	tagID *uuid.UUID,
	snaps []ImportInvestmentSnapshotRow,
	ledger []ImportTransactionRow,
	bounds termBounds,
	createCore func(ctx context.Context, qtx *db.Queries, user, hid uuid.UUID) (db.Investment, error),
) error {
	user, hid, err := currentUser(ctx)
	if err != nil {
		return err
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := r.q.WithTx(tx)

	inv, err := createCore(ctx, qtx, user, hid)
	if err != nil {
		return err
	}

	if tagID != nil {
		if _, err := qtx.AssignInvestmentTag(ctx, db.AssignInvestmentTagParams{
			TagID:       tagID,
			ID:          inv.ID,
			HouseholdID: hid,
			UpdatedBy:   &user,
		}); err != nil {
			return fmt.Errorf("assign tag: %w", err)
		}
	}

	if err := seedSnapshots(ctx, qtx, inv.ID, subtype, snaps, bounds, user, hid); err != nil {
		return err
	}
	if err := seedLedger(ctx, qtx, inv.ID, ledger, bounds, user, hid); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// seedSnapshots upserts the seeded snapshot rows inside an existing tx, validating
// each row's value-shape against the subtype first (the DB CHECK is the backstop).
func seedSnapshots(ctx context.Context, qtx *db.Queries, invID uuid.UUID, subtype string, snaps []ImportInvestmentSnapshotRow, bounds termBounds, user, hid uuid.UUID) error {
	for _, row := range snaps {
		if err := validateInvestmentSnapshotShape(subtype, row.Quantity, row.PricePerUnit, row.AccruedInterest); err != nil {
			return err
		}
		// A seeded time deposit's readings are confined to its term (issue #62);
		// unbounded for every other subtype.
		if err := bounds.checkSnapshotMonth(row.YearMonth); err != nil {
			return fmt.Errorf("seed snapshot %s: %w", row.YearMonth.Format("2006-01"), err)
		}
		if _, err := qtx.UpsertInvestmentSnapshot(ctx, db.UpsertInvestmentSnapshotParams{
			ID:              invID,
			YearMonth:       row.YearMonth,
			Amount:          row.Amount,
			Currency:        row.Currency,
			Quantity:        row.Quantity,
			PricePerUnit:    row.PricePerUnit,
			AccruedInterest: row.AccruedInterest,
			AsOfDate:        row.AsOfDate,
			Description:     row.Description,
			CreatedBy:       &user,
			HouseholdID:     hid,
		}); err != nil {
			return fmt.Errorf("seed snapshot %s: %w", row.YearMonth.Format("2006-01"), err)
		}
	}
	return nil
}

// seedLedger inserts the seeded ledger inside an existing tx, applying any
// Maturity row LAST (decision (b), #90) so the seed write-order matches the
// terminal-event model. A Maturity additionally flips the position to 'matured',
// sets terminated_at, and upserts the 0-value close snapshot — the exact terminal
// behavior of CreateInvestmentTransaction, reproduced here against the shared qtx.
// The 0 close wins over any seeded snapshot in the maturity month because
// seedSnapshots has already run (createInvestmentWithHistory orders them).
func seedLedger(ctx context.Context, qtx *db.Queries, invID uuid.UUID, ledger []ImportTransactionRow, bounds termBounds, user, hid uuid.UUID) error {
	ordered := make([]ImportTransactionRow, len(ledger))
	copy(ordered, ledger)
	sort.SliceStable(ordered, func(i, j int) bool {
		// Maturity sinks to the end; everything else keeps file order.
		return ordered[i].TransactionType != TxnTypeMaturity && ordered[j].TransactionType == TxnTypeMaturity
	})

	for _, row := range ordered {
		// A seeded time deposit's only ledger row, Maturity, must land inside the
		// term (issue #62); unbounded for every other subtype.
		if err := bounds.checkTransactionDate(row.TransactionDate); err != nil {
			return fmt.Errorf("seed transaction (%s): %w", row.TransactionType, err)
		}
		if _, err := qtx.CreateInvestmentTransaction(ctx, db.CreateInvestmentTransactionParams{
			ID:                   invID,
			TransactionType:      row.TransactionType,
			TransactionDate:      row.TransactionDate,
			Currency:             row.Currency,
			Description:          row.Description,
			Amount:               row.Amount,
			Quantity:             row.Quantity,
			PricePerUnit:         row.PricePerUnit,
			PrincipalAmount:      row.PrincipalAmount,
			InterestAmount:       row.InterestAmount,
			PrincipalDisposition: row.PrincipalDisposition,
			InterestDisposition:  row.InterestDisposition,
			CreatedBy:            &user,
			HouseholdID:          hid,
		}); err != nil {
			return fmt.Errorf("seed transaction (%s): %w", row.TransactionType, err)
		}

		if row.TransactionType != TxnTypeMaturity {
			continue
		}
		// Terminal flip + truthful 0 close snapshot (mirrors
		// CreateInvestmentTransaction; ADR-0009 / issue #25).
		termDate := row.TransactionDate
		if _, err := qtx.UpdateInvestmentLifecycle(ctx, db.UpdateInvestmentLifecycleParams{
			ID:           invID,
			HouseholdID:  hid,
			Status:       StatusMatured,
			TerminatedAt: &termDate,
			UpdatedBy:    &user,
		}); err != nil {
			return fmt.Errorf("seed maturity: flip to matured: %w", err)
		}
		zero := decimal.Zero
		ym := time.Date(row.TransactionDate.Year(), row.TransactionDate.Month(), 1, 0, 0, 0, 0, time.UTC)
		asOf := row.TransactionDate
		if _, err := qtx.UpsertInvestmentSnapshot(ctx, db.UpsertInvestmentSnapshotParams{
			ID:              invID,
			YearMonth:       ym,
			Amount:          zero,
			Currency:        row.Currency,
			AccruedInterest: &zero,
			AsOfDate:        &asOf,
			CreatedBy:       &user,
			HouseholdID:     hid,
		}); err != nil {
			return fmt.Errorf("seed maturity: close snapshot: %w", err)
		}
	}
	return nil
}
