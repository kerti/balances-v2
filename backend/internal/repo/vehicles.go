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

type Vehicle struct {
	Asset   db.Asset         `json:"asset"`
	Details db.VehicleDetail `json:"details"`
}

type VehicleListItem struct {
	Asset          db.Asset          `json:"asset"`
	Details        db.VehicleDetail  `json:"details"`
	LatestSnapshot *db.AssetSnapshot `json:"latest_snapshot"`
}

type CreateVehicleParams struct {
	DisplayName            string
	Description            *string
	OwnershipType          string
	SoleOwnerUserID        *uuid.UUID
	NativeCurrency         string
	VehicleType            string // "car" | "motorcycle" | "other"
	Make                   *string
	Model                  *string
	Year                   *int32
	PlateNumber            *string
	AnnualDepreciationRate *decimal.Decimal
}

type UpdateVehicleParams struct {
	DisplayName            string
	Description            *string
	OwnershipType          string
	SoleOwnerUserID        *uuid.UUID
	VehicleType            string
	Make                   *string
	Model                  *string
	Year                   *int32
	PlateNumber            *string
	AnnualDepreciationRate *decimal.Decimal
}

func (r *AssetRepo) CreateVehicle(ctx context.Context, p CreateVehicleParams) (*Vehicle, error) {
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
	asset, err := qtx.CreateAsset(ctx, db.CreateAssetParams{
		HouseholdID:     hid,
		DisplayName:     p.DisplayName,
		Description:     p.Description,
		Subtype:         "vehicle",
		OwnershipType:   p.OwnershipType,
		SoleOwnerUserID: p.SoleOwnerUserID,
		NativeCurrency:  p.NativeCurrency,
		CreatedBy:       &user,
	})
	if err != nil {
		return nil, fmt.Errorf("create asset: %w", err)
	}

	details, err := qtx.CreateVehicleDetails(ctx, db.CreateVehicleDetailsParams{
		AssetID:                asset.ID,
		VehicleType:            p.VehicleType,
		Make:                   p.Make,
		Model:                  p.Model,
		Year:                   p.Year,
		PlateNumber:            p.PlateNumber,
		AnnualDepreciationRate: p.AnnualDepreciationRate,
	})
	if err != nil {
		return nil, fmt.Errorf("create vehicle_details: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return &Vehicle{Asset: asset, Details: details}, nil
}

func (r *AssetRepo) GetVehicle(ctx context.Context, id uuid.UUID) (*Vehicle, error) {
	_, hid, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}

	asset, err := r.q.GetAssetByID(ctx, db.GetAssetByIDParams{ID: id, HouseholdID: hid})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if asset.Subtype != "vehicle" {
		return nil, ErrNotFound
	}

	details, err := r.q.GetVehicleDetailsByAssetID(ctx, asset.ID)
	if err != nil {
		return nil, fmt.Errorf("get vehicle_details: %w", err)
	}

	return &Vehicle{Asset: asset, Details: details}, nil
}

func (r *AssetRepo) ListVehicles(ctx context.Context) ([]VehicleListItem, error) {
	_, hid, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}

	subtype := "vehicle"
	assets, err := r.q.ListAssetsByHousehold(ctx, db.ListAssetsByHouseholdParams{
		HouseholdID: hid,
		Subtype:     &subtype,
	})
	if err != nil {
		return nil, fmt.Errorf("list assets: %w", err)
	}
	if len(assets) == 0 {
		return []VehicleListItem{}, nil
	}

	ids := make([]uuid.UUID, len(assets))
	for i, a := range assets {
		ids[i] = a.ID
	}

	details, err := r.q.ListVehicleDetailsByAssetIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("list vehicle_details: %w", err)
	}
	detailByID := make(map[uuid.UUID]db.VehicleDetail, len(details))
	for _, d := range details {
		detailByID[d.AssetID] = d
	}

	snapshots, err := r.q.ListLatestSnapshotsByAssetIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("list latest snapshots: %w", err)
	}
	snapByID := make(map[uuid.UUID]db.AssetSnapshot, len(snapshots))
	for _, s := range snapshots {
		snapByID[s.AssetID] = s
	}

	out := make([]VehicleListItem, 0, len(assets))
	for _, a := range assets {
		item := VehicleListItem{Asset: a, Details: detailByID[a.ID]}
		if s, ok := snapByID[a.ID]; ok {
			s := s
			item.LatestSnapshot = &s
		}
		out = append(out, item)
	}
	return out, nil
}

func (r *AssetRepo) UpdateVehicle(ctx context.Context, id uuid.UUID, p UpdateVehicleParams) (*Vehicle, error) {
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
	asset, err := qtx.UpdateAsset(ctx, db.UpdateAssetParams{
		ID:              id,
		HouseholdID:     hid,
		DisplayName:     p.DisplayName,
		Description:     p.Description,
		OwnershipType:   p.OwnershipType,
		SoleOwnerUserID: p.SoleOwnerUserID,
		UpdatedBy:       &user,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("update asset: %w", err)
	}
	if asset.Subtype != "vehicle" {
		return nil, ErrNotFound
	}

	details, err := qtx.UpdateVehicleDetails(ctx, db.UpdateVehicleDetailsParams{
		AssetID:                asset.ID,
		VehicleType:            p.VehicleType,
		Make:                   p.Make,
		Model:                  p.Model,
		Year:                   p.Year,
		PlateNumber:            p.PlateNumber,
		AnnualDepreciationRate: p.AnnualDepreciationRate,
	})
	if err != nil {
		return nil, fmt.Errorf("update vehicle_details: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return &Vehicle{Asset: asset, Details: details}, nil
}

// VehicleExport is everything export needs to render a full position workbook:
// the aggregate, the human-facing values for the two id-typed fields (owner
// email, tag name — resolved per the Detail-sheet conventions), and the full
// snapshot history.
type VehicleExport struct {
	Vehicle    Vehicle
	OwnerEmail string // sole_owner's email; "" for joint
	TagName    string // resolved tag name; "" when untagged
	Snapshots  []db.AssetSnapshot
}

// ExportVehicle gathers a vehicle, its resolved owner email + tag name, and its
// snapshot history, scoped + ownership-checked to the caller's household (404
// via GetVehicle when not owned or wrong subtype).
func (r *AssetRepo) ExportVehicle(ctx context.Context, id uuid.UUID) (*VehicleExport, error) {
	_, hid, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}

	vehicle, err := r.GetVehicle(ctx, id)
	if err != nil {
		return nil, err
	}

	out := &VehicleExport{Vehicle: *vehicle}

	if uid := vehicle.Asset.SoleOwnerUserID; uid != nil {
		user, err := r.q.GetUserByID(ctx, *uid)
		if err != nil {
			return nil, fmt.Errorf("export: resolve owner: %w", err)
		}
		out.OwnerEmail = user.Email
	}

	if tid := vehicle.Asset.TagID; tid != nil {
		tag, err := r.q.GetTagByID(ctx, db.GetTagByIDParams{ID: *tid, HouseholdID: hid})
		if err != nil {
			return nil, fmt.Errorf("export: resolve tag: %w", err)
		}
		out.TagName = tag.Name
	}

	snaps, err := r.q.ListAssetSnapshotsForAsset(ctx, db.ListAssetSnapshotsForAssetParams{
		AssetID:     id,
		HouseholdID: hid,
	})
	if err != nil {
		return nil, fmt.Errorf("export: list snapshots: %w", err)
	}
	out.Snapshots = snaps

	return out, nil
}

func (r *AssetRepo) DeleteVehicle(ctx context.Context, id uuid.UUID) error {
	if _, err := r.GetVehicle(ctx, id); err != nil {
		return err
	}
	return r.softDeleteAsset(ctx, id)
}
