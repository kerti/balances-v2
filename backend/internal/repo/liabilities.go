package repo

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/kerti/balances-v2/backend/internal/db"
)

// LiabilityRepo wraps the generated queries for the Liability position group.
// Liabilities use inline metadata (no extension table) and carry a subtype
// enum ('personal' | 'institutional'). Liability snapshots share their own
// per-group table (see ADR-0022) and live here since they're not
// subtype-specific.
type LiabilityRepo struct {
	pool *pgxpool.Pool
	q    *db.Queries
}

func NewLiabilityRepo(pool *pgxpool.Pool) *LiabilityRepo {
	return &LiabilityRepo{pool: pool, q: db.New(pool)}
}

// LiabilityListItem extends a Liability row with the most-recent Snapshot if
// any (for list views).
type LiabilityListItem struct {
	Liability      db.Liability          `json:"liability"`
	LatestSnapshot *db.LiabilitySnapshot `json:"latest_snapshot"`
}

type CreateLiabilityParams struct {
	DisplayName      string
	Description      *string
	Subtype          string // "personal" | "institutional"
	OwnershipType    string // "sole" | "joint"
	SoleOwnerUserID  *uuid.UUID
	NativeCurrency   string
	CounterpartyName string
	Principal        *decimal.Decimal
	InterestRate     *decimal.Decimal
	TermMonths       *int32
	StartDate        *time.Time
	MaturityDate     *time.Time
}

type UpdateLiabilityParams struct {
	DisplayName      string
	Description      *string
	OwnershipType    string
	SoleOwnerUserID  *uuid.UUID
	CounterpartyName string
	Principal        *decimal.Decimal
	InterestRate     *decimal.Decimal
	TermMonths       *int32
	StartDate        *time.Time
	MaturityDate     *time.Time
}

func (r *LiabilityRepo) CreateLiability(ctx context.Context, p CreateLiabilityParams) (*db.Liability, error) {
	user, hid, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}

	row, err := r.q.CreateLiability(ctx, db.CreateLiabilityParams{
		HouseholdID:      hid,
		DisplayName:      p.DisplayName,
		Description:      p.Description,
		Subtype:          p.Subtype,
		OwnershipType:    p.OwnershipType,
		SoleOwnerUserID:  p.SoleOwnerUserID,
		NativeCurrency:   p.NativeCurrency,
		CounterpartyName: p.CounterpartyName,
		Principal:        p.Principal,
		InterestRate:     p.InterestRate,
		TermMonths:       p.TermMonths,
		StartDate:        p.StartDate,
		MaturityDate:     p.MaturityDate,
		CreatedBy:        &user,
	})
	if err != nil {
		return nil, fmt.Errorf("create liability: %w", err)
	}
	return &row, nil
}

func (r *LiabilityRepo) GetLiability(ctx context.Context, id uuid.UUID) (*db.Liability, error) {
	_, hid, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	row, err := r.q.GetLiabilityByID(ctx, db.GetLiabilityByIDParams{ID: id, HouseholdID: hid})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &row, nil
}

func (r *LiabilityRepo) ListLiabilities(ctx context.Context, subtype *string) ([]LiabilityListItem, error) {
	_, hid, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := r.q.ListLiabilitiesByHousehold(ctx, db.ListLiabilitiesByHouseholdParams{
		HouseholdID: hid,
		Subtype:     subtype,
	})
	if err != nil {
		return nil, fmt.Errorf("list liabilities: %w", err)
	}
	if len(rows) == 0 {
		return []LiabilityListItem{}, nil
	}

	ids := make([]uuid.UUID, len(rows))
	for i, l := range rows {
		ids[i] = l.ID
	}

	snapshots, err := r.q.ListLatestLiabilitySnapshotsByLiabilityIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("list latest liability snapshots: %w", err)
	}
	snapByID := make(map[uuid.UUID]db.LiabilitySnapshot, len(snapshots))
	for _, s := range snapshots {
		snapByID[s.LiabilityID] = s
	}

	out := make([]LiabilityListItem, 0, len(rows))
	for _, l := range rows {
		item := LiabilityListItem{Liability: l}
		if s, ok := snapByID[l.ID]; ok {
			s := s
			item.LatestSnapshot = &s
		}
		out = append(out, item)
	}
	return out, nil
}

func (r *LiabilityRepo) UpdateLiability(ctx context.Context, id uuid.UUID, p UpdateLiabilityParams) (*db.Liability, error) {
	user, hid, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	row, err := r.q.UpdateLiability(ctx, db.UpdateLiabilityParams{
		ID:               id,
		HouseholdID:      hid,
		DisplayName:      p.DisplayName,
		Description:      p.Description,
		OwnershipType:    p.OwnershipType,
		SoleOwnerUserID:  p.SoleOwnerUserID,
		CounterpartyName: p.CounterpartyName,
		Principal:        p.Principal,
		InterestRate:     p.InterestRate,
		TermMonths:       p.TermMonths,
		StartDate:        p.StartDate,
		MaturityDate:     p.MaturityDate,
		UpdatedBy:        &user,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("update liability: %w", err)
	}
	return &row, nil
}

func (r *LiabilityRepo) DeleteLiability(ctx context.Context, id uuid.UUID) error {
	user, hid, err := currentUser(ctx)
	if err != nil {
		return err
	}
	rows, err := r.q.SoftDeleteLiability(ctx, db.SoftDeleteLiabilityParams{
		ID:          id,
		HouseholdID: hid,
		UpdatedBy:   &user,
	})
	if err != nil {
		return fmt.Errorf("soft delete liability: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// LiabilityExport is everything export needs to render a full position
// workbook: the liability row, the human-facing values for the two id-typed
// fields (owner email, tag name — resolved per the Detail-sheet conventions),
// and the full snapshot history.
type LiabilityExport struct {
	Liability  db.Liability
	OwnerEmail string // sole_owner's email; "" for joint
	TagName    string // resolved tag name; "" when untagged
	Snapshots  []db.LiabilitySnapshot
}

// ExportLiability gathers a liability, its resolved owner email + tag name, and
// its snapshot history, scoped + ownership-checked to the caller's household
// (404 via GetLiability when not owned).
func (r *LiabilityRepo) ExportLiability(ctx context.Context, id uuid.UUID) (*LiabilityExport, error) {
	_, hid, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}

	liability, err := r.GetLiability(ctx, id)
	if err != nil {
		return nil, err
	}

	out := &LiabilityExport{Liability: *liability}

	if uid := liability.SoleOwnerUserID; uid != nil {
		user, err := r.q.GetUserByID(ctx, *uid)
		if err != nil {
			return nil, fmt.Errorf("export: resolve owner: %w", err)
		}
		out.OwnerEmail = user.Email
	}

	if tid := liability.TagID; tid != nil {
		tag, err := r.q.GetTagByID(ctx, db.GetTagByIDParams{ID: *tid, HouseholdID: hid})
		if err != nil {
			return nil, fmt.Errorf("export: resolve tag: %w", err)
		}
		out.TagName = tag.Name
	}

	snaps, err := r.q.ListLiabilitySnapshotsForLiability(ctx, db.ListLiabilitySnapshotsForLiabilityParams{
		LiabilityID: id,
		HouseholdID: hid,
	})
	if err != nil {
		return nil, fmt.Errorf("export: list snapshots: %w", err)
	}
	out.Snapshots = snaps

	return out, nil
}

// LookupUserIDByEmail resolves a household member's email back to their user id
// — the inverse of the Detail-sheet's sole_owner convention. Case-insensitive
// on a trimmed email; found is false when no member of the caller's household
// has that email (the handler turns that into a per-field error, not a 404).
func (r *LiabilityRepo) LookupUserIDByEmail(ctx context.Context, email string) (id uuid.UUID, found bool, err error) {
	_, hid, err := currentUser(ctx)
	if err != nil {
		return uuid.Nil, false, err
	}
	users, err := r.q.ListUsersByHousehold(ctx, hid)
	if err != nil {
		return uuid.Nil, false, fmt.Errorf("lookup user by email: %w", err)
	}
	want := strings.ToLower(strings.TrimSpace(email))
	for _, u := range users {
		if strings.ToLower(u.Email) == want {
			return u.ID, true, nil
		}
	}
	return uuid.Nil, false, nil
}

// LookupTagIDByName resolves a Tag name back to its id within the caller's
// household — the inverse of the Detail-sheet's tag convention. Returns
// (nil, nil) when no Tag matches: per the create-import contract an unmatched
// tag is left unassigned rather than erroring. Exact match on a trimmed name.
func (r *LiabilityRepo) LookupTagIDByName(ctx context.Context, name string) (*uuid.UUID, error) {
	_, hid, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	want := strings.TrimSpace(name)
	if want == "" {
		return nil, nil
	}
	tags, err := r.q.ListTagsByHousehold(ctx, hid)
	if err != nil {
		return nil, fmt.Errorf("lookup tag by name: %w", err)
	}
	for _, tg := range tags {
		if tg.Name == want {
			tid := tg.ID
			return &tid, nil
		}
	}
	return nil, nil
}

// CreateLiabilityWithSnapshots creates a liability and seeds its snapshot
// history in a single transaction (all-or-nothing) — the commit path of the
// create-from-file import. It mirrors CreateLiability, then optionally assigns
// the resolved Tag and upserts every snapshot row, all under one tx so a
// mid-batch failure leaves nothing behind. tagID nil leaves the position
// untagged; an empty snaps seeds no history.
func (r *LiabilityRepo) CreateLiabilityWithSnapshots(ctx context.Context, p CreateLiabilityParams, tagID *uuid.UUID, snaps []ImportSnapshotRow) (*db.Liability, error) {
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
	row, err := qtx.CreateLiability(ctx, db.CreateLiabilityParams{
		HouseholdID:      hid,
		DisplayName:      p.DisplayName,
		Description:      p.Description,
		Subtype:          p.Subtype,
		OwnershipType:    p.OwnershipType,
		SoleOwnerUserID:  p.SoleOwnerUserID,
		NativeCurrency:   p.NativeCurrency,
		CounterpartyName: p.CounterpartyName,
		Principal:        p.Principal,
		InterestRate:     p.InterestRate,
		TermMonths:       p.TermMonths,
		StartDate:        p.StartDate,
		MaturityDate:     p.MaturityDate,
		CreatedBy:        &user,
	})
	if err != nil {
		return nil, fmt.Errorf("create liability: %w", err)
	}

	if tagID != nil {
		if _, err := qtx.AssignLiabilityTag(ctx, db.AssignLiabilityTagParams{
			TagID:       tagID,
			ID:          row.ID,
			HouseholdID: hid,
			UpdatedBy:   &user,
		}); err != nil {
			return nil, fmt.Errorf("assign tag: %w", err)
		}
		row.TagID = tagID
	}

	for _, s := range snaps {
		if _, err := qtx.UpsertLiabilitySnapshot(ctx, db.UpsertLiabilitySnapshotParams{
			ID:          row.ID,
			YearMonth:   s.YearMonth,
			Amount:      s.Amount,
			Currency:    s.Currency,
			AsOfDate:    s.AsOfDate,
			Description: s.Description,
			CreatedBy:   &user,
			HouseholdID: hid,
		}); err != nil {
			return nil, fmt.Errorf("import: upsert %s: %w", s.YearMonth.Format("2006-01"), err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return &row, nil
}

// ----- liability snapshots -----------------------------------------------

type CreateLiabilitySnapshotParams struct {
	LiabilityID uuid.UUID
	YearMonth   time.Time
	Amount      decimal.Decimal
	Currency    string
	AsOfDate    *time.Time
	Description *string
}

type UpdateLiabilitySnapshotParams struct {
	SnapshotID  uuid.UUID
	Amount      decimal.Decimal
	Currency    string
	AsOfDate    *time.Time
	Description *string
}

func (r *LiabilityRepo) CreateLiabilitySnapshot(ctx context.Context, p CreateLiabilitySnapshotParams) (*db.LiabilitySnapshot, error) {
	user, hid, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	snap, err := r.q.CreateLiabilitySnapshot(ctx, db.CreateLiabilitySnapshotParams{
		ID:          p.LiabilityID,
		YearMonth:   p.YearMonth,
		Amount:      p.Amount,
		Currency:    p.Currency,
		AsOfDate:    p.AsOfDate,
		Description: p.Description,
		CreatedBy:   &user,
		HouseholdID: hid,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("create liability snapshot: %w", err)
	}
	return &snap, nil
}

func (r *LiabilityRepo) ListLiabilitySnapshots(ctx context.Context, liabilityID uuid.UUID) ([]db.LiabilitySnapshot, error) {
	_, hid, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	return r.q.ListLiabilitySnapshotsForLiability(ctx, db.ListLiabilitySnapshotsForLiabilityParams{
		LiabilityID: liabilityID,
		HouseholdID: hid,
	})
}

func (r *LiabilityRepo) UpdateLiabilitySnapshot(ctx context.Context, p UpdateLiabilitySnapshotParams) (*db.LiabilitySnapshot, error) {
	user, hid, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	snap, err := r.q.UpdateLiabilitySnapshot(ctx, db.UpdateLiabilitySnapshotParams{
		ID:          p.SnapshotID,
		HouseholdID: hid,
		Amount:      p.Amount,
		Currency:    p.Currency,
		AsOfDate:    p.AsOfDate,
		Description: p.Description,
		UpdatedBy:   &user,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("update liability snapshot: %w", err)
	}
	return &snap, nil
}

func (r *LiabilityRepo) DeleteLiabilitySnapshot(ctx context.Context, snapshotID uuid.UUID) error {
	user, hid, err := currentUser(ctx)
	if err != nil {
		return err
	}
	rows, err := r.q.SoftDeleteLiabilitySnapshot(ctx, db.SoftDeleteLiabilitySnapshotParams{
		ID:          snapshotID,
		HouseholdID: hid,
		UpdatedBy:   &user,
	})
	if err != nil {
		return fmt.Errorf("soft delete liability snapshot: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}
