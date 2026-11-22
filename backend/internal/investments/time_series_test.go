package investments_test

import (
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/kerti/balances-v2/backend/internal/repo"
)

// findSeries returns the time-series item for one investment, failing if absent.
func findSeries(t *testing.T, list []repo.InvestmentTimeSeries, id uuid.UUID) repo.InvestmentTimeSeries {
	t.Helper()
	for _, it := range list {
		if it.InvestmentID == id {
			return it
		}
	}
	t.Fatalf("no time series for investment %s", id)
	return repo.InvestmentTimeSeries{}
}

// TestInvestmentTimeSeries_StockLedger asserts the value series tracks
// snapshots and the cost series replays the ledger sampled at snapshot months:
// a Jan buy (1,000,000) then a Feb buy (500,000) yield cumulative cost
// 1,000,000 at Jan, 1,500,000 at Feb, and 1,500,000 carried into Mar (no Mar
// txn). The Bond/TD flat branches are covered by ..._FlatTimeDeposit.
func TestInvestmentTimeSeries_StockLedger(t *testing.T) {
	h := newHarness(t)
	stock := h.createStock(t, "Series stock")
	id := stock.Investment.ID

	h.postTxn(t, id, map[string]any{
		"transaction_type": "buy", "transaction_date": "2026-01-15", "currency": "IDR",
		"amount": "1000000", "quantity": "100", "price_per_unit": "10000",
	})
	h.postTxn(t, id, map[string]any{
		"transaction_type": "buy", "transaction_date": "2026-02-15", "currency": "IDR",
		"amount": "500000", "quantity": "50", "price_per_unit": "10000",
	})
	// The server derives amount = quantity × price_per_unit (#968), so each
	// month's price is chosen to give the amount the series is asserted against.
	for _, s := range []struct{ ym, amount, price string }{
		{"2026-01", "1100000", "11000"},
		{"2026-02", "1600000", "16000"},
		{"2026-03", "1700000", "17000"},
	} {
		rec := h.do(t, "POST", "/investments/"+id.String()+"/snapshots", map[string]any{
			"year_month": s.ym, "amount": s.amount, "currency": "IDR",
			"quantity": "100", "price_per_unit": s.price,
		})
		requireStatus(t, rec, http.StatusCreated)
	}

	rec := h.do(t, "GET", "/investments/time-series", nil)
	requireStatus(t, rec, http.StatusOK)
	series := findSeries(t, decodeBody[[]repo.InvestmentTimeSeries](t, rec), id)

	wantValue := []struct{ ym, amount string }{
		{"2026-01", "1100000"}, {"2026-02", "1600000"}, {"2026-03", "1700000"},
	}
	if len(series.ValueSeries) != len(wantValue) {
		t.Fatalf("value series length: want %d, got %d", len(wantValue), len(series.ValueSeries))
	}
	for i, w := range wantValue {
		if got := series.ValueSeries[i].YearMonth.Format("2006-01"); got != w.ym {
			t.Errorf("value[%d] month: want %s, got %s", i, w.ym, got)
		}
		requireCostBasis(t, series.ValueSeries[i].Amount, w.amount)
	}

	wantCost := []struct{ ym, cost string }{
		{"2026-01", "1000000"}, {"2026-02", "1500000"}, {"2026-03", "1500000"},
	}
	if len(series.CostSeries) != len(wantCost) {
		t.Fatalf("cost series length: want %d, got %d", len(wantCost), len(series.CostSeries))
	}
	for i, w := range wantCost {
		if got := series.CostSeries[i].YearMonth.Format("2006-01"); got != w.ym {
			t.Errorf("cost[%d] month: want %s, got %s", i, w.ym, got)
		}
		requireCostBasis(t, series.CostSeries[i].Cost, w.cost)
	}
}

// TestInvestmentTimeSeries_FlatTimeDeposit asserts the TD cost series is the
// flat principal at every snapshot month (no ledger replay — the TD ledger
// holds only the terminal Maturity row).
func TestInvestmentTimeSeries_FlatTimeDeposit(t *testing.T) {
	h := newHarness(t)
	td := h.createTimeDeposit(t, "Series TD")
	id := td.Investment.ID

	for _, ym := range []string{"2026-01", "2026-02"} {
		rec := h.do(t, "POST", "/investments/"+id.String()+"/snapshots", map[string]any{
			"year_month": ym, "amount": "100000000", "currency": "IDR",
			"accrued_interest": "375000",
		})
		requireStatus(t, rec, http.StatusCreated)
	}

	rec := h.do(t, "GET", "/investments/time-series", nil)
	requireStatus(t, rec, http.StatusOK)
	series := findSeries(t, decodeBody[[]repo.InvestmentTimeSeries](t, rec), id)

	if len(series.CostSeries) != 2 {
		t.Fatalf("cost series length: want 2, got %d", len(series.CostSeries))
	}
	for i := range series.CostSeries {
		requireCostBasis(t, series.CostSeries[i].Cost, "100000000")
	}
}
