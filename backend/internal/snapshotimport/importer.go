// Package snapshotimport turns a spreadsheet template into validated snapshot
// rows. It is intentionally DB-free and HTTP-free so it can be unit-tested in
// isolation: BuildTemplate emits the .xlsx a user downloads, and Parse reads a
// filled-in one back. The repo + HTTP layers own tenancy, persistence, and
// currency policy; this package owns only the file shape and per-row parsing.
//
// The format is .xlsx (an open ISO standard, not Microsoft-specific): Google
// Sheets, LibreOffice, Numbers, and Excel all round-trip it. Cells are read as
// formatted strings, so the template ships numbers/dates as plain text and the
// instructions tell users to download as .xlsx (not CSV, which would
// re-introduce locale-specific number formatting).
//
// Snapshots come in three column shapes (per ADR-0022's value-column XOR):
//   - ShapeAmount         — a single `amount` (bank/property/vehicle/liability/
//     receivable, and time-deposit/bond via the accrued variant below).
//   - ShapeQuantityPrice  — `quantity` + `price_per_unit`; amount is derived as
//     quantity × price_per_unit (stock/mutual_fund/gold).
//   - ShapeAccruedInterest — `amount` (the total value, incl. accrued) +
//     `accrued_interest` (bond/time_deposit).
//
// Shape is the zero-value-safe ShapeAmount by default, so callers that don't
// set it (the flat amount-shape groups) need no change.
package snapshotimport

import (
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"github.com/xuri/excelize/v2"
)

// SheetName is the worksheet the importer reads data rows from.
const SheetName = "Snapshots"

// DetailSheetName is the worksheet that carries the position's own fields as a
// key/value listing (group-specific). The blank template ships keys with an
// example value + an enum/format hint; export fills the real values. The
// snapshot Parse path ignores this sheet entirely, so an exported workbook
// re-imports through the unchanged detail-page flow.
const DetailSheetName = "Detail"

// TransactionsSheetName is the row-based ledger sheet (ADR-0023): one
// investment transaction per row, columns covering the union of every
// transaction type's fields. Investment exports emit it; the other position
// groups carry no ledger and leave it off. Export-only for now — re-importing
// the Transactions sheet is create-only seeding handled in a later slice, so
// there is no Parse counterpart and the snapshot import path ignores it.
const TransactionsSheetName = "Transactions"

const instructionsSheet = "Instructions"

// transactionHeaders is the ADR-0023 column union, left to right: the shared
// fields first (type/date/currency/amount), then the trade columns, then the
// maturity columns, with description last.
var transactionHeaders = []string{
	"transaction_type", "transaction_date", "currency", "amount",
	"quantity", "price_per_unit",
	"principal_amount", "interest_amount", "principal_disposition", "interest_disposition",
	"description",
}

// detailHeaders label the Detail sheet's columns. The data is the key/value
// pair (field, value); the trailing notes column carries enum/format hints for
// the blank template and is ignored on parse, so values round-trip cleanly.
var detailHeaders = []string{"field", "value", "notes"}

// Shape selects the column layout of the template (and the columns Parse
// expects). The zero value is ShapeAmount, so existing flat-amount callers are
// unaffected.
type Shape int

const (
	// ShapeAmount: year_month, as_of_date, amount, currency, description.
	ShapeAmount Shape = iota
	// ShapeQuantityPrice: year_month, as_of_date, quantity, price_per_unit,
	// currency, description. Amount is derived (quantity × price_per_unit).
	ShapeQuantityPrice
	// ShapeAccruedInterest: year_month, as_of_date, amount, accrued_interest,
	// currency, description. Amount is the total value including accrued.
	ShapeAccruedInterest
)

// headersFor returns the column order of the template for a shape, left to
// right. ym + as_of_date always lead; currency + description always trail; the
// value columns sit in between.
func headersFor(shape Shape) []string {
	switch shape {
	case ShapeQuantityPrice:
		return []string{"year_month", "as_of_date", "quantity", "price_per_unit", "currency", "description"}
	case ShapeAccruedInterest:
		return []string{"year_month", "as_of_date", "amount", "accrued_interest", "currency", "description"}
	default:
		return []string{"year_month", "as_of_date", "amount", "currency", "description"}
	}
}

// valueColumns is the count of value columns between as_of_date and currency
// (1 for ShapeAmount, 2 for the others) — used to locate the trailing
// currency/description columns.
func valueColumns(shape Shape) int {
	if shape == ShapeAmount {
		return 1
	}
	return 2
}

// ParsedRow is one validated snapshot ready to upsert. Row is the 1-based
// spreadsheet row number, surfaced in errors so a non-technical user can find
// the offending line. Amount is always populated (derived for
// ShapeQuantityPrice); the shape-specific pointers are nil unless that shape
// set them.
type ParsedRow struct {
	Row             int
	YearMonth       time.Time
	Amount          decimal.Decimal
	Currency        string
	Quantity        *decimal.Decimal // ShapeQuantityPrice only
	PricePerUnit    *decimal.Decimal // ShapeQuantityPrice only
	AccruedInterest *decimal.Decimal // ShapeAccruedInterest only
	AsOfDate        *time.Time
	Description     *string
}

// RowError reports a single rejected row. The whole import is refused if any
// RowError is present (all-or-nothing), so the user fixes and re-uploads.
type RowError struct {
	Row     int    `json:"row"`
	Message string `json:"message"`
}

// Options configures parsing. DefaultCurrency fills rows that leave the
// currency column blank (the position's native currency). ValidCurrency, when
// non-nil, gates each resolved currency (ISO-4217 in production; nil in tests
// that don't care). Shape selects the column layout (default ShapeAmount).
type Options struct {
	DefaultCurrency string
	ValidCurrency   func(string) bool
	Shape           Shape
}

// TemplateMeta scopes a generated template to one position. PositionName is the
// position's display name (asset / liability / receivable / investment — the
// importer is position-group-agnostic, only the calling repo knows the group).
// Shape selects the column layout (default ShapeAmount). Detail, when set,
// emits a "Detail" sheet of the position's own fields; leave it nil for the
// flat groups that don't (yet) carry a Detail sheet.
type TemplateMeta struct {
	PositionName    string
	DefaultCurrency string
	Shape           Shape
	Detail          []DetailField
	// Transactions, when non-nil, adds the row-based "Transactions" ledger
	// sheet (ADR-0023 column union). Investment exports set it — to a possibly
	// empty (but non-nil) slice, which still emits the sheet with just its
	// header. The other position groups leave it nil: they have no ledger.
	Transactions []ExportTransaction
}

// DetailField is one key/value row on the Detail sheet. Value is the example
// value (blank template) or the populated value (export). Note is an optional
// enum/format hint shown in the trailing notes column; it is purely advisory
// and never read back by ParseDetail.
type DetailField struct {
	Key   string
	Value string
	Note  string
}

// ExportSnapshot is one fully-populated snapshot to write onto the Snapshots
// sheet of an export. Amount/Currency are always written; the shape-specific
// pointers are written only when the meta's Shape uses that column.
type ExportSnapshot struct {
	YearMonth       time.Time
	AsOfDate        *time.Time
	Amount          decimal.Decimal
	Currency        string
	Description     *string
	Quantity        *decimal.Decimal // ShapeQuantityPrice
	PricePerUnit    *decimal.Decimal // ShapeQuantityPrice
	AccruedInterest *decimal.Decimal // ShapeAccruedInterest
}

// ExportTransaction is one row of the Transactions ledger sheet (ADR-0023).
// Only the columns a given transaction type populates are non-nil; the rest are
// written blank so the column union stays aligned. This is export-only: there
// is no parse counterpart yet.
type ExportTransaction struct {
	TransactionType      string
	TransactionDate      time.Time
	Currency             string
	Amount               *decimal.Decimal
	Quantity             *decimal.Decimal
	PricePerUnit         *decimal.Decimal
	PrincipalAmount      *decimal.Decimal
	InterestAmount       *decimal.Decimal
	PrincipalDisposition *string
	InterestDisposition  *string
	Description          *string
}

// nonFilenameChars collapses anything outside a safe filename set to a single
// dash, so a position's display name can't smuggle quotes/newlines into a
// Content-Disposition header.
var nonFilenameChars = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// ExportFilename slugifies a position's display name into a filename-safe stem
// (no extension) for an export download's Content-Disposition. Unsafe runs
// collapse to a single dash; the fallback is returned when nothing safe
// survives (e.g. a name of only punctuation). Shared by every position group's
// export handler so the download naming stays uniform.
func ExportFilename(displayName, fallback string) string {
	slug := strings.Trim(nonFilenameChars.ReplaceAllString(displayName, "-"), "-")
	if slug == "" {
		return fallback
	}
	return slug + "-export"
}

// BuildTemplate returns the bytes of a blank .xlsx template scoped to one
// position: an optional "Detail" sheet (its fields with example values), a
// "Snapshots" sheet (header + one example row), and an "Instructions" sheet.
func BuildTemplate(meta TemplateMeta) ([]byte, error) {
	return BuildWorkbook(meta, nil)
}

// BuildWorkbook is the unified position-workbook builder behind both the blank
// template and the export. Pass snaps == nil to emit a blank template (one
// example Snapshots row); pass a non-nil slice (possibly empty) to emit an
// export with those rows and no example. When meta.Detail is set a "Detail"
// sheet is written first; export populates it with real values, the blank
// template with example values + hints.
func BuildWorkbook(meta TemplateMeta, snaps []ExportSnapshot) ([]byte, error) {
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()

	// Detail sheet first (the position's identity), when fields are provided.
	if len(meta.Detail) > 0 {
		if _, err := f.NewSheet(DetailSheetName); err != nil {
			return nil, fmt.Errorf("new detail sheet: %w", err)
		}
		if err := writeDetailSheet(f, meta.Detail); err != nil {
			return nil, err
		}
	}

	idx, err := f.NewSheet(SheetName)
	if err != nil {
		return nil, fmt.Errorf("new sheet: %w", err)
	}
	headers := headersFor(meta.Shape)
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		if err := f.SetCellStr(SheetName, cell, h); err != nil {
			return nil, fmt.Errorf("write header: %w", err)
		}
	}
	if snaps == nil {
		for i, v := range exampleRow(meta) {
			cell, _ := excelize.CoordinatesToCellName(i+1, 2)
			if err := f.SetCellStr(SheetName, cell, v); err != nil {
				return nil, fmt.Errorf("write example: %w", err)
			}
		}
	} else {
		for r, s := range snaps {
			for c, v := range snapshotRowCells(meta.Shape, s) {
				cell, _ := excelize.CoordinatesToCellName(c+1, r+2)
				if err := f.SetCellStr(SheetName, cell, v); err != nil {
					return nil, fmt.Errorf("write snapshot row: %w", err)
				}
			}
		}
	}

	// Transactions ledger (investment exports only — nil for the other groups).
	if meta.Transactions != nil {
		if _, err := f.NewSheet(TransactionsSheetName); err != nil {
			return nil, fmt.Errorf("new transactions sheet: %w", err)
		}
		if err := writeTransactionsSheet(f, meta.Transactions); err != nil {
			return nil, err
		}
	}

	if _, err := f.NewSheet(instructionsSheet); err != nil {
		return nil, fmt.Errorf("new instructions sheet: %w", err)
	}
	for i, line := range instructions(meta) {
		cell, _ := excelize.CoordinatesToCellName(1, i+1)
		if err := f.SetCellStr(instructionsSheet, cell, line); err != nil {
			return nil, fmt.Errorf("write instructions: %w", err)
		}
	}

	if err := f.DeleteSheet("Sheet1"); err != nil {
		return nil, fmt.Errorf("delete default sheet: %w", err)
	}
	f.SetActiveSheet(idx)

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, fmt.Errorf("serialise xlsx: %w", err)
	}
	return buf.Bytes(), nil
}

// writeDetailSheet lays out the field/value/notes header plus one row per field.
func writeDetailSheet(f *excelize.File, fields []DetailField) error {
	for i, h := range detailHeaders {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		if err := f.SetCellStr(DetailSheetName, cell, h); err != nil {
			return fmt.Errorf("write detail header: %w", err)
		}
	}
	for r, fld := range fields {
		for c, v := range []string{fld.Key, fld.Value, fld.Note} {
			cell, _ := excelize.CoordinatesToCellName(c+1, r+2)
			if err := f.SetCellStr(DetailSheetName, cell, v); err != nil {
				return fmt.Errorf("write detail row: %w", err)
			}
		}
	}
	return nil
}

// writeTransactionsSheet lays out the ADR-0023 column-union header plus one row
// per ledger transaction. Blank cells are written for columns a given type does
// not populate, keeping the union aligned.
func writeTransactionsSheet(f *excelize.File, txns []ExportTransaction) error {
	for i, h := range transactionHeaders {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		if err := f.SetCellStr(TransactionsSheetName, cell, h); err != nil {
			return fmt.Errorf("write transactions header: %w", err)
		}
	}
	for r, t := range txns {
		for c, v := range transactionRowCells(t) {
			cell, _ := excelize.CoordinatesToCellName(c+1, r+2)
			if err := f.SetCellStr(TransactionsSheetName, cell, v); err != nil {
				return fmt.Errorf("write transaction row: %w", err)
			}
		}
	}
	return nil
}

// transactionRowCells renders one transaction in transactionHeaders order.
func transactionRowCells(t ExportTransaction) []string {
	dec := func(p *decimal.Decimal) string {
		if p == nil {
			return ""
		}
		return p.String()
	}
	str := func(p *string) string {
		if p == nil {
			return ""
		}
		return *p
	}
	return []string{
		t.TransactionType,
		t.TransactionDate.Format("2006-01-02"),
		t.Currency,
		dec(t.Amount),
		dec(t.Quantity),
		dec(t.PricePerUnit),
		dec(t.PrincipalAmount),
		dec(t.InterestAmount),
		str(t.PrincipalDisposition),
		str(t.InterestDisposition),
		str(t.Description),
	}
}

// snapshotRowCells renders one populated snapshot in the column order of the
// shape's headers (mirror of exampleRow). Blank cells are written for absent
// optional values so columns stay aligned.
func snapshotRowCells(shape Shape, s ExportSnapshot) []string {
	ym := s.YearMonth.Format("2006-01")
	asOf := ""
	if s.AsOfDate != nil {
		asOf = s.AsOfDate.Format("2006-01-02")
	}
	desc := ""
	if s.Description != nil {
		desc = *s.Description
	}
	dec := func(p *decimal.Decimal) string {
		if p == nil {
			return ""
		}
		return p.String()
	}
	switch shape {
	case ShapeQuantityPrice:
		return []string{ym, asOf, dec(s.Quantity), dec(s.PricePerUnit), s.Currency, desc}
	case ShapeAccruedInterest:
		return []string{ym, asOf, s.Amount.String(), dec(s.AccruedInterest), s.Currency, desc}
	default:
		return []string{ym, asOf, s.Amount.String(), s.Currency, desc}
	}
}

// ParseDetail reads the "Detail" sheet back into a field->value map (the notes
// column is ignored). A returned error means the file is unreadable or has no
// Detail sheet. Blank/keyless rows are skipped. This is the inverse of the
// Detail half of BuildWorkbook; the snapshot importer never calls it.
func ParseDetail(r io.Reader) (map[string]string, error) {
	f, err := excelize.OpenReader(r)
	if err != nil {
		return nil, fmt.Errorf("not a readable .xlsx file: %w", err)
	}
	defer func() { _ = f.Close() }()

	rows, err := f.GetRows(DetailSheetName)
	if err != nil {
		return nil, fmt.Errorf("read sheet %q: %w", DetailSheetName, err)
	}
	out := make(map[string]string, len(rows))
	for i, cols := range rows {
		if i == 0 { // header
			continue
		}
		if len(cols) == 0 {
			continue
		}
		key := strings.TrimSpace(cols[0])
		if key == "" {
			continue
		}
		val := ""
		if len(cols) > 1 {
			val = strings.TrimSpace(cols[1])
		}
		out[key] = val
	}
	return out, nil
}

// exampleRow is the single illustrative data row, in the column order of the
// shape's headers.
func exampleRow(meta TemplateMeta) []string {
	switch meta.Shape {
	case ShapeQuantityPrice:
		return []string{"2015-01", "2015-01-31", "100", "8500", meta.DefaultCurrency, "100 units @ 8,500"}
	case ShapeAccruedInterest:
		return []string{"2015-01", "2015-01-31", "50250000", "250000", meta.DefaultCurrency, "Total incl. accrued"}
	default:
		return []string{"2015-01", "2015-01-31", "10000000", meta.DefaultCurrency, "Opening balance"}
	}
}

func instructions(meta TemplateMeta) []string {
	head := []string{
		fmt.Sprintf("Snapshot import — %s", meta.PositionName),
		"",
		"Fill in the \"Snapshots\" sheet, one row per monthly value.",
		"",
		"Columns:",
		"  year_month   — the month this value belongs to, as YYYY-MM (e.g. 2015-01).",
		"                 Optional if you fill statement date instead — the month is taken from it.",
		"  as_of_date   — the statement date, as YYYY-MM-DD (e.g. 2015-01-31). Optional.",
	}

	var valueLines []string
	switch meta.Shape {
	case ShapeQuantityPrice:
		valueLines = []string{
			"  quantity       — the number of units held that month, digits only. Required.",
			"  price_per_unit — the price of one unit that month, digits only. Required.",
			"                   (The total value is quantity × price_per_unit — computed for you.)",
		}
	case ShapeAccruedInterest:
		valueLines = []string{
			"  amount           — the total value that month (including accrued interest),",
			"                     digits only, no thousands separators. Required.",
			"  accrued_interest — the accrued-interest component, digits only. Optional;",
			"                     leave blank for 0.",
		}
	default:
		valueLines = []string{
			"  amount       — the balance, digits only, no thousands separators (e.g. 10000000). Required.",
		}
	}

	tail := []string{
		fmt.Sprintf("  currency     — 3-letter code (e.g. USD). Optional; defaults to %s.", meta.DefaultCurrency),
		"  description  — a free-text note. Optional.",
		"",
		"You do NOT need a row for every month. The report carries the latest value",
		"forward until the next one, so enter only the months you actually have a figure for.",
		"",
		"Re-importing is safe: a month you already imported is overwritten, not duplicated.",
		"",
		"Editing in Google Sheets / LibreOffice / Numbers / Excel is all fine —",
		"just use \"Download as .xlsx\" (NOT CSV) when you save to upload.",
	}

	lines := append(append(head, valueLines...), tail...)

	// The "Detail" sheet lists this position's own fields (field / value /
	// notes). Importing reads only the Snapshots sheet, so Detail is reference
	// material here; the notes column spells out each field's allowed values.
	if len(meta.Detail) > 0 {
		lines = append(lines,
			"",
			"The \"Detail\" sheet lists this position's own fields (field / value / notes).",
			"It is filled in on export and is here for reference — importing reads only the",
			"\"Snapshots\" sheet above, so changes to Detail are ignored on upload.",
		)
	}

	// The "Transactions" sheet is the investment's full ledger, one row per
	// transaction (ADR-0023). Like Detail it is export-only reference material
	// here — importing reads only the Snapshots sheet.
	if meta.Transactions != nil {
		lines = append(lines,
			"",
			"The \"Transactions\" sheet is this investment's full transaction ledger, one",
			"row per transaction. It is filled in on export and is here for reference —",
			"importing reads only the \"Snapshots\" sheet above, so it is ignored on upload.",
		)
	}

	return lines
}

// Parse reads the "Snapshots" sheet of an .xlsx and returns the valid rows plus
// a per-row error list. A returned error (vs RowError) means the file itself is
// unreadable or has no Snapshots sheet. Blank rows are skipped silently.
// Duplicate months within the file are reported (the later row is the error).
// The column layout it reads is selected by opts.Shape (default ShapeAmount).
func Parse(r io.Reader, opts Options) ([]ParsedRow, []RowError, error) {
	f, err := excelize.OpenReader(r)
	if err != nil {
		return nil, nil, fmt.Errorf("not a readable .xlsx file: %w", err)
	}
	defer func() { _ = f.Close() }()

	rows, err := f.GetRows(SheetName)
	if err != nil {
		return nil, nil, fmt.Errorf("read sheet %q: %w", SheetName, err)
	}
	if len(rows) == 0 {
		return nil, nil, errors.New("sheet \"Snapshots\" has no header row")
	}

	// Column positions: ym + as_of_date lead; currency + description trail the
	// value block (whose width depends on the shape).
	curIdx := 2 + valueColumns(opts.Shape)
	descIdx := curIdx + 1
	lastIdx := descIdx

	var (
		parsed []ParsedRow
		errs   []RowError
		seen   = map[string]int{} // year_month "2006-01" -> first row number
	)

	for i := 1; i < len(rows); i++ {
		rowNum := i + 1 // 1-based, header is row 1
		cols := rows[i]
		get := func(idx int) string {
			if idx < len(cols) {
				return strings.TrimSpace(cols[idx])
			}
			return ""
		}

		// Blank-row skip: every cell up to the last column is empty.
		blank := true
		for c := 0; c <= lastIdx; c++ {
			if get(c) != "" {
				blank = false
				break
			}
		}
		if blank {
			continue
		}

		ymStr, asOfStr := get(0), get(1)
		curStr, descStr := get(curIdx), get(descIdx)

		var asOf *time.Time
		if asOfStr != "" {
			t, err := parseDate(asOfStr)
			if err != nil {
				errs = append(errs, RowError{rowNum, "statement date must be YYYY-MM-DD (got " + asOfStr + ")"})
				continue
			}
			asOf = &t
		}

		var ym time.Time
		switch {
		case ymStr != "":
			t, err := parseYearMonth(ymStr)
			if err != nil {
				errs = append(errs, RowError{rowNum, "month must be YYYY-MM (got " + ymStr + ")"})
				continue
			}
			ym = t
		case asOf != nil:
			ym = firstOfMonth(*asOf)
		default:
			errs = append(errs, RowError{rowNum, "provide a month (year_month) or a statement date"})
			continue
		}

		row := ParsedRow{Row: rowNum, YearMonth: ym, AsOfDate: asOf}
		if rerr := parseValues(opts.Shape, get, rowNum, &row); rerr != nil {
			errs = append(errs, *rerr)
			continue
		}

		cur := strings.ToUpper(curStr)
		if cur == "" {
			cur = strings.ToUpper(opts.DefaultCurrency)
		}
		if opts.ValidCurrency != nil && !opts.ValidCurrency(cur) {
			errs = append(errs, RowError{rowNum, "currency must be a 3-letter ISO code (got " + cur + ")"})
			continue
		}
		row.Currency = cur

		if descStr != "" {
			d := descStr
			row.Description = &d
		}

		key := ym.Format("2006-01")
		if first, dup := seen[key]; dup {
			errs = append(errs, RowError{rowNum, "duplicate month " + key + " (already on row " + strconv.Itoa(first) + ")"})
			continue
		}
		seen[key] = rowNum

		parsed = append(parsed, row)
	}

	return parsed, errs, nil
}

// parseValues fills the value columns of row from the shape's layout, returning
// a *RowError to reject the row (nil on success). Column indices: value block
// starts at index 2 (after year_month + as_of_date).
func parseValues(shape Shape, get func(int) string, rowNum int, row *ParsedRow) *RowError {
	switch shape {
	case ShapeQuantityPrice:
		qtyStr, priceStr := get(2), get(3)
		if qtyStr == "" || priceStr == "" {
			return &RowError{rowNum, "quantity and price_per_unit are both required"}
		}
		qty, err := decimal.NewFromString(qtyStr)
		if err != nil {
			return &RowError{rowNum, "quantity must be a number with no thousands separators (got " + qtyStr + ")"}
		}
		price, err := decimal.NewFromString(priceStr)
		if err != nil {
			return &RowError{rowNum, "price_per_unit must be a number with no thousands separators (got " + priceStr + ")"}
		}
		row.Quantity = &qty
		row.PricePerUnit = &price
		row.Amount = qty.Mul(price)
		return nil

	case ShapeAccruedInterest:
		amtStr, accStr := get(2), get(3)
		if amtStr == "" {
			return &RowError{rowNum, "amount is required"}
		}
		amt, err := decimal.NewFromString(amtStr)
		if err != nil {
			return &RowError{rowNum, "amount must be a number with no thousands separators (got " + amtStr + ")"}
		}
		acc := decimal.Zero
		if accStr != "" {
			acc, err = decimal.NewFromString(accStr)
			if err != nil {
				return &RowError{rowNum, "accrued_interest must be a number with no thousands separators (got " + accStr + ")"}
			}
		}
		row.Amount = amt
		row.AccruedInterest = &acc
		return nil

	default: // ShapeAmount
		amtStr := get(2)
		if amtStr == "" {
			return &RowError{rowNum, "amount is required"}
		}
		amt, err := decimal.NewFromString(amtStr)
		if err != nil {
			return &RowError{rowNum, "amount must be a number with no thousands separators (got " + amtStr + ")"}
		}
		row.Amount = amt
		return nil
	}
}

// parseYearMonth accepts "YYYY-MM" or a full date, normalising to first-of-month
// (mirrors the backend handler's parseYearMonth so the importer and the
// single-row create path agree on the month convention).
func parseYearMonth(s string) (time.Time, error) {
	if t, err := time.Parse("2006-01", s); err == nil {
		return t, nil
	}
	t, err := parseDate(s)
	if err != nil {
		return time.Time{}, err
	}
	return firstOfMonth(t), nil
}

// parseDate accepts ISO-ish dates only. MM/DD vs DD/MM is deliberately not
// guessed — the template asks for YYYY-MM-DD, and an ambiguous format errors
// loudly rather than silently picking the wrong day.
func parseDate(s string) (time.Time, error) {
	for _, layout := range []string{"2006-01-02", "2006/01/02", "2006-1-2"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognised date %q (use YYYY-MM-DD)", s)
}

func firstOfMonth(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
}
