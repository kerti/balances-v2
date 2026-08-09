package email

import "fmt"

// BrandBrass is the deep brass the SPA uses for --primary (ADR-0054). It
// colours the alt-text fallback so a blocked image still degrades to brand hue,
// and the call-to-action buttons in the transactional senders.
//
// The deep #8A6A30 rather than the lighter #B08947 the brand table names as the
// accent: both of those uses are *text* on white, where #B08947 is 3.2:1 —
// below AA. The lighter brass is for marks and fills, not for setting type on
// paper.
const BrandBrass = "#8A6A30"

// Neutrals, matching the SPA's light theme: warm paper page, white card, warm
// border, graphite ink. muted is the same value the app derives for
// --muted-foreground, chosen so 12px footer type still clears AA on white — the
// pre-rebrand footer was slate-400 on white, a 2.6:1 that never passed.
const (
	brandPaper  = "#F7F5F1"
	brandCard   = "#FFFFFF"
	brandBorder = "#E7E1D8"
	brandInk    = "#3A4149"
	brandMuted  = "#6E675C"
)

// Layout wraps an HTML body fragment in the shared branded email shell: a header
// bearing the "Balances" wordmark and a muted footer. Every transactional sender
// can share it.
//
// The header renders the real wordmark as a hosted raster (#163) — the brand SVG
// is outlined Plus Jakarta Sans, a face the app does not otherwise ship, and mail
// clients strip @font-face, so an image is the only way to reproduce the actual
// letterforms. Its 140x38 box is the wordmark's own aspect ratio; `make brand`
// re-exports the PNG, so the two cannot drift. frontendURL is
// the single-origin app that serves /brand/email-logo.png (Vite copies public/
// into dist/, ADR-0030). Remote images are blocked-by-default in some clients;
// that's mitigated by the alt text, which IS the styled wordmark name — a
// blocked/failed image degrades to the brand name in brand colour, not a broken
// glyph. Net: best case exact letterforms, worst case the name in plain text.
//
// The markup is table-based with inline styles (the email-client lowest common
// denominator) — no external CSS, no <style> block.
func Layout(frontendURL, bodyHTML string) string {
	logoURL := frontendURL + "/brand/email-logo.png"
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
</head>
<body style="margin:0;padding:0;background:%s;">
<table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="background:%s;">
<tr><td align="center" style="padding:24px 12px;">
<table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="max-width:560px;background:%s;border-radius:12px;overflow:hidden;font-family:-apple-system,Segoe UI,Roboto,Helvetica,Arial,sans-serif;">
<tr><td style="padding:24px 32px;border-bottom:1px solid %s;">
<img src="%s" alt="Balances" width="140" height="38" style="display:block;border:0;outline:none;text-decoration:none;height:38px;width:140px;font-size:22px;font-weight:700;color:%s;letter-spacing:-0.02em;">
</td></tr>
<tr><td style="padding:28px 32px;color:%s;font-size:15px;line-height:1.6;">
%s
</td></tr>
<tr><td style="padding:20px 32px;border-top:1px solid %s;color:%s;font-size:12px;line-height:1.5;">
You're receiving this because you created a Balances household. Balances helps your household track its net worth.
</td></tr>
</table>
</td></tr>
</table>
</body>
</html>`, brandPaper, brandPaper, brandCard, brandBorder, logoURL, BrandBrass, brandInk, bodyHTML, brandBorder, brandMuted)
}
