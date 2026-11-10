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

// FxRateRepo wraps the manual FX-rate table (ADR-0002). Rates are household-
// scoped; year_month + currency are the identity, so a duplicate create is a
// conflict (edit the existing rate instead).
type FxRateRepo struct {
	pool *pgxpool.Pool
	q    *db.Queries
}

func NewFxRateRepo(pool *pgxpool.Pool) *FxRateRepo {
	return &FxRateRepo{pool: pool, q: db.New(pool)}
}

type CreateFxRateParams struct {
	YearMonth time.Time
	Currency  string
	Rate      decimal.Decimal
}

func (r *FxRateRepo) CreateFxRate(ctx context.Context, p CreateFxRateParams) (*db.FxRate, error) {
	user, hid, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	row, err := r.q.CreateFxRate(ctx, db.CreateFxRateParams{
		HouseholdID: hid,
		YearMonth:   p.YearMonth,
		Currency:    p.Currency,
		Rate:        p.Rate,
		CreatedBy:   &user,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique_violation
			return nil, ErrFxRateExists
		}
		return nil, fmt.Errorf("create fx rate: %w", err)
	}
	return &row, nil
}

func (r *FxRateRepo) ListFxRates(ctx context.Context) ([]db.FxRate, error) {
	_, hid, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := r.q.ListFxRatesByHousehold(ctx, hid)
	if err != nil {
		return nil, fmt.Errorf("list fx rates: %w", err)
	}
	if rows == nil {
		return []db.FxRate{}, nil
	}
	return rows, nil
}

func (r *FxRateRepo) UpdateFxRate(ctx context.Context, id uuid.UUID, rate decimal.Decimal) (*db.FxRate, error) {
	user, hid, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	row, err := r.q.UpdateFxRate(ctx, db.UpdateFxRateParams{
		ID: id, HouseholdID: hid, Rate: rate, UpdatedBy: &user,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("update fx rate: %w", err)
	}
	return &row, nil
}

func (r *FxRateRepo) DeleteFxRate(ctx context.Context, id uuid.UUID) error {
	user, hid, err := currentUser(ctx)
	if err != nil {
		return err
	}
	rows, err := r.q.SoftDeleteFxRate(ctx, db.SoftDeleteFxRateParams{ID: id, HouseholdID: hid, UpdatedBy: &user})
	if err != nil {
		return fmt.Errorf("soft delete fx rate: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// ListUsedForeignCurrencies returns the foreign currencies the household's live
// positions are recorded in (reporting currency excluded), sorted. It feeds the
// New-rate currency select; it is not limited to currencies missing a rate.
func (r *FxRateRepo) ListUsedForeignCurrencies(ctx context.Context) ([]string, error) {
	_, hid, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := r.q.ListUsedForeignCurrencies(ctx, hid)
	if err != nil {
		return nil, fmt.Errorf("list used foreign currencies: %w", err)
	}
	if rows == nil {
		return []string{}, nil
	}
	return rows, nil
}
