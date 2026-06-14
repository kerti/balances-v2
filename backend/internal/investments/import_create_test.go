package investments_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/xuri/excelize/v2"

	"github.com/kerti/balances-v2/backend/internal/auth"
	"github.com/kerti/balances-v2/backend/internal/repo"
	"github.com/kerti/balances-v2/backend/internal/snapshotimport"
)

// snapRow is the minimal shape of a listed investment snapshot the import tests
// assert on (the maturity-month close reconciliation).
type snapRow struct {
	YearMonth time.Time       `json:"year_month"`
	Amount    decimal.Decimal `json:"amount"`
}

func snapshotsOf(t *testing.T, h *handlerHarness, id string) []snapRow {
	t.Helper()
	rec := h.do(t, "GET", "/investments/"+id+"/snapshots", nil)
	requireStatus(t, rec, http.StatusOK)
	return decodeBody[[]snapRow](t, rec)
}

// Create-from-list import for the five investment subtypes (issue #90): the
// uploaded workbook's Detail sheet becomes a new investment, its Snapshots sheet
// seeds the history, and its Transactions sheet seeds the ledger — atomically.
// These cover the per-subtype commit, the Maturity-last path (decision (b)), the
// ADR-0023 ledger validation, and an export→import round-trip.

// txnHeader is the ADR-0023 Transactions column union (mirrors the unexported
// snapshotimport.transactionHeaders).
var txnHeader = []string{
	"transaction_type", "transaction_date", "currency", "amount",
	"quantity", "price_per_unit",
	"principal_amount", "interest_amount", "principal_disposition", "interest_disposition",
	"description",
}

// createImportResp mirrors importcreate.Response.
type createImportResp struct {
	Mode           string                      `json:"mode"`
	Committed      bool                        `json:"committed"`
	WouldCreate    bool                        `json:"would_create"`
	PositionID     *string                     `json:"position_id"`
	ToInsert       int                         `json:"to_insert"`
	LedgerToInsert int                         `json:"ledger_to_insert"`
	FieldErrors    []snapshotimport.FieldError `json:"field_errors"`
	Errors         []snapshotimport.RowError   `json:"errors"`
}

// buildCreateWorkbook serialises a position workbook with a Detail sheet (field/
// value pairs), a Snapshots sheet (snapHeader + rows), and a Transactions sheet
// (txnHeader + rows) — the shape the create-from-list import parses.
func buildCreateWorkbook(t *testing.T, detail [][]string, snapHeader []string, snapRows, txnRows [][]string) []byte {
	t.Helper()
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()

	write := func(sheet string, rows [][]string) {
		if _, err := f.NewSheet(sheet); err != nil {
			t.Fatalf("new sheet %s: %v", sheet, err)
		}
		for r, row := range rows {
			for c, v := range row {
				cell, _ := excelize.CoordinatesToCellName(c+1, r+1)
				if err := f.SetCellStr(sheet, cell, v); err != nil {
					t.Fatalf("set cell: %v", err)
				}
			}
		}
	}

	detailRows := append([][]string{{"field", "value", "notes"}}, detail...)
	write(snapshotimport.DetailSheetName, detailRows)
	write(snapshotimport.SheetName, append([][]string{snapHeader}, snapRows...))
	write(snapshotimport.TransactionsSheetName, append([][]string{txnHeader}, txnRows...))

	buf, err := f.WriteToBuffer()
	if err != nil {
		t.Fatalf("serialise: %v", err)
	}
	return buf.Bytes()
}

func stockDetail() [][]string {
	return [][]string{
		{"display_name", "Imported BBCA"},
		{"description", "from a file"},
		{"ownership_type", "joint"},
		{"sole_owner", ""},
		{"native_currency", "IDR"},
		{"tag", ""},
		{"risk_profile", "medium"},
		{"ticker", "BBCA"},
		{"exchange", "IDX"},
	}
}

func countList(t *testing.T, h *handlerHarness, path string) int {
	t.Helper()
	rec := h.do(t, "GET", path, nil)
	requireStatus(t, rec, http.StatusOK)
	return len(decodeBody[[]any](t, rec))
}

// covers: INV-IMPORT-01, INV-IMPORT-03
func TestInvestmentImportCreate_Stock(t *testing.T) {
	qtyPrice := qtyPriceHeader
	snaps := [][]string{
		{"2026-01", "2026-01-31", "100", "9500", "IDR", "Jan"},
		{"2026-02", "2026-02-28", "100", "9800", "", "Feb"},
	}
	txns := [][]string{
		{"buy", "2026-01-05", "IDR", "950000", "100", "9500", "", "", "", "", "opening lot"},
		{"dividend", "2026-02-10", "IDR", "120000", "", "", "", "", "", "", "div"},
	}

	t.Run("preview validates without writing", func(t *testing.T) {
		h := newHarness(t)
		rec := h.doUpload(t, "/investments/stocks/import", buildCreateWorkbook(t, stockDetail(), qtyPrice, snaps, txns))
		requireStatus(t, rec, http.StatusOK)
		body := decodeBody[createImportResp](t, rec)
		if !body.WouldCreate || body.Committed || body.ToInsert != 2 || body.LedgerToInsert != 2 {
			t.Fatalf("want clean preview snap=2 ledger=2, got %+v", body)
		}
		if countList(t, h, "/investments/stocks") != 0 {
			t.Error("preview wrote a position")
		}
	})

	t.Run("commit creates stock + snapshots + ledger", func(t *testing.T) {
		h := newHarness(t)
		rec := h.doUpload(t, "/investments/stocks/import?mode=commit", buildCreateWorkbook(t, stockDetail(), qtyPrice, snaps, txns))
		requireStatus(t, rec, http.StatusOK)
		body := decodeBody[createImportResp](t, rec)
		if !body.Committed || body.PositionID == nil {
			t.Fatalf("want committed create with id, got %+v", body)
		}
		id := *body.PositionID
		if countList(t, h, "/investments/stocks") != 1 {
			t.Error("commit did not persist the stock")
		}
		if got := countList(t, h, "/investments/"+id+"/snapshots"); got != 2 {
			t.Errorf("seeded snapshots: want 2, got %d", got)
		}
		if got := countList(t, h, "/investments/"+id+"/transactions"); got != 2 {
			t.Errorf("seeded ledger: want 2, got %d", got)
		}
	})
}

// covers: INV-IMPORT-03
func TestInvestmentImportCreate_MutualFundAndGold(t *testing.T) {
	t.Run("mutual fund commits with buy + distribution", func(t *testing.T) {
		h := newHarness(t)
		detail := [][]string{
			{"display_name", "Imported MF"},
			{"ownership_type", "joint"},
			{"native_currency", "IDR"},
			{"risk_profile", "medium"},
			{"fund_code", "BNI-AM"},
			{"fund_manager", "BNI Asset Management"},
			{"fund_type", "money_market"},
		}
		snaps := [][]string{{"2026-01", "2026-01-31", "100", "9500", "IDR", "Jan"}}
		txns := [][]string{
			{"buy", "2026-01-05", "IDR", "950000", "100", "9500", "", "", "", "", "buy"},
			{"distribution", "2026-03-01", "IDR", "40000", "", "", "", "", "", "", "dist"},
		}
		rec := h.doUpload(t, "/investments/mutual-funds/import?mode=commit", buildCreateWorkbook(t, detail, qtyPriceHeader, snaps, txns))
		requireStatus(t, rec, http.StatusOK)
		body := decodeBody[createImportResp](t, rec)
		if !body.Committed || body.PositionID == nil || body.LedgerToInsert != 2 {
			t.Fatalf("want committed MF with ledger=2, got %+v", body)
		}
		if n := countList(t, h, "/investments/"+*body.PositionID+"/transactions"); n != 2 {
			t.Errorf("MF ledger: want 2, got %d", n)
		}
	})

	t.Run("gold commits with buy + fee", func(t *testing.T) {
		h := newHarness(t)
		detail := [][]string{
			{"display_name", "Imported gold"},
			{"ownership_type", "joint"},
			{"native_currency", "IDR"},
			{"risk_profile", "medium"},
			{"form", "bar"},
			{"purity", "0.9999"},
		}
		snaps := [][]string{{"2026-01", "2026-01-31", "10", "1000000", "IDR", "Jan"}}
		txns := [][]string{
			{"buy", "2026-01-05", "IDR", "10000000", "10", "1000000", "", "", "", "", "buy"},
			{"fee", "2026-02-01", "IDR", "5000", "", "", "", "", "", "", "storage fee"},
		}
		rec := h.doUpload(t, "/investments/golds/import?mode=commit", buildCreateWorkbook(t, detail, qtyPriceHeader, snaps, txns))
		requireStatus(t, rec, http.StatusOK)
		body := decodeBody[createImportResp](t, rec)
		if !body.Committed || body.PositionID == nil || body.LedgerToInsert != 2 {
			t.Fatalf("want committed gold with ledger=2, got %+v", body)
		}
		if n := countList(t, h, "/investments/"+*body.PositionID+"/snapshots"); n != 1 {
			t.Errorf("gold snapshots: want 1, got %d", n)
		}
	})
}

// TestInvestmentImportCreate_OwnerAndTag exercises the two id-typed Detail
// conventions: a sole_owner email resolved to a household member, and a tag
// resolved by name — both seeded onto the new position.
// covers: INV-IMPORT-04
func TestInvestmentImportCreate_OwnerAndTag(t *testing.T) {
	h := newHarness(t)
	tag, err := repo.NewTagRepo(h.pool).CreateTag(auth.WithUser(context.Background(), h.user), "Brokerage", "#22c55e")
	if err != nil {
		t.Fatalf("CreateTag: %v", err)
	}

	detail := [][]string{
		{"display_name", "Sole-owned BBCA"},
		{"ownership_type", "sole"},
		{"sole_owner", h.user.Email},
		{"native_currency", "IDR"},
		{"tag", "Brokerage"},
		{"risk_profile", "medium"},
		{"ticker", "BBCA"},
		{"exchange", "IDX"},
	}
	snaps := [][]string{{"2026-01", "2026-01-31", "100", "9500", "IDR", "Jan"}}
	rec := h.doUpload(t, "/investments/stocks/import?mode=commit", buildCreateWorkbook(t, detail, qtyPriceHeader, snaps, nil))
	requireStatus(t, rec, http.StatusOK)
	body := decodeBody[createImportResp](t, rec)
	if !body.Committed || body.PositionID == nil {
		t.Fatalf("want committed sole-owned stock, got %+v", body)
	}

	got := decodeBody[*repo.Stock](t, h.do(t, "GET", "/investments/stocks/"+*body.PositionID, nil))
	if got.Investment.OwnershipType != "sole" || got.Investment.SoleOwnerUserID == nil || *got.Investment.SoleOwnerUserID != h.user.ID {
		t.Errorf("sole owner not resolved: %+v", got.Investment)
	}
	if got.Investment.TagID == nil || *got.Investment.TagID != tag.ID {
		t.Errorf("tag not resolved/assigned: want %s, got %v", tag.ID, got.Investment.TagID)
	}

	t.Run("unknown sole_owner email is a field error", func(t *testing.T) {
		bad := [][]string{
			{"display_name", "Bad owner"},
			{"ownership_type", "sole"},
			{"sole_owner", "nobody@example.com"},
			{"native_currency", "IDR"},
			{"risk_profile", "medium"},
			{"ticker", "BBCA"},
			{"exchange", "IDX"},
		}
		rec := h.doUpload(t, "/investments/stocks/import?mode=commit", buildCreateWorkbook(t, bad, qtyPriceHeader, nil, nil))
		requireStatus(t, rec, http.StatusUnprocessableEntity)
		b := decodeBody[createImportResp](t, rec)
		if len(b.FieldErrors) == 0 || b.FieldErrors[0].Field != "sole_owner" {
			t.Fatalf("want a sole_owner field error, got %+v", b.FieldErrors)
		}
	})
}

// covers: INV-IMPORT-02, INV-COST-BASIS-04
func TestInvestmentImportCreate_FieldAndLedgerErrors(t *testing.T) {
	qtyPrice := qtyPriceHeader

	t.Run("missing required Detail field is 422 (no write)", func(t *testing.T) {
		h := newHarness(t)
		detail := [][]string{
			{"display_name", "No ticker"},
			{"ownership_type", "joint"},
			{"native_currency", "IDR"},
			{"risk_profile", "medium"},
			{"ticker", ""}, // required
			{"exchange", "IDX"},
		}
		rec := h.doUpload(t, "/investments/stocks/import?mode=commit", buildCreateWorkbook(t, detail, qtyPrice, nil, nil))
		requireStatus(t, rec, http.StatusUnprocessableEntity)
		body := decodeBody[createImportResp](t, rec)
		if body.WouldCreate || len(body.FieldErrors) == 0 {
			t.Fatalf("want field errors, got %+v", body)
		}
		if countList(t, h, "/investments/stocks") != 0 {
			t.Error("422 commit wrote a position")
		}
	})

	t.Run("ledger row violating the subtype matrix is 422", func(t *testing.T) {
		h := newHarness(t)
		// Stock does not accept 'coupon' (bond/time-deposit-only-ish).
		txns := [][]string{
			{"coupon", "2026-02-10", "IDR", "50000", "", "", "", "", "", "", "not for stock"},
		}
		rec := h.doUpload(t, "/investments/stocks/import?mode=commit", buildCreateWorkbook(t, stockDetail(), qtyPrice, nil, txns))
		requireStatus(t, rec, http.StatusUnprocessableEntity)
		body := decodeBody[createImportResp](t, rec)
		if len(body.Errors) == 0 || body.Errors[0].Row != 2 {
			t.Fatalf("want a row-2 ledger error, got %+v", body.Errors)
		}
		if countList(t, h, "/investments/stocks") != 0 {
			t.Error("422 commit wrote a position")
		}
	})

	t.Run("a second maturity row is rejected", func(t *testing.T) {
		h := newHarness(t)
		txns := [][]string{
			{"maturity", "2030-01-01", "IDR", "", "", "", "10000000", "600000", "cash_out", "cash_out", "first"},
			{"maturity", "2030-02-01", "IDR", "", "", "", "10000000", "600000", "cash_out", "cash_out", "second"},
		}
		rec := h.doUpload(t, "/investments/bonds/import?mode=commit", buildCreateWorkbook(t, bondDetail(), accruedHeader, nil, txns))
		requireStatus(t, rec, http.StatusUnprocessableEntity)
		body := decodeBody[createImportResp](t, rec)
		if len(body.Errors) == 0 {
			t.Fatalf("want a maturity-count error, got %+v", body)
		}
	})
}

func bondDetail() [][]string {
	return [][]string{
		{"display_name", "Imported SBR"},
		{"ownership_type", "joint"},
		{"native_currency", "IDR"},
		{"risk_profile", "medium"},
		{"bond_type", "govt_primary"},
		{"series_code", "SBR012"},
		{"issuer", "Govt of Indonesia"},
		{"coupon_rate", "6.25"},
		{"coupon_frequency", "monthly"},
		{"maturity_date", "2030-01-01"},
	}
}

// TestInvestmentImportCreate_Maturity covers decision (b): a Maturity row applies
// last and legitimately matures the position with its 0-value close snapshot. The
// bond case also proves the placement Buy on the ledger is seeded once (the import
// path does not auto-seed, unlike CreateBond) and that a coupon survives.
// covers: INV-IMPORT-03
func TestInvestmentImportCreate_Maturity(t *testing.T) {
	t.Run("bond matures with full ledger", func(t *testing.T) {
		h := newHarness(t)
		snaps := [][]string{{"2025-06", "2025-06-30", "10100000", "100000", "IDR", "mid"}}
		txns := [][]string{
			{"buy", "2025-01-15", "IDR", "10000000", "10", "1000000", "", "", "", "", "placement"},
			{"coupon", "2025-06-01", "IDR", "52083", "", "", "", "", "", "", "monthly coupon"},
			{"maturity", "2030-01-01", "IDR", "", "", "", "10000000", "600000", "cash_out", "cash_out", "matured"},
		}
		rec := h.doUpload(t, "/investments/bonds/import?mode=commit", buildCreateWorkbook(t, bondDetail(), accruedHeader, snaps, txns))
		requireStatus(t, rec, http.StatusOK)
		body := decodeBody[createImportResp](t, rec)
		if !body.Committed || body.PositionID == nil || body.LedgerToInsert != 3 {
			t.Fatalf("want committed bond with ledger=3, got %+v", body)
		}
		id := *body.PositionID

		got := decodeBody[*repo.Bond](t, h.do(t, "GET", "/investments/bonds/"+id, nil))
		if got.Investment.Status != "matured" {
			t.Errorf("status: want matured, got %q", got.Investment.Status)
		}
		if got.Investment.TerminatedAt == nil {
			t.Error("terminated_at not set on a seeded maturity")
		}
		if n := countList(t, h, "/investments/"+id+"/transactions"); n != 3 {
			t.Errorf("ledger rows: want 3, got %d", n)
		}
		// The seeded mid snapshot plus the auto-written 0 close at the maturity
		// month: 2 distinct months.
		if n := countList(t, h, "/investments/"+id+"/snapshots"); n != 2 {
			t.Errorf("snapshots: want 2 (mid + close), got %d", n)
		}
	})

	t.Run("time deposit matures from a single maturity row", func(t *testing.T) {
		h := newHarness(t)
		detail := [][]string{
			{"display_name", "Imported TD"},
			{"ownership_type", "joint"},
			{"native_currency", "IDR"},
			{"risk_profile", "medium"},
			{"bank_name", "BCA"},
			{"principal", "100000000"},
			{"interest_rate", "4.5"},
			{"term_months", "12"},
			{"placement_date", "2026-01-01"},
			{"maturity_date", "2027-01-01"},
			{"rollover_policy", "no_rollover"},
		}
		txns := [][]string{
			{"maturity", "2027-01-01", "IDR", "", "", "", "100000000", "4500000", "cash_out", "cash_out", "matured"},
		}
		rec := h.doUpload(t, "/investments/time-deposits/import?mode=commit", buildCreateWorkbook(t, detail, accruedHeader, nil, txns))
		requireStatus(t, rec, http.StatusOK)
		body := decodeBody[createImportResp](t, rec)
		if !body.Committed || body.PositionID == nil {
			t.Fatalf("want committed TD, got %+v", body)
		}
		id := *body.PositionID
		got := decodeBody[*repo.TimeDeposit](t, h.do(t, "GET", "/investments/time-deposits/"+id, nil))
		if got.Investment.Status != "matured" {
			t.Errorf("status: want matured, got %q", got.Investment.Status)
		}
		if n := countList(t, h, "/investments/"+id+"/snapshots"); n != 1 {
			t.Errorf("snapshots: want 1 (the 0 close), got %d", n)
		}
	})
}

// TestInvestmentImportCreate_MaturityReconciliation proves the two ordering
// guarantees of the seed: the 0-value close snapshot wins over a seeded snapshot
// in the maturity month (snapshots-before-ledger), and a Maturity row listed
// before other rows still produces the matured position (seedLedger applies it
// last).
func TestInvestmentImportCreate_MaturityReconciliation(t *testing.T) {
	t.Run("0 close overwrites a seeded snapshot in the maturity month", func(t *testing.T) {
		h := newHarness(t)
		// A pre-maturity reading in the SAME month as maturity (2030-01).
		snaps := [][]string{{"2030-01", "2030-01-15", "10500000", "0", "IDR", "pre-maturity"}}
		txns := [][]string{
			{"maturity", "2030-01-01", "IDR", "", "", "", "10000000", "600000", "cash_out", "cash_out", "matured"},
		}
		rec := h.doUpload(t, "/investments/bonds/import?mode=commit", buildCreateWorkbook(t, bondDetail(), accruedHeader, snaps, txns))
		requireStatus(t, rec, http.StatusOK)
		id := *decodeBody[createImportResp](t, rec).PositionID

		rows := snapshotsOf(t, h, id)
		if len(rows) != 1 {
			t.Fatalf("want 1 snapshot (the reconciled close), got %d", len(rows))
		}
		if !rows[0].Amount.IsZero() {
			t.Errorf("maturity-month snapshot: want 0 (close overwrote the seeded reading), got %s", rows[0].Amount)
		}
	})

	t.Run("maturity listed first still matures the position", func(t *testing.T) {
		h := newHarness(t)
		// Maturity precedes a coupon in file order; the seed must reorder it last.
		txns := [][]string{
			{"maturity", "2030-01-01", "IDR", "", "", "", "10000000", "600000", "cash_out", "cash_out", "matured"},
			{"coupon", "2025-06-01", "IDR", "52083", "", "", "", "", "", "", "earlier coupon"},
		}
		rec := h.doUpload(t, "/investments/bonds/import?mode=commit", buildCreateWorkbook(t, bondDetail(), accruedHeader, nil, txns))
		requireStatus(t, rec, http.StatusOK)
		body := decodeBody[createImportResp](t, rec)
		if !body.Committed || body.PositionID == nil || body.LedgerToInsert != 2 {
			t.Fatalf("want committed bond with ledger=2, got %+v", body)
		}
		got := decodeBody[*repo.Bond](t, h.do(t, "GET", "/investments/bonds/"+*body.PositionID, nil))
		if got.Investment.Status != "matured" {
			t.Errorf("status: want matured, got %q", got.Investment.Status)
		}
	})
}

// TestInvestmentImportCreate_TransportErrors covers the shared readUpload guards
// on an investment import endpoint: bad mode, missing file, and a non-spreadsheet
// upload all 4xx without writing.
// covers: INV-IMPORT-01
func TestInvestmentImportCreate_TransportErrors(t *testing.T) {
	h := newHarness(t)
	good := buildCreateWorkbook(t, stockDetail(), qtyPriceHeader, nil, nil)

	t.Run("400 invalid mode", func(t *testing.T) {
		rec := h.doUpload(t, "/investments/stocks/import?mode=sideways", good)
		requireStatus(t, rec, http.StatusBadRequest)
	})
	t.Run("400 missing file", func(t *testing.T) {
		rec := h.doUpload(t, "/investments/stocks/import", nil)
		requireStatus(t, rec, http.StatusBadRequest)
	})
	t.Run("400 not a spreadsheet", func(t *testing.T) {
		rec := h.doUpload(t, "/investments/stocks/import", []byte("this is not xlsx"))
		requireStatus(t, rec, http.StatusBadRequest)
	})
	if countList(t, h, "/investments/stocks") != 0 {
		t.Error("a 4xx upload wrote a position")
	}
}

// TestInvestmentImportCreate_UnknownTagUntagged: a tag name that matches no Tag
// leaves the new position untagged (the create-import contract — an unmatched tag
// is not an error).
// covers: INV-IMPORT-04
func TestInvestmentImportCreate_UnknownTagUntagged(t *testing.T) {
	h := newHarness(t)
	detail := append(stockDetail()[:0:0], stockDetail()...)
	for i := range detail {
		if detail[i][0] == "tag" {
			detail[i] = []string{"tag", "No Such Tag"}
		}
	}
	rec := h.doUpload(t, "/investments/stocks/import?mode=commit", buildCreateWorkbook(t, detail, qtyPriceHeader, nil, nil))
	requireStatus(t, rec, http.StatusOK)
	body := decodeBody[createImportResp](t, rec)
	if !body.Committed || body.PositionID == nil {
		t.Fatalf("want committed stock, got %+v", body)
	}
	got := decodeBody[*repo.Stock](t, h.do(t, "GET", "/investments/stocks/"+*body.PositionID, nil))
	if got.Investment.TagID != nil {
		t.Errorf("unknown tag should leave the position untagged, got tag_id %v", got.Investment.TagID)
	}
}

// TestInvestmentImportCreate_RoundTrip exports a seeded stock then re-imports the
// exported workbook, proving the export format round-trips back through the
// create-from-list flow into an equivalent position + ledger.
// covers: INV-IMPORT-05
func TestInvestmentImportCreate_RoundTrip(t *testing.T) {
	h := newHarness(t)
	stock := h.createStock(t, "Round-trip stock")
	id := stock.Investment.ID

	h.postTxn(t, id, map[string]any{
		"transaction_type": "buy", "transaction_date": "2026-01-05", "currency": "IDR",
		"amount": "950000", "quantity": "100", "price_per_unit": "9500",
	})
	h.postTxn(t, id, map[string]any{
		"transaction_type": "dividend", "transaction_date": "2026-02-10", "currency": "IDR", "amount": "120000",
	})

	exported := h.do(t, "GET", "/investments/stocks/"+id.String()+"/export", nil)
	requireStatus(t, exported, http.StatusOK)

	rec := h.doUpload(t, "/investments/stocks/import?mode=commit", exported.Body.Bytes())
	requireStatus(t, rec, http.StatusOK)
	body := decodeBody[createImportResp](t, rec)
	if !body.Committed || body.PositionID == nil || body.LedgerToInsert != 2 {
		t.Fatalf("want round-trip commit with ledger=2, got %+v", body)
	}
	if countList(t, h, "/investments/stocks") != 2 {
		t.Error("round-trip did not yield a second stock")
	}
	if n := countList(t, h, "/investments/"+*body.PositionID+"/transactions"); n != 2 {
		t.Errorf("round-trip ledger: want 2, got %d", n)
	}
}
