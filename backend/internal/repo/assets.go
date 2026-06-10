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

	"github.com/kerti/balances-v2/backend/internal/auth"
	"github.com/kerti/balances-v2/backend/internal/db"
)

// AssetRepo wraps the generated queries with tenancy-aware methods for the
// Asset position group and its three subtypes (bank_account, property,
// vehicle — each in its own file). Asset snapshots, which share a table
// across all subtypes per ADR-0022, live here since they're not
// subtype-specific.
type AssetRepo struct {
	pool *pgxpool.Pool
	q    *db.Queries
}

func NewAssetRepo(pool *pgxpool.Pool) *AssetRepo {
	return &AssetRepo{pool: pool, q: db.New(pool)}
}

// ----- asset snapshots (shared across all subtypes) -----------------------

type CreateAssetSnapshotParams struct {
	AssetID     uuid.UUID
	YearMonth   time.Time
	Amount      decimal.Decimal
	Currency    string
	AsOfDate    *time.Time
	Description *string
}

type UpdateAssetSnapshotParams struct {
	SnapshotID  uuid.UUID
	Amount      decimal.Decimal
	Currency    string
	AsOfDate    *time.Time
	Description *string
}

func (r *AssetRepo) CreateAssetSnapshot(ctx context.Context, p CreateAssetSnapshotParams) (*db.AssetSnapshot, error) {
	user, hid, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}

	snap, err := r.q.CreateAssetSnapshot(ctx, db.CreateAssetSnapshotParams{
		ID:          p.AssetID,
		YearMonth:   p.YearMonth,
		Amount:      p.Amount,
		Currency:    p.Currency,
		AsOfDate:    p.AsOfDate,
		Description: p.Description,
		CreatedBy:   &user,
		HouseholdID: hid,
	})
	if err != nil {
		if asOfMonthViolation(err) {
			return nil, ErrSnapshotDateOutsideMonth
		}
		if errors.Is(err, pgx.ErrNoRows) {
			// CTE matched no asset, so the snapshot wasn't inserted —
			// either the asset doesn't exist in this household or it's
			// soft-deleted.
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("create asset snapshot: %w", err)
	}
	return &snap, nil
}

func (r *AssetRepo) ListAssetSnapshots(ctx context.Context, assetID uuid.UUID) ([]db.AssetSnapshot, error) {
	_, hid, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	return r.q.ListAssetSnapshotsForAsset(ctx, db.ListAssetSnapshotsForAssetParams{
		AssetID:     assetID,
		HouseholdID: hid,
	})
}

func (r *AssetRepo) UpdateAssetSnapshot(ctx context.Context, p UpdateAssetSnapshotParams) (*db.AssetSnapshot, error) {
	user, hid, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	snap, err := r.q.UpdateAssetSnapshot(ctx, db.UpdateAssetSnapshotParams{
		ID:          p.SnapshotID,
		HouseholdID: hid,
		Amount:      p.Amount,
		Currency:    p.Currency,
		AsOfDate:    p.AsOfDate,
		Description: p.Description,
		UpdatedBy:   &user,
	})
	if err != nil {
		if asOfMonthViolation(err) {
			return nil, ErrSnapshotDateOutsideMonth
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("update asset snapshot: %w", err)
	}
	return &snap, nil
}

func (r *AssetRepo) DeleteAssetSnapshot(ctx context.Context, snapshotID uuid.UUID) error {
	user, hid, err := currentUser(ctx)
	if err != nil {
		return err
	}
	rows, err := r.q.SoftDeleteAssetSnapshot(ctx, db.SoftDeleteAssetSnapshotParams{
		ID:          snapshotID,
		HouseholdID: hid,
		UpdatedBy:   &user,
	})
	if err != nil {
		return fmt.Errorf("soft delete snapshot: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// SoftDeleteAsset is the shared delete path for any Asset subtype. Each
// subtype's repo file exposes a thin wrapper (DeleteBankAccount, etc.) that
// adds a subtype guard before calling this.
func (r *AssetRepo) softDeleteAsset(ctx context.Context, id uuid.UUID) error {
	user, hid, err := currentUser(ctx)
	if err != nil {
		return err
	}
	rows, err := r.q.SoftDeleteAsset(ctx, db.SoftDeleteAssetParams{
		ID:          id,
		HouseholdID: hid,
		UpdatedBy:   &user,
	})
	if err != nil {
		return fmt.Errorf("soft delete asset: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// ----- helpers ------------------------------------------------------------

// currentUser returns (user_id, household_id) from request context, or
// ErrUnauthenticated if no user is attached.
func currentUser(ctx context.Context) (uuid.UUID, uuid.UUID, error) {
	u, ok := auth.UserFromContext(ctx)
	if !ok {
		return uuid.Nil, uuid.Nil, ErrUnauthenticated
	}
	return u.ID, u.HouseholdID, nil
}
