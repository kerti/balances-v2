package investments_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kerti/balances-v2/backend/internal/identity"
)

// TestInvestmentHandlers_ListRepoError covers the repo-error (500) branch of
// every list handler — including the cross-subtype time-series endpoint — which
// the happy-path suites never reach. A cancelled request context makes the
// underlying query fail, routing the handler through httperr.WriteRepo.
func TestInvestmentHandlers_ListRepoError(t *testing.T) {
	h := newHarness(t)
	parent := h.createStock(t, "List error parent")
	pid := parent.Investment.ID.String()
	paths := []string{
		"/investments/stocks",
		"/investments/mutual-funds",
		"/investments/golds",
		"/investments/bonds",
		"/investments/time-deposits",
		"/investments/time-series",
		"/investments/" + pid + "/snapshots",
		"/investments/" + pid + "/transactions",
	}
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			requireStatus(t, h.getCancelled(path), http.StatusInternalServerError)
		})
	}
}

func (h *handlerHarness) getCancelled(path string) *httptest.ResponseRecorder {
	ctx, cancel := context.WithCancel(identity.WithUser(context.Background(), h.user))
	cancel()
	req := httptest.NewRequest(http.MethodGet, path, nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	h.router.ServeHTTP(rec, req)
	return rec
}
