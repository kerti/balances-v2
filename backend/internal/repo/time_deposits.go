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

// RolloverRef is the minimal pointer to a neighbour in a rollover chain — just
// enough to render a clickable link on the TD detail screen (issue #29).
type RolloverRef struct {
	ID          uuid.UUID `json:"id"`
	DisplayName string    `json:"display_name"`
}

// TimeDeposit is the aggregate returned by Get/Create — the core investment
// row joined with its time_deposit_details extension.
type TimeDeposit struct {
	Investment db.Investment        `json:"investment"`
	Details    db.TimeDepositDetail `json:"details"`
	// RolledFrom / RolledTo are the immediate rollover-chain neighbours, derived
	// (not stored): RolledFrom is the matured deposit this one redeployed (from
	// the stored rolled_from_investment_id); RolledTo is the live deposit rolled
	// over from this one. The detail screen renders both as links, and a non-nil
	// RolledTo suppresses the rollover callout (issue #29). Both nil on Create.
	RolledFrom *RolloverRef `json:"rolled_from"`
	RolledTo   *RolloverRef `json:"rolled_to"`
}

type TimeDepositListItem struct {
	Investment     db.Investment          `json:"investment"`
	Details        db.TimeDepositDetail   `json:"details"`
	LatestSnapshot *db.InvestmentSnapshot `json:"latest_snapshot"`
	// CostBasis is the principal directly — a TD ledger holds only the
	// terminal Maturity transaction, never buys (issue #18).
	CostBasis decimal.Decimal `json:"cost_basis"`
	// Ledger summary for the row (issue #67). A TD ledger holds at most the
	// terminal Maturity, so this is 0 or 1. LastTransactionDate is YYYY-MM-DD,
	// nil when there are none.
	TransactionCount    int     `json:"transaction_count"`
	LastTransactionDate *string `json:"last_transaction_date"`
}

type CreateTimeDepositParams struct {
	DisplayName     string
	Description     *string
	OwnershipType   string
	SoleOwnerUserID *uuid.UUID
	NativeCurrency  string
	RiskProfile     string
	BankName        string
	Principal       decimal.Decimal
	InterestRate    decimal.Decimal
	TermMonths      int32
	PlacementDate   time.Time
	MaturityDate    time.Time
	RolloverPolicy  string // "auto_renew_principal" | "auto_renew_with_interest" | "no_rollover"
	// RolledFromInvestmentID links this deposit back to the matured position
	// whose funds it redeploys (issue #29). Nil for a fresh deposit.
	RolledFromInvestmentID *uuid.UUID
}

type UpdateTimeDepositParams struct {
	DisplayName     string
	Description     *string
	OwnershipType   string
	SoleOwnerUserID *uuid.UUID
	RiskProfile     string
	BankName        string
	Principal       decimal.Decimal
	InterestRate    decimal.Decimal
	TermMonths      int32
	PlacementDate   time.Time
	MaturityDate    time.Time
	RolloverPolicy  string
}

func (r *InvestmentRepo) CreateTimeDeposit(ctx context.Context, p CreateTimeDepositParams) (*TimeDeposit, error) {
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
	// Belt + suspenders: a rollover source must belong to this household, else a
	// crafted ID could flip another household's callout off (issue #29 / tenancy
	// convention). The FK guarantees existence; this guarantees ownership.
	if p.RolledFromInvestmentID != nil {
		if _, err := qtx.GetInvestmentByID(ctx, db.GetInvestmentByIDParams{
			ID:          *p.RolledFromInvestmentID,
			HouseholdID: hid,
		}); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, ErrNotFound
			}
			return nil, fmt.Errorf("verify rollover source: %w", err)
		}
	}
	inv, err := qtx.CreateInvestment(ctx, db.CreateInvestmentParams{
		HouseholdID:            hid,
		DisplayName:            p.DisplayName,
		Description:            p.Description,
		Subtype:                "time_deposit",
		OwnershipType:          p.OwnershipType,
		SoleOwnerUserID:        p.SoleOwnerUserID,
		NativeCurrency:         p.NativeCurrency,
		RiskProfile:            p.RiskProfile,
		RolledFromInvestmentID: p.RolledFromInvestmentID,
		CreatedBy:              &user,
	})
	if err != nil {
		return nil, fmt.Errorf("create investment: %w", err)
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
		return nil, fmt.Errorf("create time_deposit_details: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return &TimeDeposit{Investment: inv, Details: details}, nil
}

func (r *InvestmentRepo) GetTimeDeposit(ctx context.Context, id uuid.UUID) (*TimeDeposit, error) {
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
	if inv.Subtype != "time_deposit" {
		return nil, ErrNotFound
	}

	details, err := r.q.GetTimeDepositDetailsByInvestmentID(ctx, inv.ID)
	if err != nil {
		return nil, fmt.Errorf("get time_deposit_details: %w", err)
	}

	td := &TimeDeposit{Investment: inv, Details: details}

	// The deposit this one redeployed (if any). Household-scoped Get also drops a
	// soft-deleted or cross-tenant source defensively — a dangling link just
	// renders no "rolled over from" line rather than erroring.
	if inv.RolledFromInvestmentID != nil {
		src, err := r.q.GetInvestmentByID(ctx, db.GetInvestmentByIDParams{
			ID:          *inv.RolledFromInvestmentID,
			HouseholdID: hid,
		})
		switch {
		case err == nil:
			td.RolledFrom = &RolloverRef{ID: src.ID, DisplayName: src.DisplayName}
		case errors.Is(err, pgx.ErrNoRows):
			// dangling/cross-tenant source — leave RolledFrom nil
		default:
			return nil, fmt.Errorf("get rollover source: %w", err)
		}
	}

	// The live deposit rolled over from this one (if any).
	succ, err := r.q.GetRolloverSuccessor(ctx, db.GetRolloverSuccessorParams{
		RolledFromInvestmentID: &inv.ID,
		HouseholdID:            hid,
	})
	switch {
	case err == nil:
		td.RolledTo = &RolloverRef{ID: succ.ID, DisplayName: succ.DisplayName}
	case errors.Is(err, pgx.ErrNoRows):
		// no successor — leave RolledTo nil
	default:
		return nil, fmt.Errorf("get rollover successor: %w", err)
	}

	return td, nil
}

func (r *InvestmentRepo) ListTimeDeposits(ctx context.Context) ([]TimeDepositListItem, error) {
	_, hid, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}

	subtype := "time_deposit"
	invs, err := r.q.ListInvestmentsByHousehold(ctx, db.ListInvestmentsByHouseholdParams{
		HouseholdID: hid,
		Subtype:     &subtype,
	})
	if err != nil {
		return nil, fmt.Errorf("list investments: %w", err)
	}
	if len(invs) == 0 {
		return []TimeDepositListItem{}, nil
	}

	ids := make([]uuid.UUID, len(invs))
	for i, x := range invs {
		ids[i] = x.ID
	}

	details, err := r.q.ListTimeDepositDetailsByInvestmentIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("list time_deposit_details: %w", err)
	}
	detailByID := make(map[uuid.UUID]db.TimeDepositDetail, len(details))
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

	out := make([]TimeDepositListItem, 0, len(invs))
	for _, x := range invs {
		count, lastDate := transactionAggregates(txnByID[x.ID])
		item := TimeDepositListItem{
			Investment:          x,
			Details:             detailByID[x.ID],
			CostBasis:           detailByID[x.ID].Principal,
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

func (r *InvestmentRepo) UpdateTimeDeposit(ctx context.Context, id uuid.UUID, p UpdateTimeDepositParams) (*TimeDeposit, error) {
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
	if inv.Subtype != "time_deposit" {
		return nil, ErrNotFound
	}

	details, err := qtx.UpdateTimeDepositDetails(ctx, db.UpdateTimeDepositDetailsParams{
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
		return nil, fmt.Errorf("update time_deposit_details: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return &TimeDeposit{Investment: inv, Details: details}, nil
}

// LinkRolloverSuccessor records that the matured deposit sourceID rolled over
// into the existing deposit successorID, by stamping the successor's
// rolled_from_investment_id (issue #65) — the manual counterpart to the create
// path's RolledFromInvestmentID. Closes the gap where a hand-created successor
// stayed unlinked and the source kept nagging with the rollover callout.
//
// Both positions must be household-scoped time deposits. The link is rejected
// (ErrInvalidRolloverLink) when it would form an illegal chain: a self-link, a
// successor already rolled over from somewhere, a source that already has a
// successor (the chain is 1:1 by concept), or a direct cycle. Returns the
// source TD, now carrying the resolved RolledTo ref.
func (r *InvestmentRepo) LinkRolloverSuccessor(ctx context.Context, sourceID, successorID uuid.UUID) (*TimeDeposit, error) {
	user, hid, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	if sourceID == successorID {
		return nil, ErrInvalidRolloverLink
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	qtx := r.q.WithTx(tx)

	// Both ends must exist, be ours, and be time deposits. A cross-tenant or
	// wrong-subtype id is indistinguishable from a missing one (ErrNotFound).
	src, err := getOwnedTimeDeposit(ctx, qtx, sourceID, hid)
	if err != nil {
		return nil, err
	}
	succ, err := getOwnedTimeDeposit(ctx, qtx, successorID, hid)
	if err != nil {
		return nil, err
	}

	// The successor is already someone's rollover — don't silently re-point it.
	if succ.RolledFromInvestmentID != nil {
		return nil, ErrInvalidRolloverLink
	}
	// A direct cycle: the source itself rolled over from the successor.
	if src.RolledFromInvestmentID != nil && *src.RolledFromInvestmentID == successorID {
		return nil, ErrInvalidRolloverLink
	}
	// The source already has a successor — the chain is 1:1, so refuse a second.
	switch _, err := qtx.GetRolloverSuccessor(ctx, db.GetRolloverSuccessorParams{
		RolledFromInvestmentID: &sourceID,
		HouseholdID:            hid,
	}); {
	case err == nil:
		return nil, ErrInvalidRolloverLink
	case errors.Is(err, pgx.ErrNoRows):
		// no successor yet — good, proceed
	default:
		return nil, fmt.Errorf("check existing rollover successor: %w", err)
	}

	if _, err := qtx.SetRolloverSource(ctx, db.SetRolloverSourceParams{
		ID:                     successorID,
		HouseholdID:            hid,
		RolledFromInvestmentID: &sourceID,
		UpdatedBy:              &user,
	}); err != nil {
		return nil, fmt.Errorf("set rollover source: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	// Re-read the source so the response carries the freshly resolved RolledTo
	// ref (and the caller's rollover callout clears).
	return r.GetTimeDeposit(ctx, sourceID)
}

// getOwnedTimeDeposit fetches a household-scoped investment and asserts it is a
// time deposit, collapsing both "not yours / not found" and "wrong subtype"
// into ErrNotFound.
func getOwnedTimeDeposit(ctx context.Context, q *db.Queries, id, hid uuid.UUID) (db.Investment, error) {
	inv, err := q.GetInvestmentByID(ctx, db.GetInvestmentByIDParams{ID: id, HouseholdID: hid})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Investment{}, ErrNotFound
		}
		return db.Investment{}, err
	}
	if inv.Subtype != "time_deposit" {
		return db.Investment{}, ErrNotFound
	}
	return inv, nil
}

func (r *InvestmentRepo) DeleteTimeDeposit(ctx context.Context, id uuid.UUID) error {
	if _, err := r.GetTimeDeposit(ctx, id); err != nil {
		return err
	}
	return r.softDeleteInvestment(ctx, id)
}
