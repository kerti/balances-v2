package investments_test

import (
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/kerti/balances-v2/backend/internal/repo"
)

func (h *handlerHarness) createTimeDeposit(t *testing.T, displayName string) *repo.TimeDeposit {
	t.Helper()
	rec := h.do(t, "POST", "/investments/time-deposits", map[string]any{
		"display_name":    displayName,
		"ownership_type":  "joint",
		"native_currency": "IDR",
		"bank_name":       "BCA",
		"principal":       "100000000",
		"interest_rate":   "4.5",
		"term_months":     12,
		"placement_date":  "2026-01-01",
		"maturity_date":   "2027-01-01",
		"rollover_policy": "auto_renew_with_interest",
		"risk_profile":    "medium",
	})
	requireStatus(t, rec, http.StatusCreated)
	return decodeBody[*repo.TimeDeposit](t, rec)
}

func TestTimeDepositHandlers_Create(t *testing.T) {
	h := newHarness(t)

	t.Run("201 happy path", func(t *testing.T) {
		rec := h.do(t, "POST", "/investments/time-deposits", map[string]any{
			"display_name":    "BCA 12-month",
			"ownership_type":  "joint",
			"native_currency": "IDR",
			"bank_name":       "BCA",
			"principal":       "100000000",
			"interest_rate":   "4.5",
			"term_months":     12,
			"placement_date":  "2026-01-01",
			"maturity_date":   "2027-01-01",
			"rollover_policy": "no_rollover",
			"risk_profile":    "medium",
		})
		requireStatus(t, rec, http.StatusCreated)
		body := decodeBody[*repo.TimeDeposit](t, rec)
		if body.Details.BankName != "BCA" {
			t.Errorf("bank_name: got %q", body.Details.BankName)
		}
	})

	t.Run("400 invalid rollover_policy enum", func(t *testing.T) {
		rec := h.do(t, "POST", "/investments/time-deposits", map[string]any{
			"display_name":    "X",
			"ownership_type":  "joint",
			"native_currency": "IDR",
			"bank_name":       "Y",
			"principal":       "1000",
			"interest_rate":   "1",
			"term_months":     6,
			"placement_date":  "2026-01-01",
			"maturity_date":   "2026-07-01",
			"rollover_policy": "manual",
			"risk_profile":    "medium",
		})
		requireStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("400 term_months not positive", func(t *testing.T) {
		rec := h.do(t, "POST", "/investments/time-deposits", map[string]any{
			"display_name":    "X",
			"ownership_type":  "joint",
			"native_currency": "IDR",
			"bank_name":       "Y",
			"principal":       "1000",
			"interest_rate":   "1",
			"term_months":     0,
			"placement_date":  "2026-01-01",
			"maturity_date":   "2026-07-01",
			"rollover_policy": "no_rollover",
			"risk_profile":    "medium",
		})
		requireStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("400 bad placement_date format", func(t *testing.T) {
		rec := h.do(t, "POST", "/investments/time-deposits", map[string]any{
			"display_name":    "X",
			"ownership_type":  "joint",
			"native_currency": "IDR",
			"bank_name":       "Y",
			"principal":       "1000",
			"interest_rate":   "1",
			"term_months":     6,
			"placement_date":  "2026/01/01",
			"maturity_date":   "2026-07-01",
			"rollover_policy": "no_rollover",
			"risk_profile":    "medium",
		})
		requireStatus(t, rec, http.StatusBadRequest)
	})
}

func TestTimeDepositHandlers_List(t *testing.T) {
	h := newHarness(t)
	h.createTimeDeposit(t, "Listed TD")

	rec := h.do(t, "GET", "/investments/time-deposits", nil)
	requireStatus(t, rec, http.StatusOK)
	list := decodeBody[[]repo.TimeDepositListItem](t, rec)
	if len(list) != 1 {
		t.Fatalf("list length: want 1, got %d", len(list))
	}
	// TD cost basis is the principal — the ledger holds only Maturity.
	requireCostBasis(t, list[0].CostBasis, "100000000")
}

func TestTimeDepositHandlers_Get(t *testing.T) {
	h := newHarness(t)
	created := h.createTimeDeposit(t, "Get target")

	t.Run("200 happy path", func(t *testing.T) {
		rec := h.do(t, "GET", "/investments/time-deposits/"+created.Investment.ID.String(), nil)
		requireStatus(t, rec, http.StatusOK)
	})

	t.Run("404 unknown id", func(t *testing.T) {
		rec := h.do(t, "GET", "/investments/time-deposits/"+uuid.NewString(), nil)
		requireStatus(t, rec, http.StatusNotFound)
	})

	t.Run("400 invalid id format", func(t *testing.T) {
		rec := h.do(t, "GET", "/investments/time-deposits/not-a-uuid", nil)
		requireStatus(t, rec, http.StatusBadRequest)
	})
}

func TestTimeDepositHandlers_Update(t *testing.T) {
	h := newHarness(t)
	created := h.createTimeDeposit(t, "Update target")

	t.Run("200 happy path", func(t *testing.T) {
		rec := h.do(t, "PATCH", "/investments/time-deposits/"+created.Investment.ID.String(), map[string]any{
			"display_name":    "Renamed",
			"ownership_type":  "joint",
			"bank_name":       "Mandiri",
			"principal":       "100000000",
			"interest_rate":   "5.0",
			"term_months":     12,
			"placement_date":  "2026-01-01",
			"maturity_date":   "2027-01-01",
			"rollover_policy": "no_rollover",
			"risk_profile":    "medium",
		})
		requireStatus(t, rec, http.StatusOK)
		body := decodeBody[*repo.TimeDeposit](t, rec)
		if body.Details.BankName != "Mandiri" {
			t.Errorf("bank_name: want Mandiri, got %q", body.Details.BankName)
		}
	})

	t.Run("404 unknown id", func(t *testing.T) {
		rec := h.do(t, "PATCH", "/investments/time-deposits/"+uuid.NewString(), map[string]any{
			"display_name":    "x",
			"ownership_type":  "joint",
			"bank_name":       "y",
			"principal":       "1",
			"interest_rate":   "1",
			"term_months":     6,
			"placement_date":  "2026-01-01",
			"maturity_date":   "2026-07-01",
			"rollover_policy": "no_rollover",
			"risk_profile":    "medium",
		})
		requireStatus(t, rec, http.StatusNotFound)
	})

	t.Run("400 bad maturity_date format", func(t *testing.T) {
		rec := h.do(t, "PATCH", "/investments/time-deposits/"+created.Investment.ID.String(), map[string]any{
			"display_name":    "x",
			"bank_name":       "y",
			"principal":       "1",
			"interest_rate":   "1",
			"term_months":     6,
			"placement_date":  "2026-01-01",
			"maturity_date":   "07-2026-01",
			"rollover_policy": "no_rollover",
			"risk_profile":    "medium",
		})
		requireStatus(t, rec, http.StatusBadRequest)
	})
}

// covers: INV-LIFECYCLE-07
func TestTimeDepositHandlers_LinkRolloverSuccessor(t *testing.T) {
	h := newHarness(t)
	source := h.createTimeDeposit(t, "Matured source")
	successor := h.createTimeDeposit(t, "Hand-made successor")

	t.Run("200 links and source resolves rolled_to", func(t *testing.T) {
		rec := h.do(t, "POST", "/investments/time-deposits/"+source.Investment.ID.String()+"/rollover-successor", map[string]any{
			"successor_id": successor.Investment.ID.String(),
		})
		requireStatus(t, rec, http.StatusOK)
		body := decodeBody[*repo.TimeDeposit](t, rec)
		if body.RolledTo == nil || body.RolledTo.ID != successor.Investment.ID {
			t.Errorf("rolled_to: want %v, got %+v", successor.Investment.ID, body.RolledTo)
		}
	})

	t.Run("409 self-link", func(t *testing.T) {
		other := h.createTimeDeposit(t, "Self link")
		rec := h.do(t, "POST", "/investments/time-deposits/"+other.Investment.ID.String()+"/rollover-successor", map[string]any{
			"successor_id": other.Investment.ID.String(),
		})
		requireStatus(t, rec, http.StatusConflict)
	})

	t.Run("404 unknown source", func(t *testing.T) {
		fresh := h.createTimeDeposit(t, "Fresh succ")
		rec := h.do(t, "POST", "/investments/time-deposits/"+uuid.NewString()+"/rollover-successor", map[string]any{
			"successor_id": fresh.Investment.ID.String(),
		})
		requireStatus(t, rec, http.StatusNotFound)
	})

	t.Run("400 invalid source id format", func(t *testing.T) {
		rec := h.do(t, "POST", "/investments/time-deposits/not-a-uuid/rollover-successor", map[string]any{
			"successor_id": successor.Investment.ID.String(),
		})
		requireStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("400 missing successor_id", func(t *testing.T) {
		rec := h.do(t, "POST", "/investments/time-deposits/"+source.Investment.ID.String()+"/rollover-successor", map[string]any{})
		requireStatus(t, rec, http.StatusBadRequest)
	})
}

func TestTimeDepositHandlers_Delete(t *testing.T) {
	h := newHarness(t)

	t.Run("204 happy path", func(t *testing.T) {
		created := h.createTimeDeposit(t, "To delete")
		rec := h.do(t, "DELETE", "/investments/time-deposits/"+created.Investment.ID.String(), nil)
		requireStatus(t, rec, http.StatusNoContent)
	})

	t.Run("404 unknown id", func(t *testing.T) {
		rec := h.do(t, "DELETE", "/investments/time-deposits/"+uuid.NewString(), nil)
		requireStatus(t, rec, http.StatusNotFound)
	})
}
