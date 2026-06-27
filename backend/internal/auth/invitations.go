package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/kerti/balances-v2/backend/internal/db"
	"github.com/kerti/balances-v2/backend/internal/email"
	"github.com/kerti/balances-v2/backend/internal/httperr"
)

type createInvitationReq struct {
	Email string `json:"email" validate:"required,email"`
}

type createInvitationResp struct {
	ID           uuid.UUID `json:"id"`
	InvitedEmail string    `json:"invited_email"`
	ExpiresAt    time.Time `json:"expires_at"`
	AcceptURL    string    `json:"accept_url"`
	// EmailSent reports whether the best-effort invitation email went out, so the
	// inviter can be nudged to share the AcceptURL manually when it didn't. False
	// only when mailer.Send errored (e.g. a misconfigured sender); with email
	// disabled the NoopMailer succeeds and this stays true — the copy-link panel
	// is the designed affordance there (INV-NOTIFICATIONS-10/11).
	EmailSent bool `json:"email_sent"`
}

func (h *Handlers) handleCreateInvitation(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	inviter, ok := UserFromContext(ctx)
	if !ok {
		httperr.Write(w, http.StatusUnauthorized, httperr.CodeUnauthorized, nil)
		return
	}

	var req createInvitationReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httperr.Write(w, http.StatusBadRequest, httperr.CodeInvalidJSONBody, nil)
		return
	}
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	if err := h.validate.Struct(&req); err != nil {
		httperr.WriteValidation(w, err)
		return
	}
	if req.Email == strings.ToLower(inviter.Email) {
		httperr.Write(w, http.StatusBadRequest, httperr.CodeCannotInviteSelf, nil)
		return
	}

	household, err := h.q.GetHouseholdByID(ctx, inviter.HouseholdID)
	if err != nil {
		slog.Error("lookup household", "err", err)
		httperr.Write(w, http.StatusInternalServerError, httperr.CodeInternal, nil)
		return
	}

	// One random token, two representations (ADR-0039/#281): the plaintext rides
	// the link, only the hash is stored. The plaintext is never persisted or
	// logged below.
	token, tokenHash, err := GenerateToken()
	if err != nil {
		slog.Error("generate invitation token", "err", err)
		httperr.Write(w, http.StatusInternalServerError, httperr.CodeInternal, nil)
		return
	}

	expiresAt := time.Now().Add(invitationTTL)

	invite, err := h.q.CreateInvitation(ctx, db.CreateInvitationParams{
		HouseholdID:  inviter.HouseholdID,
		InvitedEmail: req.Email,
		TokenHash:    tokenHash,
		CreatedBy:    inviter.ID,
		ExpiresAt:    pgtype.Timestamptz{Time: expiresAt, Valid: true},
	})
	if err != nil {
		slog.Error("create invitation", "err", err)
		httperr.Write(w, http.StatusInternalServerError, httperr.CodeInternal, nil)
		return
	}

	acceptURL := h.inviteAcceptURL(token, inviter.Locale)

	emailSent := true
	if err := h.sendInvitationEmail(ctx, inviter, household, invite, acceptURL); err != nil {
		// Email delivery is best-effort: log the error but still return the
		// invitation. The inviter can share the accept URL manually if email
		// failed to deliver — emailSent=false lets the UI nudge them to.
		slog.Error("send invitation email", "err", err)
		emailSent = false
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(createInvitationResp{
		ID:           invite.ID,
		InvitedEmail: invite.InvitedEmail,
		ExpiresAt:    expiresAt,
		AcceptURL:    acceptURL,
		EmailSent:    emailSent,
	})
}

func (h *Handlers) sendInvitationEmail(ctx context.Context, inviter db.User, household db.Household, invite db.HouseholdInvitation, acceptURL string) error {
	c := localizedEmail(invitationCatalog, inviter.Locale)
	expires := invite.ExpiresAt.Time.Format(time.RFC1123)
	subject := fmt.Sprintf(c.subject, inviter.DisplayName)
	expiry := fmt.Sprintf(c.expiry, expires)

	bodyHTML := fmt.Sprintf(c.body,
		htmlEscape(inviter.DisplayName), "<strong>"+htmlEscape(household.DisplayName)+"</strong>")
	bodyText := fmt.Sprintf(c.body, inviter.DisplayName, `"`+household.DisplayName+`"`)

	html := fmt.Sprintf(`<p>%s</p>
<p>%s</p>
<p><a href="%s">%s</a></p>
<p>%s</p>`, c.greeting, bodyHTML, acceptURL, c.linkText, expiry)
	text := fmt.Sprintf("%s\n\n%s\n\n%s:\n%s\n\n%s\n",
		c.greeting, bodyText, c.linkText, acceptURL, expiry)

	return h.mailer.Send(ctx, email.Message{
		To:      invite.InvitedEmail,
		Subject: subject,
		HTML:    html,
		Text:    text,
	})
}

// inviteAcceptURL builds the link an invitee follows. The routing depends on
// which identity providers the instance runs (ADR-0039, decision in #281):
//
//   - local enabled → a method-neutral SPA accept route (/accept?token=…). The
//     screen resolves the token and offers a set-password form (and, if Google
//     is also live, a "continue with Google" button). A local-only self-host has
//     no Google /start to send the invitee to, so this is the only workable
//     landing.
//   - local disabled (the hosted, Google-only posture) → the unchanged backend
//     /start URL. Possession of the link is still only a hint there; Google's
//     email-match at the onboarding gate remains the proof (#159, ADR-0038).
//
// Either way the inviter's locale rides the link (ADR-0035) so the invitee
// inherits the household language by default; it is always a supported value
// (users.locale CHECK) and changeable later in Settings.
func (h *Handlers) inviteAcceptURL(token, locale string) string {
	if h.localEnabled {
		return h.frontendURL + "/accept?token=" + token + "&lng=" + locale
	}
	return h.backendURL + "/api/auth/google/start?invite=" + token + "&lng=" + locale
}

// htmlEscape is intentionally minimal — display_name comes from Google's
// OAuth claims (validated upstream) and household_name is user-controlled
// only within their own Household, so the threat surface is narrow.
func htmlEscape(s string) string {
	r := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
	)
	return r.Replace(s)
}
