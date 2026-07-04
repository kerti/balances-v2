package email

import "fmt"

// brandIndigo is the wordmark accent — indigo-500, matching the SPA brand. It
// colours the alt-text fallback so a blocked image still degrades to brand hue.
const brandIndigo = "#6366F1"

// Layout wraps an HTML body fragment in the shared branded email shell: a header
// bearing the "Balances" wordmark and a muted footer. Every transactional sender
// can share it.
//
// The header renders the real wordmark as a hosted raster (#163) — the brand SVG
// is outlined paths of a custom typeface, and mail clients strip @font-face, so
// an image is the only way to reproduce the actual letterforms. frontendURL is
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
<body style="margin:0;padding:0;background:#f1f5f9;">
<table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="background:#f1f5f9;">
<tr><td align="center" style="padding:24px 12px;">
<table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="max-width:560px;background:#ffffff;border-radius:12px;overflow:hidden;font-family:-apple-system,Segoe UI,Roboto,Helvetica,Arial,sans-serif;">
<tr><td style="padding:24px 32px;border-bottom:1px solid #e2e8f0;">
<img src="%s" alt="Balances" width="140" height="43" style="display:block;border:0;outline:none;text-decoration:none;height:43px;width:140px;font-size:22px;font-weight:700;color:%s;letter-spacing:-0.02em;">
</td></tr>
<tr><td style="padding:28px 32px;color:#475569;font-size:15px;line-height:1.6;">
%s
</td></tr>
<tr><td style="padding:20px 32px;border-top:1px solid #e2e8f0;color:#94a3b8;font-size:12px;line-height:1.5;">
You're receiving this because you created a Balances household. Balances helps your household track its net worth.
</td></tr>
</table>
</td></tr>
</table>
</body>
</html>`, logoURL, brandIndigo, bodyHTML)
}
