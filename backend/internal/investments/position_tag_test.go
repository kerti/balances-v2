package investments_test

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/kerti/balances-v2/backend/internal/db"
	"github.com/kerti/balances-v2/backend/internal/identity"
	"github.com/kerti/balances-v2/backend/internal/repo"
	"github.com/kerti/balances-v2/backend/internal/testutil"
)

// TestInvestmentHandlers_TagID: all five investment subtypes' create/update
// carry an optional tag_id (issue #839).
// covers: INV-TAGS-07
func TestInvestmentHandlers_TagID(t *testing.T) {
	h := newHarness(t)

	ownCtx := identity.WithUser(context.Background(), h.user)
	tags := repo.NewTagRepo(h.pool)
	ownTag, err := tags.CreateTag(ownCtx, "Goals", "#22c55e")
	if err != nil {
		t.Fatalf("seed tag: %v", err)
	}
	bob := testutil.CreateHouseholdWithUser(t, db.New(h.pool), "Bob")
	foreignTag, err := tags.CreateTag(identity.WithUser(context.Background(), bob), "Bob's", "#f59e0b")
	if err != nil {
		t.Fatalf("seed foreign tag: %v", err)
	}

	base := func(extra map[string]any) map[string]any {
		out := map[string]any{"display_name": "X", "ownership_type": "joint", "native_currency": "IDR", "risk_profile": "medium"}
		for k, v := range extra {
			out[k] = v
		}
		return out
	}

	testutil.RunTagIDContract(t, func(method, path string, body any) *httptest.ResponseRecorder {
		return h.do(t, method, path, body)
	}, ownTag.ID.String(), foreignTag.ID.String(), []testutil.TagIDCase{
		{Name: "stock", Path: "/investments/stocks", Core: "investment", Body: base(map[string]any{"ticker": "BBCA", "exchange": "IDX"})},
		{Name: "mutual fund", Path: "/investments/mutual-funds", Core: "investment", Body: base(map[string]any{"fund_code": "SCMUS", "fund_type": "money_market"})},
		{Name: "gold", Path: "/investments/golds", Core: "investment", Body: base(map[string]any{"form": "bar", "purity": "0.9999"})},
		{Name: "bond", Path: "/investments/bonds", Core: "investment", Body: base(map[string]any{
			"bond_type": "secondary_market", "issuer": "Govt", "coupon_rate": "6.25", "coupon_frequency": "monthly", "maturity_date": "2030-01-01",
		})},
		{Name: "time deposit", Path: "/investments/time-deposits", Core: "investment", Body: base(map[string]any{
			"bank_name": "BCA", "principal": "50000000", "interest_rate": "5.5", "term_months": 12,
			"placement_date": "2026-01-15", "maturity_date": "2027-01-15", "rollover_policy": "auto_renew_principal",
		})},
	})
}
