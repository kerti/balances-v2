package repo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/kerti/balances-v2/backend/internal/db"
)

// InflationRateRepo wraps the manual inflation-rate table (ADR-0048). Rates are
// household-scoped; year_month is the identity (one rate per month, no currency
// dimension), so a duplicate create is a conflict (edit the existing rate
// instead). The rate is an annualized percentage and may be negative (deflation).
type InflationRateRepo struct {
	pool *pgxpool.Pool
	q    *db.Queries
}

func NewInflationRateRepo(pool *pgxpool.Pool) *InflationRateRepo {
	return &InflationRateRepo{pool: pool, q: db.New(pool)}
}

type CreateInflationRateParams struct {
	YearMonth time.Time
	Rate      decimal.Decimal
}

func (r *InflationRateRepo) CreateInflationRate(ctx context.Context, p CreateInflationRateParams) (*db.InflationRate, error) {
	user, hid, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	row, err := r.q.CreateInflationRate(ctx, db.CreateInflationRateParams{
		HouseholdID: hid,
		YearMonth:   p.YearMonth,
		Rate:        p.Rate,
		CreatedBy:   &user,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique_violation
			return nil, ErrInflationRateExists
		}
		return nil, fmt.Errorf("create inflation rate: %w", err)
	}
	return &row, nil
}

func (r *InflationRateRepo) ListInflationRates(ctx context.Context) ([]db.InflationRate, error) {
	_, hid, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := r.q.ListInflationRatesByHousehold(ctx, hid)
	if err != nil {
		return nil, fmt.Errorf("list inflation rates: %w", err)
	}
	if rows == nil {
		return []db.InflationRate{}, nil
	}
	return rows, nil
}

func (r *InflationRateRepo) UpdateInflationRate(ctx context.Context, id uuid.UUID, rate decimal.Decimal) (*db.InflationRate, error) {
	user, hid, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	row, err := r.q.UpdateInflationRate(ctx, db.UpdateInflationRateParams{
		ID: id, HouseholdID: hid, Rate: rate, UpdatedBy: &user,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("update inflation rate: %w", err)
	}
	return &row, nil
}

func (r *InflationRateRepo) DeleteInflationRate(ctx context.Context, id uuid.UUID) error {
	user, hid, err := currentUser(ctx)
	if err != nil {
		return err
	}
	rows, err := r.q.SoftDeleteInflationRate(ctx, db.SoftDeleteInflationRateParams{ID: id, HouseholdID: hid, UpdatedBy: &user})
	if err != nil {
		return fmt.Errorf("soft delete inflation rate: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}
