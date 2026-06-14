package assets_test

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/kerti/balances-v2/backend/internal/db"
)

func (h *handlerHarness) createAssetSnapshot(t *testing.T, assetID uuid.UUID, yearMonth string) db.AssetSnapshot {
	t.Helper()
	rec := h.do(t, "POST", "/assets/"+assetID.String()+"/snapshots", map[string]any{
		"year_month": yearMonth,
		"amount":     "1000000",
		"currency":   "IDR",
	})
	requireStatus(t, rec, http.StatusCreated)
	return decodeBody[db.AssetSnapshot](t, rec)
}

// covers: INV-SNAPSHOTS-05
func TestAssetSnapshotHandlers_Create(t *testing.T) {
	h := newHarness(t)
	parent := h.createBankAccount(t, "Snapshot parent")

	t.Run("201 happy path with YYYY-MM", func(t *testing.T) {
		rec := h.do(t, "POST", "/assets/"+parent.Asset.ID.String()+"/snapshots", map[string]any{
			"year_month": "2026-05",
			"amount":     "5000000",
			"currency":   "IDR",
			"as_of_date": "2026-05-15",
		})
		requireStatus(t, rec, http.StatusCreated)
		body := decodeBody[db.AssetSnapshot](t, rec)
		if !decimal.NewFromInt(5000000).Equal(body.Amount) {
			t.Errorf("amount: want 5000000, got %s", body.Amount.String())
		}
	})

	t.Run("201 happy path with YYYY-MM-DD (normalised to first-of-month)", func(t *testing.T) {
		rec := h.do(t, "POST", "/assets/"+parent.Asset.ID.String()+"/snapshots", map[string]any{
			"year_month": "2026-08-17",
			"amount":     "6000000",
			"currency":   "IDR",
		})
		requireStatus(t, rec, http.StatusCreated)
		body := decodeBody[db.AssetSnapshot](t, rec)
		if body.YearMonth.Day() != 1 {
			t.Errorf("year_month should be normalised to first-of-month, got day %d", body.YearMonth.Day())
		}
	})

	t.Run("400 bad year_month", func(t *testing.T) {
		rec := h.do(t, "POST", "/assets/"+parent.Asset.ID.String()+"/snapshots", map[string]any{
			"year_month": "May 2026",
			"amount":     "1000",
			"currency":   "IDR",
		})
		requireStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("400 missing required amount", func(t *testing.T) {
		rec := h.do(t, "POST", "/assets/"+parent.Asset.ID.String()+"/snapshots", map[string]any{
			"year_month": "2026-06",
			"currency":   "IDR",
		})
		requireStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("400 bad as_of_date format", func(t *testing.T) {
		rec := h.do(t, "POST", "/assets/"+parent.Asset.ID.String()+"/snapshots", map[string]any{
			"year_month": "2026-07",
			"amount":     "1000",
			"currency":   "IDR",
			"as_of_date": "07/15/2026",
		})
		requireStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("404 unknown parent asset", func(t *testing.T) {
		rec := h.do(t, "POST", "/assets/"+uuid.NewString()+"/snapshots", map[string]any{
			"year_month": "2026-09",
			"amount":     "1000",
			"currency":   "IDR",
		})
		requireStatus(t, rec, http.StatusNotFound)
	})

	// fakeNow = 2030-01-01 UTC; anything past current month / today rejects.
	t.Run("400 future year_month", func(t *testing.T) {
		rec := h.do(t, "POST", "/assets/"+parent.Asset.ID.String()+"/snapshots", map[string]any{
			"year_month": "2030-02",
			"amount":     "1000",
			"currency":   "IDR",
		})
		requireStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("400 future as_of_date", func(t *testing.T) {
		rec := h.do(t, "POST", "/assets/"+parent.Asset.ID.String()+"/snapshots", map[string]any{
			"year_month": "2030-01",
			"amount":     "1000",
			"currency":   "IDR",
			"as_of_date": "2030-01-02",
		})
		requireStatus(t, rec, http.StatusBadRequest)
	})
}

func TestAssetSnapshotHandlers_List(t *testing.T) {
	h := newHarness(t)
	parent := h.createBankAccount(t, "Snapshot list parent")
	snap := h.createAssetSnapshot(t, parent.Asset.ID, "2026-04")

	rec := h.do(t, "GET", "/assets/"+parent.Asset.ID.String()+"/snapshots", nil)
	requireStatus(t, rec, http.StatusOK)
	list := decodeBody[[]db.AssetSnapshot](t, rec)
	if len(list) != 1 {
		t.Fatalf("list length: want 1, got %d", len(list))
	}
	if list[0].ID != snap.ID {
		t.Errorf("snapshot id: want %s, got %s", snap.ID, list[0].ID)
	}
}

// covers: INV-SNAPSHOTS-05
func TestAssetSnapshotHandlers_Update(t *testing.T) {
	h := newHarness(t)
	parent := h.createBankAccount(t, "Snapshot update parent")
	snap := h.createAssetSnapshot(t, parent.Asset.ID, "2026-03")

	t.Run("200 happy path", func(t *testing.T) {
		rec := h.do(t, "PATCH",
			"/assets/"+parent.Asset.ID.String()+"/snapshots/"+snap.ID.String(),
			map[string]any{
				"amount":   "7777777",
				"currency": "IDR",
			})
		requireStatus(t, rec, http.StatusOK)
		body := decodeBody[db.AssetSnapshot](t, rec)
		if !decimal.NewFromInt(7777777).Equal(body.Amount) {
			t.Errorf("amount: want 7777777, got %s", body.Amount.String())
		}
	})

	t.Run("404 unknown snapshot", func(t *testing.T) {
		rec := h.do(t, "PATCH",
			"/assets/"+parent.Asset.ID.String()+"/snapshots/"+uuid.NewString(),
			map[string]any{
				"amount":   "1",
				"currency": "IDR",
			})
		requireStatus(t, rec, http.StatusNotFound)
	})

	t.Run("400 bad as_of_date format", func(t *testing.T) {
		rec := h.do(t, "PATCH",
			"/assets/"+parent.Asset.ID.String()+"/snapshots/"+snap.ID.String(),
			map[string]any{
				"amount":     "1",
				"currency":   "IDR",
				"as_of_date": "tomorrow",
			})
		requireStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("400 future as_of_date", func(t *testing.T) {
		rec := h.do(t, "PATCH",
			"/assets/"+parent.Asset.ID.String()+"/snapshots/"+snap.ID.String(),
			map[string]any{
				"amount":     "1",
				"currency":   "IDR",
				"as_of_date": "2030-01-02",
			})
		requireStatus(t, rec, http.StatusBadRequest)
	})
}

func TestAssetSnapshotHandlers_Delete(t *testing.T) {
	h := newHarness(t)
	parent := h.createBankAccount(t, "Snapshot delete parent")

	t.Run("204 happy path", func(t *testing.T) {
		snap := h.createAssetSnapshot(t, parent.Asset.ID, "2026-02")
		rec := h.do(t, "DELETE",
			"/assets/"+parent.Asset.ID.String()+"/snapshots/"+snap.ID.String(), nil)
		requireStatus(t, rec, http.StatusNoContent)
	})

	t.Run("404 unknown snapshot", func(t *testing.T) {
		rec := h.do(t, "DELETE",
			"/assets/"+parent.Asset.ID.String()+"/snapshots/"+uuid.NewString(), nil)
		requireStatus(t, rec, http.StatusNotFound)
	})
}
