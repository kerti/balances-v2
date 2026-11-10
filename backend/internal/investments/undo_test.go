package investments_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/kerti/balances-v2/backend/internal/db"
	"github.com/kerti/balances-v2/backend/internal/httperr"
	"github.com/kerti/balances-v2/backend/internal/repo"
)

func requireErrCode(t *testing.T, rec *httptest.ResponseRecorder, status int, code httperr.Code) {
	t.Helper()
	requireStatus(t, rec, status)
	if got := decodeBody[httperr.Envelope](t, rec).Code; got != code {
		t.Fatalf("error code: want %s, got %s", code, got)
	}
}

// covers: INV-SOFT-DELETE-06, INV-SOFT-DELETE-12
func TestInvestmentPositionHandlers_UndoDelete(t *testing.T) {
	h := newHarness(t)
	cases := []struct {
		name, base string
		create     func() uuid.UUID
	}{
		{"stock", "/investments/stocks/", func() uuid.UUID { return h.createStock(t, "Undo me").Investment.ID }},
		{"mutual fund", "/investments/mutual-funds/", func() uuid.UUID { return h.createMutualFund(t, "Undo me").Investment.ID }},
		{"gold", "/investments/golds/", func() uuid.UUID { return h.createGold(t, "Undo me").Investment.ID }},
		{"bond", "/investments/bonds/", func() uuid.UUID { return h.createBond(t, "Undo me").Investment.ID }},
		{"time deposit", "/investments/time-deposits/", func() uuid.UUID { return h.createTimeDeposit(t, "Undo me").Investment.ID }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			id := tc.create().String()
			requireStatus(t, h.do(t, "DELETE", tc.base+id, nil), http.StatusNoContent)
			requireStatus(t, h.do(t, "GET", tc.base+id, nil), http.StatusNotFound)

			requireStatus(t, h.do(t, "POST", tc.base+id+"/undo-delete", nil), http.StatusNoContent)
			requireStatus(t, h.do(t, "GET", tc.base+id, nil), http.StatusOK)

			requireErrCode(t, h.do(t, "POST", tc.base+id+"/undo-delete", nil), http.StatusNotFound, httperr.CodeNotFound)
			requireErrCode(t, h.do(t, "POST", tc.base+"not-a-uuid/undo-delete", nil), http.StatusBadRequest, httperr.CodeInvalidID)
		})
	}

	t.Run("another subtype's route does not restore it", func(t *testing.T) {
		id := h.createStock(t, "Wrong route").Investment.ID.String()
		requireStatus(t, h.do(t, "DELETE", "/investments/stocks/"+id, nil), http.StatusNoContent)
		requireStatus(t, h.do(t, "POST", "/investments/golds/"+id+"/undo-delete", nil), http.StatusNotFound)
		requireStatus(t, h.do(t, "GET", "/investments/stocks/"+id, nil), http.StatusNotFound)
	})
}

// covers: INV-SOFT-DELETE-06, INV-SOFT-DELETE-07, INV-SOFT-DELETE-09, INV-SOFT-DELETE-10
func TestInvestmentSnapshotHandlers_Undo(t *testing.T) {
	h := newHarness(t)
	stock := h.createStock(t, "Snapshot parent")
	base := "/investments/" + stock.Investment.ID.String() + "/snapshots/"
	body := func(ym, qty string) map[string]any {
		return map[string]any{"year_month": ym, "amount": qty + "0", "currency": "IDR", "quantity": qty, "price_per_unit": "10"}
	}
	create := func(t *testing.T, ym string) db.InvestmentSnapshot {
		t.Helper()
		rec := h.do(t, "POST", base, body(ym, "100"))
		requireStatus(t, rec, http.StatusCreated)
		return decodeBody[db.InvestmentSnapshot](t, rec)
	}

	t.Run("undo-delete restores, then 409 once the month is re-filled", func(t *testing.T) {
		snap := create(t, "2026-03")
		requireStatus(t, h.do(t, "DELETE", base+snap.ID.String(), nil), http.StatusNoContent)
		requireStatus(t, h.do(t, "POST", base+snap.ID.String()+"/undo-delete", nil), http.StatusNoContent)
		requireStatus(t, h.do(t, "DELETE", base+snap.ID.String(), nil), http.StatusNoContent)
		create(t, "2026-03")
		requireErrCode(t, h.do(t, "POST", base+snap.ID.String()+"/undo-delete", nil), http.StatusConflict, httperr.CodeUndoSlotTaken)
	})

	edit := func(t *testing.T, id uuid.UUID, qty string) repo.InvestmentSnapshotWrite {
		t.Helper()
		rec := h.do(t, "PATCH", base+id.String(), map[string]any{"amount": qty + "0", "currency": "IDR", "quantity": qty, "price_per_unit": "10"})
		requireStatus(t, rec, http.StatusOK)
		w := decodeBody[repo.InvestmentSnapshotWrite](t, rec)
		if w.ReplacedID == nil {
			t.Fatal("edit did not report the displaced row")
		}
		return w
	}

	t.Run("undo-replace restores the original, refused once changed again", func(t *testing.T) {
		orig := create(t, "2026-04")
		first := edit(t, orig.ID, "7")
		requireStatus(t, h.do(t, "POST", base+first.ReplacedID.String()+"/undo-replace", map[string]any{"replacement_id": first.ID}), http.StatusNoContent)
		if amount, qty, _, _ := h.rawInvSnapshotShape(t, stock.Investment.ID, "2026-04"); amount.String() != "1000" || qty.String() != "100" {
			t.Errorf("live row after undo-replace = amount %s qty %s, want the original 1000 / 100", amount, qty)
		}

		orig2 := create(t, "2026-05")
		a := edit(t, orig2.ID, "7")
		edit(t, a.ID, "8")
		requireErrCode(t, h.do(t, "POST", base+a.ReplacedID.String()+"/undo-replace", map[string]any{"replacement_id": a.ID}),
			http.StatusConflict, httperr.CodeUndoReplacementChanged)
	})
}

// covers: INV-SOFT-DELETE-06
func TestInvestmentTransactionHandlers_UndoDelete(t *testing.T) {
	h := newHarness(t)
	stock := h.createStock(t, "Txn parent")
	base := "/investments/" + stock.Investment.ID.String() + "/transactions/"
	rec := h.do(t, "POST", base, map[string]any{
		"transaction_type": "buy", "transaction_date": "2026-05-01", "currency": "IDR",
		"amount": "5000000", "quantity": "100", "price_per_unit": "50000",
	})
	requireStatus(t, rec, http.StatusCreated)
	txn := decodeBody[db.InvestmentTransaction](t, rec)

	requireStatus(t, h.do(t, "DELETE", base+txn.ID.String(), nil), http.StatusNoContent)
	requireStatus(t, h.do(t, "POST", base+txn.ID.String()+"/undo-delete", nil), http.StatusNoContent)
	requireErrCode(t, h.do(t, "POST", base+txn.ID.String()+"/undo-delete", nil), http.StatusNotFound, httperr.CodeNotFound)
	requireErrCode(t, h.do(t, "POST", base+"not-a-uuid/undo-delete", nil), http.StatusBadRequest, httperr.CodeInvalidID)

	listRec := h.do(t, "GET", base, nil)
	requireStatus(t, listRec, http.StatusOK)
	list := decodeBody[[]db.InvestmentTransaction](t, listRec)
	if len(list) != 1 || list[0].ID != txn.ID {
		t.Errorf("transactions after undo = %d rows, want the restored buy", len(list))
	}
}
