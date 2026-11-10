package liabilities_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

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
func TestLiabilityHandlers_UndoDelete(t *testing.T) {
	h := newHarness(t)
	id := h.createLiability(t, "Undo me", "personal").ID.String()

	requireStatus(t, h.do(t, "DELETE", "/liabilities/"+id, nil), http.StatusNoContent)
	requireStatus(t, h.do(t, "GET", "/liabilities/"+id, nil), http.StatusNotFound)
	requireStatus(t, h.do(t, "POST", "/liabilities/"+id+"/undo-delete", nil), http.StatusNoContent)
	requireStatus(t, h.do(t, "GET", "/liabilities/"+id, nil), http.StatusOK)

	requireErrCode(t, h.do(t, "POST", "/liabilities/"+id+"/undo-delete", nil), http.StatusNotFound, httperr.CodeNotFound)
	requireErrCode(t, h.do(t, "POST", "/liabilities/not-a-uuid/undo-delete", nil), http.StatusBadRequest, httperr.CodeInvalidID)
}

// covers: INV-SOFT-DELETE-06, INV-SOFT-DELETE-07, INV-SOFT-DELETE-09, INV-SOFT-DELETE-10
func TestLiabilitySnapshotHandlers_Undo(t *testing.T) {
	h := newHarness(t)
	parent := h.createLiability(t, "Snapshot parent", "institutional")
	base := "/liabilities/" + parent.ID.String() + "/snapshots/"

	t.Run("undo-delete restores, then 409 once the month is re-filled", func(t *testing.T) {
		snap := h.createSnapshot(t, parent.ID, "2026-03")
		requireStatus(t, h.do(t, "DELETE", base+snap.ID.String(), nil), http.StatusNoContent)
		requireStatus(t, h.do(t, "POST", base+snap.ID.String()+"/undo-delete", nil), http.StatusNoContent)
		requireStatus(t, h.do(t, "DELETE", base+snap.ID.String(), nil), http.StatusNoContent)
		h.createSnapshot(t, parent.ID, "2026-03")
		requireErrCode(t, h.do(t, "POST", base+snap.ID.String()+"/undo-delete", nil), http.StatusConflict, httperr.CodeUndoSlotTaken)
	})

	edit := func(t *testing.T, id uuid.UUID, amount string) repo.LiabilitySnapshotWrite {
		t.Helper()
		rec := h.do(t, "PATCH", base+id.String(), map[string]any{"amount": amount, "currency": "IDR"})
		requireStatus(t, rec, http.StatusOK)
		w := decodeBody[repo.LiabilitySnapshotWrite](t, rec)
		if w.ReplacedID == nil {
			t.Fatal("edit did not report the displaced row")
		}
		return w
	}

	t.Run("undo-replace restores the original, refused once changed again", func(t *testing.T) {
		orig := h.createSnapshot(t, parent.ID, "2026-04")
		first := edit(t, orig.ID, "7")
		requireStatus(t, h.do(t, "POST", base+first.ReplacedID.String()+"/undo-replace", map[string]any{"replacement_id": first.ID}), http.StatusNoContent)
		if got := h.rawSnapshotAmount(t, parent.ID, "2026-04"); got.String() != "10000000" {
			t.Errorf("live amount after undo-replace = %s, want the original 10000000", got)
		}

		orig2 := h.createSnapshot(t, parent.ID, "2026-05")
		a := edit(t, orig2.ID, "7")
		edit(t, a.ID, "8")
		requireErrCode(t, h.do(t, "POST", base+a.ReplacedID.String()+"/undo-replace", map[string]any{"replacement_id": a.ID}),
			http.StatusConflict, httperr.CodeUndoReplacementChanged)
	})
}
