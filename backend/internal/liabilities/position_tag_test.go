package liabilities_test

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/kerti/balances-v2/backend/internal/db"
	"github.com/kerti/balances-v2/backend/internal/identity"
	"github.com/kerti/balances-v2/backend/internal/repo"
	"github.com/kerti/balances-v2/backend/internal/testutil"
)

// TestLiabilityHandlers_TagID: create/update carry an optional tag_id (issue #839).
// covers: INV-TAGS-07
func TestLiabilityHandlers_TagID(t *testing.T) {
	h := newHarness(t)
	own := h.seedTag(t, "Debts").String()

	bob := testutil.CreateHouseholdWithUser(t, db.New(h.pool), "Bob")
	ft, err := repo.NewTagRepo(h.pool).CreateTag(identity.WithUser(context.Background(), bob), "Bob's", "#f59e0b")
	if err != nil {
		t.Fatalf("seed foreign tag: %v", err)
	}

	testutil.RunTagIDContract(t, func(method, path string, body any) *httptest.ResponseRecorder {
		return h.do(t, method, path, body)
	}, own, ft.ID.String(), []testutil.TagIDCase{
		{
			Name: "liability", Path: "/liabilities",
			Body: map[string]any{"display_name": "Loan", "subtype": "personal", "ownership_type": "joint", "native_currency": "IDR", "counterparty_name": "Uncle"},
		},
	})
}
