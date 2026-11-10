package repo

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/kerti/balances-v2/backend/internal/db"
)

// TagRepo wraps the generated queries for user-defined position Tags
// (ADR-0028). A Tag is a household-scoped grouping label; a Position carries
// at most one via a nullable tag_id on its shared parent table. Assignment is
// a dedicated path here rather than a field on each position's create/update,
// because a Tag is orthogonal to a Position's identity.
type TagRepo struct {
	pool *pgxpool.Pool
	q    *db.Queries
}

func NewTagRepo(pool *pgxpool.Pool) *TagRepo {
	return &TagRepo{pool: pool, q: db.New(pool)}
}

// TagGroup is the position-group discriminator the assign endpoint switches on.
type TagGroup string

const (
	TagGroupAsset      TagGroup = "asset"
	TagGroupLiability  TagGroup = "liability"
	TagGroupReceivable TagGroup = "receivable"
	TagGroupInvestment TagGroup = "investment"
)

func (g TagGroup) valid() bool {
	switch g {
	case TagGroupAsset, TagGroupLiability, TagGroupReceivable, TagGroupInvestment:
		return true
	default:
		return false
	}
}

func (r *TagRepo) CreateTag(ctx context.Context, name, color string) (*db.Tag, error) {
	user, hid, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	row, err := r.q.CreateTag(ctx, db.CreateTagParams{
		HouseholdID: hid,
		Name:        name,
		Color:       color,
		CreatedBy:   &user,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrTagNameExists
		}
		return nil, fmt.Errorf("create tag: %w", err)
	}
	return &row, nil
}

func (r *TagRepo) GetTag(ctx context.Context, id uuid.UUID) (*db.Tag, error) {
	_, hid, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	row, err := r.q.GetTagByID(ctx, db.GetTagByIDParams{ID: id, HouseholdID: hid})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &row, nil
}

func (r *TagRepo) ListTags(ctx context.Context) ([]db.Tag, error) {
	_, hid, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := r.q.ListTagsByHousehold(ctx, hid)
	if err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}
	if rows == nil {
		rows = []db.Tag{}
	}
	return rows, nil
}

func (r *TagRepo) UpdateTag(ctx context.Context, id uuid.UUID, name, color string) (*db.Tag, error) {
	user, hid, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	row, err := r.q.UpdateTag(ctx, db.UpdateTagParams{
		ID:          id,
		HouseholdID: hid,
		Name:        name,
		Color:       color,
		UpdatedBy:   &user,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		if isUniqueViolation(err) {
			return nil, ErrTagNameExists
		}
		return nil, fmt.Errorf("update tag: %w", err)
	}
	return &row, nil
}

// DeleteTag soft-deletes the Tag and clears it from every Position in the same
// transaction, so no Position is left pointing at a dead Tag — it falls back
// to Untagged (ADR-0028).
func (r *TagRepo) DeleteTag(ctx context.Context, id uuid.UUID) error {
	user, hid, err := currentUser(ctx)
	if err != nil {
		return err
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := r.q.WithTx(tx)

	rows, err := qtx.SoftDeleteTag(ctx, db.SoftDeleteTagParams{ID: id, HouseholdID: hid, UpdatedBy: &user})
	if err != nil {
		return fmt.Errorf("soft delete tag: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}

	if err := qtx.ClearAssetTag(ctx, db.ClearAssetTagParams{TagID: &id, HouseholdID: hid}); err != nil {
		return fmt.Errorf("clear asset tag: %w", err)
	}
	if err := qtx.ClearLiabilityTag(ctx, db.ClearLiabilityTagParams{TagID: &id, HouseholdID: hid}); err != nil {
		return fmt.Errorf("clear liability tag: %w", err)
	}
	if err := qtx.ClearReceivableTag(ctx, db.ClearReceivableTagParams{TagID: &id, HouseholdID: hid}); err != nil {
		return fmt.Errorf("clear receivable tag: %w", err)
	}
	if err := qtx.ClearInvestmentTag(ctx, db.ClearInvestmentTagParams{TagID: &id, HouseholdID: hid}); err != nil {
		return fmt.Errorf("clear investment tag: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// AssignTag sets (or, with a nil tagID, clears) the Tag on a Position. A
// non-nil Tag is validated for household ownership first — a Tag from another
// household is ErrNotFound, never a silent cross-tenant link. The per-group
// UPDATE filters the Position by household_id too (belt + suspenders); zero
// rows affected means the Position isn't in this household → ErrNotFound.
func (r *TagRepo) AssignTag(ctx context.Context, group TagGroup, positionID uuid.UUID, tagID *uuid.UUID) error {
	if !group.valid() {
		return ErrNotFound
	}
	user, hid, err := currentUser(ctx)
	if err != nil {
		return err
	}
	if tagID != nil {
		if _, err := r.GetTag(ctx, *tagID); err != nil {
			return err // ErrNotFound for missing / cross-tenant
		}
	}

	var rows int64
	switch group {
	case TagGroupAsset:
		rows, err = r.q.AssignAssetTag(ctx, db.AssignAssetTagParams{TagID: tagID, ID: positionID, HouseholdID: hid, UpdatedBy: &user})
	case TagGroupLiability:
		rows, err = r.q.AssignLiabilityTag(ctx, db.AssignLiabilityTagParams{TagID: tagID, ID: positionID, HouseholdID: hid, UpdatedBy: &user})
	case TagGroupReceivable:
		rows, err = r.q.AssignReceivableTag(ctx, db.AssignReceivableTagParams{TagID: tagID, ID: positionID, HouseholdID: hid, UpdatedBy: &user})
	case TagGroupInvestment:
		rows, err = r.q.AssignInvestmentTag(ctx, db.AssignInvestmentTagParams{TagID: tagID, ID: positionID, HouseholdID: hid, UpdatedBy: &user})
	}
	if err != nil {
		return fmt.Errorf("assign %s tag: %w", group, err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// TagBreakdown returns, per (tag, group, currency), the summed most-recent
// snapshot value of contributing Positions. tag_id NULL is the Untagged
// bucket; the report layer renders liabilities as their own negative slice.
func (r *TagRepo) TagBreakdown(ctx context.Context) ([]db.TagBreakdownByHouseholdRow, error) {
	_, hid, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := r.q.TagBreakdownByHousehold(ctx, hid)
	if err != nil {
		return nil, fmt.Errorf("tag breakdown: %w", err)
	}
	if rows == nil {
		rows = []db.TagBreakdownByHouseholdRow{}
	}
	return rows, nil
}

// TagItemRef names one Position in a batch assignment.
type TagItemRef struct {
	Group      TagGroup
	PositionID uuid.UUID
}

// AssignTagBatch sets (or, with a nil tagID, clears) the Tag on every listed
// Position in ONE transaction (#853). It enforces the same two-sided tenancy as
// AssignTag: a non-nil Tag must be this household's, and each group's UPDATE
// filters by household_id, so the affected-row count must equal the distinct ids
// sent for that group. Any shortfall (unknown, deleted or cross-household id)
// rolls the whole batch back as ErrNotFound: a batch is all-or-nothing.
func (r *TagRepo) AssignTagBatch(ctx context.Context, tagID *uuid.UUID, items []TagItemRef) error {
	user, hid, err := currentUser(ctx)
	if err != nil {
		return err
	}
	if tagID != nil {
		if _, err := r.GetTag(ctx, *tagID); err != nil {
			return err
		}
	}

	byGroup := map[TagGroup][]uuid.UUID{}
	seen := map[uuid.UUID]bool{}
	for _, it := range items {
		if !it.Group.valid() {
			return ErrNotFound
		}
		if seen[it.PositionID] {
			continue
		}
		seen[it.PositionID] = true
		byGroup[it.Group] = append(byGroup[it.Group], it.PositionID)
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := r.q.WithTx(tx)

	for group, ids := range byGroup {
		var rows int64
		switch group {
		case TagGroupAsset:
			rows, err = qtx.AssignAssetsTag(ctx, db.AssignAssetsTagParams{TagID: tagID, Column2: ids, HouseholdID: hid, UpdatedBy: &user})
		case TagGroupLiability:
			rows, err = qtx.AssignLiabilitiesTag(ctx, db.AssignLiabilitiesTagParams{TagID: tagID, Column2: ids, HouseholdID: hid, UpdatedBy: &user})
		case TagGroupReceivable:
			rows, err = qtx.AssignReceivablesTag(ctx, db.AssignReceivablesTagParams{TagID: tagID, Column2: ids, HouseholdID: hid, UpdatedBy: &user})
		case TagGroupInvestment:
			rows, err = qtx.AssignInvestmentsTag(ctx, db.AssignInvestmentsTagParams{TagID: tagID, Column2: ids, HouseholdID: hid, UpdatedBy: &user})
		}
		if err != nil {
			return fmt.Errorf("assign %s tags: %w", group, err)
		}
		if rows != int64(len(ids)) {
			return ErrNotFound
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// TaggableItem is one Position on the Tags page's expand lists and picker.
// Currency/Amount are nil until the Position has a snapshot. Terminated marks a
// Position that ended this month (still in the breakdown, not "active").
type TaggableItem struct {
	Group      TagGroup         `json:"group"`
	ID         uuid.UUID        `json:"id"`
	Subtype    string           `json:"subtype"`
	Name       string           `json:"name"`
	TagID      *uuid.UUID       `json:"tag_id"`
	Currency   *string          `json:"currency"`
	Amount     *decimal.Decimal `json:"amount"`
	Terminated bool             `json:"terminated"`
}

// ListTaggableItems returns every household Position the breakdown could count,
// tagged or not, name-ordered.
func (r *TagRepo) ListTaggableItems(ctx context.Context) ([]TaggableItem, error) {
	_, hid, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := r.q.ListTaggableItemsByHousehold(ctx, hid)
	if err != nil {
		return nil, fmt.Errorf("list taggable items: %w", err)
	}
	out := make([]TaggableItem, len(rows))
	for i, row := range rows {
		it := TaggableItem{
			Group:      TagGroup(row.Grp),
			ID:         row.ID,
			Subtype:    row.Subtype,
			Name:       row.DisplayName,
			TagID:      row.TagID,
			Terminated: row.TerminatedAt != nil,
		}
		if row.HasValue {
			cur, amt := row.Currency, row.Amount
			it.Currency, it.Amount = &cur, &amt
		}
		out[i] = it
	}
	return out, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
