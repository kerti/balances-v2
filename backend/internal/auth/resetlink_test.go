package auth

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/kerti/balances-v2/backend/internal/db"
)

// TestMintResetPasswordLink_ResetsActiveMember is the #284 tracer bullet: the
// operator mints a link for a member who already holds a credential — the one
// thing the in-app reactivation path refuses — and the member follows it to set a
// new password and sign in with it (the old password no longer works). This is
// the CLI's core, independent of mail posture (it touches no mailer).
//
// covers: INV-AUTH-21
func TestMintResetPasswordLink_ResetsActiveMember(t *testing.T) {
	h := newAuthHarness(t)
	const oldPass, newPass = "the-original-passphrase", "a-freshly-chosen-passphrase"
	member := h.seedLocalUser(t, "active@example.com", oldPass)

	link, expiresAt, err := MintResetPasswordLink(
		context.Background(), h.q, h.h.frontendURL, "Active@Example.com ")
	if err != nil {
		t.Fatalf("mint link: %v", err)
	}
	if !expiresAt.After(time.Now()) {
		t.Errorf("expiry %v is not in the future", expiresAt)
	}

	token := tokenFromSetPasswordURL(t, link)

	// The plaintext never lands at rest: only its SHA-256 hash is stored, bound to
	// the member, and the plaintext is not the hash.
	if token == HashToken(token) {
		t.Fatal("stored representation equals plaintext token")
	}
	row, err := h.q.GetPasswordResetToken(context.Background(), HashToken(token))
	if err != nil {
		t.Fatalf("token not stored by hash: %v", err)
	}
	if row.UserID != member.ID {
		t.Errorf("token bound to %s, want member %s", row.UserID, member.ID)
	}

	// Following the link sets the new password and mints a session directly.
	set := h.doRaw(t, http.MethodPost, "/auth/local/reset",
		map[string]string{"token": token, "password": newPass}, nil)
	requireStatus(t, set, http.StatusNoContent)
	if findCookie(set, sessionCookieName) == nil {
		t.Fatal("reset set did not mint a session cookie")
	}

	// The new password logs in; the old one no longer does.
	ok := h.doRaw(t, http.MethodPost, "/auth/local/login",
		map[string]string{"email": member.Email, "password": newPass}, nil)
	requireStatus(t, ok, http.StatusNoContent)
	stale := h.doRaw(t, http.MethodPost, "/auth/local/login",
		map[string]string{"email": member.Email, "password": oldPass}, nil)
	requireStatus(t, stale, http.StatusUnauthorized)
}

// TestMintResetPasswordLink_DormantMember shows the CLI is a superset of in-app
// reactivation: it also brings back a credential-less (dormant) local member,
// creating their first credential — the no-founder, no-mail recovery path.
//
// covers: INV-AUTH-21
func TestMintResetPasswordLink_DormantMember(t *testing.T) {
	h := newAuthHarness(t)
	member := h.seedDormantMember(t, "dormant@example.com")

	link, _, err := MintResetPasswordLink(
		context.Background(), h.q, h.h.frontendURL, member.Email)
	if err != nil {
		t.Fatalf("mint link: %v", err)
	}
	token := tokenFromSetPasswordURL(t, link)

	const newPass = "a-brand-new-passphrase"
	set := h.doRaw(t, http.MethodPost, "/auth/local/reset",
		map[string]string{"token": token, "password": newPass}, nil)
	requireStatus(t, set, http.StatusNoContent)

	login := h.doRaw(t, http.MethodPost, "/auth/local/login",
		map[string]string{"email": member.Email, "password": newPass}, nil)
	requireStatus(t, login, http.StatusNoContent)
}

// TestMintResetPasswordLink_RefusesGoogleMember asserts a Google account is
// refused (ErrGoogleAccount): there is no password to reset and minting a local
// credential would be account-linking, out of scope (ADR-0039). No token is
// stored for them.
//
// covers: INV-AUTH-21
func TestMintResetPasswordLink_RefusesGoogleMember(t *testing.T) {
	h := newAuthHarness(t)
	g, err := h.q.CreateUser(context.Background(), db.CreateUserParams{
		HouseholdID: h.user.HouseholdID,
		DisplayName: "Googler",
		Email:       "googler@example.com",
		GoogleSub:   "google-sub-284",
		Locale:      "en-GB",
		TimeZone:    "Asia/Jakarta",
		CreatedBy:   &h.user.ID,
	})
	if err != nil {
		t.Fatalf("seed google member: %v", err)
	}

	_, _, err = MintResetPasswordLink(context.Background(), h.q, h.h.frontendURL, g.Email)
	if !errors.Is(err, ErrGoogleAccount) {
		t.Fatalf("err = %v, want ErrGoogleAccount", err)
	}
}

// TestMintResetPasswordLink_UnknownEmail asserts an email with no live user is a
// precise ErrNoLocalAccount — the operator holds DB access, so there is nothing to
// hide here (unlike the HTTP reset's no-enumeration response).
//
// covers: INV-AUTH-21
func TestMintResetPasswordLink_UnknownEmail(t *testing.T) {
	h := newAuthHarness(t)
	_, _, err := MintResetPasswordLink(
		context.Background(), h.q, h.h.frontendURL, "nobody@example.com")
	if !errors.Is(err, ErrNoLocalAccount) {
		t.Fatalf("err = %v, want ErrNoLocalAccount", err)
	}
}

// TestMintResetPasswordLink_InvalidEmail asserts a non-address argument is
// rejected before any DB lookup.
//
// covers: INV-AUTH-21
func TestMintResetPasswordLink_InvalidEmail(t *testing.T) {
	h := newAuthHarness(t)
	if _, _, err := MintResetPasswordLink(
		context.Background(), h.q, h.h.frontendURL, "not-an-email"); !errors.Is(err, ErrInvalidEmail) {
		t.Fatalf("err = %v, want ErrInvalidEmail", err)
	}
}
