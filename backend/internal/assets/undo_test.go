package assets_test

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
func TestAssetPositionHandlers_UndoDelete(t *testing.T) {
	h := newHarness(t)
	cases := []struct {
		name, base string
		create     func() uuid.UUID
	}{
		{"bank account", "/bank-accounts/", func() uuid.UUID { return h.createBankAccount(t, "Undo me").Asset.ID }},
		{"property", "/properties/", func() uuid.UUID { return h.createProperty(t, "Undo me").Asset.ID }},
		{"vehicle", "/vehicles/", func() uuid.UUID { return h.createVehicle(t, "Undo me").Asset.ID }},
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
		id := h.createBankAccount(t, "Wrong route").Asset.ID.String()
		requireStatus(t, h.do(t, "DELETE", "/bank-accounts/"+id, nil), http.StatusNoContent)
		requireStatus(t, h.do(t, "POST", "/vehicles/"+id+"/undo-delete", nil), http.StatusNotFound)
		requireStatus(t, h.do(t, "GET", "/bank-accounts/"+id, nil), http.StatusNotFound)
	})
}

// covers: INV-SOFT-DELETE-06, INV-SOFT-DELETE-07, INV-SOFT-DELETE-09, INV-SOFT-DELETE-10
func TestAssetSnapshotHandlers_Undo(t *testing.T) {
	h := newHarness(t)
	parent := h.createBankAccount(t, "Snapshot parent")
	base := "/assets/" + parent.Asset.ID.String() + "/snapshots/"

	t.Run("undo-delete restores, then 404, then 409 once the month is re-filled", func(t *testing.T) {
		snap := h.createAssetSnapshot(t, parent.Asset.ID, "2026-03")
		requireStatus(t, h.do(t, "DELETE", base+snap.ID.String(), nil), http.StatusNoContent)
		requireStatus(t, h.do(t, "POST", base+snap.ID.String()+"/undo-delete", nil), http.StatusNoContent)
		requireErrCode(t, h.do(t, "POST", base+snap.ID.String()+"/undo-delete", nil), http.StatusNotFound, httperr.CodeNotFound)

		requireStatus(t, h.do(t, "DELETE", base+snap.ID.String(), nil), http.StatusNoContent)
		h.createAssetSnapshot(t, parent.Asset.ID, "2026-03")
		requireErrCode(t, h.do(t, "POST", base+snap.ID.String()+"/undo-delete", nil), http.StatusConflict, httperr.CodeUndoSlotTaken)
	})

	edit := func(t *testing.T, id uuid.UUID, amount string) repo.AssetSnapshotWrite {
		t.Helper()
		rec := h.do(t, "PATCH", base+id.String(), map[string]any{"amount": amount, "currency": "IDR"})
		requireStatus(t, rec, http.StatusOK)
		w := decodeBody[repo.AssetSnapshotWrite](t, rec)
		if w.ReplacedID == nil {
			t.Fatal("edit did not report the displaced row")
		}
		return w
	}

	t.Run("undo-replace restores the original", func(t *testing.T) {
		orig := h.createAssetSnapshot(t, parent.Asset.ID, "2026-04")
		w := edit(t, orig.ID, "7")
		requireStatus(t, h.do(t, "POST", base+w.ReplacedID.String()+"/undo-replace", map[string]any{"replacement_id": w.ID}), http.StatusNoContent)
		if got := h.rawSnapshotAmount(t, parent.Asset.ID, "2026-04"); got.String() != "1000000" {
			t.Errorf("live amount after undo-replace = %s, want the original 1000000", got)
		}
	})

	t.Run("undo-replace refused once the replacement changed again", func(t *testing.T) {
		orig := h.createAssetSnapshot(t, parent.Asset.ID, "2026-05")
		first := edit(t, orig.ID, "7")
		edit(t, first.ID, "8")
		requireErrCode(t, h.do(t, "POST", base+first.ReplacedID.String()+"/undo-replace", map[string]any{"replacement_id": first.ID}),
			http.StatusConflict, httperr.CodeUndoReplacementChanged)
	})

	t.Run("undo-replace rejects a missing or malformed body", func(t *testing.T) {
		id := uuid.NewString()
		requireErrCode(t, h.do(t, "POST", base+id+"/undo-replace", nil), http.StatusBadRequest, httperr.CodeInvalidJSONBody)
		requireErrCode(t, h.do(t, "POST", base+id+"/undo-replace", map[string]any{}), http.StatusBadRequest, httperr.CodeValidation)
		requireErrCode(t, h.do(t, "POST", base+id+"/undo-replace", map[string]any{"replacement_id": "nope"}), http.StatusBadRequest, httperr.CodeInvalidJSONBody)
		requireErrCode(t, h.do(t, "POST", base+id+"/undo-replace", map[string]any{"replacement_id": uuid.NewString()}), http.StatusNotFound, httperr.CodeNotFound)
	})
}
