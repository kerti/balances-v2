package liabilities_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kerti/balances-v2/backend/internal/identity"
)

// TestLiabilityHandlers_ListRepoError covers the repo-error (500) branch of the
// list + snapshot-list handlers. A cancelled request context makes the
// underlying query fail, so the handler routes through httperr.WriteRepo and
// emits the INTERNAL envelope. The ?subtype= variant also exercises the filter
// assignment ahead of the failing query.
func TestLiabilityHandlers_ListRepoError(t *testing.T) {
	h := newHarness(t)
	parent := h.createLiability(t, "List error parent", "personal")
	paths := []string{
		"/liabilities",
		"/liabilities?subtype=personal",
		"/liabilities/" + parent.ID.String() + "/snapshots",
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
