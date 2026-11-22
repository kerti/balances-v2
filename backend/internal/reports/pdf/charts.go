package pdf

import (
	"math"

	"codeberg.org/go-pdf/fpdf"
)

// Hand-drawn vector charts (ADR-0045) — donut composition + net-worth trend
// line, drawn straight into the PDF with fpdf primitives (crisp at any zoom,
// theme-independent, no image pipeline). The geometry ports the frontend's
// pieChartMath / lineChartMath.

// slice is one donut segment. Color is the subtype's category colour.
type slice struct {
	Label string
	Value float64
	Color [3]int
}

// categoryColors mirrors the UI's --cat-* light-theme tokens in
// frontend/src/index.css (ADR-0054 amendment, #836): no category uses red or
// green, which mean direction. The report prints on white, so the light values
// apply.
var categoryColors = map[string][3]int{
	"bank_account":  {0x40, 0xA7, 0xF9},
	"property":      {0xB3, 0x3A, 0xA0},
	"vehicle":       {0xD3, 0x83, 0x01},
	"personal":      {0x86, 0x71, 0xFC},
	"institutional": {0xE5, 0x8E, 0x13},
	"stock":         {0x24, 0xB3, 0xCD},
	"mutual_fund":   {0x6E, 0x41, 0xC6},
	"bond":          {0x18, 0x7F, 0xDA},
	"time_deposit":  {0xBC, 0x51, 0xAF},
	"gold":          {0xB9, 0xA8, 0x43},
}

// drawDonut renders a donut at centre (cx,cy) with the given outer/inner radii
// (mm), one segment per slice sized by value share. Zero-total is a no-op.
func drawDonut(pdf *pdfDoc, cx, cy, rOuter, rInner float64, slices []slice) {
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
	for _, s := range slices {
		if s.Value <= 0 {
			continue
		}
		sweep := (s.Value / total) * 2 * math.Pi
		end := angle + sweep
		pdf.SetFillColor(s.Color[0], s.Color[1], s.Color[2])

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
// slice, using the same colours as drawDonut. Values are formatted as a
// percentage of the total. Returns the y after the last row.
func drawLegend(pdf *pdfDoc, x, y float64, slices []slice) float64 {
	var total float64
	for _, s := range slices {
		if s.Value > 0 {
			total += s.Value
		}
	}
	pdf.SetFont("Geist", "", 7)
	const rowH = 4.2
	for _, s := range slices {
		if s.Value <= 0 {
			continue
		}
		pdf.SetFillColor(s.Color[0], s.Color[1], s.Color[2])
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

// drawTrend renders a net-worth trend line inside the box (x,y,w,h): a light
// baseline, a per-month tick under each point, the polyline with end markers,
// first/last month labels, and a value callout above the latest point.
func drawTrend(pdf *pdfDoc, x, y, w, h float64, pts []TrendPoint, lastLabel string) {
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
	sx := func(i int) float64 { return x + w*float64(i)/float64(len(pts)-1) }
	sy := func(v float64) float64 { return y + h - h*(v-mn)/span }

	// baseline + a tick under each month
	pdf.SetDrawColor(rule[0], rule[1], rule[2])
	pdf.SetLineWidth(0.2)
	pdf.Line(x, y+h, x+w, y+h)
	for i := range pts {
		pdf.Line(sx(i), y+h, sx(i), y+h+1)
	}

	pdf.SetDrawColor(accent[0], accent[1], accent[2])
	pdf.SetLineWidth(0.5)
	for i := 0; i < len(pts)-1; i++ {
		pdf.Line(sx(i), sy(pts[i].NetWorth), sx(i+1), sy(pts[i+1].NetWorth))
	}
	pdf.SetFillColor(accent[0], accent[1], accent[2])
	pdf.Circle(sx(0), sy(pts[0].NetWorth), 0.7, "F")
	pdf.Circle(sx(len(pts)-1), sy(pts[len(pts)-1].NetWorth), 1.1, "F")

	// value callout above the latest point
	pdf.SetFont("Geist", "B", 7)
	pdf.SetTextColor(accent[0], accent[1], accent[2])
	pdf.SetXY(x+w-40, sy(pts[len(pts)-1].NetWorth)-5)
	pdf.CellFormat(40, 3, lastLabel, "", 0, "R", false, 0, "")

	// first/last month labels
	pdf.SetFont("Geist", "", 6.5)
	pdf.SetTextColor(muted[0], muted[1], muted[2])
	pdf.SetXY(x, y+h+1.5)
	pdf.CellFormat(w/2, 3, pts[0].Label, "", 0, "L", false, 0, "")
	pdf.CellFormat(w/2, 3, pts[len(pts)-1].Label, "", 0, "R", false, 0, "")
}
