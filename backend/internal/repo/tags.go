package repo

import (
	"context"
	"encoding/json"
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
// at most one via a nullable tag_id on its shared parent table. A Tag is set
// through the position's own create/update (tag_id, same transaction; issue
// #839) or in bulk from the Tags page (AssignTagBatch).
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

// TagChange is a position write's tag_id field with the three states the wire
// can express: absent (Set false: leave the Tag alone), null (Set true, ID nil:
// clear it) and a uuid (Set true: assign it). A create ignores the difference
// between absent and null; an update acts on it. Decoding it directly off the
// request body is what lets the handler tell the first two apart.
type TagChange struct {
	Set bool
	ID  *uuid.UUID
}

// UnmarshalJSON runs only when the key is present (encoding/json calls it for a
// literal null too, on a non-pointer field), so Set is true by construction.
func (t *TagChange) UnmarshalJSON(b []byte) error {
	t.Set = true
	if string(b) == "null" {
		t.ID = nil
		return nil
	}
	var id uuid.UUID
	if err := json.Unmarshal(b, &id); err != nil {
		return err
	}
	t.ID = &id
	return nil
}

// assignTag sets (or, with a nil tagID, clears) the Tag on one Position, on
// whichever Queries it is handed: the plain pool for AssignTag, a transaction
// for a position create/update so the Tag lands atomically with the write
// (issue #839). A non-nil Tag is looked up under the caller's household_id in
// SQL first: a Tag from another household, or a deleted one, is ErrNotFound,
// never a silent cross-tenant link and never an existence leak. The per-group
// UPDATE filters the Position by household_id too (belt + suspenders); zero rows
// affected means the Position isn't in this household -> ErrNotFound.
func assignTag(ctx context.Context, q *db.Queries, hid, user uuid.UUID, group TagGroup, positionID uuid.UUID, tagID *uuid.UUID) error {
	if !group.valid() {
		return ErrNotFound
	}
	if tagID != nil {
		if _, err := q.GetTagByID(ctx, db.GetTagByIDParams{ID: *tagID, HouseholdID: hid}); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			return fmt.Errorf("get tag: %w", err)
		}
	}

	var (
		rows int64
		err  error
	)
	switch group {
	case TagGroupAsset:
		rows, err = q.AssignAssetTag(ctx, db.AssignAssetTagParams{TagID: tagID, ID: positionID, HouseholdID: hid, UpdatedBy: &user})
	case TagGroupLiability:
		rows, err = q.AssignLiabilityTag(ctx, db.AssignLiabilityTagParams{TagID: tagID, ID: positionID, HouseholdID: hid, UpdatedBy: &user})
	case TagGroupReceivable:
		rows, err = q.AssignReceivableTag(ctx, db.AssignReceivableTagParams{TagID: tagID, ID: positionID, HouseholdID: hid, UpdatedBy: &user})
	case TagGroupInvestment:
		rows, err = q.AssignInvestmentTag(ctx, db.AssignInvestmentTagParams{TagID: tagID, ID: positionID, HouseholdID: hid, UpdatedBy: &user})
	}
	if err != nil {
		return fmt.Errorf("assign %s tag: %w", group, err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// applyTagChange runs an update's TagChange inside its transaction: unset does
// nothing, otherwise it assigns or clears. It returns the tag_id the row now
// carries so the caller can reflect it in the aggregate it returns.
func applyTagChange(ctx context.Context, q *db.Queries, hid, user uuid.UUID, group TagGroup, positionID uuid.UUID, c TagChange, current *uuid.UUID) (*uuid.UUID, error) {
	if !c.Set {
		return current, nil
	}
	if err := assignTag(ctx, q, hid, user, group, positionID, c.ID); err != nil {
		return nil, err
	}
	return c.ID, nil
}

// AssignTag sets (or, with a nil tagID, clears) the Tag on a Position outside
// any position write. HTTP no longer exposes it (a Tag is edited in the position
// dialogs, and in bulk from the Tags page via AssignTagBatch); it remains for the
// demo seed and tests.
func (r *TagRepo) AssignTag(ctx context.Context, group TagGroup, positionID uuid.UUID, tagID *uuid.UUID) error {
	user, hid, err := currentUser(ctx)
	if err != nil {
		return err
	}
	return assignTag(ctx, r.q, hid, user, group, positionID, tagID)
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
