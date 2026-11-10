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

// covers: INV-SNAPSHOTS-15, INV-SOFT-DELETE-13
func TestInvestmentSnapshotHandlers_Move(t *testing.T) {
	h := newHarness(t)
	stock := h.createStock(t, "Move parent")
	base := "/investments/" + stock.Investment.ID.String() + "/snapshots/"
	create := func(t *testing.T, ym, qty string) db.InvestmentSnapshot {
		t.Helper()
		rec := h.do(t, "POST", base, map[string]any{"year_month": ym, "amount": qty + "0", "currency": "IDR", "quantity": qty, "price_per_unit": "10"})
		requireStatus(t, rec, http.StatusCreated)
		return decodeBody[db.InvestmentSnapshot](t, rec)
	}
	move := func(id uuid.UUID, ym any) *httptest.ResponseRecorder {
		return h.do(t, "POST", base+id.String()+"/move", map[string]any{"year_month": ym})
	}

	t.Run("moves onto a taken month carrying quantity and price, and undo reverses it", func(t *testing.T) {
		taken := create(t, "2026-02", "5")
		orig := create(t, "2026-03", "100")

		rec := move(orig.ID, "2026-02")
		requireStatus(t, rec, http.StatusOK)
		w := decodeBody[repo.InvestmentSnapshotWrite](t, rec)
		if w.ID == orig.ID || w.ReplacedID == nil || *w.ReplacedID != taken.ID {
			t.Fatalf("move response id=%s replaced_id=%v, want a new row displacing %s", w.ID, w.ReplacedID, taken.ID)
		}
		if amount, qty, price, _ := h.rawInvSnapshotShape(t, stock.Investment.ID, "2026-02"); amount.String() != "1000" || qty.String() != "100" || price.String() != "10" {
			t.Errorf("Feb row = amount %s qty %s price %s, want 1000 / 100 / 10", amount, qty, price)
		}

		undo := map[string]any{"moved_id": w.ID, "displaced_id": w.ReplacedID}
		requireStatus(t, h.do(t, "POST", base+orig.ID.String()+"/undo-move", undo), http.StatusNoContent)
		if amount, qty, _, _ := h.rawInvSnapshotShape(t, stock.Investment.ID, "2026-02"); amount.String() != "50" || qty.String() != "5" {
			t.Errorf("Feb row after undo = amount %s qty %s, want the displaced 50 / 5", amount, qty)
		}
		requireErrCode(t, h.do(t, "POST", base+orig.ID.String()+"/undo-move", undo), http.StatusConflict, httperr.CodeUndoReplacementChanged)
	})

	t.Run("refusals", func(t *testing.T) {
		orig := create(t, "2026-08", "1")
		requireErrCode(t, move(orig.ID, "2026-08"), http.StatusBadRequest, httperr.CodeSnapshotMoveSameMonth)
		requireErrCode(t, move(orig.ID, "2099-01"), http.StatusBadRequest, httperr.CodeFutureYearMonth)
		requireErrCode(t, move(orig.ID, "nope"), http.StatusBadRequest, httperr.CodeInvalidYearMonth)
		requireErrCode(t, move(orig.ID, ""), http.StatusBadRequest, httperr.CodeValidation)
		requireErrCode(t, move(uuid.New(), "2026-07"), http.StatusNotFound, httperr.CodeNotFound)
		requireErrCode(t, h.do(t, "POST", base+"not-a-uuid/move", map[string]any{"year_month": "2026-07"}), http.StatusBadRequest, httperr.CodeInvalidID)
	})

	t.Run("undo-move rejects a missing or malformed body", func(t *testing.T) {
		id := uuid.NewString()
		requireErrCode(t, h.do(t, "POST", base+id+"/undo-move", nil), http.StatusBadRequest, httperr.CodeInvalidJSONBody)
		requireErrCode(t, h.do(t, "POST", base+id+"/undo-move", map[string]any{}), http.StatusBadRequest, httperr.CodeValidation)
		requireErrCode(t, h.do(t, "POST", base+id+"/undo-move", map[string]any{"moved_id": uuid.NewString()}), http.StatusNotFound, httperr.CodeNotFound)
	})
}
