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

// Stock is the aggregate returned by Get/Create — the core investment row
// joined with its stock_details extension.
type Stock struct {
	Investment db.Investment  `json:"investment"`
	Details    db.StockDetail `json:"details"`
}

// StockListItem extends Stock with the most-recent Snapshot if any (for list
// views that want to display the current value).
type StockListItem struct {
	Investment     db.Investment          `json:"investment"`
	Details        db.StockDetail         `json:"details"`
	LatestSnapshot *db.InvestmentSnapshot `json:"latest_snapshot"`
	// CostBasis is the avg-cost ledger replay (issue #18) so the list
	// payload is self-contained — the headline P/L needs no per-position
	// transaction fetch. See costBasisFromLedger.
	CostBasis decimal.Decimal `json:"cost_basis"`
	// TransactionCount / LastTransactionDate summarise the ledger for the row
	// (issue #67). Both derive from the same batch already loaded for cost
	// basis. LastTransactionDate is YYYY-MM-DD, nil when there are none.
	TransactionCount    int     `json:"transaction_count"`
	LastTransactionDate *string `json:"last_transaction_date"`
}

type CreateStockParams struct {
	DisplayName     string
	Description     *string
	OwnershipType   string // "sole" or "joint"
	SoleOwnerUserID *uuid.UUID
	NativeCurrency  string
	RiskProfile     string // "low" | "medium" | "high" — migration 00018 CHECK
	Ticker          string
	Exchange        string
}

type UpdateStockParams struct {
	DisplayName     string
	Description     *string
	OwnershipType   string
	SoleOwnerUserID *uuid.UUID
	RiskProfile     string
	Ticker          string
	Exchange        string
}

func (r *InvestmentRepo) CreateStock(ctx context.Context, p CreateStockParams) (*Stock, error) {
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
		Subtype:         "stock",
		OwnershipType:   p.OwnershipType,
		SoleOwnerUserID: p.SoleOwnerUserID,
		NativeCurrency:  p.NativeCurrency,
		RiskProfile:     p.RiskProfile,
		CreatedBy:       &user,
	})
	if err != nil {
		return nil, fmt.Errorf("create investment: %w", err)
	}

	details, err := qtx.CreateStockDetails(ctx, db.CreateStockDetailsParams{
		InvestmentID: inv.ID,
		Ticker:       p.Ticker,
		Exchange:     p.Exchange,
	})
	if err != nil {
		return nil, fmt.Errorf("create stock_details: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return &Stock{Investment: inv, Details: details}, nil
}

func (r *InvestmentRepo) GetStock(ctx context.Context, id uuid.UUID) (*Stock, error) {
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
	if inv.Subtype != "stock" {
		return nil, ErrNotFound
	}

	details, err := r.q.GetStockDetailsByInvestmentID(ctx, inv.ID)
	if err != nil {
		return nil, fmt.Errorf("get stock_details: %w", err)
	}

	return &Stock{Investment: inv, Details: details}, nil
}

func (r *InvestmentRepo) ListStocks(ctx context.Context) ([]StockListItem, error) {
	_, hid, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}

	subtype := "stock"
	invs, err := r.q.ListInvestmentsByHousehold(ctx, db.ListInvestmentsByHouseholdParams{
		HouseholdID: hid,
		Subtype:     &subtype,
	})
	if err != nil {
		return nil, fmt.Errorf("list investments: %w", err)
	}
	if len(invs) == 0 {
		return []StockListItem{}, nil
	}

	ids := make([]uuid.UUID, len(invs))
	for i, x := range invs {
		ids[i] = x.ID
	}

	details, err := r.q.ListStockDetailsByInvestmentIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("list stock_details: %w", err)
	}
	detailByID := make(map[uuid.UUID]db.StockDetail, len(details))
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

	out := make([]StockListItem, 0, len(invs))
	for _, x := range invs {
		ledger := txnByID[x.ID]
		count, lastDate := transactionAggregates(ledger)
		item := StockListItem{
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

func (r *InvestmentRepo) UpdateStock(ctx context.Context, id uuid.UUID, p UpdateStockParams) (*Stock, error) {
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
	if inv.Subtype != "stock" {
		return nil, ErrNotFound
	}

	details, err := qtx.UpdateStockDetails(ctx, db.UpdateStockDetailsParams{
		InvestmentID: inv.ID,
		Ticker:       p.Ticker,
		Exchange:     p.Exchange,
	})
	if err != nil {
		return nil, fmt.Errorf("update stock_details: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return &Stock{Investment: inv, Details: details}, nil
}

func (r *InvestmentRepo) DeleteStock(ctx context.Context, id uuid.UUID) error {
	if _, err := r.GetStock(ctx, id); err != nil {
		return err
	}
	return r.softDeleteInvestment(ctx, id)
}
