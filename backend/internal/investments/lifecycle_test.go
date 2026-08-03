package investments_test

import (
	"net/http"
	"testing"
)

// TestInvestmentHandlers_Lifecycle covers the investment-specific lifecycle
// surface: the dedicated terminate endpoint (shared shape with the other three
// groups, fully exercised in the assets package) and — uniquely for
// investments — the Maturity hard guard, where a Maturity transaction flips the
// position to 'matured' and any further transaction is rejected with 409.
// covers: INV-LIFECYCLE-02, INV-LIFECYCLE-06
func TestInvestmentHandlers_Lifecycle(t *testing.T) {
	h := newHarness(t)

	t.Run("terminate stock happy path", func(t *testing.T) {
		stock := h.createStock(t, "Lifecycle stock")
		rec := h.do(t, "PATCH", "/investments/"+stock.Investment.ID.String()+"/lifecycle", map[string]any{
			"status":        "sold",
			"terminated_at": "2026-05-25",
		})
		requireStatus(t, rec, http.StatusOK)
		got := decodeBody[struct {
			Status string `json:"status"`
		}](t, rec)
		if got.Status != "sold" {
			t.Fatalf("status not flipped: %q", got.Status)
		}
	})

	t.Run("terminate with a settlement books the sale over the same request", func(t *testing.T) {
		stock := h.createStock(t, "Settled stock")
		id := stock.Investment.ID.String()
		rec := h.do(t, "PATCH", "/investments/"+id+"/lifecycle", map[string]any{
			"status":        "sold",
			"terminated_at": "2026-05-25",
			"settlement": map[string]any{
				"quantity":       "100",
				"price_per_unit": "11000",
			},
		})
		requireStatus(t, rec, http.StatusOK)

		// The sale is visible on the ledger without a second call — the whole
		// point of capture-at-source (ADR-0052 §6).
		rec = h.do(t, "GET", "/investments/"+id+"/transactions", nil)
		requireStatus(t, rec, http.StatusOK)
		txns := decodeBody[[]struct {
			TransactionType string `json:"transaction_type"`
			Amount          string `json:"amount"`
		}](t, rec)
		if len(txns) != 1 {
			t.Fatalf("got %d transactions, want exactly 1 (the settling sell)", len(txns))
		}
		if txns[0].TransactionType != "sell" {
			t.Errorf("transaction type: got %q, want sell", txns[0].TransactionType)
		}
		if txns[0].Amount != "1100000" {
			t.Errorf("sell amount: got %q, want 1100000 (quantity × price_per_unit)", txns[0].Amount)
		}
	})

	t.Run("a settlement whose shape does not match the subtype is 400", func(t *testing.T) {
		stock := h.createStock(t, "Mis-shaped settlement stock")
		rec := h.do(t, "PATCH", "/investments/"+stock.Investment.ID.String()+"/lifecycle", map[string]any{
			"status":        "sold",
			"terminated_at": "2026-05-25",
			// A Stock settles with a Sell, which has no maturity columns.
			"settlement": map[string]any{
				"principal_amount": "1000",
				"interest_amount":  "0",
			},
		})
		requireStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("unknown status is 400", func(t *testing.T) {
		stock := h.createStock(t, "Bad-status stock")
		rec := h.do(t, "PATCH", "/investments/"+stock.Investment.ID.String()+"/lifecycle", map[string]any{
			"status": "frozen", "terminated_at": "2026-05-25",
		})
		requireStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("maturity flips status and blocks further transactions with 409", func(t *testing.T) {
		td := h.createTimeDeposit(t, "Maturity-guard TD")
		txnPath := "/investments/" + td.Investment.ID.String() + "/transactions"
		maturityBody := map[string]any{
			"transaction_type":      "maturity",
			"transaction_date":      "2027-01-01",
			"currency":              "IDR",
			"principal_amount":      "100000000",
			"interest_amount":       "4500000",
			"principal_disposition": "cash_out",
			"interest_disposition":  "cash_out",
		}

		// First maturity succeeds and flips the position to 'matured'.
		requireStatus(t, h.do(t, "POST", txnPath, maturityBody), http.StatusCreated)

		// Any further transaction on the now-terminal position is refused. A
		// second maturity is well-formed (type + shape valid), so the 409 can
		// only come from the not-active guard.
		requireStatus(t, h.do(t, "POST", txnPath, maturityBody), http.StatusConflict)
	})
}
