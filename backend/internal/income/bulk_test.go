package income_test

import (
	"net/http"
	"testing"

	"github.com/shopspring/decimal"

	"github.com/kerti/balances-v2/backend/internal/db"
)

func TestIncomeHandlers_BulkCreate(t *testing.T) {
	h := newHarness(t)
	row := func(cat, amount string) map[string]any {
		return map[string]any{
			"date":           "2026-06-30",
			"amount":         amount,
			"currency":       "IDR",
			"category":       cat,
			"ownership_type": "joint",
			"regularity":     "routine",
		}
	}
	count := func() int {
		rec := h.do(t, "GET", "/income", nil)
		requireStatus(t, rec, http.StatusOK)
		return len(decodeBody[[]db.Income](t, rec))
	}

	t.Run("201 writes every row in order", func(t *testing.T) {
		rec := h.do(t, "POST", "/income/bulk", map[string]any{
			"rows": []any{row("salary", "100"), row("pension", "250.5")},
		})
		requireStatus(t, rec, http.StatusCreated)
		got := decodeBody[[]db.Income](t, rec)
		if len(got) != 2 || got[0].Category != "salary" || got[1].Category != "pension" {
			t.Fatalf("unexpected body: %+v", got)
		}
		if !got[1].Amount.Equal(decimal.RequireFromString("250.5")) {
			t.Errorf("amount: got %s", got[1].Amount)
		}
	})

	t.Run("400 on an empty batch", func(t *testing.T) {
		rec := h.do(t, "POST", "/income/bulk", map[string]any{"rows": []any{}})
		requireStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("400 on malformed JSON", func(t *testing.T) {
		rec := h.do(t, "POST", "/income/bulk", "{nope")
		requireStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("400 on a bad row writes nothing", func(t *testing.T) {
		before := count()
		for name, bad := range map[string]map[string]any{
			"zero amount": row("gift", "0"),
			"bad date":    {"date": "nope", "amount": "5", "currency": "IDR", "category": "gift", "ownership_type": "joint", "regularity": "routine"},
			"bad enum":    row("not-a-category", "5"),
		} {
			rec := h.do(t, "POST", "/income/bulk", map[string]any{"rows": []any{row("salary", "9"), bad}})
			if rec.Code != http.StatusBadRequest {
				t.Errorf("%s: status %d, want 400", name, rec.Code)
			}
		}
		if after := count(); after != before {
			t.Errorf("rows after rejected batches: %d, want %d", after, before)
		}
	})

	t.Run("401 unauthenticated", func(t *testing.T) {
		rec := h.doRaw(t, "POST", "/income/bulk", map[string]any{"rows": []any{row("salary", "1")}}, nil)
		requireStatus(t, rec, http.StatusUnauthorized)
	})
}
