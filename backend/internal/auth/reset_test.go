package auth

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/kerti/balances-v2/backend/internal/db"
	"github.com/kerti/balances-v2/backend/internal/httperr"
)

// extractResetToken pulls the plaintext token out of the most recent reset
// email's link — the only place the plaintext ever appears (it is hashed at
// rest), mirroring how a real member retrieves it from their inbox.
func extractResetToken(t *testing.T, h *authHarness) string {
	t.Helper()
	msgs := h.mailer.sent()
	if len(msgs) == 0 {
		t.Fatal("no reset email was sent")
	}
	body := msgs[len(msgs)-1].Text
	i := strings.Index(body, "/reset?")
	if i < 0 {
		t.Fatalf("reset link not found in email body: %q", body)
	}
	rest := body[i:]
	if j := strings.IndexAny(rest, " \n"); j >= 0 {
		rest = rest[:j]
	}
	u, err := url.Parse(rest)
	if err != nil {
		t.Fatalf("parse reset link %q: %v", rest, err)
	}
	tok := u.Query().Get("token")
	if tok == "" {
		t.Fatalf("reset link carried no token: %q", rest)
	}
	return tok
}

// TestPasswordReset_HappyPath is the #282 tracer bullet: a member requests a
// reset, the emailed single-use token sets a new password, the new password logs
// in and the old one no longer does.
//
// covers: INV-AUTH-19
func TestPasswordReset_HappyPath(t *testing.T) {
	h := newAuthHarness(t)
	const email = "member@example.com"
	const oldPass = "the-original-passphrase"
	const newPass = "a-brand-new-passphrase"
	h.seedLocalUser(t, email, oldPass)

	req := h.post(t, "/auth/local/reset/request", map[string]string{"email": email})
	requireStatus(t, req, http.StatusNoContent)

	// Exactly one email, to the member, carrying the reset link.
	msgs := h.mailer.sent()
	if len(msgs) != 1 {
		t.Fatalf("want 1 reset email, got %d", len(msgs))
	}
	if msgs[0].To != email {
		t.Errorf("reset email To = %q, want %q", msgs[0].To, email)
	}
	token := extractResetToken(t, h)

	// Preview resolves the link to the bound email without consuming it.
	prev := h.doRaw(t, http.MethodGet, "/auth/local/reset?token="+token, nil, nil)
	requireStatus(t, prev, http.StatusOK)
	if pv := decodeBody[localResetPreviewResp](t, prev); pv.Email != email {
		t.Errorf("preview email = %q, want %q", pv.Email, email)
	}

	// Set the new password — mints a session directly.
	set := h.post(t, "/auth/local/reset", map[string]string{"token": token, "password": newPass})
	requireStatus(t, set, http.StatusNoContent)
	if findCookie(set, sessionCookieName) == nil {
		t.Fatal("reset set did not mint a session cookie")
	}

	// New password logs in; old one does not.
	okLogin := h.post(t, "/auth/local/login", map[string]string{"email": email, "password": newPass})
	requireStatus(t, okLogin, http.StatusNoContent)
	badLogin := h.post(t, "/auth/local/login", map[string]string{"email": email, "password": oldPass})
	requireStatus(t, badLogin, http.StatusUnauthorized)
}

// TestPasswordReset_NoEnumeration asserts the request endpoint behaves
// identically — generic 204, no email — for an unknown address and for a
// credential-less (dormant / Google-only) account, so it never reveals whether
// an email maps to a local account.
//
// covers: INV-AUTH-19
func TestPasswordReset_NoEnumeration(t *testing.T) {
	h := newAuthHarness(t)

	// Unknown email.
	unknown := h.post(t, "/auth/local/reset/request", map[string]string{"email": "nobody@example.com"})
	requireStatus(t, unknown, http.StatusNoContent)

	// A user with no local credential (dormant / Google-only): create the User
	// without a credential row.
	dormant, err := h.q.CreateLocalUser(context.Background(), db.CreateLocalUserParams{
		HouseholdID: h.user.HouseholdID,
		DisplayName: "Dormant",
		Email:       "dormant@example.com",
		Locale:      "en-GB",
		TimeZone:    "Asia/Jakarta",
	})
	if err != nil {
		t.Fatalf("seed dormant user: %v", err)
	}
	dorm := h.post(t, "/auth/local/reset/request", map[string]string{"email": dormant.Email})
	requireStatus(t, dorm, http.StatusNoContent)

	if msgs := h.mailer.sent(); len(msgs) != 0 {
		t.Errorf("no reset email should be sent for an absent/dormant account, got %d", len(msgs))
	}
}

// TestPasswordReset_SingleUse asserts a token resets the password exactly once:
// the second use of the same link is a generic 409.
//
// covers: INV-AUTH-19
func TestPasswordReset_SingleUse(t *testing.T) {
	h := newAuthHarness(t)
	const email = "single@example.com"
	h.seedLocalUser(t, email, "the-original-passphrase")

	requireStatus(t, h.post(t, "/auth/local/reset/request", map[string]string{"email": email}), http.StatusNoContent)
	token := extractResetToken(t, h)

	first := h.post(t, "/auth/local/reset", map[string]string{"token": token, "password": "first-new-passphrase"})
	requireStatus(t, first, http.StatusNoContent)

	second := h.post(t, "/auth/local/reset", map[string]string{"token": token, "password": "second-new-passphrase"})
	requireStatus(t, second, http.StatusConflict)
	if code := envelopeCode(t, second); code != string(httperr.CodeResetLinkNoLongerValid) {
		t.Errorf("reused token code = %q, want RESET_LINK_NO_LONGER_VALID", code)
	}
}

// TestPasswordReset_Expired asserts an expired link is rejected by both the
// preview and the set path with the generic 409, consuming nothing.
//
// covers: INV-AUTH-19
func TestPasswordReset_Expired(t *testing.T) {
	h := newAuthHarness(t)
	user := h.seedLocalUser(t, "expired@example.com", "the-original-passphrase")

	token, tokenHash, err := GenerateToken()
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	if _, err := h.q.CreatePasswordResetToken(context.Background(), db.CreatePasswordResetTokenParams{
		TokenHash: tokenHash,
		UserID:    user.ID,
		ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(-time.Minute), Valid: true},
	}); err != nil {
		t.Fatalf("seed expired token: %v", err)
	}

	prev := h.doRaw(t, http.MethodGet, "/auth/local/reset?token="+token, nil, nil)
	requireStatus(t, prev, http.StatusConflict)

	set := h.post(t, "/auth/local/reset", map[string]string{"token": token, "password": "a-brand-new-passphrase"})
	requireStatus(t, set, http.StatusConflict)
	if code := envelopeCode(t, set); code != string(httperr.CodeResetLinkNoLongerValid) {
		t.Errorf("expired token code = %q, want RESET_LINK_NO_LONGER_VALID", code)
	}
}

// TestPasswordReset_PreviewDoesNotConsume asserts the GET preview is reload-safe:
// previewing twice leaves the token usable for the set.
//
// covers: INV-AUTH-19
func TestPasswordReset_PreviewDoesNotConsume(t *testing.T) {
	h := newAuthHarness(t)
	const email = "reload@example.com"
	h.seedLocalUser(t, email, "the-original-passphrase")
	requireStatus(t, h.post(t, "/auth/local/reset/request", map[string]string{"email": email}), http.StatusNoContent)
	token := extractResetToken(t, h)

	for range 2 {
		requireStatus(t, h.doRaw(t, http.MethodGet, "/auth/local/reset?token="+token, nil, nil), http.StatusOK)
	}
	requireStatus(t, h.post(t, "/auth/local/reset", map[string]string{"token": token, "password": "a-brand-new-passphrase"}), http.StatusNoContent)
}

// TestPasswordReset_RevokesOtherSessions asserts a successful reset boots the
// member's pre-existing sessions — the "reset because compromised" guarantee.
//
// covers: INV-AUTH-19
func TestPasswordReset_RevokesOtherSessions(t *testing.T) {
	h := newAuthHarness(t)
	const email = "compromised@example.com"
	user := h.seedLocalUser(t, email, "the-original-passphrase")

	// A pre-existing (attacker) session.
	old, err := h.q.CreateSession(context.Background(), db.CreateSessionParams{
		ID:        "old-session-id",
		UserID:    user.ID,
		ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(time.Hour), Valid: true},
	})
	if err != nil {
		t.Fatalf("seed session: %v", err)
	}

	requireStatus(t, h.post(t, "/auth/local/reset/request", map[string]string{"email": email}), http.StatusNoContent)
	token := extractResetToken(t, h)
	requireStatus(t, h.post(t, "/auth/local/reset", map[string]string{"token": token, "password": "a-brand-new-passphrase"}), http.StatusNoContent)

	if _, err := h.q.GetSessionByID(context.Background(), old.ID); err == nil {
		t.Error("pre-existing session survived the reset — other sessions must be revoked")
	}
}

// TestPasswordReset_WeakPasswordRejected asserts the set path enforces the
// password policy floor and consumes nothing on rejection.
//
// covers: INV-AUTH-19
func TestPasswordReset_WeakPasswordRejected(t *testing.T) {
	h := newAuthHarness(t)
	const email = "weak@example.com"
	h.seedLocalUser(t, email, "the-original-passphrase")
	requireStatus(t, h.post(t, "/auth/local/reset/request", map[string]string{"email": email}), http.StatusNoContent)
	token := extractResetToken(t, h)

	weak := h.post(t, "/auth/local/reset", map[string]string{"token": token, "password": "short"})
	requireStatus(t, weak, http.StatusBadRequest)
	if code := envelopeCode(t, weak); code != string(httperr.CodeWeakPassword) {
		t.Errorf("weak password code = %q, want WEAK_PASSWORD", code)
	}
	// The rejected attempt consumed nothing: the token still works.
	requireStatus(t, h.post(t, "/auth/local/reset", map[string]string{"token": token, "password": "a-brand-new-passphrase"}), http.StatusNoContent)
}

// TestPasswordReset_NoOpWhenEmailDisabled asserts that with EMAIL_ENABLED=false
// the request mints no token and sends nothing, and the methods endpoint
// advertises reset as unavailable.
//
// covers: INV-AUTH-19
func TestPasswordReset_NoOpWhenEmailDisabled(t *testing.T) {
	h := newAuthHarness(t)
	h.h.emailEnabled = false
	const email = "mailoff@example.com"
	user := h.seedLocalUser(t, email, "the-original-passphrase")

	requireStatus(t, h.post(t, "/auth/local/reset/request", map[string]string{"email": email}), http.StatusNoContent)
	if msgs := h.mailer.sent(); len(msgs) != 0 {
		t.Errorf("email disabled: no email should be sent, got %d", len(msgs))
	}
	// No token row was minted.
	var count int
	if err := h.pool.QueryRow(context.Background(),
		"SELECT count(*) FROM password_reset_tokens WHERE user_id = $1", user.ID).Scan(&count); err != nil {
		t.Fatalf("count tokens: %v", err)
	}
	if count != 0 {
		t.Errorf("email disabled: no reset token should be minted, got %d", count)
	}

	methods := h.doRaw(t, http.MethodGet, "/auth/methods", nil, nil)
	if got := decodeBody[authMethodsResponse](t, methods); got.PasswordReset {
		t.Error("methods.password_reset should be false when email is disabled")
	}
}

// TestPasswordReset_MethodsAdvertisesReset asserts reset is advertised when both
// local auth and email are on.
func TestPasswordReset_MethodsAdvertisesReset(t *testing.T) {
	h := newAuthHarness(t)
	rec := h.doRaw(t, http.MethodGet, "/auth/methods", nil, nil)
	if got := decodeBody[authMethodsResponse](t, rec); !got.PasswordReset {
		t.Errorf("methods.password_reset should be true when local+email are on, got %+v", got)
	}
}

// TestPasswordReset_RequestRateLimited asserts repeated requests eventually hit
// the soft backoff (429) — the reset-email flooding brake.
func TestPasswordReset_RequestRateLimited(t *testing.T) {
	h := newAuthHarness(t)
	const email = "flood@example.com"
	h.seedLocalUser(t, email, "the-original-passphrase")

	var sawTooMany bool
	for range 6 {
		rec := h.post(t, "/auth/local/reset/request", map[string]string{"email": email})
		if rec.Code == http.StatusTooManyRequests {
			sawTooMany = true
			break
		}
		requireStatus(t, rec, http.StatusNoContent)
	}
	if !sawTooMany {
		t.Error("repeated reset requests never hit the rate limit")
	}
}
