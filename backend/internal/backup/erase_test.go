package backup

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kerti/balances-v2/backend/internal/auth"
	"github.com/kerti/balances-v2/backend/internal/db"
	"github.com/kerti/balances-v2/backend/internal/httperr"
	"github.com/kerti/balances-v2/backend/internal/testutil"
)

// eraseReq builds and serves a JSON erase request against the given handler.
func eraseReq(ctx context.Context, t *testing.T, fn http.HandlerFunc, body any) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/backup/erase", bytes.NewReader(raw)).WithContext(ctx)
	rec := httptest.NewRecorder()
	fn(rec, req)
	return rec
}

// covers: INV-BACKUP-12, INV-BACKUP-13
func TestEraseHousehold_HappyPath(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)
	alice := testutil.CreateHouseholdWithUser(t, q, "Alice")
	aliceCtx := auth.WithUser(context.Background(), alice)
	seedHousehold(aliceCtx, t, tdb.Pool, alice)

	// A second, invited (non-founder) member of Alice's household — must also
	// be purged and must receive the notice, not the confirmation.
	peer, err := q.CreateUser(context.Background(), db.CreateUserParams{
		HouseholdID: alice.HouseholdID,
		DisplayName: "Peer",
		Email:       "peer@example.com",
		GoogleSub:   "peer-sub",
		Locale:      "en-GB",
		TimeZone:    "Asia/Jakarta",
		CreatedBy:   &alice.ID,
	})
	if err != nil {
		t.Fatalf("seed peer: %v", err)
	}

	// Cross-tenant noise that must survive.
	bob := testutil.CreateHouseholdWithUser(t, q, "Bob")
	bobCtx := auth.WithUser(context.Background(), bob)
	seedHousehold(bobCtx, t, tdb.Pool, bob)

	issuer := &stubIssuer{}
	notifier := &stubNotifier{}
	h := New(tdb.Pool, "http://test.local", issuer, notifier, false, DemoConfig{})

	before, err := h.buildEnvelope(context.Background(), alice.HouseholdID, FidelityFull)
	if err != nil {
		t.Fatalf("export before: %v", err)
	}
	if before.Household.Household.DisplayName == "" {
		t.Fatal("seed household has no display name")
	}
	householdName := before.Household.Household.DisplayName

	rec := eraseReq(aliceCtx, t, h.handleEraseHousehold, eraseHouseholdReq{HouseholdName: householdName})
	if rec.Code != http.StatusOK {
		t.Fatalf("erase status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var resp eraseHouseholdResp
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode resp: %v", err)
	}
	if !resp.Erased {
		t.Error("resp.Erased = false, want true")
	}

	// The household is gone entirely — every table wipeHousehold reaches.
	var remaining int
	if err := tdb.Pool.QueryRow(context.Background(),
		"SELECT count(*) FROM households WHERE id = $1", alice.HouseholdID).Scan(&remaining); err != nil {
		t.Fatalf("count households: %v", err)
	}
	if remaining != 0 {
		t.Errorf("households remaining = %d, want 0", remaining)
	}
	var users int
	if err := tdb.Pool.QueryRow(context.Background(),
		"SELECT count(*) FROM users WHERE household_id = $1", alice.HouseholdID).Scan(&users); err != nil {
		t.Fatalf("count users: %v", err)
	}
	if users != 0 {
		t.Errorf("users remaining = %d, want 0", users)
	}

	// Bob's household is untouched.
	var bobRemaining int
	if err := tdb.Pool.QueryRow(context.Background(),
		"SELECT count(*) FROM households WHERE id = $1", bob.HouseholdID).Scan(&bobRemaining); err != nil {
		t.Fatalf("count bob households: %v", err)
	}
	if bobRemaining != 1 {
		t.Errorf("bob household remaining = %d, want 1 (cross-tenant bleed)", bobRemaining)
	}

	// The caller's session cookie was cleared, not re-issued.
	if !issuer.cleared {
		t.Error("session was not cleared on erase")
	}

	// The notifier was fired with both members, captured before the wipe, and
	// told who the founder was so it can split confirm vs notice.
	if notifier.eraseCalls != 1 {
		t.Fatalf("NotifyErasure calls = %d, want 1", notifier.eraseCalls)
	}
	if notifier.eraseFounderID != alice.ID {
		t.Errorf("erase founder id = %s, want %s", notifier.eraseFounderID, alice.ID)
	}
	if notifier.eraseHouseholdName != householdName {
		t.Errorf("erase household name = %q, want %q", notifier.eraseHouseholdName, householdName)
	}
	if len(notifier.eraseMembers) != 2 {
		t.Fatalf("erase members = %d, want 2 (alice + peer)", len(notifier.eraseMembers))
	}
	var sawAlice, sawPeer bool
	for _, m := range notifier.eraseMembers {
		if m.ID == alice.ID {
			sawAlice = true
		}
		if m.ID == peer.ID {
			sawPeer = true
		}
	}
	if !sawAlice || !sawPeer {
		t.Errorf("erase members = %+v, want alice + peer", notifier.eraseMembers)
	}
}

// covers: INV-BACKUP-14
func TestEraseHousehold_DemoModeForbidden(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)
	alice := testutil.CreateHouseholdWithUser(t, q, "Alice")
	aliceCtx := auth.WithUser(context.Background(), alice)
	seedHousehold(aliceCtx, t, tdb.Pool, alice)
	h := New(tdb.Pool, "http://test.local", &stubIssuer{}, &stubNotifier{}, false, DemoConfig{Enabled: true})

	rec := eraseReq(aliceCtx, t, h.handleEraseHousehold, eraseHouseholdReq{HouseholdName: "whatever"})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403, body=%s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(string(httperr.CodeErasureDisabledDemo))) {
		t.Errorf("body = %s, want %s", rec.Body.String(), httperr.CodeErasureDisabledDemo)
	}

	var remaining int
	if err := tdb.Pool.QueryRow(context.Background(),
		"SELECT count(*) FROM households WHERE id = $1", alice.HouseholdID).Scan(&remaining); err != nil {
		t.Fatalf("count households: %v", err)
	}
	if remaining != 1 {
		t.Errorf("households remaining = %d, want 1 (untouched)", remaining)
	}
}

// covers: INV-BACKUP-12
func TestEraseHousehold_NonFounderForbidden(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)
	alice := testutil.CreateHouseholdWithUser(t, q, "Alice")
	peer, err := q.CreateUser(context.Background(), db.CreateUserParams{
		HouseholdID: alice.HouseholdID,
		DisplayName: "Peer",
		Email:       "peer@example.com",
		GoogleSub:   "peer-sub",
		Locale:      "en-GB",
		TimeZone:    "Asia/Jakarta",
		CreatedBy:   &alice.ID,
	})
	if err != nil {
		t.Fatalf("seed peer: %v", err)
	}
	seedHousehold(auth.WithUser(context.Background(), alice), t, tdb.Pool, alice)
	h := New(tdb.Pool, "http://test.local", &stubIssuer{}, &stubNotifier{}, false, DemoConfig{})

	peerCtx := auth.WithUser(context.Background(), peer)
	rec := eraseReq(peerCtx, t, h.handleEraseHousehold, eraseHouseholdReq{HouseholdName: "whatever"})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403, body=%s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(string(httperr.CodeForbidden))) {
		t.Errorf("body = %s, want %s", rec.Body.String(), httperr.CodeForbidden)
	}

	var remaining int
	if err := tdb.Pool.QueryRow(context.Background(),
		"SELECT count(*) FROM households WHERE id = $1", alice.HouseholdID).Scan(&remaining); err != nil {
		t.Fatalf("count households: %v", err)
	}
	if remaining != 1 {
		t.Errorf("households remaining = %d, want 1 (untouched)", remaining)
	}
}

// covers: INV-BACKUP-12
func TestEraseHousehold_NameMismatch(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)
	alice := testutil.CreateHouseholdWithUser(t, q, "Alice")
	aliceCtx := auth.WithUser(context.Background(), alice)
	seedHousehold(aliceCtx, t, tdb.Pool, alice)
	h := New(tdb.Pool, "http://test.local", &stubIssuer{}, &stubNotifier{}, false, DemoConfig{})

	rec := eraseReq(aliceCtx, t, h.handleEraseHousehold, eraseHouseholdReq{HouseholdName: "Not The Real Name"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400, body=%s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(string(httperr.CodeHouseholdNameMismatch))) {
		t.Errorf("body = %s, want %s", rec.Body.String(), httperr.CodeHouseholdNameMismatch)
	}

	var remaining int
	if err := tdb.Pool.QueryRow(context.Background(),
		"SELECT count(*) FROM households WHERE id = $1", alice.HouseholdID).Scan(&remaining); err != nil {
		t.Fatalf("count households: %v", err)
	}
	if remaining != 1 {
		t.Errorf("households remaining = %d, want 1 (untouched)", remaining)
	}
}

// covers: INV-BACKUP-12
func TestEraseHousehold_Unauthenticated(t *testing.T) {
	h := New(nil, "http://test.local", &stubIssuer{}, &stubNotifier{}, false, DemoConfig{})
	rec := eraseReq(context.Background(), t, h.handleEraseHousehold, eraseHouseholdReq{HouseholdName: "anything"})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestEraseHousehold_BadRequest(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)
	alice := testutil.CreateHouseholdWithUser(t, q, "Alice")
	aliceCtx := auth.WithUser(context.Background(), alice)
	h := New(tdb.Pool, "http://test.local", &stubIssuer{}, &stubNotifier{}, false, DemoConfig{})

	req := httptest.NewRequest(http.MethodPost, "/api/backup/erase", bytes.NewReader([]byte("{not-json"))).WithContext(aliceCtx)
	rec := httptest.NewRecorder()
	h.handleEraseHousehold(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400, body=%s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(string(httperr.CodeInvalidJSONBody))) {
		t.Errorf("body = %s, want %s", rec.Body.String(), httperr.CodeInvalidJSONBody)
	}
}
