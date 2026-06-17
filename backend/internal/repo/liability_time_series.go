package repo

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/kerti/balances-v2/backend/internal/db"
)

// LiabilityTimeSeries carries one liability's monthly value (outstanding-owed)
// series for the Liabilities Home time graphs (epic #204). Like assets,
// liabilities have no cost basis / no ledger, so this is the value-only
// sibling of InvestmentTimeSeries — no CostSeries.
type LiabilityTimeSeries struct {
	LiabilityID uuid.UUID    `json:"liability_id"`
	ValueSeries []ValuePoint `json:"value_series"`
}

// LiabilityTimeSeries builds the per-liability value series for every
// liability in the household in one shot (epic #204). Household-scoped via
// ListLiabilitiesByHousehold; the snapshot batch is keyed to those owned ids,
// so it can never surface another household's rows. Native amounts only — no
// FX (14c convention).
func (r *LiabilityRepo) LiabilityTimeSeries(ctx context.Context) ([]LiabilityTimeSeries, error) {
	_, hid, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}

	liabs, err := r.q.ListLiabilitiesByHousehold(ctx, db.ListLiabilitiesByHouseholdParams{HouseholdID: hid})
	if err != nil {
		return nil, fmt.Errorf("list liabilities: %w", err)
	}
	if len(liabs) == 0 {
		return []LiabilityTimeSeries{}, nil
	}

	ids := make([]uuid.UUID, len(liabs))
	for i, l := range liabs {
		ids[i] = l.ID
	}

	snaps, err := r.q.ListLiabilitySnapshotsByLiabilityIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("list liability snapshots: %w", err)
	}
	snapsByID := make(map[uuid.UUID][]db.LiabilitySnapshot)
	for _, s := range snaps {
		snapsByID[s.LiabilityID] = append(snapsByID[s.LiabilityID], s)
	}

	out := make([]LiabilityTimeSeries, 0, len(liabs))
	for _, l := range liabs {
		positionSnaps := snapsByID[l.ID]
		valueSeries := make([]ValuePoint, 0, len(positionSnaps))
		for _, s := range positionSnaps {
			valueSeries = append(valueSeries, ValuePoint{YearMonth: s.YearMonth, Amount: s.Amount})
		}
		out = append(out, LiabilityTimeSeries{LiabilityID: l.ID, ValueSeries: valueSeries})
	}
	return out, nil
}
