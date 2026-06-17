package repo

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/kerti/balances-v2/backend/internal/db"
)

// AssetTimeSeries carries one asset's monthly value series for the Assets Home
// time graphs (epic #204). Assets have no cost basis / no ledger (ADR-0022
// shared snapshot table across bank_account / property / vehicle), so this is
// the value-only sibling of InvestmentTimeSeries — no CostSeries.
type AssetTimeSeries struct {
	AssetID     uuid.UUID    `json:"asset_id"`
	ValueSeries []ValuePoint `json:"value_series"`
}

// AssetTimeSeries builds the per-asset value series for every asset in the
// household in one shot (epic #204), so the Assets Home time graph needs no
// per-asset snapshot fan-out. Household-scoped via ListAssetsByHousehold; the
// snapshot batch is keyed to those owned ids, so it can never surface another
// household's rows. Native amounts only — no FX (14c convention).
func (r *AssetRepo) AssetTimeSeries(ctx context.Context) ([]AssetTimeSeries, error) {
	_, hid, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}

	assets, err := r.q.ListAssetsByHousehold(ctx, db.ListAssetsByHouseholdParams{HouseholdID: hid})
	if err != nil {
		return nil, fmt.Errorf("list assets: %w", err)
	}
	if len(assets) == 0 {
		return []AssetTimeSeries{}, nil
	}

	ids := make([]uuid.UUID, len(assets))
	for i, a := range assets {
		ids[i] = a.ID
	}

	snaps, err := r.q.ListAssetSnapshotsByAssetIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("list asset snapshots: %w", err)
	}
	snapsByID := make(map[uuid.UUID][]db.AssetSnapshot)
	for _, s := range snaps {
		snapsByID[s.AssetID] = append(snapsByID[s.AssetID], s)
	}

	out := make([]AssetTimeSeries, 0, len(assets))
	for _, a := range assets {
		positionSnaps := snapsByID[a.ID]
		valueSeries := make([]ValuePoint, 0, len(positionSnaps))
		for _, s := range positionSnaps {
			valueSeries = append(valueSeries, ValuePoint{YearMonth: s.YearMonth, Amount: s.Amount})
		}
		out = append(out, AssetTimeSeries{AssetID: a.ID, ValueSeries: valueSeries})
	}
	return out, nil
}
