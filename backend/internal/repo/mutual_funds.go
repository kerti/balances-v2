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

// MutualFund is the aggregate returned by Get/Create — the core investment
// row joined with its mutual_fund_details extension.
type MutualFund struct {
	Investment db.Investment       `json:"investment"`
	Details    db.MutualFundDetail `json:"details"`
}

type MutualFundListItem struct {
	Investment     db.Investment          `json:"investment"`
	Details        db.MutualFundDetail    `json:"details"`
	LatestSnapshot *db.InvestmentSnapshot `json:"latest_snapshot"`
	// CostBasis is the avg-cost ledger replay (issue #18). See
	// costBasisFromLedger.
	CostBasis decimal.Decimal `json:"cost_basis"`
	// Ledger summary for the row (issue #67). LastTransactionDate is
	// YYYY-MM-DD, nil when there are none.
	TransactionCount    int     `json:"transaction_count"`
	LastTransactionDate *string `json:"last_transaction_date"`
}

type CreateMutualFundParams struct {
	DisplayName     string
	Description     *string
	OwnershipType   string
	SoleOwnerUserID *uuid.UUID
	NativeCurrency  string
	RiskProfile     string
	FundCode        string
	FundManager     *string
	FundType        string
}

type UpdateMutualFundParams struct {
	DisplayName     string
	Description     *string
	OwnershipType   string
	SoleOwnerUserID *uuid.UUID
	RiskProfile     string
	FundCode        string
	FundManager     *string
	FundType        string
}

func (r *InvestmentRepo) CreateMutualFund(ctx context.Context, p CreateMutualFundParams) (*MutualFund, error) {
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
		Subtype:         "mutual_fund",
		OwnershipType:   p.OwnershipType,
		SoleOwnerUserID: p.SoleOwnerUserID,
		NativeCurrency:  p.NativeCurrency,
		RiskProfile:     p.RiskProfile,
		CreatedBy:       &user,
	})
	if err != nil {
		return nil, fmt.Errorf("create investment: %w", err)
	}

	details, err := qtx.CreateMutualFundDetails(ctx, db.CreateMutualFundDetailsParams{
		InvestmentID: inv.ID,
		FundCode:     p.FundCode,
		FundManager:  p.FundManager,
		FundType:     p.FundType,
	})
	if err != nil {
		return nil, fmt.Errorf("create mutual_fund_details: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return &MutualFund{Investment: inv, Details: details}, nil
}

func (r *InvestmentRepo) GetMutualFund(ctx context.Context, id uuid.UUID) (*MutualFund, error) {
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
	if inv.Subtype != "mutual_fund" {
		return nil, ErrNotFound
	}

	details, err := r.q.GetMutualFundDetailsByInvestmentID(ctx, inv.ID)
	if err != nil {
		return nil, fmt.Errorf("get mutual_fund_details: %w", err)
	}

	return &MutualFund{Investment: inv, Details: details}, nil
}

func (r *InvestmentRepo) ListMutualFunds(ctx context.Context) ([]MutualFundListItem, error) {
	_, hid, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}

	subtype := "mutual_fund"
	invs, err := r.q.ListInvestmentsByHousehold(ctx, db.ListInvestmentsByHouseholdParams{
		HouseholdID: hid,
		Subtype:     &subtype,
	})
	if err != nil {
		return nil, fmt.Errorf("list investments: %w", err)
	}
	if len(invs) == 0 {
		return []MutualFundListItem{}, nil
	}

	ids := make([]uuid.UUID, len(invs))
	for i, x := range invs {
		ids[i] = x.ID
	}

	details, err := r.q.ListMutualFundDetailsByInvestmentIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("list mutual_fund_details: %w", err)
	}
	detailByID := make(map[uuid.UUID]db.MutualFundDetail, len(details))
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

	out := make([]MutualFundListItem, 0, len(invs))
	for _, x := range invs {
		ledger := txnByID[x.ID]
		count, lastDate := transactionAggregates(ledger)
		item := MutualFundListItem{
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

func (r *InvestmentRepo) UpdateMutualFund(ctx context.Context, id uuid.UUID, p UpdateMutualFundParams) (*MutualFund, error) {
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
	if inv.Subtype != "mutual_fund" {
		return nil, ErrNotFound
	}

	details, err := qtx.UpdateMutualFundDetails(ctx, db.UpdateMutualFundDetailsParams{
		InvestmentID: inv.ID,
		FundCode:     p.FundCode,
		FundManager:  p.FundManager,
		FundType:     p.FundType,
	})
	if err != nil {
		return nil, fmt.Errorf("update mutual_fund_details: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return &MutualFund{Investment: inv, Details: details}, nil
}

func (r *InvestmentRepo) DeleteMutualFund(ctx context.Context, id uuid.UUID) error {
	if _, err := r.GetMutualFund(ctx, id); err != nil {
		return err
	}
	return r.softDeleteInvestment(ctx, id)
}
