package snapshotimport

import (
	"bytes"
	"strings"
	"testing"
)

// instructionsText joins the Instructions sheet's lines for substring checks.
func instructionsText(t *testing.T, xlsx []byte) string {
	t.Helper()
	var lines []string
	for _, row := range sheetRows(t, xlsx, instructionsSheet) {
		if len(row) > 0 {
			lines = append(lines, row[0])
		}
	}
	return strings.Join(lines, "\n")
}

// A time deposit's accrued_interest is derived (Value - principal) and the file's
// column ignored (#883), so its template must say so rather than call the column
// optional (#910). The accrued shape is shared with bonds, whose column still
// counts, so the bond wording is unchanged.
func TestInstructions_TimeDepositAccruedIsIgnored(t *testing.T) {
	td, err := BuildTemplate(TemplateMeta{PositionName: "BCA TD", DefaultCurrency: "IDR", Shape: ShapeAccruedInterest, AccruedDerived: true})
	if err != nil {
		t.Fatalf("BuildTemplate(time deposit): %v", err)
	}
	got := instructionsText(t, td)
	for _, want := range []string{"accrued_interest", "ignored", "principal"} {
		if !strings.Contains(got, want) {
			t.Errorf("time deposit instructions should mention %q, got:\n%s", want, got)
		}
	}
	for _, stale := range []string{"leave blank for 0", "accrued-interest component"} {
		if strings.Contains(got, stale) {
			t.Errorf("time deposit instructions still say %q:\n%s", stale, got)
		}
	}

	bond, err := BuildTemplate(TemplateMeta{PositionName: "FR0090", DefaultCurrency: "IDR", Shape: ShapeAccruedInterest})
	if err != nil {
		t.Fatalf("BuildTemplate(bond): %v", err)
	}
	b := instructionsText(t, bond)
	if !strings.Contains(b, "the accrued-interest component, digits only. Optional;") || strings.Contains(b, "ignored") {
		t.Errorf("bond instructions must be unchanged, got:\n%s", b)
	}

	// The time deposit template's own example row still parses.
	parsed, errs, err := Parse(bytes.NewReader(td), Options{DefaultCurrency: "IDR", Shape: ShapeAccruedInterest})
	if err != nil || len(errs) != 0 || len(parsed) != 1 {
		t.Fatalf("time deposit template example should parse as 1 valid row: %d rows, %+v, %v", len(parsed), errs, err)
	}
}
