package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// covers: INV-AUTH-11
// The invitation accept URL carries the inviter's locale as ?lng= so the invitee
// inherits the household language by default (ADR-0035). The accept link is a
// direct backend /start URL, so this ?lng= becomes the oauth_locale seed hint.
func TestCreateInvitation_AcceptURLCarriesInviterLocale(t *testing.T) {
	h := newAuthHarness(t)

	t.Run("id-ID inviter", func(t *testing.T) {
		h.user.Locale = "id-ID"
		rec := h.do(t, "POST", "/invitations", map[string]any{"email": "guest-id@example.com"})
		requireStatus(t, rec, http.StatusCreated)
		body := decodeBody[createInvitationResp](t, rec)
		if !strings.Contains(body.AcceptURL, "lng=id-ID") {
			t.Errorf("accept_url missing lng=id-ID: %q", body.AcceptURL)
		}
		if !strings.Contains(body.AcceptURL, "invite=") {
			t.Errorf("accept_url missing invite token: %q", body.AcceptURL)
		}
	})

	t.Run("en-GB inviter", func(t *testing.T) {
		h.user.Locale = "en-GB"
		rec := h.do(t, "POST", "/invitations", map[string]any{"email": "guest-en@example.com"})
		requireStatus(t, rec, http.StatusCreated)
		body := decodeBody[createInvitationResp](t, rec)
		if !strings.Contains(body.AcceptURL, "lng=en-GB") {
			t.Errorf("accept_url missing lng=en-GB: %q", body.AcceptURL)
		}
	})
}

// covers: INV-AUTH-10
// An invited member's locale is seeded from the oauth_locale hint exactly like a
// founder's, falling back to en-GB when the hint is absent. Under ADR-0038 the
// invited path runs through the gate: the callback captures the hint into the
// handshake's seed_locale, and the join commit applies it at account birth —
// asserted end-to-end here.
func TestHandleCallback_InvitedUserLocaleSeed(t *testing.T) {
	cases := []struct {
		name       string
		setCookie  bool
		cookie     string
		wantLocale string
	}{
		{name: "seeds from id-ID hint", setCookie: true, cookie: "id-ID", wantLocale: "id-ID"},
		{name: "falls back to en-GB when absent", setCookie: false, wantLocale: "en-GB"},
	}

	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newAuthHarness(t)
			email := "invited-locale-" + string(rune('a'+i)) + "@example.com"
			token := mustSeedInvitation(t, h, email, time.Now().Add(24*time.Hour))
			invite, err := h.q.GetInvitationByToken(context.Background(), token)
			if err != nil {
				t.Fatalf("GetInvitationByToken: %v", err)
			}

			h.installStubOAuth(&googleClaims{
				Sub:           "invited-locale-sub-" + string(rune('a'+i)),
				Email:         email,
				EmailVerified: true,
				Name:          "Invited Member",
			}, nil)

			req := callbackRequest("s", "the-code")
			req.AddCookie(&http.Cookie{Name: oauthStateCookieName, Value: "s"})
			req.AddCookie(&http.Cookie{Name: oauthInviteCookieName, Value: token})
			if tc.setCookie {
				req.AddCookie(&http.Cookie{Name: oauthLocaleCookieName, Value: tc.cookie})
			}
			rec := httptest.NewRecorder()
			h.router.ServeHTTP(rec, req)

			if rec.Code != http.StatusFound {
				t.Fatalf("status: want 302, got %d (body: %s)", rec.Code, rec.Body.String())
			}
			hsCookie := findCookie(rec, onboardingCookieName)
			if hsCookie == nil || hsCookie.Value == "" {
				t.Fatal("expected onboarding handshake cookie")
			}

			// Commit the join at the gate; the seeded locale is applied at birth.
			commit := h.onboardingRequest(t, "POST", "/onboarding/choice", hsCookie.Value,
				`{"join":true,"invitation_id":"`+invite.ID.String()+`"}`)
			requireStatus(t, commit, http.StatusNoContent)

			user, err := h.q.GetUserByGoogleSub(context.Background(),
				"invited-locale-sub-"+string(rune('a'+i)))
			if err != nil {
				t.Fatalf("GetUserByGoogleSub: %v", err)
			}
			// Invited members must still join the inviting household, not a new one.
			if user.HouseholdID != h.user.HouseholdID {
				t.Errorf("invited user should join harness household")
			}
			if user.Locale != tc.wantLocale {
				t.Errorf("invited seed locale: want %q, got %q", tc.wantLocale, user.Locale)
			}
		})
	}
}
