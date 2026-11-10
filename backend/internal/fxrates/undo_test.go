package fxrates_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/kerti/balances-v2/backend/internal/httperr"
)

func errCode(t *testing.T, body []byte) httperr.Code {
	t.Helper()
	var env httperr.Envelope
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatalf("decode error envelope %q: %v", body, err)
	}
	return env.Code
}

// covers: INV-SOFT-DELETE-06, INV-SOFT-DELETE-07
func TestFxRatesHandlers_UndoDelete(t *testing.T) {
	h := newHarness(t)
	fr := h.createUSD(t)
	path := "/fx-rates/" + fr.ID.String() + "/undo-delete"

	requireStatus(t, h.do(t, "DELETE", "/fx-rates/"+fr.ID.String(), nil), http.StatusNoContent)
	requireStatus(t, h.do(t, "POST", path, nil), http.StatusNoContent)

	rec := h.do(t, "POST", path, nil)
	requireStatus(t, rec, http.StatusNotFound)
	if got := errCode(t, rec.Body.Bytes()); got != httperr.CodeNotFound {
		t.Errorf("error code: want %s, got %s", httperr.CodeNotFound, got)
	}

	requireStatus(t, h.do(t, "DELETE", "/fx-rates/"+fr.ID.String(), nil), http.StatusNoContent)
	h.createUSD(t)
	rec = h.do(t, "POST", path, nil)
	requireStatus(t, rec, http.StatusConflict)
	if got := errCode(t, rec.Body.Bytes()); got != httperr.CodeUndoSlotTaken {
		t.Errorf("error code: want %s, got %s", httperr.CodeUndoSlotTaken, got)
	}

	rec = h.do(t, "POST", "/fx-rates/not-a-uuid/undo-delete", nil)
	requireStatus(t, rec, http.StatusBadRequest)
	if got := errCode(t, rec.Body.Bytes()); got != httperr.CodeInvalidID {
		t.Errorf("error code: want %s, got %s", httperr.CodeInvalidID, got)
	}
}
