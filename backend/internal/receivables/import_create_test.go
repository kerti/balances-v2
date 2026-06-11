package receivables_test

import (
	"net/http"
	"testing"

	"github.com/xuri/excelize/v2"

	"github.com/kerti/balances-v2/backend/internal/db"
	"github.com/kerti/balances-v2/backend/internal/snapshotimport"
)

// buildCreateXLSX serialises a Detail sheet (key/value rows) plus a Snapshots
// sheet onto an in-memory .xlsx — the position-workbook shape the create-import
// handler parses back. Both `detail` and `snaps` exclude their header row; this
// helper prepends each.
func buildCreateXLSX(t *testing.T, detail [][]string, snaps [][]string) []byte {
	t.Helper()
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()

	if _, err := f.NewSheet(snapshotimport.DetailSheetName); err != nil {
		t.Fatalf("new detail sheet: %v", err)
	}
	detailRows := append([][]string{{"field", "value", "notes"}}, detail...)
	for r, row := range detailRows {
		for c, v := range row {
			cell, _ := excelize.CoordinatesToCellName(c+1, r+1)
			if err := f.SetCellStr(snapshotimport.DetailSheetName, cell, v); err != nil {
				t.Fatalf("set detail cell: %v", err)
			}
		}
	}

	if _, err := f.NewSheet(snapshotimport.SheetName); err != nil {
		t.Fatalf("new snapshots sheet: %v", err)
	}
	snapRows := append([][]string{receivableImportHeader}, snaps...)
	for r, row := range snapRows {
		for c, v := range row {
			cell, _ := excelize.CoordinatesToCellName(c+1, r+1)
			if err := f.SetCellStr(snapshotimport.SheetName, cell, v); err != nil {
				t.Fatalf("set snapshot cell: %v", err)
			}
		}
	}

	if err := f.DeleteSheet("Sheet1"); err != nil {
		t.Fatalf("delete default sheet: %v", err)
	}
	buf, err := f.WriteToBuffer()
	if err != nil {
		t.Fatalf("serialise: %v", err)
	}
	return buf.Bytes()
}

// createImportResp mirrors the importcreate.Response the handler writes.
type createImportResp struct {
	Mode        string                      `json:"mode"`
	Committed   bool                        `json:"committed"`
	WouldCreate bool                        `json:"would_create"`
	PositionID  *string                     `json:"position_id"`
	ToInsert    int                         `json:"to_insert"`
	FieldErrors []snapshotimport.FieldError `json:"field_errors"`
	Errors      []snapshotimport.RowError   `json:"errors"`
}

// jointReceivableDetail is a valid joint-ownership receivable Detail sheet.
// Field order mirrors receivableDetailFields (the export side).
func jointReceivableDetail() [][]string {
	return [][]string{
		{"display_name", "Imported loan to friend"},
		{"description", "Brought in from a file"},
		{"ownership_type", "joint"},
		{"sole_owner", ""},
		{"native_currency", "IDR"},
		{"tag", ""},
		{"counterparty_name", "A friend"},
		{"due_date", "2027-01-31"},
	}
}

func twoSnapshots() [][]string {
	return [][]string{
		{"2026-01", "2026-01-31", "5000000", "IDR", "Jan"},
		{"2026-02", "2026-02-28", "4000000", "", "Feb"}, // blank currency -> native
	}
}

func countReceivables(t *testing.T, h *handlerHarness) int {
	t.Helper()
	rec := h.do(t, "GET", "/receivables", nil)
	requireStatus(t, rec, http.StatusOK)
	return len(decodeBody[[]any](t, rec))
}

func hasFieldError(body createImportResp, field string) bool {
	for _, fe := range body.FieldErrors {
		if fe.Field == field {
			return true
		}
	}
	return false
}

func TestReceivableHandlers_ImportCreate(t *testing.T) {
	t.Run("preview validates without writing", func(t *testing.T) {
		h := newHarness(t)
		rec := h.doUpload(t, "/receivables/import", buildCreateXLSX(t, jointReceivableDetail(), twoSnapshots()))
		requireStatus(t, rec, http.StatusOK)
		body := decodeBody[createImportResp](t, rec)
		if !body.WouldCreate || body.Committed || body.ToInsert != 2 {
			t.Fatalf("want clean preview with insert=2, got %+v", body)
		}
		if countReceivables(t, h) != 0 {
			t.Error("preview wrote a position")
		}
	})

	t.Run("commit creates the receivable and seeds snapshots", func(t *testing.T) {
		h := newHarness(t)
		rec := h.doUpload(t, "/receivables/import?mode=commit", buildCreateXLSX(t, jointReceivableDetail(), twoSnapshots()))
		requireStatus(t, rec, http.StatusOK)
		body := decodeBody[createImportResp](t, rec)
		if !body.Committed || body.PositionID == nil || body.ToInsert != 2 {
			t.Fatalf("expected committed create with id + insert=2, got %+v", body)
		}
		if countReceivables(t, h) != 1 {
			t.Error("commit did not persist the receivable")
		}
		snaps := h.do(t, "GET", "/receivables/"+*body.PositionID+"/snapshots", nil)
		requireStatus(t, snaps, http.StatusOK)
		if got := decodeBody[[]any](t, snaps); len(got) != 2 {
			t.Errorf("commit did not seed 2 snapshots, got %d", len(got))
		}
	})

	t.Run("missing counterparty_name is a field error", func(t *testing.T) {
		h := newHarness(t)
		detail := jointReceivableDetail()
		detail[6] = []string{"counterparty_name", ""} // required
		rec := h.doUpload(t, "/receivables/import", buildCreateXLSX(t, detail, twoSnapshots()))
		requireStatus(t, rec, http.StatusOK)
		body := decodeBody[createImportResp](t, rec)
		if body.WouldCreate || !hasFieldError(body, "counterparty_name") {
			t.Fatalf("want a counterparty_name field error, got %+v", body.FieldErrors)
		}
	})

	t.Run("bad due_date is a field error", func(t *testing.T) {
		h := newHarness(t)
		detail := jointReceivableDetail()
		detail[7] = []string{"due_date", "next year"} // not YYYY-MM-DD
		rec := h.doUpload(t, "/receivables/import", buildCreateXLSX(t, detail, twoSnapshots()))
		requireStatus(t, rec, http.StatusOK)
		body := decodeBody[createImportResp](t, rec)
		if body.WouldCreate || !hasFieldError(body, "due_date") {
			t.Fatalf("want a due_date field error, got %+v", body.FieldErrors)
		}
	})

	t.Run("unknown sole_owner email blocks creation", func(t *testing.T) {
		h := newHarness(t)
		detail := jointReceivableDetail()
		detail[2] = []string{"ownership_type", "sole"}
		detail[3] = []string{"sole_owner", "stranger@example.com"}
		rec := h.doUpload(t, "/receivables/import?mode=commit", buildCreateXLSX(t, detail, twoSnapshots()))
		requireStatus(t, rec, http.StatusUnprocessableEntity)
		body := decodeBody[createImportResp](t, rec)
		if !hasFieldError(body, "sole_owner") {
			t.Fatalf("want a sole_owner field error, got %+v", body.FieldErrors)
		}
		if countReceivables(t, h) != 0 {
			t.Error("422 commit wrote a position")
		}
	})

	t.Run("matching tag name resolves and is assigned", func(t *testing.T) {
		h := newHarness(t)
		tagID := h.seedTag(t, "Owed to us")
		detail := jointReceivableDetail()
		detail[5] = []string{"tag", "Owed to us"}
		rec := h.doUpload(t, "/receivables/import?mode=commit", buildCreateXLSX(t, detail, twoSnapshots()))
		requireStatus(t, rec, http.StatusOK)
		body := decodeBody[createImportResp](t, rec)
		if !body.Committed || body.PositionID == nil {
			t.Fatalf("tagged commit failed: %+v", body)
		}
		got := h.do(t, "GET", "/receivables/"+*body.PositionID, nil)
		requireStatus(t, got, http.StatusOK)
		gotTag := decodeBody[db.Receivable](t, got).TagID
		if gotTag == nil || *gotTag != tagID {
			t.Fatalf("want tag_id %s, got %v", tagID, gotTag)
		}
	})

	t.Run("a real export round-trips into a new receivable", func(t *testing.T) {
		h := newHarness(t)
		src := h.createReceivable(t, "Round trip source")
		base := "/receivables/" + src.ID.String() + "/snapshots"
		seed := h.doUpload(t, base+"/import?mode=commit",
			buildImportXLSX(t, [][]string{receivableImportHeader, {"2026-03", "2026-03-31", "3000000", "IDR", "Mar"}}))
		requireStatus(t, seed, http.StatusOK)

		exp := h.do(t, "GET", "/receivables/"+src.ID.String()+"/export", nil)
		requireStatus(t, exp, http.StatusOK)

		rec := h.doUpload(t, "/receivables/import?mode=commit", exp.Body.Bytes())
		requireStatus(t, rec, http.StatusOK)
		body := decodeBody[createImportResp](t, rec)
		if !body.Committed || body.PositionID == nil {
			t.Fatalf("round-trip commit failed: %+v", body)
		}
		if *body.PositionID == src.ID.String() {
			t.Fatal("round-trip should create a NEW position, not touch the source")
		}
		if countReceivables(t, h) != 2 || body.ToInsert != 1 {
			t.Errorf("want 2 receivables + 1 seeded snapshot, got count=%d insert=%d",
				countReceivables(t, h), body.ToInsert)
		}
	})
}
