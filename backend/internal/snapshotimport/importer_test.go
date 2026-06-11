package snapshotimport

import (
	"bytes"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/xuri/excelize/v2"
)

// buildXLSX writes the given rows (including the header row) to the Snapshots
// sheet of an in-memory .xlsx and returns its bytes.
func buildXLSX(t *testing.T, rows [][]string) []byte {
	t.Helper()
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()
	if _, err := f.NewSheet(SheetName); err != nil {
		t.Fatalf("new sheet: %v", err)
	}
	for r, row := range rows {
		for c, v := range row {
			cell, _ := excelize.CoordinatesToCellName(c+1, r+1)
			if err := f.SetCellStr(SheetName, cell, v); err != nil {
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

var header = []string{"year_month", "as_of_date", "amount", "currency", "description"}

func parse(t *testing.T, rows [][]string, opts Options) ([]ParsedRow, []RowError) {
	t.Helper()
	in := append([][]string{header}, rows...)
	parsed, errs, err := Parse(bytes.NewReader(buildXLSX(t, in)), opts)
	if err != nil {
		t.Fatalf("Parse returned file error: %v", err)
	}
	return parsed, errs
}

// parseShaped is like parse but prepends the shape's own header row, so the
// data rows line up with the shape's column layout.
func parseShaped(t *testing.T, shape Shape, rows [][]string, opts Options) ([]ParsedRow, []RowError) {
	t.Helper()
	in := append([][]string{headersFor(shape)}, rows...)
	opts.Shape = shape
	parsed, errs, err := Parse(bytes.NewReader(buildXLSX(t, in)), opts)
	if err != nil {
		t.Fatalf("Parse returned file error: %v", err)
	}
	return parsed, errs
}

func TestParse_HappyPath(t *testing.T) {
	parsed, errs := parse(t, [][]string{
		{"2015-01", "2015-01-31", "10000000", "IDR", "Opening"},
		{"2015-02", "", "10500000", "", ""},          // currency defaults, no as_of/desc
		{"", "2015-03-31", "11000000", "usd", "Mar"}, // month derived from as_of, currency upcased
	}, Options{DefaultCurrency: "IDR"})

	if len(errs) != 0 {
		t.Fatalf("unexpected row errors: %+v", errs)
	}
	if len(parsed) != 3 {
		t.Fatalf("want 3 parsed rows, got %d", len(parsed))
	}

	if got := parsed[0].YearMonth.Format("2006-01-02"); got != "2015-01-01" {
		t.Errorf("row1 year_month = %s, want 2015-01-01", got)
	}
	if parsed[0].AsOfDate == nil || parsed[0].AsOfDate.Format("2006-01-02") != "2015-01-31" {
		t.Errorf("row1 as_of_date not parsed: %v", parsed[0].AsOfDate)
	}
	if parsed[0].Description == nil || *parsed[0].Description != "Opening" {
		t.Errorf("row1 description = %v, want Opening", parsed[0].Description)
	}

	// Row 2: blank currency -> default; blank as_of/desc -> nil.
	if parsed[1].Currency != "IDR" {
		t.Errorf("row2 currency = %q, want IDR (default)", parsed[1].Currency)
	}
	if parsed[1].AsOfDate != nil || parsed[1].Description != nil {
		t.Errorf("row2 optional fields should be nil: as_of=%v desc=%v", parsed[1].AsOfDate, parsed[1].Description)
	}

	// Row 3: month derived from as_of_date; currency upcased.
	if got := parsed[2].YearMonth.Format("2006-01-02"); got != "2015-03-01" {
		t.Errorf("row3 derived month = %s, want 2015-03-01", got)
	}
	if parsed[2].Currency != "USD" {
		t.Errorf("row3 currency = %q, want USD", parsed[2].Currency)
	}
	if !parsed[2].Amount.Equal(parsed[2].Amount) || parsed[2].Amount.String() != "11000000" {
		t.Errorf("row3 amount = %s, want 11000000", parsed[2].Amount)
	}
}

func TestParse_RowErrors(t *testing.T) {
	parsed, errs := parse(t, [][]string{
		{"2016-01", "", "notanumber", "", ""},     // row 2: bad amount
		{"2016-02", "", "", "", ""},               // row 3: missing amount
		{"", "", "5000", "", "orphan"},            // row 4: no month, no as_of
		{"2016-13", "", "5000", "", ""},           // row 5: bad month
		{"2016-05", "", "5000", "ZZ", ""},         // row 6: bad currency (fails ISO check)
		{"2016-06", "2016-06-31", "5000", "", ""}, // row 7: impossible date
	}, Options{DefaultCurrency: "IDR", ValidCurrency: func(c string) bool { return c == "IDR" || c == "USD" }})

	if len(parsed) != 0 {
		t.Fatalf("want 0 parsed rows, got %d: %+v", len(parsed), parsed)
	}
	if len(errs) != 6 {
		t.Fatalf("want 6 row errors, got %d: %+v", len(errs), errs)
	}
	// Errors carry the right (1-based, header-offset) row numbers.
	wantRows := []int{2, 3, 4, 5, 6, 7}
	for i, e := range errs {
		if e.Row != wantRows[i] {
			t.Errorf("error %d row = %d, want %d (%s)", i, e.Row, wantRows[i], e.Message)
		}
	}
}

func TestParse_DuplicateMonth(t *testing.T) {
	parsed, errs := parse(t, [][]string{
		{"2017-01", "", "100", "", ""},       // row 2: kept
		{"", "2017-01-15", "200", "", "dup"}, // row 3: derives to 2017-01 -> duplicate
	}, Options{DefaultCurrency: "IDR"})

	if len(parsed) != 1 {
		t.Fatalf("want 1 parsed row, got %d", len(parsed))
	}
	if len(errs) != 1 || errs[0].Row != 3 {
		t.Fatalf("want 1 dup error on row 3, got %+v", errs)
	}
}

func TestParse_BlankRowsSkipped(t *testing.T) {
	parsed, errs := parse(t, [][]string{
		{"2018-01", "", "100", "", ""},
		{"", "", "", "", ""}, // fully blank -> skipped, not an error
		{"2018-02", "", "200", "", ""},
	}, Options{DefaultCurrency: "IDR"})

	if len(errs) != 0 {
		t.Fatalf("blank row should not error: %+v", errs)
	}
	if len(parsed) != 2 {
		t.Fatalf("want 2 parsed rows, got %d", len(parsed))
	}
}

// TestRoundTrip proves the generated template is itself parseable and its
// example row is valid — the format we emit is the format we accept.
func TestRoundTrip(t *testing.T) {
	tpl, err := BuildTemplate(TemplateMeta{PositionName: "BCA Tabungan", DefaultCurrency: "IDR"})
	if err != nil {
		t.Fatalf("BuildTemplate: %v", err)
	}
	parsed, errs, err := Parse(bytes.NewReader(tpl), Options{DefaultCurrency: "IDR"})
	if err != nil {
		t.Fatalf("Parse(template): %v", err)
	}
	if len(errs) != 0 {
		t.Fatalf("template example row should be valid: %+v", errs)
	}
	if len(parsed) != 1 {
		t.Fatalf("template should carry exactly 1 example row, got %d", len(parsed))
	}
	if got := parsed[0].YearMonth.Format("2006-01"); got != "2015-01" {
		t.Errorf("example month = %s, want 2015-01", got)
	}
}

// ----- ShapeQuantityPrice (stock / mutual_fund / gold) --------------------

func TestParse_QuantityPrice(t *testing.T) {
	// header: year_month, as_of_date, quantity, price_per_unit, currency, description
	parsed, errs := parseShaped(t, ShapeQuantityPrice, [][]string{
		{"2015-01", "2015-01-31", "100", "8500", "IDR", "100 @ 8500"}, // amount = 850000
		{"2015-02", "", "10", "", "", ""},                             // row 3: missing price
		{"2015-03", "", "", "9000", "", ""},                           // row 4: missing quantity
		{"2015-04", "", "1.5", "200.25", "", ""},                      // fractional, amount = 300.375
	}, Options{DefaultCurrency: "IDR"})

	if len(parsed) != 2 {
		t.Fatalf("want 2 parsed rows, got %d: %+v", len(parsed), parsed)
	}
	if len(errs) != 2 || errs[0].Row != 3 || errs[1].Row != 4 {
		t.Fatalf("want errors on rows 3 and 4, got %+v", errs)
	}
	if parsed[0].Quantity == nil || parsed[0].PricePerUnit == nil {
		t.Fatalf("row1 should carry quantity + price")
	}
	if parsed[0].AccruedInterest != nil {
		t.Errorf("row1 accrued_interest should be nil, got %v", parsed[0].AccruedInterest)
	}
	if parsed[0].Amount.String() != "850000" {
		t.Errorf("row1 derived amount = %s, want 850000", parsed[0].Amount)
	}
	if !parsed[1].Amount.Equal(decimalFromString(t, "300.375")) {
		t.Errorf("row4 derived amount = %s, want 300.375", parsed[1].Amount)
	}
}

func TestRoundTrip_QuantityPrice(t *testing.T) {
	tpl, err := BuildTemplate(TemplateMeta{PositionName: "BBCA", DefaultCurrency: "IDR", Shape: ShapeQuantityPrice})
	if err != nil {
		t.Fatalf("BuildTemplate: %v", err)
	}
	parsed, errs, err := Parse(bytes.NewReader(tpl), Options{DefaultCurrency: "IDR", Shape: ShapeQuantityPrice})
	if err != nil {
		t.Fatalf("Parse(template): %v", err)
	}
	if len(errs) != 0 || len(parsed) != 1 {
		t.Fatalf("template example should be 1 valid row, got %d parsed / %+v errs", len(parsed), errs)
	}
	// Example: 100 units @ 8500 = 850000.
	if parsed[0].Quantity == nil || parsed[0].PricePerUnit == nil || parsed[0].Amount.String() != "850000" {
		t.Errorf("example qty-price row malformed: qty=%v price=%v amount=%s",
			parsed[0].Quantity, parsed[0].PricePerUnit, parsed[0].Amount)
	}
}

// ----- ShapeAccruedInterest (bond / time_deposit) -------------------------

func TestParse_AccruedInterest(t *testing.T) {
	// header: year_month, as_of_date, amount, accrued_interest, currency, description
	parsed, errs := parseShaped(t, ShapeAccruedInterest, [][]string{
		{"2015-01", "2015-01-31", "50250000", "250000", "IDR", "incl accrued"},
		{"2015-02", "", "50000000", "", "", ""}, // blank accrued -> 0
		{"2015-03", "", "", "100", "", ""},      // row 4: missing amount
	}, Options{DefaultCurrency: "IDR"})

	if len(parsed) != 2 {
		t.Fatalf("want 2 parsed rows, got %d: %+v", len(parsed), parsed)
	}
	if len(errs) != 1 || errs[0].Row != 4 {
		t.Fatalf("want 1 error on row 4 (missing amount), got %+v", errs)
	}
	if parsed[0].AccruedInterest == nil || parsed[0].AccruedInterest.String() != "250000" {
		t.Errorf("row1 accrued = %v, want 250000", parsed[0].AccruedInterest)
	}
	if parsed[0].Quantity != nil || parsed[0].PricePerUnit != nil {
		t.Errorf("row1 should not carry quantity/price")
	}
	if parsed[0].Amount.String() != "50250000" {
		t.Errorf("row1 amount = %s, want 50250000 (total incl accrued)", parsed[0].Amount)
	}
	// Blank accrued defaults to 0 (non-nil, so the bond/TD shape CHECK passes).
	if parsed[1].AccruedInterest == nil || !parsed[1].AccruedInterest.IsZero() {
		t.Errorf("row3 accrued should default to 0 (non-nil), got %v", parsed[1].AccruedInterest)
	}
}

func TestRoundTrip_AccruedInterest(t *testing.T) {
	tpl, err := BuildTemplate(TemplateMeta{PositionName: "FR0090", DefaultCurrency: "IDR", Shape: ShapeAccruedInterest})
	if err != nil {
		t.Fatalf("BuildTemplate: %v", err)
	}
	parsed, errs, err := Parse(bytes.NewReader(tpl), Options{DefaultCurrency: "IDR", Shape: ShapeAccruedInterest})
	if err != nil {
		t.Fatalf("Parse(template): %v", err)
	}
	if len(errs) != 0 || len(parsed) != 1 {
		t.Fatalf("template example should be 1 valid row, got %d parsed / %+v errs", len(parsed), errs)
	}
	if parsed[0].AccruedInterest == nil || parsed[0].Amount.String() != "50250000" {
		t.Errorf("example accrued row malformed: accrued=%v amount=%s", parsed[0].AccruedInterest, parsed[0].Amount)
	}
}

func decimalFromString(t *testing.T, s string) decimal.Decimal {
	t.Helper()
	d, err := decimal.NewFromString(s)
	if err != nil {
		t.Fatalf("decimal %q: %v", s, err)
	}
	return d
}

// --- Detail sheet + export round trip (issue #85) -------------------------

func ptrTime(t *testing.T, s string) *time.Time {
	t.Helper()
	tt, err := time.Parse("2006-01-02", s)
	if err != nil {
		t.Fatalf("time %q: %v", s, err)
	}
	return &tt
}

func ptrStr(s string) *string { return &s }

// TestBuildWorkbook_DetailRoundTrip is the build->parse symmetry for the Detail
// sheet: every field's value comes back under its key; the notes column is
// dropped on parse.
func TestBuildWorkbook_DetailRoundTrip(t *testing.T) {
	fields := []DetailField{
		{Key: "display_name", Value: "Joint savings"},
		{Key: "ownership_type", Value: "joint", Note: "sole | joint"},
		{Key: "sole_owner", Value: "", Note: "owner's email; blank when joint"},
		{Key: "tag", Value: "Emergency fund"},
		{Key: "account_type", Value: "savings", Note: "savings | current | other"},
	}
	xlsx, err := BuildWorkbook(TemplateMeta{PositionName: "Joint savings", DefaultCurrency: "IDR", Detail: fields}, nil)
	if err != nil {
		t.Fatalf("BuildWorkbook: %v", err)
	}

	got, err := ParseDetail(bytes.NewReader(xlsx))
	if err != nil {
		t.Fatalf("ParseDetail: %v", err)
	}
	for _, f := range fields {
		if got[f.Key] != f.Value {
			t.Errorf("field %q: want %q, got %q", f.Key, f.Value, got[f.Key])
		}
	}
	if len(got) != len(fields) {
		t.Errorf("field count: want %d, got %d (%v)", len(fields), len(got), got)
	}
}

// TestBuildWorkbook_ExportParseRoundTrip exports a populated workbook and reads
// both sheets back: Snapshots through the importer's Parse (the extra Detail
// sheet must not break it), and Detail through ParseDetail.
func TestBuildWorkbook_ExportParseRoundTrip(t *testing.T) {
	detail := []DetailField{
		{Key: "display_name", Value: "Main checking"},
		{Key: "ownership_type", Value: "sole"},
		{Key: "sole_owner", Value: "alice@example.com"},
	}
	snaps := []ExportSnapshot{
		{YearMonth: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), AsOfDate: ptrTime(t, "2026-01-31"), Amount: decimalFromString(t, "10000000"), Currency: "IDR", Description: ptrStr("Opening")},
		{YearMonth: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC), Amount: decimalFromString(t, "11500000"), Currency: "USD"},
	}
	xlsx, err := BuildWorkbook(TemplateMeta{PositionName: "Main checking", DefaultCurrency: "IDR", Detail: detail}, snaps)
	if err != nil {
		t.Fatalf("BuildWorkbook: %v", err)
	}

	// Snapshots sheet round-trips through the unchanged importer; Detail ignored.
	parsed, rowErrs, err := Parse(bytes.NewReader(xlsx), Options{DefaultCurrency: "IDR"})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(rowErrs) != 0 {
		t.Fatalf("unexpected row errors: %v", rowErrs)
	}
	if len(parsed) != 2 {
		t.Fatalf("want 2 parsed rows, got %d", len(parsed))
	}
	if !parsed[0].Amount.Equal(decimalFromString(t, "10000000")) || parsed[0].Currency != "IDR" {
		t.Errorf("row 0: got amount=%s currency=%s", parsed[0].Amount, parsed[0].Currency)
	}
	if parsed[0].AsOfDate == nil || !parsed[0].AsOfDate.Equal(*ptrTime(t, "2026-01-31")) {
		t.Errorf("row 0 as_of_date: got %v", parsed[0].AsOfDate)
	}
	if parsed[0].Description == nil || *parsed[0].Description != "Opening" {
		t.Errorf("row 0 description: got %v", parsed[0].Description)
	}
	if !parsed[1].Amount.Equal(decimalFromString(t, "11500000")) || parsed[1].Currency != "USD" {
		t.Errorf("row 1: got amount=%s currency=%s", parsed[1].Amount, parsed[1].Currency)
	}

	// Detail sheet round-trips through ParseDetail.
	gotDetail, err := ParseDetail(bytes.NewReader(xlsx))
	if err != nil {
		t.Fatalf("ParseDetail: %v", err)
	}
	if gotDetail["sole_owner"] != "alice@example.com" || gotDetail["ownership_type"] != "sole" {
		t.Errorf("detail mismatch: %v", gotDetail)
	}
}

// TestBuildWorkbook_ExportShapes covers the quantity/price and accrued-interest
// column layouts of snapshotRowCells: build an export in each shape, then parse
// it back through the matching Shape and check the shape-specific values land.
func TestBuildWorkbook_ExportShapes(t *testing.T) {
	t.Run("quantity/price", func(t *testing.T) {
		qty, price := decimalFromString(t, "100"), decimalFromString(t, "8500")
		snaps := []ExportSnapshot{{
			YearMonth: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
			Currency:  "IDR", Quantity: &qty, PricePerUnit: &price,
		}}
		xlsx, err := BuildWorkbook(TemplateMeta{PositionName: "Gold", DefaultCurrency: "IDR", Shape: ShapeQuantityPrice}, snaps)
		if err != nil {
			t.Fatalf("BuildWorkbook: %v", err)
		}
		parsed, rowErrs, err := Parse(bytes.NewReader(xlsx), Options{DefaultCurrency: "IDR", Shape: ShapeQuantityPrice})
		if err != nil || len(rowErrs) != 0 {
			t.Fatalf("Parse: err=%v rowErrs=%v", err, rowErrs)
		}
		if len(parsed) != 1 || parsed[0].Quantity == nil || !parsed[0].Quantity.Equal(qty) || !parsed[0].PricePerUnit.Equal(price) {
			t.Fatalf("quantity/price round trip lost: %+v", parsed)
		}
		if !parsed[0].Amount.Equal(qty.Mul(price)) {
			t.Errorf("derived amount: got %s, want %s", parsed[0].Amount, qty.Mul(price))
		}
	})

	t.Run("accrued interest", func(t *testing.T) {
		acc := decimalFromString(t, "250000")
		snaps := []ExportSnapshot{{
			YearMonth: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
			Amount:    decimalFromString(t, "50250000"), Currency: "IDR", AccruedInterest: &acc,
		}}
		xlsx, err := BuildWorkbook(TemplateMeta{PositionName: "Bond", DefaultCurrency: "IDR", Shape: ShapeAccruedInterest}, snaps)
		if err != nil {
			t.Fatalf("BuildWorkbook: %v", err)
		}
		parsed, rowErrs, err := Parse(bytes.NewReader(xlsx), Options{DefaultCurrency: "IDR", Shape: ShapeAccruedInterest})
		if err != nil || len(rowErrs) != 0 {
			t.Fatalf("Parse: err=%v rowErrs=%v", err, rowErrs)
		}
		if len(parsed) != 1 || parsed[0].AccruedInterest == nil || !parsed[0].AccruedInterest.Equal(acc) {
			t.Fatalf("accrued round trip lost: %+v", parsed)
		}
	})
}

// TestParseDetail_Errors covers the no-Detail-sheet error path and the
// keyless-row skip.
func TestParseDetail_Errors(t *testing.T) {
	t.Run("missing Detail sheet errors", func(t *testing.T) {
		// A Snapshots-only workbook has no Detail sheet to read.
		xlsx := buildXLSX(t, [][]string{header})
		if _, err := ParseDetail(bytes.NewReader(xlsx)); err == nil {
			t.Fatal("want error for missing Detail sheet, got nil")
		}
	})

	t.Run("unreadable file errors", func(t *testing.T) {
		if _, err := ParseDetail(bytes.NewReader([]byte("not an xlsx"))); err == nil {
			t.Fatal("want error for non-xlsx, got nil")
		}
	})

	t.Run("keyless rows are skipped", func(t *testing.T) {
		fields := []DetailField{
			{Key: "bank_name", Value: "TestBank"},
			{Key: "", Value: "orphan value"}, // no key -> skipped
		}
		xlsx, err := BuildWorkbook(TemplateMeta{PositionName: "X", DefaultCurrency: "IDR", Detail: fields}, nil)
		if err != nil {
			t.Fatalf("BuildWorkbook: %v", err)
		}
		got, err := ParseDetail(bytes.NewReader(xlsx))
		if err != nil {
			t.Fatalf("ParseDetail: %v", err)
		}
		if len(got) != 1 || got["bank_name"] != "TestBank" {
			t.Errorf("want only bank_name, got %v", got)
		}
	})
}
