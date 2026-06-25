package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// covers: INV-AUTH-09
// TestHandleStart_LocaleCookie verifies the pre-auth language pick rides the
// OAuth round-trip in a short-lived oauth_locale cookie, set only for a
// supported BCP47 value.
func TestHandleStart_LocaleCookie(t *testing.T) {
	h := newAuthHarness(t)

	t.Run("sets oauth_locale cookie for a supported ?lng=", func(t *testing.T) {
		rec := h.doRaw(t, "GET", "/auth/google/start?lng=id-ID", nil, nil)
		c := findCookie(rec, oauthLocaleCookieName)
		if c == nil {
			t.Fatal("expected oauth_locale cookie to be set")
		}
		if c.Value != "id-ID" {
			t.Errorf("oauth_locale value: want id-ID, got %q", c.Value)
		}
		if !c.HttpOnly {
			t.Error("oauth_locale cookie should be HttpOnly")
		}
	})

	t.Run("ignores an unsupported ?lng=", func(t *testing.T) {
		rec := h.doRaw(t, "GET", "/auth/google/start?lng=fr-FR", nil, nil)
		if c := findCookie(rec, oauthLocaleCookieName); c != nil {
			t.Errorf("unexpected oauth_locale cookie for unsupported lng: %+v", c)
		}
	})

	t.Run("no cookie when ?lng= absent", func(t *testing.T) {
		rec := h.doRaw(t, "GET", "/auth/google/start", nil, nil)
		if c := findCookie(rec, oauthLocaleCookieName); c != nil {
			t.Errorf("unexpected oauth_locale cookie: %+v", c)
		}
	})
}

// covers: INV-AUTH-10
// TestHandleCallback_FounderLocaleSeed verifies a brand-new founder's locale is
// seeded server-side from the oauth_locale cookie, falling back to en-GB (the
// lingua-franca default, ADR-0035) when the hint is absent or unsupported.
// Under ADR-0038 the founder is born at the gate commit, not the callback, so
// the hint is captured into the handshake's seed_locale at the callback and
// applied when the founder choice commits — asserted end-to-end here.
func TestHandleCallback_FounderLocaleSeed(t *testing.T) {
	cases := []struct {
		name       string
		cookie     string // "" means no oauth_locale cookie
		setCookie  bool
		wantLocale string
	}{
		{name: "seeds from id-ID hint", cookie: "id-ID", setCookie: true, wantLocale: "id-ID"},
		{name: "seeds from en-GB hint", cookie: "en-GB", setCookie: true, wantLocale: "en-GB"},
		{name: "falls back to en-GB when absent", setCookie: false, wantLocale: "en-GB"},
		{name: "falls back to en-GB on unsupported hint", cookie: "fr-FR", setCookie: true, wantLocale: "en-GB"},
	}

	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newAuthHarness(t)
			sub := "founder-locale-sub-" + string(rune('a'+i))
			email := "founder-locale-" + string(rune('a'+i)) + "@example.com"
			h.installStubOAuth(&googleClaims{
				Sub:           sub,
				Email:         email,
				EmailVerified: true,
				Name:          "Locale Founder",
			}, nil)

			req := callbackRequest("s", "the-code")
			req.AddCookie(&http.Cookie{Name: oauthStateCookieName, Value: "s"})
			if tc.setCookie {
				req.AddCookie(&http.Cookie{Name: oauthLocaleCookieName, Value: tc.cookie})
			}
			rec := httptest.NewRecorder()
			h.router.ServeHTTP(rec, req)

			if rec.Code != http.StatusFound {
				t.Fatalf("status: want 302, got %d (body: %s)", rec.Code, rec.Body.String())
			}
			// The short-lived oauth_locale cookie must be cleared after use.
			if c := findCookie(rec, oauthLocaleCookieName); c == nil || c.MaxAge >= 0 {
				t.Errorf("expected oauth_locale cookie cleared, got %+v", c)
			}

			// The callback captures the resolved hint into the handshake; no
			// account exists yet.
			hsCookie := findCookie(rec, onboardingCookieName)
			if hsCookie == nil || hsCookie.Value == "" {
				t.Fatal("expected onboarding handshake cookie")
			}
			hs, err := h.q.GetOnboardingHandshake(context.Background(), hsCookie.Value)
			if err != nil {
				t.Fatalf("GetOnboardingHandshake: %v", err)
			}
			if hs.SeedLocale != tc.wantLocale {
				t.Errorf("handshake seed_locale: want %q, got %q", tc.wantLocale, hs.SeedLocale)
			}

			// Committing the founder choice applies the seeded locale at birth.
			commit := h.onboardingRequest(t, "POST", "/onboarding/choice", hsCookie.Value, `{"found":true}`)
			requireStatus(t, commit, http.StatusNoContent)

			user, err := h.q.GetUserByGoogleSub(context.Background(), sub)
			if err != nil {
				t.Fatalf("GetUserByGoogleSub: %v", err)
			}
			if user.Locale != tc.wantLocale {
				t.Errorf("seeded locale: want %q, got %q", tc.wantLocale, user.Locale)
			}
		})
	}
}
