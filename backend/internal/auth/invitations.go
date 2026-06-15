package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
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

	token, err := randomInvitationToken()
	if err != nil {
		slog.Error("generate invitation token", "err", err)
		httperr.Write(w, http.StatusInternalServerError, httperr.CodeInternal, nil)
		return
	}

	expiresAt := time.Now().Add(invitationTTL)

	invite, err := h.q.CreateInvitation(ctx, db.CreateInvitationParams{
		HouseholdID:  inviter.HouseholdID,
		InvitedEmail: req.Email,
		Token:        token,
		CreatedBy:    inviter.ID,
		ExpiresAt:    pgtype.Timestamptz{Time: expiresAt, Valid: true},
	})
	if err != nil {
		slog.Error("create invitation", "err", err)
		httperr.Write(w, http.StatusInternalServerError, httperr.CodeInternal, nil)
		return
	}

	acceptURL := h.backendURL + "/api/auth/google/start?invite=" + token

	if err := h.sendInvitationEmail(ctx, inviter, household, invite, acceptURL); err != nil {
		// Email delivery is best-effort: log the error but still return the
		// invitation. The inviter can share the accept URL manually if email
		// failed to deliver.
		slog.Error("send invitation email", "err", err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(createInvitationResp{
		ID:           invite.ID,
		InvitedEmail: invite.InvitedEmail,
		ExpiresAt:    expiresAt,
		AcceptURL:    acceptURL,
	})
}

func (h *Handlers) sendInvitationEmail(ctx context.Context, inviter db.User, household db.Household, invite db.HouseholdInvitation, acceptURL string) error {
	subject := fmt.Sprintf("%s invited you to Balances", inviter.DisplayName)
	html := fmt.Sprintf(`<p>Hi,</p>
<p>%s has invited you to join their Balances household <strong>%s</strong>.</p>
<p><a href="%s">Click here to accept the invitation</a></p>
<p>The link expires on %s. If you weren't expecting this email, you can safely ignore it.</p>`,
		htmlEscape(inviter.DisplayName), htmlEscape(household.DisplayName), acceptURL, invite.ExpiresAt.Time.Format(time.RFC1123))
	text := fmt.Sprintf(`Hi,

%s has invited you to join their Balances household "%s".

Accept the invitation here:
%s

The link expires on %s. If you weren't expecting this email, you can safely ignore it.
`, inviter.DisplayName, household.DisplayName, acceptURL, invite.ExpiresAt.Time.Format(time.RFC1123))

	return h.mailer.Send(ctx, email.Message{
		To:      invite.InvitedEmail,
		Subject: subject,
		HTML:    html,
		Text:    text,
	})
}

func randomInvitationToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
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
