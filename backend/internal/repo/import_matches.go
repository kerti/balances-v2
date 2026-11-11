package repo

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/kerti/balances-v2/backend/internal/db"
)

// PositionMatch is an existing position that a create-from-file import would
// duplicate (issue #828). Group is the engine's wire value (asset, liability,
// receivable, investment) and Subtype the DB subtype column — together they are
// all the client needs to build the position's detail route. Subtype is empty
// for a receivable, which has none. Status is the lifecycle status, so the
// preview can say a match is closed. CreatedAt tells same-named duplicates
// apart in the preview.
type PositionMatch struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	Group     string    `json:"group"`
	Subtype   string    `json:"subtype"`
	CreatedAt time.Time `json:"created_at"`
}

// FindImportMatches lists the caller's household's assets of subtype that share
// a display name (case-insensitive, trimmed) with name. For a bank account the
// account number must match too (trimmed); pass nil for the subtypes that have
// no identifier. Terminated assets are included, soft-deleted ones are not. The
// household filter lives in the SQL.
func (r *AssetRepo) FindImportMatches(ctx context.Context, subtype, name string, accountNumber *string) ([]PositionMatch, error) {
	_, hid, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := r.q.FindAssetImportMatches(ctx, db.FindAssetImportMatchesParams{
		HouseholdID:   hid,
		Subtype:       subtype,
		DisplayName:   strings.TrimSpace(name),
		AccountNumber: accountNumber,
	})
	if err != nil {
		return nil, fmt.Errorf("find asset import matches: %w", err)
	}
	out := make([]PositionMatch, len(rows))
	for i, m := range rows {
		out[i] = PositionMatch{ID: m.ID, Name: m.DisplayName, Status: m.Status, Group: "asset", Subtype: m.Subtype, CreatedAt: m.CreatedAt.Time}
	}
	return out, nil
}

// FindImportMatches is the investment counterpart: same subtype + same display
// name (case-insensitive, trimmed), terminated included, household-scoped.
func (r *InvestmentRepo) FindImportMatches(ctx context.Context, subtype, name string) ([]PositionMatch, error) {
	_, hid, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := r.q.FindInvestmentImportMatches(ctx, db.FindInvestmentImportMatchesParams{
		HouseholdID: hid,
		Subtype:     subtype,
		DisplayName: strings.TrimSpace(name),
	})
	if err != nil {
		return nil, fmt.Errorf("find investment import matches: %w", err)
	}
	out := make([]PositionMatch, len(rows))
	for i, m := range rows {
		out[i] = PositionMatch{ID: m.ID, Name: m.DisplayName, Status: m.Status, Group: "investment", Subtype: m.Subtype, CreatedAt: m.CreatedAt.Time}
	}
	return out, nil
}

// FindImportMatches is the liability counterpart: same subtype (personal /
// institutional) + same display name, terminated included, household-scoped.
func (r *LiabilityRepo) FindImportMatches(ctx context.Context, subtype, name string) ([]PositionMatch, error) {
	_, hid, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := r.q.FindLiabilityImportMatches(ctx, db.FindLiabilityImportMatchesParams{
		HouseholdID: hid,
		Subtype:     subtype,
		DisplayName: strings.TrimSpace(name),
	})
	if err != nil {
		return nil, fmt.Errorf("find liability import matches: %w", err)
	}
	out := make([]PositionMatch, len(rows))
	for i, m := range rows {
		out[i] = PositionMatch{ID: m.ID, Name: m.DisplayName, Status: m.Status, Group: "liability", Subtype: m.Subtype, CreatedAt: m.CreatedAt.Time}
	}
	return out, nil
}

// FindImportMatches is the receivable counterpart. Receivables have no
// subtype, so the display name alone decides.
func (r *ReceivableRepo) FindImportMatches(ctx context.Context, name string) ([]PositionMatch, error) {
	_, hid, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := r.q.FindReceivableImportMatches(ctx, db.FindReceivableImportMatchesParams{
		HouseholdID: hid,
		DisplayName: strings.TrimSpace(name),
	})
	if err != nil {
		return nil, fmt.Errorf("find receivable import matches: %w", err)
	}
	out := make([]PositionMatch, len(rows))
	for i, m := range rows {
		out[i] = PositionMatch{ID: m.ID, Name: m.DisplayName, Status: m.Status, Group: "receivable", CreatedAt: m.CreatedAt.Time}
	}
	return out, nil
}
