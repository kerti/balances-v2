package auth

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/kerti/balances-v2/backend/internal/db"
	"github.com/kerti/balances-v2/backend/internal/httperr"
)

const (
	// onboardingCookieName carries the opaque handshake token through the gate.
	// Like the session cookie it is httpOnly/Lax/Secure-in-prod and never read
	// by JS — the SPA proves it holds the handshake by the cookie alone.
	onboardingCookieName = "onboarding"

	// onboardingHandshakeTTL bounds how long a verified-but-unaccounted identity
	// may sit at the gate before it must re-authenticate (ADR-0038, ≈15 min).
	onboardingHandshakeTTL = 15 * time.Minute

	// maxHouseholdNameLen caps the optional founder-supplied household name. The
	// column is unbounded `text`; this is a sanity guard, not a domain rule.
	maxHouseholdNameLen = 60
)

// beginOnboarding records the transient onboarding handshake for a brand-new
// Google identity and writes the handshake cookie. It issues no session and
// creates no users/households row — the account is born only when the person
// commits a choice at the gate (ADR-0038). hintInvitationID pre-highlights a
// clicked invite link's Household at the gate (#268); nil in the founder slice.
func (h *Handlers) beginOnboarding(ctx context.Context, w http.ResponseWriter, c *googleClaims, seedLocale string, hintInvitationID *uuid.UUID) error {
	token, err := randomSessionID()
	if err != nil {
		return err
	}
	expiresAt := time.Now().Add(onboardingHandshakeTTL)
	if _, err := h.q.CreateOnboardingHandshake(ctx, db.CreateOnboardingHandshakeParams{
		ID:               token,
		GoogleSub:        c.Sub,
		Email:            c.Email,
		DisplayName:      c.Name,
		PictureUrl:       nullableString(c.Picture),
		SeedLocale:       seedLocale,
		HintInvitationID: hintInvitationID,
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

func (h *Handlers) clearOnboardingCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     onboardingCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

// onboardingHandshake reads + validates the handshake cookie, returning the
// row. A missing/expired/unknown handshake clears the stale cookie and is
// reported via the bool=false so callers answer 401 (back to sign-in).
func (h *Handlers) onboardingHandshake(w http.ResponseWriter, r *http.Request) (db.OnboardingHandshake, bool) {
	cookie, err := r.Cookie(onboardingCookieName)
	if err != nil || cookie.Value == "" {
		return db.OnboardingHandshake{}, false
	}
	hs, err := h.q.GetOnboardingHandshake(r.Context(), cookie.Value)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			h.clearOnboardingCookie(w)
		} else {
			slog.Error("get onboarding handshake", "err", err)
		}
		return db.OnboardingHandshake{}, false
	}
	return hs, true
}

// onboardingInvite is a joinable Household row on the gate. Empty in the
// founder slice (#267); populated by the pending-invite lookup in #268.
type onboardingInvite struct {
	InvitationID  uuid.UUID `json:"invitation_id"`
	HouseholdID   uuid.UUID `json:"household_id"`
	HouseholdName string    `json:"household_name"`
	InviterName   string    `json:"inviter_name"`
	Hint          bool      `json:"hint"`
}

type onboardingOptionsResponse struct {
	Email         string             `json:"email"`
	DisplayName   string             `json:"display_name"`
	SuggestedName string             `json:"suggested_household_name"`
	Invitations   []onboardingInvite `json:"invitations"`
}

// handleOnboardingOptions returns what the gate can offer the holder of a valid
// handshake: the always-present founder option (the suggested household name)
// plus any joinable invitations (none in this slice). No session is required —
// the handshake cookie is the credential; a missing/expired one is 401.
func (h *Handlers) handleOnboardingOptions(w http.ResponseWriter, r *http.Request) {
	hs, ok := h.onboardingHandshake(w, r)
	if !ok {
		httperr.Write(w, http.StatusUnauthorized, httperr.CodeUnauthorized, nil)
		return
	}
	resp := onboardingOptionsResponse{
		Email:         hs.Email,
		DisplayName:   hs.DisplayName,
		SuggestedName: hs.DisplayName + "'s Household",
		Invitations:   []onboardingInvite{},
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

type onboardingChoiceReq struct {
	// Found commits the deliberate founder path. Join + InvitationID land in
	// the next slice (#268); this slice honours only the founder choice.
	Found       bool    `json:"found"`
	DisplayName *string `json:"display_name"`
}

// handleOnboardingChoice commits the gate decision: it creates the account from
// the handshake claims, issues the real session, and deletes the handshake.
// This slice (#267) implements only the founder branch.
func (h *Handlers) handleOnboardingChoice(w http.ResponseWriter, r *http.Request) {
	hs, ok := h.onboardingHandshake(w, r)
	if !ok {
		httperr.Write(w, http.StatusUnauthorized, httperr.CodeUnauthorized, nil)
		return
	}

	var req onboardingChoiceReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httperr.Write(w, http.StatusBadRequest, httperr.CodeInvalidJSONBody, nil)
		return
	}
	if !req.Found {
		// Only the founder choice exists in this slice; anything else is a
		// malformed request until the join path arrives (#268).
		httperr.Write(w, http.StatusBadRequest, httperr.CodeValidation, map[string]any{
			"field": "found",
			"rule":  "required",
		})
		return
	}

	householdName := ""
	if req.DisplayName != nil {
		if trimmed := strings.TrimSpace(*req.DisplayName); trimmed != "" {
			if utf8.RuneCountInString(trimmed) > maxHouseholdNameLen {
				httperr.Write(w, http.StatusBadRequest, httperr.CodeValidation, map[string]any{
					"field": "display_name",
					"rule":  "max",
				})
				return
			}
			householdName = trimmed
		}
	}

	claims := &googleClaims{
		Sub:     hs.GoogleSub,
		Email:   hs.Email,
		Name:    hs.DisplayName,
		Picture: stringOrEmpty(hs.PictureUrl),
	}
	user, err := h.createFounder(r.Context(), claims, hs.SeedLocale, householdName)
	if err != nil {
		slog.Error("onboarding found household", "err", err)
		httperr.Write(w, http.StatusInternalServerError, httperr.CodeInternal, nil)
		return
	}

	// The handshake has served its purpose; remove it before issuing the
	// session so an interrupted commit can't leave a reusable gate token.
	if err := h.q.DeleteOnboardingHandshake(r.Context(), hs.ID); err != nil {
		slog.Error("delete onboarding handshake", "err", err)
	}
	h.clearOnboardingCookie(w)

	if err := h.IssueSession(r.Context(), w, user.ID, r.UserAgent()); err != nil {
		slog.Error("issue session", "err", err)
		httperr.Write(w, http.StatusInternalServerError, httperr.CodeInternal, nil)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// stringOrEmpty dereferences an optional string to "" when nil — bridging the
// nullable picture_url column back to the non-pointer googleClaims field.
func stringOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
