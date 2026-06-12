package investments_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kerti/balances-v2/backend/internal/auth"
	"github.com/kerti/balances-v2/backend/internal/db"
	"github.com/kerti/balances-v2/backend/internal/investments"
	"github.com/kerti/balances-v2/backend/internal/repo"
	"github.com/kerti/balances-v2/backend/internal/testutil"
)

// fakeNow pins the clock for snapshot + transaction future-date validation.
// Hardcoded dates across these tests live in 2026, so any "now" beyond
// 2026-12 keeps them in the past.
var fakeNow = func() time.Time { return time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC) }

type handlerHarness struct {
	router *chi.Mux
	user   db.User
	pool   *pgxpool.Pool
}

func newHarness(t *testing.T) *handlerHarness {
	t.Helper()
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)
	user := testutil.CreateHouseholdWithUser(t, q, "Alice")

	r := chi.NewRouter()
	investments.New(repo.NewInvestmentRepo(tdb.Pool), investments.WithNow(fakeNow)).Mount(r)

	return &handlerHarness{router: r, user: user, pool: tdb.Pool}
}

func (h *handlerHarness) do(t *testing.T, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	return h.doRaw(t, method, path, body, &h.user)
}

func (h *handlerHarness) doRaw(t *testing.T, method, path string, body any, user *db.User) *httptest.ResponseRecorder {
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
		req = req.WithContext(auth.WithUser(req.Context(), *user))
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

func TestInvestmentHandlers_RequiresAuth(t *testing.T) {
	h := newHarness(t)
	t.Run("stocks list", func(t *testing.T) {
		rec := h.doRaw(t, "GET", "/investments/stocks", nil, nil)
		requireStatus(t, rec, http.StatusUnauthorized)
	})
	t.Run("bonds list", func(t *testing.T) {
		rec := h.doRaw(t, "GET", "/investments/bonds", nil, nil)
		requireStatus(t, rec, http.StatusUnauthorized)
	})
	t.Run("time-deposits list", func(t *testing.T) {
		rec := h.doRaw(t, "GET", "/investments/time-deposits", nil, nil)
		requireStatus(t, rec, http.StatusUnauthorized)
	})
}
