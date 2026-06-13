package repo

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"

	"github.com/kerti/balances-v2/backend/internal/db"
)

// Gold is the aggregate returned by Get/Create — the core investment row
// joined with its gold_details extension.
type Gold struct {
	Investment db.Investment `json:"investment"`
	Details    db.GoldDetail `json:"details"`
}

type GoldListItem struct {
	Investment     db.Investment          `json:"investment"`
	Details        db.GoldDetail          `json:"details"`
	LatestSnapshot *db.InvestmentSnapshot `json:"latest_snapshot"`
	// CostBasis is the avg-cost ledger replay (issue #18). See
	// costBasisFromLedger.
	CostBasis decimal.Decimal `json:"cost_basis"`
	// Ledger summary for the row (issue #67). LastTransactionDate is
	// YYYY-MM-DD, nil when there are none.
	TransactionCount    int     `json:"transaction_count"`
	LastTransactionDate *string `json:"last_transaction_date"`
}

type CreateGoldParams struct {
	DisplayName     string
	Description     *string
	OwnershipType   string
	SoleOwnerUserID *uuid.UUID
	NativeCurrency  string
	RiskProfile     string
	Form            string // "bar" | "coin" | "digital" | "jewelry"
	Purity          decimal.Decimal
}

type UpdateGoldParams struct {
	DisplayName     string
	Description     *string
	OwnershipType   string
	SoleOwnerUserID *uuid.UUID
	RiskProfile     string
	Form            string
	Purity          decimal.Decimal
}

func (r *InvestmentRepo) CreateGold(ctx context.Context, p CreateGoldParams) (*Gold, error) {
	user, hid, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}

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
		Subtype:         "gold",
		OwnershipType:   p.OwnershipType,
		SoleOwnerUserID: p.SoleOwnerUserID,
		NativeCurrency:  p.NativeCurrency,
		RiskProfile:     p.RiskProfile,
		CreatedBy:       &user,
	})
	if err != nil {
		return nil, fmt.Errorf("create investment: %w", err)
	}

	details, err := qtx.CreateGoldDetails(ctx, db.CreateGoldDetailsParams{
		InvestmentID: inv.ID,
		Form:         p.Form,
		Purity:       p.Purity,
	})
	if err != nil {
		return nil, fmt.Errorf("create gold_details: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return &Gold{Investment: inv, Details: details}, nil
}

func (r *InvestmentRepo) GetGold(ctx context.Context, id uuid.UUID) (*Gold, error) {
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
	if inv.Subtype != "gold" {
		return nil, ErrNotFound
	}

	details, err := r.q.GetGoldDetailsByInvestmentID(ctx, inv.ID)
	if err != nil {
		return nil, fmt.Errorf("get gold_details: %w", err)
	}

	return &Gold{Investment: inv, Details: details}, nil
}

func (r *InvestmentRepo) ListGolds(ctx context.Context) ([]GoldListItem, error) {
	_, hid, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}

	subtype := "gold"
	invs, err := r.q.ListInvestmentsByHousehold(ctx, db.ListInvestmentsByHouseholdParams{
		HouseholdID: hid,
		Subtype:     &subtype,
	})
	if err != nil {
		return nil, fmt.Errorf("list investments: %w", err)
	}
	if len(invs) == 0 {
		return []GoldListItem{}, nil
	}

	ids := make([]uuid.UUID, len(invs))
	for i, x := range invs {
		ids[i] = x.ID
	}

	details, err := r.q.ListGoldDetailsByInvestmentIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("list gold_details: %w", err)
	}
	detailByID := make(map[uuid.UUID]db.GoldDetail, len(details))
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

	out := make([]GoldListItem, 0, len(invs))
	for _, x := range invs {
		ledger := txnByID[x.ID]
		count, lastDate := transactionAggregates(ledger)
		item := GoldListItem{
			Investment:          x,
			Details:             detailByID[x.ID],
			CostBasis:           costBasisFromLedger(ledger),
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

func (r *InvestmentRepo) UpdateGold(ctx context.Context, id uuid.UUID, p UpdateGoldParams) (*Gold, error) {
	user, hid, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}

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
	if inv.Subtype != "gold" {
		return nil, ErrNotFound
	}

	details, err := qtx.UpdateGoldDetails(ctx, db.UpdateGoldDetailsParams{
		InvestmentID: inv.ID,
		Form:         p.Form,
		Purity:       p.Purity,
	})
	if err != nil {
		return nil, fmt.Errorf("update gold_details: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return &Gold{Investment: inv, Details: details}, nil
}

func (r *InvestmentRepo) DeleteGold(ctx context.Context, id uuid.UUID) error {
	if _, err := r.GetGold(ctx, id); err != nil {
		return err
	}
	return r.softDeleteInvestment(ctx, id)
}
