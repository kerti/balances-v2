package pdf

import (
	"math"

	"github.com/go-pdf/fpdf"
)

// Hand-drawn vector charts (ADR-0045) — donut composition + net-worth trend
// line, drawn straight into the PDF with fpdf primitives (crisp at any zoom,
// theme-independent, no image pipeline). The geometry ports the frontend's
// pieChartMath / lineChartMath.

// slice is one donut segment.
type slice struct {
	Label string
	Value float64
}

// chartPalette is a small categorical palette (RGB), reused across donuts. Colours
// are assigned by slice order; a slice past the end wraps.
var chartPalette = [][3]int{
	{0x25, 0x63, 0xEB}, // blue
	{0x10, 0xB9, 0x81}, // emerald
	{0xF5, 0x9E, 0x0B}, // amber
	{0x8B, 0x5C, 0xF6}, // violet
	{0xEF, 0x44, 0x44}, // red
	{0x06, 0xB6, 0xD4}, // cyan
	{0x64, 0x74, 0x8B}, // slate
	{0xEC, 0x48, 0x99}, // pink
}

func paletteAt(i int) [3]int { return chartPalette[i%len(chartPalette)] }

// drawDonut renders a donut at centre (cx,cy) with the given outer/inner radii
// (mm), one segment per slice sized by value share. Zero-total is a no-op.
func drawDonut(pdf *fpdf.Fpdf, cx, cy, rOuter, rInner float64, slices []slice) {
	var total float64
	for _, s := range slices {
		if s.Value > 0 {
			total += s.Value
		}
	}
	if total <= 0 {
		return
	}
	const steps = 64 // arc smoothness per full circle
	angle := -math.Pi / 2
	for i, s := range slices {
		if s.Value <= 0 {
			continue
		}
		sweep := (s.Value / total) * 2 * math.Pi
		end := angle + sweep
		col := paletteAt(i)
		pdf.SetFillColor(col[0], col[1], col[2])

		// Annular sector as a filled polygon: outer arc forward, inner arc back.
		n := max(int(math.Ceil(float64(steps)*(sweep/(2*math.Pi)))), 1)
		pts := make([]fpdf.PointType, 0, 2*(n+1))
		for k := 0; k <= n; k++ {
			a := angle + (end-angle)*float64(k)/float64(n)
			pts = append(pts, fpdf.PointType{X: cx + rOuter*math.Cos(a), Y: cy + rOuter*math.Sin(a)})
		}
		for k := n; k >= 0; k-- {
			a := angle + (end-angle)*float64(k)/float64(n)
			pts = append(pts, fpdf.PointType{X: cx + rInner*math.Cos(a), Y: cy + rInner*math.Sin(a)})
		}
		pdf.Polygon(pts, "F")
		angle = end
	}
}

// drawLegend draws a colour-swatch + label list starting at (x,y), one row per
// slice, using the same palette order as drawDonut. Values are formatted as a
// percentage of the total. Returns the y after the last row.
func drawLegend(pdf *fpdf.Fpdf, x, y float64, slices []slice) float64 {
	var total float64
	for _, s := range slices {
		if s.Value > 0 {
			total += s.Value
		}
	}
	pdf.SetFont("Geist", "", 7)
	const rowH = 4.2
	for i, s := range slices {
		if s.Value <= 0 {
			continue
		}
		col := paletteAt(i)
		pdf.SetFillColor(col[0], col[1], col[2])
		pdf.Rect(x, y+0.8, 2.4, 2.4, "F")
		pct := 0.0
		if total > 0 {
			pct = s.Value / total * 100
		}
		pdf.SetTextColor(ink[0], ink[1], ink[2])
		pdf.SetXY(x+3.6, y)
		pdf.CellFormat(0, rowH, sprintfPct(s.Label, pct), "", 0, "L", false, 0, "")
		y += rowH
	}
	return y
}

// drawTrend renders a net-worth trend line inside the box (x,y,w,h). Values are
// scaled to [min,max]; a light baseline + the polyline + end markers are drawn.
func drawTrend(pdf *fpdf.Fpdf, x, y, w, h float64, pts []TrendPoint) {
	if len(pts) < 2 {
		return
	}
	mn, mx := pts[0].NetWorth, pts[0].NetWorth
	for _, p := range pts {
		mn = math.Min(mn, p.NetWorth)
		mx = math.Max(mx, p.NetWorth)
	}
	span := mx - mn
	if span == 0 {
		span = 1
	}
	// baseline
	pdf.SetDrawColor(0xE2, 0xE8, 0xF0)
	pdf.SetLineWidth(0.2)
	pdf.Line(x, y+h, x+w, y+h)

	sx := func(i int) float64 { return x + w*float64(i)/float64(len(pts)-1) }
	sy := func(v float64) float64 { return y + h - h*(v-mn)/span }

	pdf.SetDrawColor(chartPalette[0][0], chartPalette[0][1], chartPalette[0][2])
	pdf.SetLineWidth(0.5)
	for i := 0; i < len(pts)-1; i++ {
		pdf.Line(sx(i), sy(pts[i].NetWorth), sx(i+1), sy(pts[i+1].NetWorth))
	}
	// end markers
	pdf.SetFillColor(chartPalette[0][0], chartPalette[0][1], chartPalette[0][2])
	pdf.Circle(sx(0), sy(pts[0].NetWorth), 0.7, "F")
	pdf.Circle(sx(len(pts)-1), sy(pts[len(pts)-1].NetWorth), 0.9, "F")

	// first/last month labels
	pdf.SetFont("Geist", "", 6.5)
	pdf.SetTextColor(muted[0], muted[1], muted[2])
	pdf.SetXY(x, y+h+0.5)
	pdf.CellFormat(w/2, 3, pts[0].Label, "", 0, "L", false, 0, "")
	pdf.CellFormat(w/2, 3, pts[len(pts)-1].Label, "", 0, "R", false, 0, "")
}
