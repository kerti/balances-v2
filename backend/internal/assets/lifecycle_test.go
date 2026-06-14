package assets_test

import (
	"net/http"
	"testing"
)

// TestAssetHandlers_Lifecycle covers PATCH /assets/{id}/lifecycle: the happy
// terminate path, the validator's required_unless guard (terminal status needs
// a date), the repo's biconditional (active must not carry a date) and unknown
// status, plus the bad-id / bad-json branches. The same handler shape is shared
// verbatim by the liabilities/receivables/investments packages, so this stands
// in for all four.
// covers: INV-LIFECYCLE-01
func TestAssetHandlers_Lifecycle(t *testing.T) {
	h := newHarness(t)
	parent := h.createBankAccount(t, "Lifecycle parent")
	base := "/assets/" + parent.Asset.ID.String() + "/lifecycle"

	type lifecycleResp struct {
		Status       string  `json:"status"`
		TerminatedAt *string `json:"terminated_at"`
	}

	t.Run("terminate happy path", func(t *testing.T) {
		rec := h.do(t, "PATCH", base, map[string]any{
			"status":           "sold",
			"terminated_at":    "2026-05-25",
			"termination_note": "sold to dealer",
		})
		requireStatus(t, rec, http.StatusOK)
		got := decodeBody[lifecycleResp](t, rec)
		if got.Status != "sold" || got.TerminatedAt == nil {
			t.Fatalf("unexpected body: %+v", got)
		}
	})

	t.Run("terminal status without date is 400", func(t *testing.T) {
		rec := h.do(t, "PATCH", base, map[string]any{"status": "closed"})
		requireStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("active status with date is 400", func(t *testing.T) {
		rec := h.do(t, "PATCH", base, map[string]any{
			"status": "active", "terminated_at": "2026-05-25",
		})
		requireStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("unknown status is 400", func(t *testing.T) {
		rec := h.do(t, "PATCH", base, map[string]any{
			"status": "frozen", "terminated_at": "2026-05-25",
		})
		requireStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("bad date format is 400", func(t *testing.T) {
		rec := h.do(t, "PATCH", base, map[string]any{
			"status": "sold", "terminated_at": "05/25/2026",
		})
		requireStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("invalid id is 400", func(t *testing.T) {
		rec := h.do(t, "PATCH", "/assets/not-a-uuid/lifecycle", map[string]any{
			"status": "sold", "terminated_at": "2026-05-25",
		})
		requireStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("invalid json is 400", func(t *testing.T) {
		rec := h.do(t, "PATCH", base, "{not-json")
		requireStatus(t, rec, http.StatusBadRequest)
	})
}
