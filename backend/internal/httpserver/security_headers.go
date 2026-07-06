package httpserver

import "net/http"

// securityHeaders sets a fixed set of hardening headers on every response.
// React's JSX escaping is otherwise the only XSS defense layer, and there's
// no clickjacking defense on this cookie-authenticated app (#362).
//
// script-src omits 'unsafe-inline': the app has no inline <script> (the
// theme-flash-prevention script lives in the static public/theme-init.js
// file specifically so this can stay strict, see index.html).
//
// style-src keeps 'unsafe-inline': several components set the React `style`
// prop with request-derived values (chart bar widths/colors) that can't be
// moved to an external stylesheet. Inline styles can't execute script, so
// this doesn't reopen the XSS surface script-src closes.
//
// HSTS is gated on cookieSecure (COOKIE_SECURE=true): sending it over plain
// HTTP is a no-op per spec, and asserting it in a local/self-host HTTP setup
// would be actively wrong.
func securityHeaders(cookieSecure bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()
			h.Set("Content-Security-Policy",
				"default-src 'self'; "+
					"script-src 'self'; "+
					"style-src 'self' 'unsafe-inline'; "+
					"img-src 'self'; "+
					"connect-src 'self'; "+
					"frame-ancestors 'none'; "+
					"base-uri 'self'; "+
					"form-action 'self'")
			h.Set("X-Content-Type-Options", "nosniff")
			h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
			if cookieSecure {
				h.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
			}
			next.ServeHTTP(w, r)
		})
	}
}
