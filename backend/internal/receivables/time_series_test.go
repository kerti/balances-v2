package receivables_test

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/kerti/balances-v2/backend/internal/repo"
)

// findReceivableSeries returns the time-series item for one receivable,
// failing if absent.
func findReceivableSeries(t *testing.T, list []repo.ReceivableTimeSeries, id uuid.UUID) repo.ReceivableTimeSeries {
	t.Helper()
	for _, it := range list {
		if it.ReceivableID == id {
			return it
		}
	}
	t.Fatalf("no time series for receivable %s", id)
	return repo.ReceivableTimeSeries{}
}

// TestReceivableTimeSeries_ValueSeries asserts the per-receivable value series
// tracks the posted snapshots in ascending month order, with no cost basis.
func TestReceivableTimeSeries_ValueSeries(t *testing.T) {
	h := newHarness(t)

	rcv := h.createReceivable(t, "Series loan")

	for _, s := range []struct{ ym, amount string }{
		{"2026-01", "50000000"},
		{"2026-02", "30000000"},
	} {
		rec := h.do(t, "POST", "/receivables/"+rcv.ID.String()+"/snapshots", map[string]any{
			"year_month": s.ym, "amount": s.amount, "currency": "IDR",
		})
		requireStatus(t, rec, http.StatusCreated)
	}

	rec := h.do(t, "GET", "/receivables/time-series", nil)
	requireStatus(t, rec, http.StatusOK)
	list := decodeBody[[]repo.ReceivableTimeSeries](t, rec)

	series := findReceivableSeries(t, list, rcv.ID)
	want := []struct{ ym, amount string }{
		{"2026-01", "50000000"}, {"2026-02", "30000000"},
	}
	if len(series.ValueSeries) != len(want) {
		t.Fatalf("value series length: want %d, got %d", len(want), len(series.ValueSeries))
	}
	for i, w := range want {
		if got := series.ValueSeries[i].YearMonth.Format("2006-01"); got != w.ym {
			t.Errorf("value[%d] month: want %s, got %s", i, w.ym, got)
		}
		if !decimal.RequireFromString(w.amount).Equal(series.ValueSeries[i].Amount) {
			t.Errorf("value[%d] amount: want %s, got %s", i, w.amount, series.ValueSeries[i].Amount)
		}
	}
}

// TestReceivableTimeSeries_RequiresAuth guards the new route behind RequireAuth.
func TestReceivableTimeSeries_RequiresAuth(t *testing.T) {
	h := newHarness(t)
	rec := h.doRaw(t, "GET", "/receivables/time-series", nil, nil)
	requireStatus(t, rec, http.StatusUnauthorized)
}
