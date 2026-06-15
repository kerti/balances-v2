package receivables_test

import (
	"bytes"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/kerti/balances-v2/backend/internal/db"
	"github.com/kerti/balances-v2/backend/internal/snapshotimport"
)

// TestReceivableHandlers_Export covers the export endpoint end to end: a
// populated workbook streams back, its Detail sheet carries the receivable's
// fields (owner resolved to an email), and its Snapshots sheet re-imports
// through the unchanged importer — i.e. the file round-trips back in.
//
// covers: INV-EXPORT-01, INV-EXPORT-02, INV-EXPORT-04
func TestReceivableHandlers_Export(t *testing.T) {
	h := newHarness(t)

	t.Run("400 invalid id", func(t *testing.T) {
		rec := h.do(t, "GET", "/receivables/not-a-uuid/export", nil)
		requireStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("404 unknown receivable", func(t *testing.T) {
		rec := h.do(t, "GET", "/receivables/"+uuid.NewString()+"/export", nil)
		requireStatus(t, rec, http.StatusNotFound)
	})

	t.Run("name with no filename-safe chars falls back", func(t *testing.T) {
		create := h.do(t, "POST", "/receivables", map[string]any{
			"display_name":      "!!!",
			"ownership_type":    "joint",
			"native_currency":   "IDR",
			"counterparty_name": "Someone",
		})
		requireStatus(t, create, http.StatusCreated)
		id := decodeBody[db.Receivable](t, create).ID

		rec := h.do(t, "GET", "/receivables/"+id.String()+"/export", nil)
		requireStatus(t, rec, http.StatusOK)
		if cd := rec.Header().Get("Content-Disposition"); cd != `attachment; filename="receivable-export.xlsx"` {
			t.Errorf("fallback filename: got %q", cd)
		}
	})

	t.Run("round trip: export then re-import", func(t *testing.T) {
		create := h.do(t, "POST", "/receivables", map[string]any{
			"display_name":       "Loan to friend",
			"description":        "IOU",
			"ownership_type":     "sole",
			"sole_owner_user_id": h.user.ID.String(),
			"native_currency":    "IDR",
			"counterparty_name":  "A friend",
			"due_date":           "2026-12-31",
		})
		requireStatus(t, create, http.StatusCreated)
		id := decodeBody[db.Receivable](t, create).ID

		base := "/receivables/" + id.String() + "/snapshots"
		rows := [][]string{
			receivableImportHeader,
			{"2026-01", "2026-01-31", "50000000", "IDR", "Opening"},
			{"2026-02", "2026-02-28", "45000000", "", "Feb"},
		}
		commit := h.doUpload(t, base+"/import?mode=commit", buildImportXLSX(t, rows))
		requireStatus(t, commit, http.StatusOK)

		rec := h.do(t, "GET", "/receivables/"+id.String()+"/export", nil)
		requireStatus(t, rec, http.StatusOK)
		if ct := rec.Header().Get("Content-Type"); ct != "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" {
			t.Errorf("content-type: got %q", ct)
		}
		xlsx := rec.Body.Bytes()

		detail, err := snapshotimport.ParseDetail(bytes.NewReader(xlsx))
		if err != nil {
			t.Fatalf("ParseDetail: %v", err)
		}
		if detail["display_name"] != "Loan to friend" {
			t.Errorf("detail display_name: got %q", detail["display_name"])
		}
		if detail["sole_owner"] != "Alice@example.com" {
			t.Errorf("detail sole_owner: got %q", detail["sole_owner"])
		}
		if detail["counterparty_name"] != "A friend" || detail["due_date"] != "2026-12-31" {
			t.Errorf("detail receivable fields: %v", detail)
		}

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

		reimport := h.doUpload(t, base+"/import?mode=commit", xlsx)
		requireStatus(t, reimport, http.StatusOK)
		body := decodeBody[importResp](t, reimport)
		if body.ToUpdate != 2 || body.ToInsert != 0 {
			t.Errorf("re-import counts: want update=2 insert=0, got update=%d insert=%d", body.ToUpdate, body.ToInsert)
		}
	})
}
