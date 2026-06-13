package investments_test

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/kerti/balances-v2/backend/internal/repo"
)

// postTxn posts an investment transaction and asserts it was created.
func (h *handlerHarness) postTxn(t *testing.T, invID uuid.UUID, body map[string]any) {
	t.Helper()
	rec := h.do(t, "POST", "/investments/"+invID.String()+"/transactions", body)
	requireStatus(t, rec, http.StatusCreated)
}

// requireCostBasis fails unless the decimal equals want (RequireFromString).
func requireCostBasis(t *testing.T, got decimal.Decimal, want string) {
	t.Helper()
	if !got.Equal(decimal.RequireFromString(want)) {
		t.Errorf("cost_basis: want %s, got %s", want, got.String())
	}
}

func (h *handlerHarness) createStock(t *testing.T, displayName string) *repo.Stock {
	t.Helper()
	rec := h.do(t, "POST", "/investments/stocks", map[string]any{
		"display_name":    displayName,
		"ownership_type":  "joint",
		"native_currency": "IDR",
		"ticker":          "BBCA",
		"exchange":        "IDX",
		"risk_profile":    "medium",
	})
	requireStatus(t, rec, http.StatusCreated)
	return decodeBody[*repo.Stock](t, rec)
}

func TestStockHandlers_Create(t *testing.T) {
	h := newHarness(t)

	t.Run("201 happy path", func(t *testing.T) {
		rec := h.do(t, "POST", "/investments/stocks", map[string]any{
			"display_name":    "Bank Central Asia",
			"ownership_type":  "joint",
			"native_currency": "IDR",
			"ticker":          "BBCA",
			"exchange":        "IDX",
			"risk_profile":    "medium",
		})
		requireStatus(t, rec, http.StatusCreated)
		body := decodeBody[*repo.Stock](t, rec)
		if body.Details.Ticker != "BBCA" {
			t.Errorf("ticker: got %q", body.Details.Ticker)
		}
	})

	t.Run("400 invalid json", func(t *testing.T) {
		rec := h.do(t, "POST", "/investments/stocks", "{not-json")
		requireStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("400 missing required ticker", func(t *testing.T) {
		rec := h.do(t, "POST", "/investments/stocks", map[string]any{
			"display_name":    "X",
			"ownership_type":  "joint",
			"native_currency": "IDR",
			"exchange":        "IDX",
			"risk_profile":    "medium",
		})
		requireStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("400 missing required risk_profile", func(t *testing.T) {
		rec := h.do(t, "POST", "/investments/stocks", map[string]any{
			"display_name":    "X",
			"ownership_type":  "joint",
			"native_currency": "IDR",
			"ticker":          "X",
			"exchange":        "IDX",
		})
		requireStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("400 invalid risk_profile enum", func(t *testing.T) {
		rec := h.do(t, "POST", "/investments/stocks", map[string]any{
			"display_name":    "X",
			"ownership_type":  "joint",
			"native_currency": "IDR",
			"ticker":          "X",
			"exchange":        "IDX",
			"risk_profile":    "extreme",
		})
		requireStatus(t, rec, http.StatusBadRequest)
	})
}

func TestStockHandlers_List(t *testing.T) {
	h := newHarness(t)
	created := h.createStock(t, "Listed stock")

	rec := h.do(t, "GET", "/investments/stocks", nil)
	requireStatus(t, rec, http.StatusOK)
	list := decodeBody[[]repo.StockListItem](t, rec)
	if len(list) != 1 {
		t.Fatalf("list length: want 1, got %d", len(list))
	}
	if list[0].Investment.ID != created.Investment.ID {
		t.Errorf("list[0] id: want %s, got %s", created.Investment.ID, list[0].Investment.ID)
	}
}

// TestStockHandlers_List_CostBasis exercises the full avg-cost ledger replay
// folded into the list payload (issue #18): two buys, a partial sell at avg
// cost, and a capitalised fee. cost = 2,000,000 (buys) − 1,000,000 (sell of
// half the 200 held) + 50,000 (fee) = 1,050,000.
func TestStockHandlers_List_CostBasis(t *testing.T) {
	h := newHarness(t)
	stock := h.createStock(t, "Cost-basis stock")
	id := stock.Investment.ID

	h.postTxn(t, id, map[string]any{
		"transaction_type": "buy", "transaction_date": "2026-01-01", "currency": "IDR",
		"amount": "1000000", "quantity": "100", "price_per_unit": "10000",
	})
	h.postTxn(t, id, map[string]any{
		"transaction_type": "buy", "transaction_date": "2026-02-01", "currency": "IDR",
		"amount": "1000000", "quantity": "100", "price_per_unit": "10000",
	})
	h.postTxn(t, id, map[string]any{
		"transaction_type": "sell", "transaction_date": "2026-03-01", "currency": "IDR",
		"amount": "1200000", "quantity": "100", "price_per_unit": "12000",
	})
	h.postTxn(t, id, map[string]any{
		"transaction_type": "fee", "transaction_date": "2026-03-02", "currency": "IDR",
		"amount": "50000",
	})

	rec := h.do(t, "GET", "/investments/stocks", nil)
	requireStatus(t, rec, http.StatusOK)
	list := decodeBody[[]repo.StockListItem](t, rec)
	if len(list) != 1 {
		t.Fatalf("list length: want 1, got %d", len(list))
	}
	requireCostBasis(t, list[0].CostBasis, "1050000")

	// Same ledger feeds the row's activity summary (issue #67): four
	// transactions, latest dated 2026-03-02.
	if list[0].TransactionCount != 4 {
		t.Errorf("transaction_count: want 4, got %d", list[0].TransactionCount)
	}
	if list[0].LastTransactionDate == nil {
		t.Fatalf("last_transaction_date: want 2026-03-02, got nil")
	}
	if *list[0].LastTransactionDate != "2026-03-02" {
		t.Errorf("last_transaction_date: want 2026-03-02, got %s", *list[0].LastTransactionDate)
	}
}

// TestStockHandlers_List_NoTransactions confirms a position with an empty
// ledger reports a zero count and nil last-transaction date (issue #67).
func TestStockHandlers_List_NoTransactions(t *testing.T) {
	h := newHarness(t)
	h.createStock(t, "Quiet stock")

	rec := h.do(t, "GET", "/investments/stocks", nil)
	requireStatus(t, rec, http.StatusOK)
	list := decodeBody[[]repo.StockListItem](t, rec)
	if len(list) != 1 {
		t.Fatalf("list length: want 1, got %d", len(list))
	}
	if list[0].TransactionCount != 0 {
		t.Errorf("transaction_count: want 0, got %d", list[0].TransactionCount)
	}
	if list[0].LastTransactionDate != nil {
		t.Errorf("last_transaction_date: want nil, got %s", *list[0].LastTransactionDate)
	}
}

func TestStockHandlers_Get(t *testing.T) {
	h := newHarness(t)
	created := h.createStock(t, "Get target")

	t.Run("200 happy path", func(t *testing.T) {
		rec := h.do(t, "GET", "/investments/stocks/"+created.Investment.ID.String(), nil)
		requireStatus(t, rec, http.StatusOK)
		body := decodeBody[*repo.Stock](t, rec)
		if body.Investment.ID != created.Investment.ID {
			t.Errorf("id: want %s, got %s", created.Investment.ID, body.Investment.ID)
		}
	})

	t.Run("404 unknown id", func(t *testing.T) {
		rec := h.do(t, "GET", "/investments/stocks/"+uuid.NewString(), nil)
		requireStatus(t, rec, http.StatusNotFound)
	})

	t.Run("400 invalid id format", func(t *testing.T) {
		rec := h.do(t, "GET", "/investments/stocks/not-a-uuid", nil)
		requireStatus(t, rec, http.StatusBadRequest)
	})
}

func TestStockHandlers_Update(t *testing.T) {
	h := newHarness(t)
	created := h.createStock(t, "Update target")

	t.Run("200 happy path", func(t *testing.T) {
		rec := h.do(t, "PATCH", "/investments/stocks/"+created.Investment.ID.String(), map[string]any{
			"display_name":   "Renamed",
			"ownership_type": "joint",
			"ticker":         "BBRI",
			"exchange":       "IDX",
			"risk_profile":   "medium",
		})
		requireStatus(t, rec, http.StatusOK)
		body := decodeBody[*repo.Stock](t, rec)
		if body.Details.Ticker != "BBRI" {
			t.Errorf("ticker: want BBRI, got %q", body.Details.Ticker)
		}
	})

	t.Run("404 unknown id", func(t *testing.T) {
		rec := h.do(t, "PATCH", "/investments/stocks/"+uuid.NewString(), map[string]any{
			"display_name":   "x",
			"ownership_type": "joint",
			"ticker":         "x",
			"exchange":       "x",
			"risk_profile":   "medium",
		})
		requireStatus(t, rec, http.StatusNotFound)
	})

	t.Run("400 missing required ticker", func(t *testing.T) {
		rec := h.do(t, "PATCH", "/investments/stocks/"+created.Investment.ID.String(), map[string]any{
			"display_name": "x",
			"exchange":     "IDX",
			"risk_profile": "medium",
		})
		requireStatus(t, rec, http.StatusBadRequest)
	})
}

func TestStockHandlers_Delete(t *testing.T) {
	h := newHarness(t)

	t.Run("204 happy path", func(t *testing.T) {
		created := h.createStock(t, "To delete")
		rec := h.do(t, "DELETE", "/investments/stocks/"+created.Investment.ID.String(), nil)
		requireStatus(t, rec, http.StatusNoContent)

		rec = h.do(t, "GET", "/investments/stocks/"+created.Investment.ID.String(), nil)
		requireStatus(t, rec, http.StatusNotFound)
	})

	t.Run("404 unknown id", func(t *testing.T) {
		rec := h.do(t, "DELETE", "/investments/stocks/"+uuid.NewString(), nil)
		requireStatus(t, rec, http.StatusNotFound)
	})
}
