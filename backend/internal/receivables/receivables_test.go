package receivables_test

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
	"github.com/kerti/balances-v2/backend/internal/receivables"
	"github.com/kerti/balances-v2/backend/internal/repo"
	"github.com/kerti/balances-v2/backend/internal/testutil"
)

// fakeNow pins the clock for snapshot future-date validation. Hardcoded
// dates across these tests live in 2026, so any "now" beyond 2026-12 keeps
// them in the past.
var fakeNow = func() time.Time { return time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC) }

// handlerHarness wires real testcontainer DB + real repo + real handlers behind
// a chi router. We intentionally avoid mocking the repo — handler tests are
// thin and the cost of a shared testcontainer is amortised across subtests.
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
	receivables.New(repo.NewReceivableRepo(tdb.Pool), receivables.WithNow(fakeNow)).Mount(r)

	return &handlerHarness{router: r, user: user, pool: tdb.Pool}
}

// seedTag creates a Tag in the harness user's household and returns its id —
// for create-import tests that exercise the Detail-sheet tag-name resolution
// (the /tags routes aren't mounted on this receivable router).
func (h *handlerHarness) seedTag(t *testing.T, name string) uuid.UUID {
	t.Helper()
	ctx := auth.WithUser(context.Background(), h.user)
	tag, err := repo.NewTagRepo(h.pool).CreateTag(ctx, name, "#22c55e")
	if err != nil {
		t.Fatalf("seed tag: %v", err)
	}
	return tag.ID
}

// do issues an authed request as the harness's default user.
func (h *handlerHarness) do(t *testing.T, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	return h.doRaw(t, method, path, body, &h.user)
}

// doRaw lets callers override the auth user (or pass nil for unauthed) and
// supply body as a struct (marshalled to JSON), a string, or raw bytes.
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

// createReceivable is a test convenience that POSTs a minimal receivable and
// returns the persisted row. Subtests use this when they need a fresh
// receivable to mutate without polluting other subtests.
func (h *handlerHarness) createReceivable(t *testing.T, displayName string) db.Receivable {
	t.Helper()
	rec := h.do(t, "POST", "/receivables", map[string]any{
		"display_name":      displayName,
		"ownership_type":    "joint",
		"native_currency":   "IDR",
		"counterparty_name": "Counterparty",
	})
	requireStatus(t, rec, http.StatusCreated)
	return decodeBody[db.Receivable](t, rec)
}

func (h *handlerHarness) createSnapshot(t *testing.T, receivableID uuid.UUID, yearMonth string) db.ReceivableSnapshot {
	t.Helper()
	rec := h.do(t, "POST", "/receivables/"+receivableID.String()+"/snapshots", map[string]any{
		"year_month": yearMonth,
		"amount":     "1000000",
		"currency":   "IDR",
	})
	requireStatus(t, rec, http.StatusCreated)
	return decodeBody[db.ReceivableSnapshot](t, rec)
}

// ----- Tests --------------------------------------------------------------

func TestReceivableHandlers_Create(t *testing.T) {
	h := newHarness(t)

	t.Run("201 happy path", func(t *testing.T) {
		rec := h.do(t, "POST", "/receivables", map[string]any{
			"display_name":      "Loan to brother",
			"ownership_type":    "joint",
			"native_currency":   "IDR",
			"counterparty_name": "Brother",
			"due_date":          "2026-12-31",
		})
		requireStatus(t, rec, http.StatusCreated)
		body := decodeBody[db.Receivable](t, rec)
		if body.DisplayName != "Loan to brother" {
			t.Errorf("display_name: got %q", body.DisplayName)
		}
		if body.CounterpartyName != "Brother" {
			t.Errorf("counterparty_name: got %q", body.CounterpartyName)
		}
		if body.DueDate == nil || body.DueDate.Format("2006-01-02") != "2026-12-31" {
			t.Errorf("due_date: got %v", body.DueDate)
		}
	})

	t.Run("400 invalid json", func(t *testing.T) {
		rec := h.do(t, "POST", "/receivables", "{not-json")
		requireStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("400 missing required display_name", func(t *testing.T) {
		rec := h.do(t, "POST", "/receivables", map[string]any{
			"ownership_type":    "joint",
			"native_currency":   "IDR",
			"counterparty_name": "Brother",
		})
		requireStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("400 bad due_date format", func(t *testing.T) {
		rec := h.do(t, "POST", "/receivables", map[string]any{
			"display_name":      "Test",
			"ownership_type":    "joint",
			"native_currency":   "IDR",
			"counterparty_name": "X",
			"due_date":          "31/12/2026",
		})
		requireStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("400 invalid ownership_type enum", func(t *testing.T) {
		rec := h.do(t, "POST", "/receivables", map[string]any{
			"display_name":      "Test",
			"ownership_type":    "communal",
			"native_currency":   "IDR",
			"counterparty_name": "X",
		})
		requireStatus(t, rec, http.StatusBadRequest)
	})
}

func TestReceivableHandlers_List(t *testing.T) {
	h := newHarness(t)
	created := h.createReceivable(t, "Listed receivable")

	rec := h.do(t, "GET", "/receivables", nil)
	requireStatus(t, rec, http.StatusOK)
	list := decodeBody[[]repo.ReceivableListItem](t, rec)
	if len(list) != 1 {
		t.Fatalf("list length: want 1, got %d", len(list))
	}
	if list[0].Receivable.ID != created.ID {
		t.Errorf("list[0].id: want %s, got %s", created.ID, list[0].Receivable.ID)
	}
	if list[0].LatestSnapshot != nil {
		t.Errorf("latest_snapshot: want nil for fresh receivable, got %+v", list[0].LatestSnapshot)
	}
}

func TestReceivableHandlers_Get(t *testing.T) {
	h := newHarness(t)
	created := h.createReceivable(t, "Get target")

	t.Run("200 happy path", func(t *testing.T) {
		rec := h.do(t, "GET", "/receivables/"+created.ID.String(), nil)
		requireStatus(t, rec, http.StatusOK)
		body := decodeBody[db.Receivable](t, rec)
		if body.ID != created.ID {
			t.Errorf("id: want %s, got %s", created.ID, body.ID)
		}
	})

	t.Run("404 unknown id", func(t *testing.T) {
		rec := h.do(t, "GET", "/receivables/"+uuid.NewString(), nil)
		requireStatus(t, rec, http.StatusNotFound)
	})

	t.Run("400 invalid id format", func(t *testing.T) {
		rec := h.do(t, "GET", "/receivables/not-a-uuid", nil)
		requireStatus(t, rec, http.StatusBadRequest)
	})
}

func TestReceivableHandlers_Update(t *testing.T) {
	h := newHarness(t)
	created := h.createReceivable(t, "Update target")

	t.Run("200 happy path", func(t *testing.T) {
		rec := h.do(t, "PATCH", "/receivables/"+created.ID.String(), map[string]any{
			"display_name":      "Renamed",
			"ownership_type":    "joint",
			"counterparty_name": "New Counterparty",
		})
		requireStatus(t, rec, http.StatusOK)
		body := decodeBody[db.Receivable](t, rec)
		if body.DisplayName != "Renamed" {
			t.Errorf("display_name: want Renamed, got %q", body.DisplayName)
		}
		if body.CounterpartyName != "New Counterparty" {
			t.Errorf("counterparty_name: got %q", body.CounterpartyName)
		}
	})

	t.Run("404 unknown id", func(t *testing.T) {
		rec := h.do(t, "PATCH", "/receivables/"+uuid.NewString(), map[string]any{
			"display_name":      "x",
			"ownership_type":    "joint",
			"counterparty_name": "y",
		})
		requireStatus(t, rec, http.StatusNotFound)
	})

	t.Run("400 missing required display_name", func(t *testing.T) {
		rec := h.do(t, "PATCH", "/receivables/"+created.ID.String(), map[string]any{
			"counterparty_name": "y",
		})
		requireStatus(t, rec, http.StatusBadRequest)
	})
}

func TestReceivableHandlers_Delete(t *testing.T) {
	h := newHarness(t)

	t.Run("204 happy path", func(t *testing.T) {
		created := h.createReceivable(t, "To delete")
		rec := h.do(t, "DELETE", "/receivables/"+created.ID.String(), nil)
		requireStatus(t, rec, http.StatusNoContent)

		// confirm it's gone
		rec = h.do(t, "GET", "/receivables/"+created.ID.String(), nil)
		requireStatus(t, rec, http.StatusNotFound)
	})

	t.Run("404 unknown id", func(t *testing.T) {
		rec := h.do(t, "DELETE", "/receivables/"+uuid.NewString(), nil)
		requireStatus(t, rec, http.StatusNotFound)
	})
}

func TestReceivableHandlers_CreateSnapshot(t *testing.T) {
	h := newHarness(t)
	parent := h.createReceivable(t, "Snapshot parent")

	t.Run("201 happy path with YYYY-MM", func(t *testing.T) {
		rec := h.do(t, "POST", "/receivables/"+parent.ID.String()+"/snapshots", map[string]any{
			"year_month": "2026-05",
			"amount":     "5000000",
			"currency":   "IDR",
			"as_of_date": "2026-05-15",
		})
		requireStatus(t, rec, http.StatusCreated)
		body := decodeBody[db.ReceivableSnapshot](t, rec)
		if !decimal.NewFromInt(5000000).Equal(body.Amount) {
			t.Errorf("amount: want 5000000, got %s", body.Amount.String())
		}
	})

	t.Run("400 bad year_month format", func(t *testing.T) {
		rec := h.do(t, "POST", "/receivables/"+parent.ID.String()+"/snapshots", map[string]any{
			"year_month": "May 2026",
			"amount":     "1000",
			"currency":   "IDR",
		})
		requireStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("400 missing required amount", func(t *testing.T) {
		rec := h.do(t, "POST", "/receivables/"+parent.ID.String()+"/snapshots", map[string]any{
			"year_month": "2026-06",
			"currency":   "IDR",
		})
		requireStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("404 unknown parent receivable", func(t *testing.T) {
		rec := h.do(t, "POST", "/receivables/"+uuid.NewString()+"/snapshots", map[string]any{
			"year_month": "2026-07",
			"amount":     "1000",
			"currency":   "IDR",
		})
		requireStatus(t, rec, http.StatusNotFound)
	})

	// fakeNow = 2030-01-01 UTC; anything past current month / today rejects.
	t.Run("400 future year_month", func(t *testing.T) {
		rec := h.do(t, "POST", "/receivables/"+parent.ID.String()+"/snapshots", map[string]any{
			"year_month": "2030-02",
			"amount":     "1000",
			"currency":   "IDR",
		})
		requireStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("400 future as_of_date", func(t *testing.T) {
		rec := h.do(t, "POST", "/receivables/"+parent.ID.String()+"/snapshots", map[string]any{
			"year_month": "2030-01",
			"amount":     "1000",
			"currency":   "IDR",
			"as_of_date": "2030-01-02",
		})
		requireStatus(t, rec, http.StatusBadRequest)
	})
}

func TestReceivableHandlers_ListSnapshots(t *testing.T) {
	h := newHarness(t)
	parent := h.createReceivable(t, "Snapshot list parent")
	snap := h.createSnapshot(t, parent.ID, "2026-04")

	rec := h.do(t, "GET", "/receivables/"+parent.ID.String()+"/snapshots", nil)
	requireStatus(t, rec, http.StatusOK)
	list := decodeBody[[]db.ReceivableSnapshot](t, rec)
	if len(list) != 1 {
		t.Fatalf("list length: want 1, got %d", len(list))
	}
	if list[0].ID != snap.ID {
		t.Errorf("snapshot id: want %s, got %s", snap.ID, list[0].ID)
	}
}

func TestReceivableHandlers_UpdateSnapshot(t *testing.T) {
	h := newHarness(t)
	parent := h.createReceivable(t, "Snapshot update parent")
	snap := h.createSnapshot(t, parent.ID, "2026-03")

	t.Run("200 happy path", func(t *testing.T) {
		rec := h.do(t, "PATCH",
			"/receivables/"+parent.ID.String()+"/snapshots/"+snap.ID.String(),
			map[string]any{
				"amount":   "2500000",
				"currency": "IDR",
			})
		requireStatus(t, rec, http.StatusOK)
		body := decodeBody[db.ReceivableSnapshot](t, rec)
		if !decimal.NewFromInt(2500000).Equal(body.Amount) {
			t.Errorf("amount: want 2500000, got %s", body.Amount.String())
		}
	})

	t.Run("404 unknown snapshot", func(t *testing.T) {
		rec := h.do(t, "PATCH",
			"/receivables/"+parent.ID.String()+"/snapshots/"+uuid.NewString(),
			map[string]any{
				"amount":   "1",
				"currency": "IDR",
			})
		requireStatus(t, rec, http.StatusNotFound)
	})

	t.Run("400 future as_of_date", func(t *testing.T) {
		rec := h.do(t, "PATCH",
			"/receivables/"+parent.ID.String()+"/snapshots/"+snap.ID.String(),
			map[string]any{
				"amount":     "1",
				"currency":   "IDR",
				"as_of_date": "2030-01-02",
			})
		requireStatus(t, rec, http.StatusBadRequest)
	})
}

func TestReceivableHandlers_DeleteSnapshot(t *testing.T) {
	h := newHarness(t)
	parent := h.createReceivable(t, "Snapshot delete parent")

	t.Run("204 happy path", func(t *testing.T) {
		snap := h.createSnapshot(t, parent.ID, "2026-02")
		rec := h.do(t, "DELETE",
			"/receivables/"+parent.ID.String()+"/snapshots/"+snap.ID.String(), nil)
		requireStatus(t, rec, http.StatusNoContent)
	})

	t.Run("404 unknown snapshot", func(t *testing.T) {
		rec := h.do(t, "DELETE",
			"/receivables/"+parent.ID.String()+"/snapshots/"+uuid.NewString(), nil)
		requireStatus(t, rec, http.StatusNotFound)
	})
}

func TestReceivableHandlers_RequiresAuth(t *testing.T) {
	h := newHarness(t)
	rec := h.doRaw(t, "GET", "/receivables", nil, nil)
	requireStatus(t, rec, http.StatusUnauthorized)
}
