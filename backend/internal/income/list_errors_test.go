package income_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kerti/balances-v2/backend/internal/identity"
)

// TestIncomeHandlers_ListRepoError covers the repo-error (500) branch of the
// list handler. A cancelled request context makes the underlying query fail, so
// the handler routes through httperr.WriteRepo and emits the INTERNAL envelope.
func TestIncomeHandlers_ListRepoError(t *testing.T) {
	h := newHarness(t)
	requireStatus(t, h.getCancelled("/income"), http.StatusInternalServerError)
}

func (h *handlerHarness) getCancelled(path string) *httptest.ResponseRecorder {
	ctx, cancel := context.WithCancel(identity.WithUser(context.Background(), h.user))
	cancel()
	req := httptest.NewRequest(http.MethodGet, path, nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	h.router.ServeHTTP(rec, req)
	return rec
}
