package liabilities_test

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/kerti/balances-v2/backend/internal/repo"
)

// findLiabilitySeries returns the time-series item for one liability, failing
// if absent.
func findLiabilitySeries(t *testing.T, list []repo.LiabilityTimeSeries, id uuid.UUID) repo.LiabilityTimeSeries {
	t.Helper()
	for _, it := range list {
		if it.LiabilityID == id {
			return it
		}
	}
	t.Fatalf("no time series for liability %s", id)
	return repo.LiabilityTimeSeries{}
}

// TestLiabilityTimeSeries_ValueSeries asserts the per-liability value series
// tracks the posted snapshots in ascending month order, across the personal /
// institutional subtypes, with no cost basis.
func TestLiabilityTimeSeries_ValueSeries(t *testing.T) {
	h := newHarness(t)

	personal := h.createLiability(t, "Series personal loan", "personal")
	institutional := h.createLiability(t, "Series mortgage", "institutional")

	for _, s := range []struct{ ym, amount string }{
		{"2026-01", "50000000"},
		{"2026-02", "48000000"},
	} {
		rec := h.do(t, "POST", "/liabilities/"+personal.ID.String()+"/snapshots", map[string]any{
			"year_month": s.ym, "amount": s.amount, "currency": "IDR",
		})
		requireStatus(t, rec, http.StatusCreated)
	}
	rec := h.do(t, "POST", "/liabilities/"+institutional.ID.String()+"/snapshots", map[string]any{
		"year_month": "2026-02", "amount": "1400000000", "currency": "IDR",
	})
	requireStatus(t, rec, http.StatusCreated)

	rec = h.do(t, "GET", "/liabilities/time-series", nil)
	requireStatus(t, rec, http.StatusOK)
	list := decodeBody[[]repo.LiabilityTimeSeries](t, rec)

	series := findLiabilitySeries(t, list, personal.ID)
	want := []struct{ ym, amount string }{
		{"2026-01", "50000000"}, {"2026-02", "48000000"},
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

	instSeries := findLiabilitySeries(t, list, institutional.ID)
	if len(instSeries.ValueSeries) != 1 {
		t.Fatalf("institutional series length: want 1, got %d", len(instSeries.ValueSeries))
	}
}

// TestLiabilityTimeSeries_RequiresAuth guards the new route behind RequireAuth.
func TestLiabilityTimeSeries_RequiresAuth(t *testing.T) {
	h := newHarness(t)
	rec := h.doRaw(t, "GET", "/liabilities/time-series", nil, nil)
	requireStatus(t, rec, http.StatusUnauthorized)
}
