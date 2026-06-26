package httpserver_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kerti/balances-v2/backend/internal/config"
	"github.com/kerti/balances-v2/backend/internal/httpserver"
	"github.com/kerti/balances-v2/backend/internal/testutil"
)

// TestServing_SingleOriginPath exercises the production single-origin serving
// path through the *real* router — New → buildRouter → Handler — with WebDir set
// to a fixture bundle laid out the way Vite emits one (an index.html plus a
// content-hashed chunk under assets/). The unit test in spa_test.go verifies
// spaHandler() in isolation; this verifies the wiring around it: that the SPA
// catch-all is mounted alongside /api + /healthz and never shadows them
// (INV-SERVING-04), the one property a handler-in-isolation test cannot reach.
// It exists because #190 and #241 — two /assets/* SPA-fallback bugs — both
// shipped past an e2e harness that serves the SPA via Vite's dev server, whose
// own fallback masks the Go spaHandler the production image actually ships.
//
// The route handlers are passed nil, like server_test.go: buildRouter and each
// Mount only register method values, which is valid Go on a nil receiver. We
// never drive a request into a mounted /api handler (which would dereference
// it); the precedence assertion hits an *unmounted* /api path, which chi
// resolves to a 404 without invoking any handler.
//
// covers: INV-SERVING-01, INV-SERVING-02, INV-SERVING-03, INV-SERVING-04
func TestServing_SingleOriginPath(t *testing.T) {
	const (
		shell = "<!doctype html><html><body>SPA_SHELL" +
			`<script type="module" src="/assets/index-abc12345.js"></script></body></html>`
		chunk = "console.log('REAL_CHUNK')"
	)
	write := func(path, body string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	dir := t.TempDir()
	write(filepath.Join(dir, "index.html"), shell)
	if err := os.MkdirAll(filepath.Join(dir, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	write(filepath.Join(dir, "assets", "index-abc12345.js"), chunk)

	tdb := testutil.NewTestDB(t)
	s := httpserver.New(
		tdb.Pool, &config.Config{WebDir: dir},
		localOnlyAuth(t, tdb.Pool), nil, nil, nil, nil, nil, nil, nil, nil,
	)
	h := s.Handler()

	get := func(path string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		return rec
	}

	t.Run("root serves the built shell", func(t *testing.T) {
		rec := get("/")
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "SPA_SHELL") {
			t.Errorf("GET /: code=%d body=%q, want 200 + SPA_SHELL", rec.Code, rec.Body.String())
		}
	})

	t.Run("real hashed chunk is served", func(t *testing.T) {
		rec := get("/assets/index-abc12345.js")
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "REAL_CHUNK") {
			t.Errorf("GET chunk: code=%d body=%q, want 200 + REAL_CHUNK", rec.Code, rec.Body.String())
		}
	})

	// INV-SERVING-01: deep client route on a hard refresh resolves to the shell.
	t.Run("deep client route falls back to the shell", func(t *testing.T) {
		rec := get("/investments/123")
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "SPA_SHELL") {
			t.Errorf("GET deep route: code=%d body=%q, want 200 + SPA_SHELL", rec.Code, rec.Body.String())
		}
	})

	// INV-SERVING-03: an extensionless client route that collides with the
	// build-output dir (the Assets section, #241) falls back, never 404s.
	t.Run("extensionless client route under /assets falls back", func(t *testing.T) {
		for _, p := range []string{"/assets/bank-accounts", "/assets/bank-accounts/456"} {
			rec := get(p)
			if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "SPA_SHELL") {
				t.Errorf("GET %s: code=%d body=%q, want 200 + SPA_SHELL", p, rec.Code, rec.Body.String())
			}
		}
	})

	// INV-SERVING-02: a missing extension-bearing chunk under /assets/ 404s
	// rather than serving the shell, so a stale chunk request never gets 200
	// text/html (#190).
	t.Run("missing hashed chunk 404s instead of the shell", func(t *testing.T) {
		rec := get("/assets/index-deadbeef.js")
		if rec.Code != http.StatusNotFound {
			t.Errorf("GET missing chunk: code=%d, want 404", rec.Code)
		}
		if strings.Contains(rec.Body.String(), "SPA_SHELL") {
			t.Errorf("GET missing chunk served the SPA shell, want a 404 body")
		}
	})

	// INV-SERVING-04 — the integration-only assertion the isolated spaHandler
	// test cannot make: the SPA catch-all is mounted last and does not shadow
	// the sibling route trees. A registered non-frontend route resolves to its
	// own handler, and an unmounted /api path resolves to chi's 404 — never the
	// SPA shell. This is the route precedence that makes single-origin safe.
	t.Run("real routes win over the SPA catch-all", func(t *testing.T) {
		// /healthz is the one route that needs only the pool; it must resolve to
		// its handler, not the shell.
		if rec := get("/healthz"); rec.Code != http.StatusOK ||
			strings.Contains(rec.Body.String(), "SPA_SHELL") {
			t.Errorf("GET /healthz: code=%d body=%q, want its JSON handler, not the shell",
				rec.Code, rec.Body.String())
		}
		// An unmounted path under /api resolves within the /api subtree (404),
		// proving /* does not swallow the API namespace.
		rec := get("/api/__no_such_route__")
		if strings.Contains(rec.Body.String(), "SPA_SHELL") {
			t.Errorf("GET unmounted /api path served the SPA shell — the catch-all is shadowing /api")
		}
		if rec.Code == http.StatusOK {
			t.Errorf("GET unmounted /api path: code=200, want a 4xx from the /api subtree")
		}
	})
}
