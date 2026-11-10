package repo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/kerti/balances-v2/backend/internal/db"
)

// IncomeRepo wraps the generated queries for the Income flow-event entity.
// Per ADR-0008, Income is a flat flow event — no subtype, no snapshots, no
// transactions, no extension tables — so this repo is small.
type IncomeRepo struct {
	pool *pgxpool.Pool
	q    *db.Queries
}

func NewIncomeRepo(pool *pgxpool.Pool) *IncomeRepo {
	return &IncomeRepo{pool: pool, q: db.New(pool)}
}

type CreateIncomeParams struct {
	Date            time.Time
	Amount          decimal.Decimal
	Currency        string
	Category        string // see income migration CHECK
	Description     *string
	OwnershipType   string // "sole" | "joint"
	SoleOwnerUserID *uuid.UUID
	Regularity      string // "routine" | "incidental" — migration 00017 CHECK
}

type UpdateIncomeParams struct {
	Date            time.Time
	Amount          decimal.Decimal
	Currency        string
	Category        string
	Description     *string
	OwnershipType   string
	SoleOwnerUserID *uuid.UUID
	Regularity      string
}

func (r *IncomeRepo) CreateIncome(ctx context.Context, p CreateIncomeParams) (*db.Income, error) {
	user, hid, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	row, err := r.q.CreateIncome(ctx, db.CreateIncomeParams{
		HouseholdID:     hid,
		Date:            p.Date,
		Amount:          p.Amount,
		Currency:        p.Currency,
		Category:        p.Category,
		Description:     p.Description,
		OwnershipType:   p.OwnershipType,
		SoleOwnerUserID: p.SoleOwnerUserID,
		Regularity:      p.Regularity,
		CreatedBy:       &user,
	})
	if err != nil {
		return nil, fmt.Errorf("create income: %w", err)
	}
	return &row, nil
}

// CreateIncomeBulk inserts every row in one transaction, all under the caller's
// household: a failing row rolls the whole batch back, so a copy-forward save
// is all-or-nothing (#846). Rows come back in input order.
func (r *IncomeRepo) CreateIncomeBulk(ctx context.Context, ps []CreateIncomeParams) ([]db.Income, error) {
	user, hid, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("create income bulk: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := r.q.WithTx(tx)
	out := make([]db.Income, 0, len(ps))
	for _, p := range ps {
		row, err := qtx.CreateIncome(ctx, db.CreateIncomeParams{
			HouseholdID:     hid,
			Date:            p.Date,
			Amount:          p.Amount,
			Currency:        p.Currency,
			Category:        p.Category,
			Description:     p.Description,
			OwnershipType:   p.OwnershipType,
			SoleOwnerUserID: p.SoleOwnerUserID,
			Regularity:      p.Regularity,
			CreatedBy:       &user,
		})
		if err != nil {
			return nil, fmt.Errorf("create income bulk: %w", err)
		}
		out = append(out, row)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("create income bulk: commit: %w", err)
	}
	return out, nil
}

func (r *IncomeRepo) GetIncome(ctx context.Context, id uuid.UUID) (*db.Income, error) {
	_, hid, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	row, err := r.q.GetIncomeByID(ctx, db.GetIncomeByIDParams{ID: id, HouseholdID: hid})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &row, nil
}

func (r *IncomeRepo) ListIncome(ctx context.Context) ([]db.Income, error) {
	_, hid, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := r.q.ListIncomeByHousehold(ctx, hid)
	if err != nil {
		return nil, fmt.Errorf("list income: %w", err)
	}
	if rows == nil {
		return []db.Income{}, nil
	}
	return rows, nil
}

func (r *IncomeRepo) UpdateIncome(ctx context.Context, id uuid.UUID, p UpdateIncomeParams) (*db.Income, error) {
	user, hid, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	row, err := r.q.UpdateIncome(ctx, db.UpdateIncomeParams{
		ID:              id,
		HouseholdID:     hid,
		Date:            p.Date,
		Amount:          p.Amount,
		Currency:        p.Currency,
		Category:        p.Category,
		Description:     p.Description,
		OwnershipType:   p.OwnershipType,
		SoleOwnerUserID: p.SoleOwnerUserID,
		Regularity:      p.Regularity,
		UpdatedBy:       &user,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("update income: %w", err)
	}
	return &row, nil
}

func (r *IncomeRepo) DeleteIncome(ctx context.Context, id uuid.UUID) error {
	user, hid, err := currentUser(ctx)
	if err != nil {
		return err
	}
	rows, err := r.q.SoftDeleteIncome(ctx, db.SoftDeleteIncomeParams{
		ID:          id,
		HouseholdID: hid,
		UpdatedBy:   &user,
	})
	if err != nil {
		return fmt.Errorf("soft delete income: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}
