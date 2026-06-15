package auth

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// covers: INV-NOTIFICATIONS-01
func TestHandleCreateInvitation(t *testing.T) {
	h := newAuthHarness(t)

	t.Run("201 creates invitation, persists row, sends email", func(t *testing.T) {
		rec := h.do(t, "POST", "/invitations", map[string]any{
			"email": "guest@example.com",
		})
		requireStatus(t, rec, http.StatusCreated)
		body := decodeBody[createInvitationResp](t, rec)
		if body.InvitedEmail != "guest@example.com" {
			t.Errorf("invited_email: got %q", body.InvitedEmail)
		}
		if !strings.Contains(body.AcceptURL, "invite=") {
			t.Errorf("accept_url missing invite token: %q", body.AcceptURL)
		}

		// DB row exists by the token embedded in accept_url.
		u, err := url.Parse(body.AcceptURL)
		if err != nil {
			t.Fatalf("parse accept_url: %v", err)
		}
		token := u.Query().Get("invite")
		if token == "" {
			t.Fatal("accept_url missing invite query param")
		}
		row, err := h.q.GetInvitationByToken(context.Background(), token)
		if err != nil {
			t.Fatalf("GetInvitationByToken: %v", err)
		}
		if row.ID != body.ID {
			t.Errorf("db row id: want %s, got %s", body.ID, row.ID)
		}
		if row.InvitedEmail != "guest@example.com" {
			t.Errorf("db row invited_email: got %q", row.InvitedEmail)
		}

		// Mailer was called exactly once with the invited address.
		sent := h.mailer.sent()
		if len(sent) != 1 {
			t.Fatalf("mailer messages: want 1, got %d", len(sent))
		}
		if sent[0].To != "guest@example.com" {
			t.Errorf("mailer to: got %q", sent[0].To)
		}
		if !strings.Contains(sent[0].HTML, body.AcceptURL) {
			t.Error("HTML body should contain accept URL")
		}
	})

	t.Run("lowercases and trims email", func(t *testing.T) {
		h := newAuthHarness(t) // fresh harness so we can assert exactly one mail
		rec := h.do(t, "POST", "/invitations", map[string]any{
			"email": "  Friend@Example.COM  ",
		})
		requireStatus(t, rec, http.StatusCreated)
		body := decodeBody[createInvitationResp](t, rec)
		if body.InvitedEmail != "friend@example.com" {
			t.Errorf("invited_email: want lowercase trimmed, got %q", body.InvitedEmail)
		}
	})

	t.Run("400 invalid json", func(t *testing.T) {
		rec := h.do(t, "POST", "/invitations", "{not-json")
		requireStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("400 invalid email", func(t *testing.T) {
		rec := h.do(t, "POST", "/invitations", map[string]any{
			"email": "not-an-email",
		})
		requireStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("400 cannot invite self", func(t *testing.T) {
		rec := h.do(t, "POST", "/invitations", map[string]any{
			"email": h.user.Email,
		})
		requireStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("401 without user in context", func(t *testing.T) {
		rec := h.doRaw(t, "POST", "/invitations", map[string]any{"email": "x@example.com"}, nil)
		requireStatus(t, rec, http.StatusUnauthorized)
	})
}

// covers: INV-NOTIFICATIONS-03
func TestHtmlEscape(t *testing.T) {
	cases := []struct{ in, want string }{
		{"plain", "plain"},
		{`<script>alert("x")</script>`, `&lt;script&gt;alert(&quot;x&quot;)&lt;/script&gt;`},
		{"a & b", "a &amp; b"},
		{"", ""},
	}
	for _, c := range cases {
		if got := htmlEscape(c.in); got != c.want {
			t.Errorf("htmlEscape(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// covers: INV-AUTH-06
func TestRandomInvitationToken(t *testing.T) {
	seen := make(map[string]bool, 16)
	for range 16 {
		tok, err := randomInvitationToken()
		if err != nil {
			t.Fatalf("randomInvitationToken: %v", err)
		}
		if tok == "" {
			t.Fatal("empty token")
		}
		if seen[tok] {
			t.Errorf("duplicate token %q", tok)
		}
		seen[tok] = true
	}
}

// covers: INV-AUTH-02
func TestRandomState(t *testing.T) {
	seen := make(map[string]bool, 16)
	for range 16 {
		s, err := randomState()
		if err != nil {
			t.Fatalf("randomState: %v", err)
		}
		if s == "" {
			t.Fatal("empty state")
		}
		if seen[s] {
			t.Errorf("duplicate state %q", s)
		}
		seen[s] = true
	}
}
