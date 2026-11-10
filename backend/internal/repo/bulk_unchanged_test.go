package repo_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"

	"github.com/kerti/balances-v2/backend/internal/db"
	"github.com/kerti/balances-v2/backend/internal/identity"
	"github.com/kerti/balances-v2/backend/internal/repo"
	"github.com/kerti/balances-v2/backend/internal/testutil"
)

// An "Unchanged" row from bulk entry (#789) is a row whose values equal the
// carried-forward ones. When the month has no live snapshot yet it must simply
// be created, and the position then leaves that month's stale list; saving the
// same values again is the existing no-op. Driven across every snapshot group
// and shape. A control position that was never marked stays stale.
// covers: INV-SNAPSHOTS-14
func TestBulkUnchanged_CreatesCarriedValueAndClearsStale(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)
	alice := testutil.CreateHouseholdWithUser(t, q, "Alice")
	ctx := identity.WithUser(context.Background(), alice)
	reports := repo.NewMonthlyReportRepo(tdb.Pool)

	staleIn := func(t *testing.T) map[uuid.UUID]bool {
		t.Helper()
		rows, err := reports.ListReports(ctx)
		if err != nil {
			t.Fatalf("ListReports: %v", err)
		}
		oct := mustMonth(t, rows, replOct)
		var stale []struct {
			PositionID uuid.UUID `json:"position_id"`
		}
		if err := json.Unmarshal(oct.StalePositions, &stale); err != nil {
			t.Fatalf("unmarshal stale_positions: %v", err)
		}
		out := map[uuid.UUID]bool{}
		for _, s := range stale {
			out[s.PositionID] = true
		}
		return out
	}

	for _, g := range replaceGroups(tdb.Pool) {
		t.Run(g.name, func(t *testing.T) {
			marked := g.newPosition(t, ctx)
			untouched := g.newPosition(t, ctx)
			g.create(t, ctx, marked, replSep, rd("500"))
			g.create(t, ctx, untouched, replSep, rd("500"))

			before := staleIn(t)
			if !before[marked] || !before[untouched] {
				t.Fatalf("both positions should be stale in Oct before saving; got %v", before)
			}

			// Unchanged: the carried value (500) saved as October's snapshot.
			res, rowErrs, err := g.bulk(ctx, marked, replOct, rd("500"), nil)
			if err != nil || len(rowErrs) > 0 {
				t.Fatalf("bulk: err=%v rowErrs=%v", err, rowErrs)
			}
			if res.Written != 1 || res.Replaced != 0 || res.Unchanged != 0 {
				t.Errorf("bulk result = %+v, want written=1", res)
			}

			after := staleIn(t)
			if after[marked] {
				t.Errorf("marked position still stale in Oct")
			}
			if !after[untouched] {
				t.Errorf("untouched position should stay stale in Oct")
			}

			// Saving the same values again writes and archives nothing.
			res, _, err = g.bulk(ctx, marked, replOct, rd("500"), nil)
			if err != nil {
				t.Fatalf("bulk again: %v", err)
			}
			if res.Written != 0 || res.Replaced != 0 || res.Unchanged != 1 {
				t.Errorf("repeat bulk result = %+v, want unchanged=1", res)
			}
		})
	}
}
