package investments_test

import (
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/kerti/balances-v2/backend/internal/repo"
)

func (h *handlerHarness) createBond(t *testing.T, displayName string) *repo.Bond {
	t.Helper()
	rec := h.do(t, "POST", "/investments/bonds", map[string]any{
		"display_name":     displayName,
		"ownership_type":   "joint",
		"native_currency":  "IDR",
		"bond_type":        "govt_primary",
		"issuer":           "Govt of Indonesia",
		"face_value":       "10000000",
		"placement_date":   "2025-01-15",
		"coupon_rate":      "6.25",
		"coupon_frequency": "monthly",
		"maturity_date":    "2030-01-01",
		"risk_profile":     "medium",
	})
	requireStatus(t, rec, http.StatusCreated)
	return decodeBody[*repo.Bond](t, rec)
}

// covers: INV-BONDS-01
func TestBondHandlers_Create(t *testing.T) {
	h := newHarness(t)

	t.Run("201 happy path", func(t *testing.T) {
		rec := h.do(t, "POST", "/investments/bonds", map[string]any{
			"display_name":     "SBR012",
			"ownership_type":   "joint",
			"native_currency":  "IDR",
			"bond_type":        "govt_primary",
			"series_code":      "SBR012",
			"issuer":           "Govt of Indonesia",
			"face_value":       "5000000",
			"placement_date":   "2024-07-10",
			"coupon_rate":      "5.95",
			"coupon_frequency": "monthly",
			"maturity_date":    "2027-07-10",
			"risk_profile":     "medium",
		})
		requireStatus(t, rec, http.StatusCreated)
		body := decodeBody[*repo.Bond](t, rec)
		if body.Details.Issuer != "Govt of Indonesia" {
			t.Errorf("issuer: got %q", body.Details.Issuer)
		}
		// govt_primary seeds a placement Buy at par (issue #27): outstanding
		// nominal = face, derived from the ledger.
		requireCostBasis(t, body.OutstandingFace, "5000000")
	})

	t.Run("400 govt_primary missing placement_date", func(t *testing.T) {
		rec := h.do(t, "POST", "/investments/bonds", map[string]any{
			"display_name":     "X",
			"ownership_type":   "joint",
			"native_currency":  "IDR",
			"bond_type":        "govt_primary",
			"issuer":           "Y",
			"face_value":       "1000",
			"coupon_rate":      "5",
			"coupon_frequency": "annual",
			"maturity_date":    "2027-01-01",
			"risk_profile":     "medium",
		})
		requireStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("400 invalid coupon_frequency enum", func(t *testing.T) {
		rec := h.do(t, "POST", "/investments/bonds", map[string]any{
			"display_name":     "X",
			"ownership_type":   "joint",
			"native_currency":  "IDR",
			"bond_type":        "govt_primary",
			"issuer":           "Y",
			"face_value":       "1000",
			"placement_date":   "2025-01-01",
			"coupon_rate":      "5",
			"coupon_frequency": "fortnightly",
			"maturity_date":    "2027-01-01",
			"risk_profile":     "medium",
		})
		requireStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("400 bad maturity_date format", func(t *testing.T) {
		rec := h.do(t, "POST", "/investments/bonds", map[string]any{
			"display_name":     "X",
			"ownership_type":   "joint",
			"native_currency":  "IDR",
			"bond_type":        "govt_primary",
			"issuer":           "Y",
			"face_value":       "1000",
			"placement_date":   "2025-01-01",
			"coupon_rate":      "5",
			"coupon_frequency": "annual",
			"maturity_date":    "01-07-2027",
			"risk_profile":     "medium",
		})
		requireStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("400 missing required face_value", func(t *testing.T) {
		rec := h.do(t, "POST", "/investments/bonds", map[string]any{
			"display_name":     "X",
			"ownership_type":   "joint",
			"native_currency":  "IDR",
			"bond_type":        "govt_primary",
			"issuer":           "Y",
			"coupon_rate":      "5",
			"coupon_frequency": "annual",
			"maturity_date":    "2027-01-01",
			"risk_profile":     "medium",
		})
		requireStatus(t, rec, http.StatusBadRequest)
	})
}

// covers: INV-BONDS-01
func TestBondHandlers_List(t *testing.T) {
	h := newHarness(t)
	h.createBond(t, "Listed bond")

	rec := h.do(t, "GET", "/investments/bonds", nil)
	requireStatus(t, rec, http.StatusOK)
	list := decodeBody[[]repo.BondListItem](t, rec)
	if len(list) != 1 {
		t.Fatalf("list length: want 1, got %d", len(list))
	}
	// govt_primary now seeds a placement Buy (issue #27): cost basis is the
	// ledger replay (Σ amount), which at par equals the placed nominal.
	requireCostBasis(t, list[0].CostBasis, "10000000")
}

func TestBondHandlers_Get(t *testing.T) {
	h := newHarness(t)
	created := h.createBond(t, "Get target")

	t.Run("200 happy path", func(t *testing.T) {
		rec := h.do(t, "GET", "/investments/bonds/"+created.Investment.ID.String(), nil)
		requireStatus(t, rec, http.StatusOK)
	})

	t.Run("404 unknown id", func(t *testing.T) {
		rec := h.do(t, "GET", "/investments/bonds/"+uuid.NewString(), nil)
		requireStatus(t, rec, http.StatusNotFound)
	})

	t.Run("400 invalid id format", func(t *testing.T) {
		rec := h.do(t, "GET", "/investments/bonds/not-a-uuid", nil)
		requireStatus(t, rec, http.StatusBadRequest)
	})
}

func TestBondHandlers_Update(t *testing.T) {
	h := newHarness(t)
	created := h.createBond(t, "Update target")

	t.Run("200 happy path", func(t *testing.T) {
		rec := h.do(t, "PATCH", "/investments/bonds/"+created.Investment.ID.String(), map[string]any{
			"display_name":     "Renamed",
			"ownership_type":   "joint",
			"bond_type":        "secondary_market",
			"issuer":           "Govt of Indonesia",
			"face_value":       "10000000",
			"coupon_rate":      "6.50",
			"coupon_frequency": "semi_annual",
			"maturity_date":    "2030-01-01",
			"risk_profile":     "medium",
		})
		requireStatus(t, rec, http.StatusOK)
		body := decodeBody[*repo.Bond](t, rec)
		if body.Details.BondType != "secondary_market" {
			t.Errorf("bond_type: want secondary_market, got %q", body.Details.BondType)
		}
	})

	t.Run("404 unknown id", func(t *testing.T) {
		rec := h.do(t, "PATCH", "/investments/bonds/"+uuid.NewString(), map[string]any{
			"display_name":     "x",
			"ownership_type":   "joint",
			"bond_type":        "govt_primary",
			"issuer":           "y",
			"face_value":       "1",
			"coupon_rate":      "1",
			"coupon_frequency": "annual",
			"maturity_date":    "2027-01-01",
			"risk_profile":     "medium",
		})
		requireStatus(t, rec, http.StatusNotFound)
	})

	t.Run("400 bad maturity_date format", func(t *testing.T) {
		rec := h.do(t, "PATCH", "/investments/bonds/"+created.Investment.ID.String(), map[string]any{
			"display_name":     "x",
			"bond_type":        "govt_primary",
			"issuer":           "y",
			"face_value":       "1",
			"coupon_rate":      "1",
			"coupon_frequency": "annual",
			"maturity_date":    "2030/01/01",
			"risk_profile":     "medium",
		})
		requireStatus(t, rec, http.StatusBadRequest)
	})
}

func TestBondHandlers_Delete(t *testing.T) {
	h := newHarness(t)

	t.Run("204 happy path", func(t *testing.T) {
		created := h.createBond(t, "To delete")
		rec := h.do(t, "DELETE", "/investments/bonds/"+created.Investment.ID.String(), nil)
		requireStatus(t, rec, http.StatusNoContent)
	})

	t.Run("404 unknown id", func(t *testing.T) {
		rec := h.do(t, "DELETE", "/investments/bonds/"+uuid.NewString(), nil)
		requireStatus(t, rec, http.StatusNotFound)
	})
}
