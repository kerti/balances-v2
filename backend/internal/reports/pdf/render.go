// Package pdf renders the monthly financial report as a native PDF, server-side
// (ADR-0045, superseding ADR-0044's client-side @react-pdf/renderer path). It
// draws a portrait, itemized financial statement directly with go-pdf/fpdf —
// pure Go, no browser, satisfying the lean self-hostable image (ADR-0030/0037).
//
// Geist (Regular + Bold) is embedded and registered as a UTF-8 font so the
// report's typography matches the web UI; the Latin subset covers en-GB and
// id-ID copy. Copy comes from reportcopy.go; money formatting from the moneyfmt
// package (golden-parity with the frontend's Intl.NumberFormat).
package pdf

import (
	"bytes"
	_ "embed"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/go-pdf/fpdf"
	"github.com/shopspring/decimal"

	"github.com/kerti/balances-v2/backend/internal/moneyfmt"
)

//go:embed fonts/Geist-Regular.ttf
var geistRegular []byte

//go:embed fonts/Geist-Bold.ttf
var geistBold []byte

// Palette: ink/muted match the app's primary + muted text; accent is the brand
// indigo (docs/brand/logo.md — the documented brand colour, same as the
// wordmark, so the report's headings/totals match its own mark rather than the
// app's teal UI primary); gain/loss are the up/down colours for the net-worth
// delta and negative amounts; rule is the hairline colour for table borders.
var (
	ink    = [3]int{0x0F, 0x17, 0x2A}
	muted  = [3]int{0x64, 0x74, 0x8B}
	accent = brandIndigo // #6366F1 — kept identical to the wordmark's accent
	gain   = [3]int{0x05, 0x96, 0x69}
	loss   = [3]int{0xDC, 0x26, 0x26}
	rule   = [3]int{0xCB, 0xD5, 0xE1}
)

const (
	marginL = 18.0
	marginT = 16.0
	marginR = 18.0
	marginB = 16.0
	valueW  = 46.0 // right-aligned amount column width (mm)
	lineH   = 5.0
)

type doc struct {
	pdf *fpdf.Fpdf
	c   reportCopy
	in  Input
	x0  float64 // left content edge
	w   float64 // content width
}

// Render produces the report PDF bytes for one month.
func Render(in Input) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.AddUTF8FontFromBytes("Geist", "", geistRegular)
	pdf.AddUTF8FontFromBytes("Geist", "B", geistBold)
	pdf.SetMargins(marginL, marginT, marginR)
	pdf.SetAutoPageBreak(true, marginB)

	d := &doc{pdf: pdf, c: copyFor(in.Locale), in: in, x0: marginL, w: 210 - marginL - marginR}
	pdf.AliasNbPages("{nb}")
	pdf.SetFooterFunc(d.footer)

	pdf.AddPage()
	pdf.SetDrawColor(rule[0], rule[1], rule[2])
	pdf.SetLineWidth(0.2)
	d.header()
	d.headline()
	d.statistics()
	d.assets()
	d.liabilities()
	d.investments()
	d.receivables()
	d.cashFlow()
	d.fxRates()
	d.charts()
	d.staleFootnote()

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("render report pdf: %w", err)
	}
	return buf.Bytes(), nil
}

// ---- primitives -------------------------------------------------------------

func (d *doc) money(amount string) string {
	return moneyfmt.FormatCurrency(amount, d.in.ReportingCurrency, d.in.Locale)
}

func (d *doc) fmtMonthYear(t time.Time) string {
	names := monthNamesEN
	if len(d.in.Locale) >= 2 && d.in.Locale[:2] == "id" {
		names = monthNamesIDCat
	}
	return fmt.Sprintf("%s %d", names[int(t.Month())], t.Year())
}

// footer draws the branding + app version + page number on every page
// (ADR-0045, #414). The version suffix (" · <tag>") trails the wordmark in
// muted ink; the page number is centred on the same band, right-aligned.
// "{nb}" is the total-pages alias fpdf substitutes at output time.
func (d *doc) footer() {
	d.pdf.SetY(-12)
	d.pdf.SetDrawColor(rule[0], rule[1], rule[2])
	d.pdf.SetLineWidth(0.2)
	yLine := d.pdf.GetY()
	d.pdf.Line(d.x0, yLine, d.x0+d.w, yLine)

	// Full-colour brand lockup on the left, then the version suffix trailing it;
	// page number vertically centred on the same band on the right.
	const glyphH = 4.0
	logoY := yLine + 2.5
	lockupW := drawLogo(d.pdf, d.x0, logoY, glyphH, 9, ink)
	d.pdf.SetFont("Geist", "", 7.5)
	d.pdf.SetTextColor(muted[0], muted[1], muted[2])
	if suffix := versionSuffix(d.in.Version); suffix != "" {
		d.pdf.SetXY(d.x0+lockupW, logoY)
		d.pdf.CellFormat(d.pdf.GetStringWidth(suffix)+2, glyphH, suffix, "", 0, "LM", false, 0, "")
	}
	d.pdf.SetXY(d.x0, logoY)
	d.pdf.CellFormat(d.w, glyphH, fmt.Sprintf(d.c.footerPage, d.pdf.PageNo(), "{nb}"), "", 0, "RM", false, 0, "")
}

// versionSuffix formats the build tag as a footer suffix trailing the wordmark
// (" · v0.8.0-alpha.1"), or "" for an unset version so the slot reads as just
// the wordmark rather than a dangling separator.
func versionSuffix(v string) string {
	if v == "" {
		return ""
	}
	return " · " + v
}

type lineOpt struct {
	bold      bool
	mutedText bool
	accent    bool // brand indigo — section/net totals
	negative  bool // red — deficits and losses (wins over accent)
	size      float64
	topBorder bool
}

// line draws a label (left, indented) + value (right-aligned amount column).
func (d *doc) line(label, value string, indent float64, o lineOpt) {
	if o.size == 0 {
		o.size = 9
	}
	style := ""
	if o.bold {
		style = "B"
	}
	border := ""
	if o.topBorder {
		border = "T"
	}
	d.pdf.SetFont("Geist", style, o.size)
	col := ink
	switch {
	case o.negative:
		col = loss
	case o.accent:
		col = accent
	case o.mutedText:
		col = muted
	}
	d.pdf.SetTextColor(col[0], col[1], col[2])
	d.pdf.SetX(d.x0 + indent)
	d.pdf.CellFormat(d.w-valueW-indent, lineH, label, border, 0, "L", false, 0, "")
	d.pdf.CellFormat(valueW, lineH, value, border, 1, "R", false, 0, "")
}

// keepTogether forces a page break if `needed` mm won't fit before the bottom
// margin — used to stop a heading being orphaned at a page bottom with its rows
// pushed to the next page (fpdf has no native keep-with-next).
func (d *doc) keepTogether(needed float64) {
	const pageH = 297.0 // A4 portrait
	if d.pdf.GetY()+needed > pageH-marginB {
		d.pdf.AddPage()
	}
}

func (d *doc) sectionTitle(title string) {
	d.keepTogether(22) // title + a couple of rows
	d.pdf.Ln(3)
	d.pdf.SetFont("Geist", "B", 12)
	d.pdf.SetTextColor(accent[0], accent[1], accent[2])
	d.pdf.SetDrawColor(accent[0], accent[1], accent[2])
	d.pdf.SetX(d.x0)
	d.pdf.CellFormat(d.w, 7, title, "B", 1, "L", false, 0, "")
	d.pdf.SetDrawColor(rule[0], rule[1], rule[2])
	d.pdf.Ln(1)
}

// subGroup draws a light level-1 sub-heading (e.g. Current Assets).
func (d *doc) subGroup(label string) {
	d.line(label, "", 0, lineOpt{mutedText: true, size: 8.5})
}

// subtypeHeader draws a bold level-2 heading (e.g. Bank Accounts).
func (d *doc) subtypeHeader(label string) {
	d.keepTogether(14) // heading + first row
	d.line(label, "", 2, lineOpt{bold: true, size: 9.5})
}

func (d *doc) position(p Position, indent float64) {
	name := p.Name
	if p.Stale {
		name += " *"
	}
	d.line(name, d.money(p.Amount), indent, lineOpt{})
	if p.NativeCurrency != "" && p.NativeCurrency != d.in.ReportingCurrency {
		d.line(moneyfmt.FormatCurrency(p.NativeAmount, p.NativeCurrency, d.in.Locale), "", indent+4,
			lineOpt{mutedText: true, size: 7.5})
	}
}

// ---- sections ---------------------------------------------------------------

func (d *doc) header() {
	y0 := d.pdf.GetY()
	const glyphH = 9.5
	drawLogo(d.pdf, d.x0, y0, glyphH, 22, ink) // full-colour brand lockup
	d.pdf.SetXY(d.x0, y0+glyphH+2.5)
	d.pdf.SetTextColor(ink[0], ink[1], ink[2])
	d.pdf.SetFont("Geist", "B", 13)
	d.pdf.CellFormat(0, 8, d.c.title+" — "+d.fmtMonthYear(d.in.YearMonth), "", 1, "L", false, 0, "")
	d.pdf.SetTextColor(muted[0], muted[1], muted[2])
	d.pdf.SetFont("Geist", "", 9)
	d.pdf.CellFormat(0, 6, fmt.Sprintf(d.c.subtitle, d.fmtMonthYear(d.in.YearMonth)), "", 1, "L", false, 0, "")
}

func (d *doc) headline() {
	d.pdf.Ln(5)
	d.pdf.SetTextColor(muted[0], muted[1], muted[2])
	d.pdf.SetFont("Geist", "", 10)
	d.pdf.CellFormat(0, 6, d.c.netWorth, "", 1, "L", false, 0, "")

	// Net-worth figure on the left; MoM + YoY comparisons stacked, right-aligned,
	// on the same band to the right of the figure.
	y0 := d.pdf.GetY()
	d.pdf.SetTextColor(ink[0], ink[1], ink[2])
	d.pdf.SetFont("Geist", "B", 24)
	d.pdf.SetXY(d.x0, y0)
	d.pdf.CellFormat(0, 12, d.money(d.in.NetWorth), "", 0, "L", false, 0, "")

	yy := y0 + 1.5
	for _, dl := range []*Delta{d.in.Delta, d.in.YoY} {
		if dl == nil {
			continue
		}
		text, col := d.deltaText(dl)
		d.pdf.SetFont("Geist", "", 8.5)
		d.pdf.SetTextColor(col[0], col[1], col[2])
		d.pdf.SetXY(d.x0, yy)
		d.pdf.CellFormat(d.w, 4.5, text, "", 0, "R", false, 0, "")
		yy += 5
	}
	d.pdf.SetXY(d.x0, y0+13)
}

// deltaText formats a net-worth change ("+Rp 110.000.000 (+1,8%) vs May 2026")
// and its colour — green (gain) or red (loss). The compared month naming (prior
// month for MoM, year-ago month for YoY) makes each line self-describing.
func (d *doc) deltaText(dl *Delta) (string, [3]int) {
	amt := decAmt(dl.Amount)
	col, sign := gain, "+"
	if amt.IsNegative() {
		col, sign = loss, "-"
	}
	pctSign := "+"
	if dl.Percent < 0 {
		pctSign = "-"
	}
	text := fmt.Sprintf("%s%s (%s%s%%) %s",
		sign, d.money(amt.Abs().String()),
		pctSign, moneyfmt.FormatNumber(fmt.Sprintf("%.1f", math.Abs(dl.Percent)), d.in.Locale),
		fmt.Sprintf(d.c.deltaVs, d.fmtMonthYear(dl.Prev)))
	return text, col
}

// statistics renders the financial-health panel (#412, ADR-0048) directly under
// the headline: four household-health ratios, each a bold value plus one short
// plain-language explanation. No colour-coding or targets in v1 — the values
// stay neutral ink. A ratio whose inputs are unavailable shows an em-dash with a
// short note, keeping the reserved slot intact.
func (d *doc) statistics() {
	d.sectionTitle(d.c.statistics)
	s := d.in.Stats
	d.statRow(d.c.statRows[0], d.c.statDescs[0], d.pctValue(s.CashFlow), s.CashFlow.Defined)
	d.statRow(d.c.statRows[1], d.c.statDescs[1], d.pctValue(s.PassiveIncome), s.PassiveIncome.Defined)
	d.statRow(d.c.statRows[2], d.c.statDescs[2], d.pctValue(s.InstantLiquidity), s.InstantLiquidity.Defined)
	d.statRow(d.c.statRows[3], d.c.statDescs[3], d.resilienceValue(), s.Resilience.Defined)
}

// statRow draws one ratio: a bold label + right-aligned value, then a muted,
// wrapped explanation beneath. When the ratio is undefined the value collapses
// to an em-dash and the explanation to the shared "not enough history" note.
func (d *doc) statRow(label, desc, value string, defined bool) {
	d.keepTogether(16)
	opt := lineOpt{bold: true}
	if !defined {
		value = "—"
		opt.mutedText = true
		desc = d.c.statUndefined
	}
	d.line(label, value, 2, opt)
	d.pdf.SetFont("Geist", "", 7.5)
	d.pdf.SetTextColor(muted[0], muted[1], muted[2])
	d.pdf.SetX(d.x0 + 2)
	d.pdf.MultiCell(d.w-2, 3.6, desc, "", "L", false)
	d.pdf.Ln(1)
}

// pctValue formats a ratio as a locale-aware percentage to one decimal place;
// the sign is carried naturally, so a deficit or a market loss prints negative.
func (d *doc) pctValue(r Ratio) string {
	return moneyfmt.FormatNumber(fmt.Sprintf("%.1f", r.Percent), d.in.Locale) + "%"
}

// resilienceValue formats the Fund Resilience runway as a month count (singular
// aware) or the localized "indefinite" word when the pool never depletes.
func (d *doc) resilienceValue() string {
	r := d.in.Stats.Resilience
	if r.Indefinite {
		return d.c.statIndefinite
	}
	unit := d.c.statMonthsUnit
	if r.Months == 1 {
		unit = d.c.statMonthUnit
	}
	return fmt.Sprintf(unit, moneyfmt.FormatNumber(fmt.Sprintf("%d", r.Months), d.in.Locale))
}

func (d *doc) assets() {
	banks := d.positions("asset", "bank_account")
	props := d.positions("asset", "property")
	vehicles := d.positions("asset", "vehicle")
	if len(banks)+len(props)+len(vehicles) == 0 {
		return
	}
	d.sectionTitle(d.c.assets)

	if len(banks) > 0 {
		d.subGroup(d.c.currentAssets)
		d.subtypeHeader(subtypeLabel(d.in.Locale, "bank_account"))
		for _, owner := range ownersOf(banks) {
			ps := ownerPositions(banks, owner)
			d.line(owner, "", 5, lineOpt{mutedText: true, size: 8.5})
			for _, p := range ps {
				d.position(p, 8)
			}
			d.line(d.c.total(owner), d.money(sum(ps).String()), 5,
				lineOpt{mutedText: true, size: 8, topBorder: true})
		}
		d.line(d.c.total(subtypeLabel(d.in.Locale, "bank_account")), d.money(sum(banks).String()), 2,
			lineOpt{bold: true, topBorder: true})
	}

	if len(props)+len(vehicles) > 0 {
		d.subGroup(d.c.nonCurrentAssets)
		d.itemizedSubtype("property", props)
		d.itemizedSubtype("vehicle", vehicles)
	}

	d.line(d.c.total(d.c.assets), d.money(sum(append(append(banks, props...), vehicles...)).String()), 0,
		lineOpt{bold: true, accent: true, size: 10.5, topBorder: true})
}

func (d *doc) liabilities() {
	inst := d.positions("liability", "institutional")
	pers := d.positions("liability", "personal")
	if len(inst)+len(pers) == 0 {
		return
	}
	d.sectionTitle(d.c.liabilities)
	d.itemizedSubtype("institutional", inst)
	d.itemizedSubtype("personal", pers)
	d.line(d.c.total(d.c.liabilities), d.money(sum(append(inst, pers...)).String()), 0,
		lineOpt{bold: true, accent: true, size: 10.5, topBorder: true})
}

func (d *doc) investments() {
	order := []string{"mutual_fund", "bond", "gold", "stock", "time_deposit"}
	var all []Position
	present := false
	for _, st := range order {
		if len(d.positions("investment", st)) > 0 {
			present = true
		}
	}
	if !present {
		return
	}
	d.sectionTitle(d.c.investments)
	for _, st := range order {
		ps := d.positions("investment", st)
		d.itemizedSubtype(st, ps)
		all = append(all, ps...)
	}
	d.line(d.c.total(d.c.investments), d.money(sum(all).String()), 0,
		lineOpt{bold: true, accent: true, size: 10.5, topBorder: true})
}

func (d *doc) receivables() {
	rs := d.positions("receivable", "")
	if len(rs) == 0 {
		return
	}
	d.sectionTitle(d.c.receivables)
	for _, p := range rs {
		d.position(p, 4)
	}
	d.line(d.c.total(d.c.receivables), d.money(sum(rs).String()), 0,
		lineOpt{bold: true, accent: true, size: 10.5, topBorder: true})
}

// itemizedSubtype draws a subtype heading, its positions, and a subtotal — the
// shared shape for property/vehicle/liability/investment groups. No-op if empty.
func (d *doc) itemizedSubtype(subtype string, ps []Position) {
	if len(ps) == 0 {
		return
	}
	label := subtypeLabel(d.in.Locale, subtype)
	d.subtypeHeader(label)
	for _, p := range ps {
		d.position(p, 5)
	}
	d.line(d.c.total(label), d.money(sum(ps).String()), 2, lineOpt{bold: true, topBorder: true})
}

func (d *doc) cashFlow() {
	d.sectionTitle(d.c.cashFlow)
	if d.in.CashFlow == nil {
		d.pdf.SetTextColor(muted[0], muted[1], muted[2])
		d.pdf.SetFont("Geist", "", 9)
		d.pdf.SetX(d.x0)
		d.pdf.CellFormat(d.w, lineH, d.c.baseline, "", 1, "L", false, 0, "")
		return
	}
	cf := d.in.CashFlow
	d.subtypeHeader(d.c.cashIn)
	for _, m := range cf.Members {
		d.line(m.Label, d.money(m.Amount), 5, lineOpt{})
	}
	d.line(d.c.income, d.money(cf.Income), 2, lineOpt{bold: true, topBorder: true})
	d.subtypeHeader(d.c.cashOut)
	d.line(d.c.expenses, d.money(cf.Expenses), 5, lineOpt{})
	d.line(d.c.netCashFlow, d.money(cf.Net), 0,
		lineOpt{bold: true, accent: true, negative: decAmt(cf.Net).IsNegative(), size: 10.5, topBorder: true})
}

func (d *doc) fxRates() {
	if len(d.in.FxRates) == 0 {
		return
	}
	d.sectionTitle(d.c.fxTitle)
	d.pdf.SetFont("Geist", "", 8.5)
	d.pdf.SetTextColor(muted[0], muted[1], muted[2])
	for _, r := range d.in.FxRates {
		d.pdf.SetX(d.x0)
		line := fmt.Sprintf(d.c.fxLine, r.Currency, moneyfmt.FormatNumber(r.Rate, d.in.Locale), d.in.ReportingCurrency)
		d.pdf.CellFormat(d.w, 4.5, line, "", 1, "L", false, 0, "")
	}
}

func (d *doc) charts() {
	assetsComp := d.composition("asset", []string{"bank_account", "property", "vehicle"})
	invComp := d.composition("investment", []string{"mutual_fund", "bond", "gold", "stock", "time_deposit"})
	liabComp := d.composition("liability", []string{"institutional", "personal"})
	if len(assetsComp)+len(invComp)+len(liabComp) == 0 && len(d.in.Trend) < 2 {
		return
	}
	d.keepTogether(62) // keep the section title with the donut row
	d.sectionTitle(d.c.charts)

	donuts := []struct {
		title  string
		group  string
		slices []slice
	}{
		{d.c.chartAssets, "asset", assetsComp},
		{d.c.chartInvestments, "investment", invComp},
		{d.c.chartLiabilities, "liability", liabComp},
	}
	colW := d.w / 3
	top := d.pdf.GetY()
	maxY := top
	for j, dn := range donuts {
		if len(dn.slices) == 0 {
			continue
		}
		colX := d.x0 + colW*float64(j)
		cx := colX + colW/2
		d.pdf.SetXY(colX, top)
		d.pdf.SetFont("Geist", "B", 8.5)
		d.pdf.SetTextColor(ink[0], ink[1], ink[2])
		d.pdf.CellFormat(colW, 5, dn.title, "", 0, "C", false, 0, "")
		drawDonut(d.pdf, cx, top+19, 13, 6.5, dn.slices)
		// group total, centred below the donut
		d.pdf.SetXY(colX, top+32)
		d.pdf.SetFont("Geist", "B", 7.5)
		d.pdf.SetTextColor(accent[0], accent[1], accent[2])
		d.pdf.CellFormat(colW, 4, d.money(sum(d.positions(dn.group, "")).String()), "", 0, "C", false, 0, "")
		endY := drawLegend(d.pdf, colX+4, top+38, dn.slices)
		if endY > maxY {
			maxY = endY
		}
	}
	d.pdf.SetY(maxY + 4)

	if len(d.in.Trend) >= 2 {
		d.keepTogether(38) // keep the trend title with the line
		d.pdf.SetFont("Geist", "B", 8.5)
		d.pdf.SetTextColor(ink[0], ink[1], ink[2])
		d.pdf.SetX(d.x0)
		d.pdf.CellFormat(d.w, 5, d.c.chartTrend, "", 1, "L", false, 0, "")
		drawTrend(d.pdf, d.x0, d.pdf.GetY()+4, d.w, 26, d.in.Trend, d.money(d.in.NetWorth))
		d.pdf.SetY(d.pdf.GetY() + 34)
	}
}

func (d *doc) staleFootnote() {
	for _, p := range d.in.Positions {
		if p.Stale {
			d.pdf.Ln(3)
			d.pdf.SetFont("Geist", "", 7.5)
			d.pdf.SetTextColor(muted[0], muted[1], muted[2])
			d.pdf.SetX(d.x0)
			d.pdf.CellFormat(d.w, 4, d.c.staleNote, "", 1, "L", false, 0, "")
			return
		}
	}
}

// ---- helpers ----------------------------------------------------------------

// positions returns this month's positions in a group (and subtype, if given),
// sorted by reporting amount descending.
func (d *doc) positions(group, subtype string) []Position {
	var out []Position
	for _, p := range d.in.Positions {
		if p.Group == group && (subtype == "" || p.Subtype == subtype) {
			out = append(out, p)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		return decAmt(out[i].Amount).GreaterThan(decAmt(out[j].Amount))
	})
	return out
}

func (d *doc) composition(group string, order []string) []slice {
	var out []slice
	for _, st := range order {
		ps := d.positions(group, st)
		if len(ps) == 0 {
			continue
		}
		v, _ := sum(ps).Float64()
		if v <= 0 {
			continue
		}
		out = append(out, slice{Label: subtypeLabel(d.in.Locale, st), Value: v})
	}
	return out
}

func decAmt(s string) decimal.Decimal {
	v, err := decimal.NewFromString(s)
	if err != nil {
		return decimal.Zero
	}
	return v
}

func sum(ps []Position) decimal.Decimal {
	total := decimal.Zero
	for _, p := range ps {
		total = total.Add(decAmt(p.Amount))
	}
	return total
}

// ownersOf returns the distinct owner labels among positions, ordered by their
// combined amount descending (stable for equal totals).
func ownersOf(ps []Position) []string {
	totals := map[string]decimal.Decimal{}
	var order []string
	for _, p := range ps {
		if _, seen := totals[p.OwnerLabel]; !seen {
			order = append(order, p.OwnerLabel)
		}
		totals[p.OwnerLabel] = totals[p.OwnerLabel].Add(decAmt(p.Amount))
	}
	sort.SliceStable(order, func(i, j int) bool {
		return totals[order[i]].GreaterThan(totals[order[j]])
	})
	return order
}

func ownerPositions(ps []Position, owner string) []Position {
	var out []Position
	for _, p := range ps {
		if p.OwnerLabel == owner {
			out = append(out, p)
		}
	}
	return out
}

func sprintfPct(label string, pct float64) string {
	return fmt.Sprintf("%s  %.1f%%", label, pct)
}
