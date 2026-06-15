package repo_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/kerti/balances-v2/backend/internal/auth"
	"github.com/kerti/balances-v2/backend/internal/db"
	"github.com/kerti/balances-v2/backend/internal/repo"
	"github.com/kerti/balances-v2/backend/internal/testutil"
)

// TestTagRepo covers the Tag lifecycle (ADR-0028): tenancy isolation on CRUD +
// assignment, the dedicated-endpoint assignment path, the per-currency
// breakdown aggregate, and delete-clears-assignments. Mirrors the per-group
// tenancy suites.
// covers: INV-TENANCY-12, INV-TAGS-01, INV-TAGS-02, INV-TAGS-03, INV-TAGS-04
func TestTagRepo(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)

	aliceUser := testutil.CreateHouseholdWithUser(t, q, "Alice")
	bobUser := testutil.CreateHouseholdWithUser(t, q, "Bob")
	if aliceUser.HouseholdID == bobUser.HouseholdID {
		t.Fatalf("fixture: alice and bob ended up in the same household")
	}
	aliceCtx := auth.WithUser(context.Background(), aliceUser)
	bobCtx := auth.WithUser(context.Background(), bobUser)

	tr := repo.NewTagRepo(tdb.Pool)
	rr := repo.NewReceivableRepo(tdb.Pool)

	aliceTag, err := tr.CreateTag(aliceCtx, "BCA", "#3b82f6")
	if err != nil {
		t.Fatalf("alice CreateTag: %v", err)
	}

	// Alice's tagged receivable, with a snapshot so it contributes to the
	// breakdown.
	recv, err := rr.CreateReceivable(aliceCtx, repo.CreateReceivableParams{
		DisplayName:      "Loan to brother",
		OwnershipType:    "joint",
		NativeCurrency:   "IDR",
		CounterpartyName: "Brother",
	})
	if err != nil {
		t.Fatalf("alice CreateReceivable: %v", err)
	}
	if _, err := rr.CreateReceivableSnapshot(aliceCtx, repo.CreateReceivableSnapshotParams{
		ReceivableID: recv.ID,
		YearMonth:    time.Date(2026, time.May, 1, 0, 0, 0, 0, time.UTC),
		Amount:       decimal.NewFromInt(50_000_000),
		Currency:     "IDR",
	}); err != nil {
		t.Fatalf("alice CreateReceivableSnapshot: %v", err)
	}

	// A second, deliberately untagged receivable so the breakdown has both a
	// tagged cell and the Untagged bucket to reconcile against.
	recvUntagged, err := rr.CreateReceivable(aliceCtx, repo.CreateReceivableParams{
		DisplayName:      "Loan to sister",
		OwnershipType:    "joint",
		NativeCurrency:   "IDR",
		CounterpartyName: "Sister",
	})
	if err != nil {
		t.Fatalf("alice CreateReceivable (untagged): %v", err)
	}
	if _, err := rr.CreateReceivableSnapshot(aliceCtx, repo.CreateReceivableSnapshotParams{
		ReceivableID: recvUntagged.ID,
		YearMonth:    time.Date(2026, time.May, 1, 0, 0, 0, 0, time.UTC),
		Amount:       decimal.NewFromInt(20_000_000),
		Currency:     "IDR",
	}); err != nil {
		t.Fatalf("alice CreateReceivableSnapshot (untagged): %v", err)
	}

	t.Run("duplicate name (case-insensitive) is ErrTagNameExists", func(t *testing.T) {
		if _, err := tr.CreateTag(aliceCtx, "bca", "#fff"); !errors.Is(err, repo.ErrTagNameExists) {
			t.Errorf("CreateTag dup: got %v, want ErrTagNameExists", err)
		}
	})

	t.Run("rename onto an existing name (case-insensitive) is ErrTagNameExists", func(t *testing.T) {
		other, err := tr.CreateTag(aliceCtx, "Mandiri", "#10b981")
		if err != nil {
			t.Fatalf("CreateTag Mandiri: %v", err)
		}
		if _, err := tr.UpdateTag(aliceCtx, other.ID, "bca", "#fff"); !errors.Is(err, repo.ErrTagNameExists) {
			t.Errorf("UpdateTag onto existing name: got %v, want ErrTagNameExists", err)
		}
		// The collision must not have mutated the row.
		if got, err := tr.GetTag(aliceCtx, other.ID); err != nil || got.Name != "Mandiri" {
			t.Errorf("after failed rename: got (%v, %v); want name unchanged", got, err)
		}
		// A soft-deleted name is freed for reuse: the partial unique index is
		// WHERE deleted_at IS NULL, so renaming Mandiri onto it now succeeds.
		if err := tr.DeleteTag(aliceCtx, other.ID); err != nil {
			t.Fatalf("DeleteTag Mandiri: %v", err)
		}
		freed, err := tr.CreateTag(aliceCtx, "Freed", "#ec4899")
		if err != nil {
			t.Fatalf("CreateTag Freed: %v", err)
		}
		if _, err := tr.UpdateTag(aliceCtx, freed.ID, "Mandiri", "#10b981"); err != nil {
			t.Errorf("reuse of soft-deleted name: got %v, want nil", err)
		}
		// Clean up so the breakdown/list assertions below stay about aliceTag.
		if err := tr.DeleteTag(aliceCtx, freed.ID); err != nil {
			t.Fatalf("DeleteTag Freed: %v", err)
		}
	})

	t.Run("bob cannot see, update, or delete alice's tag", func(t *testing.T) {
		if list, err := tr.ListTags(bobCtx); err != nil || len(list) != 0 {
			t.Errorf("bob ListTags: got (%v, %v); want ([], nil)", list, err)
		}
		if _, err := tr.GetTag(bobCtx, aliceTag.ID); !errors.Is(err, repo.ErrNotFound) {
			t.Errorf("bob GetTag: got %v, want ErrNotFound", err)
		}
		if _, err := tr.UpdateTag(bobCtx, aliceTag.ID, "Hacked", "#000"); !errors.Is(err, repo.ErrNotFound) {
			t.Errorf("bob UpdateTag: got %v, want ErrNotFound", err)
		}
		if err := tr.DeleteTag(bobCtx, aliceTag.ID); !errors.Is(err, repo.ErrNotFound) {
			t.Errorf("bob DeleteTag: got %v, want ErrNotFound", err)
		}
	})

	t.Run("assign rejects cross-tenant tag and unknown position", func(t *testing.T) {
		// Bob assigning Alice's tag to Alice's receivable: the tag isn't his.
		if err := tr.AssignTag(bobCtx, repo.TagGroupReceivable, recv.ID, &aliceTag.ID); !errors.Is(err, repo.ErrNotFound) {
			t.Errorf("bob assign alice tag: got %v, want ErrNotFound", err)
		}
		// Alice assigning her tag to a position that isn't hers / doesn't exist.
		if err := tr.AssignTag(aliceCtx, repo.TagGroupReceivable, bobUser.ID, &aliceTag.ID); !errors.Is(err, repo.ErrNotFound) {
			t.Errorf("alice assign to unknown position: got %v, want ErrNotFound", err)
		}
	})

	t.Run("assign then breakdown reflects the tag, scoped per household", func(t *testing.T) {
		if err := tr.AssignTag(aliceCtx, repo.TagGroupReceivable, recv.ID, &aliceTag.ID); err != nil {
			t.Fatalf("alice AssignTag: %v", err)
		}
		got, err := rr.GetReceivable(aliceCtx, recv.ID)
		if err != nil {
			t.Fatalf("GetReceivable: %v", err)
		}
		if got.TagID == nil || *got.TagID != aliceTag.ID {
			t.Errorf("receivable tag_id = %v, want %v", got.TagID, aliceTag.ID)
		}

		rows, err := tr.TagBreakdown(aliceCtx)
		if err != nil {
			t.Fatalf("alice TagBreakdown: %v", err)
		}
		var tagged, untagged bool
		recvTotal := decimal.Zero
		for _, r := range rows {
			if r.Grp != "receivable" {
				continue
			}
			recvTotal = recvTotal.Add(r.Total)
			switch {
			case r.TagID != nil && *r.TagID == aliceTag.ID:
				tagged = true
				if !r.Total.Equal(decimal.NewFromInt(50_000_000)) {
					t.Errorf("tagged breakdown total = %s, want 50000000", r.Total)
				}
			case r.TagID == nil:
				untagged = true
				if !r.Total.Equal(decimal.NewFromInt(20_000_000)) {
					t.Errorf("untagged breakdown total = %s, want 20000000", r.Total)
				}
			}
		}
		if !tagged {
			t.Errorf("alice breakdown missing the tagged receivable cell: %+v", rows)
		}
		if !untagged {
			t.Errorf("alice breakdown missing the Untagged bucket: %+v", rows)
		}
		// Reconciliation: tagged + Untagged cover every receivable, so the cells
		// sum back to the household's receivable total (INV-FINANCE-01 cut by tag).
		if !recvTotal.Equal(decimal.NewFromInt(70_000_000)) {
			t.Errorf("receivable cells sum = %s, want 70000000 (50M tagged + 20M untagged)", recvTotal)
		}

		if bobRows, err := tr.TagBreakdown(bobCtx); err != nil || len(bobRows) != 0 {
			t.Errorf("bob TagBreakdown: got (%+v, %v); want ([], nil)", bobRows, err)
		}
	})

	t.Run("deleting a tag clears its assignments", func(t *testing.T) {
		if err := tr.DeleteTag(aliceCtx, aliceTag.ID); err != nil {
			t.Fatalf("alice DeleteTag: %v", err)
		}
		got, err := rr.GetReceivable(aliceCtx, recv.ID)
		if err != nil {
			t.Fatalf("GetReceivable after delete: %v", err)
		}
		if got.TagID != nil {
			t.Errorf("receivable tag_id = %v after tag delete; want nil", got.TagID)
		}
		// The receivable now reports under the Untagged bucket (nil tag_id).
		rows, err := tr.TagBreakdown(aliceCtx)
		if err != nil {
			t.Fatalf("TagBreakdown after delete: %v", err)
		}
		for _, r := range rows {
			if r.TagID != nil {
				t.Errorf("breakdown still has a tagged cell after delete: %+v", r)
			}
		}
		// And the tag is gone from the list.
		if list, err := tr.ListTags(aliceCtx); err != nil || len(list) != 0 {
			t.Errorf("alice ListTags after delete: got (%v, %v); want ([], nil)", list, err)
		}
	})
}
