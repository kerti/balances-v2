package assets_test

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/kerti/balances-v2/backend/internal/db"
	"github.com/kerti/balances-v2/backend/internal/identity"
	"github.com/kerti/balances-v2/backend/internal/repo"
	"github.com/kerti/balances-v2/backend/internal/testutil"
)

// foreignTag creates a Tag in a second household and returns its id.
func (h *handlerHarness) foreignTag(t *testing.T) string {
	t.Helper()
	bob := testutil.CreateHouseholdWithUser(t, db.New(h.pool), "Bob")
	tag, err := repo.NewTagRepo(h.pool).CreateTag(identity.WithUser(context.Background(), bob), "Bob's", "#f59e0b")
	if err != nil {
		t.Fatalf("seed foreign tag: %v", err)
	}
	return tag.ID.String()
}

// TestAssetHandlers_TagID: bank account, property and vehicle create/update
// carry an optional tag_id (issue #839).
// covers: INV-TAGS-07
func TestAssetHandlers_TagID(t *testing.T) {
	h := newHarness(t)
	own := h.seedTag(t, "Goals").String()
	foreign := h.foreignTag(t)

	testutil.RunTagIDContract(t, func(method, path string, body any) *httptest.ResponseRecorder {
		return h.do(t, method, path, body)
	}, own, foreign, []testutil.TagIDCase{
		{
			Name: "bank account", Path: "/bank-accounts", Core: "asset",
			Body: map[string]any{"display_name": "Main", "ownership_type": "joint", "native_currency": "IDR", "bank_name": "BCA", "account_type": "savings"},
		},
		{
			Name: "property", Path: "/properties", Core: "asset",
			Body: map[string]any{"display_name": "House", "ownership_type": "joint", "native_currency": "IDR", "property_type": "house"},
		},
		{
			Name: "vehicle", Path: "/vehicles", Core: "asset",
			Body: map[string]any{"display_name": "Car", "ownership_type": "joint", "native_currency": "IDR", "vehicle_type": "car"},
		},
	})
}
