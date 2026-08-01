package repo

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/kerti/balances-v2/backend/internal/db"
	"github.com/kerti/balances-v2/backend/internal/identity"
	"github.com/kerti/balances-v2/backend/internal/testutil"
)

// TestSoftDeletePosition_SharedHelperGuards pins the entry guards of
// softDeleteAsset / softDeleteInvestment — the shared delete path every Asset
// and Investment subtype wrapper funnels into.
//
// These live in an in-package test on purpose. Every wrapper today
// (DeleteBankAccount, DeleteStock, …) runs a Get guard first, which returns on
// the same three conditions, so from outside the package the helper's own
// guards are unreachable rather than untested. They are not redundant: the
// helper is the single point every present and future subtype delete passes
// through, and the `rows == 0` arm in particular is a real race guard — the
// position can be deleted by someone else between the wrapper's Get and this
// UPDATE. Asserting them here keeps a future wrapper that forgets the Get from
// turning a missing identity, a dead context or a lost race into a silent
// success. Liability and Receivable get the same assertions through their
// public Delete in soft_delete_cascade_test.go.
//
// covers: INV-SOFT-DELETE-05
func TestSoftDeletePosition_SharedHelperGuards(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)

	user := testutil.CreateHouseholdWithUser(t, q, "Alice")
	ctx := identity.WithUser(context.Background(), user)

	assets := NewAssetRepo(tdb.Pool)
	investments := NewInvestmentRepo(tdb.Pool)

	helpers := []struct {
		name string
		del  func(context.Context, uuid.UUID) error
	}{
		{"softDeleteAsset", assets.softDeleteAsset},
		{"softDeleteInvestment", investments.softDeleteInvestment},
	}

	for _, h := range helpers {
		t.Run(h.name, func(t *testing.T) {
			t.Run("no identity on the context", func(t *testing.T) {
				if err := h.del(context.Background(), uuid.New()); !errors.Is(err, ErrUnauthenticated) {
					t.Errorf("want ErrUnauthenticated, got %v", err)
				}
			})

			t.Run("context already cancelled", func(t *testing.T) {
				deadCtx, cancel := context.WithCancel(ctx)
				cancel()
				if err := h.del(deadCtx, uuid.New()); !errors.Is(err, context.Canceled) {
					t.Errorf("want context.Canceled, got %v", err)
				}
			})

			// The race the wrapper's Get cannot close: by the time the UPDATE
			// runs the row is gone (or was never ours). The cascade matches no
			// rows and reports no error, so the parent UPDATE is what has to
			// notice — otherwise a delete of nothing reports success.
			t.Run("unknown id is ErrNotFound, not a silent success", func(t *testing.T) {
				if err := h.del(ctx, uuid.New()); !errors.Is(err, ErrNotFound) {
					t.Errorf("want ErrNotFound, got %v", err)
				}
			})
		})
	}
}
