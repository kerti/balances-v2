package income_test

import (
	"net/http"
	"testing"

	"github.com/kerti/balances-v2/backend/internal/httperr"
)

// covers: INV-SOFT-DELETE-06
func TestIncomeHandlers_UndoDelete(t *testing.T) {
	h := newHarness(t)
	id := h.createIncome(t, "salary").ID.String()

	requireStatus(t, h.do(t, "DELETE", "/income/"+id, nil), http.StatusNoContent)
	requireStatus(t, h.do(t, "GET", "/income/"+id, nil), http.StatusNotFound)
	requireStatus(t, h.do(t, "POST", "/income/"+id+"/undo-delete", nil), http.StatusNoContent)
	requireStatus(t, h.do(t, "GET", "/income/"+id, nil), http.StatusOK)

	rec := h.do(t, "POST", "/income/"+id+"/undo-delete", nil)
	requireStatus(t, rec, http.StatusNotFound)
	if got := decodeBody[httperr.Envelope](t, rec).Code; got != httperr.CodeNotFound {
		t.Errorf("error code: want %s, got %s", httperr.CodeNotFound, got)
	}

	rec = h.do(t, "POST", "/income/not-a-uuid/undo-delete", nil)
	requireStatus(t, rec, http.StatusBadRequest)
	if got := decodeBody[httperr.Envelope](t, rec).Code; got != httperr.CodeInvalidID {
		t.Errorf("error code: want %s, got %s", httperr.CodeInvalidID, got)
	}
}
