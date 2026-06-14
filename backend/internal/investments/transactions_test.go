package investments_test

import (
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/kerti/balances-v2/backend/internal/db"
)

// covers: INV-COST-BASIS-04
func TestInvestmentTransactionHandlers_TradeShape(t *testing.T) {
	h := newHarness(t)
	stock := h.createStock(t, "Trade parent")

	t.Run("201 buy happy path", func(t *testing.T) {
		rec := h.do(t, "POST", "/investments/"+stock.Investment.ID.String()+"/transactions", map[string]any{
			"transaction_type": "buy",
			"transaction_date": "2026-05-01",
			"currency":         "IDR",
			"amount":           "5000000",
			"quantity":         "100",
			"price_per_unit":   "50000",
		})
		requireStatus(t, rec, http.StatusCreated)
	})

	t.Run("201 sell happy path", func(t *testing.T) {
		rec := h.do(t, "POST", "/investments/"+stock.Investment.ID.String()+"/transactions", map[string]any{
			"transaction_type": "sell",
			"transaction_date": "2026-05-15",
			"currency":         "IDR",
			"amount":           "5500000",
			"quantity":         "100",
			"price_per_unit":   "55000",
		})
		requireStatus(t, rec, http.StatusCreated)
	})

	t.Run("400 buy without quantity (wrong shape)", func(t *testing.T) {
		rec := h.do(t, "POST", "/investments/"+stock.Investment.ID.String()+"/transactions", map[string]any{
			"transaction_type": "buy",
			"transaction_date": "2026-06-01",
			"currency":         "IDR",
			"amount":           "5000000",
			"price_per_unit":   "50000",
		})
		requireStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("400 buy without price_per_unit (wrong shape)", func(t *testing.T) {
		rec := h.do(t, "POST", "/investments/"+stock.Investment.ID.String()+"/transactions", map[string]any{
			"transaction_type": "buy",
			"transaction_date": "2026-06-15",
			"currency":         "IDR",
			"amount":           "5000000",
			"quantity":         "100",
		})
		requireStatus(t, rec, http.StatusBadRequest)
	})
}

// covers: INV-COST-BASIS-04
func TestInvestmentTransactionHandlers_CashIncomeShape(t *testing.T) {
	h := newHarness(t)
	stock := h.createStock(t, "Dividend parent")
	bond := h.createBond(t, "Coupon parent")

	t.Run("201 dividend on stock", func(t *testing.T) {
		rec := h.do(t, "POST", "/investments/"+stock.Investment.ID.String()+"/transactions", map[string]any{
			"transaction_type": "dividend",
			"transaction_date": "2026-04-01",
			"currency":         "IDR",
			"amount":           "100000",
		})
		requireStatus(t, rec, http.StatusCreated)
	})

	t.Run("201 coupon on bond", func(t *testing.T) {
		rec := h.do(t, "POST", "/investments/"+bond.Investment.ID.String()+"/transactions", map[string]any{
			"transaction_type": "coupon",
			"transaction_date": "2026-04-01",
			"currency":         "IDR",
			"amount":           "50000",
		})
		requireStatus(t, rec, http.StatusCreated)
	})

	t.Run("400 dividend with quantity (wrong shape)", func(t *testing.T) {
		rec := h.do(t, "POST", "/investments/"+stock.Investment.ID.String()+"/transactions", map[string]any{
			"transaction_type": "dividend",
			"transaction_date": "2026-04-15",
			"currency":         "IDR",
			"amount":           "100000",
			"quantity":         "100",
		})
		requireStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("400 coupon on stock (wrong type for subtype)", func(t *testing.T) {
		rec := h.do(t, "POST", "/investments/"+stock.Investment.ID.String()+"/transactions", map[string]any{
			"transaction_type": "coupon",
			"transaction_date": "2026-04-20",
			"currency":         "IDR",
			"amount":           "50000",
		})
		requireStatus(t, rec, http.StatusBadRequest)
	})
}

// covers: INV-COST-BASIS-04
func TestInvestmentTransactionHandlers_MaturityShape(t *testing.T) {
	h := newHarness(t)
	td := h.createTimeDeposit(t, "Maturity TD parent")
	stock := h.createStock(t, "Maturity-on-stock parent")

	t.Run("201 maturity on TD with both rolled", func(t *testing.T) {
		rec := h.do(t, "POST", "/investments/"+td.Investment.ID.String()+"/transactions", map[string]any{
			"transaction_type":      "maturity",
			"transaction_date":      "2027-01-01",
			"currency":              "IDR",
			"principal_amount":      "100000000",
			"interest_amount":       "4500000",
			"principal_disposition": "rolled_to_new",
			"interest_disposition":  "rolled_to_new",
		})
		requireStatus(t, rec, http.StatusCreated)
	})

	t.Run("400 maturity on stock (wrong type for subtype)", func(t *testing.T) {
		rec := h.do(t, "POST", "/investments/"+stock.Investment.ID.String()+"/transactions", map[string]any{
			"transaction_type":      "maturity",
			"transaction_date":      "2027-01-01",
			"currency":              "IDR",
			"principal_amount":      "900",
			"interest_amount":       "100",
			"principal_disposition": "cash_out",
			"interest_disposition":  "cash_out",
		})
		requireStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("400 maturity missing dispositions (wrong shape)", func(t *testing.T) {
		bond := h.createBond(t, "Bond for maturity-shape test")
		rec := h.do(t, "POST", "/investments/"+bond.Investment.ID.String()+"/transactions", map[string]any{
			"transaction_type": "maturity",
			"transaction_date": "2030-01-01",
			"currency":         "IDR",
			"principal_amount": "10000000",
			"interest_amount":  "500000",
		})
		requireStatus(t, rec, http.StatusBadRequest)
	})
}

// covers: INV-COST-BASIS-04
func TestInvestmentTransactionHandlers_FeeShape(t *testing.T) {
	h := newHarness(t)
	stock := h.createStock(t, "Fee parent")

	t.Run("201 fee with amount only", func(t *testing.T) {
		rec := h.do(t, "POST", "/investments/"+stock.Investment.ID.String()+"/transactions", map[string]any{
			"transaction_type": "fee",
			"transaction_date": "2026-04-01",
			"currency":         "IDR",
			"amount":           "10000",
		})
		requireStatus(t, rec, http.StatusCreated)
	})

	t.Run("201 fee with paired quantity+price (e.g. fractional share deducted)", func(t *testing.T) {
		rec := h.do(t, "POST", "/investments/"+stock.Investment.ID.String()+"/transactions", map[string]any{
			"transaction_type": "fee",
			"transaction_date": "2026-04-15",
			"currency":         "IDR",
			"amount":           "5000",
			"quantity":         "0.1",
			"price_per_unit":   "50000",
		})
		requireStatus(t, rec, http.StatusCreated)
	})

	t.Run("400 fee with quantity but no price (wrong shape)", func(t *testing.T) {
		rec := h.do(t, "POST", "/investments/"+stock.Investment.ID.String()+"/transactions", map[string]any{
			"transaction_type": "fee",
			"transaction_date": "2026-04-20",
			"currency":         "IDR",
			"amount":           "10000",
			"quantity":         "0.1",
		})
		requireStatus(t, rec, http.StatusBadRequest)
	})
}

// covers: INV-COST-BASIS-04
func TestInvestmentTransactionHandlers_CommonErrors(t *testing.T) {
	h := newHarness(t)
	stock := h.createStock(t, "Common errors parent")

	t.Run("400 invalid transaction_type enum", func(t *testing.T) {
		rec := h.do(t, "POST", "/investments/"+stock.Investment.ID.String()+"/transactions", map[string]any{
			"transaction_type": "splat",
			"transaction_date": "2026-05-01",
			"currency":         "IDR",
			"amount":           "1",
		})
		requireStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("400 bad transaction_date format", func(t *testing.T) {
		rec := h.do(t, "POST", "/investments/"+stock.Investment.ID.String()+"/transactions", map[string]any{
			"transaction_type": "dividend",
			"transaction_date": "01-05-2026",
			"currency":         "IDR",
			"amount":           "1",
		})
		requireStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("400 invalid json", func(t *testing.T) {
		rec := h.do(t, "POST", "/investments/"+stock.Investment.ID.String()+"/transactions", "{not-json")
		requireStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("404 unknown parent investment", func(t *testing.T) {
		rec := h.do(t, "POST", "/investments/"+uuid.NewString()+"/transactions", map[string]any{
			"transaction_type": "dividend",
			"transaction_date": "2026-05-01",
			"currency":         "IDR",
			"amount":           "1",
		})
		requireStatus(t, rec, http.StatusNotFound)
	})

	t.Run("400 invalid parent id", func(t *testing.T) {
		rec := h.do(t, "POST", "/investments/not-a-uuid/transactions", map[string]any{
			"transaction_type": "dividend",
			"transaction_date": "2026-05-01",
			"currency":         "IDR",
			"amount":           "1",
		})
		requireStatus(t, rec, http.StatusBadRequest)
	})

	// fakeNow = 2030-01-01 UTC; anything after today rejects.
	t.Run("400 future transaction_date on create", func(t *testing.T) {
		rec := h.do(t, "POST", "/investments/"+stock.Investment.ID.String()+"/transactions", map[string]any{
			"transaction_type": "dividend",
			"transaction_date": "2030-01-02",
			"currency":         "IDR",
			"amount":           "1",
		})
		requireStatus(t, rec, http.StatusBadRequest)
	})
}

func TestInvestmentTransactionHandlers_ListUpdateDelete(t *testing.T) {
	h := newHarness(t)
	stock := h.createStock(t, "List/Update/Delete parent")

	createRec := h.do(t, "POST", "/investments/"+stock.Investment.ID.String()+"/transactions", map[string]any{
		"transaction_type": "buy",
		"transaction_date": "2026-05-01",
		"currency":         "IDR",
		"amount":           "5000000",
		"quantity":         "100",
		"price_per_unit":   "50000",
	})
	requireStatus(t, createRec, http.StatusCreated)
	txn := decodeBody[db.InvestmentTransaction](t, createRec)

	t.Run("List 200 happy", func(t *testing.T) {
		rec := h.do(t, "GET", "/investments/"+stock.Investment.ID.String()+"/transactions", nil)
		requireStatus(t, rec, http.StatusOK)
		list := decodeBody[[]db.InvestmentTransaction](t, rec)
		if len(list) != 1 {
			t.Fatalf("list length: want 1, got %d", len(list))
		}
	})

	t.Run("Update 200 happy", func(t *testing.T) {
		rec := h.do(t, "PATCH",
			"/investments/"+stock.Investment.ID.String()+"/transactions/"+txn.ID.String(),
			map[string]any{
				"transaction_date": "2026-05-02",
				"currency":         "IDR",
				"amount":           "5100000",
				"quantity":         "100",
				"price_per_unit":   "51000",
			})
		requireStatus(t, rec, http.StatusOK)
	})

	t.Run("Update 404 unknown txn", func(t *testing.T) {
		rec := h.do(t, "PATCH",
			"/investments/"+stock.Investment.ID.String()+"/transactions/"+uuid.NewString(),
			map[string]any{
				"transaction_date": "2026-05-01",
				"currency":         "IDR",
				"amount":           "1",
				"quantity":         "1",
				"price_per_unit":   "1",
			})
		requireStatus(t, rec, http.StatusNotFound)
	})

	t.Run("Update 400 bad transaction_date format", func(t *testing.T) {
		rec := h.do(t, "PATCH",
			"/investments/"+stock.Investment.ID.String()+"/transactions/"+txn.ID.String(),
			map[string]any{
				"transaction_date": "2026/05/01",
				"currency":         "IDR",
				"amount":           "1",
				"quantity":         "1",
				"price_per_unit":   "1",
			})
		requireStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("Update 400 future transaction_date", func(t *testing.T) {
		rec := h.do(t, "PATCH",
			"/investments/"+stock.Investment.ID.String()+"/transactions/"+txn.ID.String(),
			map[string]any{
				"transaction_date": "2030-01-02",
				"currency":         "IDR",
				"amount":           "1",
				"quantity":         "1",
				"price_per_unit":   "1",
			})
		requireStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("Delete 204 happy", func(t *testing.T) {
		rec := h.do(t, "DELETE",
			"/investments/"+stock.Investment.ID.String()+"/transactions/"+txn.ID.String(), nil)
		requireStatus(t, rec, http.StatusNoContent)
	})

	t.Run("Delete 404 unknown txn", func(t *testing.T) {
		rec := h.do(t, "DELETE",
			"/investments/"+stock.Investment.ID.String()+"/transactions/"+uuid.NewString(), nil)
		requireStatus(t, rec, http.StatusNotFound)
	})
}
