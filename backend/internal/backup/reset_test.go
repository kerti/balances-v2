package backup

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/kerti/balances-v2/backend/internal/auth"
	"github.com/kerti/balances-v2/backend/internal/db"
	"github.com/kerti/balances-v2/backend/internal/testutil"
)

const (
	testDemoEmail    = "demo@balances.local"
	testDemoPassword = "BalancesDemo!2026"
	testDemoToken    = "test-demo-reset-token"
)

// covers: INV-BACKUP-15
func TestDemoReset_NotMountedWhenDemoModeOff(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	h := New(tdb.Pool, "http://test.local", &stubIssuer{}, &stubNotifier{}, false, DemoConfig{})
	r := chi.NewRouter()
	h.Mount(r)

	req := httptest.NewRequest(http.MethodPost, "/admin/demo-reset", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (route must not exist when DEMO_MODE is off)", rec.Code)
	}
}

// covers: INV-BACKUP-15
func TestDemoReset_RequiresBearerToken(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)
	testutil.CreateHouseholdWithUser(t, q, "Demo")
	h := New(tdb.Pool, "http://test.local", &stubIssuer{}, &stubNotifier{}, false, DemoConfig{
		Enabled:    true,
		ResetToken: testDemoToken,
		Email:      testDemoEmail,
		Password:   testDemoPassword,
	})
	r := chi.NewRouter()
	h.Mount(r)

	cases := []struct {
		name   string
		header string
	}{
		{"no header", ""},
		{"wrong token", "Bearer wrong-token"},
		{"malformed", testDemoToken},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/admin/demo-reset", nil)
			if c.header != "" {
				req.Header.Set("Authorization", c.header)
			}
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401, body=%s", rec.Code, rec.Body.String())
			}
		})
	}
}

// covers: INV-BACKUP-15
func TestDemoReset_WipesAndReseeds(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)

	// Seed the existing demo household + shared credential, exactly as the
	// maintainer's one-time manual founding would (ADR-0041) — the reset
	// endpoint locates it by DEMO_EMAIL, wipes it, and rebuilds it fresh.
	household, err := q.CreateHousehold(context.Background(), db.CreateHouseholdParams{
		DisplayName:       "Old Demo Household",
		ReportingCurrency: "IDR",
	})
	if err != nil {
		t.Fatalf("seed household: %v", err)
	}
	demoUser, err := q.CreateLocalUser(context.Background(), db.CreateLocalUserParams{
		HouseholdID: household.ID,
		DisplayName: "Demo",
		Email:       testDemoEmail,
		Locale:      "en-GB",
		TimeZone:    "Asia/Jakarta",
	})
	if err != nil {
		t.Fatalf("seed demo user: %v", err)
	}
	oldHash, err := auth.HashPassword("some-old-password")
	if err != nil {
		t.Fatalf("hash old password: %v", err)
	}
	if _, err := q.UpsertLocalCredential(context.Background(), db.UpsertLocalCredentialParams{
		UserID:       demoUser.ID,
		PasswordHash: oldHash,
	}); err != nil {
		t.Fatalf("seed old credential: %v", err)
	}

	h := New(tdb.Pool, "http://test.local", &stubIssuer{}, &stubNotifier{}, false, DemoConfig{
		Enabled:    true,
		ResetToken: testDemoToken,
		Email:      testDemoEmail,
		Password:   testDemoPassword,
	})
	r := chi.NewRouter()
	h.Mount(r)

	req := httptest.NewRequest(http.MethodPost, "/admin/demo-reset", nil)
	req.Header.Set("Authorization", "Bearer "+testDemoToken)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204, body=%s", rec.Code, rec.Body.String())
	}

	// The old household is gone entirely.
	var oldRemaining int
	if err := tdb.Pool.QueryRow(context.Background(),
		"SELECT count(*) FROM households WHERE id = $1", household.ID).Scan(&oldRemaining); err != nil {
		t.Fatalf("count old households: %v", err)
	}
	if oldRemaining != 0 {
		t.Errorf("old household remaining = %d, want 0", oldRemaining)
	}

	// A fresh demo user exists under the same email, with the *configured*
	// password now live (not the old one).
	newUser, err := q.GetUserByEmail(context.Background(), testDemoEmail)
	if err != nil {
		t.Fatalf("get reseeded demo user: %v", err)
	}
	if newUser.HouseholdID == household.ID {
		t.Error("reseeded household should be a fresh id, not the wiped one")
	}
	cred, err := q.GetLocalCredentialByUserID(context.Background(), newUser.ID)
	if err != nil {
		t.Fatalf("get reseeded credential: %v", err)
	}
	ok, err := auth.VerifyPassword(testDemoPassword, cred.PasswordHash)
	if err != nil {
		t.Fatalf("verify reseeded password: %v", err)
	}
	if !ok {
		t.Error("reseeded credential does not match the configured demo password")
	}

	// A second, credential-less member exists for ownership-attribution realism.
	var memberCount int
	if err := tdb.Pool.QueryRow(context.Background(),
		"SELECT count(*) FROM users WHERE household_id = $1 AND id != $2", newUser.HouseholdID, newUser.ID).Scan(&memberCount); err != nil {
		t.Fatalf("count second member: %v", err)
	}
	if memberCount != 1 {
		t.Errorf("second member count = %d, want 1", memberCount)
	}

	// At least one toy position exists so the dashboard isn't empty.
	var assetCount int
	if err := tdb.Pool.QueryRow(context.Background(),
		"SELECT count(*) FROM assets WHERE household_id = $1", newUser.HouseholdID).Scan(&assetCount); err != nil {
		t.Fatalf("count seeded assets: %v", err)
	}
	if assetCount == 0 {
		t.Error("expected at least one seeded toy position, found none")
	}
}
