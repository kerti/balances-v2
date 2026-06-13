package importcreate_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"

	"github.com/kerti/balances-v2/backend/internal/httperr"
	"github.com/kerti/balances-v2/backend/internal/importcreate"
	"github.com/kerti/balances-v2/backend/internal/repo"
	"github.com/kerti/balances-v2/backend/internal/snapshotimport"
)

// These exercise the shared create-import flow in isolation — no database. Run
// takes the per-group resolve + commit as closures, so the 5xx fault paths
// (resolve / commit failures) that the handler suites can't reach without a
// fault-injecting pool are driven here by stub closures that just return an
// error. The pure helpers are unit-tested directly.

// buildWorkbook serialises a Detail sheet (key/value rows) + a Snapshots sheet
// onto an in-memory .xlsx — the position-workbook shape Run parses. A nil
// detail omits the Detail sheet (the "nothing to create from" branch).
func buildWorkbook(t *testing.T, detail [][]string, snaps [][]string) []byte {
	t.Helper()
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()

	if detail != nil {
		if _, err := f.NewSheet(snapshotimport.DetailSheetName); err != nil {
			t.Fatalf("new detail sheet: %v", err)
		}
		rows := append([][]string{{"field", "value", "notes"}}, detail...)
		writeSheet(t, f, snapshotimport.DetailSheetName, rows)
	}

	if _, err := f.NewSheet(snapshotimport.SheetName); err != nil {
		t.Fatalf("new snapshots sheet: %v", err)
	}
	header := []string{"year_month", "as_of_date", "amount", "currency", "description"}
	writeSheet(t, f, snapshotimport.SheetName, append([][]string{header}, snaps...))

	if err := f.DeleteSheet("Sheet1"); err != nil {
		t.Fatalf("delete default sheet: %v", err)
	}
	buf, err := f.WriteToBuffer()
	if err != nil {
		t.Fatalf("serialise: %v", err)
	}
	return buf.Bytes()
}

func writeSheet(t *testing.T, f *excelize.File, sheet string, rows [][]string) {
	t.Helper()
	for r, row := range rows {
		for c, v := range row {
			cell, _ := excelize.CoordinatesToCellName(c+1, r+1)
			if err := f.SetCellStr(sheet, cell, v); err != nil {
				t.Fatalf("set cell: %v", err)
			}
		}
	}
}

func detailRows() [][]string {
	return [][]string{
		{"display_name", "Imported"},
		{"native_currency", "IDR"},
	}
}

func twoSnapshotRows() [][]string {
	return [][]string{
		{"2026-01", "2026-01-31", "100", "IDR", "Jan"},
		{"2026-02", "2026-02-28", "200", "", "Feb"},
	}
}

// stub records what Run handed the resolve + commit closures and lets a test
// inject the field errors / errors each should return.
type stub struct {
	fieldErrs    []snapshotimport.FieldError
	resolveErr   error
	commitErr    error
	commitCalled bool
	gotRows      int
}

func (s *stub) resolve(_ context.Context, _ map[string]string) (string, *uuid.UUID, []snapshotimport.FieldError, error) {
	return "params", nil, s.fieldErrs, s.resolveErr
}

func (s *stub) commit(_ context.Context, _ string, _ *uuid.UUID, rows []repo.ImportSnapshotRow) (uuid.UUID, error) {
	s.commitCalled = true
	s.gotRows = len(rows)
	if s.commitErr != nil {
		return uuid.Nil, s.commitErr
	}
	return uuid.New(), nil
}

// runUpload posts a workbook (or raw body, when xlsx is non-nil but not a real
// file the caller still controls bytes) to Run via a throwaway request.
func runUpload(t *testing.T, query string, body []byte, withFile bool, s *stub) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	if withFile {
		fw, err := mw.CreateFormFile("file", "wb.xlsx")
		if err != nil {
			t.Fatalf("form file: %v", err)
		}
		if _, err := fw.Write(body); err != nil {
			t.Fatalf("write file: %v", err)
		}
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("close mw: %v", err)
	}
	req := httptest.NewRequest("POST", "/import"+query, &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	importcreate.Run(rec, req, httperr.NewValidator(), s.resolve, s.commit)
	return rec
}

func decodeResp(t *testing.T, rec *httptest.ResponseRecorder) importcreate.Response {
	t.Helper()
	var resp importcreate.Response
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode (status %d, body %q): %v", rec.Code, rec.Body.String(), err)
	}
	return resp
}

func TestRun(t *testing.T) {
	clean := buildWorkbook(t, detailRows(), twoSnapshotRows())

	t.Run("invalid mode is 400", func(t *testing.T) {
		rec := runUpload(t, "?mode=sideways", clean, true, &stub{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("want 400, got %d", rec.Code)
		}
	})

	t.Run("missing file is 400", func(t *testing.T) {
		rec := runUpload(t, "", nil, false, &stub{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("want 400, got %d", rec.Code)
		}
	})

	t.Run("not a spreadsheet is 400", func(t *testing.T) {
		rec := runUpload(t, "", []byte("not xlsx"), true, &stub{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("want 400, got %d", rec.Code)
		}
	})

	t.Run("workbook without a Detail sheet is 400", func(t *testing.T) {
		rec := runUpload(t, "", buildWorkbook(t, nil, twoSnapshotRows()), true, &stub{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("want 400, got %d", rec.Code)
		}
	})

	t.Run("clean preview writes nothing", func(t *testing.T) {
		s := &stub{}
		rec := runUpload(t, "", clean, true, s)
		if rec.Code != http.StatusOK {
			t.Fatalf("want 200, got %d", rec.Code)
		}
		resp := decodeResp(t, rec)
		if !resp.WouldCreate || resp.Committed || resp.ToInsert != 2 {
			t.Fatalf("want clean preview insert=2, got %+v", resp)
		}
		if s.commitCalled {
			t.Error("preview must not call commit")
		}
		if len(resp.FieldErrors) != 0 || len(resp.Errors) != 0 {
			t.Errorf("clean preview should carry empty (non-null) error slices, got %+v", resp)
		}
	})

	t.Run("clean commit creates and seeds", func(t *testing.T) {
		s := &stub{}
		rec := runUpload(t, "?mode=commit", clean, true, s)
		if rec.Code != http.StatusOK {
			t.Fatalf("want 200, got %d", rec.Code)
		}
		resp := decodeResp(t, rec)
		if !resp.Committed || resp.PositionID == nil {
			t.Fatalf("want committed with id, got %+v", resp)
		}
		if !s.commitCalled || s.gotRows != 2 {
			t.Errorf("commit got rows=%d called=%v", s.gotRows, s.commitCalled)
		}
	})

	t.Run("resolve field error blocks preview and fails commit 422", func(t *testing.T) {
		fe := []snapshotimport.FieldError{{Field: "display_name", Message: "is required"}}

		prev := runUpload(t, "", clean, true, &stub{fieldErrs: fe})
		if prev.Code != http.StatusOK || decodeResp(t, prev).WouldCreate {
			t.Fatalf("field error must block would_create, got %d %+v", prev.Code, decodeResp(t, prev))
		}

		s := &stub{fieldErrs: fe}
		commit := runUpload(t, "?mode=commit", clean, true, s)
		if commit.Code != http.StatusUnprocessableEntity {
			t.Fatalf("want 422, got %d", commit.Code)
		}
		if s.commitCalled {
			t.Error("422 must not call commit")
		}
	})

	t.Run("bad snapshot row blocks commit 422", func(t *testing.T) {
		bad := buildWorkbook(t, detailRows(), [][]string{{"not-a-month", "2026-01-31", "100", "IDR", "x"}})
		s := &stub{}
		rec := runUpload(t, "?mode=commit", bad, true, s)
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("want 422, got %d", rec.Code)
		}
		if s.commitCalled {
			t.Error("row error must not call commit")
		}
	})

	t.Run("resolve error is 500", func(t *testing.T) {
		rec := runUpload(t, "", clean, true, &stub{resolveErr: errors.New("boom")})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("want 500, got %d", rec.Code)
		}
	})

	t.Run("commit error is 500", func(t *testing.T) {
		rec := runUpload(t, "?mode=commit", clean, true, &stub{commitErr: errors.New("boom")})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("want 500, got %d", rec.Code)
		}
	})
}

func TestResolveSoleOwner(t *testing.T) {
	ctx := context.Background()
	id := uuid.New()
	found := func(context.Context, string) (uuid.UUID, bool, error) { return id, true, nil }
	missing := func(context.Context, string) (uuid.UUID, bool, error) { return uuid.Nil, false, nil }
	boom := func(context.Context, string) (uuid.UUID, bool, error) { return uuid.Nil, false, errors.New("db") }

	t.Run("joint ignores any email", func(t *testing.T) {
		owner, handled, fe, err := importcreate.ResolveSoleOwner(ctx, "joint", "x@example.com", found)
		if owner != nil || handled || fe != nil || err != nil {
			t.Fatalf("joint should no-op, got owner=%v handled=%v fe=%v err=%v", owner, handled, fe, err)
		}
	})

	t.Run("sole with blank email defers to validator", func(t *testing.T) {
		owner, handled, fe, err := importcreate.ResolveSoleOwner(ctx, "sole", "  ", found)
		if owner != nil || handled || fe != nil || err != nil {
			t.Fatalf("blank email should no-op, got owner=%v handled=%v fe=%v err=%v", owner, handled, fe, err)
		}
	})

	t.Run("sole with known email resolves", func(t *testing.T) {
		owner, handled, fe, err := importcreate.ResolveSoleOwner(ctx, "sole", "a@example.com", found)
		if owner == nil || *owner != id || !handled || len(fe) != 0 || err != nil {
			t.Fatalf("want resolved owner, got owner=%v handled=%v fe=%v err=%v", owner, handled, fe, err)
		}
	})

	t.Run("sole with unknown email is a field error", func(t *testing.T) {
		owner, handled, fe, err := importcreate.ResolveSoleOwner(ctx, "sole", "ghost@example.com", missing)
		if owner != nil || !handled || err != nil {
			t.Fatalf("unknown owner: owner=%v handled=%v err=%v", owner, handled, err)
		}
		if len(fe) != 1 || fe[0].Field != "sole_owner" {
			t.Fatalf("want one sole_owner field error, got %+v", fe)
		}
	})

	t.Run("lookup error propagates", func(t *testing.T) {
		_, handled, fe, err := importcreate.ResolveSoleOwner(ctx, "sole", "a@example.com", boom)
		if err == nil || !handled || fe != nil {
			t.Fatalf("want a propagated error, got handled=%v fe=%v err=%v", handled, fe, err)
		}
	})
}

// sample mirrors a create-request enough to fail one rule per field, so
// CollectFieldErrors + RuleMessage are exercised across every branch.
type sample struct {
	DisplayName     string     `json:"display_name"       validate:"required"`
	NativeCurrency  string     `json:"native_currency"    validate:"iso4217"`
	Kind            string     `json:"kind"               validate:"oneof=a b"`
	Email           string     `json:"email"              validate:"omitempty,email"`
	SoleOwnerUserID *uuid.UUID `json:"sole_owner_user_id" validate:"required"`
}

func TestCollectFieldErrorsAndRuleMessage(t *testing.T) {
	v := httperr.NewValidator()
	err := v.Struct(&sample{NativeCurrency: "RUPIAH", Kind: "z", Email: "nope"})
	fes := importcreate.CollectFieldErrors(err)

	got := make(map[string]string, len(fes))
	for _, fe := range fes {
		got[fe.Field] = fe.Message
	}
	want := map[string]string{
		"display_name":    "is required",
		"native_currency": "must be a 3-letter ISO currency code",
		"kind":            "must be one of: a, b",
		"email":           "is invalid",
		"sole_owner":      "is required", // sole_owner_user_id remapped to the Detail key
	}
	for field, msg := range want {
		if got[field] != msg {
			t.Errorf("field %q: want %q, got %q", field, msg, got[field])
		}
	}

	t.Run("non-validation error yields nil", func(t *testing.T) {
		if importcreate.CollectFieldErrors(errors.New("plain")) != nil {
			t.Error("a non-validator error should map to nil")
		}
	})
}

func TestDetailCellParsers(t *testing.T) {
	detail := map[string]string{
		"good_dec":  "12.5",
		"bad_dec":   "lots",
		"good_date": "2026-01-31",
		"bad_date":  "soon",
		"good_int":  "2019",
		"bad_int":   "many",
		"blank":     "  ",
	}

	t.Run("Decimal", func(t *testing.T) {
		var fe []snapshotimport.FieldError
		if d := importcreate.Decimal(detail, "good_dec", &fe); d == nil || d.String() != "12.5" {
			t.Errorf("good decimal: got %v", d)
		}
		if importcreate.Decimal(detail, "blank", &fe) != nil {
			t.Error("blank decimal should be nil")
		}
		if importcreate.Decimal(detail, "bad_dec", &fe) != nil || len(fe) != 1 || fe[0].Field != "bad_dec" {
			t.Errorf("bad decimal should append one field error, got %+v", fe)
		}
	})

	t.Run("Date", func(t *testing.T) {
		var fe []snapshotimport.FieldError
		if d := importcreate.Date(detail, "good_date", &fe); d == nil || d.Format("2006-01-02") != "2026-01-31" {
			t.Errorf("good date: got %v", d)
		}
		if importcreate.Date(detail, "blank", &fe) != nil {
			t.Error("blank date should be nil")
		}
		if importcreate.Date(detail, "bad_date", &fe) != nil || len(fe) != 1 || fe[0].Field != "bad_date" {
			t.Errorf("bad date should append one field error, got %+v", fe)
		}
	})

	t.Run("Int32", func(t *testing.T) {
		var fe []snapshotimport.FieldError
		if n := importcreate.Int32(detail, "good_int", &fe); n == nil || *n != 2019 {
			t.Errorf("good int: got %v", n)
		}
		if importcreate.Int32(detail, "blank", &fe) != nil {
			t.Error("blank int should be nil")
		}
		if importcreate.Int32(detail, "bad_int", &fe) != nil || len(fe) != 1 || fe[0].Field != "bad_int" {
			t.Errorf("bad int should append one field error, got %+v", fe)
		}
	})

	t.Run("OptionalStr", func(t *testing.T) {
		if importcreate.OptionalStr("  ") != nil {
			t.Error("blank should be nil")
		}
		if s := importcreate.OptionalStr("  hi  "); s == nil || *s != "hi" {
			t.Errorf("non-blank should trim, got %v", s)
		}
	})
}

// ---- RunWithLedger (issue #90) -------------------------------------------

// These drive the investment create-import flow in isolation — no database. The
// resolve + commit closures are stubs, so RunWithLedger / validateLedger /
// readUpload are exercised within this package (the handler suites cross-package
// exercise doesn't count toward per-package coverage). subtype + shape select the
// snapshot layout + ledger matrix.

var qtyPriceSnapHeader = []string{"year_month", "as_of_date", "quantity", "price_per_unit", "currency", "description"}
var accruedSnapHeader = []string{"year_month", "as_of_date", "amount", "accrued_interest", "currency", "description"}
var ledgerTxnHeader = []string{
	"transaction_type", "transaction_date", "currency", "amount",
	"quantity", "price_per_unit",
	"principal_amount", "interest_amount", "principal_disposition", "interest_disposition",
	"description",
}

// buildLedgerWorkbook serialises Detail + Snapshots (snapHeader) + Transactions
// onto an in-memory .xlsx — the investment position-workbook shape RunWithLedger
// parses.
func buildLedgerWorkbook(t *testing.T, detail [][]string, snapHeader []string, snaps, txns [][]string) []byte {
	t.Helper()
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()

	if _, err := f.NewSheet(snapshotimport.DetailSheetName); err != nil {
		t.Fatalf("new detail sheet: %v", err)
	}
	writeSheet(t, f, snapshotimport.DetailSheetName, append([][]string{{"field", "value", "notes"}}, detail...))

	if _, err := f.NewSheet(snapshotimport.SheetName); err != nil {
		t.Fatalf("new snapshots sheet: %v", err)
	}
	writeSheet(t, f, snapshotimport.SheetName, append([][]string{snapHeader}, snaps...))

	if _, err := f.NewSheet(snapshotimport.TransactionsSheetName); err != nil {
		t.Fatalf("new transactions sheet: %v", err)
	}
	writeSheet(t, f, snapshotimport.TransactionsSheetName, append([][]string{ledgerTxnHeader}, txns...))

	if err := f.DeleteSheet("Sheet1"); err != nil {
		t.Fatalf("delete default sheet: %v", err)
	}
	buf, err := f.WriteToBuffer()
	if err != nil {
		t.Fatalf("serialise: %v", err)
	}
	return buf.Bytes()
}

type ledgerStub struct {
	fieldErrs    []snapshotimport.FieldError
	resolveErr   error
	commitErr    error
	commitCalled bool
	gotSnaps     int
	gotLedger    int
}

func (s *ledgerStub) resolve(_ context.Context, _ map[string]string) (string, *uuid.UUID, []snapshotimport.FieldError, error) {
	return "params", nil, s.fieldErrs, s.resolveErr
}

func (s *ledgerStub) commit(_ context.Context, _ string, _ *uuid.UUID, snaps []repo.ImportInvestmentSnapshotRow, ledger []repo.ImportTransactionRow) (uuid.UUID, error) {
	s.commitCalled = true
	s.gotSnaps = len(snaps)
	s.gotLedger = len(ledger)
	if s.commitErr != nil {
		return uuid.Nil, s.commitErr
	}
	return uuid.New(), nil
}

func runLedgerUpload(t *testing.T, query string, body []byte, withFile bool, subtype string, shape snapshotimport.Shape, s *ledgerStub) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	if withFile {
		fw, err := mw.CreateFormFile("file", "wb.xlsx")
		if err != nil {
			t.Fatalf("form file: %v", err)
		}
		if _, err := fw.Write(body); err != nil {
			t.Fatalf("write file: %v", err)
		}
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("close mw: %v", err)
	}
	req := httptest.NewRequest("POST", "/import"+query, &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	importcreate.RunWithLedger(rec, req, httperr.NewValidator(), subtype, shape, s.resolve, s.commit)
	return rec
}

func stockLedgerDetail() [][]string {
	return [][]string{{"display_name", "Imported"}, {"native_currency", "IDR"}}
}

func stockSnaps() [][]string {
	return [][]string{{"2026-01", "2026-01-31", "100", "9500", "IDR", "Jan"}}
}

func TestRunWithLedger(t *testing.T) {
	buyDiv := [][]string{
		{"buy", "2026-01-05", "IDR", "950000", "100", "9500", "", "", "", "", "buy"},
		{"dividend", "2026-02-10", "IDR", "120000", "", "", "", "", "", "", "div"},
	}
	clean := func() []byte {
		return buildLedgerWorkbook(t, stockLedgerDetail(), qtyPriceSnapHeader, stockSnaps(), buyDiv)
	}

	t.Run("invalid mode is 400", func(t *testing.T) {
		rec := runLedgerUpload(t, "?mode=sideways", clean(), true, "stock", snapshotimport.ShapeQuantityPrice, &ledgerStub{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("want 400, got %d", rec.Code)
		}
	})

	t.Run("missing file is 400", func(t *testing.T) {
		rec := runLedgerUpload(t, "", nil, false, "stock", snapshotimport.ShapeQuantityPrice, &ledgerStub{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("want 400, got %d", rec.Code)
		}
	})

	t.Run("not a spreadsheet is 400", func(t *testing.T) {
		rec := runLedgerUpload(t, "", []byte("nope"), true, "stock", snapshotimport.ShapeQuantityPrice, &ledgerStub{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("want 400, got %d", rec.Code)
		}
	})

	t.Run("resolve error is 500", func(t *testing.T) {
		rec := runLedgerUpload(t, "?mode=commit", clean(), true, "stock", snapshotimport.ShapeQuantityPrice,
			&ledgerStub{resolveErr: errors.New("db down")})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("want 500, got %d", rec.Code)
		}
	})

	t.Run("clean preview counts snapshots + ledger, writes nothing", func(t *testing.T) {
		s := &ledgerStub{}
		rec := runLedgerUpload(t, "", clean(), true, "stock", snapshotimport.ShapeQuantityPrice, s)
		if rec.Code != http.StatusOK {
			t.Fatalf("want 200, got %d", rec.Code)
		}
		resp := decodeResp(t, rec)
		if !resp.WouldCreate || resp.Committed || resp.ToInsert != 1 || resp.LedgerToInsert != 2 {
			t.Fatalf("preview: %+v", resp)
		}
		if s.commitCalled {
			t.Error("preview called commit")
		}
	})

	t.Run("clean commit hands snapshots + ledger to commit", func(t *testing.T) {
		s := &ledgerStub{}
		rec := runLedgerUpload(t, "?mode=commit", clean(), true, "stock", snapshotimport.ShapeQuantityPrice, s)
		if rec.Code != http.StatusOK {
			t.Fatalf("want 200, got %d", rec.Code)
		}
		resp := decodeResp(t, rec)
		if !resp.Committed || resp.PositionID == nil {
			t.Fatalf("commit: %+v", resp)
		}
		if !s.commitCalled || s.gotSnaps != 1 || s.gotLedger != 2 {
			t.Errorf("commit got snaps=%d ledger=%d", s.gotSnaps, s.gotLedger)
		}
	})

	t.Run("resolve field error blocks preview and fails commit 422", func(t *testing.T) {
		s := &ledgerStub{fieldErrs: []snapshotimport.FieldError{{Field: "ticker", Message: "is required"}}}
		preview := runLedgerUpload(t, "", clean(), true, "stock", snapshotimport.ShapeQuantityPrice, s)
		if r := decodeResp(t, preview); r.WouldCreate {
			t.Error("field error should block would_create")
		}
		commit := runLedgerUpload(t, "?mode=commit", clean(), true, "stock", snapshotimport.ShapeQuantityPrice,
			&ledgerStub{fieldErrs: []snapshotimport.FieldError{{Field: "ticker", Message: "is required"}}})
		if commit.Code != http.StatusUnprocessableEntity {
			t.Fatalf("want 422, got %d", commit.Code)
		}
	})

	t.Run("bad snapshot row blocks commit 422", func(t *testing.T) {
		bad := buildLedgerWorkbook(t, stockLedgerDetail(), qtyPriceSnapHeader,
			[][]string{{"not-a-month", "", "100", "9500", "IDR", "bad"}}, nil)
		rec := runLedgerUpload(t, "?mode=commit", bad, true, "stock", snapshotimport.ShapeQuantityPrice, &ledgerStub{})
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("want 422, got %d", rec.Code)
		}
		if len(decodeResp(t, rec).Errors) == 0 {
			t.Error("want a row error")
		}
	})

	t.Run("ledger row off the subtype matrix is a row error (422 on commit)", func(t *testing.T) {
		// coupon is not a stock transaction type.
		txns := [][]string{{"coupon", "2026-02-10", "IDR", "50000", "", "", "", "", "", "", "nope"}}
		wb := buildLedgerWorkbook(t, stockLedgerDetail(), qtyPriceSnapHeader, stockSnaps(), txns)
		rec := runLedgerUpload(t, "?mode=commit", wb, true, "stock", snapshotimport.ShapeQuantityPrice, &ledgerStub{})
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("want 422, got %d", rec.Code)
		}
		if errs := decodeResp(t, rec).Errors; len(errs) == 0 || errs[0].Row != 2 {
			t.Fatalf("want a row-2 ledger error, got %+v", errs)
		}
	})

	t.Run("a valid maturity passes validation; a second is rejected", func(t *testing.T) {
		one := [][]string{
			{"maturity", "2030-01-01", "IDR", "", "", "", "10000000", "600000", "cash_out", "cash_out", "matured"},
		}
		s := &ledgerStub{}
		rec := runLedgerUpload(t, "?mode=commit", buildLedgerWorkbook(t, stockLedgerDetail(), accruedSnapHeader, nil, one),
			true, "bond", snapshotimport.ShapeAccruedInterest, s)
		if rec.Code != http.StatusOK || s.gotLedger != 1 {
			t.Fatalf("single maturity should commit, got %d ledger=%d", rec.Code, s.gotLedger)
		}

		two := append(one, []string{"maturity", "2030-02-01", "IDR", "", "", "", "10000000", "600000", "cash_out", "cash_out", "second"})
		rec2 := runLedgerUpload(t, "?mode=commit", buildLedgerWorkbook(t, stockLedgerDetail(), accruedSnapHeader, nil, two),
			true, "bond", snapshotimport.ShapeAccruedInterest, &ledgerStub{})
		if rec2.Code != http.StatusUnprocessableEntity {
			t.Fatalf("second maturity should 422, got %d", rec2.Code)
		}
	})

	t.Run("commit error is 500", func(t *testing.T) {
		rec := runLedgerUpload(t, "?mode=commit", clean(), true, "stock", snapshotimport.ShapeQuantityPrice,
			&ledgerStub{commitErr: errors.New("boom")})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("want 500, got %d", rec.Code)
		}
	})
}
