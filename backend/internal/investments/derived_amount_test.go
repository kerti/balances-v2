package investments_test

import (
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/kerti/balances-v2/backend/internal/db"
)

func requireStoredAmount(t *testing.T, h *handlerHarness, id uuid.UUID, month, want string) {
	t.Helper()
	amount, _, _, _ := h.rawInvSnapshotShape(t, id, month)
	if amount.String() != want {
		t.Errorf("%s stored amount: want %s, got %s", month, want, amount.String())
	}
}

// A stock, mutual fund or gold snapshot's amount is quantity × price_per_unit,
// rounded half up to 4 places; the amount a client sends has no effect (#968).
// Checked on create and on update, against a raw read of the stored row.
//
// covers: INV-SNAPSHOTS-20
func TestQuantityPriceSnapshot_AmountIsServerDerived(t *testing.T) {
	h := newHarness(t)
	positions := []struct {
		name string
		id   uuid.UUID
	}{
		{"stock", h.createStock(t, "Derived stock").Investment.ID},
		{"mutual fund", h.createMutualFund(t, "Derived fund").Investment.ID},
		{"gold", h.createGold(t, "Derived gold").Investment.ID},
	}
	for _, p := range positions {
		t.Run(p.name, func(t *testing.T) {
			base := "/investments/" + p.id.String() + "/snapshots"

			rec := h.do(t, "POST", base, map[string]any{
				"year_month": "2026-05", "amount": "1", "currency": "IDR",
				"quantity": "0.1", "price_per_unit": "3",
			})
			requireStatus(t, rec, http.StatusCreated)
			created := decodeBody[db.InvestmentSnapshot](t, rec)
			if created.Amount.String() != "0.3" {
				t.Errorf("create response amount: want 0.3, got %s", created.Amount.String())
			}
			requireStoredAmount(t, h, p.id, "2026-05", "0.3")

			rec = h.do(t, "PATCH", base+"/"+created.ID.String(), map[string]any{
				"amount": "999999", "currency": "IDR",
				"quantity": "1.12345678", "price_per_unit": "2.1234",
			})
			requireStatus(t, rec, http.StatusOK)
			updated := decodeBody[db.InvestmentSnapshot](t, rec)
			if updated.Amount.String() != "2.3855" {
				t.Errorf("update response amount: want 2.3855, got %s", updated.Amount.String())
			}
			requireStoredAmount(t, h, p.id, "2026-05", "2.3855")
		})
	}

	t.Run("a product that rounds up at the fifth decimal", func(t *testing.T) {
		rec := h.do(t, "POST", "/investments/"+positions[0].id.String()+"/snapshots", map[string]any{
			"year_month": "2026-06", "amount": "0", "currency": "IDR",
			"quantity": "0.00005", "price_per_unit": "1",
		})
		requireStatus(t, rec, http.StatusCreated)
		requireStoredAmount(t, h, positions[0].id, "2026-06", "0.0001")
	})

	t.Run("a product that rounds down at the fifth decimal", func(t *testing.T) {
		rec := h.do(t, "POST", "/investments/"+positions[0].id.String()+"/snapshots", map[string]any{
			"year_month": "2026-07", "amount": "5", "currency": "IDR",
			"quantity": "1.00004999", "price_per_unit": "1",
		})
		requireStatus(t, rec, http.StatusCreated)
		requireStoredAmount(t, h, positions[0].id, "2026-07", "1")
	})

	t.Run("a bond still stores the amount it was given", func(t *testing.T) {
		bond := h.createBond(t, "Derived bond")
		rec := h.do(t, "POST", "/investments/"+bond.Investment.ID.String()+"/snapshots", map[string]any{
			"year_month": "2026-05", "amount": "10500000", "currency": "IDR", "accrued_interest": "500000",
		})
		requireStatus(t, rec, http.StatusCreated)
		requireStoredAmount(t, h, bond.Investment.ID, "2026-05", "10500000")
	})
}
