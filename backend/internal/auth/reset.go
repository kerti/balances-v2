package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/kerti/balances-v2/backend/internal/db"
	"github.com/kerti/balances-v2/backend/internal/email"
	"github.com/kerti/balances-v2/backend/internal/httperr"
)

// Emailed self-service password reset (ADR-0039/#282). Reuses the shared
// set-password-token primitive (token.go, #281): a single-use, short-TTL,
// ≥256-bit token, stored only as a SHA-256 hash, whose plaintext lives only in
// the emailed link. Three endpoints:
//
//   - request: never reveals whether the email maps to an account (no
//     enumeration); the email is sent off the request goroutine so its timing
//     can't betray a hit, and a generic 204 comes back either way.
//   - preview (GET): validates a token for the set screen WITHOUT consuming it,
//     so a reload doesn't burn the single-use link.
//   - set (POST): atomically consumes the token, replaces the credential,
//     revokes the member's other sessions ("reset because compromised"), and
//     signs them in.

type localResetRequestReq struct {
	Email string `json:"email"`
}

// handleLocalResetRequest starts a reset: if the email names a reachable local
// account it mints a single-use token and emails the link, but it ALWAYS returns
// the same generic 204 — an unknown email, a Google-only/dormant user, and a
// successful send are indistinguishable to the caller (no user enumeration,
// ADR-0039). The email is dispatched on a background goroutine so the SMTP
// round-trip never lands in the response timing, the timing half of the same
// guarantee. With email disabled the whole path is a no-op (the mail-off
// recovery routes are #283/#284).
func (h *Handlers) handleLocalResetRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req localResetRequestReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httperr.Write(w, http.StatusBadRequest, httperr.CodeInvalidJSONBody, nil)
		return
	}
	email, _ := normalizeEmail(req.Email)

	// Soft per-IP + per-email backoff to blunt reset-email flooding of a victim's
	// inbox and any timing-amplification probing. Keyed under a `reset` namespace
	// so it never crosses wires with the login limiter. A 429 here is independent
	// of whether the email exists, so it does not enumerate. An unparseable email
	// still consumes rate (keyed on the empty string) but never proceeds.
	ipKey := "reset-ip:" + clientIP(r)
	emailKey := "reset:" + email
	if d := h.limiter.maxRetryAfter(ipKey, emailKey); d > 0 {
		w.Header().Set("Retry-After", strconv.Itoa(int(d.Seconds())+1))
		httperr.Write(w, http.StatusTooManyRequests, httperr.CodeTooManyAttempts, nil)
		return
	}
	h.limiter.recordFailure(ipKey, emailKey)

	// No mail, no reset link to deliver — return the same generic 204 without
	// minting a dead token. The methods endpoint already hides the affordance when
	// email is off, so this is belt-and-braces.
	if email != "" && h.emailEnabled {
		h.issueResetToken(ctx, email)
	}

	// One answer for every path. 204: nothing to show the caller; the SPA tells the
	// member to check their email regardless.
	w.WriteHeader(http.StatusNoContent)
}

// issueResetToken looks up the account and, only when it is a reachable local
// user (has a local_credentials row), mints a token and dispatches the email.
// All lookups are best-effort and silent: a miss is the no-enumeration design,
// not an error to surface. Runs inline (the cheap DB work) but hands the email
// send to h.dispatch so the response has already returned by the time SMTP runs.
func (h *Handlers) issueResetToken(ctx context.Context, email string) {
	user, err := h.q.GetUserByEmail(ctx, email)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			slog.Error("reset request: lookup email", "err", err)
		}
		return
	}
	// Only a user who can already authenticate locally gets a reset link. A
	// Google-only or dormant user (no credential) is silently skipped — a reset
	// would otherwise mint a password for an account that never had one.
	if _, err := h.q.GetLocalCredentialByUserID(ctx, user.ID); err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			slog.Error("reset request: lookup credential", "err", err)
		}
		return
	}

	token, tokenHash, err := GenerateToken()
	if err != nil {
		slog.Error("reset request: generate token", "err", err)
		return
	}
	expiresAt := time.Now().Add(passwordResetTTL)
	if _, err := h.q.CreatePasswordResetToken(ctx, db.CreatePasswordResetTokenParams{
		TokenHash: tokenHash,
		UserID:    user.ID,
		ExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true},
	}); err != nil {
		slog.Error("reset request: create token", "err", err)
		return
	}

	resetURL := h.resetURL(token, user.Locale)
	h.dispatch(func() {
		// The request goroutine is gone; use a fresh bounded context, not r's.
		sctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := h.sendResetEmail(sctx, user, resetURL, expiresAt); err != nil {
			slog.Error("reset request: send email", "err", err)
		}
	})
}

type localResetPreviewResp struct {
	// Email is the address the token resets. The holder already proved possession
	// of the link, so echoing the bound email helps the screen, it doesn't leak.
	Email string `json:"email"`
}

// handleLocalResetPreview resolves a reset link for the set screen WITHOUT
// consuming it: it returns the bound email when the hashed token names a
// still-valid (unused, unexpired) reset token, else the same generic 409 the set
// path uses. A read-only GET, so reloading the form does not burn the single-use
// link — consumption happens only at POST.
func (h *Handlers) handleLocalResetPreview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	token := r.URL.Query().Get("token")
	if token == "" {
		httperr.Write(w, http.StatusBadRequest, httperr.CodeValidation, map[string]any{
			"field": "token", "rule": "required",
		})
		return
	}

	row, err := h.q.GetPasswordResetToken(ctx, HashToken(token))
	if err != nil || !resetTokenUsable(row) {
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			slog.Error("reset preview: lookup", "err", err)
			httperr.Write(w, http.StatusInternalServerError, httperr.CodeInternal, nil)
			return
		}
		httperr.Write(w, http.StatusConflict, httperr.CodeResetLinkNoLongerValid, nil)
		return
	}

	user, err := h.q.GetUserByID(ctx, row.UserID)
	if err != nil {
		slog.Error("reset preview: lookup user", "err", err)
		httperr.Write(w, http.StatusInternalServerError, httperr.CodeInternal, nil)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(localResetPreviewResp{Email: user.Email})
}

type localResetSetReq struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

// handleLocalResetSet completes a reset: it validates the new password, then in
// one transaction consumes the token, replaces the credential, and revokes the
// member's other sessions, finally minting a fresh session so the member lands
// straight in the app. The conditional consume (used_at IS NULL AND not expired)
// is the single-use guard — under a double submit the second UPDATE matches zero
// rows after the first commits, so a link resets the password exactly once. A
// mid-transaction failure rolls the consume back, so the link is never burned
// without a password change to show for it.
func (h *Handlers) handleLocalResetSet(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req localResetSetReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httperr.Write(w, http.StatusBadRequest, httperr.CodeInvalidJSONBody, nil)
		return
	}
	if req.Token == "" {
		httperr.Write(w, http.StatusBadRequest, httperr.CodeValidation, map[string]any{
			"field": "token", "rule": "required",
		})
		return
	}
	if err := ValidatePasswordPolicy(req.Password); err != nil {
		httperr.Write(w, http.StatusBadRequest, httperr.CodeWeakPassword, map[string]any{
			"reason": weakPasswordReason(err),
		})
		return
	}

	hash, err := HashPassword(req.Password)
	if err != nil {
		slog.Error("reset set: hash password", "err", err)
		httperr.Write(w, http.StatusInternalServerError, httperr.CodeInternal, nil)
		return
	}

	userID, err := h.commitPasswordReset(ctx, HashToken(req.Token), hash)
	switch {
	case errors.Is(err, errResetTokenInvalid):
		httperr.Write(w, http.StatusConflict, httperr.CodeResetLinkNoLongerValid, nil)
		return
	case err != nil:
		slog.Error("reset set: commit", "err", err)
		httperr.Write(w, http.StatusInternalServerError, httperr.CodeInternal, nil)
		return
	}

	user, err := h.q.GetUserByID(ctx, userID)
	if err != nil {
		slog.Error("reset set: lookup user", "err", err)
		httperr.Write(w, http.StatusInternalServerError, httperr.CodeInternal, nil)
		return
	}
	// A fresh password clears any login backoff so the member isn't throttled by
	// the failures that drove them to reset in the first place.
	h.limiter.recordSuccess("email:"+user.Email, "reset:"+user.Email)

	// All prior sessions were revoked in the transaction; mint the one new session
	// so the member is signed in straight away.
	if err := h.IssueSession(ctx, w, user.ID, r.UserAgent()); err != nil {
		slog.Error("reset set: issue session", "err", err)
		httperr.Write(w, http.StatusInternalServerError, httperr.CodeInternal, nil)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

var errResetTokenInvalid = errors.New("reset token not valid")

// commitPasswordReset runs the reset as a single transaction: atomically consume
// the token, replace the local credential, revoke EVERY existing session for the
// user (boot any attacker before the new one is minted), and invalidate the
// user's other outstanding reset tokens. Returns errResetTokenInvalid when the
// conditional consume matches nothing; that rolls everything back.
func (h *Handlers) commitPasswordReset(ctx context.Context, tokenHash, passwordHash string) (uuid.UUID, error) {
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after a successful Commit

	qtx := h.q.WithTx(tx)

	row, err := qtx.ConsumePasswordResetToken(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, errResetTokenInvalid
		}
		return uuid.Nil, err
	}

	if _, err := qtx.UpsertLocalCredential(ctx, db.UpsertLocalCredentialParams{
		UserID:       row.UserID,
		PasswordHash: passwordHash,
	}); err != nil {
		return uuid.Nil, err
	}

	// Revoke other sessions before minting the new one (done after commit) so a
	// reset always ends with exactly one live session — the one we issue.
	if err := qtx.DeleteSessionsForUser(ctx, row.UserID); err != nil {
		return uuid.Nil, err
	}

	// Burn any sibling reset links (e.g. a double request) so only this one ever
	// took effect.
	if err := qtx.DeletePasswordResetTokensForUser(ctx, row.UserID); err != nil {
		return uuid.Nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, err
	}
	return row.UserID, nil
}

// resetTokenUsable reports whether a reset token row is still unused and within
// its TTL — the read-only check the preview uses. The POST set does not rely on
// it: there the conditional UPDATE re-checks both facts atomically.
func resetTokenUsable(row db.PasswordResetToken) bool {
	if row.UsedAt.Valid {
		return false
	}
	return row.ExpiresAt.Valid && row.ExpiresAt.Time.After(time.Now())
}

// resetURL builds the link a member follows to set a new password. Always a
// method-neutral SPA route (/reset?token=…); the inviter-locale convention rides
// along as ?lng= (ADR-0035) so the email and the screen speak the same language.
func (h *Handlers) resetURL(token, locale string) string {
	return h.frontendURL + "/reset?token=" + token + "&lng=" + locale
}

func (h *Handlers) sendResetEmail(ctx context.Context, user db.User, resetURL string, expiresAt time.Time) error {
	c := localizedEmail(passwordResetCatalog, user.Locale)
	expires := localizedDate(expiresAt, user.Locale, user.TimeZone)
	subject := c.subject
	greeting := fmt.Sprintf(c.greeting, user.DisplayName)
	greetingHTML := fmt.Sprintf(c.greeting, htmlEscape(user.DisplayName))
	expiry := fmt.Sprintf(c.expiry, expires)

	html := fmt.Sprintf(`<p>%s</p>
<p>%s</p>
<p><a href="%s">%s</a></p>
<p>%s</p>
<p>%s</p>`, greetingHTML, c.body, resetURL, c.linkText, expiry, c.ignore)
	text := fmt.Sprintf("%s\n\n%s\n\n%s:\n%s\n\n%s\n\n%s\n",
		greeting, c.body, c.linkText, resetURL, expiry, c.ignore)

	return h.mailer.Send(ctx, email.Message{
		To:      user.Email,
		Subject: subject,
		HTML:    html,
		Text:    text,
	})
}
