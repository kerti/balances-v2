package liabilities_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Bulk monthly-entry conformance for the Liability group (ADR-0046) — the
// amount-only twin of the Asset tracer's suite (#421), proving the mode
// generalises to liabilities. Raw DB assertions bypass the handler getters so a
// mutation is provable without trusting the same read path that could mask a
// false positive.

func (h *handlerHarness) rawSnapshotCount(t *testing.T, liabilityID uuid.UUID) int {
	t.Helper()
	var n int
	err := h.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM liability_snapshots WHERE liability_id = $1 AND deleted_at IS NULL`,
		liabilityID).Scan(&n)
	if err != nil {
		t.Fatalf("raw snapshot count: %v", err)
	}
	return n
}

func (h *handlerHarness) rawSnapshotAmount(t *testing.T, liabilityID uuid.UUID, yearMonth string) decimal.Decimal {
	t.Helper()
	var amt decimal.Decimal
	err := h.pool.QueryRow(context.Background(),
		`SELECT amount FROM liability_snapshots
		 WHERE liability_id = $1 AND year_month = $2::date AND deleted_at IS NULL`,
		liabilityID, yearMonth+"-01").Scan(&amt)
	if err != nil {
		t.Fatalf("raw snapshot amount: %v", err)
	}
	return amt
}

func (h *handlerHarness) bulkSave(t *testing.T, yearMonth string, rows []map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	return h.do(t, "POST", "/liabilities/snapshots/bulk", map[string]any{
		"year_month": yearMonth,
		"as_of_date": yearMonth + "-28",
		"rows":       rows,
	})
}

type bulkErrRow struct {
	LiabilityID string `json:"liability_id"`
	Code        string `json:"code"`
}

type bulkErrBody struct {
	Errors []bulkErrRow `json:"errors"`
}

// covers: INV-SNAPSHOTS-06
func TestLiabilitySnapshotHandlers_Bulk(t *testing.T) {
	t.Run("T1 one dirty row writes one snapshot", func(t *testing.T) {
		h := newHarness(t)
		l := h.createLiability(t, "Bulk tracer", "personal")

		rec := h.bulkSave(t, "2026-05", []map[string]any{
			{"liability_id": l.ID.String(), "amount": "5000000", "currency": "IDR"},
		})
		requireStatus(t, rec, http.StatusOK)

		if got := h.rawSnapshotCount(t, l.ID); got != 1 {
			t.Errorf("want 1 snapshot written, got %d", got)
		}
	})

	t.Run("T2 re-entering a month upserts, no duplicate", func(t *testing.T) {
		h := newHarness(t)
		l := h.createLiability(t, "Bulk reentry", "institutional")

		requireStatus(t, h.bulkSave(t, "2026-05", []map[string]any{
			{"liability_id": l.ID.String(), "amount": "1000000", "currency": "IDR"},
		}), http.StatusOK)
		requireStatus(t, h.bulkSave(t, "2026-05", []map[string]any{
			{"liability_id": l.ID.String(), "amount": "7000000", "currency": "IDR"},
		}), http.StatusOK)

		if got := h.rawSnapshotCount(t, l.ID); got != 1 {
			t.Fatalf("want 1 snapshot after re-entry, got %d", got)
		}
		if got := h.rawSnapshotAmount(t, l.ID, "2026-05"); !decimal.NewFromInt(7000000).Equal(got) {
			t.Errorf("want upserted amount 7000000, got %s", got.String())
		}
	})

	t.Run("T3 one ineligible row aborts the whole batch with a per-row error", func(t *testing.T) {
		h := newHarness(t)
		good := h.createLiability(t, "Good row", "personal")
		bad := uuid.New() // never created — not owned by this household

		rec := h.bulkSave(t, "2026-05", []map[string]any{
			{"liability_id": good.ID.String(), "amount": "1000000", "currency": "IDR"},
			{"liability_id": bad.String(), "amount": "2000000", "currency": "IDR"},
		})
		requireStatus(t, rec, http.StatusUnprocessableEntity)

		// Atomicity: the valid row must not have been written either.
		if got := h.rawSnapshotCount(t, good.ID); got != 0 {
			t.Errorf("a batch with any bad row must write nothing; wrote %d", got)
		}

		body := decodeBody[bulkErrBody](t, rec)
		if len(body.Errors) != 1 {
			t.Fatalf("want 1 per-row error, got %d", len(body.Errors))
		}
		if body.Errors[0].LiabilityID != bad.String() {
			t.Errorf("per-row error should key the bad liability %s, got %s", bad, body.Errors[0].LiabilityID)
		}
	})

	t.Run("T4 a liability terminated before the target month is ineligible", func(t *testing.T) {
		h := newHarness(t)
		closed := h.createLiability(t, "Paid off in April", "personal")

		requireStatus(t, h.do(t, "PATCH", "/liabilities/"+closed.ID.String()+"/lifecycle",
			map[string]any{"status": "paid_off", "terminated_at": "2026-04-30"}), http.StatusOK)

		rec := h.bulkSave(t, "2026-05", []map[string]any{
			{"liability_id": closed.ID.String(), "amount": "1000000", "currency": "IDR"},
		})
		requireStatus(t, rec, http.StatusUnprocessableEntity)
		if got := h.rawSnapshotCount(t, closed.ID); got != 0 {
			t.Errorf("terminated-before-month liability must not be written; wrote %d", got)
		}
	})

	t.Run("T4b a liability terminated in the target month is still eligible", func(t *testing.T) {
		h := newHarness(t)
		l := h.createLiability(t, "Paid off in May", "personal")

		requireStatus(t, h.do(t, "PATCH", "/liabilities/"+l.ID.String()+"/lifecycle",
			map[string]any{"status": "paid_off", "terminated_at": "2026-05-15"}), http.StatusOK)

		requireStatus(t, h.bulkSave(t, "2026-05", []map[string]any{
			{"liability_id": l.ID.String(), "amount": "1000000", "currency": "IDR"},
		}), http.StatusOK)
		if got := h.rawSnapshotCount(t, l.ID); got != 1 {
			t.Errorf("liability terminated in the target month should accept the snapshot; wrote %d", got)
		}
	})

	t.Run("T6 an empty batch writes nothing", func(t *testing.T) {
		h := newHarness(t)
		requireStatus(t, h.bulkSave(t, "2026-05", []map[string]any{}), http.StatusOK)
	})
}

type entryRow struct {
	LiabilityID   string  `json:"liability_id"`
	DisplayName   string  `json:"display_name"`
	Currency      string  `json:"currency"`
	Subtype       string  `json:"subtype"`
	PrefillAmount *string `json:"prefill_amount"`
	CarriedFrom   *string `json:"carried_from"`
}

type entryBody struct {
	Rows []entryRow `json:"rows"`
}

// covers: INV-SNAPSHOTS-07
func TestLiabilitySnapshotHandlers_EntryList(t *testing.T) {
	h := newHarness(t)
	withHist := h.createLiability(t, "Has history", "personal")
	fresh := h.createLiability(t, "No history", "institutional")
	closedEarly := h.createLiability(t, "Paid off in March", "personal")

	requireStatus(t, h.bulkSave(t, "2026-04", []map[string]any{
		{"liability_id": withHist.ID.String(), "amount": "1234", "currency": "IDR"},
	}), http.StatusOK)
	requireStatus(t, h.do(t, "PATCH", "/liabilities/"+closedEarly.ID.String()+"/lifecycle",
		map[string]any{"status": "paid_off", "terminated_at": "2026-03-31"}), http.StatusOK)

	rec := h.do(t, "GET", "/liabilities/snapshots/entry?year_month=2026-05", nil)
	requireStatus(t, rec, http.StatusOK)
	body := decodeBody[entryBody](t, rec)

	rows := make(map[string]entryRow, len(body.Rows))
	for _, r := range body.Rows {
		rows[r.LiabilityID] = r
	}

	if _, ok := rows[closedEarly.ID.String()]; ok {
		t.Errorf("liability terminated before the target month must not appear in the entry list")
	}
	hist, ok := rows[withHist.ID.String()]
	if !ok {
		t.Fatalf("liability with history missing from entry list")
	}
	if hist.PrefillAmount == nil || *hist.PrefillAmount != "1234" {
		t.Errorf("prefill should carry the April value 1234, got %v", hist.PrefillAmount)
	}
	if hist.CarriedFrom == nil || *hist.CarriedFrom != "2026-04" {
		t.Errorf("carried_from should be 2026-04, got %v", hist.CarriedFrom)
	}
	if hist.Currency != "IDR" {
		t.Errorf("currency should be the liability's native IDR, got %s", hist.Currency)
	}
	if hist.Subtype != "personal" {
		t.Errorf("subtype should carry the liability subtype personal, got %q", hist.Subtype)
	}
	fr, ok := rows[fresh.ID.String()]
	if !ok {
		t.Fatalf("fresh liability missing from entry list")
	}
	if fr.PrefillAmount != nil || fr.CarriedFrom != nil {
		t.Errorf("liability with no history should have null prefill, got %v / %v", fr.PrefillAmount, fr.CarriedFrom)
	}
}
