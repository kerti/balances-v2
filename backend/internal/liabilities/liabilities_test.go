package liabilities_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/kerti/balances-v2/backend/internal/auth"
	"github.com/kerti/balances-v2/backend/internal/db"
	"github.com/kerti/balances-v2/backend/internal/liabilities"
	"github.com/kerti/balances-v2/backend/internal/repo"
	"github.com/kerti/balances-v2/backend/internal/testutil"
)

// fakeNow pins the clock for snapshot future-date validation. Hardcoded
// dates across these tests live in 2026, so any "now" beyond 2026-12 keeps
// them in the past.
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
	liabilities.New(repo.NewLiabilityRepo(tdb.Pool), liabilities.WithNow(fakeNow)).Mount(r)

	return &handlerHarness{router: r, user: user, pool: tdb.Pool}
}

// seedTag creates a Tag in the harness user's household and returns its id —
// for create-import tests that exercise the Detail-sheet tag-name resolution
// (the /tags routes aren't mounted on this liability router).
func (h *handlerHarness) seedTag(t *testing.T, name string) uuid.UUID {
	t.Helper()
	ctx := auth.WithUser(context.Background(), h.user)
	tag, err := repo.NewTagRepo(h.pool).CreateTag(ctx, name, "#22c55e")
	if err != nil {
		t.Fatalf("seed tag: %v", err)
	}
	return tag.ID
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

func (h *handlerHarness) createLiability(t *testing.T, displayName, subtype string) db.Liability {
	t.Helper()
	rec := h.do(t, "POST", "/liabilities", map[string]any{
		"display_name":      displayName,
		"subtype":           subtype,
		"ownership_type":    "joint",
		"native_currency":   "IDR",
		"counterparty_name": "Bank",
	})
	requireStatus(t, rec, http.StatusCreated)
	return decodeBody[db.Liability](t, rec)
}

func (h *handlerHarness) createSnapshot(t *testing.T, liabilityID uuid.UUID, yearMonth string) db.LiabilitySnapshot {
	t.Helper()
	rec := h.do(t, "POST", "/liabilities/"+liabilityID.String()+"/snapshots", map[string]any{
		"year_month": yearMonth,
		"amount":     "10000000",
		"currency":   "IDR",
	})
	requireStatus(t, rec, http.StatusCreated)
	return decodeBody[db.LiabilitySnapshot](t, rec)
}

// ----- Tests --------------------------------------------------------------

func TestLiabilityHandlers_Create(t *testing.T) {
	h := newHarness(t)

	t.Run("201 happy path personal", func(t *testing.T) {
		rec := h.do(t, "POST", "/liabilities", map[string]any{
			"display_name":      "Personal loan",
			"subtype":           "personal",
			"ownership_type":    "joint",
			"native_currency":   "IDR",
			"counterparty_name": "Friend",
			"principal":         "5000000",
			"interest_rate":     "5.5",
			"term_months":       12,
			"start_date":        "2026-01-01",
			"maturity_date":     "2027-01-01",
		})
		requireStatus(t, rec, http.StatusCreated)
		body := decodeBody[db.Liability](t, rec)
		if body.Subtype != "personal" {
			t.Errorf("subtype: got %q", body.Subtype)
		}
		if body.Principal == nil || !decimal.NewFromInt(5000000).Equal(*body.Principal) {
			t.Errorf("principal: got %v", body.Principal)
		}
	})

	t.Run("201 happy path institutional", func(t *testing.T) {
		rec := h.do(t, "POST", "/liabilities", map[string]any{
			"display_name":      "Mortgage",
			"subtype":           "institutional",
			"ownership_type":    "joint",
			"native_currency":   "IDR",
			"counterparty_name": "Bank",
		})
		requireStatus(t, rec, http.StatusCreated)
	})

	t.Run("400 invalid json", func(t *testing.T) {
		rec := h.do(t, "POST", "/liabilities", "{not-json")
		requireStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("400 invalid subtype enum", func(t *testing.T) {
		rec := h.do(t, "POST", "/liabilities", map[string]any{
			"display_name":      "X",
			"subtype":           "corporate",
			"ownership_type":    "joint",
			"native_currency":   "IDR",
			"counterparty_name": "Y",
		})
		requireStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("400 bad start_date format", func(t *testing.T) {
		rec := h.do(t, "POST", "/liabilities", map[string]any{
			"display_name":      "X",
			"subtype":           "personal",
			"ownership_type":    "joint",
			"native_currency":   "IDR",
			"counterparty_name": "Y",
			"start_date":        "2026/01/01",
		})
		requireStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("400 bad maturity_date format", func(t *testing.T) {
		rec := h.do(t, "POST", "/liabilities", map[string]any{
			"display_name":      "X",
			"subtype":           "personal",
			"ownership_type":    "joint",
			"native_currency":   "IDR",
			"counterparty_name": "Y",
			"maturity_date":     "not-a-date",
		})
		requireStatus(t, rec, http.StatusBadRequest)
	})
}

func TestLiabilityHandlers_List(t *testing.T) {
	h := newHarness(t)
	personal := h.createLiability(t, "Personal one", "personal")
	institutional := h.createLiability(t, "Bank one", "institutional")

	t.Run("200 returns all when no filter", func(t *testing.T) {
		rec := h.do(t, "GET", "/liabilities", nil)
		requireStatus(t, rec, http.StatusOK)
		list := decodeBody[[]repo.LiabilityListItem](t, rec)
		if len(list) != 2 {
			t.Fatalf("list length: want 2, got %d", len(list))
		}
	})

	t.Run("200 filters by subtype=personal", func(t *testing.T) {
		rec := h.do(t, "GET", "/liabilities?subtype=personal", nil)
		requireStatus(t, rec, http.StatusOK)
		list := decodeBody[[]repo.LiabilityListItem](t, rec)
		if len(list) != 1 {
			t.Fatalf("personal filter: want 1, got %d", len(list))
		}
		if list[0].Liability.ID != personal.ID {
			t.Errorf("filtered id: want %s, got %s", personal.ID, list[0].Liability.ID)
		}
	})

	t.Run("200 filters by subtype=institutional", func(t *testing.T) {
		rec := h.do(t, "GET", "/liabilities?subtype=institutional", nil)
		requireStatus(t, rec, http.StatusOK)
		list := decodeBody[[]repo.LiabilityListItem](t, rec)
		if len(list) != 1 {
			t.Fatalf("institutional filter: want 1, got %d", len(list))
		}
		if list[0].Liability.ID != institutional.ID {
			t.Errorf("filtered id: want %s, got %s", institutional.ID, list[0].Liability.ID)
		}
	})
}

func TestLiabilityHandlers_Get(t *testing.T) {
	h := newHarness(t)
	created := h.createLiability(t, "Get target", "personal")

	t.Run("200 happy path", func(t *testing.T) {
		rec := h.do(t, "GET", "/liabilities/"+created.ID.String(), nil)
		requireStatus(t, rec, http.StatusOK)
		body := decodeBody[db.Liability](t, rec)
		if body.ID != created.ID {
			t.Errorf("id: want %s, got %s", created.ID, body.ID)
		}
	})

	t.Run("404 unknown id", func(t *testing.T) {
		rec := h.do(t, "GET", "/liabilities/"+uuid.NewString(), nil)
		requireStatus(t, rec, http.StatusNotFound)
	})

	t.Run("400 invalid id format", func(t *testing.T) {
		rec := h.do(t, "GET", "/liabilities/not-a-uuid", nil)
		requireStatus(t, rec, http.StatusBadRequest)
	})
}

func TestLiabilityHandlers_Update(t *testing.T) {
	h := newHarness(t)
	created := h.createLiability(t, "Update target", "institutional")

	t.Run("200 happy path", func(t *testing.T) {
		rec := h.do(t, "PATCH", "/liabilities/"+created.ID.String(), map[string]any{
			"display_name":      "Renamed",
			"ownership_type":    "joint",
			"counterparty_name": "New Bank",
			"interest_rate":     "6.25",
		})
		requireStatus(t, rec, http.StatusOK)
		body := decodeBody[db.Liability](t, rec)
		if body.DisplayName != "Renamed" {
			t.Errorf("display_name: want Renamed, got %q", body.DisplayName)
		}
	})

	t.Run("404 unknown id", func(t *testing.T) {
		rec := h.do(t, "PATCH", "/liabilities/"+uuid.NewString(), map[string]any{
			"display_name":      "x",
			"ownership_type":    "joint",
			"counterparty_name": "y",
		})
		requireStatus(t, rec, http.StatusNotFound)
	})

	t.Run("400 missing required display_name", func(t *testing.T) {
		rec := h.do(t, "PATCH", "/liabilities/"+created.ID.String(), map[string]any{
			"counterparty_name": "y",
		})
		requireStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("400 bad maturity_date format", func(t *testing.T) {
		rec := h.do(t, "PATCH", "/liabilities/"+created.ID.String(), map[string]any{
			"display_name":      "X",
			"counterparty_name": "Y",
			"maturity_date":     "31-12-2026",
		})
		requireStatus(t, rec, http.StatusBadRequest)
	})
}

func TestLiabilityHandlers_Delete(t *testing.T) {
	h := newHarness(t)

	t.Run("204 happy path", func(t *testing.T) {
		created := h.createLiability(t, "To delete", "personal")
		rec := h.do(t, "DELETE", "/liabilities/"+created.ID.String(), nil)
		requireStatus(t, rec, http.StatusNoContent)

		rec = h.do(t, "GET", "/liabilities/"+created.ID.String(), nil)
		requireStatus(t, rec, http.StatusNotFound)
	})

	t.Run("404 unknown id", func(t *testing.T) {
		rec := h.do(t, "DELETE", "/liabilities/"+uuid.NewString(), nil)
		requireStatus(t, rec, http.StatusNotFound)
	})
}

func TestLiabilityHandlers_CreateSnapshot(t *testing.T) {
	h := newHarness(t)
	parent := h.createLiability(t, "Snapshot parent", "personal")

	t.Run("201 happy path", func(t *testing.T) {
		rec := h.do(t, "POST", "/liabilities/"+parent.ID.String()+"/snapshots", map[string]any{
			"year_month": "2026-05",
			"amount":     "4500000",
			"currency":   "IDR",
			"as_of_date": "2026-05-15",
		})
		requireStatus(t, rec, http.StatusCreated)
		body := decodeBody[db.LiabilitySnapshot](t, rec)
		if !decimal.NewFromInt(4500000).Equal(body.Amount) {
			t.Errorf("amount: want 4500000, got %s", body.Amount.String())
		}
	})

	t.Run("400 bad year_month", func(t *testing.T) {
		rec := h.do(t, "POST", "/liabilities/"+parent.ID.String()+"/snapshots", map[string]any{
			"year_month": "May 2026",
			"amount":     "1000",
			"currency":   "IDR",
		})
		requireStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("400 missing required currency", func(t *testing.T) {
		rec := h.do(t, "POST", "/liabilities/"+parent.ID.String()+"/snapshots", map[string]any{
			"year_month": "2026-06",
			"amount":     "1000",
		})
		requireStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("404 unknown parent liability", func(t *testing.T) {
		rec := h.do(t, "POST", "/liabilities/"+uuid.NewString()+"/snapshots", map[string]any{
			"year_month": "2026-07",
			"amount":     "1000",
			"currency":   "IDR",
		})
		requireStatus(t, rec, http.StatusNotFound)
	})

	// fakeNow = 2030-01-01 UTC; anything past current month / today rejects.
	t.Run("400 future year_month", func(t *testing.T) {
		rec := h.do(t, "POST", "/liabilities/"+parent.ID.String()+"/snapshots", map[string]any{
			"year_month": "2030-02",
			"amount":     "1000",
			"currency":   "IDR",
		})
		requireStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("400 future as_of_date", func(t *testing.T) {
		rec := h.do(t, "POST", "/liabilities/"+parent.ID.String()+"/snapshots", map[string]any{
			"year_month": "2030-01",
			"amount":     "1000",
			"currency":   "IDR",
			"as_of_date": "2030-01-02",
		})
		requireStatus(t, rec, http.StatusBadRequest)
	})
}

func TestLiabilityHandlers_ListSnapshots(t *testing.T) {
	h := newHarness(t)
	parent := h.createLiability(t, "Snapshot list parent", "personal")
	snap := h.createSnapshot(t, parent.ID, "2026-04")

	rec := h.do(t, "GET", "/liabilities/"+parent.ID.String()+"/snapshots", nil)
	requireStatus(t, rec, http.StatusOK)
	list := decodeBody[[]db.LiabilitySnapshot](t, rec)
	if len(list) != 1 {
		t.Fatalf("list length: want 1, got %d", len(list))
	}
	if list[0].ID != snap.ID {
		t.Errorf("snapshot id: want %s, got %s", snap.ID, list[0].ID)
	}
}

func TestLiabilityHandlers_UpdateSnapshot(t *testing.T) {
	h := newHarness(t)
	parent := h.createLiability(t, "Snapshot update parent", "personal")
	snap := h.createSnapshot(t, parent.ID, "2026-03")

	t.Run("200 happy path", func(t *testing.T) {
		rec := h.do(t, "PATCH",
			"/liabilities/"+parent.ID.String()+"/snapshots/"+snap.ID.String(),
			map[string]any{
				"amount":   "8000000",
				"currency": "IDR",
			})
		requireStatus(t, rec, http.StatusOK)
		body := decodeBody[db.LiabilitySnapshot](t, rec)
		if !decimal.NewFromInt(8000000).Equal(body.Amount) {
			t.Errorf("amount: want 8000000, got %s", body.Amount.String())
		}
	})

	t.Run("404 unknown snapshot", func(t *testing.T) {
		rec := h.do(t, "PATCH",
			"/liabilities/"+parent.ID.String()+"/snapshots/"+uuid.NewString(),
			map[string]any{
				"amount":   "1",
				"currency": "IDR",
			})
		requireStatus(t, rec, http.StatusNotFound)
	})

	t.Run("400 future as_of_date", func(t *testing.T) {
		rec := h.do(t, "PATCH",
			"/liabilities/"+parent.ID.String()+"/snapshots/"+snap.ID.String(),
			map[string]any{
				"amount":     "1",
				"currency":   "IDR",
				"as_of_date": "2030-01-02",
			})
		requireStatus(t, rec, http.StatusBadRequest)
	})
}

func TestLiabilityHandlers_DeleteSnapshot(t *testing.T) {
	h := newHarness(t)
	parent := h.createLiability(t, "Snapshot delete parent", "personal")

	t.Run("204 happy path", func(t *testing.T) {
		snap := h.createSnapshot(t, parent.ID, "2026-02")
		rec := h.do(t, "DELETE",
			"/liabilities/"+parent.ID.String()+"/snapshots/"+snap.ID.String(), nil)
		requireStatus(t, rec, http.StatusNoContent)
	})

	t.Run("404 unknown snapshot", func(t *testing.T) {
		rec := h.do(t, "DELETE",
			"/liabilities/"+parent.ID.String()+"/snapshots/"+uuid.NewString(), nil)
		requireStatus(t, rec, http.StatusNotFound)
	})
}

func TestLiabilityHandlers_RequiresAuth(t *testing.T) {
	h := newHarness(t)
	rec := h.doRaw(t, "GET", "/liabilities", nil, nil)
	requireStatus(t, rec, http.StatusUnauthorized)
}
