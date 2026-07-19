package inflationrates_test

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
	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/kerti/balances-v2/backend/internal/db"
	"github.com/kerti/balances-v2/backend/internal/identity"
	"github.com/kerti/balances-v2/backend/internal/inflationrates"
	"github.com/kerti/balances-v2/backend/internal/repo"
	"github.com/kerti/balances-v2/backend/internal/testutil"
)

// Real testcontainer DB + real repo + real handlers behind chi, auth injected
// via context — mirrors the fxrates handler harness.
type harness struct {
	router *chi.Mux
	user   db.User
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)
	user := testutil.CreateHouseholdWithUser(t, q, "Alice")
	r := chi.NewRouter()
	inflationrates.New(repo.NewInflationRateRepo(tdb.Pool)).Mount(r)
	return &harness{router: r, user: user}
}

func (h *harness) do(t *testing.T, method, path string, body any) *httptest.ResponseRecorder {
	return h.doAs(t, method, path, body, &h.user)
}

func (h *harness) doAs(t *testing.T, method, path string, body any, user *db.User) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	switch v := body.(type) {
	case nil:
		reader = nil
	case string:
		reader = strings.NewReader(v)
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
		req = req.WithContext(identity.WithUser(req.Context(), *user))
	}
	rec := httptest.NewRecorder()
	h.router.ServeHTTP(rec, req)
	return rec
}

func requireStatus(t *testing.T, rec *httptest.ResponseRecorder, want int) {
	t.Helper()
	if rec.Code != want {
		t.Fatalf("status: want %d, got %d (body: %s)", want, rec.Code, rec.Body.String())
	}
}

func (h *harness) create(t *testing.T, ym, rate string) db.InflationRate {
	t.Helper()
	rec := h.do(t, "POST", "/inflation-rates", map[string]any{"year_month": ym, "rate": rate})
	requireStatus(t, rec, http.StatusCreated)
	var ir db.InflationRate
	if err := json.NewDecoder(rec.Body).Decode(&ir); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return ir
}

func TestInflationRatesHandlers_Create(t *testing.T) {
	h := newHarness(t)

	t.Run("201 happy path", func(t *testing.T) {
		ir := h.create(t, "2026-01", "3.5")
		if !ir.Rate.Equal(decimal.RequireFromString("3.5")) {
			t.Errorf("rate: got %s", ir.Rate)
		}
	})
	t.Run("201 negative rate accepted (deflation)", func(t *testing.T) {
		ir := h.create(t, "2026-03", "-1.2")
		if !ir.Rate.Equal(decimal.RequireFromString("-1.2")) {
			t.Errorf("rate: got %s, want -1.2", ir.Rate)
		}
	})
	t.Run("409 duplicate month", func(t *testing.T) {
		rec := h.do(t, "POST", "/inflation-rates", map[string]any{"year_month": "2026-01", "rate": "4"})
		requireStatus(t, rec, http.StatusConflict)
	})
	t.Run("400 invalid json", func(t *testing.T) {
		requireStatus(t, h.do(t, "POST", "/inflation-rates", "{nope"), http.StatusBadRequest)
	})
	t.Run("400 bad year_month", func(t *testing.T) {
		requireStatus(t, h.do(t, "POST", "/inflation-rates", map[string]any{
			"year_month": "Jan 2026", "rate": "3",
		}), http.StatusBadRequest)
	})
	t.Run("400 missing rate", func(t *testing.T) {
		requireStatus(t, h.do(t, "POST", "/inflation-rates", map[string]any{
			"year_month": "2026-02",
		}), http.StatusBadRequest)
	})
	t.Run("400 rate at or below -100", func(t *testing.T) {
		requireStatus(t, h.do(t, "POST", "/inflation-rates", map[string]any{
			"year_month": "2026-04", "rate": "-100",
		}), http.StatusBadRequest)
	})
	t.Run("400 rate above 1000", func(t *testing.T) {
		requireStatus(t, h.do(t, "POST", "/inflation-rates", map[string]any{
			"year_month": "2026-04", "rate": "1000.01",
		}), http.StatusBadRequest)
	})
	t.Run("201 full YYYY-MM-DD is normalized to first-of-month", func(t *testing.T) {
		ir := h.create(t, "2026-07-15", "2")
		if ir.YearMonth.Day() != 1 || ir.YearMonth.Month() != time.July || ir.YearMonth.Year() != 2026 {
			t.Errorf("year_month: got %s, want 2026-07-01", ir.YearMonth.Format("2006-01-02"))
		}
	})
}

func TestInflationRatesHandlers_ListUpdateDelete(t *testing.T) {
	h := newHarness(t)
	created := h.create(t, "2026-05", "3")

	t.Run("list returns it", func(t *testing.T) {
		rec := h.do(t, "GET", "/inflation-rates", nil)
		requireStatus(t, rec, http.StatusOK)
		var list []db.InflationRate
		if err := json.NewDecoder(rec.Body).Decode(&list); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(list) != 1 {
			t.Fatalf("list len: got %d, want 1", len(list))
		}
	})
	t.Run("update rate 200", func(t *testing.T) {
		rec := h.do(t, "PATCH", "/inflation-rates/"+created.ID.String(), map[string]any{"rate": "2.5"})
		requireStatus(t, rec, http.StatusOK)
	})
	t.Run("update unknown 404", func(t *testing.T) {
		rec := h.do(t, "PATCH", "/inflation-rates/"+uuid.NewString(), map[string]any{"rate": "1"})
		requireStatus(t, rec, http.StatusNotFound)
	})
	t.Run("update invalid id 400", func(t *testing.T) {
		requireStatus(t, h.do(t, "PATCH", "/inflation-rates/not-a-uuid", map[string]any{"rate": "1"}), http.StatusBadRequest)
	})
	t.Run("update invalid json 400", func(t *testing.T) {
		requireStatus(t, h.do(t, "PATCH", "/inflation-rates/"+created.ID.String(), "{nope"), http.StatusBadRequest)
	})
	t.Run("update missing rate 400", func(t *testing.T) {
		requireStatus(t, h.do(t, "PATCH", "/inflation-rates/"+created.ID.String(), map[string]any{}), http.StatusBadRequest)
	})
	t.Run("update rate out of bounds 400", func(t *testing.T) {
		requireStatus(t, h.do(t, "PATCH", "/inflation-rates/"+created.ID.String(), map[string]any{"rate": "-100"}), http.StatusBadRequest)
	})
	t.Run("delete 204 then gone", func(t *testing.T) {
		requireStatus(t, h.do(t, "DELETE", "/inflation-rates/"+created.ID.String(), nil), http.StatusNoContent)
		rec := h.do(t, "GET", "/inflation-rates", nil)
		var list []db.InflationRate
		_ = json.NewDecoder(rec.Body).Decode(&list)
		if len(list) != 0 {
			t.Fatalf("after delete list len: got %d, want 0", len(list))
		}
	})
	t.Run("delete unknown 404", func(t *testing.T) {
		requireStatus(t, h.do(t, "DELETE", "/inflation-rates/"+uuid.NewString(), nil), http.StatusNotFound)
	})
}
