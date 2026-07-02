package auth

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/mail"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/kerti/balances-v2/backend/internal/db"
	"github.com/kerti/balances-v2/backend/internal/httperr"
)

// authMethodsResponse is the public pre-auth config surface (ADR-0039): the SPA
// reads which identity providers are live and renders only those.
type authMethodsResponse struct {
	Google bool `json:"google"`
	Local  bool `json:"local"`
	// PasswordReset reports whether emailed self-service reset is available — true
	// only when local auth and outbound email are both on (#282). The SPA hides the
	// "Forgot password?" affordance when false, so a mail-off self-host doesn't lead
	// the member to a dead end (the mail-off recovery paths are #283/#284).
	PasswordReset bool `json:"password_reset"`
	// DemoMode and the two fields below are the public demo posture (ADR-0041,
	// #217). When true, DemoEmail/DemoPassword carry the shared demo login so the
	// SPA can pre-fill the local-login form — there is no confidentiality cost to
	// exposing them here, since every demo visitor authenticates as this same
	// identity regardless of whether the SPA shows them.
	DemoMode     bool   `json:"demo_mode"`
	DemoEmail    string `json:"demo_email,omitempty"`
	DemoPassword string `json:"demo_password,omitempty"`
}

func (h *Handlers) handleAuthMethods(w http.ResponseWriter, _ *http.Request) {
	resp := authMethodsResponse{
		Google:        h.googleEnabled,
		Local:         h.localEnabled,
		PasswordReset: h.localEnabled && h.emailEnabled,
		DemoMode:      h.demoMode,
	}
	if h.demoMode {
		resp.DemoEmail = h.demoEmail
		resp.DemoPassword = h.demoPassword
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

type localRegisterReq struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
	Locale      string `json:"locale"`
}

// handleLocalRegister is the local founder's self-registration (ADR-0039). It
// does not write a users/households row: like the Google path it routes through
// the onboarding gate (ADR-0038) — validate, hash, stash the would-be credential
// in a short-lived handshake, and bounce the SPA to /onboarding where the
// founder choice commits the account. The founder's email is not independently
// verified (the operator owns the box; see the first-run-founder-window note in
// the self-host docs).
func (h *Handlers) handleLocalRegister(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req localRegisterReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httperr.Write(w, http.StatusBadRequest, httperr.CodeInvalidJSONBody, nil)
		return
	}

	email, ok := normalizeEmail(req.Email)
	if !ok {
		httperr.Write(w, http.StatusBadRequest, httperr.CodeValidation, map[string]any{
			"field": "email", "rule": "email",
		})
		return
	}
	if err := ValidatePasswordPolicy(req.Password); err != nil {
		httperr.Write(w, http.StatusBadRequest, httperr.CodeWeakPassword, map[string]any{
			"reason": weakPasswordReason(err),
		})
		return
	}

	// Email uniqueness is enforced by users_email_idx, but check first so a
	// collision is a clean 409 rather than a 500 at the gate commit. Registration
	// is the self-serve founder path, so surfacing "email in use" is acceptable
	// here (login never does — that path must not enumerate).
	if _, err := h.q.GetUserByEmail(ctx, email); err == nil {
		httperr.Write(w, http.StatusConflict, httperr.CodeEmailTaken, nil)
		return
	} else if !errors.Is(err, pgx.ErrNoRows) {
		slog.Error("local register: lookup email", "err", err)
		httperr.Write(w, http.StatusInternalServerError, httperr.CodeInternal, nil)
		return
	}

	hash, err := HashPassword(req.Password)
	if err != nil {
		slog.Error("local register: hash password", "err", err)
		httperr.Write(w, http.StatusInternalServerError, httperr.CodeInternal, nil)
		return
	}

	displayName := strings.TrimSpace(req.DisplayName)
	if displayName == "" {
		displayName = emailLocalPart(email)
	}

	// The handshake carries the credential across the gate (no users row yet). It
	// is google_sub-less and password_hash-bearing; the gate's found commit reads
	// PasswordHash != nil to take the local create path.
	if err := h.beginLocalOnboarding(ctx, w, email, displayName, resolveSeedLocale(req.Locale), hash); err != nil {
		slog.Error("local register: begin onboarding", "err", err)
		httperr.Write(w, http.StatusInternalServerError, httperr.CodeInternal, nil)
		return
	}
	// 204: the SPA navigates to /onboarding, where the handshake cookie authorises
	// the gate exactly as the Google redirect does.
	w.WriteHeader(http.StatusNoContent)
}

type localLoginReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// handleLocalLogin verifies an email+password against a local credential and
// mints a fresh session (ADR-0039). Hardened against enumeration and online
// guessing: every failure mode (unknown email, dormant/Google-only user, wrong
// password) returns the same generic 401, the password compare is constant-time,
// a missing account still pays the Argon2id cost (so timing can't distinguish
// present from absent), and per-IP + per-email backoff blunts guessing without a
// hard lockout.
func (h *Handlers) handleLocalLogin(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req localLoginReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httperr.Write(w, http.StatusBadRequest, httperr.CodeInvalidJSONBody, nil)
		return
	}
	email, _ := normalizeEmail(req.Email)
	ipKey := "ip:" + clientIP(r)
	emailKey := "email:" + email

	if d := h.limiter.maxRetryAfter(ipKey, emailKey); d > 0 {
		w.Header().Set("Retry-After", strconv.Itoa(int(d.Seconds())+1))
		httperr.Write(w, http.StatusTooManyRequests, httperr.CodeTooManyAttempts, nil)
		return
	}

	if h.verifyLocalLogin(ctx, email, req.Password) {
		h.limiter.recordSuccess(ipKey, emailKey)
		user, err := h.q.GetUserByEmail(ctx, email)
		if err != nil {
			// Raced away between verify and here (deleted) — treat as a failure.
			slog.Warn("local login: user vanished after verify", "err", err)
			httperr.Write(w, http.StatusUnauthorized, httperr.CodeInvalidCredentials, nil)
			return
		}
		if err := h.IssueSession(ctx, w, user.ID, r.UserAgent()); err != nil {
			slog.Error("local login: issue session", "err", err)
			httperr.Write(w, http.StatusInternalServerError, httperr.CodeInternal, nil)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}

	h.limiter.recordFailure(ipKey, emailKey)
	httperr.Write(w, http.StatusUnauthorized, httperr.CodeInvalidCredentials, nil)
}

// verifyLocalLogin reports whether email+password names a live local user with a
// matching credential. It runs the Argon2id verify on every path — including the
// no-user and no-credential cases (against a dummy hash) — so the response time
// does not betray whether the account exists.
func (h *Handlers) verifyLocalLogin(ctx context.Context, email, password string) bool {
	user, err := h.q.GetUserByEmail(ctx, email)
	if err != nil {
		_, _ = VerifyPassword(password, dummyArgonHash)
		return false
	}
	cred, err := h.q.GetLocalCredentialByUserID(ctx, user.ID)
	if err != nil {
		// Dormant (no credential) or Google-only user: still pay the hash cost.
		_, _ = VerifyPassword(password, dummyArgonHash)
		return false
	}
	ok, err := VerifyPassword(password, cred.PasswordHash)
	if err != nil {
		slog.Error("local login: verify password", "err", err, "user_id", user.ID)
		return false
	}
	return ok
}

// normalizeEmail lower-cases and trims an email and reports whether it parses as
// a single valid address. Mirrors the invitation flow's handling so a local
// account and an invitation key the same way.
func normalizeEmail(raw string) (string, bool) {
	trimmed := strings.ToLower(strings.TrimSpace(raw))
	if trimmed == "" {
		return "", false
	}
	addr, err := mail.ParseAddress(trimmed)
	if err != nil || addr.Address != trimmed {
		return "", false
	}
	return trimmed, true
}

func emailLocalPart(email string) string {
	if i := strings.IndexByte(email, '@'); i > 0 {
		return email[:i]
	}
	return email
}

func weakPasswordReason(err error) string {
	if errors.Is(err, errPasswordBreached) {
		return "breached"
	}
	return "min"
}

// clientIP extracts the connecting IP from RemoteAddr. We deliberately do not
// trust X-Forwarded-For / X-Real-IP (the server runs with no trusted proxy; see
// the RealIP note in httpserver), so the rate-limit key is the real transport
// peer, not a spoofable header.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// beginLocalOnboarding records a credential-bearing, google_sub-less handshake
// and writes the handshake cookie — the local mirror of beginOnboarding.
func (h *Handlers) beginLocalOnboarding(ctx context.Context, w http.ResponseWriter, email, displayName, seedLocale, passwordHash string) error {
	token, err := randomSessionID()
	if err != nil {
		return err
	}
	expiresAt := time.Now().Add(onboardingHandshakeTTL)
	if _, err := h.q.CreateOnboardingHandshake(ctx, db.CreateOnboardingHandshakeParams{
		ID:               token,
		GoogleSub:        nil,
		Email:            email,
		DisplayName:      displayName,
		PictureUrl:       nil,
		SeedLocale:       seedLocale,
		HintInvitationID: nil,
		PasswordHash:     &passwordHash,
		ExpiresAt:        pgtype.Timestamptz{Time: expiresAt, Valid: true},
	}); err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     onboardingCookieName,
		Value:    token,
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
	return nil
}
