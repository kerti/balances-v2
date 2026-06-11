package assets_test

import (
	"net/http"
	"testing"

	"github.com/xuri/excelize/v2"

	"github.com/kerti/balances-v2/backend/internal/snapshotimport"
)

// buildCreateXLSX serialises a Detail sheet (key/value rows) plus a Snapshots
// sheet onto an in-memory .xlsx — the position-workbook shape the create-import
// handler parses back. Both `detail` and `snaps` exclude their header row; this
// helper prepends each. A nil snaps omits the data rows (header only).
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
	snapRows := append([][]string{{"year_month", "as_of_date", "amount", "currency", "description"}}, snaps...)
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

// createImportResp mirrors the unexported importCreateResponse the handler writes.
type createImportResp struct {
	Mode        string                      `json:"mode"`
	Committed   bool                        `json:"committed"`
	WouldCreate bool                        `json:"would_create"`
	PositionID  *string                     `json:"position_id"`
	ToInsert    int                         `json:"to_insert"`
	FieldErrors []snapshotimport.FieldError `json:"field_errors"`
	Errors      []snapshotimport.RowError   `json:"errors"`
}

// jointDetail is a valid joint-ownership Detail sheet (no sole_owner, no tag).
func jointDetail() [][]string {
	return [][]string{
		{"display_name", "Imported checking"},
		{"description", "Brought in from a file"},
		{"ownership_type", "joint"},
		{"sole_owner", ""},
		{"native_currency", "IDR"},
		{"tag", ""},
		{"bank_name", "TestBank"},
		{"account_number", "1234567890"},
		{"account_type", "savings"},
	}
}

func twoSnapshots() [][]string {
	return [][]string{
		{"2026-01", "2026-01-31", "10000000", "IDR", "Jan"},
		{"2026-02", "2026-02-28", "11000000", "", "Feb"}, // blank currency -> native
	}
}

func countBankAccounts(t *testing.T, h *handlerHarness) int {
	t.Helper()
	rec := h.do(t, "GET", "/bank-accounts", nil)
	requireStatus(t, rec, http.StatusOK)
	return len(decodeBody[[]any](t, rec))
}

func TestBankAccountHandlers_ImportCreate(t *testing.T) {
	t.Run("preview validates without writing", func(t *testing.T) {
		h := newHarness(t)
		before := countBankAccounts(t, h)

		rec := h.doUpload(t, "/bank-accounts/import", buildCreateXLSX(t, jointDetail(), twoSnapshots()))
		requireStatus(t, rec, http.StatusOK)
		body := decodeBody[createImportResp](t, rec)
		if body.Mode != "preview" || body.Committed {
			t.Fatalf("expected uncommitted preview, got %+v", body)
		}
		if !body.WouldCreate || body.ToInsert != 2 {
			t.Errorf("want would_create + to_insert=2, got %+v", body)
		}
		if len(body.FieldErrors) != 0 || len(body.Errors) != 0 {
			t.Errorf("clean preview should have no errors, got %+v", body)
		}
		if after := countBankAccounts(t, h); after != before {
			t.Errorf("preview wrote a position: before=%d after=%d", before, after)
		}
	})

	t.Run("commit creates the position and seeds snapshots", func(t *testing.T) {
		h := newHarness(t)

		rec := h.doUpload(t, "/bank-accounts/import?mode=commit", buildCreateXLSX(t, jointDetail(), twoSnapshots()))
		requireStatus(t, rec, http.StatusOK)
		body := decodeBody[createImportResp](t, rec)
		if !body.Committed || body.PositionID == nil || body.ToInsert != 2 {
			t.Fatalf("expected committed create with id + insert=2, got %+v", body)
		}
		if countBankAccounts(t, h) != 1 {
			t.Errorf("commit did not persist the bank account")
		}
		list := h.do(t, "GET", "/assets/"+*body.PositionID+"/snapshots", nil)
		requireStatus(t, list, http.StatusOK)
		if got := decodeBody[[]any](t, list); len(got) != 2 {
			t.Errorf("commit did not seed 2 snapshots, got %d", len(got))
		}
	})

	t.Run("sole_owner email resolves to the member, case-insensitive", func(t *testing.T) {
		h := newHarness(t)
		detail := jointDetail()
		detail[2] = []string{"ownership_type", "sole"}
		detail[3] = []string{"sole_owner", "alice@example.com"} // harness user is Alice@example.com

		rec := h.doUpload(t, "/bank-accounts/import?mode=commit", buildCreateXLSX(t, detail, twoSnapshots()))
		requireStatus(t, rec, http.StatusOK)
		body := decodeBody[createImportResp](t, rec)
		if !body.Committed || len(body.FieldErrors) != 0 {
			t.Fatalf("sole owner commit failed: %+v", body)
		}
	})

	t.Run("unknown sole_owner email is a field error", func(t *testing.T) {
		h := newHarness(t)
		detail := jointDetail()
		detail[2] = []string{"ownership_type", "sole"}
		detail[3] = []string{"sole_owner", "stranger@example.com"}

		rec := h.doUpload(t, "/bank-accounts/import", buildCreateXLSX(t, detail, twoSnapshots()))
		requireStatus(t, rec, http.StatusOK)
		body := decodeBody[createImportResp](t, rec)
		if body.WouldCreate {
			t.Fatal("an unknown owner should block creation")
		}
		if len(body.FieldErrors) != 1 || body.FieldErrors[0].Field != "sole_owner" {
			t.Fatalf("want one sole_owner field error, got %+v", body.FieldErrors)
		}

		commit := h.doUpload(t, "/bank-accounts/import?mode=commit", buildCreateXLSX(t, detail, twoSnapshots()))
		requireStatus(t, commit, http.StatusUnprocessableEntity)
		if countBankAccounts(t, h) != 0 {
			t.Error("422 commit wrote a position")
		}
	})

	t.Run("missing required field is a field error", func(t *testing.T) {
		h := newHarness(t)
		detail := jointDetail()
		detail[6] = []string{"bank_name", ""} // required

		rec := h.doUpload(t, "/bank-accounts/import", buildCreateXLSX(t, detail, twoSnapshots()))
		requireStatus(t, rec, http.StatusOK)
		body := decodeBody[createImportResp](t, rec)
		if body.WouldCreate {
			t.Fatal("a missing required field should block creation")
		}
		var found bool
		for _, fe := range body.FieldErrors {
			if fe.Field == "bank_name" {
				found = true
			}
		}
		if !found {
			t.Fatalf("want a bank_name field error, got %+v", body.FieldErrors)
		}
	})

	t.Run("bad enum is a field error", func(t *testing.T) {
		h := newHarness(t)
		detail := jointDetail()
		detail[8] = []string{"account_type", "checking"} // not in savings|current|other

		rec := h.doUpload(t, "/bank-accounts/import", buildCreateXLSX(t, detail, twoSnapshots()))
		requireStatus(t, rec, http.StatusOK)
		body := decodeBody[createImportResp](t, rec)
		var found bool
		for _, fe := range body.FieldErrors {
			if fe.Field == "account_type" {
				found = true
			}
		}
		if !found {
			t.Fatalf("want an account_type field error, got %+v", body.FieldErrors)
		}
	})

	t.Run("bad native_currency is a field error", func(t *testing.T) {
		h := newHarness(t)
		detail := jointDetail()
		detail[4] = []string{"native_currency", "RUPIAH"} // not a 3-letter ISO code

		rec := h.doUpload(t, "/bank-accounts/import", buildCreateXLSX(t, detail, twoSnapshots()))
		requireStatus(t, rec, http.StatusOK)
		body := decodeBody[createImportResp](t, rec)
		var found bool
		for _, fe := range body.FieldErrors {
			if fe.Field == "native_currency" {
				found = true
			}
		}
		if !found {
			t.Fatalf("want a native_currency field error, got %+v", body.FieldErrors)
		}
	})

	t.Run("bad snapshot row is a row error, commit writes nothing", func(t *testing.T) {
		h := newHarness(t)
		badSnaps := [][]string{{"not-a-month", "2026-01-31", "10000000", "IDR", "bad"}}

		rec := h.doUpload(t, "/bank-accounts/import?mode=commit", buildCreateXLSX(t, jointDetail(), badSnaps))
		requireStatus(t, rec, http.StatusUnprocessableEntity)
		body := decodeBody[createImportResp](t, rec)
		if len(body.Errors) == 0 {
			t.Error("expected a row error")
		}
		if countBankAccounts(t, h) != 0 {
			t.Error("422 commit wrote a position")
		}
	})

	t.Run("400 invalid mode", func(t *testing.T) {
		h := newHarness(t)
		rec := h.doUpload(t, "/bank-accounts/import?mode=sideways", buildCreateXLSX(t, jointDetail(), twoSnapshots()))
		requireStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("400 missing file", func(t *testing.T) {
		h := newHarness(t)
		rec := h.doUpload(t, "/bank-accounts/import", nil)
		requireStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("400 not a spreadsheet", func(t *testing.T) {
		h := newHarness(t)
		rec := h.doUpload(t, "/bank-accounts/import", []byte("this is not xlsx"))
		requireStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("400 workbook without a Detail sheet", func(t *testing.T) {
		h := newHarness(t)
		// Only a Snapshots sheet — nothing to create the position from.
		rec := h.doUpload(t, "/bank-accounts/import", buildImportXLSX(t, [][]string{assetImportHeader}))
		requireStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("a real export round-trips into a new position", func(t *testing.T) {
		h := newHarness(t)
		src := h.createBankAccount(t, "Round trip source")
		// Seed a snapshot so the export carries history.
		seed := h.doUpload(t, "/assets/"+src.Asset.ID.String()+"/snapshots/import?mode=commit",
			buildImportXLSX(t, [][]string{assetImportHeader, {"2026-03", "2026-03-31", "12000000", "IDR", "Mar"}}))
		requireStatus(t, seed, http.StatusOK)

		// Export the populated workbook, then re-import it from the list screen.
		exp := h.do(t, "GET", "/bank-accounts/"+src.Asset.ID.String()+"/export", nil)
		requireStatus(t, exp, http.StatusOK)
		workbook := exp.Body.Bytes()

		rec := h.doUpload(t, "/bank-accounts/import?mode=commit", workbook)
		requireStatus(t, rec, http.StatusOK)
		body := decodeBody[createImportResp](t, rec)
		if !body.Committed || body.PositionID == nil {
			t.Fatalf("round-trip commit failed: %+v", body)
		}
		if *body.PositionID == src.Asset.ID.String() {
			t.Fatal("round-trip should create a NEW position, not touch the source")
		}
		if countBankAccounts(t, h) != 2 {
			t.Errorf("want source + imported = 2 accounts, got %d", countBankAccounts(t, h))
		}
		if body.ToInsert != 1 {
			t.Errorf("want 1 seeded snapshot from the export, got %d", body.ToInsert)
		}
	})
}
