package email

import "fmt"

// brandIndigo is the wordmark accent — indigo-500, matching the SPA brand.
const brandIndigo = "#6366F1"

// Layout wraps an HTML body fragment in the shared branded email shell: a header
// bearing the "balances" wordmark and a muted footer. Both transactional senders
// can share it.
//
// Branding is inline by design. Remote images and SVG are unreliable in mail
// clients (frequently blocked by default), and a data-URI logo is stripped by
// Gmail — so a hosted/embedded logo's best case is "sometimes nicer" and its
// worst case is a broken image on a first impression. The brand here is a
// wordmark (text), so styled text reproduces it faithfully with nothing to
// block. Upgradeable: when we want the glyph mark, drop an <img> into the header
// and every sender that adopts Layout inherits it.
//
// The markup is table-based with inline styles (the email-client lowest common
// denominator) — no external CSS, no <style> block.
func Layout(bodyHTML string) string {
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
<span style="font-size:22px;font-weight:700;color:%s;letter-spacing:-0.02em;">Balances</span>
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
</html>`, brandIndigo, bodyHTML)
}
