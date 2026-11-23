package investments_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/kerti/balances-v2/backend/internal/repo"
)

// The three repros of #574 over the wire, plus the window's stranding refusals
// reaching the client as the ADR-0027 envelope. fakeNow is 2030-01-01.

func bondBody(bondType, placement, maturity string) map[string]any {
	m := map[string]any{
		"display_name": "Window bond", "ownership_type": "joint", "native_currency": "IDR",
		"bond_type": bondType, "issuer": "Govt of Indonesia", "coupon_rate": "6.25",
		"coupon_frequency": "monthly", "maturity_date": maturity, "risk_profile": "medium",
	}
	if bondType == "govt_primary" {
		m["face_value"] = "10000000"
		m["placement_date"] = placement
	}
	return m
}

func errCode(t *testing.T, body []byte) string {
	t.Helper()
	var env struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatalf("decode envelope %q: %v", body, err)
	}
	return env.Code
}

// covers: INV-BONDS-05, INV-SNAPSHOTS-05
func TestBondHandlers_TermWindow_IssueRepros(t *testing.T) {
	h := newHarness(t)

	t.Run("future placement is a 400 and writes no bond", func(t *testing.T) {
		rec := h.do(t, "POST", "/investments/bonds", bondBody("govt_primary", "2099-06-01", "2100-06-01"))
		requireStatus(t, rec, http.StatusBadRequest)
		if got := errCode(t, rec.Body.Bytes()); got != "TRANSACTION_FUTURE_DATE" {
			t.Errorf("code: want TRANSACTION_FUTURE_DATE, got %q", got)
		}
		list := decodeBody[[]repo.BondListItem](t, h.do(t, "GET", "/investments/bonds", nil))
		if len(list) != 0 {
			t.Fatalf("a refused create must leave no bond, got %d", len(list))
		}
	})

	t.Run("today's placement is accepted (the boundary is exclusive of today)", func(t *testing.T) {
		rec := h.do(t, "POST", "/investments/bonds", bondBody("govt_primary", "2030-01-01", "2031-01-01"))
		requireStatus(t, rec, http.StatusCreated)
	})

	t.Run("maturity before placement is a 400", func(t *testing.T) {
		rec := h.do(t, "POST", "/investments/bonds", bondBody("govt_primary", "2026-01-01", "2025-01-01"))
		requireStatus(t, rec, http.StatusBadRequest)
		if got := errCode(t, rec.Body.Bytes()); got != "INVALID_BOND_TERM" {
			t.Errorf("code: want INVALID_BOND_TERM, got %q", got)
		}
	})

	t.Run("PATCH maturity before the only Buy is a 400 and persists nothing", func(t *testing.T) {
		b := decodeBody[*repo.Bond](t, h.do(t, "POST", "/investments/bonds", bondBody("govt_primary", "2026-01-15", "2027-01-15")))
		patch := bondBody("govt_primary", "", "2000-01-01")
		rec := h.do(t, "PATCH", "/investments/bonds/"+b.Investment.ID.String(), patch)
		requireStatus(t, rec, http.StatusBadRequest)
		if got := errCode(t, rec.Body.Bytes()); got != "INVALID_BOND_TERM" {
			t.Errorf("code: want INVALID_BOND_TERM, got %q", got)
		}
		got := decodeBody[*repo.Bond](t, h.do(t, "GET", "/investments/bonds/"+b.Investment.ID.String(), nil))
		if got.Details.MaturityDate.Format("2006-01-02") != "2027-01-15" {
			t.Errorf("maturity must be unchanged, got %s", got.Details.MaturityDate.Format("2006-01-02"))
		}
	})
}

// covers: INV-BONDS-05
func TestBondHandlers_TermWindow_Stranding(t *testing.T) {
	h := newHarness(t)
	b := decodeBody[*repo.Bond](t, h.do(t, "POST", "/investments/bonds", bondBody("govt_primary", "2026-01-15", "2027-01-15")))
	id := b.Investment.ID.String()

	t.Run("a snapshot before the earliest Buy is OUTSIDE_BOND_TERM", func(t *testing.T) {
		rec := h.do(t, "POST", "/investments/"+id+"/snapshots", map[string]any{
			"year_month": "2025-12", "amount": "10000000", "accrued_interest": "0", "currency": "IDR",
		})
		requireStatus(t, rec, http.StatusBadRequest)
		if got := errCode(t, rec.Body.Bytes()); got != "OUTSIDE_BOND_TERM" {
			t.Errorf("code: want OUTSIDE_BOND_TERM, got %q", got)
		}
	})

	t.Run("a coupon after maturity is OUTSIDE_BOND_TERM", func(t *testing.T) {
		rec := h.do(t, "POST", "/investments/"+id+"/transactions", map[string]any{
			"transaction_type": "coupon", "transaction_date": "2027-01-16", "currency": "IDR", "amount": "46875",
		})
		requireStatus(t, rec, http.StatusBadRequest)
		if got := errCode(t, rec.Body.Bytes()); got != "OUTSIDE_BOND_TERM" {
			t.Errorf("code: want OUTSIDE_BOND_TERM, got %q", got)
		}
	})

	rec := h.do(t, "POST", "/investments/"+id+"/snapshots", map[string]any{
		"year_month": "2026-02", "amount": "10000000", "accrued_interest": "0", "currency": "IDR",
	})
	requireStatus(t, rec, http.StatusCreated)
	txns := decodeBody[[]map[string]any](t, h.do(t, "GET", "/investments/"+id+"/transactions", nil))
	if len(txns) != 1 {
		t.Fatalf("want the one seeded Buy, got %d", len(txns))
	}
	buyID := txns[0]["id"].(string)

	t.Run("moving the earliest Buy past a snapshot is refused", func(t *testing.T) {
		rec := h.do(t, "PATCH", "/investments/"+id+"/transactions/"+buyID, map[string]any{
			"transaction_date": "2026-03-01", "currency": "IDR",
			"amount": "10000000", "quantity": "10", "price_per_unit": "1000000",
		})
		requireStatus(t, rec, http.StatusBadRequest)
		if got := errCode(t, rec.Body.Bytes()); got != "OUTSIDE_BOND_TERM" {
			t.Errorf("code: want OUTSIDE_BOND_TERM, got %q", got)
		}
	})

	t.Run("a maturity edit that strands the snapshot is refused", func(t *testing.T) {
		rec := h.do(t, "PATCH", "/investments/bonds/"+id, bondBody("govt_primary", "", "2026-01-31"))
		requireStatus(t, rec, http.StatusBadRequest)
		if got := errCode(t, rec.Body.Bytes()); got != "OUTSIDE_BOND_TERM" {
			t.Errorf("code: want OUTSIDE_BOND_TERM, got %q", got)
		}
	})

	t.Run("a future-dated Buy edit is still refused by the date guard first", func(t *testing.T) {
		rec := h.do(t, "PATCH", "/investments/"+id+"/transactions/"+buyID, map[string]any{
			"transaction_date": "2031-03-01", "currency": "IDR",
			"amount": "10000000", "quantity": "10", "price_per_unit": "1000000",
		})
		requireStatus(t, rec, http.StatusBadRequest)
		if got := errCode(t, rec.Body.Bytes()); got != "TRANSACTION_FUTURE_DATE" {
			t.Errorf("code: want TRANSACTION_FUTURE_DATE, got %q", got)
		}
	})

	t.Run("deleting the earliest Buy while a later Buy leaves the snapshot early is refused", func(t *testing.T) {
		rec := h.do(t, "POST", "/investments/"+id+"/transactions", map[string]any{
			"transaction_type": "buy", "transaction_date": "2026-06-01", "currency": "IDR",
			"amount": "1000000", "quantity": "1", "price_per_unit": "1000000",
		})
		requireStatus(t, rec, http.StatusCreated)
		del := h.do(t, "DELETE", "/investments/"+id+"/transactions/"+buyID, nil)
		requireStatus(t, del, http.StatusBadRequest)
		if got := errCode(t, del.Body.Bytes()); got != "OUTSIDE_BOND_TERM" {
			t.Errorf("code: want OUTSIDE_BOND_TERM, got %q", got)
		}
	})
}
