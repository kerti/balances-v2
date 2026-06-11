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

type Property struct {
	Asset   db.Asset          `json:"asset"`
	Details db.PropertyDetail `json:"details"`
}

type PropertyListItem struct {
	Asset          db.Asset          `json:"asset"`
	Details        db.PropertyDetail `json:"details"`
	LatestSnapshot *db.AssetSnapshot `json:"latest_snapshot"`
}

type CreatePropertyParams struct {
	DisplayName            string
	Description            *string
	OwnershipType          string
	SoleOwnerUserID        *uuid.UUID
	NativeCurrency         string
	PropertyType           string // "house" | "apartment" | "land" | "commercial"
	Address                *string
	AcquisitionDate        *time.Time
	AcquisitionCost        *decimal.Decimal
	AnnualAppreciationRate *decimal.Decimal
}

type UpdatePropertyParams struct {
	DisplayName            string
	Description            *string
	OwnershipType          string
	SoleOwnerUserID        *uuid.UUID
	PropertyType           string
	Address                *string
	AcquisitionDate        *time.Time
	AcquisitionCost        *decimal.Decimal
	AnnualAppreciationRate *decimal.Decimal
}

func (r *AssetRepo) CreateProperty(ctx context.Context, p CreatePropertyParams) (*Property, error) {
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
		Subtype:         "property",
		OwnershipType:   p.OwnershipType,
		SoleOwnerUserID: p.SoleOwnerUserID,
		NativeCurrency:  p.NativeCurrency,
		CreatedBy:       &user,
	})
	if err != nil {
		return nil, fmt.Errorf("create asset: %w", err)
	}

	details, err := qtx.CreatePropertyDetails(ctx, db.CreatePropertyDetailsParams{
		AssetID:                asset.ID,
		PropertyType:           p.PropertyType,
		Address:                p.Address,
		AcquisitionDate:        p.AcquisitionDate,
		AcquisitionCost:        p.AcquisitionCost,
		AnnualAppreciationRate: p.AnnualAppreciationRate,
	})
	if err != nil {
		return nil, fmt.Errorf("create property_details: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return &Property{Asset: asset, Details: details}, nil
}

func (r *AssetRepo) GetProperty(ctx context.Context, id uuid.UUID) (*Property, error) {
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
	if asset.Subtype != "property" {
		return nil, ErrNotFound
	}

	details, err := r.q.GetPropertyDetailsByAssetID(ctx, asset.ID)
	if err != nil {
		return nil, fmt.Errorf("get property_details: %w", err)
	}

	return &Property{Asset: asset, Details: details}, nil
}

func (r *AssetRepo) ListProperties(ctx context.Context) ([]PropertyListItem, error) {
	_, hid, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}

	subtype := "property"
	assets, err := r.q.ListAssetsByHousehold(ctx, db.ListAssetsByHouseholdParams{
		HouseholdID: hid,
		Subtype:     &subtype,
	})
	if err != nil {
		return nil, fmt.Errorf("list assets: %w", err)
	}
	if len(assets) == 0 {
		return []PropertyListItem{}, nil
	}

	ids := make([]uuid.UUID, len(assets))
	for i, a := range assets {
		ids[i] = a.ID
	}

	details, err := r.q.ListPropertyDetailsByAssetIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("list property_details: %w", err)
	}
	detailByID := make(map[uuid.UUID]db.PropertyDetail, len(details))
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

	out := make([]PropertyListItem, 0, len(assets))
	for _, a := range assets {
		item := PropertyListItem{Asset: a, Details: detailByID[a.ID]}
		if s, ok := snapByID[a.ID]; ok {
			s := s
			item.LatestSnapshot = &s
		}
		out = append(out, item)
	}
	return out, nil
}

func (r *AssetRepo) UpdateProperty(ctx context.Context, id uuid.UUID, p UpdatePropertyParams) (*Property, error) {
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
	if asset.Subtype != "property" {
		return nil, ErrNotFound
	}

	details, err := qtx.UpdatePropertyDetails(ctx, db.UpdatePropertyDetailsParams{
		AssetID:                asset.ID,
		PropertyType:           p.PropertyType,
		Address:                p.Address,
		AcquisitionDate:        p.AcquisitionDate,
		AcquisitionCost:        p.AcquisitionCost,
		AnnualAppreciationRate: p.AnnualAppreciationRate,
	})
	if err != nil {
		return nil, fmt.Errorf("update property_details: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return &Property{Asset: asset, Details: details}, nil
}

// PropertyExport is everything export needs to render a full position workbook:
// the aggregate, the human-facing values for the two id-typed fields (owner
// email, tag name — resolved per the Detail-sheet conventions), and the full
// snapshot history.
type PropertyExport struct {
	Property   Property
	OwnerEmail string // sole_owner's email; "" for joint
	TagName    string // resolved tag name; "" when untagged
	Snapshots  []db.AssetSnapshot
}

// ExportProperty gathers a property, its resolved owner email + tag name, and
// its snapshot history, scoped + ownership-checked to the caller's household
// (404 via GetProperty when not owned or wrong subtype).
func (r *AssetRepo) ExportProperty(ctx context.Context, id uuid.UUID) (*PropertyExport, error) {
	_, hid, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}

	property, err := r.GetProperty(ctx, id)
	if err != nil {
		return nil, err
	}

	out := &PropertyExport{Property: *property}

	if uid := property.Asset.SoleOwnerUserID; uid != nil {
		user, err := r.q.GetUserByID(ctx, *uid)
		if err != nil {
			return nil, fmt.Errorf("export: resolve owner: %w", err)
		}
		out.OwnerEmail = user.Email
	}

	if tid := property.Asset.TagID; tid != nil {
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

// CreatePropertyWithSnapshots creates a property and seeds its snapshot history
// in a single transaction (all-or-nothing) — the commit path of the create-
// from-file import. It mirrors CreateProperty's asset+details writes, then
// optionally assigns the resolved Tag and upserts every snapshot row, all under
// one tx so a mid-batch failure leaves nothing behind. tagID nil leaves the
// position untagged; an empty snaps seeds no history.
func (r *AssetRepo) CreatePropertyWithSnapshots(ctx context.Context, p CreatePropertyParams, tagID *uuid.UUID, snaps []ImportSnapshotRow) (*Property, error) {
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
		Subtype:         "property",
		OwnershipType:   p.OwnershipType,
		SoleOwnerUserID: p.SoleOwnerUserID,
		NativeCurrency:  p.NativeCurrency,
		CreatedBy:       &user,
	})
	if err != nil {
		return nil, fmt.Errorf("create asset: %w", err)
	}

	details, err := qtx.CreatePropertyDetails(ctx, db.CreatePropertyDetailsParams{
		AssetID:                asset.ID,
		PropertyType:           p.PropertyType,
		Address:                p.Address,
		AcquisitionDate:        p.AcquisitionDate,
		AcquisitionCost:        p.AcquisitionCost,
		AnnualAppreciationRate: p.AnnualAppreciationRate,
	})
	if err != nil {
		return nil, fmt.Errorf("create property_details: %w", err)
	}

	if tagID != nil {
		if _, err := qtx.AssignAssetTag(ctx, db.AssignAssetTagParams{
			TagID:       tagID,
			ID:          asset.ID,
			HouseholdID: hid,
			UpdatedBy:   &user,
		}); err != nil {
			return nil, fmt.Errorf("assign tag: %w", err)
		}
		asset.TagID = tagID
	}

	for _, row := range snaps {
		if _, err := qtx.UpsertAssetSnapshot(ctx, db.UpsertAssetSnapshotParams{
			ID:          asset.ID,
			YearMonth:   row.YearMonth,
			Amount:      row.Amount,
			Currency:    row.Currency,
			AsOfDate:    row.AsOfDate,
			Description: row.Description,
			CreatedBy:   &user,
			HouseholdID: hid,
		}); err != nil {
			return nil, fmt.Errorf("import: upsert %s: %w", row.YearMonth.Format("2006-01"), err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return &Property{Asset: asset, Details: details}, nil
}

func (r *AssetRepo) DeleteProperty(ctx context.Context, id uuid.UUID) error {
	if _, err := r.GetProperty(ctx, id); err != nil {
		return err
	}
	return r.softDeleteAsset(ctx, id)
}
