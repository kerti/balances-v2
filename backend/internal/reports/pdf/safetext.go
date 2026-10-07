package pdf

import (
	"strings"

	"codeberg.org/go-pdf/fpdf"
)

// pdfDoc shadows fpdf's text-drawing methods so every string — including
// user-typed account and member names — is made safe before fpdf sees it.
// fpdf's UTF-8 CID font map is sized for the BMP (U+0000–U+FFFF), so any rune
// above it (nearly every emoji) panics inside Output (#703). Wrapping the
// methods, not the call sites, keeps future draw calls covered too.
type pdfDoc struct {
	*fpdf.Fpdf
}

func (p *pdfDoc) CellFormat(w, h float64, txtStr, borderStr string, ln int, alignStr string, fill bool, link int, linkStr string) {
	p.Fpdf.CellFormat(w, h, pdfSafeText(txtStr), borderStr, ln, alignStr, fill, link, linkStr)
}

func (p *pdfDoc) MultiCell(w, h float64, txtStr, borderStr, alignStr string, fill bool) {
	p.Fpdf.MultiCell(w, h, pdfSafeText(txtStr), borderStr, alignStr, fill)
}

func (p *pdfDoc) GetStringWidth(s string) float64 {
	return p.Fpdf.GetStringWidth(pdfSafeText(s))
}

// pdfSafeText drops runes outside the BMP plus the emoji presentation
// selector (U+FE0F) and zero-width joiner (U+200D) left behind by them, then
// tidies the spaces the removal leaves. Text with nothing to drop is returned
// untouched.
func pdfSafeText(s string) string {
	var b strings.Builder
	dropped := false
	for _, r := range s {
		if r > 0xFFFF || r == 0xFE0F || r == 0x200D {
			dropped = true
			continue
		}
		b.WriteRune(r)
	}
	if !dropped {
		return s
	}
	out := b.String()
	for strings.Contains(out, "  ") {
		out = strings.ReplaceAll(out, "  ", " ")
	}
	return strings.TrimSpace(out)
}
