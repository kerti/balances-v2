package investments_test

import (
	"bytes"
	"net/http"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"

	"github.com/kerti/balances-v2/backend/internal/db"
	"github.com/kerti/balances-v2/backend/internal/testutil"
)

// instructionLines reads the Instructions sheet of a downloaded template.
func instructionLines(t *testing.T, xlsx []byte) string {
	t.Helper()
	f, err := excelize.OpenReader(bytes.NewReader(xlsx))
	if err != nil {
		t.Fatalf("open template: %v", err)
	}
	defer func() { _ = f.Close() }()
	rows, err := f.GetRows("Instructions")
	if err != nil {
		t.Fatalf("read Instructions sheet: %v", err)
	}
	var lines []string
	for _, r := range rows {
		if len(r) > 0 {
			lines = append(lines, r[0])
		}
	}
	return strings.Join(lines, "\n")
}

// The downloaded template says what the importer does with accrued_interest
// (#910): a time deposit's column is ignored (interest is Value − principal), a
// bond's still counts.
//
// covers: INV-SNAPSHOTS-17
func TestImportTemplate_AccruedColumnMatchesBehaviour(t *testing.T) {
	h := newHarness(t)
	download := func(id string) string {
		rec := h.do(t, "GET", "/investments/"+id+"/snapshots/import-template", nil)
		requireStatus(t, rec, http.StatusOK)
		return instructionLines(t, rec.Body.Bytes())
	}

	td := download(h.createTimeDeposit(t, "Template TD").Investment.ID.String())
	if !strings.Contains(td, "accrued_interest — ignored") || strings.Contains(td, "leave blank for 0") {
		t.Errorf("time deposit template must call accrued_interest ignored, got:\n%s", td)
	}

	bond := download(h.createBond(t, "Template bond").Investment.ID.String())
	if strings.Contains(bond, "ignored") || !strings.Contains(bond, "accrued-interest component") {
		t.Errorf("bond template must keep describing accrued_interest, got:\n%s", bond)
	}
}

// Workbook snapshot import into an existing time deposit is confined to the
// deposit's term like every other snapshot write (#911, issue #62): a row
// outside placement..maturity is a 400 OUTSIDE_DEPOSIT_TERM in the preview and
// the commit alike, and the all-or-nothing batch writes nothing. Bonds have no
// term and are unaffected.
//
// covers: INV-BONDS-03
func TestTimeDepositImport_ConfinedToTerm(t *testing.T) {
	// createTimeDeposit's term is 2026-01-01 .. 2027-01-01.
	outside := [][]string{
		accruedHeader,
		{"2026-05", "", "101500000", "", "IDR", "in term"},
		{"2027-02", "", "102000000", "", "IDR", "after maturity"},
	}

	for _, mode := range []string{"preview", "commit"} {
		t.Run(mode+" refuses an out-of-term row with the shared error and writes nothing", func(t *testing.T) {
			h := newHarness(t)
			td := h.createTimeDeposit(t, "Term import "+mode)
			id := td.Investment.ID

			rec := h.doUpload(t, tdImportURL(id, mode), buildImportXLSX(t, outside))
			requireStatus(t, rec, http.StatusBadRequest)
			if !strings.Contains(rec.Body.String(), "OUTSIDE_DEPOSIT_TERM") {
				t.Errorf("want code OUTSIDE_DEPOSIT_TERM, got body %q", rec.Body.String())
			}
			if n := h.rawInvSnapshotCount(t, id); n != 0 {
				t.Errorf("a refused import must leave nothing behind, got %d rows", n)
			}
		})
	}

	t.Run("a month before placement is refused too", func(t *testing.T) {
		h := newHarness(t)
		td := h.createTimeDeposit(t, "Term import early")
		rec := h.doUpload(t, tdImportURL(td.Investment.ID, "commit"), buildImportXLSX(t, [][]string{
			accruedHeader, {"2025-12", "", "100000000", "", "IDR", "before placement"},
		}))
		requireStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("the placement and maturity months import", func(t *testing.T) {
		h := newHarness(t)
		td := h.createTimeDeposit(t, "Term import edges")
		c := h.commitImport(t, td.Investment.ID, [][]string{
			accruedHeader,
			{"2026-01", "", "100000000", "", "IDR", "placement month"},
			{"2027-01", "", "104500000", "", "IDR", "maturity month"},
		})
		if c.ToInsert != 2 {
			t.Fatalf("want insert=2, got %+v", c)
		}
	})

	t.Run("a bond has no term", func(t *testing.T) {
		h := newHarness(t)
		bond := h.createBond(t, "Term import bond")
		h.commitImport(t, bond.Investment.ID, [][]string{
			accruedHeader, {"2019-05", "", "10500000", "1", "IDR", ""},
		})
	})

	t.Run("another household's deposit stays a 404, not a term error", func(t *testing.T) {
		h := newHarness(t)
		td := h.createTimeDeposit(t, "Alice's term TD")
		h.user = testutil.CreateHouseholdWithUser(t, db.New(h.pool), "Bob")
		rec := h.doUpload(t, tdImportURL(td.Investment.ID, "commit"), buildImportXLSX(t, outside))
		requireStatus(t, rec, http.StatusNotFound)
	})
}
