package liabilities_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/kerti/balances-v2/backend/internal/httperr"
	"github.com/kerti/balances-v2/backend/internal/repo"
)

// covers: INV-SNAPSHOTS-15, INV-SOFT-DELETE-13
func TestLiabilitySnapshotHandlers_Move(t *testing.T) {
	h := newHarness(t)
	parent := h.createLiability(t, "Move parent", "institutional")
	base := "/liabilities/" + parent.ID.String() + "/snapshots/"
	move := func(id uuid.UUID, ym any) *httptest.ResponseRecorder {
		return h.do(t, "POST", base+id.String()+"/move", map[string]any{"year_month": ym})
	}

	t.Run("moves onto a taken month, names the displaced row, and undo reverses it", func(t *testing.T) {
		taken := h.createSnapshot(t, parent.ID, "2026-02")
		orig := h.createSnapshot(t, parent.ID, "2026-03")

		rec := move(orig.ID, "2026-02")
		requireStatus(t, rec, http.StatusOK)
		w := decodeBody[repo.LiabilitySnapshotWrite](t, rec)
		if w.ID == orig.ID || w.ReplacedID == nil || *w.ReplacedID != taken.ID {
			t.Fatalf("move response id=%s replaced_id=%v, want a new row displacing %s", w.ID, w.ReplacedID, taken.ID)
		}

		undo := map[string]any{"moved_id": w.ID, "displaced_id": w.ReplacedID}
		requireStatus(t, h.do(t, "POST", base+orig.ID.String()+"/undo-move", undo), http.StatusNoContent)
		// The moved row is archived now, so replaying the undo is refused.
		requireErrCode(t, h.do(t, "POST", base+orig.ID.String()+"/undo-move", undo), http.StatusConflict, httperr.CodeUndoReplacementChanged)
	})

	t.Run("the target month accepts YYYY-MM-DD and normalises to the first", func(t *testing.T) {
		orig := h.createSnapshot(t, parent.ID, "2026-06")
		rec := move(orig.ID, "2026-05-17")
		requireStatus(t, rec, http.StatusOK)
		if w := decodeBody[repo.LiabilitySnapshotWrite](t, rec); w.YearMonth.Day() != 1 || w.YearMonth.Month() != 5 {
			t.Errorf("year_month = %s, want 2026-05-01", w.YearMonth)
		}
	})

	t.Run("refusals", func(t *testing.T) {
		orig := h.createSnapshot(t, parent.ID, "2026-08")
		requireErrCode(t, move(orig.ID, "2026-08"), http.StatusBadRequest, httperr.CodeSnapshotMoveSameMonth)
		requireErrCode(t, move(orig.ID, "2099-01"), http.StatusBadRequest, httperr.CodeFutureYearMonth)
		requireErrCode(t, move(orig.ID, "nope"), http.StatusBadRequest, httperr.CodeInvalidYearMonth)
		requireErrCode(t, move(orig.ID, ""), http.StatusBadRequest, httperr.CodeValidation)
		requireErrCode(t, move(uuid.New(), "2026-07"), http.StatusNotFound, httperr.CodeNotFound)
		requireErrCode(t, h.do(t, "POST", base+"not-a-uuid/move", map[string]any{"year_month": "2026-07"}), http.StatusBadRequest, httperr.CodeInvalidID)
		requireErrCode(t, h.do(t, "POST", base+orig.ID.String()+"/move", nil), http.StatusBadRequest, httperr.CodeInvalidJSONBody)
	})

	t.Run("undo-move rejects a missing or malformed body", func(t *testing.T) {
		id := uuid.NewString()
		requireErrCode(t, h.do(t, "POST", base+id+"/undo-move", nil), http.StatusBadRequest, httperr.CodeInvalidJSONBody)
		requireErrCode(t, h.do(t, "POST", base+id+"/undo-move", map[string]any{}), http.StatusBadRequest, httperr.CodeValidation)
		requireErrCode(t, h.do(t, "POST", base+id+"/undo-move", map[string]any{"moved_id": uuid.NewString()}), http.StatusNotFound, httperr.CodeNotFound)
	})
}
