package reports_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/shopspring/decimal"

	"github.com/kerti/balances-v2/backend/internal/db"
	"github.com/kerti/balances-v2/backend/internal/identity"
	"github.com/kerti/balances-v2/backend/internal/repo"
	"github.com/kerti/balances-v2/backend/internal/reports"
	"github.com/kerti/balances-v2/backend/internal/testutil"
)

type reportDTO struct {
	YearMonth         string         `json:"year_month"`
	ReportingCurrency string         `json:"reporting_currency"`
	NWTotal           string         `json:"nw_total"`
	UserBreakdowns    map[string]any `json:"user_breakdowns"`
	StalePositions    []any          `json:"stale_positions"`
	FxRatesUsed       map[string]any `json:"fx_rates_used"`
	MissingFx         []any          `json:"missing_fx"`
}

// Real DB + repo + handlers behind chi, with a seeded joint bank account whose
// Jan-2026 snapshot is 100 IDR — so the report engine has something to produce.
func newHarness(t *testing.T) (*chi.Mux, db.User) {
	t.Helper()
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)
	user := testutil.CreateHouseholdWithUser(t, q, "Alice")

	ctx := context.Background()
	asset, err := q.CreateAsset(ctx, db.CreateAssetParams{
		HouseholdID: user.HouseholdID, DisplayName: "Acct", Subtype: "bank_account",
		OwnershipType: "joint", NativeCurrency: "IDR", CreatedBy: &user.ID,
		EntryType: repo.EntryTypeAcquired,
	})
	if err != nil {
		t.Fatalf("CreateAsset: %v", err)
	}
	if _, err := q.CreateAssetSnapshot(ctx, db.CreateAssetSnapshotParams{
		ID: asset.ID, HouseholdID: user.HouseholdID,
		YearMonth: time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
		Amount:    decimal.NewFromInt(100), Currency: "IDR", CreatedBy: &user.ID,
	}); err != nil {
		t.Fatalf("CreateAssetSnapshot: %v", err)
	}

	r := chi.NewRouter()
	reports.New(repo.NewMonthlyReportRepo(tdb.Pool)).Mount(r)
	return r, user
}

func do(t *testing.T, router *chi.Mux, path string, user *db.User) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if user != nil {
		req = req.WithContext(identity.WithUser(req.Context(), *user))
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestReportsHandlers_List(t *testing.T) {
	router, user := newHarness(t)
	rec := do(t, router, "/reports", &user)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d (%s)", rec.Code, rec.Body.String())
	}
	var list []reportDTO
	if err := json.NewDecoder(rec.Body).Decode(&list); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(list) == 0 {
		t.Fatalf("empty report list")
	}
	var jan *reportDTO
	for i := range list {
		if list[i].YearMonth[:7] == "2026-01" {
			jan = &list[i]
		}
	}
	if jan == nil {
		t.Fatalf("no 2026-01 report in %d months", len(list))
	}
	if jan.NWTotal != "100" {
		t.Errorf("2026-01 nw_total: got %q, want 100", jan.NWTotal)
	}
	if jan.ReportingCurrency != "IDR" {
		t.Errorf("reporting_currency: got %q", jan.ReportingCurrency)
	}
	if _, ok := jan.UserBreakdowns["joint"]; !ok {
		t.Errorf("user_breakdowns missing joint key: %v", jan.UserBreakdowns)
	}
}

func TestReportsHandlers_Get(t *testing.T) {
	router, user := newHarness(t)

	t.Run("200 known month", func(t *testing.T) {
		rec := do(t, router, "/reports/2026-01", &user)
		if rec.Code != http.StatusOK {
			t.Fatalf("status: got %d (%s)", rec.Code, rec.Body.String())
		}
		var r reportDTO
		_ = json.NewDecoder(rec.Body).Decode(&r)
		if r.NWTotal != "100" {
			t.Errorf("nw_total: got %q, want 100", r.NWTotal)
		}
	})

	t.Run("404 month out of range", func(t *testing.T) {
		rec := do(t, router, "/reports/1999-01", &user)
		if rec.Code != http.StatusNotFound {
			t.Errorf("status: got %d, want 404", rec.Code)
		}
	})

	t.Run("400 bad year_month", func(t *testing.T) {
		rec := do(t, router, "/reports/not-a-month", &user)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("status: got %d, want 400", rec.Code)
		}
	})
}

func post(t *testing.T, router *chi.Mux, path string, user *db.User) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, nil)
	if user != nil {
		req = req.WithContext(identity.WithUser(req.Context(), *user))
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestReportsHandlers_Rebuild(t *testing.T) {
	router, user := newHarness(t)

	t.Run("rebuild all returns refreshed series", func(t *testing.T) {
		rec := post(t, router, "/reports/rebuild", &user)
		if rec.Code != http.StatusOK {
			t.Fatalf("status: got %d (%s)", rec.Code, rec.Body.String())
		}
		var list []reportDTO
		if err := json.NewDecoder(rec.Body).Decode(&list); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(list) == 0 {
			t.Fatalf("empty report list after rebuild")
		}
	})

	t.Run("rebuild month returns the month", func(t *testing.T) {
		rec := post(t, router, "/reports/2026-01/rebuild", &user)
		if rec.Code != http.StatusOK {
			t.Fatalf("status: got %d (%s)", rec.Code, rec.Body.String())
		}
		var r reportDTO
		_ = json.NewDecoder(rec.Body).Decode(&r)
		if r.NWTotal != "100" {
			t.Errorf("nw_total: got %q, want 100", r.NWTotal)
		}
	})

	t.Run("rebuild month out of range is 404", func(t *testing.T) {
		rec := post(t, router, "/reports/1999-01/rebuild", &user)
		if rec.Code != http.StatusNotFound {
			t.Errorf("status: got %d, want 404", rec.Code)
		}
	})

	t.Run("rebuild month bad year_month is 400", func(t *testing.T) {
		rec := post(t, router, "/reports/not-a-month/rebuild", &user)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("status: got %d, want 400", rec.Code)
		}
	})

	t.Run("rebuild requires auth", func(t *testing.T) {
		rec := post(t, router, "/reports/rebuild", nil)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status: got %d, want 401", rec.Code)
		}
	})
}

func TestReportsHandlers_PDF(t *testing.T) {
	router, user := newHarness(t)

	t.Run("200 returns a PDF with an attachment filename", func(t *testing.T) {
		rec := do(t, router, "/reports/2026-01/pdf", &user)
		if rec.Code != http.StatusOK {
			t.Fatalf("status: got %d (%s)", rec.Code, rec.Body.String())
		}
		if ct := rec.Header().Get("Content-Type"); ct != "application/pdf" {
			t.Errorf("content-type: got %q, want application/pdf", ct)
		}
		if cd := rec.Header().Get("Content-Disposition"); !strings.Contains(cd, `filename="Balances_2026-01.pdf"`) {
			t.Errorf("content-disposition: got %q", cd)
		}
		if !bytes.HasPrefix(rec.Body.Bytes(), []byte("%PDF-")) {
			t.Errorf("body is not a PDF (prefix %q)", rec.Body.Bytes()[:min(8, rec.Body.Len())])
		}
	})

	t.Run("404 month out of range", func(t *testing.T) {
		rec := do(t, router, "/reports/1999-01/pdf", &user)
		if rec.Code != http.StatusNotFound {
			t.Errorf("status: got %d, want 404", rec.Code)
		}
	})

	t.Run("400 bad year_month", func(t *testing.T) {
		rec := do(t, router, "/reports/not-a-month/pdf", &user)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("status: got %d, want 400", rec.Code)
		}
	})

	t.Run("requires auth", func(t *testing.T) {
		rec := do(t, router, "/reports/2026-01/pdf", nil)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status: got %d, want 401", rec.Code)
		}
	})
}

func TestReportsHandlers_RequiresAuth(t *testing.T) {
	router, _ := newHarness(t)
	rec := do(t, router, "/reports", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want 401", rec.Code)
	}
}
