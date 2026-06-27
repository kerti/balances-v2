package auth

import (
	"strings"
	"testing"
)

// The accept link routes by which providers the instance runs (ADR-0039/#281):
// local enabled → the SPA /accept set-password route; local disabled (the hosted,
// Google-only posture) → the unchanged backend google /start URL. Either way the
// inviter's locale rides the link.
func TestInviteAcceptURL_RoutesByEnabledProviders(t *testing.T) {
	base := &Handlers{
		frontendURL: "https://app.example",
		backendURL:  "https://api.example",
	}

	t.Run("local enabled → SPA accept route", func(t *testing.T) {
		h := *base
		h.localEnabled = true
		got := h.inviteAcceptURL("tok123", "id-ID")
		if !strings.HasPrefix(got, "https://app.example/accept?token=tok123") {
			t.Errorf("local accept URL = %q, want SPA /accept route", got)
		}
		if !strings.Contains(got, "lng=id-ID") {
			t.Errorf("accept URL dropped the locale: %q", got)
		}
	})

	t.Run("local disabled → backend google start", func(t *testing.T) {
		h := *base
		h.localEnabled = false
		got := h.inviteAcceptURL("tok123", "en-GB")
		if !strings.HasPrefix(got, "https://api.example/api/auth/google/start?invite=tok123") {
			t.Errorf("google-only accept URL = %q, want backend /start", got)
		}
		if !strings.Contains(got, "lng=en-GB") {
			t.Errorf("accept URL dropped the locale: %q", got)
		}
	})
}
