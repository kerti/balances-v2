package investments_test

import (
	"bytes"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/kerti/balances-v2/backend/internal/db"
	"github.com/kerti/balances-v2/backend/internal/snapshotimport"
	"github.com/kerti/balances-v2/backend/internal/testutil"
)

// Workbook import stores a time deposit's accrued_interest as Value − principal
// in force, ignoring the file's accrued column, exactly as every other write path
// does since #844 (#883). Bonds keep the file's figure; the 0/0 close snapshot at
// termination stays 0/0.

func tdImportURL(id uuid.UUID, mode string) string {
	return "/investments/" + id.String() + "/snapshots/import?mode=" + mode
}

// commitImport posts rows (header included) as a snapshot import and returns the
// counts of a 200 commit.
func (h *handlerHarness) commitImport(t *testing.T, id uuid.UUID, rows [][]string) importResp {
	t.Helper()
	rec := h.doUpload(t, tdImportURL(id, "commit"), buildImportXLSX(t, rows))
	requireStatus(t, rec, http.StatusOK)
	return decodeBody[importResp](t, rec)
}

// covers: INV-SNAPSHOTS-17
func TestTimeDepositImport_AccruedIsDerived(t *testing.T) {
	t.Run("a mismatching accrued column is stored derived; preview and re-import agree", func(t *testing.T) {
		h := newHarness(t)
		td := h.createTimeDeposit(t, "Import derived") // principal 100,000,000
		id := td.Investment.ID
		rows := [][]string{
			accruedHeader,
			{"2026-05", "2026-05-31", "101500000", "7", "IDR", "file says 7"},
			{"2026-06", "2026-06-30", "102000000", "", "IDR", "file says blank"},
		}

		preview := h.doUpload(t, tdImportURL(id, "preview"), buildImportXLSX(t, rows))
		requireStatus(t, preview, http.StatusOK)
		if c := decodeBody[importResp](t, preview); c.ToInsert != 2 || c.Committed {
			t.Fatalf("preview: want insert=2 uncommitted, got %+v", c)
		}
		if n := h.rawInvSnapshotCount(t, id); n != 0 {
			t.Fatalf("preview wrote %d rows", n)
		}

		if c := h.commitImport(t, id, rows); c.ToInsert != 2 {
			t.Fatalf("commit: want insert=2, got %+v", c)
		}
		requireAccrued(t, h, id, "2026-05", "101500000", "1500000")
		requireAccrued(t, h, id, "2026-06", "102000000", "2000000")

		// The same file again: classified against the derived figure, so nothing
		// differs and nothing is archived (a file whose accrued disagrees must not
		// look "changed" forever).
		if c := h.commitImport(t, id, rows); c.Unchanged != 2 || c.ToUpdate != 0 || c.ToInsert != 0 {
			t.Errorf("re-import: want unchanged=2, got %+v", c)
		}
		if n := h.rawInvSnapshotCount(t, id); n != 2 {
			t.Errorf("re-import must not add rows: want 2 live, got %d", n)
		}
	})

	t.Run("replacing a month re-derives from the new Value", func(t *testing.T) {
		h := newHarness(t)
		td := h.createTimeDeposit(t, "Import replace")
		id := td.Investment.ID
		h.commitImport(t, id, [][]string{accruedHeader, {"2026-05", "2026-05-31", "101500000", "1500000", "IDR", ""}})
		c := h.commitImport(t, id, [][]string{accruedHeader, {"2026-05", "2026-05-31", "103000000", "1500000", "IDR", ""}})
		if c.ToUpdate != 1 {
			t.Fatalf("want update=1, got %+v", c)
		}
		requireAccrued(t, h, id, "2026-05", "103000000", "3000000")
	})

	t.Run("zero, equal to principal, below principal, and a soft-deleted month", func(t *testing.T) {
		h := newHarness(t)
		td := h.createTimeDeposit(t, "Import boundaries")
		id := td.Investment.ID
		// A deleted month is inserted afresh, not replaced.
		rec := h.do(t, "POST", "/investments/"+id.String()+"/snapshots", map[string]any{
			"year_month": "2026-04", "amount": "100500000", "currency": "IDR",
		})
		requireStatus(t, rec, http.StatusCreated)
		snap := decodeBody[db.InvestmentSnapshot](t, rec)
		requireStatus(t, h.do(t, "DELETE", "/investments/"+id.String()+"/snapshots/"+snap.ID.String(), nil), http.StatusNoContent)

		c := h.commitImport(t, id, [][]string{
			accruedHeader,
			{"2026-02", "", "0", "0", "IDR", "zero Value on an active deposit"},
			{"2026-03", "", "100000000", "999", "IDR", "equal"},
			{"2026-04", "", "99000000", "5", "IDR", "below, over a deleted month"},
		})
		if c.ToInsert != 3 || c.ToUpdate != 0 {
			t.Fatalf("want insert=3 (deleted month is a fresh insert), got %+v", c)
		}
		// An active deposit has no close snapshot: a zero Value derives like any other.
		requireAccrued(t, h, id, "2026-02", "0", "-100000000")
		requireAccrued(t, h, id, "2026-03", "100000000", "0")
		requireAccrued(t, h, id, "2026-04", "99000000", "-1000000")
	})

	t.Run("a second currency keeps its cents exact", func(t *testing.T) {
		h := newHarness(t)
		td := h.createTDWith(t, "Import USD", "USD", "1000.5000", nil)
		id := td.Investment.ID
		h.commitImport(t, id, [][]string{accruedHeader, {"2026-05", "", "1234.56", "1", "USD", ""}})
		requireAccrued(t, h, id, "2026-05", "1234.56", "234.06")
	})

	t.Run("a rolled-over deposit derives from its own principal", func(t *testing.T) {
		h := newHarness(t)
		src := h.createTimeDeposit(t, "Import rollover source")
		succ := h.createTDWith(t, "Import rollover successor", "IDR", "101125000", &src.Investment.ID)
		h.commitImport(t, succ.Investment.ID, [][]string{accruedHeader, {"2026-06", "", "101500000", "1500000", "IDR", ""}})
		requireAccrued(t, h, succ.Investment.ID, "2026-06", "101500000", "375000")
	})

	t.Run("a bond keeps the file's accrued column", func(t *testing.T) {
		h := newHarness(t)
		bond := h.createBond(t, "Import bond")
		h.commitImport(t, bond.Investment.ID, [][]string{accruedHeader, {"2026-05", "", "10500000", "123456", "IDR", ""}})
		requireAccrued(t, h, bond.Investment.ID, "2026-05", "10500000", "123456")
	})

	t.Run("another household's deposit is a 404 and writes nothing", func(t *testing.T) {
		h := newHarness(t)
		td := h.createTimeDeposit(t, "Alice's TD")
		bob := testutil.CreateHouseholdWithUser(t, db.New(h.pool), "Bob")
		h.user = bob
		rec := h.doUpload(t, tdImportURL(td.Investment.ID, "commit"),
			buildImportXLSX(t, [][]string{accruedHeader, {"2026-05", "", "101500000", "1", "IDR", ""}}))
		requireStatus(t, rec, http.StatusNotFound)
		if n := h.rawInvSnapshotCount(t, td.Investment.ID); n != 0 {
			t.Errorf("a cross-household import must leave nothing behind, got %d rows", n)
		}
	})
}

// A matured deposit's 0/0 close snapshot is exported with the rest of its
// history; importing it back must not turn the 0 into 0 − principal (#883).
//
// covers: INV-SNAPSHOTS-17
func TestTimeDepositImport_CloseSnapshotStaysZero(t *testing.T) {
	h := newHarness(t)
	td := h.createTimeDeposit(t, "Import close") // 2026-01..2027-01, principal 100,000,000
	id := td.Investment.ID
	requireStatus(t, h.do(t, "POST", "/investments/"+id.String()+"/snapshots", map[string]any{
		"year_month": "2026-12", "amount": "104500000", "currency": "IDR",
	}), http.StatusCreated)
	h.postTxn(t, id, map[string]any{
		"transaction_type": "maturity", "transaction_date": "2027-01-01", "currency": "IDR",
		"principal_amount": "100000000", "interest_amount": "4500000",
		"principal_disposition": "cash_out", "interest_disposition": "cash_out",
	})
	requireAccrued(t, h, id, "2027-01", "0", "0") // the close, written by the lifecycle

	t.Run("the exported file re-imports unchanged", func(t *testing.T) {
		exported := h.do(t, "GET", "/investments/time-deposits/"+id.String()+"/export", nil)
		requireStatus(t, exported, http.StatusOK)
		rec := h.doUpload(t, tdImportURL(id, "commit"), exported.Body.Bytes())
		requireStatus(t, rec, http.StatusOK)
		if c := decodeBody[importResp](t, rec); c.Unchanged != 2 || c.ToUpdate != 0 || c.ToInsert != 0 {
			t.Errorf("want both months unchanged, got %+v", c)
		}
		requireAccrued(t, h, id, "2027-01", "0", "0")
		requireAccrued(t, h, id, "2026-12", "104500000", "4500000")
	})

	t.Run("a close row carrying a stray accrued figure is stored 0/0", func(t *testing.T) {
		c := h.commitImport(t, id, [][]string{
			accruedHeader,
			{"2027-01", "2027-01-02", "0", "55", "IDR", "stray accrued on the close"}, // as-of differs: replaces the close
		})
		if c.ToUpdate != 1 {
			t.Fatalf("want the close replaced (update=1), got %+v", c)
		}
		requireAccrued(t, h, id, "2027-01", "0", "0")
	})
}

// Export → import → export is symmetric for a time deposit: re-importing the
// workbook into the same deposit changes nothing, and importing it as a new
// position (import-create) yields the same snapshot amounts and accrued (#883).
//
// covers: INV-SNAPSHOTS-17
func TestTimeDepositImport_RoundTripIsSymmetric(t *testing.T) {
	h := newHarness(t)
	td := h.createTimeDeposit(t, "Round-trip TD") // principal 100,000,000
	id := td.Investment.ID
	for _, s := range [][2]string{{"2026-05", "101500000"}, {"2026-06", "102100000"}, {"2026-12", "99500000"}} {
		requireStatus(t, h.do(t, "POST", "/investments/"+id.String()+"/snapshots", map[string]any{
			"year_month": s[0], "amount": s[1], "currency": "IDR",
		}), http.StatusCreated)
	}
	h.postTxn(t, id, map[string]any{
		"transaction_type": "maturity", "transaction_date": "2027-01-01", "currency": "IDR",
		"principal_amount": "100000000", "interest_amount": "4500000",
		"principal_disposition": "cash_out", "interest_disposition": "cash_out",
	})

	export := func(path string) []byte {
		rec := h.do(t, "GET", path, nil)
		requireStatus(t, rec, http.StatusOK)
		return rec.Body.Bytes()
	}
	parseSnaps := func(xlsx []byte) []snapshotimport.ParsedRow {
		rows, rowErrs, err := snapshotimport.Parse(bytes.NewReader(xlsx), snapshotimport.Options{
			DefaultCurrency: "IDR", Shape: snapshotimport.ShapeAccruedInterest,
		})
		if err != nil || len(rowErrs) != 0 {
			t.Fatalf("parse export: err=%v rowErrs=%v", err, rowErrs)
		}
		return rows
	}

	first := export("/investments/time-deposits/" + id.String() + "/export")
	want := parseSnaps(first)
	if len(want) != 4 {
		t.Fatalf("setup: want 3 readings + the close, got %d", len(want))
	}

	t.Run("re-import into the same deposit changes nothing", func(t *testing.T) {
		rec := h.doUpload(t, tdImportURL(id, "commit"), first)
		requireStatus(t, rec, http.StatusOK)
		if c := decodeBody[importResp](t, rec); c.Unchanged != 4 || c.ToUpdate != 0 || c.ToInsert != 0 {
			t.Errorf("want unchanged=4, got %+v", c)
		}
		after := parseSnaps(export("/investments/time-deposits/" + id.String() + "/export"))
		requireSameSnapshots(t, want, after)
	})

	t.Run("import-create from the export reproduces amounts and accrued", func(t *testing.T) {
		rec := h.doUpload(t, "/investments/time-deposits/import?mode=commit", first)
		requireStatus(t, rec, http.StatusOK)
		body := decodeBody[createImportResp](t, rec)
		if !body.Committed || body.PositionID == nil {
			t.Fatalf("want committed import-create, got %+v", body)
		}
		newID := uuid.MustParse(*body.PositionID)
		requireAccrued(t, h, newID, "2026-05", "101500000", "1500000")
		requireAccrued(t, h, newID, "2026-12", "99500000", "-500000")
		requireAccrued(t, h, newID, "2027-01", "0", "0")

		got := parseSnaps(export("/investments/time-deposits/" + *body.PositionID + "/export"))
		requireSameSnapshots(t, want, got)
	})
}

func requireSameSnapshots(t *testing.T, want, got []snapshotimport.ParsedRow) {
	t.Helper()
	if len(want) != len(got) {
		t.Fatalf("snapshot count: want %d, got %d", len(want), len(got))
	}
	for i := range want {
		w, g := want[i], got[i]
		if !w.YearMonth.Equal(g.YearMonth) || !w.Amount.Equal(g.Amount) ||
			w.AccruedInterest == nil || g.AccruedInterest == nil || !w.AccruedInterest.Equal(*g.AccruedInterest) {
			t.Errorf("row %d: want %s %s/%v, got %s %s/%v", i,
				w.YearMonth.Format("2006-01"), w.Amount, w.AccruedInterest,
				g.YearMonth.Format("2006-01"), g.Amount, g.AccruedInterest)
		}
	}
}

// Import-create seeds snapshots after writing the deposit's details, so the
// principal is in hand to derive from; a seeded close month still ends 0/0 and a
// bond's seeded accrued stays as given (#883).
//
// covers: INV-SNAPSHOTS-17
func TestTimeDepositImportCreate_AccruedIsDerived(t *testing.T) {
	tdDetail := func(currency, principal string) [][]string {
		return [][]string{
			{"display_name", "Imported TD derived"},
			{"ownership_type", "joint"},
			{"native_currency", currency},
			{"risk_profile", "medium"},
			{"bank_name", "BCA"},
			{"principal", principal},
			{"interest_rate", "4.5"},
			{"term_months", "12"},
			{"placement_date", "2026-01-01"},
			{"maturity_date", "2027-01-01"},
			{"rollover_policy", "no_rollover"},
		}
	}
	commit := func(h *handlerHarness, path string, detail [][]string, snaps, txns [][]string) uuid.UUID {
		rec := h.doUpload(t, path+"?mode=commit", buildCreateWorkbook(t, detail, accruedHeader, snaps, txns))
		requireStatus(t, rec, http.StatusOK)
		body := decodeBody[createImportResp](t, rec)
		if !body.Committed || body.PositionID == nil {
			t.Fatalf("want committed, got %+v", body)
		}
		return uuid.MustParse(*body.PositionID)
	}

	t.Run("a mismatching accrued column is stored derived", func(t *testing.T) {
		h := newHarness(t)
		id := commit(h, "/investments/time-deposits/import", tdDetail("IDR", "100000000"), [][]string{
			{"2026-05", "2026-05-31", "101500000", "7", "IDR", ""},
			{"2026-06", "2026-06-30", "99000000", "", "IDR", ""},
		}, nil)
		requireAccrued(t, h, id, "2026-05", "101500000", "1500000")
		requireAccrued(t, h, id, "2026-06", "99000000", "-1000000")
	})

	t.Run("a second currency keeps its cents exact", func(t *testing.T) {
		h := newHarness(t)
		id := commit(h, "/investments/time-deposits/import", tdDetail("USD", "1000.50"), [][]string{
			{"2026-05", "", "1234.56", "9", "USD", ""},
		}, nil)
		requireAccrued(t, h, id, "2026-05", "1234.56", "234.06")
	})

	t.Run("the maturity close wins over a seeded close-month row and stays 0/0", func(t *testing.T) {
		h := newHarness(t)
		txns := [][]string{{"maturity", "2027-01-01", "IDR", "", "", "", "100000000", "4500000", "cash_out", "cash_out", ""}}
		id := commit(h, "/investments/time-deposits/import", tdDetail("IDR", "100000000"), [][]string{
			{"2026-12", "2026-12-31", "104500000", "1", "IDR", ""},
			{"2027-01", "2027-01-01", "0", "123", "IDR", "close row with a stray accrued"},
		}, txns)
		requireAccrued(t, h, id, "2026-12", "104500000", "4500000")
		requireAccrued(t, h, id, "2027-01", "0", "0")
	})

	t.Run("a bond's seeded accrued stays as given", func(t *testing.T) {
		h := newHarness(t)
		id := commit(h, "/investments/bonds/import", bondDetail(), [][]string{
			{"2026-05", "2026-05-31", "10500000", "123456", "IDR", ""},
		}, nil)
		requireAccrued(t, h, id, "2026-05", "10500000", "123456")
	})
}
