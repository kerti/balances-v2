package pdf

import "github.com/go-pdf/fpdf"

// The Balances brand mark, drawn as fpdf vector primitives (a port of the
// frontend's glyph-light.svg — docs/brand/logo.md) rather than an embedded
// raster, so it stays crisp at any zoom with no image pipeline — the same
// rationale the charts follow (ADR-0045). The mark is a snapshot balance-scale:
// a fulcrum dot (the monthly snapshot), a beam with two hanging stacks reading
// as asset (left, indigo, taller) vs liability (right, slate) bars.
//
// These are the logo's own palette (docs/brand/logo.md). The constant indigo
// brand accent is the report's accent too (see render.go's `accent`), so the
// mark and the section headings/totals are one colour — the documented brand
// colour, not the app's teal UI primary.
var (
	brandIndigo    = [3]int{0x63, 0x66, 0xF1} // fulcrum dot + top asset bar
	brandIndigoMid = [3]int{0x81, 0x8C, 0xF8} // asset bar 2
	brandIndigoLow = [3]int{0xA5, 0xB4, 0xFC} // asset bar 3
	brandSlate     = [3]int{0x33, 0x41, 0x55} // liability bars
	brandHanger    = [3]int{0x94, 0xA3, 0xB8} // beam hangers
)

// drawGlyph renders the mark at top-left (x,y) scaled to height h (mm), on the
// glyph's native 170×163 design box. Returns the drawn width (mm).
func drawGlyph(pdf *fpdf.Fpdf, x, y, h float64) float64 {
	const designW, designH = 170.0, 163.0
	s := h / designH
	// Design-space → page-space mappers (coords are glyph-light.svg's, already
	// shifted by its inner translate(-43,-39)).
	xAt := func(u float64) float64 { return x + u*s }
	yAt := func(v float64) float64 { return y + v*s }
	d := func(v float64) float64 { return v * s }
	fill := func(c [3]int) { pdf.SetFillColor(c[0], c[1], c[2]) }

	fill(ink) // post + beam
	pdf.RoundedRect(xAt(81), yAt(32), d(8), d(42), d(4), "1234", "F")
	pdf.RoundedRect(xAt(9), yAt(69), d(152), d(10), d(5), "1234", "F")

	fill(brandHanger) // thin square hangers
	pdf.Rect(xAt(26), yAt(79), d(4), d(22), "F")
	pdf.Rect(xAt(140), yAt(79), d(4), d(22), "F")

	fill(brandIndigo) // asset stack (left) — assets outweigh liabilities
	pdf.RoundedRect(xAt(6), yAt(101), d(44), d(16), d(3), "1234", "F")
	fill(brandIndigoMid)
	pdf.RoundedRect(xAt(6), yAt(121), d(44), d(16), d(3), "1234", "F")
	fill(brandIndigoLow)
	pdf.RoundedRect(xAt(6), yAt(141), d(44), d(16), d(3), "1234", "F")

	fill(brandSlate) // liability stack (right)
	pdf.RoundedRect(xAt(120), yAt(101), d(44), d(16), d(3), "1234", "F")
	pdf.RoundedRect(xAt(120), yAt(121), d(44), d(16), d(3), "1234", "F")

	fill(brandIndigo) // fulcrum dot, on top of the post
	pdf.Circle(xAt(85), yAt(19), d(13), "F")

	return designW * s
}

// drawLogo draws the full wordmark lockup — the brand glyph + "Balances" in
// Geist Bold — at top-left (x,y), the word vertically centred against the glyph.
// textColor lets each placement pick its ink. Returns total lockup width (mm).
func drawLogo(pdf *fpdf.Fpdf, x, y, glyphH, fontPt float64, textColor [3]int) float64 {
	gw := drawGlyph(pdf, x, y, glyphH)
	gap := glyphH * 0.16
	pdf.SetFont("Geist", "B", fontPt)
	pdf.SetTextColor(textColor[0], textColor[1], textColor[2])
	tw := pdf.GetStringWidth("Balances")
	pdf.SetXY(x+gw+gap, y)
	pdf.CellFormat(tw+2, glyphH, "Balances", "", 0, "LM", false, 0, "")
	return gw + gap + tw + 2
}
