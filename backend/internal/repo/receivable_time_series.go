package repo

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/kerti/balances-v2/backend/internal/db"
)

// ReceivableTimeSeries carries one receivable's monthly value series for the
// Receivables list total-over-time chart (epic #204). Receivables are a flat
// group (no subtypes) with no cost basis, so this is the simplest value-only
// sibling of InvestmentTimeSeries — no CostSeries.
type ReceivableTimeSeries struct {
	ReceivableID uuid.UUID    `json:"receivable_id"`
	ValueSeries  []ValuePoint `json:"value_series"`
}

// ReceivableTimeSeries builds the per-receivable value series for every
// receivable in the household in one shot (epic #204). Household-scoped via
// ListReceivablesByHousehold; the snapshot batch is keyed to those owned ids,
// so it can never surface another household's rows. Native amounts only — no
// FX (14c convention).
func (r *ReceivableRepo) ReceivableTimeSeries(ctx context.Context) ([]ReceivableTimeSeries, error) {
	_, hid, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}

	recs, err := r.q.ListReceivablesByHousehold(ctx, hid)
	if err != nil {
		return nil, fmt.Errorf("list receivables: %w", err)
	}
	if len(recs) == 0 {
		return []ReceivableTimeSeries{}, nil
	}

	ids := make([]uuid.UUID, len(recs))
	for i, rec := range recs {
		ids[i] = rec.ID
	}

	snaps, err := r.q.ListReceivableSnapshotsByReceivableIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("list receivable snapshots: %w", err)
	}
	snapsByID := make(map[uuid.UUID][]db.ReceivableSnapshot)
	for _, s := range snaps {
		snapsByID[s.ReceivableID] = append(snapsByID[s.ReceivableID], s)
	}

	out := make([]ReceivableTimeSeries, 0, len(recs))
	for _, rec := range recs {
		positionSnaps := snapsByID[rec.ID]
		valueSeries := make([]ValuePoint, 0, len(positionSnaps))
		for _, s := range positionSnaps {
			valueSeries = append(valueSeries, ValuePoint{YearMonth: s.YearMonth, Amount: s.Amount})
		}
		out = append(out, ReceivableTimeSeries{ReceivableID: rec.ID, ValueSeries: valueSeries})
	}
	return out, nil
}
