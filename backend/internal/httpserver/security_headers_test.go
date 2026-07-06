package httpserver

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// covers: INV-SERVING-06
func TestSecurityHeaders_SetOnEveryResponse(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/anything", nil)
	securityHeaders(false)(next).ServeHTTP(rec, req)

	h := rec.Header()
	if got := h.Get("Content-Security-Policy"); got == "" {
		t.Errorf("Content-Security-Policy not set")
	} else {
		if !strings.Contains(got, "default-src 'self'") {
			t.Errorf("CSP missing default-src 'self': %q", got)
		}
		if !strings.Contains(got, "frame-ancestors 'none'") {
			t.Errorf("CSP missing frame-ancestors 'none': %q", got)
		}
		if strings.Contains(got, "script-src 'self' 'unsafe-inline'") {
			t.Errorf("CSP script-src must not allow unsafe-inline: %q", got)
		}
	}
	if got := h.Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q, want nosniff", got)
	}
	if got := h.Get("Referrer-Policy"); got == "" {
		t.Errorf("Referrer-Policy not set")
	}
}

func TestSecurityHeaders_HSTSGatedOnCookieSecure(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	t.Run("cookieSecure false: no HSTS", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/anything", nil)
		securityHeaders(false)(next).ServeHTTP(rec, req)
		if got := rec.Header().Get("Strict-Transport-Security"); got != "" {
			t.Errorf("Strict-Transport-Security = %q, want empty when cookieSecure=false", got)
		}
	})

	t.Run("cookieSecure true: HSTS set", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/anything", nil)
		securityHeaders(true)(next).ServeHTTP(rec, req)
		if got := rec.Header().Get("Strict-Transport-Security"); got == "" {
			t.Errorf("Strict-Transport-Security not set when cookieSecure=true")
		}
	})
}
