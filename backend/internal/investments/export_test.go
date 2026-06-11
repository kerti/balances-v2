package investments_test

import (
	"bytes"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"

	"github.com/kerti/balances-v2/backend/internal/snapshotimport"
)

// accrued_interest shape: bond/time_deposit carry amount + accrued_interest.
var accruedHeader = []string{"year_month", "as_of_date", "amount", "accrued_interest", "currency", "description"}

// transactionRows opens an exported workbook and returns the data rows (header
// dropped) of the Transactions sheet.
func transactionRows(t *testing.T, xlsx []byte) [][]string {
	t.Helper()
	f, err := excelize.OpenReader(bytes.NewReader(xlsx))
	if err != nil {
		t.Fatalf("open exported workbook: %v", err)
	}
	defer func() { _ = f.Close() }()
	rows, err := f.GetRows(snapshotimport.TransactionsSheetName)
	if err != nil {
		t.Fatalf("read Transactions sheet: %v", err)
	}
	if len(rows) == 0 {
		t.Fatal("Transactions sheet has no header row")
	}
	return rows[1:]
}

// TestStockHandlers_Export covers the quantity/price snapshot shape: a populated
// workbook streams back with the stock's Detail fields, its Snapshots re-import
// cleanly through the unchanged importer, and its full transaction ledger lands
// on the Transactions sheet.
func TestStockHandlers_Export(t *testing.T) {
	h := newHarness(t)

	t.Run("400 invalid id", func(t *testing.T) {
		rec := h.do(t, "GET", "/investments/stocks/not-a-uuid/export", nil)
		requireStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("404 unknown stock", func(t *testing.T) {
		rec := h.do(t, "GET", "/investments/stocks/"+uuid.NewString()+"/export", nil)
		requireStatus(t, rec, http.StatusNotFound)
	})

	t.Run("round trip: export then re-import", func(t *testing.T) {
		stock := h.createStock(t, "Export stock")
		id := stock.Investment.ID

		// A buy and a dividend so the ledger exercises two shapes.
		h.postTxn(t, id, map[string]any{
			"transaction_type": "buy", "transaction_date": "2026-01-05", "currency": "IDR",
			"amount": "1000000", "quantity": "100", "price_per_unit": "10000", "description": "opening lot",
		})
		h.postTxn(t, id, map[string]any{
			"transaction_type": "dividend", "transaction_date": "2026-02-10", "currency": "IDR",
			"amount": "120000",
		})

		// Snapshot history via the import-commit path (quantity/price shape).
		snapBase := "/investments/" + id.String() + "/snapshots"
		rows := [][]string{
			qtyPriceHeader,
			{"2026-01", "2026-01-31", "100", "10500", "IDR", "Jan"},
			{"2026-02", "2026-02-28", "100", "10800", "", "Feb"},
		}
		commit := h.doUpload(t, snapBase+"/import?mode=commit", buildImportXLSX(t, rows))
		requireStatus(t, commit, http.StatusOK)

		rec := h.do(t, "GET", "/investments/stocks/"+id.String()+"/export", nil)
		requireStatus(t, rec, http.StatusOK)
		if ct := rec.Header().Get("Content-Type"); ct != "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" {
			t.Errorf("content-type: got %q", ct)
		}
		xlsx := rec.Body.Bytes()

		// Detail sheet carries the stock's fields; joint => sole_owner blank.
		detail, err := snapshotimport.ParseDetail(bytes.NewReader(xlsx))
		if err != nil {
			t.Fatalf("ParseDetail: %v", err)
		}
		if detail["display_name"] != "Export stock" {
			t.Errorf("detail display_name: got %q", detail["display_name"])
		}
		if detail["ownership_type"] != "joint" || detail["sole_owner"] != "" {
			t.Errorf("detail ownership: type=%q sole_owner=%q", detail["ownership_type"], detail["sole_owner"])
		}
		if detail["ticker"] != "BBCA" || detail["exchange"] != "IDX" || detail["risk_profile"] != "medium" {
			t.Errorf("detail stock fields: %v", detail)
		}

		// Snapshots sheet re-parses + re-imports as updates (real round trip).
		parsed, rowErrs, err := snapshotimport.Parse(bytes.NewReader(xlsx), snapshotimport.Options{DefaultCurrency: "IDR", Shape: snapshotimport.ShapeQuantityPrice})
		if err != nil || len(rowErrs) != 0 {
			t.Fatalf("Parse exported Snapshots: err=%v rowErrs=%v", err, rowErrs)
		}
		if len(parsed) != 2 {
			t.Fatalf("want 2 snapshot rows, got %d", len(parsed))
		}
		reimport := h.doUpload(t, snapBase+"/import?mode=commit", xlsx)
		requireStatus(t, reimport, http.StatusOK)
		if body := decodeBody[importResp](t, reimport); body.ToUpdate != 2 || body.ToInsert != 0 {
			t.Errorf("re-import counts: want update=2 insert=0, got update=%d insert=%d", body.ToUpdate, body.ToInsert)
		}

		// Transactions sheet carries the full ledger (the buy + the dividend).
		txnRows := transactionRows(t, xlsx)
		if len(txnRows) != 2 {
			t.Fatalf("want 2 transaction rows, got %d: %v", len(txnRows), txnRows)
		}
		var sawBuy, sawDividend bool
		for _, r := range txnRows {
			switch r[0] {
			case "buy":
				sawBuy = true
				if r[3] != "1000000" || r[4] != "100" || r[5] != "10000" {
					t.Errorf("buy row cells: %v", r)
				}
			case "dividend":
				sawDividend = true
				if r[3] != "120000" {
					t.Errorf("dividend amount cell: %v", r)
				}
			}
		}
		if !sawBuy || !sawDividend {
			t.Errorf("ledger missing rows: buy=%v dividend=%v", sawBuy, sawDividend)
		}
	})
}

// TestInvestmentHandlers_ExportSmoke covers the remaining three subtype export
// endpoints: each streams a workbook with its subtype-specific Detail fields and
// an (empty but present) Transactions sheet. Stock + Bond get the deeper
// snapshot/ledger round-trip above; this keeps the others honest cheaply.
func TestInvestmentHandlers_ExportSmoke(t *testing.T) {
	h := newHarness(t)

	cases := []struct {
		name    string
		path    string
		id      func() string
		wantKey string
		wantVal string
	}{
		{
			name:    "mutual fund",
			path:    "mutual-funds",
			id:      func() string { return h.createMutualFund(t, "Export MF").Investment.ID.String() },
			wantKey: "fund_code", wantVal: "BNI-AM",
		},
		{
			name:    "gold",
			path:    "golds",
			id:      func() string { return h.createGold(t, "Export Gold").Investment.ID.String() },
			wantKey: "form", wantVal: "bar",
		},
		{
			name:    "time deposit",
			path:    "time-deposits",
			id:      func() string { return h.createTimeDeposit(t, "Export TD").Investment.ID.String() },
			wantKey: "bank_name", wantVal: "BCA",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			id := tc.id()
			rec := h.do(t, "GET", "/investments/"+tc.path+"/"+id+"/export", nil)
			requireStatus(t, rec, http.StatusOK)
			xlsx := rec.Body.Bytes()

			detail, err := snapshotimport.ParseDetail(bytes.NewReader(xlsx))
			if err != nil {
				t.Fatalf("ParseDetail: %v", err)
			}
			if detail[tc.wantKey] != tc.wantVal {
				t.Errorf("detail[%q] = %q, want %q", tc.wantKey, detail[tc.wantKey], tc.wantVal)
			}

			// The Transactions sheet is present even with no ledger rows.
			if rows := transactionRows(t, xlsx); len(rows) != 0 {
				t.Errorf("want empty ledger, got %d rows", len(rows))
			}
		})
	}
}

// TestBondHandlers_Export covers the accrued-interest snapshot shape. A
// govt_primary bond auto-seeds a Buy at placement (issue #27), so its export
// ships a non-empty ledger without an explicit postTxn.
func TestBondHandlers_Export(t *testing.T) {
	h := newHarness(t)

	t.Run("404 unknown bond", func(t *testing.T) {
		rec := h.do(t, "GET", "/investments/bonds/"+uuid.NewString()+"/export", nil)
		requireStatus(t, rec, http.StatusNotFound)
	})

	t.Run("round trip: export then re-import", func(t *testing.T) {
		bond := h.createBond(t, "Export bond")
		id := bond.Investment.ID

		snapBase := "/investments/" + id.String() + "/snapshots"
		rows := [][]string{
			accruedHeader,
			{"2026-01", "2026-01-31", "10100000", "100000", "IDR", "Jan"},
		}
		commit := h.doUpload(t, snapBase+"/import?mode=commit", buildImportXLSX(t, rows))
		requireStatus(t, commit, http.StatusOK)

		rec := h.do(t, "GET", "/investments/bonds/"+id.String()+"/export", nil)
		requireStatus(t, rec, http.StatusOK)
		xlsx := rec.Body.Bytes()

		detail, err := snapshotimport.ParseDetail(bytes.NewReader(xlsx))
		if err != nil {
			t.Fatalf("ParseDetail: %v", err)
		}
		if detail["display_name"] != "Export bond" {
			t.Errorf("detail display_name: got %q", detail["display_name"])
		}
		if detail["bond_type"] != "govt_primary" || detail["issuer"] != "Govt of Indonesia" {
			t.Errorf("detail bond fields: %v", detail)
		}
		if detail["coupon_frequency"] != "monthly" || detail["maturity_date"] != "2030-01-01" {
			t.Errorf("detail coupon/maturity: %v", detail)
		}

		parsed, rowErrs, err := snapshotimport.Parse(bytes.NewReader(xlsx), snapshotimport.Options{DefaultCurrency: "IDR", Shape: snapshotimport.ShapeAccruedInterest})
		if err != nil || len(rowErrs) != 0 {
			t.Fatalf("Parse exported Snapshots: err=%v rowErrs=%v", err, rowErrs)
		}
		if len(parsed) != 1 || parsed[0].AccruedInterest == nil || parsed[0].AccruedInterest.String() != "100000" {
			t.Fatalf("accrued snapshot round trip: %+v", parsed)
		}
		reimport := h.doUpload(t, snapBase+"/import?mode=commit", xlsx)
		requireStatus(t, reimport, http.StatusOK)
		if body := decodeBody[importResp](t, reimport); body.ToUpdate != 1 || body.ToInsert != 0 {
			t.Errorf("re-import counts: want update=1 insert=0, got update=%d insert=%d", body.ToUpdate, body.ToInsert)
		}

		// The auto-seeded placement Buy is on the Transactions sheet.
		txnRows := transactionRows(t, xlsx)
		if len(txnRows) != 1 || txnRows[0][0] != "buy" {
			t.Fatalf("want 1 seeded buy row, got %v", txnRows)
		}
	})
}
