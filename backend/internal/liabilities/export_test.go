package liabilities_test

import (
	"bytes"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/kerti/balances-v2/backend/internal/db"
	"github.com/kerti/balances-v2/backend/internal/snapshotimport"
)

// TestLiabilityHandlers_Export covers the export endpoint end to end: a
// populated workbook streams back, its Detail sheet carries the liability's
// fields (owner resolved to an email), and its Snapshots sheet re-imports
// through the unchanged importer — i.e. the file round-trips back in.
//
// covers: INV-EXPORT-01, INV-EXPORT-02, INV-EXPORT-04
func TestLiabilityHandlers_Export(t *testing.T) {
	h := newHarness(t)

	t.Run("400 invalid id", func(t *testing.T) {
		rec := h.do(t, "GET", "/liabilities/not-a-uuid/export", nil)
		requireStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("404 unknown liability", func(t *testing.T) {
		rec := h.do(t, "GET", "/liabilities/"+uuid.NewString()+"/export", nil)
		requireStatus(t, rec, http.StatusNotFound)
	})

	t.Run("name with no filename-safe chars falls back", func(t *testing.T) {
		create := h.do(t, "POST", "/liabilities", map[string]any{
			"display_name":      "!!!",
			"subtype":           "personal",
			"ownership_type":    "joint",
			"native_currency":   "IDR",
			"counterparty_name": "Someone",
		})
		requireStatus(t, create, http.StatusCreated)
		id := decodeBody[db.Liability](t, create).ID

		rec := h.do(t, "GET", "/liabilities/"+id.String()+"/export", nil)
		requireStatus(t, rec, http.StatusOK)
		if cd := rec.Header().Get("Content-Disposition"); cd != `attachment; filename="liability-export.xlsx"` {
			t.Errorf("fallback filename: got %q", cd)
		}
	})

	t.Run("round trip: export then re-import", func(t *testing.T) {
		create := h.do(t, "POST", "/liabilities", map[string]any{
			"display_name":       "Home loan",
			"description":        "Mortgage",
			"subtype":            "institutional",
			"ownership_type":     "sole",
			"sole_owner_user_id": h.user.ID.String(),
			"native_currency":    "IDR",
			"counterparty_name":  "TestBank",
			"principal":          "1400000000",
			"interest_rate":      "7.5",
			"term_months":        180,
			"start_date":         "2015-06-01",
			"maturity_date":      "2030-06-01",
		})
		requireStatus(t, create, http.StatusCreated)
		id := decodeBody[db.Liability](t, create).ID

		base := "/liabilities/" + id.String() + "/snapshots"
		rows := [][]string{
			liabilityImportHeader,
			{"2026-01", "2026-01-31", "1350000000", "IDR", "Opening"},
			{"2026-02", "2026-02-28", "1345000000", "", "Feb"},
		}
		commit := h.doUpload(t, base+"/import?mode=commit", buildImportXLSX(t, rows))
		requireStatus(t, commit, http.StatusOK)

		rec := h.do(t, "GET", "/liabilities/"+id.String()+"/export", nil)
		requireStatus(t, rec, http.StatusOK)
		if ct := rec.Header().Get("Content-Type"); ct != "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" {
			t.Errorf("content-type: got %q", ct)
		}
		xlsx := rec.Body.Bytes()

		detail, err := snapshotimport.ParseDetail(bytes.NewReader(xlsx))
		if err != nil {
			t.Fatalf("ParseDetail: %v", err)
		}
		if detail["display_name"] != "Home loan" {
			t.Errorf("detail display_name: got %q", detail["display_name"])
		}
		if detail["subtype"] != "institutional" {
			t.Errorf("detail subtype: got %q", detail["subtype"])
		}
		if detail["sole_owner"] != "Alice@example.com" {
			t.Errorf("detail sole_owner: got %q", detail["sole_owner"])
		}
		if detail["counterparty_name"] != "TestBank" || detail["principal"] != "1400000000" {
			t.Errorf("detail liability fields: %v", detail)
		}
		if detail["term_months"] != "180" || detail["maturity_date"] != "2030-06-01" {
			t.Errorf("detail term/maturity: %v", detail)
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
