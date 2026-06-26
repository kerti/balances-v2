package auth

// This is an internal test file (package auth, not auth_test) so the harness
// can construct *Handlers via a struct literal — bypassing New, which calls
// newGoogleOAuth + oidc.NewProvider and tries to do real network discovery
// against accounts.google.com. The handleCallback path that exercises
// googleOAuth.exchange is deferred to Phase 2 (needs an injectable
// exchanger interface).

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kerti/balances-v2/backend/internal/db"
	"github.com/kerti/balances-v2/backend/internal/email"
	"github.com/kerti/balances-v2/backend/internal/testutil"
)

// stubMailer captures sent messages for assertion. Send always succeeds —
// invitation tests that need a failure path can wire a one-shot failing
// variant inline.
type stubMailer struct {
	mu       sync.Mutex
	messages []email.Message
}

func (m *stubMailer) Send(_ context.Context, msg email.Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, msg)
	return nil
}

func (m *stubMailer) sent() []email.Message {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]email.Message, len(m.messages))
	copy(out, m.messages)
	return out
}

type authHarness struct {
	pool   *pgxpool.Pool
	q      *db.Queries
	h      *Handlers
	user   db.User
	mailer *stubMailer
	router *chi.Mux
}

func newAuthHarness(t *testing.T) *authHarness {
	t.Helper()
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)
	user := testutil.CreateHouseholdWithUser(t, q, "Alice")

	mailer := &stubMailer{}
	h := &Handlers{
		q:             q,
		googleEnabled: true,
		localEnabled:  true,
		limiter:       newLoginLimiter(),
		googleOAuth: &googleOAuth{
			// cfg is enough for handleStart (AuthCodeURL). verifier stays nil
			// because we don't exercise exchange in this test phase.
			cfg: oauth2.Config{
				ClientID:     "test-client-id",
				ClientSecret: "test-client-secret",
				RedirectURL:  "http://localhost:8080/api/auth/google/callback",
				Endpoint:     google.Endpoint,
				Scopes:       []string{"openid", "email", "profile"},
			},
		},
		mailer:       mailer,
		validate:     validator.New(validator.WithRequiredStructEnabled()),
		sessionTTL:   30 * time.Minute,
		cookieSecure: false,
		frontendURL:  "http://localhost:5173",
		backendURL:   "http://localhost:8080",
		emailFrom:    "test@example.com",
	}

	r := chi.NewRouter()
	h.Mount(r)

	return &authHarness{pool: tdb.Pool, q: q, h: h, user: user, mailer: mailer, router: r}
}

// do issues an authed request: ctx carries the harness user via WithUser
// so RequireAuth and handleMe see them. For tests that exercise
// SessionMiddleware itself (real cookie flow), use doRaw + AddCookie instead.
func (h *authHarness) do(t *testing.T, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	return h.doRaw(t, method, path, body, &h.user)
}

func (h *authHarness) doRaw(t *testing.T, method, path string, body any, user *db.User) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	switch v := body.(type) {
	case nil:
		reader = nil
	case string:
		reader = strings.NewReader(v)
	case []byte:
		reader = bytes.NewReader(v)
	default:
		buf, err := json.Marshal(v)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(buf)
	}
	req := httptest.NewRequest(method, path, reader)
	if reader != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if user != nil {
		req = req.WithContext(WithUser(req.Context(), *user))
	}
	rec := httptest.NewRecorder()
	h.router.ServeHTTP(rec, req)
	return rec
}

func decodeBody[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.NewDecoder(rec.Body).Decode(&v); err != nil {
		t.Fatalf("decode response (status %d, body %q): %v", rec.Code, rec.Body.String(), err)
	}
	return v
}

func requireStatus(t *testing.T, rec *httptest.ResponseRecorder, want int) {
	t.Helper()
	if rec.Code != want {
		t.Fatalf("status: want %d, got %d (body: %s)", want, rec.Code, rec.Body.String())
	}
}

// findCookie returns the named Set-Cookie header from the recorder, or nil.
func findCookie(rec *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// stubOAuthClient is a googleOAuthClient that returns canned claims (or a
// canned error) without contacting Google. Tests construct one per scenario.
type stubOAuthClient struct {
	authURL string
	claims  *googleClaims
	err     error

	lastCode string
}

func (s *stubOAuthClient) authCodeURL(state string) string {
	if s.authURL == "" {
		return "https://stub/oauth?state=" + state
	}
	return s.authURL + "?state=" + state
}

func (s *stubOAuthClient) exchange(_ context.Context, code string) (*googleClaims, error) {
	s.lastCode = code
	return s.claims, s.err
}

// installStubOAuth swaps the harness's googleOAuth client for a stub and
// returns it so the test can assert on lastCode or mutate the canned claims.
func (h *authHarness) installStubOAuth(claims *googleClaims, err error) *stubOAuthClient {
	s := &stubOAuthClient{claims: claims, err: err}
	h.h.googleOAuth = s
	return s
}
