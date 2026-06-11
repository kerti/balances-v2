package liabilities_test

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
	snapRows := append([][]string{liabilityImportHeader}, snaps...)
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

// jointLiabilityDetail is a valid joint-ownership liability Detail sheet. Field
// order mirrors liabilityDetailFields (the export side).
func jointLiabilityDetail() [][]string {
	return [][]string{
		{"display_name", "Imported loan"},
		{"description", "Brought in from a file"},
		{"subtype", "institutional"},
		{"ownership_type", "joint"},
		{"sole_owner", ""},
		{"native_currency", "IDR"},
		{"tag", ""},
		{"counterparty_name", "TestBank"},
		{"principal", "1400000000"},
		{"interest_rate", "7.5"},
		{"term_months", "180"},
		{"start_date", "2015-06-01"},
		{"maturity_date", "2030-06-01"},
	}
}

func twoSnapshots() [][]string {
	return [][]string{
		{"2026-01", "2026-01-31", "1350000000", "IDR", "Jan"},
		{"2026-02", "2026-02-28", "1345000000", "", "Feb"}, // blank currency -> native
	}
}

func countLiabilities(t *testing.T, h *handlerHarness) int {
	t.Helper()
	rec := h.do(t, "GET", "/liabilities", nil)
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

func TestLiabilityHandlers_ImportCreate(t *testing.T) {
	t.Run("preview validates without writing", func(t *testing.T) {
		h := newHarness(t)
		rec := h.doUpload(t, "/liabilities/import", buildCreateXLSX(t, jointLiabilityDetail(), twoSnapshots()))
		requireStatus(t, rec, http.StatusOK)
		body := decodeBody[createImportResp](t, rec)
		if !body.WouldCreate || body.Committed || body.ToInsert != 2 {
			t.Fatalf("want clean preview with insert=2, got %+v", body)
		}
		if countLiabilities(t, h) != 0 {
			t.Error("preview wrote a position")
		}
	})

	t.Run("commit creates the liability and seeds snapshots", func(t *testing.T) {
		h := newHarness(t)
		rec := h.doUpload(t, "/liabilities/import?mode=commit", buildCreateXLSX(t, jointLiabilityDetail(), twoSnapshots()))
		requireStatus(t, rec, http.StatusOK)
		body := decodeBody[createImportResp](t, rec)
		if !body.Committed || body.PositionID == nil || body.ToInsert != 2 {
			t.Fatalf("expected committed create with id + insert=2, got %+v", body)
		}
		if countLiabilities(t, h) != 1 {
			t.Error("commit did not persist the liability")
		}
		snaps := h.do(t, "GET", "/liabilities/"+*body.PositionID+"/snapshots", nil)
		requireStatus(t, snaps, http.StatusOK)
		if got := decodeBody[[]any](t, snaps); len(got) != 2 {
			t.Errorf("commit did not seed 2 snapshots, got %d", len(got))
		}
	})

	t.Run("sole_owner email resolves to the member", func(t *testing.T) {
		h := newHarness(t)
		detail := jointLiabilityDetail()
		detail[3] = []string{"ownership_type", "sole"}
		detail[4] = []string{"sole_owner", "alice@example.com"} // harness user is Alice@example.com
		rec := h.doUpload(t, "/liabilities/import?mode=commit", buildCreateXLSX(t, detail, twoSnapshots()))
		requireStatus(t, rec, http.StatusOK)
		body := decodeBody[createImportResp](t, rec)
		if !body.Committed || len(body.FieldErrors) != 0 {
			t.Fatalf("sole owner commit failed: %+v", body)
		}
	})

	t.Run("bad subtype enum is a field error", func(t *testing.T) {
		h := newHarness(t)
		detail := jointLiabilityDetail()
		detail[2] = []string{"subtype", "mortgage"} // not personal|institutional
		rec := h.doUpload(t, "/liabilities/import", buildCreateXLSX(t, detail, twoSnapshots()))
		requireStatus(t, rec, http.StatusOK)
		body := decodeBody[createImportResp](t, rec)
		if body.WouldCreate || !hasFieldError(body, "subtype") {
			t.Fatalf("want a subtype field error, got %+v", body.FieldErrors)
		}
	})

	t.Run("non-integer term_months is a field error", func(t *testing.T) {
		h := newHarness(t)
		detail := jointLiabilityDetail()
		detail[10] = []string{"term_months", "fifteen years"}
		rec := h.doUpload(t, "/liabilities/import", buildCreateXLSX(t, detail, twoSnapshots()))
		requireStatus(t, rec, http.StatusOK)
		body := decodeBody[createImportResp](t, rec)
		if body.WouldCreate || !hasFieldError(body, "term_months") {
			t.Fatalf("want a term_months field error, got %+v", body.FieldErrors)
		}
	})

	t.Run("bad snapshot row blocks commit, writes nothing", func(t *testing.T) {
		h := newHarness(t)
		badSnaps := [][]string{{"not-a-month", "2026-01-31", "1000", "IDR", "bad"}}
		rec := h.doUpload(t, "/liabilities/import?mode=commit", buildCreateXLSX(t, jointLiabilityDetail(), badSnaps))
		requireStatus(t, rec, http.StatusUnprocessableEntity)
		if countLiabilities(t, h) != 0 {
			t.Error("422 commit wrote a position")
		}
	})

	t.Run("matching tag name resolves and is assigned", func(t *testing.T) {
		h := newHarness(t)
		tagID := h.seedTag(t, "Debts")
		detail := jointLiabilityDetail()
		detail[6] = []string{"tag", "Debts"}
		rec := h.doUpload(t, "/liabilities/import?mode=commit", buildCreateXLSX(t, detail, twoSnapshots()))
		requireStatus(t, rec, http.StatusOK)
		body := decodeBody[createImportResp](t, rec)
		if !body.Committed || body.PositionID == nil {
			t.Fatalf("tagged commit failed: %+v", body)
		}
		got := h.do(t, "GET", "/liabilities/"+*body.PositionID, nil)
		requireStatus(t, got, http.StatusOK)
		gotTag := decodeBody[db.Liability](t, got).TagID
		if gotTag == nil || *gotTag != tagID {
			t.Fatalf("want tag_id %s, got %v", tagID, gotTag)
		}
	})

	t.Run("a real export round-trips into a new liability", func(t *testing.T) {
		h := newHarness(t)
		src := h.createLiability(t, "Round trip source", "personal")
		base := "/liabilities/" + src.ID.String() + "/snapshots"
		seed := h.doUpload(t, base+"/import?mode=commit",
			buildImportXLSX(t, [][]string{liabilityImportHeader, {"2026-03", "2026-03-31", "5000000", "IDR", "Mar"}}))
		requireStatus(t, seed, http.StatusOK)

		exp := h.do(t, "GET", "/liabilities/"+src.ID.String()+"/export", nil)
		requireStatus(t, exp, http.StatusOK)

		rec := h.doUpload(t, "/liabilities/import?mode=commit", exp.Body.Bytes())
		requireStatus(t, rec, http.StatusOK)
		body := decodeBody[createImportResp](t, rec)
		if !body.Committed || body.PositionID == nil {
			t.Fatalf("round-trip commit failed: %+v", body)
		}
		if *body.PositionID == src.ID.String() {
			t.Fatal("round-trip should create a NEW position, not touch the source")
		}
		if countLiabilities(t, h) != 2 || body.ToInsert != 1 {
			t.Errorf("want 2 liabilities + 1 seeded snapshot, got count=%d insert=%d",
				countLiabilities(t, h), body.ToInsert)
		}
	})
}
