package assets_test

import (
	"bytes"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/kerti/balances-v2/backend/internal/snapshotimport"
)

// TestBankAccountHandlers_Export covers the export endpoint end to end: a
// populated workbook streams back, its Detail sheet carries the position's
// fields (owner resolved to an email), and its Snapshots sheet re-parses
// through the unchanged importer — i.e. the file round-trips back in.
func TestBankAccountHandlers_Export(t *testing.T) {
	h := newHarness(t)

	t.Run("400 invalid id", func(t *testing.T) {
		rec := h.do(t, "GET", "/bank-accounts/not-a-uuid/export", nil)
		requireStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("404 unknown account", func(t *testing.T) {
		rec := h.do(t, "GET", "/bank-accounts/"+uuid.NewString()+"/export", nil)
		requireStatus(t, rec, http.StatusNotFound)
	})

	t.Run("name with no filename-safe chars falls back", func(t *testing.T) {
		create := h.do(t, "POST", "/bank-accounts", map[string]any{
			"display_name":    "!!!",
			"ownership_type":  "joint",
			"native_currency": "IDR",
			"bank_name":       "TestBank",
			"account_number":  "444",
			"account_type":    "savings",
		})
		requireStatus(t, create, http.StatusCreated)
		id := decodeBody[map[string]any](t, create)["asset"].(map[string]any)["id"].(string)

		rec := h.do(t, "GET", "/bank-accounts/"+id+"/export", nil)
		requireStatus(t, rec, http.StatusOK)
		if cd := rec.Header().Get("Content-Disposition"); cd != `attachment; filename="bank-account-export.xlsx"` {
			t.Errorf("fallback filename: got %q", cd)
		}
	})

	t.Run("round trip: export then re-import", func(t *testing.T) {
		// A sole account so the Detail sheet exercises owner-email resolution.
		create := h.do(t, "POST", "/bank-accounts", map[string]any{
			"display_name":       "Main checking",
			"description":        "Primary account",
			"ownership_type":     "sole",
			"sole_owner_user_id": h.user.ID.String(),
			"native_currency":    "IDR",
			"bank_name":          "TestBank",
			"account_number":     "1234567890",
			"account_type":       "savings",
		})
		requireStatus(t, create, http.StatusCreated)
		acct := decodeBody[map[string]any](t, create)
		id := acct["asset"].(map[string]any)["id"].(string)

		// Give it some history via the existing import-commit path.
		snapBase := "/assets/" + id + "/snapshots"
		rows := [][]string{
			assetImportHeader,
			{"2026-01", "2026-01-31", "10000000", "IDR", "Opening"},
			{"2026-02", "2026-02-28", "11500000", "", "Feb"},
		}
		commit := h.doUpload(t, snapBase+"/import?mode=commit", buildImportXLSX(t, rows))
		requireStatus(t, commit, http.StatusOK)

		// Export.
		rec := h.do(t, "GET", "/bank-accounts/"+id+"/export", nil)
		requireStatus(t, rec, http.StatusOK)
		if ct := rec.Header().Get("Content-Type"); ct != "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" {
			t.Errorf("content-type: got %q", ct)
		}
		if cd := rec.Header().Get("Content-Disposition"); cd == "" {
			t.Error("missing Content-Disposition")
		}
		xlsx := rec.Body.Bytes()

		// Detail sheet carries the resolved fields.
		detail, err := snapshotimport.ParseDetail(bytes.NewReader(xlsx))
		if err != nil {
			t.Fatalf("ParseDetail: %v", err)
		}
		if detail["display_name"] != "Main checking" {
			t.Errorf("detail display_name: got %q", detail["display_name"])
		}
		if detail["description"] != "Primary account" {
			t.Errorf("detail description: got %q", detail["description"])
		}
		if detail["ownership_type"] != "sole" {
			t.Errorf("detail ownership_type: got %q", detail["ownership_type"])
		}
		if detail["sole_owner"] != "Alice@example.com" {
			t.Errorf("detail sole_owner: got %q", detail["sole_owner"])
		}
		if detail["bank_name"] != "TestBank" || detail["account_type"] != "savings" {
			t.Errorf("detail bank fields: %v", detail)
		}

		// Snapshots sheet re-parses cleanly through the importer (Detail ignored).
		parsed, rowErrs, err := snapshotimport.Parse(bytes.NewReader(xlsx), snapshotimport.Options{DefaultCurrency: "IDR"})
		if err != nil {
			t.Fatalf("Parse exported Snapshots: %v", err)
		}
		if len(rowErrs) != 0 {
			t.Fatalf("exported file has row errors: %v", rowErrs)
		}
		if len(parsed) != 2 {
			t.Fatalf("want 2 snapshot rows, got %d", len(parsed))
		}

		// And the exported workbook commits back through the import endpoint as
		// pure updates (same two months), proving a real round trip.
		reimport := h.doUpload(t, snapBase+"/import?mode=commit", xlsx)
		requireStatus(t, reimport, http.StatusOK)
		body := decodeBody[importResp](t, reimport)
		if body.ToUpdate != 2 || body.ToInsert != 0 {
			t.Errorf("re-import counts: want update=2 insert=0, got update=%d insert=%d", body.ToUpdate, body.ToInsert)
		}
	})
}
