package assets_test

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

// rawSnapshotCount counts live snapshots for an asset directly against the DB,
// bypassing the handler getters — a mutation must be provable without trusting
// the same read path that could mask a false positive.
func (h *handlerHarness) rawSnapshotCount(t *testing.T, assetID uuid.UUID) int {
	t.Helper()
	var n int
	err := h.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM asset_snapshots WHERE asset_id = $1 AND deleted_at IS NULL`,
		assetID).Scan(&n)
	if err != nil {
		t.Fatalf("raw snapshot count: %v", err)
	}
	return n
}

// rawLatestSnapshotAmount returns the amount of the single live snapshot for an
// asset in a given month (raw, bypassing the handler getters).
func (h *handlerHarness) rawSnapshotAmount(t *testing.T, assetID uuid.UUID, yearMonth string) decimal.Decimal {
	t.Helper()
	var amt decimal.Decimal
	err := h.pool.QueryRow(context.Background(),
		`SELECT amount FROM asset_snapshots
		 WHERE asset_id = $1 AND year_month = $2::date AND deleted_at IS NULL`,
		assetID, yearMonth+"-01").Scan(&amt)
	if err != nil {
		t.Fatalf("raw snapshot amount: %v", err)
	}
	return amt
}

func (h *handlerHarness) bulkSave(t *testing.T, yearMonth string, rows []map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	return h.do(t, "POST", "/assets/snapshots/bulk", map[string]any{
		"year_month": yearMonth,
		"as_of_date": yearMonth + "-28",
		"rows":       rows,
	})
}

// covers: INV-SNAPSHOTS-06
func TestAssetSnapshotHandlers_Bulk(t *testing.T) {
	t.Run("T1 one dirty row writes one snapshot", func(t *testing.T) {
		h := newHarness(t)
		acct := h.createBankAccount(t, "Bulk tracer")

		rec := h.do(t, "POST", "/assets/snapshots/bulk", map[string]any{
			"year_month": "2026-05",
			"as_of_date": "2026-05-31",
			"rows": []map[string]any{
				{"asset_id": acct.Asset.ID.String(), "amount": "5000000", "currency": "IDR"},
			},
		})
		requireStatus(t, rec, http.StatusOK)

		if got := h.rawSnapshotCount(t, acct.Asset.ID); got != 1 {
			t.Errorf("want 1 snapshot written, got %d", got)
		}
	})

	t.Run("T2 re-entering a month upserts, no duplicate", func(t *testing.T) {
		h := newHarness(t)
		acct := h.createBankAccount(t, "Bulk reentry")

		requireStatus(t, h.bulkSave(t, "2026-05", []map[string]any{
			{"asset_id": acct.Asset.ID.String(), "amount": "1000000", "currency": "IDR"},
		}), http.StatusOK)

		requireStatus(t, h.bulkSave(t, "2026-05", []map[string]any{
			{"asset_id": acct.Asset.ID.String(), "amount": "7000000", "currency": "IDR"},
		}), http.StatusOK)

		if got := h.rawSnapshotCount(t, acct.Asset.ID); got != 1 {
			t.Fatalf("want 1 snapshot after re-entry, got %d", got)
		}
		if got := h.rawSnapshotAmount(t, acct.Asset.ID, "2026-05"); !decimal.NewFromInt(7000000).Equal(got) {
			t.Errorf("want upserted amount 7000000, got %s", got.String())
		}
	})

	t.Run("T3 one ineligible row aborts the whole batch with a per-row error", func(t *testing.T) {
		h := newHarness(t)
		good := h.createBankAccount(t, "Good row")
		bad := uuid.New() // never created — not owned by this household

		rec := h.bulkSave(t, "2026-05", []map[string]any{
			{"asset_id": good.Asset.ID.String(), "amount": "1000000", "currency": "IDR"},
			{"asset_id": bad.String(), "amount": "2000000", "currency": "IDR"},
		})
		requireStatus(t, rec, http.StatusUnprocessableEntity)

		// Atomicity: the valid row must not have been written either.
		if got := h.rawSnapshotCount(t, good.Asset.ID); got != 0 {
			t.Errorf("a batch with any bad row must write nothing; wrote %d", got)
		}

		body := decodeBody[bulkErrBody](t, rec)
		if len(body.Errors) != 1 {
			t.Fatalf("want 1 per-row error, got %d", len(body.Errors))
		}
		if body.Errors[0].AssetID != bad.String() {
			t.Errorf("per-row error should key the bad asset %s, got %s", bad, body.Errors[0].AssetID)
		}
	})

	t.Run("T4 an asset terminated before the target month is ineligible", func(t *testing.T) {
		h := newHarness(t)
		closed := h.createBankAccount(t, "Closed in April")
		stillOpen := h.createBankAccount(t, "Still open")

		// Close the first account effective 2026-04-30 — before the 2026-05 target.
		requireStatus(t, h.do(t, "PATCH", "/assets/"+closed.Asset.ID.String()+"/lifecycle",
			map[string]any{"status": "closed", "terminated_at": "2026-04-30"}), http.StatusOK)

		rec := h.bulkSave(t, "2026-05", []map[string]any{
			{"asset_id": stillOpen.Asset.ID.String(), "amount": "1000000", "currency": "IDR"},
			{"asset_id": closed.Asset.ID.String(), "amount": "2000000", "currency": "IDR"},
		})
		requireStatus(t, rec, http.StatusUnprocessableEntity)

		if got := h.rawSnapshotCount(t, stillOpen.Asset.ID); got != 0 {
			t.Errorf("batch with an ineligible row must write nothing; wrote %d", got)
		}
		body := decodeBody[bulkErrBody](t, rec)
		if len(body.Errors) != 1 || body.Errors[0].AssetID != closed.Asset.ID.String() {
			t.Fatalf("want the closed asset flagged ineligible, got %+v", body.Errors)
		}
	})

	t.Run("T5 another household's asset is ineligible (tenancy)", func(t *testing.T) {
		h := newHarness(t)
		mine := h.createBankAccount(t, "Mine")

		// Bob's household + an asset in it; created as Bob, not Alice.
		bob := testutil.CreateHouseholdWithUser(t, db.New(h.pool), "Bob")
		rec := h.doRaw(t, "POST", "/bank-accounts", map[string]any{
			"display_name": "Bob's account", "ownership_type": "joint", "native_currency": "IDR",
			"bank_name": "TestBank", "account_number": "1234567890", "account_type": "savings",
		}, &bob)
		requireStatus(t, rec, http.StatusCreated)
		bobsAsset := decodeBody[*repo.BankAccount](t, rec).Asset.ID.String()

		// Alice tries to write a snapshot against Bob's asset.
		save := h.bulkSave(t, "2026-05", []map[string]any{
			{"asset_id": mine.Asset.ID.String(), "amount": "1000000", "currency": "IDR"},
			{"asset_id": bobsAsset, "amount": "2000000", "currency": "IDR"},
		})
		requireStatus(t, save, http.StatusUnprocessableEntity)
		if got := h.rawSnapshotCount(t, mine.Asset.ID); got != 0 {
			t.Errorf("cross-tenant batch must write nothing; wrote %d", got)
		}
		body := decodeBody[bulkErrBody](t, save)
		if len(body.Errors) != 1 || body.Errors[0].AssetID != bobsAsset {
			t.Fatalf("want Bob's asset flagged, got %+v", body.Errors)
		}
	})

	t.Run("T6 an empty batch writes nothing", func(t *testing.T) {
		h := newHarness(t)
		rec := h.bulkSave(t, "2026-05", []map[string]any{})
		requireStatus(t, rec, http.StatusOK)
		body := decodeBody[struct {
			Written int `json:"written"`
		}](t, rec)
		if body.Written != 0 {
			t.Errorf("empty batch should write 0, got %d", body.Written)
		}
	})

	t.Run("T4b an asset terminated in the target month is still eligible", func(t *testing.T) {
		h := newHarness(t)
		closedThisMonth := h.createBankAccount(t, "Closed in May")

		// Terminated 2026-05-20 — you still enter the closing month's balance.
		requireStatus(t, h.do(t, "PATCH", "/assets/"+closedThisMonth.Asset.ID.String()+"/lifecycle",
			map[string]any{"status": "closed", "terminated_at": "2026-05-20"}), http.StatusOK)

		rec := h.bulkSave(t, "2026-05", []map[string]any{
			{"asset_id": closedThisMonth.Asset.ID.String(), "amount": "3000000", "currency": "IDR"},
		})
		requireStatus(t, rec, http.StatusOK)
		if got := h.rawSnapshotCount(t, closedThisMonth.Asset.ID); got != 1 {
			t.Errorf("asset closed in the target month should accept its snapshot; wrote %d", got)
		}
	})
}

type bulkErrBody struct {
	Errors []struct {
		AssetID string `json:"asset_id"`
		Code    string `json:"code"`
	} `json:"errors"`
}

func TestAssetSnapshotHandlers_BulkValidation(t *testing.T) {
	h := newHarness(t)
	acct := h.createBankAccount(t, "Validation parent")
	goodRow := []map[string]any{{"asset_id": acct.Asset.ID.String(), "amount": "1000", "currency": "IDR"}}

	cases := []struct {
		name string
		body any
		want int
	}{
		{"malformed JSON", "{not json", http.StatusBadRequest},
		{"missing year_month", map[string]any{"rows": goodRow}, http.StatusBadRequest},
		{"unparseable year_month", map[string]any{"year_month": "May 2026", "rows": goodRow}, http.StatusBadRequest},
		{"future year_month", map[string]any{"year_month": "2030-02", "rows": goodRow}, http.StatusBadRequest},
		{"bad as_of_date format", map[string]any{"year_month": "2026-05", "as_of_date": "05/31/2026", "rows": goodRow}, http.StatusBadRequest},
		{"future as_of_date", map[string]any{"year_month": "2030-01", "as_of_date": "2030-01-03", "rows": goodRow}, http.StatusBadRequest},
		// #426: fakeNow = 2030-01-01 UTC; a UTC+ member's local-today can be
		// one civil day ahead, so 2030-01-02 is a valid same-day save here.
		{"as_of_date one civil day ahead (tz tolerance)", map[string]any{"year_month": "2030-01", "as_of_date": "2030-01-02", "rows": goodRow}, http.StatusOK},
		{"row missing currency", map[string]any{"year_month": "2026-05", "rows": []map[string]any{{"asset_id": acct.Asset.ID.String(), "amount": "1000"}}}, http.StatusBadRequest},
		{"row bad uuid shape", map[string]any{"year_month": "2026-05", "rows": []map[string]any{{"asset_id": "not-a-uuid", "amount": "1000", "currency": "IDR"}}}, http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := h.do(t, "POST", "/assets/snapshots/bulk", tc.body)
			requireStatus(t, rec, tc.want)
		})
	}

	t.Run("entry list rejects a bad year_month", func(t *testing.T) {
		requireStatus(t, h.do(t, "GET", "/assets/snapshots/entry?year_month=nope", nil), http.StatusBadRequest)
	})
	t.Run("entry list rejects a future year_month", func(t *testing.T) {
		requireStatus(t, h.do(t, "GET", "/assets/snapshots/entry?year_month=2030-02", nil), http.StatusBadRequest)
	})
}

type entryRow struct {
	AssetID       string  `json:"asset_id"`
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
func TestAssetSnapshotHandlers_EntryList(t *testing.T) {
	h := newHarness(t)
	withHist := h.createBankAccount(t, "Has history")
	fresh := h.createBankAccount(t, "No history")
	closedEarly := h.createBankAccount(t, "Closed in March")

	// withHist gets an April snapshot; closedEarly is terminated before May.
	requireStatus(t, h.bulkSave(t, "2026-04", []map[string]any{
		{"asset_id": withHist.Asset.ID.String(), "amount": "1234", "currency": "IDR"},
	}), http.StatusOK)
	requireStatus(t, h.do(t, "PATCH", "/assets/"+closedEarly.Asset.ID.String()+"/lifecycle",
		map[string]any{"status": "closed", "terminated_at": "2026-03-31"}), http.StatusOK)

	rec := h.do(t, "GET", "/assets/snapshots/entry?year_month=2026-05", nil)
	requireStatus(t, rec, http.StatusOK)
	body := decodeBody[entryBody](t, rec)

	rows := make(map[string]entryRow, len(body.Rows))
	for _, r := range body.Rows {
		rows[r.AssetID] = r
	}

	if _, ok := rows[closedEarly.Asset.ID.String()]; ok {
		t.Errorf("asset terminated before the target month must not appear in the entry list")
	}
	hist, ok := rows[withHist.Asset.ID.String()]
	if !ok {
		t.Fatalf("asset with history missing from entry list")
	}
	if hist.PrefillAmount == nil || *hist.PrefillAmount != "1234" {
		t.Errorf("prefill should carry the April value 1234, got %v", hist.PrefillAmount)
	}
	if hist.CarriedFrom == nil || *hist.CarriedFrom != "2026-04" {
		t.Errorf("carried_from should be 2026-04, got %v", hist.CarriedFrom)
	}
	if hist.Currency != "IDR" {
		t.Errorf("currency should be the asset's native IDR, got %s", hist.Currency)
	}
	if hist.Subtype != "bank_account" {
		t.Errorf("subtype should be bank_account (createBankAccount), got %q", hist.Subtype)
	}
	fr, ok := rows[fresh.Asset.ID.String()]
	if !ok {
		t.Fatalf("fresh asset missing from entry list")
	}
	if fr.PrefillAmount != nil || fr.CarriedFrom != nil {
		t.Errorf("asset with no history should have null prefill, got %v / %v", fr.PrefillAmount, fr.CarriedFrom)
	}
}

// covers: INV-SNAPSHOTS-07
func TestAssetSnapshotHandlers_EntryList_SoleOwner(t *testing.T) {
	h := newHarness(t)
	rec := h.do(t, "POST", "/bank-accounts", map[string]any{
		"display_name":       "Alice's private account",
		"ownership_type":     "sole",
		"sole_owner_user_id": h.user.ID.String(),
		"native_currency":    "IDR",
		"bank_name":          "TestBank",
		"account_number":     "9999",
		"account_type":       "savings",
	})
	requireStatus(t, rec, http.StatusCreated)
	acct := decodeBody[*repo.BankAccount](t, rec)

	got := h.do(t, "GET", "/assets/snapshots/entry?year_month=2026-05", nil)
	requireStatus(t, got, http.StatusOK)
	body := decodeBody[struct {
		Rows []struct {
			AssetID         string  `json:"asset_id"`
			SoleOwnerUserID *string `json:"sole_owner_user_id"`
		} `json:"rows"`
	}](t, got)

	var found bool
	for _, r := range body.Rows {
		if r.AssetID == acct.Asset.ID.String() {
			found = true
			if r.SoleOwnerUserID == nil || *r.SoleOwnerUserID != h.user.ID.String() {
				t.Errorf("sole-owned asset entry row: want sole_owner_user_id %s, got %v", h.user.ID, r.SoleOwnerUserID)
			}
		}
	}
	if !found {
		t.Fatalf("sole-owned asset missing from entry list")
	}
}
