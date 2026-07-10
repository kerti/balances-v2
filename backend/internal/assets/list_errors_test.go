package assets_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kerti/balances-v2/backend/internal/identity"
)

// TestAssetHandlers_ListRepoError covers the repo-error (500) branch of each
// list handler — the one arm the happy-path suites never reach. A cancelled
// request context makes the underlying query fail, so the handler routes
// through httperr.WriteRepo and emits the INTERNAL envelope.
func TestAssetHandlers_ListRepoError(t *testing.T) {
	h := newHarness(t)
	parent := h.createBankAccount(t, "List error parent")
	paths := []string{
		"/bank-accounts",
		"/properties",
		"/vehicles",
		"/assets/" + parent.Asset.ID.String() + "/snapshots",
	}
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			requireStatus(t, h.getCancelled(path), http.StatusInternalServerError)
		})
	}
}

// getCancelled issues a GET with an already-cancelled (but authenticated)
// context so the handler's downstream query fails deterministically.
func (h *handlerHarness) getCancelled(path string) *httptest.ResponseRecorder {
	ctx, cancel := context.WithCancel(identity.WithUser(context.Background(), h.user))
	cancel()
	req := httptest.NewRequest(http.MethodGet, path, nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	h.router.ServeHTTP(rec, req)
	return rec
}
