package investments_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/kerti/balances-v2/backend/internal/db"
	"github.com/kerti/balances-v2/backend/internal/repo"
	"github.com/kerti/balances-v2/backend/internal/testutil"
)

// rawInvSnapshotCount counts live snapshots for an investment directly against
// the DB, bypassing the handler getters — a mutation must be provable without
// trusting the same read path that could mask a false positive.
func (h *handlerHarness) rawInvSnapshotCount(t *testing.T, investmentID uuid.UUID) int {
	t.Helper()
	var n int
	err := h.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM investment_snapshots WHERE investment_id = $1 AND deleted_at IS NULL`,
		investmentID).Scan(&n)
	if err != nil {
		t.Fatalf("raw inv snapshot count: %v", err)
	}
	return n
}

// rawInvSnapshotShape returns the (amount, quantity, price_per_unit,
// accrued_interest) of the single live snapshot for an investment in a month.
func (h *handlerHarness) rawInvSnapshotShape(t *testing.T, investmentID uuid.UUID, yearMonth string) (amount decimal.Decimal, qty, price, accrued *decimal.Decimal) {
	t.Helper()
	err := h.pool.QueryRow(context.Background(),
		`SELECT amount, quantity, price_per_unit, accrued_interest FROM investment_snapshots
		 WHERE investment_id = $1 AND year_month = $2::date AND deleted_at IS NULL`,
		investmentID, yearMonth+"-01").Scan(&amount, &qty, &price, &accrued)
	if err != nil {
		t.Fatalf("raw inv snapshot shape: %v", err)
	}
	return amount, qty, price, accrued
}

func (h *handlerHarness) bulkSaveInv(t *testing.T, yearMonth string, rows []map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	return h.do(t, "POST", "/investments/snapshots/bulk", map[string]any{
		"year_month": yearMonth,
		"as_of_date": yearMonth + "-28",
		"rows":       rows,
	})
}

type invBulkErrBody struct {
	Errors []struct {
		InvestmentID string `json:"investment_id"`
		Code         string `json:"code"`
	} `json:"errors"`
}

// covers: INV-SNAPSHOTS-06
// covers: INV-SNAPSHOTS-08
func TestInvestmentSnapshotHandlers_BulkQtyPrice(t *testing.T) {
	t.Run("T1 one dirty row writes one snapshot with amount = qty × price", func(t *testing.T) {
		h := newHarness(t)
		stock := h.createStock(t, "Bulk tracer")

		rec := h.bulkSaveInv(t, "2026-05", []map[string]any{
			{"investment_id": stock.Investment.ID.String(), "quantity": "10", "price_per_unit": "1500", "currency": "IDR"},
		})
		requireStatus(t, rec, http.StatusOK)

		if got := h.rawInvSnapshotCount(t, stock.Investment.ID); got != 1 {
			t.Fatalf("want 1 snapshot written, got %d", got)
		}
		amount, qty, price, accrued := h.rawInvSnapshotShape(t, stock.Investment.ID, "2026-05")
		if !decimal.NewFromInt(15000).Equal(amount) {
			t.Errorf("amount should be derived qty×price = 15000, got %s", amount)
		}
		if qty == nil || !decimal.NewFromInt(10).Equal(*qty) {
			t.Errorf("quantity should be 10, got %v", qty)
		}
		if price == nil || !decimal.NewFromInt(1500).Equal(*price) {
			t.Errorf("price_per_unit should be 1500, got %v", price)
		}
		if accrued != nil {
			t.Errorf("accrued_interest must be null on the qty×price branch, got %v", accrued)
		}
	})

	t.Run("T2 re-entering a month upserts, no duplicate", func(t *testing.T) {
		h := newHarness(t)
		mf := h.createMutualFund(t, "Bulk reentry MF")

		requireStatus(t, h.bulkSaveInv(t, "2026-05", []map[string]any{
			{"investment_id": mf.Investment.ID.String(), "quantity": "100", "price_per_unit": "1000", "currency": "IDR"},
		}), http.StatusOK)
		requireStatus(t, h.bulkSaveInv(t, "2026-05", []map[string]any{
			{"investment_id": mf.Investment.ID.String(), "quantity": "100", "price_per_unit": "1200", "currency": "IDR"},
		}), http.StatusOK)

		if got := h.rawInvSnapshotCount(t, mf.Investment.ID); got != 1 {
			t.Fatalf("want 1 snapshot after re-entry, got %d", got)
		}
		amount, _, price, _ := h.rawInvSnapshotShape(t, mf.Investment.ID, "2026-05")
		if !decimal.NewFromInt(120000).Equal(amount) {
			t.Errorf("want upserted amount 120000, got %s", amount)
		}
		if price == nil || !decimal.NewFromInt(1200).Equal(*price) {
			t.Errorf("want upserted price 1200, got %v", price)
		}
	})

	t.Run("T3 one ineligible row aborts the whole batch with a per-row error", func(t *testing.T) {
		h := newHarness(t)
		good := h.createStock(t, "Good row")
		bad := uuid.New() // never created — not owned by this household

		rec := h.bulkSaveInv(t, "2026-05", []map[string]any{
			{"investment_id": good.Investment.ID.String(), "quantity": "5", "price_per_unit": "1000", "currency": "IDR"},
			{"investment_id": bad.String(), "quantity": "5", "price_per_unit": "1000", "currency": "IDR"},
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

	t.Run("T4 an investment terminated before the target month is ineligible", func(t *testing.T) {
		h := newHarness(t)
		closed := h.createStock(t, "Sold in April")
		stillOpen := h.createStock(t, "Still open")

		requireStatus(t, h.do(t, "PATCH", "/investments/"+closed.Investment.ID.String()+"/lifecycle",
			map[string]any{"status": "sold", "terminated_at": "2026-04-30"}), http.StatusOK)

		rec := h.bulkSaveInv(t, "2026-05", []map[string]any{
			{"investment_id": stillOpen.Investment.ID.String(), "quantity": "1", "price_per_unit": "1", "currency": "IDR"},
			{"investment_id": closed.Investment.ID.String(), "quantity": "1", "price_per_unit": "1", "currency": "IDR"},
		})
		requireStatus(t, rec, http.StatusUnprocessableEntity)
		if got := h.rawInvSnapshotCount(t, stillOpen.Investment.ID); got != 0 {
			t.Errorf("batch with an ineligible row must write nothing; wrote %d", got)
		}
		body := decodeBody[invBulkErrBody](t, rec)
		if len(body.Errors) != 1 || body.Errors[0].InvestmentID != closed.Investment.ID.String() {
			t.Fatalf("want the sold investment flagged ineligible, got %+v", body.Errors)
		}
	})

	t.Run("T4b an investment terminated in the target month is still eligible", func(t *testing.T) {
		h := newHarness(t)
		soldThisMonth := h.createStock(t, "Sold in May")
		requireStatus(t, h.do(t, "PATCH", "/investments/"+soldThisMonth.Investment.ID.String()+"/lifecycle",
			map[string]any{"status": "sold", "terminated_at": "2026-05-20"}), http.StatusOK)

		rec := h.bulkSaveInv(t, "2026-05", []map[string]any{
			{"investment_id": soldThisMonth.Investment.ID.String(), "quantity": "3", "price_per_unit": "1000", "currency": "IDR"},
		})
		requireStatus(t, rec, http.StatusOK)
		if got := h.rawInvSnapshotCount(t, soldThisMonth.Investment.ID); got != 1 {
			t.Errorf("investment sold in the target month should accept its snapshot; wrote %d", got)
		}
	})

	t.Run("T5 a bond (accrued shape) can never be written through the qty×price path", func(t *testing.T) {
		h := newHarness(t)
		bond := h.createBond(t, "Accrued-shape bond")

		rec := h.bulkSaveInv(t, "2026-05", []map[string]any{
			{"investment_id": bond.Investment.ID.String(), "quantity": "1", "price_per_unit": "1000", "currency": "IDR"},
		})
		requireStatus(t, rec, http.StatusUnprocessableEntity)
		if got := h.rawInvSnapshotCount(t, bond.Investment.ID); got != 0 {
			t.Errorf("bond must not receive a qty×price snapshot; wrote %d", got)
		}
		body := decodeBody[invBulkErrBody](t, rec)
		if len(body.Errors) != 1 || body.Errors[0].InvestmentID != bond.Investment.ID.String() {
			t.Fatalf("want the bond flagged ineligible for qty×price, got %+v", body.Errors)
		}
	})

	t.Run("T6 another household's investment is ineligible (tenancy)", func(t *testing.T) {
		h := newHarness(t)
		mine := h.createStock(t, "Mine")

		bob := testutil.CreateHouseholdWithUser(t, db.New(h.pool), "Bob")
		rec := h.doRaw(t, "POST", "/investments/stocks", map[string]any{
			"display_name": "Bob's stock", "ownership_type": "joint", "native_currency": "IDR",
			"ticker": "TLKM", "exchange": "IDX", "risk_profile": "medium",
		}, &bob)
		requireStatus(t, rec, http.StatusCreated)
		bobsInv := decodeBody[*repo.Stock](t, rec).Investment.ID.String()

		save := h.bulkSaveInv(t, "2026-05", []map[string]any{
			{"investment_id": mine.Investment.ID.String(), "quantity": "1", "price_per_unit": "1", "currency": "IDR"},
			{"investment_id": bobsInv, "quantity": "1", "price_per_unit": "1", "currency": "IDR"},
		})
		requireStatus(t, save, http.StatusUnprocessableEntity)
		if got := h.rawInvSnapshotCount(t, mine.Investment.ID); got != 0 {
			t.Errorf("cross-tenant batch must write nothing; wrote %d", got)
		}
		body := decodeBody[invBulkErrBody](t, save)
		if len(body.Errors) != 1 || body.Errors[0].InvestmentID != bobsInv {
			t.Fatalf("want Bob's investment flagged, got %+v", body.Errors)
		}
	})

	t.Run("T7 an empty batch writes nothing", func(t *testing.T) {
		h := newHarness(t)
		rec := h.bulkSaveInv(t, "2026-05", []map[string]any{})
		requireStatus(t, rec, http.StatusOK)
		body := decodeBody[struct {
			Written int `json:"written"`
		}](t, rec)
		if body.Written != 0 {
			t.Errorf("empty batch should write 0, got %d", body.Written)
		}
	})
}

func TestInvestmentSnapshotHandlers_BulkQtyPriceValidation(t *testing.T) {
	h := newHarness(t)
	stock := h.createStock(t, "Validation parent")
	goodRow := []map[string]any{{"investment_id": stock.Investment.ID.String(), "quantity": "1", "price_per_unit": "1", "currency": "IDR"}}

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
		{"row missing quantity", map[string]any{"year_month": "2026-05", "rows": []map[string]any{{"investment_id": stock.Investment.ID.String(), "price_per_unit": "1", "currency": "IDR"}}}, http.StatusBadRequest},
		{"row missing price_per_unit", map[string]any{"year_month": "2026-05", "rows": []map[string]any{{"investment_id": stock.Investment.ID.String(), "quantity": "1", "currency": "IDR"}}}, http.StatusBadRequest},
		{"row missing currency", map[string]any{"year_month": "2026-05", "rows": []map[string]any{{"investment_id": stock.Investment.ID.String(), "quantity": "1", "price_per_unit": "1"}}}, http.StatusBadRequest},
		{"row bad uuid shape", map[string]any{"year_month": "2026-05", "rows": []map[string]any{{"investment_id": "not-a-uuid", "quantity": "1", "price_per_unit": "1", "currency": "IDR"}}}, http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := h.do(t, "POST", "/investments/snapshots/bulk", tc.body)
			requireStatus(t, rec, tc.want)
		})
	}

	t.Run("entry list rejects a bad year_month", func(t *testing.T) {
		requireStatus(t, h.do(t, "GET", "/investments/snapshots/entry?year_month=nope", nil), http.StatusBadRequest)
	})
	t.Run("entry list rejects a future year_month", func(t *testing.T) {
		requireStatus(t, h.do(t, "GET", "/investments/snapshots/entry?year_month=2031-02", nil), http.StatusBadRequest)
	})
}

type invEntryRow struct {
	InvestmentID    string  `json:"investment_id"`
	DisplayName     string  `json:"display_name"`
	Currency        string  `json:"currency"`
	Subtype         string  `json:"subtype"`
	PrefillQuantity *string `json:"prefill_quantity"`
	PrefillPrice    *string `json:"prefill_price"`
	CarriedFrom     *string `json:"carried_from"`
}

type invEntryBody struct {
	Rows []invEntryRow `json:"rows"`
}

// covers: INV-SNAPSHOTS-07
// covers: INV-SNAPSHOTS-08
func TestInvestmentSnapshotHandlers_QtyPriceEntryList(t *testing.T) {
	h := newHarness(t)
	withHist := h.createStock(t, "Has history")
	fresh := h.createGold(t, "No history")
	soldEarly := h.createMutualFund(t, "Sold in March")
	bond := h.createBond(t, "Accrued bond") // must be excluded — accrued shape

	// withHist gets an April snapshot; soldEarly is terminated before May.
	requireStatus(t, h.bulkSaveInv(t, "2026-04", []map[string]any{
		{"investment_id": withHist.Investment.ID.String(), "quantity": "10", "price_per_unit": "1500", "currency": "IDR"},
	}), http.StatusOK)
	requireStatus(t, h.do(t, "PATCH", "/investments/"+soldEarly.Investment.ID.String()+"/lifecycle",
		map[string]any{"status": "sold", "terminated_at": "2026-03-31"}), http.StatusOK)

	rec := h.do(t, "GET", "/investments/snapshots/entry?year_month=2026-05", nil)
	requireStatus(t, rec, http.StatusOK)
	body := decodeBody[invEntryBody](t, rec)

	rows := make(map[string]invEntryRow, len(body.Rows))
	for _, r := range body.Rows {
		rows[r.InvestmentID] = r
	}

	if _, ok := rows[soldEarly.Investment.ID.String()]; ok {
		t.Errorf("investment sold before the target month must not appear in the entry list")
	}
	if _, ok := rows[bond.Investment.ID.String()]; ok {
		t.Errorf("a bond (accrued shape) must not appear in the qty×price entry list")
	}

	hist, ok := rows[withHist.Investment.ID.String()]
	if !ok {
		t.Fatalf("investment with history missing from entry list")
	}
	if hist.PrefillQuantity == nil || *hist.PrefillQuantity != "10" {
		t.Errorf("prefill should carry April quantity 10, got %v", hist.PrefillQuantity)
	}
	if hist.PrefillPrice == nil || *hist.PrefillPrice != "1500" {
		t.Errorf("prefill should carry April price 1500, got %v", hist.PrefillPrice)
	}
	if hist.CarriedFrom == nil || *hist.CarriedFrom != "2026-04" {
		t.Errorf("carried_from should be 2026-04, got %v", hist.CarriedFrom)
	}
	if hist.Subtype != "stock" {
		t.Errorf("subtype should be stock, got %q", hist.Subtype)
	}

	fr, ok := rows[fresh.Investment.ID.String()]
	if !ok {
		t.Fatalf("fresh investment missing from entry list")
	}
	if fr.PrefillQuantity != nil || fr.PrefillPrice != nil || fr.CarriedFrom != nil {
		t.Errorf("investment with no history should have null prefill, got %v / %v / %v",
			fr.PrefillQuantity, fr.PrefillPrice, fr.CarriedFrom)
	}
}
