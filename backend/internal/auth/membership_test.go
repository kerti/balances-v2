package auth

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/kerti/balances-v2/backend/internal/db"
	"github.com/kerti/balances-v2/backend/internal/httperr"
	"github.com/kerti/balances-v2/backend/internal/testutil"
)

// seedInvitationFor inserts an invitation into an arbitrary household and
// returns its id and plaintext token. mustSeedInvitation is pinned to the
// harness user's household; the isolation tests need to plant rows elsewhere.
func seedInvitationFor(t *testing.T, h *authHarness, inviter db.User, email string, expiresAt time.Time) (uuid.UUID, string) {
	t.Helper()
	token, tokenHash, err := GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	inv, err := h.q.CreateInvitation(context.Background(), db.CreateInvitationParams{
		HouseholdID:  inviter.HouseholdID,
		InvitedEmail: email,
		TokenHash:    tokenHash,
		CreatedBy:    inviter.ID,
		ExpiresAt:    pgtype.Timestamptz{Time: expiresAt, Valid: true},
	})
	if err != nil {
		t.Fatalf("CreateInvitation: %v", err)
	}
	return inv.ID, token
}

// countInvitations is a raw COUNT(*) on the table: the revoke tests assert on the
// row itself, not on a getter that might hide it behind an expiry or a filter.
func countInvitations(t *testing.T, h *authHarness, id uuid.UUID) int {
	t.Helper()
	var n int
	if err := h.pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM household_invitations WHERE id = $1`, id).Scan(&n); err != nil {
		t.Fatalf("count invitations: %v", err)
	}
	return n
}

// TestMembership_ListsMembersAndPendingInvites: the table's one read returns every
// live member (with dormant derived and soft-deleted rows skipped) and each pending,
// unexpired invitation — not an expired one, not an accepted one — and never a token.
//
// covers: INV-AUTH-31
func TestMembership_ListsMembersAndPendingInvites(t *testing.T) {
	h := newAuthHarness(t)
	active := h.seedLocalUser(t, "active@example.com", "the-existing-passphrase")
	dormant := h.seedDormantMember(t, "dormant@example.com")
	gone := h.seedDormantMember(t, "gone@example.com")
	if _, err := h.pool.Exec(context.Background(),
		`UPDATE users SET deleted_at = now() WHERE id = $1`, gone.ID); err != nil {
		t.Fatalf("soft-delete member: %v", err)
	}

	pendingID, _ := seedInvitationFor(t, h, h.user, "pending@example.com", time.Now().Add(time.Hour))
	seedInvitationFor(t, h, h.user, "expired@example.com", time.Now().Add(-time.Minute))
	_, usedToken := seedInvitationFor(t, h, h.user, "used@example.com", time.Now().Add(time.Hour))
	if _, err := h.pool.Exec(context.Background(),
		`UPDATE household_invitations SET used_at = now() WHERE token_hash = $1`, HashToken(usedToken)); err != nil {
		t.Fatalf("mark used: %v", err)
	}

	rec := h.do(t, http.MethodGet, "/household/membership", nil)
	requireStatus(t, rec, http.StatusOK)
	if strings.Contains(strings.ToLower(rec.Body.String()), "token") {
		t.Errorf("response mentions a token: %s", rec.Body.String())
	}
	resp := decodeBody[membershipResp](t, rec)

	byID := map[uuid.UUID]membershipMember{}
	for _, m := range resp.Members {
		byID[m.ID] = m
	}
	if len(resp.Members) != 3 {
		t.Fatalf("members = %d, want 3 (founder, active, dormant): %+v", len(resp.Members), resp.Members)
	}
	if _, ok := byID[gone.ID]; ok {
		t.Error("soft-deleted member was listed")
	}
	if resp.Members[0].ID != h.user.ID {
		t.Errorf("first member = %s, want the founder %s (oldest first)", resp.Members[0].ID, h.user.ID)
	}
	if byID[active.ID].Dormant {
		t.Error("credentialed member reported dormant")
	}
	if !byID[dormant.ID].Dormant {
		t.Error("credential-less member not reported dormant")
	}
	if byID[active.ID].JoinedAt.IsZero() {
		t.Error("joined_at is zero")
	}

	if len(resp.Invitations) != 1 || resp.Invitations[0].ID != pendingID {
		t.Fatalf("invitations = %+v, want only the pending one %s", resp.Invitations, pendingID)
	}
	if got := resp.Invitations[0]; got.InvitedEmail != "pending@example.com" || !got.ExpiresAt.After(got.SentAt) {
		t.Errorf("invitation row = %+v", got)
	}
}

// TestMembership_ListIsHouseholdScoped: household A's request sees none of B's
// members or invitations, and B's none of A's — and each still sees its own.
//
// covers: INV-AUTH-31
func TestMembership_ListIsHouseholdScoped(t *testing.T) {
	h := newAuthHarness(t)
	bob := testutil.CreateHouseholdWithUser(t, h.q, "Bob")
	aliceInvite, _ := seedInvitationFor(t, h, h.user, "for-alice-house@example.com", time.Now().Add(time.Hour))
	bobInvite, _ := seedInvitationFor(t, h, bob, "for-bob-house@example.com", time.Now().Add(time.Hour))

	aliceView := decodeBody[membershipResp](t, h.do(t, http.MethodGet, "/household/membership", nil))
	if len(aliceView.Members) != 1 || aliceView.Members[0].ID != h.user.ID {
		t.Errorf("alice members = %+v, want only alice", aliceView.Members)
	}
	if len(aliceView.Invitations) != 1 || aliceView.Invitations[0].ID != aliceInvite {
		t.Errorf("alice invitations = %+v, want only %s", aliceView.Invitations, aliceInvite)
	}

	bobView := decodeBody[membershipResp](t, h.doRaw(t, http.MethodGet, "/household/membership", nil, &bob))
	if len(bobView.Members) != 1 || bobView.Members[0].ID != bob.ID {
		t.Errorf("bob members = %+v, want only bob", bobView.Members)
	}
	if len(bobView.Invitations) != 1 || bobView.Invitations[0].ID != bobInvite {
		t.Errorf("bob invitations = %+v, want only %s", bobView.Invitations, bobInvite)
	}
}

func TestMembership_RequiresAuth(t *testing.T) {
	h := newAuthHarness(t)
	requireStatus(t, h.doRaw(t, http.MethodGet, "/household/membership", nil, nil), http.StatusUnauthorized)
	id, _ := seedInvitationFor(t, h, h.user, "x@example.com", time.Now().Add(time.Hour))
	requireStatus(t, h.doRaw(t, http.MethodDelete, "/invitations/"+id.String(), nil, nil), http.StatusUnauthorized)
	if countInvitations(t, h, id) != 1 {
		t.Error("an unauthenticated revoke deleted the row")
	}
}

// TestRevokeInvitation_LinkStopsWorking: revoke removes the row, and the link then
// fails through the real accept path — preview and accept both answer 409
// INVITATION_NO_LONGER_VALID, no account is created, and the list no longer shows it.
//
// covers: INV-AUTH-31
func TestRevokeInvitation_LinkStopsWorking(t *testing.T) {
	h := newAuthHarness(t)
	const invitee = "revoked@example.com"
	id, token := seedInvitationFor(t, h, h.user, invitee, time.Now().Add(time.Hour))

	// Before: the link resolves.
	requireStatus(t, h.doRaw(t, http.MethodGet, "/auth/local/invite?token="+token, nil, nil), http.StatusOK)

	requireStatus(t, h.do(t, http.MethodDelete, "/invitations/"+id.String(), nil), http.StatusNoContent)
	if n := countInvitations(t, h, id); n != 0 {
		t.Fatalf("invitation rows after revoke = %d, want 0", n)
	}

	prev := h.doRaw(t, http.MethodGet, "/auth/local/invite?token="+token, nil, nil)
	requireStatus(t, prev, http.StatusConflict)
	if code := envelopeCode(t, prev); code != string(httperr.CodeInvitationNoLongerValid) {
		t.Errorf("preview code = %q, want %q", code, httperr.CodeInvitationNoLongerValid)
	}

	acc := h.post(t, "/auth/local/invite/accept", map[string]string{
		"token": token, "password": "a-decent-invitee-passphrase",
	})
	requireStatus(t, acc, http.StatusConflict)
	if code := envelopeCode(t, acc); code != string(httperr.CodeInvitationNoLongerValid) {
		t.Errorf("accept code = %q, want %q", code, httperr.CodeInvitationNoLongerValid)
	}
	if findCookie(acc, sessionCookieName) != nil {
		t.Error("a revoked link minted a session")
	}
	if _, err := h.q.GetUserByEmail(context.Background(), invitee); err == nil {
		t.Error("a revoked link created an account")
	}

	view := decodeBody[membershipResp](t, h.do(t, http.MethodGet, "/household/membership", nil))
	if len(view.Invitations) != 0 {
		t.Errorf("invitations after revoke = %+v, want none", view.Invitations)
	}

	// Revoking again is a plain 404 (the row is gone).
	requireStatus(t, h.do(t, http.MethodDelete, "/invitations/"+id.String(), nil), http.StatusNotFound)
}

// TestRevokeInvitation_CrossHouseholdRejected: household B cannot revoke household
// A's invitation — 404, the row survives, and A's link still works — while A can
// revoke its own (the happy path, so the 404 is the guard and not a broken route).
//
// covers: INV-AUTH-31
func TestRevokeInvitation_CrossHouseholdRejected(t *testing.T) {
	h := newAuthHarness(t)
	bob := testutil.CreateHouseholdWithUser(t, h.q, "Bob")
	id, token := seedInvitationFor(t, h, h.user, "guarded@example.com", time.Now().Add(time.Hour))

	rec := h.doRaw(t, http.MethodDelete, "/invitations/"+id.String(), nil, &bob)
	requireStatus(t, rec, http.StatusNotFound)
	if code := envelopeCode(t, rec); code != string(httperr.CodeNotFound) {
		t.Errorf("code = %q, want %q", code, httperr.CodeNotFound)
	}
	if n := countInvitations(t, h, id); n != 1 {
		t.Fatalf("invitation rows after cross-household revoke = %d, want 1", n)
	}
	requireStatus(t, h.doRaw(t, http.MethodGet, "/auth/local/invite?token="+token, nil, nil), http.StatusOK)

	requireStatus(t, h.do(t, http.MethodDelete, "/invitations/"+id.String(), nil), http.StatusNoContent)
	if n := countInvitations(t, h, id); n != 0 {
		t.Errorf("invitation rows after own-household revoke = %d, want 0", n)
	}
}

// TestRevokeInvitation_AnyMemberMayRevoke: revoke is not founder-only — a peer who
// joined by invitation can retire an invite the founder sent, as any member may send one.
//
// covers: INV-AUTH-31
func TestRevokeInvitation_AnyMemberMayRevoke(t *testing.T) {
	h := newAuthHarness(t)
	// A peer, not the founder: created_by is set, as for any invited member.
	peer, err := h.q.CreateLocalUser(context.Background(), db.CreateLocalUserParams{
		HouseholdID: h.user.HouseholdID,
		DisplayName: "Peer",
		Email:       "peer@example.com",
		Locale:      "en-GB",
		TimeZone:    "Asia/Jakarta",
		CreatedBy:   &h.user.ID,
	})
	if err != nil {
		t.Fatalf("seed peer: %v", err)
	}
	id, _ := seedInvitationFor(t, h, h.user, "later@example.com", time.Now().Add(time.Hour))

	requireStatus(t, h.doRaw(t, http.MethodDelete, "/invitations/"+id.String(), nil, &peer), http.StatusNoContent)
	if n := countInvitations(t, h, id); n != 0 {
		t.Errorf("invitation rows = %d, want 0", n)
	}
}

// TestRevokeInvitation_RefusesAcceptedAndMalformed: an accepted invitation is
// history, not revocable (404, row kept), and a malformed id is a 404, not a 500.
//
// covers: INV-AUTH-31
func TestRevokeInvitation_RefusesAcceptedAndMalformed(t *testing.T) {
	h := newAuthHarness(t)
	id, token := seedInvitationFor(t, h, h.user, "joined@example.com", time.Now().Add(time.Hour))
	requireStatus(t, h.post(t, "/auth/local/invite/accept", map[string]string{
		"token": token, "password": "a-decent-invitee-passphrase",
	}), http.StatusNoContent)

	requireStatus(t, h.do(t, http.MethodDelete, "/invitations/"+id.String(), nil), http.StatusNotFound)
	if n := countInvitations(t, h, id); n != 1 {
		t.Errorf("accepted invitation rows = %d, want 1 (kept as history)", n)
	}

	requireStatus(t, h.do(t, http.MethodDelete, "/invitations/not-a-uuid", nil), http.StatusNotFound)
	requireStatus(t, h.do(t, http.MethodDelete, "/invitations/"+uuid.NewString(), nil), http.StatusNotFound)
}
