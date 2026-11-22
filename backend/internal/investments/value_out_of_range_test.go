package investments_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/kerti/balances-v2/backend/internal/httperr"
)

// A number wider than its NUMERIC column is Postgres 22003. The wire answers it
// with a 422 VALUE_OUT_OF_RANGE envelope instead of a 500 INTERNAL, and nothing is
// stored. Counts are read raw (a soft-delete-aware getter could fake "nothing
// stored").
//
// covers: INV-CONTRACT-06
func TestInvestmentHandlers_ValueOutOfRange(t *testing.T) {
	h := newHarness(t)

	count := func(t *testing.T, query, id string) int {
		t.Helper()
		var n int
		if err := h.pool.QueryRow(context.Background(), query, id).Scan(&n); err != nil {
			t.Fatalf("count: %v", err)
		}
		return n
	}
	const txns = `SELECT COUNT(*) FROM investment_transactions WHERE investment_id = $1`

	t.Run("a Buy whose amount is wider than DECIMAL(20,4) is 422", func(t *testing.T) {
		stock := h.createStock(t, "Overflow buy")
		id := stock.Investment.ID.String()
		rec := h.do(t, "POST", "/investments/"+id+"/transactions", map[string]any{
			"transaction_type": "buy", "transaction_date": "2026-01-01", "currency": "IDR",
			"amount": "152415787531954592444.0808", "quantity": "123456789012", "price_per_unit": "1234567890.1234",
		})
		requireStatus(t, rec, http.StatusUnprocessableEntity)
		env := decodeBody[httperr.Envelope](t, rec)
		if env.Code != httperr.CodeValueOutOfRange {
			t.Errorf("code: want %s, got %s", httperr.CodeValueOutOfRange, env.Code)
		}
		if n := count(t, txns, id); n != 0 {
			t.Errorf("transactions stored: want 0, got %d", n)
		}
	})

	t.Run("a quantity wider than DECIMAL(20,8) is 422", func(t *testing.T) {
		stock := h.createStock(t, "Overflow quantity")
		id := stock.Investment.ID.String()
		rec := h.do(t, "POST", "/investments/"+id+"/transactions", map[string]any{
			"transaction_type": "buy", "transaction_date": "2026-01-01", "currency": "IDR",
			"amount": "1", "quantity": "1234567890123", "price_per_unit": "1",
		})
		requireStatus(t, rec, http.StatusUnprocessableEntity)
		if env := decodeBody[httperr.Envelope](t, rec); env.Code != httperr.CodeValueOutOfRange {
			t.Errorf("code: want %s, got %s", httperr.CodeValueOutOfRange, env.Code)
		}
	})

	t.Run("a sale whose derived proceeds overflow rolls the whole termination back", func(t *testing.T) {
		stock := h.createStock(t, "Overflow sale")
		id := stock.Investment.ID.String()
		rec := h.do(t, "PATCH", "/investments/"+id+"/lifecycle", map[string]any{
			"status":        "sold",
			"terminated_at": "2026-05-25",
			"settlement":    map[string]any{"quantity": "99999999999", "price_per_unit": "99999999999"},
		})
		requireStatus(t, rec, http.StatusUnprocessableEntity)
		if env := decodeBody[httperr.Envelope](t, rec); env.Code != httperr.CodeValueOutOfRange {
			t.Errorf("code: want %s, got %s", httperr.CodeValueOutOfRange, env.Code)
		}
		if n := count(t, txns, id); n != 0 {
			t.Errorf("transactions stored: want 0, got %d", n)
		}
		if n := count(t, `SELECT COUNT(*) FROM investments WHERE id = $1 AND status = 'active'`, id); n != 1 {
			t.Errorf("the position must still be active after the rollback, got %d active rows", n)
		}
	})
}
