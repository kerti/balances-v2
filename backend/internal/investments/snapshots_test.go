package investments_test

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/kerti/balances-v2/backend/internal/db"
)

// quantity-price shape is for stock / mutual_fund / gold parents.
// accrued-interest shape is for bond / time_deposit parents.

func TestInvestmentSnapshotHandlers_QuantityPriceShape(t *testing.T) {
	h := newHarness(t)
	stock := h.createStock(t, "Snapshot parent (stock)")

	t.Run("201 happy path with quantity+price", func(t *testing.T) {
		rec := h.do(t, "POST", "/investments/"+stock.Investment.ID.String()+"/snapshots", map[string]any{
			"year_month":     "2026-05",
			"amount":         "5000000",
			"currency":       "IDR",
			"quantity":       "100",
			"price_per_unit": "50000",
		})
		requireStatus(t, rec, http.StatusCreated)
		body := decodeBody[db.InvestmentSnapshot](t, rec)
		if !decimal.NewFromInt(5000000).Equal(body.Amount) {
			t.Errorf("amount: want 5000000, got %s", body.Amount.String())
		}
	})

	t.Run("400 quantity-price shape with accrued_interest (wrong shape for stock)", func(t *testing.T) {
		rec := h.do(t, "POST", "/investments/"+stock.Investment.ID.String()+"/snapshots", map[string]any{
			"year_month":       "2026-06",
			"amount":           "5000000",
			"currency":         "IDR",
			"accrued_interest": "100000",
		})
		requireStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("400 quantity-price shape missing price_per_unit", func(t *testing.T) {
		rec := h.do(t, "POST", "/investments/"+stock.Investment.ID.String()+"/snapshots", map[string]any{
			"year_month": "2026-07",
			"amount":     "5000000",
			"currency":   "IDR",
			"quantity":   "100",
		})
		requireStatus(t, rec, http.StatusBadRequest)
	})
}

// covers: INV-BONDS-02
func TestInvestmentSnapshotHandlers_AccruedInterestShape(t *testing.T) {
	h := newHarness(t)
	bond := h.createBond(t, "Snapshot parent (bond)")

	t.Run("201 happy path with accrued_interest", func(t *testing.T) {
		rec := h.do(t, "POST", "/investments/"+bond.Investment.ID.String()+"/snapshots", map[string]any{
			"year_month":       "2026-05",
			"amount":           "10500000",
			"currency":         "IDR",
			"accrued_interest": "500000",
		})
		requireStatus(t, rec, http.StatusCreated)
		body := decodeBody[db.InvestmentSnapshot](t, rec)
		if body.AccruedInterest == nil || !decimal.NewFromInt(500000).Equal(*body.AccruedInterest) {
			t.Errorf("accrued_interest: got %v", body.AccruedInterest)
		}
	})

	t.Run("400 accrued shape with quantity+price (wrong shape for bond)", func(t *testing.T) {
		rec := h.do(t, "POST", "/investments/"+bond.Investment.ID.String()+"/snapshots", map[string]any{
			"year_month":     "2026-06",
			"amount":         "10500000",
			"currency":       "IDR",
			"quantity":       "1",
			"price_per_unit": "10500000",
		})
		requireStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("400 missing accrued_interest", func(t *testing.T) {
		rec := h.do(t, "POST", "/investments/"+bond.Investment.ID.String()+"/snapshots", map[string]any{
			"year_month": "2026-07",
			"amount":     "10500000",
			"currency":   "IDR",
		})
		requireStatus(t, rec, http.StatusBadRequest)
	})
}

// covers: INV-SNAPSHOTS-05
func TestInvestmentSnapshotHandlers_CommonErrors(t *testing.T) {
	h := newHarness(t)
	stock := h.createStock(t, "Common errors parent")

	t.Run("400 bad year_month", func(t *testing.T) {
		rec := h.do(t, "POST", "/investments/"+stock.Investment.ID.String()+"/snapshots", map[string]any{
			"year_month":     "May 2026",
			"amount":         "1",
			"currency":       "IDR",
			"quantity":       "1",
			"price_per_unit": "1",
		})
		requireStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("400 bad as_of_date format", func(t *testing.T) {
		rec := h.do(t, "POST", "/investments/"+stock.Investment.ID.String()+"/snapshots", map[string]any{
			"year_month":     "2026-08",
			"amount":         "1",
			"currency":       "IDR",
			"quantity":       "1",
			"price_per_unit": "1",
			"as_of_date":     "08/15/2026",
		})
		requireStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("400 missing required amount", func(t *testing.T) {
		rec := h.do(t, "POST", "/investments/"+stock.Investment.ID.String()+"/snapshots", map[string]any{
			"year_month":     "2026-09",
			"currency":       "IDR",
			"quantity":       "1",
			"price_per_unit": "1",
		})
		requireStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("404 unknown parent investment", func(t *testing.T) {
		rec := h.do(t, "POST", "/investments/"+uuid.NewString()+"/snapshots", map[string]any{
			"year_month":     "2026-10",
			"amount":         "1",
			"currency":       "IDR",
			"quantity":       "1",
			"price_per_unit": "1",
		})
		requireStatus(t, rec, http.StatusNotFound)
	})

	t.Run("400 invalid parent id", func(t *testing.T) {
		rec := h.do(t, "POST", "/investments/not-a-uuid/snapshots", map[string]any{
			"year_month":     "2026-11",
			"amount":         "1",
			"currency":       "IDR",
			"quantity":       "1",
			"price_per_unit": "1",
		})
		requireStatus(t, rec, http.StatusBadRequest)
	})

	// fakeNow = 2030-01-01 UTC; anything past current month / today rejects.
	t.Run("400 future year_month (quantity-price shape)", func(t *testing.T) {
		rec := h.do(t, "POST", "/investments/"+stock.Investment.ID.String()+"/snapshots", map[string]any{
			"year_month":     "2030-02",
			"amount":         "1",
			"currency":       "IDR",
			"quantity":       "1",
			"price_per_unit": "1",
		})
		requireStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("400 future as_of_date (quantity-price shape)", func(t *testing.T) {
		rec := h.do(t, "POST", "/investments/"+stock.Investment.ID.String()+"/snapshots", map[string]any{
			"year_month":     "2030-01",
			"amount":         "1",
			"currency":       "IDR",
			"quantity":       "1",
			"price_per_unit": "1",
			"as_of_date":     "2030-01-02",
		})
		requireStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("400 future year_month (accrued-interest shape)", func(t *testing.T) {
		bond := h.createBond(t, "Future-date bond parent")
		rec := h.do(t, "POST", "/investments/"+bond.Investment.ID.String()+"/snapshots", map[string]any{
			"year_month":       "2030-02",
			"amount":           "1",
			"currency":         "IDR",
			"accrued_interest": "1",
		})
		requireStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("400 future as_of_date (accrued-interest shape)", func(t *testing.T) {
		bond := h.createBond(t, "Future-date bond parent 2")
		rec := h.do(t, "POST", "/investments/"+bond.Investment.ID.String()+"/snapshots", map[string]any{
			"year_month":       "2030-01",
			"amount":           "1",
			"currency":         "IDR",
			"accrued_interest": "1",
			"as_of_date":       "2030-01-02",
		})
		requireStatus(t, rec, http.StatusBadRequest)
	})
}

// covers: INV-SNAPSHOTS-05
func TestInvestmentSnapshotHandlers_ListUpdateDelete(t *testing.T) {
	h := newHarness(t)
	stock := h.createStock(t, "List/Update/Delete parent")

	// seed a snapshot via the API
	createRec := h.do(t, "POST", "/investments/"+stock.Investment.ID.String()+"/snapshots", map[string]any{
		"year_month":     "2026-03",
		"amount":         "1000000",
		"currency":       "IDR",
		"quantity":       "10",
		"price_per_unit": "100000",
	})
	requireStatus(t, createRec, http.StatusCreated)
	snap := decodeBody[db.InvestmentSnapshot](t, createRec)

	t.Run("List 200 happy", func(t *testing.T) {
		rec := h.do(t, "GET", "/investments/"+stock.Investment.ID.String()+"/snapshots", nil)
		requireStatus(t, rec, http.StatusOK)
		list := decodeBody[[]db.InvestmentSnapshot](t, rec)
		if len(list) != 1 {
			t.Fatalf("list length: want 1, got %d", len(list))
		}
	})

	t.Run("Update 200 happy", func(t *testing.T) {
		rec := h.do(t, "PATCH",
			"/investments/"+stock.Investment.ID.String()+"/snapshots/"+snap.ID.String(),
			map[string]any{
				"amount":         "2000000",
				"currency":       "IDR",
				"quantity":       "10",
				"price_per_unit": "200000",
			})
		requireStatus(t, rec, http.StatusOK)
	})

	t.Run("Update 404 unknown snapshot", func(t *testing.T) {
		rec := h.do(t, "PATCH",
			"/investments/"+stock.Investment.ID.String()+"/snapshots/"+uuid.NewString(),
			map[string]any{
				"amount":         "1",
				"currency":       "IDR",
				"quantity":       "1",
				"price_per_unit": "1",
			})
		requireStatus(t, rec, http.StatusNotFound)
	})

	t.Run("Update 400 future as_of_date", func(t *testing.T) {
		rec := h.do(t, "PATCH",
			"/investments/"+stock.Investment.ID.String()+"/snapshots/"+snap.ID.String(),
			map[string]any{
				"amount":         "1",
				"currency":       "IDR",
				"quantity":       "1",
				"price_per_unit": "1",
				"as_of_date":     "2030-01-02",
			})
		requireStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("Delete 204 happy", func(t *testing.T) {
		rec := h.do(t, "DELETE",
			"/investments/"+stock.Investment.ID.String()+"/snapshots/"+snap.ID.String(), nil)
		requireStatus(t, rec, http.StatusNoContent)
	})

	t.Run("Delete 404 unknown snapshot", func(t *testing.T) {
		rec := h.do(t, "DELETE",
			"/investments/"+stock.Investment.ID.String()+"/snapshots/"+uuid.NewString(), nil)
		requireStatus(t, rec, http.StatusNotFound)
	})
}
