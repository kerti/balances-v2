package repo_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/kerti/balances-v2/backend/internal/db"
	"github.com/kerti/balances-v2/backend/internal/identity"
	"github.com/kerti/balances-v2/backend/internal/repo"
	"github.com/kerti/balances-v2/backend/internal/testutil"
)

// TestTagRepoBatch covers the Tags page's batch set-tag and item listing (#853):
// one transaction across all four position groups, a move off another tag,
// all-or-nothing on a foreign id, and household isolation on both the write and
// the item list.
// covers: INV-TENANCY-12, INV-TAGS-06
func TestTagRepoBatch(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)

	alice := testutil.CreateHouseholdWithUser(t, q, "Alice")
	bob := testutil.CreateHouseholdWithUser(t, q, "Bob")
	aliceCtx := identity.WithUser(context.Background(), alice)
	bobCtx := identity.WithUser(context.Background(), bob)

	tr := repo.NewTagRepo(tdb.Pool)
	ar := repo.NewAssetRepo(tdb.Pool)
	lr := repo.NewLiabilityRepo(tdb.Pool)
	rr := repo.NewReceivableRepo(tdb.Pool)
	ir := repo.NewInvestmentRepo(tdb.Pool)

	prop, err := ar.CreateProperty(aliceCtx, repo.CreatePropertyParams{
		DisplayName: "House", OwnershipType: "joint", NativeCurrency: "IDR", PropertyType: "house",
	})
	if err != nil {
		t.Fatalf("CreateProperty: %v", err)
	}
	liab, err := lr.CreateLiability(aliceCtx, repo.CreateLiabilityParams{
		DisplayName: "Loan", Subtype: "personal", OwnershipType: "joint", NativeCurrency: "IDR", CounterpartyName: "Friend",
	})
	if err != nil {
		t.Fatalf("CreateLiability: %v", err)
	}
	recv, err := rr.CreateReceivable(aliceCtx, repo.CreateReceivableParams{
		DisplayName: "Owed", OwnershipType: "joint", NativeCurrency: "IDR", CounterpartyName: "Cousin",
	})
	if err != nil {
		t.Fatalf("CreateReceivable: %v", err)
	}
	stock, err := ir.CreateStock(aliceCtx, repo.CreateStockParams{
		DisplayName: "Shares", OwnershipType: "joint", NativeCurrency: "IDR", RiskProfile: "medium", Ticker: "ABCD", Exchange: "IDX",
	})
	if err != nil {
		t.Fatalf("CreateStock: %v", err)
	}
	bobRecv, err := rr.CreateReceivable(bobCtx, repo.CreateReceivableParams{
		DisplayName: "Bob owed", OwnershipType: "joint", NativeCurrency: "IDR", CounterpartyName: "Pal",
	})
	if err != nil {
		t.Fatalf("bob CreateReceivable: %v", err)
	}

	tagA, err := tr.CreateTag(aliceCtx, "Alpha", "#3b82f6")
	if err != nil {
		t.Fatalf("CreateTag A: %v", err)
	}
	tagB, err := tr.CreateTag(aliceCtx, "Beta", "#10b981")
	if err != nil {
		t.Fatalf("CreateTag B: %v", err)
	}
	bobTag, err := tr.CreateTag(bobCtx, "Gamma", "#ec4899")
	if err != nil {
		t.Fatalf("bob CreateTag: %v", err)
	}

	tagOf := func(name string) *uuid.UUID {
		items, err := tr.ListTaggableItems(aliceCtx)
		if err != nil {
			t.Fatalf("ListTaggableItems: %v", err)
		}
		for _, it := range items {
			if it.Name == name {
				return it.TagID
			}
		}
		t.Fatalf("item %q not listed", name)
		return nil
	}

	four := []repo.TagItemRef{
		{Group: repo.TagGroupAsset, PositionID: prop.Asset.ID},
		{Group: repo.TagGroupLiability, PositionID: liab.ID},
		{Group: repo.TagGroupReceivable, PositionID: recv.ID},
		{Group: repo.TagGroupInvestment, PositionID: stock.Investment.ID},
	}

	t.Run("one batch tags every group, and an item with no snapshot still lists", func(t *testing.T) {
		if err := tr.AssignTagBatch(aliceCtx, &tagA.ID, four); err != nil {
			t.Fatalf("AssignTagBatch: %v", err)
		}
		for _, n := range []string{"House", "Loan", "Owed", "Shares"} {
			if got := tagOf(n); got == nil || *got != tagA.ID {
				t.Errorf("%s tag = %v, want %v", n, got, tagA.ID)
			}
		}
		items, _ := tr.ListTaggableItems(aliceCtx)
		if len(items) != 4 || items[0].Currency != nil || items[0].Amount != nil {
			t.Errorf("items = %+v; want 4 rows with nil value (no snapshots)", items)
		}
	})

	t.Run("a batch moves items off another tag (one tag per item)", func(t *testing.T) {
		if err := tr.AssignTagBatch(aliceCtx, &tagB.ID, four[:2]); err != nil {
			t.Fatalf("AssignTagBatch move: %v", err)
		}
		if got := tagOf("House"); got == nil || *got != tagB.ID {
			t.Errorf("House tag = %v, want %v", got, tagB.ID)
		}
		if got := tagOf("Owed"); got == nil || *got != tagA.ID {
			t.Errorf("Owed tag = %v, want unchanged %v", got, tagA.ID)
		}
	})

	t.Run("a nil tag clears the batch", func(t *testing.T) {
		if err := tr.AssignTagBatch(aliceCtx, nil, four[:2]); err != nil {
			t.Fatalf("AssignTagBatch clear: %v", err)
		}
		if got := tagOf("House"); got != nil {
			t.Errorf("House tag = %v, want nil", got)
		}
	})

	t.Run("a foreign id rolls the whole batch back", func(t *testing.T) {
		mixed := append([]repo.TagItemRef{}, four[:2]...)
		mixed = append(mixed, repo.TagItemRef{Group: repo.TagGroupReceivable, PositionID: bobRecv.ID})
		if err := tr.AssignTagBatch(aliceCtx, &tagB.ID, mixed); !errors.Is(err, repo.ErrNotFound) {
			t.Fatalf("mixed batch: got %v, want ErrNotFound", err)
		}
		if got := tagOf("House"); got != nil {
			t.Errorf("House tag = %v after failed batch; want nil (rolled back)", got)
		}
	})

	t.Run("a foreign tag is ErrNotFound with no write", func(t *testing.T) {
		if err := tr.AssignTagBatch(aliceCtx, &bobTag.ID, four); !errors.Is(err, repo.ErrNotFound) {
			t.Errorf("alice with bob's tag: got %v, want ErrNotFound", err)
		}
		if err := tr.AssignTagBatch(bobCtx, &tagA.ID, []repo.TagItemRef{{Group: repo.TagGroupReceivable, PositionID: bobRecv.ID}}); !errors.Is(err, repo.ErrNotFound) {
			t.Errorf("bob with alice's tag: got %v, want ErrNotFound", err)
		}
	})

	t.Run("bob cannot tag alice's items and does not list them", func(t *testing.T) {
		if err := tr.AssignTagBatch(bobCtx, &bobTag.ID, four); !errors.Is(err, repo.ErrNotFound) {
			t.Errorf("bob batch on alice items: got %v, want ErrNotFound", err)
		}
		items, err := tr.ListTaggableItems(bobCtx)
		if err != nil {
			t.Fatalf("bob ListTaggableItems: %v", err)
		}
		if len(items) != 1 || items[0].Name != "Bob owed" {
			t.Errorf("bob items = %+v; want only his own receivable", items)
		}
	})

	t.Run("an item with a snapshot lists its latest value and currency", func(t *testing.T) {
		if _, err := rr.CreateReceivableSnapshot(aliceCtx, repo.CreateReceivableSnapshotParams{
			ReceivableID: recv.ID,
			YearMonth:    time.Date(2026, time.May, 1, 0, 0, 0, 0, time.UTC),
			Amount:       decimal.NewFromInt(1234),
			Currency:     "IDR",
		}); err != nil {
			t.Fatalf("CreateReceivableSnapshot: %v", err)
		}
		items, _ := tr.ListTaggableItems(aliceCtx)
		for _, it := range items {
			if it.Name == "Owed" && (it.Currency == nil || *it.Currency != "IDR" || it.Amount == nil || !it.Amount.Equal(decimal.NewFromInt(1234))) {
				t.Errorf("Owed = %+v; want IDR 1234", it)
			}
		}
	})

	t.Run("duplicate ids in one batch are tolerated", func(t *testing.T) {
		dup := []repo.TagItemRef{four[2], four[2]}
		if err := tr.AssignTagBatch(aliceCtx, &tagB.ID, dup); err != nil {
			t.Errorf("duplicate ids: got %v, want nil", err)
		}
	})
}
