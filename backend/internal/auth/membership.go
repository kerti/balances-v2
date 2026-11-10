package auth

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/kerti/balances-v2/backend/internal/db"
	"github.com/kerti/balances-v2/backend/internal/httperr"
	"github.com/kerti/balances-v2/backend/internal/identity"
)

// Settings → Members (#847): one read that feeds the members table and its
// pending-invites rows, and the revoke that retires an unused invite. Both are
// open to any member — the household is a peer group (ADR-0017), and any member
// may invite, so any member may take an invite back. Reactivation, the one
// founder-only action in that table, keeps its own routes (reactivation.go).

type membershipMember struct {
	ID uuid.UUID `json:"id"`
	// DisplayName is the account's name; Nickname the member's own override. The
	// table shows the nickname when there is one, as the rest of the app does.
	DisplayName string  `json:"display_name"`
	Nickname    *string `json:"nickname"`
	Email       string  `json:"email"`
	// JoinedAt is the account's creation time.
	JoinedAt time.Time `json:"joined_at"`
	// Dormant: no google_sub and no local credential (INV-AUTH-15) — a member who
	// owns data but cannot sign in yet, e.g. a local member after a restore.
	Dormant bool `json:"dormant"`
}

type membershipInvitation struct {
	ID           uuid.UUID `json:"id"`
	InvitedEmail string    `json:"invited_email"`
	SentAt       time.Time `json:"sent_at"`
	ExpiresAt    time.Time `json:"expires_at"`
}

type membershipResp struct {
	Members     []membershipMember     `json:"members"`
	Invitations []membershipInvitation `json:"invitations"`
}

// handleGetMembership lists the caller's household's members and its pending,
// unexpired invitations. Both queries filter on the caller's household_id in SQL;
// the token hash is never selected, so no link can be rebuilt from the response.
func (h *Handlers) handleGetMembership(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, ok := identity.UserFromContext(ctx)
	if !ok {
		httperr.Write(w, http.StatusUnauthorized, httperr.CodeUnauthorized, nil)
		return
	}

	roster, err := h.q.ListHouseholdRoster(ctx, user.HouseholdID)
	if err != nil {
		slog.Error("membership: list roster", "err", err)
		httperr.Write(w, http.StatusInternalServerError, httperr.CodeInternal, nil)
		return
	}
	pending, err := h.q.ListPendingInvitationsByHousehold(ctx, user.HouseholdID)
	if err != nil {
		slog.Error("membership: list pending invitations", "err", err)
		httperr.Write(w, http.StatusInternalServerError, httperr.CodeInternal, nil)
		return
	}

	out := membershipResp{
		Members:     make([]membershipMember, 0, len(roster)),
		Invitations: make([]membershipInvitation, 0, len(pending)),
	}
	for _, m := range roster {
		out.Members = append(out.Members, membershipMember{
			ID:          m.ID,
			DisplayName: m.DisplayName,
			Nickname:    m.Nickname,
			Email:       m.Email,
			JoinedAt:    m.CreatedAt.Time,
			Dormant:     m.Dormant,
		})
	}
	for _, inv := range pending {
		out.Invitations = append(out.Invitations, membershipInvitation{
			ID:           inv.ID,
			InvitedEmail: inv.InvitedEmail,
			SentAt:       inv.CreatedAt.Time,
			ExpiresAt:    inv.ExpiresAt.Time,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

// handleRevokeInvitation retires an unused invitation by deleting its row. The
// link's token hash goes with it, so the preview and accept paths answer as for
// any unknown link (409 INVITATION_NO_LONGER_VALID) and the Google onboarding
// gate no longer offers the household. The DELETE is scoped by household_id and
// used_at IS NULL in SQL: another household's invitation, an already-accepted
// one, a missing one, and one revoked a moment ago are all the same 404 — no
// cross-tenant existence leak, and re-sending is the recovery (ADR-0058 §3, no Undo).
func (h *Handlers) handleRevokeInvitation(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, ok := identity.UserFromContext(ctx)
	if !ok {
		httperr.Write(w, http.StatusUnauthorized, httperr.CodeUnauthorized, nil)
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httperr.Write(w, http.StatusNotFound, httperr.CodeNotFound, nil)
		return
	}

	n, err := h.q.RevokeInvitation(ctx, db.RevokeInvitationParams{ID: id, HouseholdID: user.HouseholdID})
	if err != nil {
		slog.Error("revoke invitation", "err", err)
		httperr.Write(w, http.StatusInternalServerError, httperr.CodeInternal, nil)
		return
	}
	if n == 0 {
		httperr.Write(w, http.StatusNotFound, httperr.CodeNotFound, nil)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
