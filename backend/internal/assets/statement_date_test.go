package assets_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
)

// rawAsOfDate reads the stored statement date of the live snapshot for a month
// straight from the table. A nil result is a SQL NULL, not a zero time.
func (h *handlerHarness) rawAsOfDate(t *testing.T, assetID uuid.UUID, yearMonth string) *time.Time {
	t.Helper()
	var d *time.Time
	err := h.pool.QueryRow(context.Background(),
		`SELECT as_of_date FROM asset_snapshots
		 WHERE asset_id = $1 AND year_month = $2::date AND deleted_at IS NULL`,
		assetID, yearMonth+"-01").Scan(&d)
	if err != nil {
		t.Fatalf("raw as_of_date: %v", err)
	}
	return d
}

// covers: INV-SNAPSHOTS-13
//
// The statement date is never invented (ADR-0046 amendment, #790). A bulk save
// or a single Record snapshot that carries no date stores as_of_date = NULL —
// whether the key is absent, null, or an empty string (what a cleared date input
// sends) — and a date the user did pick is stored as given.
func TestAssetSnapshots_StatementDateStaysNullUnlessPicked(t *testing.T) {
	h := newHarness(t)
	acct := h.createBankAccount(t, "Statement date")
	id := acct.Asset.ID
	row := func(amount string) []map[string]any {
		return []map[string]any{{"asset_id": id.String(), "amount": amount, "currency": "IDR"}}
	}

	cases := []struct {
		name  string
		month string
		body  map[string]any
		want  string // "" = NULL
	}{
		{"bulk, key absent", "2026-01", map[string]any{"year_month": "2026-01", "rows": row("100")}, ""},
		{"bulk, null", "2026-02", map[string]any{"year_month": "2026-02", "as_of_date": nil, "rows": row("200")}, ""},
		{"bulk, empty string", "2026-03", map[string]any{"year_month": "2026-03", "as_of_date": "", "rows": row("300")}, ""},
		{"bulk, picked date", "2026-04", map[string]any{"year_month": "2026-04", "as_of_date": "2026-04-17", "rows": row("400")}, "2026-04-17"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			requireStatus(t, h.do(t, "POST", "/assets/snapshots/bulk", tc.body), http.StatusOK)
			assertAsOf(t, h.rawAsOfDate(t, id, tc.month), tc.want)
		})
	}

	t.Run("single create without a date", func(t *testing.T) {
		rec := h.do(t, "POST", "/assets/"+id.String()+"/snapshots", map[string]any{
			"year_month": "2026-05", "amount": "500", "currency": "IDR",
		})
		requireStatus(t, rec, http.StatusCreated)
		assertAsOf(t, h.rawAsOfDate(t, id, "2026-05"), "")
	})

	t.Run("single create with a picked date", func(t *testing.T) {
		rec := h.do(t, "POST", "/assets/"+id.String()+"/snapshots", map[string]any{
			"year_month": "2026-06", "amount": "600", "currency": "IDR", "as_of_date": "2026-06-09",
		})
		requireStatus(t, rec, http.StatusCreated)
		assertAsOf(t, h.rawAsOfDate(t, id, "2026-06"), "2026-06-09")
	})
}

func assertAsOf(t *testing.T, got *time.Time, want string) {
	t.Helper()
	switch {
	case want == "" && got != nil:
		t.Errorf("as_of_date = %s, want NULL", got.Format("2006-01-02"))
	case want != "" && got == nil:
		t.Errorf("as_of_date is NULL, want %s", want)
	case want != "" && got.Format("2006-01-02") != want:
		t.Errorf("as_of_date = %s, want %s", got.Format("2006-01-02"), want)
	}
}
