package assets_test

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"

	"github.com/kerti/balances-v2/backend/internal/auth"
	"github.com/kerti/balances-v2/backend/internal/snapshotimport"
)

// buildImportXLSX serialises rows (header included) onto the Snapshots sheet of
// an in-memory .xlsx — the shape handleImportSnapshots parses back.
func buildImportXLSX(t *testing.T, rows [][]string) []byte {
	t.Helper()
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()
	if _, err := f.NewSheet(snapshotimport.SheetName); err != nil {
		t.Fatalf("new sheet: %v", err)
	}
	for r, row := range rows {
		for c, v := range row {
			cell, _ := excelize.CoordinatesToCellName(c+1, r+1)
			if err := f.SetCellStr(snapshotimport.SheetName, cell, v); err != nil {
				t.Fatalf("set cell: %v", err)
			}
		}
	}
	buf, err := f.WriteToBuffer()
	if err != nil {
		t.Fatalf("serialise: %v", err)
	}
	return buf.Bytes()
}

// doUpload posts xlsx bytes under multipart field "file" to path as the harness
// user. A nil xlsx omits the file field, exercising the missing-file branch.
func (h *handlerHarness) doUpload(t *testing.T, path string, xlsx []byte) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	if xlsx != nil {
		fw, err := mw.CreateFormFile("file", "snapshots.xlsx")
		if err != nil {
			t.Fatalf("create form file: %v", err)
		}
		if _, err := fw.Write(xlsx); err != nil {
			t.Fatalf("write form file: %v", err)
		}
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("close multipart: %v", err)
	}
	req := httptest.NewRequest("POST", path, &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req = req.WithContext(auth.WithUser(req.Context(), h.user))
	rec := httptest.NewRecorder()
	h.router.ServeHTTP(rec, req)
	return rec
}

var assetImportHeader = []string{"year_month", "as_of_date", "amount", "currency", "description"}

// covers: INV-IMPORT-07
func TestAssetHandlers_ImportTemplate(t *testing.T) {
	h := newHarness(t)
	parent := h.createBankAccount(t, "Template target")
	base := "/assets/" + parent.Asset.ID.String() + "/snapshots"

	t.Run("200 streams xlsx", func(t *testing.T) {
		rec := h.do(t, "GET", base+"/import-template", nil)
		requireStatus(t, rec, http.StatusOK)
		if ct := rec.Header().Get("Content-Type"); ct != "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" {
			t.Errorf("content-type: got %q", ct)
		}
		if rec.Body.Len() == 0 {
			t.Error("template body is empty")
		}
	})

	t.Run("404 unknown asset", func(t *testing.T) {
		rec := h.do(t, "GET", "/assets/"+uuid.NewString()+"/snapshots/import-template", nil)
		requireStatus(t, rec, http.StatusNotFound)
	})

	t.Run("400 invalid id", func(t *testing.T) {
		rec := h.do(t, "GET", "/assets/not-a-uuid/snapshots/import-template", nil)
		requireStatus(t, rec, http.StatusBadRequest)
	})
}

// covers: INV-IMPORT-01, INV-IMPORT-02
func TestAssetHandlers_ImportSnapshots(t *testing.T) {
	h := newHarness(t)
	parent := h.createBankAccount(t, "Import target")
	base := "/assets/" + parent.Asset.ID.String() + "/snapshots"

	validRows := func() [][]string {
		return [][]string{
			assetImportHeader,
			{"2026-01", "2026-01-31", "10000000", "IDR", "Jan"},
			{"2026-02", "2026-02-28", "11000000", "", "Feb"}, // blank currency -> native
		}
	}

	t.Run("preview counts without writing", func(t *testing.T) {
		rec := h.doUpload(t, base+"/import", buildImportXLSX(t, validRows()))
		requireStatus(t, rec, http.StatusOK)
		body := decodeBody[importResp](t, rec)
		if body.Mode != "preview" || body.Committed {
			t.Fatalf("expected uncommitted preview, got %+v", body)
		}
		if body.ToInsert != 2 || body.ToUpdate != 0 {
			t.Errorf("counts: want insert=2 update=0, got insert=%d update=%d", body.ToInsert, body.ToUpdate)
		}
		list := h.do(t, "GET", base, nil)
		if got := decodeBody[[]any](t, list); len(got) != 0 {
			t.Errorf("preview wrote rows: %d snapshots present", len(got))
		}
	})

	t.Run("commit writes rows", func(t *testing.T) {
		rec := h.doUpload(t, base+"/import?mode=commit", buildImportXLSX(t, validRows()))
		requireStatus(t, rec, http.StatusOK)
		body := decodeBody[importResp](t, rec)
		if !body.Committed || body.ToInsert != 2 {
			t.Fatalf("expected committed insert=2, got %+v", body)
		}
		list := h.do(t, "GET", base, nil)
		if got := decodeBody[[]any](t, list); len(got) != 2 {
			t.Errorf("commit did not persist 2 rows, got %d", len(got))
		}

		again := h.doUpload(t, base+"/import?mode=commit", buildImportXLSX(t, validRows()))
		requireStatus(t, again, http.StatusOK)
		ab := decodeBody[importResp](t, again)
		if ab.ToUpdate != 2 || ab.ToInsert != 0 {
			t.Errorf("re-commit counts: want update=2 insert=0, got update=%d insert=%d", ab.ToUpdate, ab.ToInsert)
		}
	})

	t.Run("commit with bad row is 422 and writes nothing", func(t *testing.T) {
		other := h.createBankAccount(t, "Bad-row target")
		obase := "/assets/" + other.Asset.ID.String() + "/snapshots"
		rows := [][]string{
			assetImportHeader,
			{"not-a-month", "2026-01-31", "10000000", "IDR", "bad"},
		}
		rec := h.doUpload(t, obase+"/import?mode=commit", buildImportXLSX(t, rows))
		requireStatus(t, rec, http.StatusUnprocessableEntity)
		if body := decodeBody[importResp](t, rec); len(body.Errors) == 0 {
			t.Error("expected row errors in 422 body")
		}
		list := h.do(t, "GET", obase, nil)
		if got := decodeBody[[]any](t, list); len(got) != 0 {
			t.Errorf("422 commit wrote rows: %d", len(got))
		}
	})

	t.Run("400 invalid mode", func(t *testing.T) {
		rec := h.doUpload(t, base+"/import?mode=sideways", buildImportXLSX(t, validRows()))
		requireStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("400 missing file", func(t *testing.T) {
		rec := h.doUpload(t, base+"/import", nil)
		requireStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("404 unknown asset", func(t *testing.T) {
		path := "/assets/" + uuid.NewString() + "/snapshots/import"
		rec := h.doUpload(t, path, buildImportXLSX(t, validRows()))
		requireStatus(t, rec, http.StatusNotFound)
	})

	t.Run("400 invalid id", func(t *testing.T) {
		rec := h.doUpload(t, "/assets/not-a-uuid/snapshots/import", buildImportXLSX(t, validRows()))
		requireStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("400 not a spreadsheet", func(t *testing.T) {
		rec := h.doUpload(t, base+"/import", []byte("this is not xlsx"))
		requireStatus(t, rec, http.StatusBadRequest)
	})
}

// importResp mirrors the unexported importResponse the handler writes.
type importResp struct {
	Mode      string                    `json:"mode"`
	Committed bool                      `json:"committed"`
	ToInsert  int                       `json:"to_insert"`
	ToUpdate  int                       `json:"to_update"`
	Errors    []snapshotimport.RowError `json:"errors"`
}
