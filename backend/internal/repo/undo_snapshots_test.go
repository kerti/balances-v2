package repo_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/kerti/balances-v2/backend/internal/db"
	"github.com/kerti/balances-v2/backend/internal/identity"
	"github.com/kerti/balances-v2/backend/internal/repo"
	"github.com/kerti/balances-v2/backend/internal/testutil"
)

// Undo of a snapshot delete or replace (ADR-0058 §2), across all four groups and
// all three shapes. Every assertion about row state reads the table directly
// (readMonth / rawCount): the repo's own reads hide archived rows, so they
// cannot tell "restored" from "still archived but invisible".

// undoSnapOps is one snapshot group's delete / undo calls, indexed in step with
// replaceGroups so the shared fixtures (position, create, edit) are reused.
type undoSnapOps struct {
	del         func(ctx context.Context, id uuid.UUID) error
	undo        func(ctx context.Context, id uuid.UUID) error
	undoReplace func(ctx context.Context, restoreID, replacementID uuid.UUID) error
}

func undoSnapOpsFor(pool *pgxpool.Pool) []undoSnapOps {
	ar := repo.NewAssetRepo(pool)
	lr := repo.NewLiabilityRepo(pool)
	rr := repo.NewReceivableRepo(pool)
	ir := repo.NewInvestmentRepo(pool)
	asset := undoSnapOps{ar.DeleteAssetSnapshot, ar.UndoDeleteAssetSnapshot, ar.UndoReplaceAssetSnapshot}
	liab := undoSnapOps{lr.DeleteLiabilitySnapshot, lr.UndoDeleteLiabilitySnapshot, lr.UndoReplaceLiabilitySnapshot}
	recv := undoSnapOps{rr.DeleteReceivableSnapshot, rr.UndoDeleteReceivableSnapshot, rr.UndoReplaceReceivableSnapshot}
	inv := undoSnapOps{ir.DeleteInvestmentSnapshot, ir.UndoDeleteInvestmentSnapshot, ir.UndoReplaceInvestmentSnapshot}
	return []undoSnapOps{asset, liab, recv, inv, inv}
}

type undoSnapEnv struct {
	g        replaceGroup
	ops      undoSnapOps
	pool     *pgxpool.Pool
	alice    context.Context
	bob      context.Context
	position uuid.UUID
}

// forEachUndoSnapGroup runs fn once per snapshot group, each with a fresh
// database, an alice-owned position and a bob in another household.
func forEachUndoSnapGroup(t *testing.T, fn func(t *testing.T, e undoSnapEnv)) {
	t.Helper()
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)
	aliceUser := testutil.CreateHouseholdWithUser(t, q, "Alice")
	bobUser := testutil.CreateHouseholdWithUser(t, q, "Bob")
	alice := identity.WithUser(context.Background(), aliceUser)
	bob := identity.WithUser(context.Background(), bobUser)

	groups := replaceGroups(tdb.Pool)
	ops := undoSnapOpsFor(tdb.Pool)
	for i, g := range groups {
		t.Run(g.name, func(t *testing.T) {
			fn(t, undoSnapEnv{g: g, ops: ops[i], pool: tdb.Pool, alice: alice, bob: bob, position: g.newPosition(t, alice)})
		})
	}
}

func (e undoSnapEnv) archived(t *testing.T, id uuid.UUID) bool {
	t.Helper()
	return deletedAtOf(t, e.pool, e.g.table, id) != nil
}

// TestUndoDeleteSnapshot_RestoresExactlyOneRow: undoing a delete clears
// deleted_at on that one row and touches nothing else in the month or the
// position; a second undo of the now-live row is a not-found, not a no-op
// success.
//
// covers: INV-SOFT-DELETE-06
func TestUndoDeleteSnapshot_RestoresExactlyOneRow(t *testing.T) {
	forEachUndoSnapGroup(t, func(t *testing.T, e undoSnapEnv) {
		sep := e.g.create(t, e.alice, e.position, replSep, rd("100"))
		oct := e.g.create(t, e.alice, e.position, replOct, rd("200"))
		nov := e.g.create(t, e.alice, e.position, replNov, rd("300"))

		if err := e.ops.del(e.alice, oct); err != nil {
			t.Fatalf("delete: %v", err)
		}
		if err := e.ops.del(e.alice, nov); err != nil {
			t.Fatalf("delete: %v", err)
		}
		if err := e.ops.undo(e.alice, oct); err != nil {
			t.Fatalf("undo: %v", err)
		}

		if e.archived(t, oct) {
			t.Error("undone snapshot is still archived")
		}
		if !e.archived(t, nov) {
			t.Error("undo restored a snapshot it was not asked to")
		}
		if e.archived(t, sep) {
			t.Error("undo archived an unrelated live snapshot")
		}
		if got := rawCount(t, e.pool, e.g, e.position, "deleted_at IS NULL"); got != 2 {
			t.Errorf("live rows = %d, want 2 (sep, oct)", got)
		}
		if err := e.ops.undo(e.alice, oct); !errors.Is(err, repo.ErrNotFound) {
			t.Errorf("second undo of a live row: want ErrNotFound, got %v", err)
		}
	})
}

// TestUndoDeleteSnapshot_SlotTaken: if the month was re-recorded after the
// delete, the undo is refused with ErrUndoSlotTaken and nothing changes - the
// archived row stays archived and the new row stays the live one.
//
// covers: INV-SOFT-DELETE-07
func TestUndoDeleteSnapshot_SlotTaken(t *testing.T) {
	forEachUndoSnapGroup(t, func(t *testing.T, e undoSnapEnv) {
		old := e.g.create(t, e.alice, e.position, replSep, rd("100"))
		if err := e.ops.del(e.alice, old); err != nil {
			t.Fatalf("delete: %v", err)
		}
		fresh := e.g.create(t, e.alice, e.position, replSep, rd("999"))

		if err := e.ops.undo(e.alice, old); !errors.Is(err, repo.ErrUndoSlotTaken) {
			t.Fatalf("undo into a taken month: want ErrUndoSlotTaken, got %v", err)
		}
		requireOneLive(t, e.pool, e.g, e.position, replSep, &fresh, "999", 1)
		if !e.archived(t, old) {
			t.Error("a refused undo restored the row anyway")
		}
	})
}

// TestUndoDeleteSnapshot_Tenancy: another household's id is not found and
// leaves the row archived; the owner's own undo still works afterwards.
//
// covers: INV-SOFT-DELETE-08
func TestUndoDeleteSnapshot_Tenancy(t *testing.T) {
	forEachUndoSnapGroup(t, func(t *testing.T, e undoSnapEnv) {
		id := e.g.create(t, e.alice, e.position, replSep, rd("100"))
		if err := e.ops.del(e.alice, id); err != nil {
			t.Fatalf("delete: %v", err)
		}

		if err := e.ops.undo(e.bob, id); !errors.Is(err, repo.ErrNotFound) {
			t.Errorf("bob undo of alice's snapshot: want ErrNotFound, got %v", err)
		}
		if !e.archived(t, id) {
			t.Fatal("a cross-household undo restored alice's snapshot")
		}

		if err := e.ops.undo(e.alice, id); err != nil {
			t.Fatalf("alice undo: %v", err)
		}
		if e.archived(t, id) {
			t.Error("alice's undo did not restore her snapshot")
		}
	})
}

// TestUndoReplaceSnapshot_RestoresOriginal: replace then undo leaves the
// original the one live row for the month and the replacement archived, with
// nothing else in the position touched.
//
// covers: INV-SOFT-DELETE-09
func TestUndoReplaceSnapshot_RestoresOriginal(t *testing.T) {
	forEachUndoSnapGroup(t, func(t *testing.T, e undoSnapEnv) {
		other := e.g.create(t, e.alice, e.position, replOct, rd("50"))
		orig := e.g.create(t, e.alice, e.position, replSep, rd("100"))
		repl, replaced, err := e.g.edit(e.alice, orig, rd("250"), "IDR", nil)
		if err != nil || replaced == nil || *replaced != orig {
			t.Fatalf("edit: replacement=%v replaced=%v err=%v", repl, replaced, err)
		}

		if err := e.ops.undoReplace(e.alice, orig, repl); err != nil {
			t.Fatalf("undo replace: %v", err)
		}

		requireOneLive(t, e.pool, e.g, e.position, replSep, &orig, "100", 1)
		if !e.archived(t, repl) {
			t.Error("replacement is still live after the undo")
		}
		if e.archived(t, other) {
			t.Error("undo touched another month")
		}
		for _, r := range readMonth(t, e.pool, e.g, e.position, replSep) {
			if r.Supersedes != nil {
				t.Errorf("undo wrote supersedes on %s", r.ID)
			}
		}
	})
}

// TestUndoReplaceSnapshot_RefusedUnlessReplacementLive: the undo refuses with
// ErrUndoReplacementChanged, changing nothing, when the replacement is no
// longer the live row for the slot - replaced again, deleted, the same row as
// the original, or sitting in another month.
//
// covers: INV-SOFT-DELETE-10
func TestUndoReplaceSnapshot_RefusedUnlessReplacementLive(t *testing.T) {
	forEachUndoSnapGroup(t, func(t *testing.T, e undoSnapEnv) {
		wantRefused := func(t *testing.T, restore, replacement uuid.UUID) {
			t.Helper()
			if err := e.ops.undoReplace(e.alice, restore, replacement); !errors.Is(err, repo.ErrUndoReplacementChanged) {
				t.Fatalf("want ErrUndoReplacementChanged, got %v", err)
			}
		}

		t.Run("replacement edited again", func(t *testing.T) {
			a := e.g.create(t, e.alice, e.position, replSep, rd("100"))
			b, _, err := e.g.edit(e.alice, a, rd("200"), "IDR", nil)
			if err != nil {
				t.Fatalf("edit 1: %v", err)
			}
			c, _, err := e.g.edit(e.alice, b, rd("300"), "IDR", nil)
			if err != nil {
				t.Fatalf("edit 2: %v", err)
			}
			wantRefused(t, a, b)
			requireOneLive(t, e.pool, e.g, e.position, replSep, &c, "300", 2)
			if !e.archived(t, a) {
				t.Error("the original was restored by a refused undo")
			}
		})

		t.Run("replacement deleted", func(t *testing.T) {
			pos := e.g.newPosition(t, e.alice)
			a := e.g.create(t, e.alice, pos, replSep, rd("100"))
			b, _, err := e.g.edit(e.alice, a, rd("200"), "IDR", nil)
			if err != nil {
				t.Fatalf("edit: %v", err)
			}
			if err := e.ops.del(e.alice, b); err != nil {
				t.Fatalf("delete: %v", err)
			}
			wantRefused(t, a, b)
			if got := rawCount(t, e.pool, e.g, pos, "deleted_at IS NULL"); got != 0 {
				t.Errorf("live rows = %d, want 0: a refused undo wrote something", got)
			}
		})

		t.Run("same row twice", func(t *testing.T) {
			pos := e.g.newPosition(t, e.alice)
			a := e.g.create(t, e.alice, pos, replSep, rd("100"))
			wantRefused(t, a, a)
			requireOneLive(t, e.pool, e.g, pos, replSep, &a, "100", 0)
		})

		t.Run("replacement in another month", func(t *testing.T) {
			pos := e.g.newPosition(t, e.alice)
			a := e.g.create(t, e.alice, pos, replSep, rd("100"))
			oct := e.g.create(t, e.alice, pos, replOct, rd("200"))
			if err := e.ops.del(e.alice, a); err != nil {
				t.Fatalf("delete: %v", err)
			}
			wantRefused(t, a, oct)
			if !e.archived(t, a) || e.archived(t, oct) {
				t.Error("a refused undo changed row state")
			}
		})

		t.Run("replacement on another position", func(t *testing.T) {
			pos := e.g.newPosition(t, e.alice)
			pos2 := e.g.newPosition(t, e.alice)
			a := e.g.create(t, e.alice, pos, replSep, rd("100"))
			x := e.g.create(t, e.alice, pos2, replSep, rd("100"))
			if err := e.ops.del(e.alice, a); err != nil {
				t.Fatalf("delete: %v", err)
			}
			wantRefused(t, a, x)
			if !e.archived(t, a) || e.archived(t, x) {
				t.Error("a refused undo changed row state")
			}
		})
	})
}

// TestUndoReplaceSnapshot_Tenancy: either id belonging to another household is
// not found, and nothing changes for either household.
//
// covers: INV-SOFT-DELETE-08
func TestUndoReplaceSnapshot_Tenancy(t *testing.T) {
	forEachUndoSnapGroup(t, func(t *testing.T, e undoSnapEnv) {
		orig := e.g.create(t, e.alice, e.position, replSep, rd("100"))
		repl, _, err := e.g.edit(e.alice, orig, rd("250"), "IDR", nil)
		if err != nil {
			t.Fatalf("edit: %v", err)
		}

		if err := e.ops.undoReplace(e.bob, orig, repl); !errors.Is(err, repo.ErrNotFound) {
			t.Errorf("bob undo-replace of alice's pair: want ErrNotFound, got %v", err)
		}

		// Bob's own live snapshot paired with alice's archived original: the
		// original is not his, so it is not found rather than a conflict.
		bobPos := e.g.newPosition(t, e.bob)
		bobSnap := e.g.create(t, e.bob, bobPos, replSep, rd("1"))
		if err := e.ops.undoReplace(e.bob, orig, bobSnap); !errors.Is(err, repo.ErrNotFound) {
			t.Errorf("bob pairing alice's original with his own row: want ErrNotFound, got %v", err)
		}
		if err := e.ops.undoReplace(e.alice, bobSnap, repl); !errors.Is(err, repo.ErrNotFound) {
			t.Errorf("alice naming bob's row as the original: want ErrNotFound, got %v", err)
		}

		requireOneLive(t, e.pool, e.g, e.position, replSep, &repl, "250", 1)
		requireOneLive(t, e.pool, e.g, bobPos, replSep, &bobSnap, "1", 0)

		if err := e.ops.undoReplace(e.alice, orig, repl); err != nil {
			t.Fatalf("alice's own undo-replace: %v", err)
		}
		requireOneLive(t, e.pool, e.g, e.position, replSep, &orig, "100", 1)
	})
}

// TestUndo_RestoresReportEffect: the delete and the undo both move the report
// watermark, so the month's report is regenerated on the next read and the
// undone value is back in net worth - for a snapshot delete and for a replace.
//
// covers: INV-SOFT-DELETE-11
func TestUndo_RestoresReportEffect(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)
	user := testutil.CreateHouseholdWithUser(t, q, "Alice")
	ctx := identity.WithUser(context.Background(), user)
	ar := repo.NewAssetRepo(tdb.Pool)
	mr := repo.NewMonthlyReportRepo(tdb.Pool)

	nw := func(t *testing.T) decimal.Decimal {
		t.Helper()
		rep, err := mr.GetReport(ctx, replSep)
		if err != nil {
			t.Fatalf("GetReport: %v", err)
		}
		return rep.NwTotal
	}
	want := func(t *testing.T, label, amount string) {
		t.Helper()
		if got := nw(t); !got.Equal(rd(amount)) {
			t.Fatalf("%s: nw_total = %s, want %s", label, got, amount)
		}
	}

	keep := newBankAccount(ctx, t, ar, "Keep")
	gone := newBankAccount(ctx, t, ar, "Gone")
	if _, err := ar.CreateAssetSnapshot(ctx, repo.CreateAssetSnapshotParams{AssetID: keep.Asset.ID, YearMonth: replSep, Amount: rd("100"), Currency: "IDR"}); err != nil {
		t.Fatalf("create: %v", err)
	}
	snap, err := ar.CreateAssetSnapshot(ctx, repo.CreateAssetSnapshotParams{AssetID: gone.Asset.ID, YearMonth: replSep, Amount: rd("100"), Currency: "IDR"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	want(t, "before", "200")

	t.Run("delete then undo", func(t *testing.T) {
		if err := ar.DeleteAssetSnapshot(ctx, snap.ID); err != nil {
			t.Fatalf("delete: %v", err)
		}
		want(t, "after delete", "100")
		if err := ar.UndoDeleteAssetSnapshot(ctx, snap.ID); err != nil {
			t.Fatalf("undo: %v", err)
		}
		want(t, "after undo", "200")
	})

	t.Run("replace then undo", func(t *testing.T) {
		w, err := ar.UpdateAssetSnapshot(ctx, repo.UpdateAssetSnapshotParams{SnapshotID: snap.ID, Amount: rd("500"), Currency: "IDR"})
		if err != nil || w.ReplacedID == nil {
			t.Fatalf("edit: %+v %v", w, err)
		}
		want(t, "after replace", "600")
		if err := ar.UndoReplaceAssetSnapshot(ctx, *w.ReplacedID, w.ID); err != nil {
			t.Fatalf("undo replace: %v", err)
		}
		want(t, "after undo replace", "200")
	})
}

// TestUndoDeleteSnapshot_NotUnderDeletedPosition: a snapshot whose position was
// deleted afterwards is not restored on its own - that would leave a live child
// under a tombstoned parent.
//
// covers: INV-SOFT-DELETE-08
func TestUndoDeleteSnapshot_NotUnderDeletedPosition(t *testing.T) {
	parentOf := map[string]string{
		"asset_snapshots": "assets", "liability_snapshots": "liabilities",
		"receivable_snapshots": "receivables", "investment_snapshots": "investments",
	}
	forEachUndoSnapGroup(t, func(t *testing.T, e undoSnapEnv) {
		id := e.g.create(t, e.alice, e.position, replSep, rd("100"))
		if err := e.ops.del(e.alice, id); err != nil {
			t.Fatalf("delete: %v", err)
		}
		if _, err := e.pool.Exec(context.Background(), "UPDATE "+parentOf[e.g.table]+" SET deleted_at = now() WHERE id = $1", e.position); err != nil {
			t.Fatalf("delete position: %v", err)
		}

		if err := e.ops.undo(e.alice, id); !errors.Is(err, repo.ErrNotFound) {
			t.Fatalf("undo under a deleted position: want ErrNotFound, got %v", err)
		}
		if !e.archived(t, id) {
			t.Error("a refused undo restored the snapshot")
		}
	})
}
