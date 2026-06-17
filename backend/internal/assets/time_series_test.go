package assets_test

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/kerti/balances-v2/backend/internal/repo"
)

// findAssetSeries returns the time-series item for one asset, failing if absent.
func findAssetSeries(t *testing.T, list []repo.AssetTimeSeries, id uuid.UUID) repo.AssetTimeSeries {
	t.Helper()
	for _, it := range list {
		if it.AssetID == id {
			return it
		}
	}
	t.Fatalf("no time series for asset %s", id)
	return repo.AssetTimeSeries{}
}

// TestAssetTimeSeries_ValueSeries asserts the per-asset value series tracks the
// posted snapshots in ascending month order, across the three asset subtypes
// (bank_account / property / vehicle — ADR-0022 shared snapshot table), with
// no cost basis. Assets carry no ledger, so the series is purely the snapshot
// values keyed to their months.
func TestAssetTimeSeries_ValueSeries(t *testing.T) {
	h := newHarness(t)

	bank := h.createBankAccount(t, "Series bank")
	prop := h.createProperty(t, "Series property")
	bankID := bank.Asset.ID

	for _, s := range []struct{ ym, amount string }{
		{"2026-01", "1000000"},
		{"2026-02", "1500000"},
		{"2026-03", "1200000"},
	} {
		rec := h.do(t, "POST", "/assets/"+bankID.String()+"/snapshots", map[string]any{
			"year_month": s.ym, "amount": s.amount, "currency": "IDR",
		})
		requireStatus(t, rec, http.StatusCreated)
	}
	// A single property snapshot, to prove multi-asset isolation within the
	// household (each asset gets its own series).
	rec := h.do(t, "POST", "/assets/"+prop.Asset.ID.String()+"/snapshots", map[string]any{
		"year_month": "2026-02", "amount": "750000000", "currency": "IDR",
	})
	requireStatus(t, rec, http.StatusCreated)

	rec = h.do(t, "GET", "/assets/time-series", nil)
	requireStatus(t, rec, http.StatusOK)
	list := decodeBody[[]repo.AssetTimeSeries](t, rec)

	series := findAssetSeries(t, list, bankID)
	want := []struct{ ym, amount string }{
		{"2026-01", "1000000"}, {"2026-02", "1500000"}, {"2026-03", "1200000"},
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

	propSeries := findAssetSeries(t, list, prop.Asset.ID)
	if len(propSeries.ValueSeries) != 1 {
		t.Fatalf("property series length: want 1, got %d", len(propSeries.ValueSeries))
	}
}

// TestAssetTimeSeries_RequiresAuth guards the new route behind RequireAuth.
func TestAssetTimeSeries_RequiresAuth(t *testing.T) {
	h := newHarness(t)
	rec := h.doRaw(t, "GET", "/assets/time-series", nil, nil)
	requireStatus(t, rec, http.StatusUnauthorized)
}
