package investments_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/kerti/balances-v2/backend/internal/db"
	"github.com/kerti/balances-v2/backend/internal/repo"
	"github.com/kerti/balances-v2/backend/internal/testutil"
)

// createBondWithDisposition creates a secondary-market bond with an explicit
// coupon_disposition so the entry-list default (accrues → forced entry,
// pays_out → 0) can be exercised. The govt_primary createBond helper always
// defaults to pays_out.
func (h *handlerHarness) createBondWithDisposition(t *testing.T, displayName, disposition string) *repo.Bond {
	t.Helper()
	rec := h.do(t, "POST", "/investments/bonds", map[string]any{
		"display_name":       displayName,
		"ownership_type":     "joint",
		"native_currency":    "IDR",
		"bond_type":          "secondary_market",
		"issuer":             "Govt of Indonesia",
		"coupon_rate":        "6.25",
		"coupon_frequency":   "semi_annual",
		"coupon_disposition": disposition,
		"maturity_date":      "2030-01-01",
		"risk_profile":       "medium",
	})
	requireStatus(t, rec, http.StatusCreated)
	return decodeBody[*repo.Bond](t, rec)
}

func (h *handlerHarness) bulkSaveAccrued(t *testing.T, yearMonth string, rows []map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	return h.do(t, "POST", "/investments/snapshots/accrued/bulk", map[string]any{
		"year_month": yearMonth,
		"as_of_date": yearMonth + "-28",
		"rows":       rows,
	})
}

// covers: INV-SNAPSHOTS-06
// covers: INV-SNAPSHOTS-09
func TestInvestmentSnapshotHandlers_BulkAccrued(t *testing.T) {
	t.Run("T1 one dirty row writes one accrued snapshot (amount + accrued_interest, no qty/price)", func(t *testing.T) {
		h := newHarness(t)
		bond := h.createBond(t, "Bulk accrued tracer")

		rec := h.bulkSaveAccrued(t, "2026-05", []map[string]any{
			{"investment_id": bond.Investment.ID.String(), "amount": "50250000", "accrued_interest": "250000", "currency": "IDR"},
		})
		requireStatus(t, rec, http.StatusOK)

		if got := h.rawInvSnapshotCount(t, bond.Investment.ID); got != 1 {
			t.Fatalf("want 1 snapshot written, got %d", got)
		}
		amount, qty, price, accrued := h.rawInvSnapshotShape(t, bond.Investment.ID, "2026-05")
		if !decimal.NewFromInt(50250000).Equal(amount) {
			t.Errorf("amount should be the entered total value 50250000, got %s", amount)
		}
		if accrued == nil || !decimal.NewFromInt(250000).Equal(*accrued) {
			t.Errorf("accrued_interest should be 250000, got %v", accrued)
		}
		if qty != nil || price != nil {
			t.Errorf("quantity/price must be null on the accrued branch, got %v / %v", qty, price)
		}
	})

	t.Run("T2 re-entering a month upserts, no duplicate", func(t *testing.T) {
		h := newHarness(t)
		td := h.createTimeDeposit(t, "Bulk reentry TD")

		requireStatus(t, h.bulkSaveAccrued(t, "2026-05", []map[string]any{
			{"investment_id": td.Investment.ID.String(), "amount": "100000000", "accrued_interest": "0", "currency": "IDR"},
		}), http.StatusOK)
		requireStatus(t, h.bulkSaveAccrued(t, "2026-05", []map[string]any{
			{"investment_id": td.Investment.ID.String(), "amount": "100375000", "accrued_interest": "375000", "currency": "IDR"},
		}), http.StatusOK)

		if got := h.rawInvSnapshotCount(t, td.Investment.ID); got != 1 {
			t.Fatalf("want 1 snapshot after re-entry, got %d", got)
		}
		amount, _, _, accrued := h.rawInvSnapshotShape(t, td.Investment.ID, "2026-05")
		if !decimal.NewFromInt(100375000).Equal(amount) {
			t.Errorf("want upserted amount 100375000, got %s", amount)
		}
		if accrued == nil || !decimal.NewFromInt(375000).Equal(*accrued) {
			t.Errorf("want upserted accrued 375000, got %v", accrued)
		}
	})

	t.Run("T3 one ineligible row aborts the whole batch with a per-row error", func(t *testing.T) {
		h := newHarness(t)
		good := h.createBond(t, "Good row")
		bad := uuid.New() // never created — not owned by this household

		rec := h.bulkSaveAccrued(t, "2026-05", []map[string]any{
			{"investment_id": good.Investment.ID.String(), "amount": "1000", "accrued_interest": "0", "currency": "IDR"},
			{"investment_id": bad.String(), "amount": "1000", "accrued_interest": "0", "currency": "IDR"},
		})
		requireStatus(t, rec, http.StatusUnprocessableEntity)

		if got := h.rawInvSnapshotCount(t, good.Investment.ID); got != 0 {
			t.Errorf("a batch with any bad row must write nothing; wrote %d", got)
		}
		body := decodeBody[invBulkErrBody](t, rec)
		if len(body.Errors) != 1 || body.Errors[0].InvestmentID != bad.String() {
			t.Fatalf("want the bad investment %s flagged, got %+v", bad, body.Errors)
		}
	})

	t.Run("T4 a stock (qty×price shape) can never be written through the accrued path", func(t *testing.T) {
		h := newHarness(t)
		stock := h.createStock(t, "Qty×price-shape stock")

		rec := h.bulkSaveAccrued(t, "2026-05", []map[string]any{
			{"investment_id": stock.Investment.ID.String(), "amount": "1000", "accrued_interest": "0", "currency": "IDR"},
		})
		requireStatus(t, rec, http.StatusUnprocessableEntity)
		if got := h.rawInvSnapshotCount(t, stock.Investment.ID); got != 0 {
			t.Errorf("stock must not receive an accrued snapshot; wrote %d", got)
		}
		body := decodeBody[invBulkErrBody](t, rec)
		if len(body.Errors) != 1 || body.Errors[0].InvestmentID != stock.Investment.ID.String() {
			t.Fatalf("want the stock flagged ineligible for accrued, got %+v", body.Errors)
		}
	})

	t.Run("T5 a time deposit is ineligible outside its term window", func(t *testing.T) {
		h := newHarness(t)
		// The helper's TD runs 2026-01-01..2027-01-01; a target month before
		// placement is outside the term.
		td := h.createTimeDeposit(t, "Placed in Jan 2026")

		rec := h.bulkSaveAccrued(t, "2025-12", []map[string]any{
			{"investment_id": td.Investment.ID.String(), "amount": "100000000", "accrued_interest": "0", "currency": "IDR"},
		})
		requireStatus(t, rec, http.StatusUnprocessableEntity)
		if got := h.rawInvSnapshotCount(t, td.Investment.ID); got != 0 {
			t.Errorf("out-of-term TD must write nothing; wrote %d", got)
		}
		body := decodeBody[invBulkErrBody](t, rec)
		if len(body.Errors) != 1 || body.Errors[0].InvestmentID != td.Investment.ID.String() {
			t.Fatalf("want the out-of-term TD flagged ineligible, got %+v", body.Errors)
		}
	})

	t.Run("T6 another household's bond is ineligible (tenancy)", func(t *testing.T) {
		h := newHarness(t)
		mine := h.createBond(t, "Mine")

		bob := testutil.CreateHouseholdWithUser(t, db.New(h.pool), "Bob")
		rec := h.doRaw(t, "POST", "/investments/bonds", map[string]any{
			"display_name": "Bob's bond", "ownership_type": "joint", "native_currency": "IDR",
			"bond_type": "govt_primary", "issuer": "Govt of Indonesia", "face_value": "10000000",
			"placement_date": "2025-01-15", "coupon_rate": "6.25", "coupon_frequency": "monthly",
			"maturity_date": "2030-01-01", "risk_profile": "medium",
		}, &bob)
		requireStatus(t, rec, http.StatusCreated)
		bobsInv := decodeBody[*repo.Bond](t, rec).Investment.ID.String()

		save := h.bulkSaveAccrued(t, "2026-05", []map[string]any{
			{"investment_id": mine.Investment.ID.String(), "amount": "1000", "accrued_interest": "0", "currency": "IDR"},
			{"investment_id": bobsInv, "amount": "1000", "accrued_interest": "0", "currency": "IDR"},
		})
		requireStatus(t, save, http.StatusUnprocessableEntity)
		if got := h.rawInvSnapshotCount(t, mine.Investment.ID); got != 0 {
			t.Errorf("cross-tenant batch must write nothing; wrote %d", got)
		}
		body := decodeBody[invBulkErrBody](t, save)
		if len(body.Errors) != 1 || body.Errors[0].InvestmentID != bobsInv {
			t.Fatalf("want Bob's bond flagged, got %+v", body.Errors)
		}
	})

	t.Run("T7 an empty batch writes nothing", func(t *testing.T) {
		h := newHarness(t)
		rec := h.bulkSaveAccrued(t, "2026-05", []map[string]any{})
		requireStatus(t, rec, http.StatusOK)
		body := decodeBody[struct {
			Written int `json:"written"`
		}](t, rec)
		if body.Written != 0 {
			t.Errorf("empty batch should write 0, got %d", body.Written)
		}
	})
}

func TestInvestmentSnapshotHandlers_BulkAccruedValidation(t *testing.T) {
	h := newHarness(t)
	bond := h.createBond(t, "Validation parent")
	goodRow := []map[string]any{{"investment_id": bond.Investment.ID.String(), "amount": "1000", "accrued_interest": "0", "currency": "IDR"}}

	cases := []struct {
		name string
		body any
		want int
	}{
		{"malformed JSON", "{not json", http.StatusBadRequest},
		{"missing year_month", map[string]any{"rows": goodRow}, http.StatusBadRequest},
		{"unparseable year_month", map[string]any{"year_month": "May 2026", "rows": goodRow}, http.StatusBadRequest},
		{"future year_month", map[string]any{"year_month": "2031-02", "rows": goodRow}, http.StatusBadRequest},
		{"bad as_of_date format", map[string]any{"year_month": "2026-05", "as_of_date": "05/31/2026", "rows": goodRow}, http.StatusBadRequest},
		{"future as_of_date", map[string]any{"year_month": "2031-01", "as_of_date": "2031-01-02", "rows": goodRow}, http.StatusBadRequest},
		{"row missing amount", map[string]any{"year_month": "2026-05", "rows": []map[string]any{{"investment_id": bond.Investment.ID.String(), "accrued_interest": "0", "currency": "IDR"}}}, http.StatusBadRequest},
		{"row missing accrued_interest", map[string]any{"year_month": "2026-05", "rows": []map[string]any{{"investment_id": bond.Investment.ID.String(), "amount": "1000", "currency": "IDR"}}}, http.StatusBadRequest},
		{"row missing currency", map[string]any{"year_month": "2026-05", "rows": []map[string]any{{"investment_id": bond.Investment.ID.String(), "amount": "1000", "accrued_interest": "0"}}}, http.StatusBadRequest},
		{"row bad uuid shape", map[string]any{"year_month": "2026-05", "rows": []map[string]any{{"investment_id": "not-a-uuid", "amount": "1000", "accrued_interest": "0", "currency": "IDR"}}}, http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := h.do(t, "POST", "/investments/snapshots/accrued/bulk", tc.body)
			requireStatus(t, rec, tc.want)
		})
	}

	t.Run("entry list rejects a bad year_month", func(t *testing.T) {
		requireStatus(t, h.do(t, "GET", "/investments/snapshots/accrued/entry?year_month=nope", nil), http.StatusBadRequest)
	})
	t.Run("entry list rejects a future year_month", func(t *testing.T) {
		requireStatus(t, h.do(t, "GET", "/investments/snapshots/accrued/entry?year_month=2031-02", nil), http.StatusBadRequest)
	})
}

type accruedEntryRow struct {
	InvestmentID           string  `json:"investment_id"`
	DisplayName            string  `json:"display_name"`
	Currency               string  `json:"currency"`
	Subtype                string  `json:"subtype"`
	CouponDisposition      *string `json:"coupon_disposition"`
	PrefillAmount          *string `json:"prefill_amount"`
	PrefillAccruedInterest *string `json:"prefill_accrued_interest"`
	CarriedFrom            *string `json:"carried_from"`
}

type accruedEntryBody struct {
	Rows []accruedEntryRow `json:"rows"`
}

// covers: INV-SNAPSHOTS-07
// covers: INV-SNAPSHOTS-09
func TestInvestmentSnapshotHandlers_AccruedEntryList(t *testing.T) {
	h := newHarness(t)
	withHist := h.createBond(t, "Has history")
	accrues := h.createBondWithDisposition(t, "Accrues bond", "accrues")
	paysOut := h.createBondWithDisposition(t, "Pays-out bond", "pays_out")
	td := h.createTimeDeposit(t, "In-term TD")   // 2026-01..2027-01, no disposition
	stock := h.createStock(t, "Qty×price stock") // must be excluded — qty×price shape

	// withHist gets an April accrued snapshot.
	requireStatus(t, h.bulkSaveAccrued(t, "2026-04", []map[string]any{
		{"investment_id": withHist.Investment.ID.String(), "amount": "50250000", "accrued_interest": "250000", "currency": "IDR"},
	}), http.StatusOK)

	rec := h.do(t, "GET", "/investments/snapshots/accrued/entry?year_month=2026-05", nil)
	requireStatus(t, rec, http.StatusOK)
	body := decodeBody[accruedEntryBody](t, rec)

	rows := make(map[string]accruedEntryRow, len(body.Rows))
	for _, r := range body.Rows {
		rows[r.InvestmentID] = r
	}

	if _, ok := rows[stock.Investment.ID.String()]; ok {
		t.Errorf("a stock (qty×price shape) must not appear in the accrued entry list")
	}

	hist, ok := rows[withHist.Investment.ID.String()]
	if !ok {
		t.Fatalf("bond with history missing from entry list")
	}
	if hist.PrefillAmount == nil || *hist.PrefillAmount != "50250000" {
		t.Errorf("prefill should carry April total value 50250000, got %v", hist.PrefillAmount)
	}
	if hist.PrefillAccruedInterest == nil || *hist.PrefillAccruedInterest != "250000" {
		t.Errorf("prefill should carry April accrued 250000, got %v", hist.PrefillAccruedInterest)
	}
	if hist.CarriedFrom == nil || *hist.CarriedFrom != "2026-04" {
		t.Errorf("carried_from should be 2026-04, got %v", hist.CarriedFrom)
	}
	if hist.Subtype != "bond" {
		t.Errorf("subtype should be bond, got %q", hist.Subtype)
	}

	// Coupon disposition rides through so the client can seed the accrued
	// default (accrues → forced entry, pays_out → 0).
	if ac := rows[accrues.Investment.ID.String()]; ac.CouponDisposition == nil || *ac.CouponDisposition != "accrues" {
		t.Errorf("accrues bond should carry coupon_disposition=accrues, got %v", ac.CouponDisposition)
	}
	if po := rows[paysOut.Investment.ID.String()]; po.CouponDisposition == nil || *po.CouponDisposition != "pays_out" {
		t.Errorf("pays-out bond should carry coupon_disposition=pays_out, got %v", po.CouponDisposition)
	}

	// A time deposit has no bond_details row → null coupon_disposition (client
	// treats as pays_out) and null prefill (no history yet).
	tdRow, ok := rows[td.Investment.ID.String()]
	if !ok {
		t.Fatalf("in-term time deposit missing from entry list")
	}
	if tdRow.CouponDisposition != nil {
		t.Errorf("time deposit should carry null coupon_disposition, got %v", tdRow.CouponDisposition)
	}
	if tdRow.PrefillAmount != nil || tdRow.PrefillAccruedInterest != nil || tdRow.CarriedFrom != nil {
		t.Errorf("TD with no history should have null prefill, got %v / %v / %v",
			tdRow.PrefillAmount, tdRow.PrefillAccruedInterest, tdRow.CarriedFrom)
	}
}

// covers: INV-SNAPSHOTS-09
func TestInvestmentSnapshotHandlers_AccruedEntryList_ExcludesOutOfTermTD(t *testing.T) {
	h := newHarness(t)
	td := h.createTimeDeposit(t, "Placed Jan 2026") // 2026-01..2027-01

	// A month before placement: the TD is outside its term, so it must not
	// appear in the list for that month.
	rec := h.do(t, "GET", "/investments/snapshots/accrued/entry?year_month=2025-12", nil)
	requireStatus(t, rec, http.StatusOK)
	body := decodeBody[accruedEntryBody](t, rec)
	for _, r := range body.Rows {
		if r.InvestmentID == td.Investment.ID.String() {
			t.Errorf("out-of-term TD must not appear in the accrued entry list for 2025-12")
		}
	}
}
