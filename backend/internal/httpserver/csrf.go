package httpserver

import (
	"net/http"
	"net/url"

	"github.com/kerti/balances-v2/backend/internal/httperr"
)

// csrfOriginCheck is a second CSRF layer behind the SameSite=Lax session
// cookie (INV-AUTH-03, #364 CF-21): SameSite alone would silently regress to
// a no-op if a future browser bug or config change ever let a cross-site
// request carry the cookie. It only inspects non-safe methods — GET/HEAD/OPTIONS
// have no side effects to forge.
//
// Modern browsers attach Sec-Fetch-Site (Fetch Metadata) to every request;
// where that's absent, they still always attach Origin on non-safe methods
// (per the Fetch spec, same-origin or not). A request with neither header is
// from a client SameSite never protected against anyway (curl, a mobile app
// using the session cookie directly) — it's let through rather than broken.
func csrfOriginCheck(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(w, r)
			return
		}

		if site := r.Header.Get("Sec-Fetch-Site"); site != "" {
			if site != "same-origin" && site != "none" {
				httperr.Write(w, http.StatusForbidden, httperr.CodeCrossSiteRequestBlocked, nil)
				return
			}
			next.ServeHTTP(w, r)
			return
		}

		if origin := r.Header.Get("Origin"); origin != "" {
			originURL, err := url.Parse(origin)
			if err != nil || originURL.Host != r.Host {
				httperr.Write(w, http.StatusForbidden, httperr.CodeCrossSiteRequestBlocked, nil)
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}
