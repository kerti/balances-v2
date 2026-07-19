package income_test

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
	"github.com/kerti/balances-v2/backend/internal/identity"
	"github.com/kerti/balances-v2/backend/internal/income"
	"github.com/kerti/balances-v2/backend/internal/repo"
	"github.com/kerti/balances-v2/backend/internal/testutil"
)

// handlerHarness wires a real testcontainer DB + real repo + real handlers
// behind a chi router. Same harness pattern as the position-group HTTP
// packages — see HANDOFF "HTTP handler coverage Phase 2c" notes.
type handlerHarness struct {
	router *chi.Mux
	user   db.User
}

func newHarness(t *testing.T) *handlerHarness {
	t.Helper()
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)
	user := testutil.CreateHouseholdWithUser(t, q, "Alice")

	r := chi.NewRouter()
	income.New(repo.NewIncomeRepo(tdb.Pool)).Mount(r)

	return &handlerHarness{router: r, user: user}
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
		req = req.WithContext(identity.WithUser(req.Context(), *user))
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

// createIncome posts a minimal income row and returns the persisted entity.
// Regularity defaults to routine to match the dialog's create-side default
// (M6 grilling) so existing subtests stay focused on the field under test.
func (h *handlerHarness) createIncome(t *testing.T, category string) db.Income {
	t.Helper()
	body := map[string]any{
		"date":           "2026-05-15",
		"amount":         "15000000",
		"currency":       "IDR",
		"category":       category,
		"ownership_type": "joint",
		"regularity":     "routine",
	}
	rec := h.do(t, "POST", "/income", body)
	requireStatus(t, rec, http.StatusCreated)
	return decodeBody[db.Income](t, rec)
}

// ----- Tests --------------------------------------------------------------

func TestIncomeHandlers_Create(t *testing.T) {
	h := newHarness(t)

	t.Run("201 happy path joint", func(t *testing.T) {
		rec := h.do(t, "POST", "/income", map[string]any{
			"date":           "2026-05-15",
			"amount":         "15000000",
			"currency":       "IDR",
			"category":       "salary",
			"description":    "Base salary",
			"ownership_type": "joint",
			"regularity":     "routine",
		})
		requireStatus(t, rec, http.StatusCreated)
		body := decodeBody[db.Income](t, rec)
		if body.Category != "salary" {
			t.Errorf("category: got %q", body.Category)
		}
		if body.Regularity != "routine" {
			t.Errorf("regularity: want routine, got %q", body.Regularity)
		}
		if !decimal.NewFromInt(15000000).Equal(body.Amount) {
			t.Errorf("amount: got %s", body.Amount.String())
		}
	})

	t.Run("201 happy path sole with sole_owner_user_id", func(t *testing.T) {
		rec := h.do(t, "POST", "/income", map[string]any{
			"date":               "2026-05-20",
			"amount":             "2000000",
			"currency":           "IDR",
			"category":           "gift",
			"ownership_type":     "sole",
			"sole_owner_user_id": h.user.ID.String(),
			"regularity":         "incidental",
		})
		requireStatus(t, rec, http.StatusCreated)
		body := decodeBody[db.Income](t, rec)
		if body.Regularity != "incidental" {
			t.Errorf("regularity: want incidental, got %q", body.Regularity)
		}
	})

	t.Run("201 pension category accepted", func(t *testing.T) {
		// Pension income category (ADR-0048, #412 stats epic).
		rec := h.do(t, "POST", "/income", map[string]any{
			"date":           "2026-05-25",
			"amount":         "5000000",
			"currency":       "IDR",
			"category":       "pension",
			"ownership_type": "joint",
			"regularity":     "routine",
		})
		requireStatus(t, rec, http.StatusCreated)
		body := decodeBody[db.Income](t, rec)
		if body.Category != "pension" {
			t.Errorf("category: got %q, want pension", body.Category)
		}
	})

	t.Run("201 interest category accepted", func(t *testing.T) {
		// Interest income category (ADR-0048, #412 stats epic) — bank/deposit
		// interest, a passive *cash* stream.
		rec := h.do(t, "POST", "/income", map[string]any{
			"date":           "2026-05-25",
			"amount":         "125000",
			"currency":       "IDR",
			"category":       "interest",
			"ownership_type": "joint",
			"regularity":     "routine",
		})
		requireStatus(t, rec, http.StatusCreated)
		body := decodeBody[db.Income](t, rec)
		if body.Category != "interest" {
			t.Errorf("category: got %q, want interest", body.Category)
		}
	})

	t.Run("400 invalid json", func(t *testing.T) {
		rec := h.do(t, "POST", "/income", "{not-json")
		requireStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("400 missing required amount", func(t *testing.T) {
		rec := h.do(t, "POST", "/income", map[string]any{
			"date":           "2026-05-15",
			"currency":       "IDR",
			"category":       "salary",
			"ownership_type": "joint",
			"regularity":     "routine",
		})
		requireStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("400 missing required category", func(t *testing.T) {
		rec := h.do(t, "POST", "/income", map[string]any{
			"date":           "2026-05-15",
			"amount":         "1000",
			"currency":       "IDR",
			"ownership_type": "joint",
			"regularity":     "routine",
		})
		requireStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("400 invalid category enum", func(t *testing.T) {
		rec := h.do(t, "POST", "/income", map[string]any{
			"date":           "2026-05-15",
			"amount":         "1000",
			"currency":       "IDR",
			"category":       "bribe",
			"ownership_type": "joint",
			"regularity":     "routine",
		})
		requireStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("400 invalid ownership_type enum", func(t *testing.T) {
		rec := h.do(t, "POST", "/income", map[string]any{
			"date":           "2026-05-15",
			"amount":         "1000",
			"currency":       "IDR",
			"category":       "salary",
			"ownership_type": "communal",
			"regularity":     "routine",
		})
		requireStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("400 sole without sole_owner_user_id", func(t *testing.T) {
		rec := h.do(t, "POST", "/income", map[string]any{
			"date":           "2026-05-15",
			"amount":         "1000",
			"currency":       "IDR",
			"category":       "salary",
			"ownership_type": "sole",
			"regularity":     "routine",
		})
		requireStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("400 bad date format", func(t *testing.T) {
		rec := h.do(t, "POST", "/income", map[string]any{
			"date":           "15/05/2026",
			"amount":         "1000",
			"currency":       "IDR",
			"category":       "salary",
			"ownership_type": "joint",
			"regularity":     "routine",
		})
		requireStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("400 zero amount", func(t *testing.T) {
		rec := h.do(t, "POST", "/income", map[string]any{
			"date":           "2026-05-15",
			"amount":         "0",
			"currency":       "IDR",
			"category":       "salary",
			"ownership_type": "joint",
			"regularity":     "routine",
		})
		requireStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("400 negative amount", func(t *testing.T) {
		rec := h.do(t, "POST", "/income", map[string]any{
			"date":           "2026-05-15",
			"amount":         "-100",
			"currency":       "IDR",
			"category":       "salary",
			"ownership_type": "joint",
			"regularity":     "routine",
		})
		requireStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("400 missing required regularity", func(t *testing.T) {
		rec := h.do(t, "POST", "/income", map[string]any{
			"date":           "2026-05-15",
			"amount":         "1000",
			"currency":       "IDR",
			"category":       "salary",
			"ownership_type": "joint",
		})
		requireStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("400 invalid regularity enum", func(t *testing.T) {
		rec := h.do(t, "POST", "/income", map[string]any{
			"date":           "2026-05-15",
			"amount":         "1000",
			"currency":       "IDR",
			"category":       "salary",
			"ownership_type": "joint",
			"regularity":     "sporadic",
		})
		requireStatus(t, rec, http.StatusBadRequest)
	})
}

func TestIncomeHandlers_List(t *testing.T) {
	h := newHarness(t)
	created := h.createIncome(t, "salary")

	rec := h.do(t, "GET", "/income", nil)
	requireStatus(t, rec, http.StatusOK)
	list := decodeBody[[]db.Income](t, rec)
	if len(list) != 1 {
		t.Fatalf("list length: want 1, got %d", len(list))
	}
	if list[0].ID != created.ID {
		t.Errorf("list[0].id: want %s, got %s", created.ID, list[0].ID)
	}
}

func TestIncomeHandlers_Get(t *testing.T) {
	h := newHarness(t)
	created := h.createIncome(t, "salary")

	t.Run("200 happy path", func(t *testing.T) {
		rec := h.do(t, "GET", "/income/"+created.ID.String(), nil)
		requireStatus(t, rec, http.StatusOK)
		body := decodeBody[db.Income](t, rec)
		if body.ID != created.ID {
			t.Errorf("id: want %s, got %s", created.ID, body.ID)
		}
	})

	t.Run("404 unknown id", func(t *testing.T) {
		rec := h.do(t, "GET", "/income/"+uuid.NewString(), nil)
		requireStatus(t, rec, http.StatusNotFound)
	})

	t.Run("400 invalid id format", func(t *testing.T) {
		rec := h.do(t, "GET", "/income/not-a-uuid", nil)
		requireStatus(t, rec, http.StatusBadRequest)
	})
}

func TestIncomeHandlers_Update(t *testing.T) {
	h := newHarness(t)
	created := h.createIncome(t, "salary")

	t.Run("200 happy path; category + regularity mutated", func(t *testing.T) {
		rec := h.do(t, "PATCH", "/income/"+created.ID.String(), map[string]any{
			"date":           "2026-05-15",
			"amount":         "16000000",
			"currency":       "IDR",
			"category":       "business_income",
			"ownership_type": "joint",
			"regularity":     "incidental",
		})
		requireStatus(t, rec, http.StatusOK)
		body := decodeBody[db.Income](t, rec)
		if !decimal.NewFromInt(16000000).Equal(body.Amount) {
			t.Errorf("amount: got %s", body.Amount.String())
		}
		if body.Category != "business_income" {
			t.Errorf("category: want business_income, got %q", body.Category)
		}
		if body.Regularity != "incidental" {
			t.Errorf("regularity: want incidental, got %q", body.Regularity)
		}
	})

	t.Run("404 unknown id", func(t *testing.T) {
		rec := h.do(t, "PATCH", "/income/"+uuid.NewString(), map[string]any{
			"date":           "2026-05-15",
			"amount":         "1",
			"currency":       "IDR",
			"category":       "salary",
			"ownership_type": "joint",
			"regularity":     "routine",
		})
		requireStatus(t, rec, http.StatusNotFound)
	})

	t.Run("400 missing required amount", func(t *testing.T) {
		rec := h.do(t, "PATCH", "/income/"+created.ID.String(), map[string]any{
			"date":           "2026-05-15",
			"currency":       "IDR",
			"category":       "salary",
			"ownership_type": "joint",
			"regularity":     "routine",
		})
		requireStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("400 missing required regularity", func(t *testing.T) {
		rec := h.do(t, "PATCH", "/income/"+created.ID.String(), map[string]any{
			"date":           "2026-05-15",
			"amount":         "1",
			"currency":       "IDR",
			"category":       "salary",
			"ownership_type": "joint",
		})
		requireStatus(t, rec, http.StatusBadRequest)
	})
}

func TestIncomeHandlers_Delete(t *testing.T) {
	h := newHarness(t)

	t.Run("204 happy path", func(t *testing.T) {
		created := h.createIncome(t, "salary")
		rec := h.do(t, "DELETE", "/income/"+created.ID.String(), nil)
		requireStatus(t, rec, http.StatusNoContent)

		// confirm it's gone
		rec = h.do(t, "GET", "/income/"+created.ID.String(), nil)
		requireStatus(t, rec, http.StatusNotFound)
	})

	t.Run("404 unknown id", func(t *testing.T) {
		rec := h.do(t, "DELETE", "/income/"+uuid.NewString(), nil)
		requireStatus(t, rec, http.StatusNotFound)
	})
}

func TestIncomeHandlers_RequiresAuth(t *testing.T) {
	h := newHarness(t)
	rec := h.doRaw(t, "GET", "/income", nil, nil)
	requireStatus(t, rec, http.StatusUnauthorized)
}
