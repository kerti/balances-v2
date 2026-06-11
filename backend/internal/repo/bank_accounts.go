package repo

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/kerti/balances-v2/backend/internal/db"
)

// BankAccount is the aggregate returned by Get/Create — the core asset row
// joined with its bank_account_details extension.
type BankAccount struct {
	Asset   db.Asset             `json:"asset"`
	Details db.BankAccountDetail `json:"details"`
}

// BankAccountListItem extends BankAccount with the most-recent Snapshot if
// any (for list views that want to display the current balance).
type BankAccountListItem struct {
	Asset          db.Asset             `json:"asset"`
	Details        db.BankAccountDetail `json:"details"`
	LatestSnapshot *db.AssetSnapshot    `json:"latest_snapshot"`
}

type CreateBankAccountParams struct {
	DisplayName     string
	Description     *string
	OwnershipType   string // "sole" or "joint"
	SoleOwnerUserID *uuid.UUID
	NativeCurrency  string
	BankName        string
	AccountNumber   string
	AccountType     string // "savings" | "current" | "other"
}

type UpdateBankAccountParams struct {
	DisplayName     string
	Description     *string
	OwnershipType   string
	SoleOwnerUserID *uuid.UUID
	BankName        string
	AccountNumber   string
	AccountType     string
}

func (r *AssetRepo) CreateBankAccount(ctx context.Context, p CreateBankAccountParams) (*BankAccount, error) {
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
		Subtype:         "bank_account",
		OwnershipType:   p.OwnershipType,
		SoleOwnerUserID: p.SoleOwnerUserID,
		NativeCurrency:  p.NativeCurrency,
		CreatedBy:       &user,
	})
	if err != nil {
		return nil, fmt.Errorf("create asset: %w", err)
	}

	details, err := qtx.CreateBankAccountDetails(ctx, db.CreateBankAccountDetailsParams{
		AssetID:       asset.ID,
		BankName:      p.BankName,
		AccountNumber: p.AccountNumber,
		AccountType:   p.AccountType,
	})
	if err != nil {
		return nil, fmt.Errorf("create bank_account_details: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return &BankAccount{Asset: asset, Details: details}, nil
}

func (r *AssetRepo) GetBankAccount(ctx context.Context, id uuid.UUID) (*BankAccount, error) {
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
	if asset.Subtype != "bank_account" {
		return nil, ErrNotFound
	}

	details, err := r.q.GetBankAccountDetailsByAssetID(ctx, asset.ID)
	if err != nil {
		// Asset row exists but its extension is missing — schema invariant
		// violation. Surface as a real error rather than ErrNotFound.
		return nil, fmt.Errorf("get bank_account_details: %w", err)
	}

	return &BankAccount{Asset: asset, Details: details}, nil
}

func (r *AssetRepo) ListBankAccounts(ctx context.Context) ([]BankAccountListItem, error) {
	_, hid, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}

	subtype := "bank_account"
	assets, err := r.q.ListAssetsByHousehold(ctx, db.ListAssetsByHouseholdParams{
		HouseholdID: hid,
		Subtype:     &subtype,
	})
	if err != nil {
		return nil, fmt.Errorf("list assets: %w", err)
	}
	if len(assets) == 0 {
		return []BankAccountListItem{}, nil
	}

	ids := make([]uuid.UUID, len(assets))
	for i, a := range assets {
		ids[i] = a.ID
	}

	details, err := r.q.ListBankAccountDetailsByAssetIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("list bank_account_details: %w", err)
	}
	detailByID := make(map[uuid.UUID]db.BankAccountDetail, len(details))
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

	out := make([]BankAccountListItem, 0, len(assets))
	for _, a := range assets {
		item := BankAccountListItem{Asset: a, Details: detailByID[a.ID]}
		if s, ok := snapByID[a.ID]; ok {
			s := s
			item.LatestSnapshot = &s
		}
		out = append(out, item)
	}
	return out, nil
}

func (r *AssetRepo) UpdateBankAccount(ctx context.Context, id uuid.UUID, p UpdateBankAccountParams) (*BankAccount, error) {
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
	if asset.Subtype != "bank_account" {
		return nil, ErrNotFound
	}

	details, err := qtx.UpdateBankAccountDetails(ctx, db.UpdateBankAccountDetailsParams{
		AssetID:       asset.ID,
		BankName:      p.BankName,
		AccountNumber: p.AccountNumber,
		AccountType:   p.AccountType,
	})
	if err != nil {
		return nil, fmt.Errorf("update bank_account_details: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return &BankAccount{Asset: asset, Details: details}, nil
}

// BankAccountExport is everything export needs to render a full position
// workbook: the aggregate, the human-facing values for the two id-typed fields
// (owner email, tag name — resolved per the Detail-sheet conventions), and the
// full snapshot history.
type BankAccountExport struct {
	Account    BankAccount
	OwnerEmail string // sole_owner's email; "" for joint
	TagName    string // resolved tag name; "" when untagged
	Snapshots  []db.AssetSnapshot
}

// ExportBankAccount gathers a bank account, its resolved owner email + tag
// name, and its snapshot history, scoped + ownership-checked to the caller's
// household (404 via GetBankAccount when not owned or wrong subtype).
func (r *AssetRepo) ExportBankAccount(ctx context.Context, id uuid.UUID) (*BankAccountExport, error) {
	_, hid, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}

	account, err := r.GetBankAccount(ctx, id)
	if err != nil {
		return nil, err
	}

	out := &BankAccountExport{Account: *account}

	if uid := account.Asset.SoleOwnerUserID; uid != nil {
		user, err := r.q.GetUserByID(ctx, *uid)
		if err != nil {
			return nil, fmt.Errorf("export: resolve owner: %w", err)
		}
		out.OwnerEmail = user.Email
	}

	if tid := account.Asset.TagID; tid != nil {
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

func (r *AssetRepo) DeleteBankAccount(ctx context.Context, id uuid.UUID) error {
	// Verify the asset is actually a bank_account before deleting, so a
	// caller can't accidentally delete a property via this method.
	if _, err := r.GetBankAccount(ctx, id); err != nil {
		return err
	}
	return r.softDeleteAsset(ctx, id)
}
