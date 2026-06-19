package httpserver

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestSPAHandler(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "index.html"), "INDEX")
	if err := os.MkdirAll(filepath.Join(dir, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(dir, "assets", "app.js"), "APP")

	h := spaHandler(dir)
	cases := []struct {
		name, path, want string
	}{
		{"root serves index", "/", "INDEX"},
		{"real asset served", "/assets/app.js", "APP"},
		{"client route falls back", "/investments/123", "INDEX"},
		{"missing path falls back", "/nope.txt", "INDEX"},
		// The Assets section lives at client routes under /assets/ (e.g.
		// assets/bank-accounts in App.tsx), which collide with Vite's build-output
		// dir. An extensionless miss there is a client route, not a stale chunk, so
		// it must serve the SPA shell on a hard refresh — not 404 (#241).
		{"client route under assets falls back", "/assets/bank-accounts", "INDEX"},
		{"nested client route under assets falls back", "/assets/bank-accounts/123", "INDEX"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h(rec, httptest.NewRequest("GET", c.path, nil))
			if got := rec.Body.String(); got != c.want {
				t.Errorf("%s: body = %q, want %q", c.path, got, c.want)
			}
		})
	}

	// A missing build chunk under /assets/ must 404, not fall back to the SPA
	// shell — otherwise a stale hashed-chunk request gets 200 text/html and the
	// browser fails to parse the HTML as a module (#190).
	t.Run("missing asset 404s instead of falling back", func(t *testing.T) {
		rec := httptest.NewRecorder()
		h(rec, httptest.NewRequest("GET", "/assets/SnapshotChartImpl-deadbeef.js", nil))
		if rec.Code != http.StatusNotFound {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
		}
		if got := rec.Body.String(); got == "INDEX" {
			t.Errorf("missing asset served the SPA shell, want a 404 body")
		}
	})

	// A path with .. is rejected outright (http.ServeFile guards it), never
	// serving anything outside the web dir.
	t.Run("traversal is rejected", func(t *testing.T) {
		rec := httptest.NewRecorder()
		h(rec, httptest.NewRequest("GET", "/../etc/passwd", nil))
		if rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
		}
	})
}

func mustWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
