package repo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"

	"github.com/kerti/balances-v2/backend/internal/db"
)

// CouponDispositionPaysOut is the default coupon disposition (#66): coupons pay
// out to the bank account, no in-instrument accrual — the common Indonesian
// govt-primary case and the historical (pre-#66) behaviour.
const CouponDispositionPaysOut = "pays_out"

// defaultCouponDisposition backfills an empty disposition to the column default,
// so callers that omit it (older API clients, import templates predating #66)
// never push an empty string into the NOT NULL/CHECK-constrained column.
func defaultCouponDisposition(s string) string {
	if s == "" {
		return CouponDispositionPaysOut
	}
	return s
}

// Bond is the aggregate returned by Get/Create — the core investment row
// joined with its bond_details extension.
type Bond struct {
	Investment db.Investment `json:"investment"`
	Details    db.BondDetail `json:"details"`
	// OutstandingFace is the held nominal derived from the ledger (issue #27):
	// (Σ buy_qty − Σ sell_qty) × 1,000,000. Replaces the dropped
	// bond_details.face_value scalar.
	OutstandingFace decimal.Decimal `json:"outstanding_face"`
}

type BondListItem struct {
	Investment     db.Investment          `json:"investment"`
	Details        db.BondDetail          `json:"details"`
	LatestSnapshot *db.InvestmentSnapshot `json:"latest_snapshot"`
	// CostBasis is the avg-cost ledger replay (Σ amount, net of sells) — every
	// bond now carries a Buy at placement (issue #27), so it always replays;
	// mirrors lib/costBasis.ts (issue #18).
	CostBasis decimal.Decimal `json:"cost_basis"`
	// OutstandingFace is the held nominal derived from the ledger (issue #27).
	OutstandingFace decimal.Decimal `json:"outstanding_face"`
	// Ledger summary for the row (issue #67). LastTransactionDate is
	// YYYY-MM-DD, nil when there are none.
	TransactionCount    int     `json:"transaction_count"`
	LastTransactionDate *string `json:"last_transaction_date"`
}

type CreateBondParams struct {
	DisplayName     string
	Description     *string
	OwnershipType   string
	SoleOwnerUserID *uuid.UUID
	NativeCurrency  string
	RiskProfile     string
	BondType        string // "govt_primary" | "secondary_market"
	SeriesCode      *string
	Issuer          string
	CouponRate      decimal.Decimal
	CouponFrequency string // "monthly" | "quarterly" | "semi_annual" | "annual"
	// CouponDisposition is whether the coupon pays out to the bank account or
	// accrues inside the instrument (#66): "pays_out" | "accrues".
	CouponDisposition string
	MaturityDate      time.Time
	// FaceValue + PlacementDate seed the first Buy for a govt_primary bond
	// (issue #27): qty = FaceValue / 1,000,000, price_per_unit = 1,000,000 at
	// par. Ignored for secondary_market, where the user records the actual Buy
	// (with the real price) themselves. nil for secondary_market.
	FaceValue     *decimal.Decimal
	PlacementDate *time.Time
}

type UpdateBondParams struct {
	DisplayName       string
	Description       *string
	OwnershipType     string
	SoleOwnerUserID   *uuid.UUID
	RiskProfile       string
	BondType          string
	SeriesCode        *string
	Issuer            string
	CouponRate        decimal.Decimal
	CouponFrequency   string
	CouponDisposition string
	MaturityDate      time.Time
}

func (r *InvestmentRepo) CreateBond(ctx context.Context, p CreateBondParams) (*Bond, error) {
	user, hid, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	p.CouponDisposition = defaultCouponDisposition(p.CouponDisposition)

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	qtx := r.q.WithTx(tx)
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
		return nil, fmt.Errorf("create investment: %w", err)
	}

	details, err := qtx.CreateBondDetails(ctx, db.CreateBondDetailsParams{
		InvestmentID:      inv.ID,
		BondType:          p.BondType,
		SeriesCode:        p.SeriesCode,
		Issuer:            p.Issuer,
		CouponRate:        p.CouponRate,
		CouponFrequency:   p.CouponFrequency,
		CouponDisposition: p.CouponDisposition,
		MaturityDate:      p.MaturityDate,
	})
	if err != nil {
		return nil, fmt.Errorf("create bond_details: %w", err)
	}

	// Seed the placement Buy for a govt_primary bond (issue #27): capital placed
	// at issuance is a cash_in transaction, never investment return. Recording it
	// as a Buy makes the placement-month snapshot 0→nominal net to 0 return and
	// lets outstanding nominal + cost basis derive from the ledger. At par:
	// qty = nominal / 1,000,000, price_per_unit = 1,000,000, amount = nominal.
	// secondary_market bonds are not seeded — the user records the real Buy.
	outstandingFace := decimal.Zero
	if p.BondType == "govt_primary" && p.FaceValue != nil && p.PlacementDate != nil {
		qty := p.FaceValue.Div(bondFaceUnit)
		amount := *p.FaceValue
		price := bondFaceUnit
		if _, err := qtx.CreateInvestmentTransaction(ctx, db.CreateInvestmentTransactionParams{
			ID:              inv.ID,
			TransactionType: TxnTypeBuy,
			TransactionDate: *p.PlacementDate,
			Currency:        p.NativeCurrency,
			Amount:          &amount,
			Quantity:        &qty,
			PricePerUnit:    &price,
			CreatedBy:       &user,
			HouseholdID:     hid,
		}); err != nil {
			return nil, fmt.Errorf("seed placement buy: %w", err)
		}
		outstandingFace = amount
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return &Bond{Investment: inv, Details: details, OutstandingFace: outstandingFace}, nil
}

func (r *InvestmentRepo) GetBond(ctx context.Context, id uuid.UUID) (*Bond, error) {
	_, hid, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}

	inv, err := r.q.GetInvestmentByID(ctx, db.GetInvestmentByIDParams{ID: id, HouseholdID: hid})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if inv.Subtype != "bond" {
		return nil, ErrNotFound
	}

	details, err := r.q.GetBondDetailsByInvestmentID(ctx, inv.ID)
	if err != nil {
		return nil, fmt.Errorf("get bond_details: %w", err)
	}

	ledger, err := r.q.ListInvestmentTransactionsByInvestmentIDs(ctx, []uuid.UUID{inv.ID})
	if err != nil {
		return nil, fmt.Errorf("list bond transactions: %w", err)
	}

	return &Bond{Investment: inv, Details: details, OutstandingFace: outstandingFaceFromLedger(ledger)}, nil
}

func (r *InvestmentRepo) ListBonds(ctx context.Context) ([]BondListItem, error) {
	_, hid, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}

	subtype := "bond"
	invs, err := r.q.ListInvestmentsByHousehold(ctx, db.ListInvestmentsByHouseholdParams{
		HouseholdID: hid,
		Subtype:     &subtype,
	})
	if err != nil {
		return nil, fmt.Errorf("list investments: %w", err)
	}
	if len(invs) == 0 {
		return []BondListItem{}, nil
	}

	ids := make([]uuid.UUID, len(invs))
	for i, x := range invs {
		ids[i] = x.ID
	}

	details, err := r.q.ListBondDetailsByInvestmentIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("list bond_details: %w", err)
	}
	detailByID := make(map[uuid.UUID]db.BondDetail, len(details))
	for _, d := range details {
		detailByID[d.InvestmentID] = d
	}

	snapshots, err := r.q.ListLatestInvestmentSnapshotsByInvestmentIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("list latest investment snapshots: %w", err)
	}
	snapByID := make(map[uuid.UUID]db.InvestmentSnapshot, len(snapshots))
	for _, s := range snapshots {
		snapByID[s.InvestmentID] = s
	}

	txns, err := r.q.ListInvestmentTransactionsByInvestmentIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("list investment transactions: %w", err)
	}
	txnByID := groupTransactionsByInvestment(txns)

	out := make([]BondListItem, 0, len(invs))
	for _, x := range invs {
		ledger := txnByID[x.ID]
		count, lastDate := transactionAggregates(ledger)
		item := BondListItem{
			Investment:          x,
			Details:             detailByID[x.ID],
			CostBasis:           costBasisFromLedger(ledger),
			OutstandingFace:     outstandingFaceFromLedger(ledger),
			TransactionCount:    count,
			LastTransactionDate: lastDate,
		}
		if s, ok := snapByID[x.ID]; ok {
			s := s
			item.LatestSnapshot = &s
		}
		out = append(out, item)
	}
	return out, nil
}

func (r *InvestmentRepo) UpdateBond(ctx context.Context, id uuid.UUID, p UpdateBondParams) (*Bond, error) {
	user, hid, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}

	p.CouponDisposition = defaultCouponDisposition(p.CouponDisposition)

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	qtx := r.q.WithTx(tx)
	inv, err := qtx.UpdateInvestment(ctx, db.UpdateInvestmentParams{
		ID:              id,
		HouseholdID:     hid,
		DisplayName:     p.DisplayName,
		Description:     p.Description,
		OwnershipType:   p.OwnershipType,
		SoleOwnerUserID: p.SoleOwnerUserID,
		RiskProfile:     p.RiskProfile,
		UpdatedBy:       &user,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("update investment: %w", err)
	}
	if inv.Subtype != "bond" {
		return nil, ErrNotFound
	}

	details, err := qtx.UpdateBondDetails(ctx, db.UpdateBondDetailsParams{
		InvestmentID:      inv.ID,
		BondType:          p.BondType,
		SeriesCode:        p.SeriesCode,
		Issuer:            p.Issuer,
		CouponRate:        p.CouponRate,
		CouponFrequency:   p.CouponFrequency,
		CouponDisposition: p.CouponDisposition,
		MaturityDate:      p.MaturityDate,
	})
	if err != nil {
		return nil, fmt.Errorf("update bond_details: %w", err)
	}

	ledger, err := qtx.ListInvestmentTransactionsByInvestmentIDs(ctx, []uuid.UUID{inv.ID})
	if err != nil {
		return nil, fmt.Errorf("list bond transactions: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return &Bond{Investment: inv, Details: details, OutstandingFace: outstandingFaceFromLedger(ledger)}, nil
}

func (r *InvestmentRepo) DeleteBond(ctx context.Context, id uuid.UUID) error {
	if _, err := r.GetBond(ctx, id); err != nil {
		return err
	}
	return r.softDeleteInvestment(ctx, id)
}
