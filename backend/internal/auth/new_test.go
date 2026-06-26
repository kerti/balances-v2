package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/kerti/balances-v2/backend/internal/db"
	"github.com/kerti/balances-v2/backend/internal/email"
	"github.com/kerti/balances-v2/backend/internal/testutil"
)

// TestNew_RejectsIncompleteGoogleConfig covers the short-circuit in
// newGoogleOAuth that rejects empty client_id/secret/redirect_url — reachable
// only when Google is enabled. The happy-path constructor and the real exchange
// flow are exercised in production; tests substitute googleOAuthClient via
// installStubOAuth.
func TestNew_RejectsIncompleteGoogleConfig(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)

	_, err := New(context.Background(), q, Config{
		GoogleEnabled: true,
		// Google config left zero — newGoogleOAuth returns its sentinel error.
		Mailer:     stubMailerForNew{},
		BackendURL: "http://localhost:8080",
	})
	if err == nil {
		t.Fatal("New: expected error when Google config is empty")
	}
	if !strings.Contains(err.Error(), "client id") {
		t.Errorf("err: want mention of client id, got %v", err)
	}
}

// TestNew_RejectsNoProviderEnabled guards the fail-fast: a server with neither
// identity provider enabled has no way to sign anyone in, so construction must
// error rather than boot a dead instance (ADR-0039).
func TestNew_RejectsNoProviderEnabled(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)

	_, err := New(context.Background(), q, Config{
		Mailer:     stubMailerForNew{},
		BackendURL: "http://localhost:8080",
	})
	if err == nil || !strings.Contains(err.Error(), "no provider enabled") {
		t.Fatalf("New: want no-provider-enabled error, got %v", err)
	}
}

// TestNew_LocalOnlyMakesNoOIDCDiscoveryCall is the local-only-no-OIDC invariant
// (INV-AUTH-16, ADR-0039): a self-host with AUTH_LOCAL_ENABLED and Google off
// must construct no OAuth client and make no outbound OIDC discovery call. We
// point the Google issuer at a counting test server and assert it is never hit
// even though a (would-be valid) Google config is present.
//
// covers: INV-AUTH-16
func TestNew_LocalOnlyMakesNoOIDCDiscoveryCall(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)

	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	h, err := New(context.Background(), q, Config{
		GoogleEnabled: false,
		LocalEnabled:  true,
		Google: GoogleConfig{
			ClientID:     "id",
			ClientSecret: "secret",
			RedirectURL:  "http://localhost:8080/cb",
			IssuerURL:    srv.URL,
		},
		Mailer:     stubMailerForNew{},
		BackendURL: "http://localhost:8080",
	})
	if err != nil {
		t.Fatalf("New (local-only): unexpected error: %v", err)
	}
	if got := atomic.LoadInt32(&hits); got != 0 {
		t.Errorf("local-only boot hit the OIDC issuer %d time(s); want 0", got)
	}
	if h.googleOAuth != nil {
		t.Error("local-only boot constructed a Google OAuth client; want nil")
	}
}

// stubMailerForNew is a no-op mailer used purely to satisfy the cfg.Mailer
// non-nil requirement so the other construction checks can fire first.
type stubMailerForNew struct{}

func (stubMailerForNew) Send(context.Context, email.Message) error { return nil }
