package httpserver_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kerti/balances-v2/backend/internal/auth"
	"github.com/kerti/balances-v2/backend/internal/config"
	"github.com/kerti/balances-v2/backend/internal/db"
	"github.com/kerti/balances-v2/backend/internal/email"
	"github.com/kerti/balances-v2/backend/internal/httpserver"
	"github.com/kerti/balances-v2/backend/internal/testutil"
)

// TestServer_Healthz exercises New → buildRouter → Handler → handleHealthz end
// to end with a real pool. The auth handler is built local-only (no Google), so
// auth.New constructs no OAuth client and makes no OIDC discovery call — keeping
// the test offline while still mounting the real router. The other section
// handlers are nil: their Mount only registers method values (valid on a nil
// receiver) and no request below reaches them. The request carries no session
// cookie, so SessionMiddleware short-circuits before any handler runs. /healthz
// is the one route that depends solely on the pool, proving the wiring.
func TestServer_Healthz(t *testing.T) {
	tdb := testutil.NewTestDB(t)

	s := httpserver.New(tdb.Pool, &config.Config{}, localOnlyAuth(t, tdb.Pool), nil, nil, nil, nil, nil, nil, nil, nil)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}

	var body struct {
		OK     bool   `json:"ok"`
		DBTime string `json:"db_time"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v (raw: %s)", err, rec.Body.String())
	}
	if !body.OK {
		t.Errorf("ok = false, want true")
	}
	if body.DBTime == "" {
		t.Errorf("db_time empty, want a timestamp from SELECT now()")
	}
}

// localOnlyAuth builds a real but offline auth handler for router-wiring tests:
// local-only (Google off) means auth.New constructs no OAuth client and makes no
// OIDC discovery call, so the router mounts intact without touching the network.
func localOnlyAuth(t *testing.T, pool *pgxpool.Pool) *auth.Handlers {
	t.Helper()
	authH, err := auth.New(context.Background(), db.New(pool), auth.Config{
		LocalEnabled: true,
		Mailer:       email.NewNoopMailer(),
		BackendURL:   "http://localhost:8080",
	})
	if err != nil {
		t.Fatalf("auth.New: %v", err)
	}
	return authH
}
