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

// InvestmentRepo wraps the generated queries with tenancy-aware methods for
// the Investment position group and its subtypes (stock, mutual_fund, gold
// in M4.3a; bond, time_deposit in M4.3b — each in its own file). Investment
// snapshots, which share a table across all subtypes per ADR-0022, live here
// since the snapshot CRUD is not subtype-specific apart from the
// value-column XOR shape validated against the parent's subtype.
type InvestmentRepo struct {
	pool *pgxpool.Pool
	q    *db.Queries
}

func NewInvestmentRepo(pool *pgxpool.Pool) *InvestmentRepo {
	return &InvestmentRepo{pool: pool, q: db.New(pool)}
}

// ----- investment snapshots (shared across all subtypes) -----------------

type CreateInvestmentSnapshotParams struct {
	InvestmentID    uuid.UUID
	YearMonth       time.Time
	Amount          decimal.Decimal
	Currency        string
	Quantity        *decimal.Decimal
	PricePerUnit    *decimal.Decimal
	AccruedInterest *decimal.Decimal
	AsOfDate        *time.Time
	Description     *string
}

type UpdateInvestmentSnapshotParams struct {
	SnapshotID      uuid.UUID
	Amount          decimal.Decimal
	Currency        string
	Quantity        *decimal.Decimal
	PricePerUnit    *decimal.Decimal
	AccruedInterest *decimal.Decimal
	AsOfDate        *time.Time
	Description     *string
}

func (r *InvestmentRepo) CreateInvestmentSnapshot(ctx context.Context, p CreateInvestmentSnapshotParams) (*db.InvestmentSnapshot, error) {
	user, hid, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}

	// Look up parent's subtype so we can enforce the shape rule the DB CHECK
	// can't see ("stock => quantity+price; bond => accrued_interest").
	inv, err := r.q.GetInvestmentByID(ctx, db.GetInvestmentByIDParams{ID: p.InvestmentID, HouseholdID: hid})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get investment for snapshot: %w", err)
	}
	if err := validateInvestmentSnapshotShape(inv.Subtype, p.Quantity, p.PricePerUnit, p.AccruedInterest); err != nil {
		return nil, err
	}
	// A time deposit's snapshots are confined to its term window (issue #62);
	// other subtypes are unbounded, so this is a no-op for them.
	bounds, err := timeDepositBounds(ctx, r.q, inv)
	if err != nil {
		return nil, err
	}
	if err := bounds.checkSnapshotMonth(p.YearMonth); err != nil {
		return nil, err
	}

	snap, err := r.q.CreateInvestmentSnapshot(ctx, db.CreateInvestmentSnapshotParams{
		ID:              p.InvestmentID,
		YearMonth:       p.YearMonth,
		Amount:          p.Amount,
		Currency:        p.Currency,
		Quantity:        p.Quantity,
		PricePerUnit:    p.PricePerUnit,
		AccruedInterest: p.AccruedInterest,
		AsOfDate:        p.AsOfDate,
		Description:     p.Description,
		CreatedBy:       &user,
		HouseholdID:     hid,
	})
	if err != nil {
		if asOfMonthViolation(err) {
			return nil, ErrSnapshotDateOutsideMonth
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("create investment snapshot: %w", err)
	}
	return &snap, nil
}

func (r *InvestmentRepo) ListInvestmentSnapshots(ctx context.Context, investmentID uuid.UUID) ([]db.InvestmentSnapshot, error) {
	_, hid, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	return r.q.ListInvestmentSnapshotsForInvestment(ctx, db.ListInvestmentSnapshotsForInvestmentParams{
		InvestmentID: investmentID,
		HouseholdID:  hid,
	})
}

func (r *InvestmentRepo) UpdateInvestmentSnapshot(ctx context.Context, p UpdateInvestmentSnapshotParams) (*db.InvestmentSnapshot, error) {
	user, hid, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}

	// Validate shape against the existing snapshot's parent subtype. Both
	// lookups are household-scoped, so a cross-tenant update reaches the
	// first ErrNotFound here.
	existing, err := r.q.GetInvestmentSnapshotByID(ctx, db.GetInvestmentSnapshotByIDParams{ID: p.SnapshotID, HouseholdID: hid})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get investment snapshot: %w", err)
	}
	inv, err := r.q.GetInvestmentByID(ctx, db.GetInvestmentByIDParams{ID: existing.InvestmentID, HouseholdID: hid})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get parent investment: %w", err)
	}
	if err := validateInvestmentSnapshotShape(inv.Subtype, p.Quantity, p.PricePerUnit, p.AccruedInterest); err != nil {
		return nil, err
	}
	// year_month is immutable on update, so this can only fire on a row that
	// already sits outside the term (e.g. legacy data) — we decline to bless it
	// rather than silently re-save it (issue #62). No-op for non-time-deposits.
	bounds, err := timeDepositBounds(ctx, r.q, inv)
	if err != nil {
		return nil, err
	}
	if err := bounds.checkSnapshotMonth(existing.YearMonth); err != nil {
		return nil, err
	}

	snap, err := r.q.UpdateInvestmentSnapshot(ctx, db.UpdateInvestmentSnapshotParams{
		ID:              p.SnapshotID,
		HouseholdID:     hid,
		Amount:          p.Amount,
		Currency:        p.Currency,
		Quantity:        p.Quantity,
		PricePerUnit:    p.PricePerUnit,
		AccruedInterest: p.AccruedInterest,
		AsOfDate:        p.AsOfDate,
		Description:     p.Description,
		UpdatedBy:       &user,
	})
	if err != nil {
		if asOfMonthViolation(err) {
			return nil, ErrSnapshotDateOutsideMonth
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("update investment snapshot: %w", err)
	}
	return &snap, nil
}

func (r *InvestmentRepo) DeleteInvestmentSnapshot(ctx context.Context, snapshotID uuid.UUID) error {
	user, hid, err := currentUser(ctx)
	if err != nil {
		return err
	}
	rows, err := r.q.SoftDeleteInvestmentSnapshot(ctx, db.SoftDeleteInvestmentSnapshotParams{
		ID:          snapshotID,
		HouseholdID: hid,
		UpdatedBy:   &user,
	})
	if err != nil {
		return fmt.Errorf("soft delete investment snapshot: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// softDeleteInvestment is the shared delete path for any Investment subtype.
// Each subtype's repo file exposes a thin wrapper (DeleteStock, etc.) that
// adds a subtype guard before calling this.
func (r *InvestmentRepo) softDeleteInvestment(ctx context.Context, id uuid.UUID) error {
	user, hid, err := currentUser(ctx)
	if err != nil {
		return err
	}
	rows, err := r.q.SoftDeleteInvestment(ctx, db.SoftDeleteInvestmentParams{
		ID:          id,
		HouseholdID: hid,
		UpdatedBy:   &user,
	})
	if err != nil {
		return fmt.Errorf("soft delete investment: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// validateInvestmentSnapshotShape enforces the subtype→shape mapping that
// the DB's CHECK constraint can't express (ADR-0022).
func validateInvestmentSnapshotShape(subtype string, quantity, pricePerUnit, accruedInterest *decimal.Decimal) error {
	switch subtype {
	case "stock", "mutual_fund", "gold":
		if quantity == nil || pricePerUnit == nil {
			return fmt.Errorf("%w: %s requires quantity and price_per_unit", ErrInvalidSnapshotShape, subtype)
		}
		if accruedInterest != nil {
			return fmt.Errorf("%w: %s must not have accrued_interest", ErrInvalidSnapshotShape, subtype)
		}
	case "bond", "time_deposit":
		if accruedInterest == nil {
			return fmt.Errorf("%w: %s requires accrued_interest", ErrInvalidSnapshotShape, subtype)
		}
		if quantity != nil || pricePerUnit != nil {
			return fmt.Errorf("%w: %s must not have quantity or price_per_unit", ErrInvalidSnapshotShape, subtype)
		}
	default:
		return fmt.Errorf("%w: unknown subtype %q", ErrInvalidSnapshotShape, subtype)
	}
	return nil
}
