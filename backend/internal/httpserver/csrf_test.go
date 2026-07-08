package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

// covers: INV-AUTH-29
func TestCSRFOriginCheck_SafeMethodsAlwaysPass(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodOptions} {
		req := httptest.NewRequest(method, "/api/anything", nil)
		req.Header.Set("Sec-Fetch-Site", "cross-site")
		rec := httptest.NewRecorder()
		csrfOriginCheck(okHandler()).ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("method %s: status = %d, want 200 (safe methods bypass the check)", method, rec.Code)
		}
	}
}

// covers: INV-AUTH-29
func TestCSRFOriginCheck_SecFetchSite(t *testing.T) {
	cases := []struct {
		name string
		site string
		want int
	}{
		{"same-origin allowed", "same-origin", http.StatusOK},
		{"none allowed (non-browser navigation)", "none", http.StatusOK},
		{"cross-site blocked", "cross-site", http.StatusForbidden},
		{"same-site blocked (different subdomain)", "same-site", http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/anything", nil)
			req.Header.Set("Sec-Fetch-Site", tc.site)
			rec := httptest.NewRecorder()
			csrfOriginCheck(okHandler()).ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Errorf("Sec-Fetch-Site=%q: status = %d, want %d", tc.site, rec.Code, tc.want)
			}
		})
	}
}

// covers: INV-AUTH-29
func TestCSRFOriginCheck_OriginFallback(t *testing.T) {
	cases := []struct {
		name   string
		origin string
		want   int
	}{
		{"matching origin allowed", "https://example.com", http.StatusOK},
		{"cross-origin blocked", "https://evil.example", http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/anything", nil)
			req.Host = "example.com"
			req.Header.Set("Origin", tc.origin)
			rec := httptest.NewRecorder()
			csrfOriginCheck(okHandler()).ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Errorf("Origin=%q: status = %d, want %d", tc.origin, rec.Code, tc.want)
			}
		})
	}
}

// covers: INV-AUTH-29
func TestCSRFOriginCheck_NoSignalPassesThrough(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/anything", nil)
	rec := httptest.NewRecorder()
	csrfOriginCheck(okHandler()).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("no Sec-Fetch-Site/Origin: status = %d, want 200 (non-browser client, SameSite never protected this anyway)", rec.Code)
	}
}
