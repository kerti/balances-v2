package auth

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/kerti/balances-v2/backend/internal/db"
	"github.com/kerti/balances-v2/backend/internal/httperr"
)

// Founder-assisted in-app member reactivation (ADR-0039/#283). The no-mail home
// deploy (EMAIL_ENABLED=false) needs a way to bring a member back in without the
// operator CLI: after a restore, local members land DORMANT (a users row with no
// google_sub and no local_credentials — see ADR-0036/0039), unable to sign in.
// The founder reactivates them from the UI, minting a per-member one-time
// set-password link shown once to relay out-of-band — the same shape as the
// copy-link invite panel.
//
// This is deliberately NOT a "reset anyone" power. It is scoped to reactivation:
//
//   - only the FOUNDER (the lineage root, created_by IS NULL) may act — a peer
//     member cannot reactivate (403). This preserves ADR-0017's peer model:
//     reactivating a credential-less row is operator bring-up, not a standing
//     privilege over peers.
//   - it acts ONLY on a DORMANT member (no google_sub, no local_credentials).
//     A member who already holds a credential is refused (409) — resetting an
//     active account's password is impersonation; the operator CLI (#284) is the
//     escape hatch for that.
//
// The minted link reuses the emailed-reset set-password path verbatim: the
// returned URL is the same /reset?token=… route, so the member follows it,
// previews the bound email, sets a password (UpsertLocalCredential creates the
// dormant member's first credential), and is signed in — no new consume endpoint.
// The token rides the shared primitive (token.go): a single-use, short-TTL,
// ≥256-bit random token stored only as a SHA-256 hash. Unlike the emailed reset,
// the plaintext is returned to the founder to relay by hand (never emailed,
// never logged), so its TTL is the shared RelayTokenTTL (token.go) — the
// copy-link relay window, not the 1h email TTL.

// requireFounder resolves the authenticated caller and asserts they are the
// household founder (created_by IS NULL). Returns false and writes the response
// on any failure — an unauthenticated request is 401, a non-founder member 403 —
// so callers just `return` on !ok.
func (h *Handlers) requireFounder(w http.ResponseWriter, r *http.Request) (db.User, bool) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		httperr.Write(w, http.StatusUnauthorized, httperr.CodeUnauthorized, nil)
		return db.User{}, false
	}
	// The founder is the lineage root: the only member created with no creator
	// (createFounder passes CreatedBy: nil; every invited member carries one).
	if user.CreatedBy != nil {
		httperr.Write(w, http.StatusForbidden, httperr.CodeForbidden, nil)
		return db.User{}, false
	}
	return user, true
}

type dormantMember struct {
	ID          uuid.UUID `json:"id"`
	DisplayName string    `json:"display_name"`
	Email       string    `json:"email"`
}

// handleListDormantMembers returns the founder's reactivatable members — the
// dormant ones (no google_sub, no local_credentials) in their household. The
// founder is never in this list (they are reachable), so it is exactly the set of
// members awaiting a first credential, e.g. after a disaster-recovery restore.
func (h *Handlers) handleListDormantMembers(w http.ResponseWriter, r *http.Request) {
	founder, ok := h.requireFounder(w, r)
	if !ok {
		return
	}
	rows, err := h.q.ListDormantMembersByHousehold(r.Context(), founder.HouseholdID)
	if err != nil {
		slog.Error("reactivation: list dormant members", "err", err)
		httperr.Write(w, http.StatusInternalServerError, httperr.CodeInternal, nil)
		return
	}
	out := make([]dormantMember, 0, len(rows))
	for _, u := range rows {
		out = append(out, dormantMember{ID: u.ID, DisplayName: u.DisplayName, Email: u.Email})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

type reactivateMemberReq struct {
	UserID uuid.UUID `json:"user_id"`
}

type reactivateMemberResp struct {
	// Email is the reactivated member's address, echoed so the founder can confirm
	// which member the link is for before relaying it.
	Email string `json:"email"`
	// SetPasswordURL is the one-time link the founder relays out-of-band. It is the
	// plaintext token's only appearance — shown once, never stored, never logged.
	SetPasswordURL string `json:"set_password_url"`
	// ExpiresAt bounds the link's validity so the founder knows the relay window.
	ExpiresAt time.Time `json:"expires_at"`
}

// handleReactivateMember mints a one-time set-password link for a dormant member
// and returns it to the founder to relay. It refuses any target that is not a
// dormant member of the founder's household: unknown / other-household → 404,
// already-reachable (holds a credential) → 409. The link reuses the reset
// set-password route, so following it creates the member's first credential and
// signs them in.
func (h *Handlers) handleReactivateMember(w http.ResponseWriter, r *http.Request) {
	founder, ok := h.requireFounder(w, r)
	if !ok {
		return
	}
	ctx := r.Context()

	var req reactivateMemberReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httperr.Write(w, http.StatusBadRequest, httperr.CodeInvalidJSONBody, nil)
		return
	}
	if req.UserID == uuid.Nil {
		httperr.Write(w, http.StatusBadRequest, httperr.CodeValidation, map[string]any{
			"field": "user_id", "rule": "required",
		})
		return
	}

	target, err := h.q.GetUserByID(ctx, req.UserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httperr.Write(w, http.StatusNotFound, httperr.CodeNotFound, nil)
			return
		}
		slog.Error("reactivation: lookup target", "err", err)
		httperr.Write(w, http.StatusInternalServerError, httperr.CodeInternal, nil)
		return
	}
	// Household scoping: a founder may only reactivate members of their own
	// household. A cross-household id is indistinguishable from a non-existent one
	// (404) — no cross-tenant existence leak.
	if target.HouseholdID != founder.HouseholdID {
		httperr.Write(w, http.StatusNotFound, httperr.CodeNotFound, nil)
		return
	}
	// Dormancy guard. A Google member is reachable via Google; refuse rather than
	// silently minting them a second (local) credential — account-linking is out of
	// scope (ADR-0039). This branch also covers the founder targeting themselves.
	if target.GoogleSub != nil {
		httperr.Write(w, http.StatusConflict, httperr.CodeMemberNotDormant, nil)
		return
	}
	// The core refusal: a member who already holds a local credential is active,
	// not dormant. Re-setting their password in-app is impersonation and is refused
	// here (the operator CLI is the escape hatch for an active member) — the peer
	// model is preserved (ADR-0017/0039).
	if _, err := h.q.GetLocalCredentialByUserID(ctx, target.ID); err == nil {
		httperr.Write(w, http.StatusConflict, httperr.CodeMemberNotDormant, nil)
		return
	} else if !errors.Is(err, pgx.ErrNoRows) {
		slog.Error("reactivation: lookup credential", "err", err)
		httperr.Write(w, http.StatusInternalServerError, httperr.CodeInternal, nil)
		return
	}

	// One random token, two representations (token.go): the plaintext rides the
	// returned link, only the hash is stored. The plaintext is never persisted or
	// logged below. Bound to the dormant member's user_id, it is consumed by the
	// shared reset set-password path.
	token, tokenHash, err := GenerateToken()
	if err != nil {
		slog.Error("reactivation: generate token", "err", err)
		httperr.Write(w, http.StatusInternalServerError, httperr.CodeInternal, nil)
		return
	}
	expiresAt := time.Now().Add(RelayTokenTTL)
	if _, err := h.q.CreatePasswordResetToken(ctx, db.CreatePasswordResetTokenParams{
		TokenHash: tokenHash,
		UserID:    target.ID,
		ExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true},
	}); err != nil {
		slog.Error("reactivation: create token", "err", err)
		httperr.Write(w, http.StatusInternalServerError, httperr.CodeInternal, nil)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(reactivateMemberResp{
		Email:          target.Email,
		SetPasswordURL: h.resetURL(token, target.Locale),
		ExpiresAt:      expiresAt,
	})
}
