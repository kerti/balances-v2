package fxrates_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/kerti/balances-v2/backend/internal/db"
	"github.com/kerti/balances-v2/backend/internal/fxrates"
	"github.com/kerti/balances-v2/backend/internal/identity"
	"github.com/kerti/balances-v2/backend/internal/repo"
	"github.com/kerti/balances-v2/backend/internal/testutil"
)

// Real testcontainer DB + real repo + real handlers behind chi, auth injected
// via context — the established HTTP-handler harness (HANDOFF Phase 2c).
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
	fxrates.New(repo.NewFxRateRepo(tdb.Pool)).Mount(r)
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

func (h *harness) createUSD(t *testing.T) db.FxRate {
	t.Helper()
	rec := h.do(t, "POST", "/fx-rates", map[string]any{
		"year_month": "2026-01", "currency": "USD", "rate": "16000",
	})
	requireStatus(t, rec, http.StatusCreated)
	var fr db.FxRate
	if err := json.NewDecoder(rec.Body).Decode(&fr); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return fr
}

func TestFxRatesHandlers_Create(t *testing.T) {
	h := newHarness(t)

	t.Run("201 happy path", func(t *testing.T) {
		fr := h.createUSD(t)
		if fr.Currency != "USD" {
			t.Errorf("currency: got %q", fr.Currency)
		}
		if !fr.Rate.Equal(decimal.RequireFromString("16000")) {
			t.Errorf("rate: got %s", fr.Rate)
		}
	})
	t.Run("409 duplicate month+currency", func(t *testing.T) {
		rec := h.do(t, "POST", "/fx-rates", map[string]any{
			"year_month": "2026-01", "currency": "USD", "rate": "17000",
		})
		requireStatus(t, rec, http.StatusConflict)
	})
	t.Run("400 invalid json", func(t *testing.T) {
		requireStatus(t, h.do(t, "POST", "/fx-rates", "{nope"), http.StatusBadRequest)
	})
	t.Run("400 bad currency code", func(t *testing.T) {
		requireStatus(t, h.do(t, "POST", "/fx-rates", map[string]any{
			"year_month": "2026-02", "currency": "DOLLARS", "rate": "1",
		}), http.StatusBadRequest)
	})
	t.Run("400 bad year_month", func(t *testing.T) {
		requireStatus(t, h.do(t, "POST", "/fx-rates", map[string]any{
			"year_month": "Jan 2026", "currency": "EUR", "rate": "1",
		}), http.StatusBadRequest)
	})
	t.Run("400 missing rate", func(t *testing.T) {
		requireStatus(t, h.do(t, "POST", "/fx-rates", map[string]any{
			"year_month": "2026-02", "currency": "EUR",
		}), http.StatusBadRequest)
	})
	t.Run("400 zero rate", func(t *testing.T) {
		requireStatus(t, h.do(t, "POST", "/fx-rates", map[string]any{
			"year_month": "2026-02", "currency": "EUR", "rate": "0",
		}), http.StatusBadRequest)
	})
}

func TestFxRatesHandlers_ListUpdateDelete(t *testing.T) {
	h := newHarness(t)
	fr := h.createUSD(t)

	t.Run("list returns it", func(t *testing.T) {
		rec := h.do(t, "GET", "/fx-rates", nil)
		requireStatus(t, rec, http.StatusOK)
		var list []db.FxRate
		_ = json.NewDecoder(rec.Body).Decode(&list)
		if len(list) != 1 || list[0].ID != fr.ID {
			t.Fatalf("list: got %+v", list)
		}
	})
	t.Run("update rate 200", func(t *testing.T) {
		rec := h.do(t, "PATCH", "/fx-rates/"+fr.ID.String(), map[string]any{"rate": "16500"})
		requireStatus(t, rec, http.StatusOK)
	})
	t.Run("update unknown 404", func(t *testing.T) {
		rec := h.do(t, "PATCH", "/fx-rates/"+uuid.NewString(), map[string]any{"rate": "1"})
		requireStatus(t, rec, http.StatusNotFound)
	})
	t.Run("update invalid id 400", func(t *testing.T) {
		requireStatus(t, h.do(t, "PATCH", "/fx-rates/not-a-uuid", map[string]any{"rate": "1"}), http.StatusBadRequest)
	})
	t.Run("delete 204 then gone", func(t *testing.T) {
		requireStatus(t, h.do(t, "DELETE", "/fx-rates/"+fr.ID.String(), nil), http.StatusNoContent)
		rec := h.do(t, "GET", "/fx-rates", nil)
		var list []db.FxRate
		_ = json.NewDecoder(rec.Body).Decode(&list)
		if len(list) != 0 {
			t.Errorf("after delete: got %d rows", len(list))
		}
	})
	t.Run("delete unknown 404", func(t *testing.T) {
		requireStatus(t, h.do(t, "DELETE", "/fx-rates/"+uuid.NewString(), nil), http.StatusNotFound)
	})
}

func TestFxRatesHandlers_RequiresAuth(t *testing.T) {
	h := newHarness(t)
	requireStatus(t, h.doAs(t, "GET", "/fx-rates", nil, nil), http.StatusUnauthorized)
}
