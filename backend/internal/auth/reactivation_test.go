package auth

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/kerti/balances-v2/backend/internal/db"
	"github.com/kerti/balances-v2/backend/internal/httperr"
	"github.com/kerti/balances-v2/backend/internal/testutil"
)

// tokenFromSetPasswordURL pulls the plaintext token out of a reactivation
// response's set_password_url — the only place the plaintext ever appears (it is
// hashed at rest), mirroring how the founder relays it to the member.
func tokenFromSetPasswordURL(t *testing.T, raw string) string {
	t.Helper()
	i := strings.Index(raw, "/reset?")
	if i < 0 {
		t.Fatalf("set_password_url is not a /reset link: %q", raw)
	}
	u, err := url.Parse(raw[i:])
	if err != nil {
		t.Fatalf("parse set_password_url %q: %v", raw, err)
	}
	tok := u.Query().Get("token")
	if tok == "" {
		t.Fatalf("set_password_url carried no token: %q", raw)
	}
	return tok
}

// seedDormantMember creates a household member with neither a google_sub nor a
// local_credentials row — the dormant state a local member lands in post-restore
// (ADR-0036/0039). created_by is set so it reads as an invited member, not the
// founder (dormancy itself ignores lineage).
func (h *authHarness) seedDormantMember(t *testing.T, email string) db.User {
	t.Helper()
	u, err := h.q.CreateLocalUser(context.Background(), db.CreateLocalUserParams{
		HouseholdID: h.user.HouseholdID,
		DisplayName: "Dormant",
		Email:       email,
		Locale:      "en-GB",
		TimeZone:    "Asia/Jakarta",
		CreatedBy:   &h.user.ID,
	})
	if err != nil {
		t.Fatalf("seed dormant member: %v", err)
	}
	return u
}

// TestReactivation_HappyPath is the #283 tracer bullet: the founder lists dormant
// members, mints a one-time set-password link for one, and the member follows the
// link to set a password and sign in — the no-mail recovery path end to end.
//
// covers: INV-AUTH-20
func TestReactivation_HappyPath(t *testing.T) {
	h := newAuthHarness(t)
	member := h.seedDormantMember(t, "dormant@example.com")

	// The founder sees exactly the dormant member.
	list := h.do(t, http.MethodGet, "/auth/local/reactivation/members", nil)
	requireStatus(t, list, http.StatusOK)
	members := decodeBody[[]dormantMember](t, list)
	if len(members) != 1 || members[0].ID != member.ID {
		t.Fatalf("dormant list = %+v, want just %s", members, member.ID)
	}

	// Reactivate → a one-time link for the member, shown once.
	react := h.do(t, http.MethodPost, "/auth/local/reactivation", map[string]any{"user_id": member.ID})
	requireStatus(t, react, http.StatusCreated)
	resp := decodeBody[reactivateMemberResp](t, react)
	if resp.Email != member.Email {
		t.Errorf("resp email = %q, want %q", resp.Email, member.Email)
	}
	token := tokenFromSetPasswordURL(t, resp.SetPasswordURL)

	// The member follows the link: preview resolves the bound email without
	// consuming it, then the set mints a session directly.
	prev := h.doRaw(t, http.MethodGet, "/auth/local/reset?token="+token, nil, nil)
	requireStatus(t, prev, http.StatusOK)
	if pv := decodeBody[localResetPreviewResp](t, prev); pv.Email != member.Email {
		t.Errorf("preview email = %q, want %q", pv.Email, member.Email)
	}

	const newPass = "a-freshly-chosen-passphrase"
	set := h.doRaw(t, http.MethodPost, "/auth/local/reset",
		map[string]string{"token": token, "password": newPass}, nil)
	requireStatus(t, set, http.StatusNoContent)
	if findCookie(set, sessionCookieName) == nil {
		t.Fatal("reactivation set did not mint a session cookie")
	}

	// The reactivated member can now log in — they were dormant before.
	login := h.doRaw(t, http.MethodPost, "/auth/local/login",
		map[string]string{"email": member.Email, "password": newPass}, nil)
	requireStatus(t, login, http.StatusNoContent)

	// And they are no longer dormant, so no longer reactivatable.
	list2 := h.do(t, http.MethodGet, "/auth/local/reactivation/members", nil)
	requireStatus(t, list2, http.StatusOK)
	if m := decodeBody[[]dormantMember](t, list2); len(m) != 0 {
		t.Errorf("dormant list after reactivation = %+v, want empty", m)
	}
}

// TestReactivation_RefusesActiveMember asserts the core scoping: an in-app
// reactivation of a member who already holds a credential is refused (409), and
// such a member never appears in the dormant list. Resetting an active account is
// impersonation; the CLI is the escape hatch (ADR-0039).
//
// covers: INV-AUTH-20
func TestReactivation_RefusesActiveMember(t *testing.T) {
	h := newAuthHarness(t)
	active := h.seedLocalUser(t, "active@example.com", "the-existing-passphrase")

	list := h.do(t, http.MethodGet, "/auth/local/reactivation/members", nil)
	requireStatus(t, list, http.StatusOK)
	for _, m := range decodeBody[[]dormantMember](t, list) {
		if m.ID == active.ID {
			t.Fatalf("active member %s appeared in dormant list", active.ID)
		}
	}

	react := h.do(t, http.MethodPost, "/auth/local/reactivation", map[string]any{"user_id": active.ID})
	requireStatus(t, react, http.StatusConflict)
	if code := envelopeCode(t, react); code != string(httperr.CodeMemberNotDormant) {
		t.Errorf("code = %q, want %q", code, httperr.CodeMemberNotDormant)
	}
}

// TestReactivation_RefusesGoogleMember asserts a Google member is reachable, not
// dormant: refused (409) and absent from the dormant list. Minting them a local
// credential would be account-linking, which is out of scope (ADR-0039).
//
// covers: INV-AUTH-20
func TestReactivation_RefusesGoogleMember(t *testing.T) {
	h := newAuthHarness(t)
	sub := "google-sub-123"
	g, err := h.q.CreateUser(context.Background(), db.CreateUserParams{
		HouseholdID: h.user.HouseholdID,
		DisplayName: "Googler",
		Email:       "googler@example.com",
		GoogleSub:   sub,
		Locale:      "en-GB",
		TimeZone:    "Asia/Jakarta",
		CreatedBy:   &h.user.ID,
	})
	if err != nil {
		t.Fatalf("seed google member: %v", err)
	}

	list := h.do(t, http.MethodGet, "/auth/local/reactivation/members", nil)
	requireStatus(t, list, http.StatusOK)
	for _, m := range decodeBody[[]dormantMember](t, list) {
		if m.ID == g.ID {
			t.Fatalf("google member %s appeared in dormant list", g.ID)
		}
	}

	react := h.do(t, http.MethodPost, "/auth/local/reactivation", map[string]any{"user_id": g.ID})
	requireStatus(t, react, http.StatusConflict)
	if code := envelopeCode(t, react); code != string(httperr.CodeMemberNotDormant) {
		t.Errorf("code = %q, want %q", code, httperr.CodeMemberNotDormant)
	}
}

// TestReactivation_NonFounderForbidden asserts reactivation is founder-only: a
// peer member (created_by set) is refused (403) on both routes, preserving the
// peer model (ADR-0017/0039) — reactivation is operator bring-up, not a standing
// privilege over peers.
//
// covers: INV-AUTH-20
func TestReactivation_NonFounderForbidden(t *testing.T) {
	h := newAuthHarness(t)
	peer := h.seedLocalUser(t, "peer@example.com", "peer-passphrase")
	// seedLocalUser leaves created_by NULL; make this member a non-founder.
	peer.CreatedBy = &h.user.ID
	dormant := h.seedDormantMember(t, "dormant@example.com")

	list := h.doRaw(t, http.MethodGet, "/auth/local/reactivation/members", nil, &peer)
	requireStatus(t, list, http.StatusForbidden)
	if code := envelopeCode(t, list); code != string(httperr.CodeForbidden) {
		t.Errorf("list code = %q, want %q", code, httperr.CodeForbidden)
	}

	react := h.doRaw(t, http.MethodPost, "/auth/local/reactivation",
		map[string]any{"user_id": dormant.ID}, &peer)
	requireStatus(t, react, http.StatusForbidden)
	if code := envelopeCode(t, react); code != string(httperr.CodeForbidden) {
		t.Errorf("reactivate code = %q, want %q", code, httperr.CodeForbidden)
	}
}

// TestReactivation_CrossHouseholdIs404 asserts a founder cannot reactivate a
// member of another household — the id is indistinguishable from a non-existent
// one (404), so no cross-tenant existence leak.
//
// covers: INV-AUTH-20
func TestReactivation_CrossHouseholdIs404(t *testing.T) {
	h := newAuthHarness(t)
	// A second household with its own dormant member.
	other := testutil.CreateHouseholdWithUser(t, h.q, "Bob")
	outsider, err := h.q.CreateLocalUser(context.Background(), db.CreateLocalUserParams{
		HouseholdID: other.HouseholdID,
		DisplayName: "Outsider",
		Email:       "outsider@example.com",
		Locale:      "en-GB",
		TimeZone:    "Asia/Jakarta",
		CreatedBy:   &other.ID,
	})
	if err != nil {
		t.Fatalf("seed outsider: %v", err)
	}

	react := h.do(t, http.MethodPost, "/auth/local/reactivation", map[string]any{"user_id": outsider.ID})
	requireStatus(t, react, http.StatusNotFound)
	if code := envelopeCode(t, react); code != string(httperr.CodeNotFound) {
		t.Errorf("code = %q, want %q", code, httperr.CodeNotFound)
	}
}

// TestReactivation_UnknownUserIs404 asserts an unknown target id is a 404.
//
// covers: INV-AUTH-20
func TestReactivation_UnknownUserIs404(t *testing.T) {
	h := newAuthHarness(t)
	react := h.do(t, http.MethodPost, "/auth/local/reactivation", map[string]any{"user_id": uuid.New()})
	requireStatus(t, react, http.StatusNotFound)
}

// TestReactivation_BadRequest asserts the input guards: a malformed body is a
// 400, and a missing/zero user_id is a 400 validation error.
//
// covers: INV-AUTH-20
func TestReactivation_BadRequest(t *testing.T) {
	h := newAuthHarness(t)

	bad := h.do(t, http.MethodPost, "/auth/local/reactivation", "{not-json")
	requireStatus(t, bad, http.StatusBadRequest)
	if code := envelopeCode(t, bad); code != string(httperr.CodeInvalidJSONBody) {
		t.Errorf("code = %q, want %q", code, httperr.CodeInvalidJSONBody)
	}

	missing := h.do(t, http.MethodPost, "/auth/local/reactivation", map[string]any{"user_id": uuid.Nil})
	requireStatus(t, missing, http.StatusBadRequest)
	if code := envelopeCode(t, missing); code != string(httperr.CodeValidation) {
		t.Errorf("code = %q, want %q", code, httperr.CodeValidation)
	}
}

// TestReactivation_Unauthenticated asserts both routes are behind RequireAuth.
//
// covers: INV-AUTH-20
func TestReactivation_Unauthenticated(t *testing.T) {
	h := newAuthHarness(t)
	list := h.doRaw(t, http.MethodGet, "/auth/local/reactivation/members", nil, nil)
	requireStatus(t, list, http.StatusUnauthorized)
	react := h.doRaw(t, http.MethodPost, "/auth/local/reactivation",
		map[string]any{"user_id": uuid.New()}, nil)
	requireStatus(t, react, http.StatusUnauthorized)
}
