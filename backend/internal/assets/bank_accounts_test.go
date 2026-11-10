package assets_test

import (
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/kerti/balances-v2/backend/internal/repo"
)

func (h *handlerHarness) createBankAccount(t *testing.T, displayName string) *repo.BankAccount {
	t.Helper()
	rec := h.do(t, "POST", "/bank-accounts", map[string]any{
		"display_name":    displayName,
		"ownership_type":  "joint",
		"native_currency": "IDR",
		"bank_name":       "TestBank",
		"account_number":  "1234567890",
		"account_type":    "savings",
	})
	requireStatus(t, rec, http.StatusCreated)
	return decodeBody[*repo.BankAccount](t, rec)
}

func TestBankAccountHandlers_Create(t *testing.T) {
	h := newHarness(t)

	t.Run("201 happy path", func(t *testing.T) {
		rec := h.do(t, "POST", "/bank-accounts", map[string]any{
			"display_name":    "Main checking",
			"ownership_type":  "joint",
			"native_currency": "IDR",
			"bank_name":       "BCA",
			"account_number":  "987654321",
			"account_type":    "current",
		})
		requireStatus(t, rec, http.StatusCreated)
		body := decodeBody[*repo.BankAccount](t, rec)
		if body.Asset.DisplayName != "Main checking" {
			t.Errorf("display_name: got %q", body.Asset.DisplayName)
		}
		if body.Details.BankName != "BCA" {
			t.Errorf("bank_name: got %q", body.Details.BankName)
		}
	})

	t.Run("400 invalid json", func(t *testing.T) {
		rec := h.do(t, "POST", "/bank-accounts", "{not-json")
		requireStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("201 blank account_number is accepted and stored as empty", func(t *testing.T) {
		rec := h.do(t, "POST", "/bank-accounts", map[string]any{
			"display_name":    "No number",
			"ownership_type":  "joint",
			"native_currency": "IDR",
			"bank_name":       "BCA",
			"account_number":  "",
			"account_type":    "savings",
		})
		requireStatus(t, rec, http.StatusCreated)
		body := decodeBody[*repo.BankAccount](t, rec)
		if body.Details.AccountNumber != "" {
			t.Errorf("account_number: want empty, got %q", body.Details.AccountNumber)
		}
	})

	t.Run("400 missing required bank_name", func(t *testing.T) {
		rec := h.do(t, "POST", "/bank-accounts", map[string]any{
			"display_name":    "X",
			"ownership_type":  "joint",
			"native_currency": "IDR",
			"account_number":  "1",
			"account_type":    "savings",
		})
		requireStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("400 invalid account_type enum", func(t *testing.T) {
		rec := h.do(t, "POST", "/bank-accounts", map[string]any{
			"display_name":    "X",
			"ownership_type":  "joint",
			"native_currency": "IDR",
			"bank_name":       "Y",
			"account_number":  "1",
			"account_type":    "crypto",
		})
		requireStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("400 invalid native_currency (not iso4217)", func(t *testing.T) {
		rec := h.do(t, "POST", "/bank-accounts", map[string]any{
			"display_name":    "X",
			"ownership_type":  "joint",
			"native_currency": "ZZZ",
			"bank_name":       "Y",
			"account_number":  "1",
			"account_type":    "savings",
		})
		requireStatus(t, rec, http.StatusBadRequest)
	})
}

func TestBankAccountHandlers_List(t *testing.T) {
	h := newHarness(t)
	created := h.createBankAccount(t, "Listed")

	rec := h.do(t, "GET", "/bank-accounts", nil)
	requireStatus(t, rec, http.StatusOK)
	list := decodeBody[[]repo.BankAccountListItem](t, rec)
	if len(list) != 1 {
		t.Fatalf("list length: want 1, got %d", len(list))
	}
	if list[0].Asset.ID != created.Asset.ID {
		t.Errorf("list[0] id: want %s, got %s", created.Asset.ID, list[0].Asset.ID)
	}
}

func TestBankAccountHandlers_Get(t *testing.T) {
	h := newHarness(t)
	created := h.createBankAccount(t, "Get target")

	t.Run("200 happy path", func(t *testing.T) {
		rec := h.do(t, "GET", "/bank-accounts/"+created.Asset.ID.String(), nil)
		requireStatus(t, rec, http.StatusOK)
		body := decodeBody[*repo.BankAccount](t, rec)
		if body.Asset.ID != created.Asset.ID {
			t.Errorf("id: want %s, got %s", created.Asset.ID, body.Asset.ID)
		}
	})

	t.Run("404 unknown id", func(t *testing.T) {
		rec := h.do(t, "GET", "/bank-accounts/"+uuid.NewString(), nil)
		requireStatus(t, rec, http.StatusNotFound)
	})

	t.Run("400 invalid id format", func(t *testing.T) {
		rec := h.do(t, "GET", "/bank-accounts/not-a-uuid", nil)
		requireStatus(t, rec, http.StatusBadRequest)
	})
}

func TestBankAccountHandlers_Update(t *testing.T) {
	h := newHarness(t)
	created := h.createBankAccount(t, "Update target")

	t.Run("200 happy path", func(t *testing.T) {
		rec := h.do(t, "PATCH", "/bank-accounts/"+created.Asset.ID.String(), map[string]any{
			"display_name":   "Renamed",
			"ownership_type": "joint",
			"bank_name":      "Mandiri",
			"account_number": "555555",
			"account_type":   "current",
		})
		requireStatus(t, rec, http.StatusOK)
		body := decodeBody[*repo.BankAccount](t, rec)
		if body.Asset.DisplayName != "Renamed" {
			t.Errorf("display_name: want Renamed, got %q", body.Asset.DisplayName)
		}
		if body.Details.BankName != "Mandiri" {
			t.Errorf("bank_name: want Mandiri, got %q", body.Details.BankName)
		}
	})

	t.Run("200 blank account_number clears the number", func(t *testing.T) {
		rec := h.do(t, "PATCH", "/bank-accounts/"+created.Asset.ID.String(), map[string]any{
			"display_name":   "Renamed",
			"ownership_type": "joint",
			"bank_name":      "Mandiri",
			"account_number": "",
			"account_type":   "current",
		})
		requireStatus(t, rec, http.StatusOK)
		body := decodeBody[*repo.BankAccount](t, rec)
		if body.Details.AccountNumber != "" {
			t.Errorf("account_number: want empty, got %q", body.Details.AccountNumber)
		}
	})

	t.Run("404 unknown id", func(t *testing.T) {
		rec := h.do(t, "PATCH", "/bank-accounts/"+uuid.NewString(), map[string]any{
			"display_name":   "x",
			"ownership_type": "joint",
			"bank_name":      "y",
			"account_number": "1",
			"account_type":   "savings",
		})
		requireStatus(t, rec, http.StatusNotFound)
	})

	t.Run("400 missing required bank_name", func(t *testing.T) {
		rec := h.do(t, "PATCH", "/bank-accounts/"+created.Asset.ID.String(), map[string]any{
			"display_name":   "x",
			"account_number": "1",
			"account_type":   "savings",
		})
		requireStatus(t, rec, http.StatusBadRequest)
	})
}

func TestBankAccountHandlers_Delete(t *testing.T) {
	h := newHarness(t)

	t.Run("204 happy path", func(t *testing.T) {
		created := h.createBankAccount(t, "To delete")
		rec := h.do(t, "DELETE", "/bank-accounts/"+created.Asset.ID.String(), nil)
		requireStatus(t, rec, http.StatusNoContent)

		rec = h.do(t, "GET", "/bank-accounts/"+created.Asset.ID.String(), nil)
		requireStatus(t, rec, http.StatusNotFound)
	})

	t.Run("404 unknown id", func(t *testing.T) {
		rec := h.do(t, "DELETE", "/bank-accounts/"+uuid.NewString(), nil)
		requireStatus(t, rec, http.StatusNotFound)
	})
}
