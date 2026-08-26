package pdf

import (
	"strconv"
	"strings"

	"github.com/go-pdf/fpdf"
)

// The Balances wordmark, drawn as fpdf vector paths rather than an embedded
// raster, so it stays crisp at any zoom with no image pipeline — the same
// rationale the charts follow (ADR-0045).
//
// The outline is *generated* into wordmark_gen.go by docs/brand/gen.py (run
// `make brand`), so the report's wordmark and the shipped SVG cannot drift.
// That is deliberately unlike the pre-rebrand report, which set the brand name
// as live Geist Bold text beside the mark: at that point the wordmark was a
// custom-typeface asset the report had no way to reproduce, so it approximated
// it. Generating the real outline removes the approximation.
//
// The report shows the wordmark alone, no mark, matching AppLogo (ADR-0054):
// the identity lives in the word, so setting the mark beside it would say the
// same thing twice.
var brandBrass = [3]int{0xB0, 0x89, 0x47} // the `l` post

// wordmarkBox is the wordmark's aspect ratio: box width ÷ box height, both
// including the even pad, matching the shipped SVG's viewBox.
func wordmarkBox() (w, h float64) {
	return wordmarkAdvance + 2*wordmarkPad, wordmarkTop + 2*wordmarkPad
}

// drawWordmark renders the outlined "balances" wordmark with its box's top-left
// at (x,y), scaled to box height h (mm). Returns the drawn box width (mm).
//
// Note the box is taller than the word: the tapered post rises well above the
// ascender, so the word itself sits in the lower part of the box. Callers that
// need to align the *word* with something (rather than the box) should use
// wordmarkBaseline.
func drawWordmark(pdf *fpdf.Fpdf, x, y, h float64) float64 {
	bw, bh := wordmarkBox()
	s := h / bh
	ox := x + wordmarkPad*s
	oy := y + (wordmarkTop+wordmarkPad)*s // the baseline

	for _, p := range []struct {
		d string
		c [3]int
	}{{wordmarkInkPath, ink}, {wordmarkBrassPath, brandBrass}} {
		pdf.SetFillColor(p.c[0], p.c[1], p.c[2])
		// One path per colour, filled once: the counters in b/a/e are wound
		// opposite their outer contours, so they only stay hollow if every
		// contour of a colour is filled together under the nonzero rule.
		tracePath(pdf, p.d, ox, oy, s)
		pdf.DrawPath("F")
	}
	return bw * s
}

// wordmarkBaseline is the distance from the wordmark box's top edge down to the
// word's baseline, for a box drawn at height h (mm).
func wordmarkBaseline(h float64) float64 {
	_, bh := wordmarkBox()
	return (wordmarkTop + wordmarkPad) * (h / bh)
}

// tracePath walks the generated path token stream (an SVG subset — M/L/C/Z,
// absolute, whitespace-separated, font units, y-up) into fpdf's path API,
// scaling by s and flipping y about the baseline at oy. It does not fill; the
// caller issues DrawPath so that multiple contours share one fill operation.
func tracePath(pdf *fpdf.Fpdf, d string, ox, oy, s float64) {
	tok := strings.Fields(d)
	at := func(i int) (float64, float64) {
		u, _ := strconv.ParseFloat(tok[i], 64)
		v, _ := strconv.ParseFloat(tok[i+1], 64)
		return ox + u*s, oy - v*s
	}
	for i := 0; i < len(tok); {
		switch tok[i] {
		case "M":
			px, py := at(i + 1)
			pdf.MoveTo(px, py)
			i += 3
		case "L":
			px, py := at(i + 1)
			pdf.LineTo(px, py)
			i += 3
		case "C":
			c1x, c1y := at(i + 1)
			c2x, c2y := at(i + 3)
			px, py := at(i + 5)
			pdf.CurveBezierCubicTo(c1x, c1y, c2x, c2y, px, py)
			i += 7
		case "Z":
			pdf.ClosePath()
			i++
		default:
			// Generated input, so this is unreachable in practice; skipping
			// rather than panicking keeps a malformed regen from taking the
			// whole report down with it.
			i++
		}
	}
}
