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
		GoogleSub:        &c.Sub,
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

// onboardingInvite is a joinable Household row on the gate — one per distinct
// Household the verified email has a pending invitation to (ADR-0038).
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
	// FoundingDisabled mirrors the operator's FOUNDING_DISABLED flag (#302) so
	// the gate can hide/relabel the founder affordance before it's ever
	// clicked, rather than letting a zero-invite stranger pick an option that
	// then dead-ends on the commit. Invite-based joining is unaffected.
	FoundingDisabled bool `json:"founding_disabled"`
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

	rows, err := h.q.ListPendingInvitationsForEmail(r.Context(), hs.Email)
	if err != nil {
		slog.Error("list pending invitations", "err", err)
		httperr.Write(w, http.StatusInternalServerError, httperr.CodeInternal, nil)
		return
	}
	// A clicked `?invite=` link pre-highlights its Household — a hint, not the
	// decision. Resolve the hinted invitation to its Household (it may itself
	// have been deduped out in favour of a more recent invite to the same
	// Household, so match on Household, not the exact invitation id).
	var hintHousehold *uuid.UUID
	if hs.HintInvitationID != nil {
		if invite, err := h.q.GetInvitationByID(r.Context(), *hs.HintInvitationID); err == nil {
			hintHousehold = &invite.HouseholdID
		}
	}

	invites := make([]onboardingInvite, 0, len(rows))
	for _, row := range rows {
		invites = append(invites, onboardingInvite{
			InvitationID:  row.InvitationID,
			HouseholdID:   row.HouseholdID,
			HouseholdName: row.HouseholdName,
			InviterName:   row.InviterName,
			Hint:          hintHousehold != nil && *hintHousehold == row.HouseholdID,
		})
	}

	resp := onboardingOptionsResponse{
		Email:            hs.Email,
		DisplayName:      hs.DisplayName,
		SuggestedName:    hs.DisplayName + "'s Household",
		Invitations:      invites,
		FoundingDisabled: h.foundingDisabled,
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

type onboardingChoiceReq struct {
	// Exactly one of Found / Join is expected. Found commits the deliberate
	// founder path (optional DisplayName household-name override); Join accepts
	// a pending invitation identified by InvitationID (re-validated server-side
	// against the handshake's verified email).
	Found        bool       `json:"found"`
	DisplayName  *string    `json:"display_name"`
	Join         bool       `json:"join"`
	InvitationID *uuid.UUID `json:"invitation_id"`
}

// handleOnboardingChoice commits the gate decision: it creates the account from
// the handshake claims (founder → new Household; join → the invited Household
// after a TOCTOU re-validation), issues the real session, and deletes the
// handshake. The client's claim of which invitation is never trusted — the
// chosen invite is re-checked against the verified email at commit (ADR-0038).
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

	if req.Join {
		h.commitJoin(w, r, hs, req.InvitationID)
		return
	}
	if !req.Found {
		// Neither a join nor a found — a malformed choice.
		httperr.Write(w, http.StatusBadRequest, httperr.CodeValidation, map[string]any{
			"field": "found",
			"rule":  "required",
		})
		return
	}
	if h.foundingDisabled {
		// The operator has frozen this instance's household population (#302).
		// Applies uniformly to Google and local: both funnel through this one
		// commit path. Invite-based joining (the req.Join branch above) is
		// untouched.
		httperr.Write(w, http.StatusForbidden, httperr.CodeFoundingDisabled, nil)
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

	// A credential-bearing handshake is a local founder (ADR-0039): create the
	// User with no google_sub and a local_credentials row from the stashed hash.
	// Otherwise it is the Google path. Both create the Household + User from the
	// handshake claims and fire the best-effort welcome email.
	var user db.User
	var err error
	if hs.PasswordHash != nil {
		user, err = h.createLocalFounder(r.Context(), hs, householdName)
	} else {
		claims := &googleClaims{
			Sub:     stringOrEmpty(hs.GoogleSub),
			Email:   hs.Email,
			Name:    hs.DisplayName,
			Picture: stringOrEmpty(hs.PictureUrl),
		}
		user, err = h.createFounder(r.Context(), claims, hs.SeedLocale, householdName)
	}
	if err != nil {
		slog.Error("onboarding found household", "err", err)
		httperr.Write(w, http.StatusInternalServerError, httperr.CodeInternal, nil)
		return
	}
	h.finishOnboarding(w, r, hs.ID, user.ID)
}

// createLocalFounder mints a new Household + local (google_sub-less) User from a
// credential-bearing handshake, then writes the local_credentials row from the
// hash the register step stashed (ADR-0039). The hash moves handshake-row →
// credentials-row inside the same instance and is never re-derived from a
// plaintext we no longer hold. Mirrors createFounder's best-effort welcome email.
func (h *Handlers) createLocalFounder(ctx context.Context, hs db.OnboardingHandshake, householdName string) (db.User, error) {
	if householdName == "" {
		householdName = hs.DisplayName + "'s Household"
	}
	household, err := h.q.CreateHousehold(ctx, db.CreateHouseholdParams{
		DisplayName:       householdName,
		ReportingCurrency: "IDR",
	})
	if err != nil {
		return db.User{}, err
	}
	user, err := h.q.CreateLocalUser(ctx, db.CreateLocalUserParams{
		HouseholdID: household.ID,
		DisplayName: hs.DisplayName,
		Email:       hs.Email,
		Locale:      hs.SeedLocale,
		TimeZone:    "Asia/Jakarta",
		PictureUrl:  nil,
		CreatedBy:   nil,
	})
	if err != nil {
		return db.User{}, err
	}
	if _, err := h.q.UpsertLocalCredential(ctx, db.UpsertLocalCredentialParams{
		UserID:       user.ID,
		PasswordHash: *hs.PasswordHash,
	}); err != nil {
		return db.User{}, err
	}
	if err := h.sendWelcomeEmail(ctx, user); err != nil {
		slog.Error("send welcome email", "err", err, "user_id", user.ID)
	}
	return user, nil
}

// commitJoin accepts a pending invitation at the gate. It re-validates the
// chosen invitation server-side against the handshake's verified email (TOCTOU,
// ADR-0038) — never trusting the client's id alone. If the invite is no longer
// valid (used/expired between the gate's read and this write, or addressed to a
// different email) it answers 409 so the SPA refreshes the gate; otherwise it
// binds the new User to that Household, marks the invitation used, and finishes.
func (h *Handlers) commitJoin(w http.ResponseWriter, r *http.Request, hs db.OnboardingHandshake, invitationID *uuid.UUID) {
	if invitationID == nil {
		httperr.Write(w, http.StatusBadRequest, httperr.CodeValidation, map[string]any{
			"field": "invitation_id",
			"rule":  "required",
		})
		return
	}
	invite, err := h.q.GetValidInvitationForEmail(r.Context(), db.GetValidInvitationForEmailParams{
		ID:           *invitationID,
		InvitedEmail: hs.Email,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Stale/used/expired or wrong email — bounce to a refreshed gate.
			httperr.Write(w, http.StatusConflict, httperr.CodeInvitationNoLongerValid, nil)
			return
		}
		slog.Error("revalidate invitation", "err", err)
		httperr.Write(w, http.StatusInternalServerError, httperr.CodeInternal, nil)
		return
	}

	user, err := h.q.CreateUser(r.Context(), db.CreateUserParams{
		HouseholdID: invite.HouseholdID,
		DisplayName: hs.DisplayName,
		Email:       hs.Email,
		GoogleSub:   stringOrEmpty(hs.GoogleSub),
		Locale:      hs.SeedLocale,
		TimeZone:    "Asia/Jakarta",
		PictureUrl:  hs.PictureUrl,
		CreatedBy:   &invite.CreatedBy,
	})
	if err != nil {
		slog.Error("onboarding join household", "err", err)
		httperr.Write(w, http.StatusInternalServerError, httperr.CodeInternal, nil)
		return
	}
	if err := h.q.MarkInvitationUsed(r.Context(), invite.ID); err != nil {
		slog.Error("mark invitation used", "err", err)
		httperr.Write(w, http.StatusInternalServerError, httperr.CodeInternal, nil)
		return
	}
	h.finishOnboarding(w, r, hs.ID, user.ID)
}

// finishOnboarding consumes the handshake and issues the real session, shared
// by the founder and join commits. The handshake is deleted before the session
// is issued so an interrupted commit can't leave a reusable gate token.
func (h *Handlers) finishOnboarding(w http.ResponseWriter, r *http.Request, handshakeID string, userID uuid.UUID) {
	if err := h.q.DeleteOnboardingHandshake(r.Context(), handshakeID); err != nil {
		slog.Error("delete onboarding handshake", "err", err)
	}
	h.clearOnboardingCookie(w)

	if err := h.IssueSession(r.Context(), w, userID, r.UserAgent()); err != nil {
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
