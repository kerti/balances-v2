package tags_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/shopspring/decimal"

	"github.com/kerti/balances-v2/backend/internal/db"
	"github.com/kerti/balances-v2/backend/internal/identity"
	"github.com/kerti/balances-v2/backend/internal/repo"
	"github.com/kerti/balances-v2/backend/internal/tags"
	"github.com/kerti/balances-v2/backend/internal/testutil"
)

func get(t *testing.T, router *chi.Mux, path string, user *db.User) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if user != nil {
		req = req.WithContext(identity.WithUser(req.Context(), *user))
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

// GET /tags/breakdown serves the converted, single-month report: decimals as
// strings, an Untagged entry with a null tag_id, and per-household scoping.
// covers: INV-FINANCE-40, INV-TENANCY-12
func TestTagsHandlers_Breakdown(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)
	ctx := context.Background()
	alice := testutil.CreateHouseholdWithUser(t, q, "Alice")
	bob := testutil.CreateHouseholdWithUser(t, q, "Bob")
	jan := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)

	seed := func(u db.User, amount string) {
		a, err := q.CreateAsset(ctx, db.CreateAssetParams{
			HouseholdID: u.HouseholdID, DisplayName: "Acct", Subtype: "bank_account",
			OwnershipType: "joint", NativeCurrency: "IDR", CreatedBy: &u.ID, EntryType: repo.EntryTypeAcquired,
		})
		if err != nil {
			t.Fatalf("CreateAsset: %v", err)
		}
		if _, err := q.CreateAssetSnapshot(ctx, db.CreateAssetSnapshotParams{
			ID: a.ID, HouseholdID: u.HouseholdID, YearMonth: jan,
			Amount: decimal.RequireFromString(amount), Currency: "IDR", CreatedBy: &u.ID,
		}); err != nil {
			t.Fatalf("CreateAssetSnapshot: %v", err)
		}
	}
	seed(alice, "100")
	seed(bob, "555")

	router := chi.NewRouter()
	tags.New(repo.NewTagRepo(tdb.Pool), repo.NewMonthlyReportRepo(tdb.Pool)).Mount(router)

	type body struct {
		ReportingCurrency string `json:"reporting_currency"`
		YearMonth         string `json:"year_month"`
		Tags              []struct {
			TagID       *string `json:"tag_id"`
			Holdings    string  `json:"holdings"`
			Liabilities string  `json:"liabilities"`
		} `json:"tags"`
		MissingFx   []any             `json:"missing_fx"`
		FxRatesUsed map[string]string `json:"fx_rates_used"`
	}
	decode := func(u *db.User) body {
		t.Helper()
		rec := get(t, router, "/tags/breakdown", u)
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d (%s)", rec.Code, rec.Body.String())
		}
		var b body
		if err := json.NewDecoder(rec.Body).Decode(&b); err != nil {
			t.Fatalf("decode: %v", err)
		}
		return b
	}

	b := decode(&alice)
	if b.ReportingCurrency != "IDR" || b.YearMonth == "" {
		t.Errorf("header %+v", b)
	}
	if len(b.Tags) != 1 || b.Tags[0].TagID != nil || b.Tags[0].Holdings != "100" || b.Tags[0].Liabilities != "0" {
		t.Errorf("alice tags %+v, want one Untagged entry 100 / 0", b.Tags)
	}
	if b.MissingFx == nil || b.FxRatesUsed == nil {
		t.Errorf("missing_fx / fx_rates_used must be [] / {} not null: %+v", b)
	}
	if bb := decode(&bob); len(bb.Tags) != 1 || bb.Tags[0].Holdings != "555" {
		t.Errorf("bob tags %+v, want only his 555", bb.Tags)
	}

	if rec := get(t, router, "/tags/breakdown", nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("no session: status %d, want 401", rec.Code)
	}
}
