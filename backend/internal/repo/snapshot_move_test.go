package repo_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kerti/balances-v2/backend/internal/db"
	"github.com/kerti/balances-v2/backend/internal/identity"
	"github.com/kerti/balances-v2/backend/internal/repo"
	"github.com/kerti/balances-v2/backend/internal/testutil"
)

// Moving a snapshot to another month (ADR-0046 amendment, ADR-0058). One atomic
// act, across all four groups and all three shapes: the original is archived,
// whatever held the target month is archived the same way, and the same values
// are inserted in the target month. Undo reverses all three rows.

// moveOps is one snapshot group's move and undo-move calls, indexed in step with
// replaceGroups so the shared fixtures (position, create, edit) are reused.
type moveOps struct {
	move      func(ctx context.Context, id uuid.UUID, target time.Time) (live uuid.UUID, displaced *uuid.UUID, err error)
	undoMove  func(ctx context.Context, restoreID, movedID uuid.UUID, displacedID *uuid.UUID) error
	terminate func(ctx context.Context, pos uuid.UUID, at time.Time) error
}

func moveOpsFor(pool *pgxpool.Pool) []moveOps {
	ar := repo.NewAssetRepo(pool)
	lr := repo.NewLiabilityRepo(pool)
	rr := repo.NewReceivableRepo(pool)
	ir := repo.NewInvestmentRepo(pool)
	note := "stopped tracking"
	// "untracked" is a terminal status of every group and needs no settlement
	// (INV-LIFECYCLE-09), so one lifecycle call writes a close snapshot anywhere.
	lc := func(at time.Time) repo.LifecycleParams {
		return repo.LifecycleParams{Status: "untracked", TerminatedAt: &at, TerminationNote: &note}
	}
	inv := moveOps{
		move: func(ctx context.Context, id uuid.UUID, target time.Time) (uuid.UUID, *uuid.UUID, error) {
			w, err := ir.MoveInvestmentSnapshot(ctx, id, target)
			if err != nil {
				return uuid.Nil, nil, err
			}
			return w.ID, w.ReplacedID, nil
		},
		undoMove: ir.UndoMoveInvestmentSnapshot,
		terminate: func(ctx context.Context, pos uuid.UUID, at time.Time) error {
			_, err := ir.UpdateInvestmentLifecycle(ctx, pos, lc(at), nil)
			return err
		},
	}
	return []moveOps{
		{
			move: func(ctx context.Context, id uuid.UUID, target time.Time) (uuid.UUID, *uuid.UUID, error) {
				w, err := ar.MoveAssetSnapshot(ctx, id, target)
				if err != nil {
					return uuid.Nil, nil, err
				}
				return w.ID, w.ReplacedID, nil
			},
			undoMove: ar.UndoMoveAssetSnapshot,
			terminate: func(ctx context.Context, pos uuid.UUID, at time.Time) error {
				_, err := ar.UpdateAssetLifecycle(ctx, pos, lc(at))
				return err
			},
		},
		{
			move: func(ctx context.Context, id uuid.UUID, target time.Time) (uuid.UUID, *uuid.UUID, error) {
				w, err := lr.MoveLiabilitySnapshot(ctx, id, target)
				if err != nil {
					return uuid.Nil, nil, err
				}
				return w.ID, w.ReplacedID, nil
			},
			undoMove: lr.UndoMoveLiabilitySnapshot,
			terminate: func(ctx context.Context, pos uuid.UUID, at time.Time) error {
				_, err := lr.UpdateLiabilityLifecycle(ctx, pos, lc(at))
				return err
			},
		},
		{
			move: func(ctx context.Context, id uuid.UUID, target time.Time) (uuid.UUID, *uuid.UUID, error) {
				w, err := rr.MoveReceivableSnapshot(ctx, id, target)
				if err != nil {
					return uuid.Nil, nil, err
				}
				return w.ID, w.ReplacedID, nil
			},
			undoMove: rr.UndoMoveReceivableSnapshot,
			terminate: func(ctx context.Context, pos uuid.UUID, at time.Time) error {
				_, err := rr.UpdateReceivableLifecycle(ctx, pos, lc(at))
				return err
			},
		},
		inv, inv,
	}
}

type moveEnv struct {
	g     replaceGroup
	ops   moveOps
	pool  *pgxpool.Pool
	alice context.Context
	bob   context.Context
}

// forEachMoveGroup runs fn once per snapshot group with a fresh database, an
// alice-owned position and a bob in another household.
func forEachMoveGroup(t *testing.T, fn func(t *testing.T, e moveEnv)) {
	t.Helper()
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)
	alice := identity.WithUser(context.Background(), testutil.CreateHouseholdWithUser(t, q, "Alice"))
	bob := identity.WithUser(context.Background(), testutil.CreateHouseholdWithUser(t, q, "Bob"))
	ops := moveOpsFor(tdb.Pool)
	for i, g := range replaceGroups(tdb.Pool) {
		t.Run(g.name, func(t *testing.T) {
			fn(t, moveEnv{g: g, ops: ops[i], pool: tdb.Pool, alice: alice, bob: bob})
		})
	}
}

func (e moveEnv) exec(t *testing.T, sql string, args ...any) {
	t.Helper()
	if _, err := e.pool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("exec %q: %v", sql, err)
	}
}

// valueColumns reads every column that carries a snapshot's value (amount,
// currency, description, and quantity / price / accrued where the shape has
// them), minus identity, month, statement date and audit stamps, so two rows can
// be compared as "the same values".
func (e moveEnv) valueColumns(t *testing.T, id uuid.UUID) map[string]any {
	t.Helper()
	var out map[string]any
	err := e.pool.QueryRow(context.Background(),
		"SELECT to_jsonb(t) - 'id' - '"+e.g.fkCol+"' - 'year_month' - 'as_of_date' - 'created_at' - 'updated_at' - 'deleted_at' - 'created_by' - 'updated_by' - 'supersedes' FROM "+e.g.table+" t WHERE id = $1", id).Scan(&out)
	if err != nil {
		t.Fatalf("read value columns of %s: %v", id, err)
	}
	return out
}

func (e moveEnv) asOf(t *testing.T, id uuid.UUID) *time.Time {
	t.Helper()
	var d *time.Time
	if err := e.pool.QueryRow(context.Background(), "SELECT as_of_date FROM "+e.g.table+" WHERE id = $1", id).Scan(&d); err != nil {
		t.Fatalf("read as_of_date: %v", err)
	}
	return d
}

// covers: INV-SNAPSHOTS-15, INV-SNAPSHOTS-13, INV-SOFT-DELETE-08
func TestSnapshotMove_AllGroups(t *testing.T) {
	forEachMoveGroup(t, func(t *testing.T, e moveEnv) {
		g := e.g

		t.Run("moving to an empty month leaves one live row there, none behind, the original archived", func(t *testing.T) {
			p := g.newPosition(t, e.alice)
			orig := g.create(t, e.alice, p, replOct, rd("100"))
			untouched := g.create(t, e.alice, p, replNov, rd("7"))

			live, displaced, err := e.ops.move(e.alice, orig, replSep)
			if err != nil {
				t.Fatalf("move: %v", err)
			}
			if live == orig || displaced != nil {
				t.Fatalf("live=%s displaced=%v, want a new row and nothing displaced", live, displaced)
			}
			requireOneLive(t, e.pool, g, p, replSep, &live, "100", 0)
			// Oct: nothing live, the original archived.
			rows := readMonth(t, e.pool, g, p, replOct)
			if len(rows) != 1 || !rows[0].Archived || rows[0].ID != orig || !rows[0].Amount.Equal(rd("100")) {
				t.Fatalf("Oct rows = %+v, want only the archived original", rows)
			}
			if n := rawCount(t, e.pool, g, p, "year_month = '2026-10-01' AND deleted_at IS NULL"); n != 0 {
				t.Fatalf("%d live Oct rows remain, want 0", n)
			}
			requireOneLive(t, e.pool, g, p, replNov, &untouched, "7", 0)
		})

		t.Run("every value carries over; zero, negative and a second currency included", func(t *testing.T) {
			for _, c := range []struct{ amt, cur string }{{"0", "IDR"}, {"-5.5", "IDR"}, {"-5.5", "USD"}, {"1234567.8912", "IDR"}} {
				p := g.newPosition(t, e.alice)
				orig := g.create(t, e.alice, p, replOct, rd("100"))
				orig, _, err := g.edit(e.alice, orig, rd(c.amt), c.cur, nil)
				if err != nil {
					t.Fatalf("edit to %s %s: %v", c.amt, c.cur, err)
				}
				e.exec(t, "UPDATE "+g.table+" SET description = 'rent' WHERE id = $1", orig)

				live, _, err := e.ops.move(e.alice, orig, replSep)
				if err != nil {
					t.Fatalf("move %s %s: %v", c.amt, c.cur, err)
				}
				if got, want := e.valueColumns(t, live), e.valueColumns(t, orig); !reflect.DeepEqual(got, want) {
					t.Fatalf("%s %s: moved values %v, want %v", c.amt, c.cur, got, want)
				}
				requireOneLive(t, e.pool, g, p, replSep, &live, c.amt, 0)
			}
		})

		t.Run("a statement date is dropped to NULL, the month's CHECK would reject it", func(t *testing.T) {
			p := g.newPosition(t, e.alice)
			orig := g.create(t, e.alice, p, replOct, rd("100"))
			orig, _, err := g.edit(e.alice, orig, rd("100"), "IDR", dayOf(2026, time.October, 31))
			if err != nil {
				t.Fatalf("edit: %v", err)
			}
			if e.asOf(t, orig) == nil {
				t.Fatalf("fixture lost its statement date")
			}
			live, _, err := e.ops.move(e.alice, orig, replNov)
			if err != nil {
				t.Fatalf("move: %v", err)
			}
			if d := e.asOf(t, live); d != nil {
				t.Fatalf("moved as_of_date = %v, want NULL", d)
			}
		})

		t.Run("across a year boundary", func(t *testing.T) {
			p := g.newPosition(t, e.alice)
			orig := g.create(t, e.alice, p, time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC), rd("40"))
			dec := time.Date(2025, time.December, 1, 0, 0, 0, 0, time.UTC)
			live, _, err := e.ops.move(e.alice, orig, dec)
			if err != nil {
				t.Fatalf("move: %v", err)
			}
			requireOneLive(t, e.pool, g, p, dec, &live, "40", 0)
			if n := rawCount(t, e.pool, g, p, "year_month = '2026-01-01' AND deleted_at IS NULL"); n != 0 {
				t.Fatalf("%d live Jan rows remain, want 0", n)
			}
		})

		t.Run("onto a taken month replaces it and archives the displaced value", func(t *testing.T) {
			p := g.newPosition(t, e.alice)
			taken := g.create(t, e.alice, p, replSep, rd("50"))
			orig := g.create(t, e.alice, p, replOct, rd("100"))

			live, displaced, err := e.ops.move(e.alice, orig, replSep)
			if err != nil {
				t.Fatalf("move: %v", err)
			}
			if displaced == nil || *displaced != taken {
				t.Fatalf("replaced_id = %v, want %s", displaced, taken)
			}
			requireOneLive(t, e.pool, g, p, replSep, &live, "100", 1)
			for _, r := range readMonth(t, e.pool, g, p, replSep) {
				if r.ID == taken && (!r.Archived || !r.Amount.Equal(rd("50"))) {
					t.Errorf("displaced row = %+v, want archived with amount 50", r)
				}
			}
		})

		t.Run("onto a month holding identical values still archives it and names it", func(t *testing.T) {
			p := g.newPosition(t, e.alice)
			taken := g.create(t, e.alice, p, replSep, rd("100"))
			orig := g.create(t, e.alice, p, replOct, rd("100"))

			live, displaced, err := e.ops.move(e.alice, orig, replSep)
			if err != nil {
				t.Fatalf("move: %v", err)
			}
			if live == taken || displaced == nil || *displaced != taken {
				t.Fatalf("live=%s displaced=%v: an Edit's unchanged shortcut must not apply to a move", live, displaced)
			}
			requireOneLive(t, e.pool, g, p, replSep, &live, "100", 1)
		})

		t.Run("the same month is refused and nothing changes", func(t *testing.T) {
			p := g.newPosition(t, e.alice)
			orig := g.create(t, e.alice, p, replOct, rd("100"))
			if _, _, err := e.ops.move(e.alice, orig, replOct); !errors.Is(err, repo.ErrSnapshotMoveSameMonth) {
				t.Fatalf("err = %v, want ErrSnapshotMoveSameMonth", err)
			}
			requireOneLive(t, e.pool, g, p, replOct, &orig, "100", 0)
		})

		t.Run("a failure midway leaves nothing changed", func(t *testing.T) {
			p := g.newPosition(t, e.alice)
			taken := g.create(t, e.alice, p, replSep, rd("50"))
			orig := g.create(t, e.alice, p, replOct, rd("100"))
			// The insert is the last write of the move, after both archives. A
			// trigger that rejects a row carrying this description fails it there.
			e.exec(t, `CREATE OR REPLACE FUNCTION move_boom() RETURNS trigger AS $$
				BEGIN RAISE EXCEPTION 'forced failure'; END $$ LANGUAGE plpgsql`)
			e.exec(t, "CREATE TRIGGER move_boom BEFORE INSERT ON "+g.table+
				" FOR EACH ROW WHEN (NEW.description = 'boom') EXECUTE FUNCTION move_boom()")
			t.Cleanup(func() { e.exec(t, "DROP TRIGGER IF EXISTS move_boom ON "+g.table) })
			e.exec(t, "UPDATE "+g.table+" SET description = 'boom' WHERE id = $1", orig)

			if _, _, err := e.ops.move(e.alice, orig, replSep); err == nil {
				t.Fatalf("move succeeded despite the forced failure")
			}
			// Both archives ran inside the transaction before the failing insert;
			// neither may have survived.
			requireOneLive(t, e.pool, g, p, replOct, &orig, "100", 0)
			requireOneLive(t, e.pool, g, p, replSep, &taken, "50", 0)
		})

		t.Run("another household cannot move it", func(t *testing.T) {
			p := g.newPosition(t, e.alice)
			orig := g.create(t, e.alice, p, replOct, rd("100"))
			if _, _, err := e.ops.move(e.bob, orig, replSep); !errors.Is(err, repo.ErrNotFound) {
				t.Fatalf("bob move err = %v, want ErrNotFound", err)
			}
			requireOneLive(t, e.pool, g, p, replOct, &orig, "100", 0)
			if n := rawCount(t, e.pool, g, p, "year_month = '2026-09-01'"); n != 0 {
				t.Fatalf("%d Sep rows appeared, want 0", n)
			}
		})

		t.Run("a soft-deleted snapshot cannot be moved", func(t *testing.T) {
			p := g.newPosition(t, e.alice)
			orig := g.create(t, e.alice, p, replOct, rd("100"))
			e.exec(t, "UPDATE "+g.table+" SET deleted_at = now() WHERE id = $1", orig)
			if _, _, err := e.ops.move(e.alice, orig, replSep); !errors.Is(err, repo.ErrNotFound) {
				t.Fatalf("err = %v, want ErrNotFound", err)
			}
			if n := rawCount(t, e.pool, g, p, "deleted_at IS NULL"); n != 0 {
				t.Fatalf("%d live rows appeared, want 0", n)
			}
		})

		t.Run("a close snapshot cannot be moved, whether or not it displaced anything", func(t *testing.T) {
			// One that displaced the month's own reading: it names it in supersedes.
			p := g.newPosition(t, e.alice)
			sep := g.create(t, e.alice, p, replSep, rd("25"))
			g.create(t, e.alice, p, replOct, rd("50"))
			if err := e.ops.terminate(e.alice, p, time.Date(2026, time.October, 15, 0, 0, 0, 0, time.UTC)); err != nil {
				t.Fatalf("terminate: %v", err)
			}
			closeRow := liveRow(t, e, p, replOct)
			if closeRow.Supersedes == nil {
				t.Fatalf("fixture: the close row should name the reading it displaced")
			}
			if _, _, err := e.ops.move(e.alice, closeRow.ID, replNov); !errors.Is(err, repo.ErrSnapshotCloseNotMovable) {
				t.Fatalf("err = %v, want ErrSnapshotCloseNotMovable", err)
			}
			if got := liveRow(t, e, p, replOct); got.ID != closeRow.ID {
				t.Fatalf("the close row changed: %+v", got)
			}
			// The Position's ordinary readings still move.
			if _, _, err := e.ops.move(e.alice, sep, time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC)); err != nil {
				t.Fatalf("an ordinary reading of a terminated position should move: %v", err)
			}

			// One that displaced nothing: recognised as a zero at the termination month.
			p2 := g.newPosition(t, e.alice)
			g.create(t, e.alice, p2, replSep, rd("25"))
			if err := e.ops.terminate(e.alice, p2, time.Date(2026, time.October, 15, 0, 0, 0, 0, time.UTC)); err != nil {
				t.Fatalf("terminate: %v", err)
			}
			bare := liveRow(t, e, p2, replOct)
			if bare.Supersedes != nil || !bare.Amount.IsZero() {
				t.Fatalf("fixture: want a bare 0 close row, got %+v", bare)
			}
			if _, _, err := e.ops.move(e.alice, bare.ID, replNov); !errors.Is(err, repo.ErrSnapshotCloseNotMovable) {
				t.Fatalf("bare close row: err = %v, want ErrSnapshotCloseNotMovable", err)
			}
		})

		t.Run("a row that names what it displaced is a close row even if the position's dates moved on", func(t *testing.T) {
			p := g.newPosition(t, e.alice)
			older := g.create(t, e.alice, p, replSep, rd("25"))
			orig := g.create(t, e.alice, p, replOct, rd("50"))
			e.exec(t, "UPDATE "+g.table+" SET supersedes = $2 WHERE id = $1", orig, older)
			if _, _, err := e.ops.move(e.alice, orig, replNov); !errors.Is(err, repo.ErrSnapshotCloseNotMovable) {
				t.Fatalf("err = %v, want ErrSnapshotCloseNotMovable", err)
			}
		})

		t.Run("a user-recorded zero is an ordinary reading and moves", func(t *testing.T) {
			p := g.newPosition(t, e.alice)
			orig := g.create(t, e.alice, p, replOct, rd("0"))
			live, _, err := e.ops.move(e.alice, orig, replSep)
			if err != nil {
				t.Fatalf("move: %v", err)
			}
			requireOneLive(t, e.pool, g, p, replSep, &live, "0", 0)
		})
	})
}

// liveRow returns the single live row of a month.
func liveRow(t *testing.T, e moveEnv, pos uuid.UUID, month time.Time) snapRow {
	t.Helper()
	var live []snapRow
	for _, r := range readMonth(t, e.pool, e.g, pos, month) {
		if !r.Archived {
			live = append(live, r)
		}
	}
	if len(live) != 1 {
		t.Fatalf("%s: %d live rows, want 1", month.Format("2006-01"), len(live))
	}
	return live[0]
}

// A time deposit's snapshots are confined to its term (issue #62); a move is a
// snapshot write, so the target month must lie inside it.
//
// covers: INV-SNAPSHOTS-15
func TestSnapshotMove_TimeDepositTerm(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)
	alice := identity.WithUser(context.Background(), testutil.CreateHouseholdWithUser(t, q, "Alice"))
	ir := repo.NewInvestmentRepo(tdb.Pool)

	td, err := ir.CreateTimeDeposit(alice, repo.CreateTimeDepositParams{
		DisplayName: "TD", OwnershipType: "joint", NativeCurrency: "IDR", RiskProfile: "low",
		BankName: "BCA", Principal: rd("100"), InterestRate: rd("5"), TermMonths: 4,
		PlacementDate:  time.Date(2026, time.August, 10, 0, 0, 0, 0, time.UTC),
		MaturityDate:   time.Date(2026, time.December, 10, 0, 0, 0, 0, time.UTC),
		RolloverPolicy: "no_rollover",
	})
	if err != nil {
		t.Fatalf("CreateTimeDeposit: %v", err)
	}
	acc := rd("1")
	snap, err := ir.CreateInvestmentSnapshot(alice, repo.CreateInvestmentSnapshotParams{
		InvestmentID: td.Investment.ID, YearMonth: replOct, Amount: rd("100"), Currency: "IDR", AccruedInterest: &acc,
	})
	if err != nil {
		t.Fatalf("CreateInvestmentSnapshot: %v", err)
	}

	before := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	if _, err := ir.MoveInvestmentSnapshot(alice, snap.ID, before); !errors.Is(err, repo.ErrOutsideDepositTerm) {
		t.Fatalf("move before placement: err = %v, want ErrOutsideDepositTerm", err)
	}
	var live int
	if err := tdb.Pool.QueryRow(context.Background(),
		"SELECT count(*) FROM investment_snapshots WHERE id = $1 AND deleted_at IS NULL", snap.ID).Scan(&live); err != nil || live != 1 {
		t.Fatalf("original live count = %d (%v), want 1: a refused move changes nothing", live, err)
	}
	// The placement month itself is inside the term.
	if _, err := ir.MoveInvestmentSnapshot(alice, snap.ID, time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("move to the placement month: %v", err)
	}
}

// covers: INV-SOFT-DELETE-13, INV-SOFT-DELETE-07, INV-SOFT-DELETE-08
func TestUndoMove_AllGroups(t *testing.T) {
	forEachMoveGroup(t, func(t *testing.T, e moveEnv) {
		g := e.g

		t.Run("restores the original in its month and archives the moved row", func(t *testing.T) {
			p := g.newPosition(t, e.alice)
			orig := g.create(t, e.alice, p, replOct, rd("100"))
			other := g.create(t, e.alice, p, replNov, rd("7"))
			live, displaced, err := e.ops.move(e.alice, orig, replSep)
			if err != nil || displaced != nil {
				t.Fatalf("move: live=%s displaced=%v err=%v", live, displaced, err)
			}

			if err := e.ops.undoMove(e.alice, orig, live, displaced); err != nil {
				t.Fatalf("undo move: %v", err)
			}
			requireOneLive(t, e.pool, g, p, replOct, &orig, "100", 0)
			if n := rawCount(t, e.pool, g, p, "year_month = '2026-09-01' AND deleted_at IS NULL"); n != 0 {
				t.Fatalf("%d live Sep rows remain after undo, want 0", n)
			}
			if sep := readMonth(t, e.pool, g, p, replSep); len(sep) != 1 || sep[0].ID != live || !sep[0].Archived {
				t.Fatalf("Sep rows = %+v, want only the archived moved row", sep)
			}
			requireOneLive(t, e.pool, g, p, replNov, &other, "7", 0)
		})

		t.Run("restores a displaced target value too", func(t *testing.T) {
			p := g.newPosition(t, e.alice)
			taken := g.create(t, e.alice, p, replSep, rd("50"))
			orig := g.create(t, e.alice, p, replOct, rd("100"))
			live, displaced, err := e.ops.move(e.alice, orig, replSep)
			if err != nil || displaced == nil || *displaced != taken {
				t.Fatalf("move: live=%s displaced=%v err=%v", live, displaced, err)
			}

			if err := e.ops.undoMove(e.alice, orig, live, displaced); err != nil {
				t.Fatalf("undo move: %v", err)
			}
			requireOneLive(t, e.pool, g, p, replOct, &orig, "100", 0)
			// Sep holds the displaced 50 again, with the moved row archived beside it.
			requireOneLive(t, e.pool, g, p, replSep, &taken, "50", 1)
		})

		t.Run("refuses when the moved row was changed since", func(t *testing.T) {
			p := g.newPosition(t, e.alice)
			taken := g.create(t, e.alice, p, replSep, rd("50"))
			orig := g.create(t, e.alice, p, replOct, rd("100"))
			live, displaced, err := e.ops.move(e.alice, orig, replSep)
			if err != nil {
				t.Fatalf("move: %v", err)
			}
			edited, _, err := g.edit(e.alice, live, rd("999"), "IDR", nil)
			if err != nil {
				t.Fatalf("edit after move: %v", err)
			}

			if err := e.ops.undoMove(e.alice, orig, live, displaced); !errors.Is(err, repo.ErrUndoReplacementChanged) {
				t.Fatalf("err = %v, want ErrUndoReplacementChanged", err)
			}
			// Nothing was touched: the later edit is still the live Sep value, the
			// displaced row and the original are still archived.
			requireOneLive(t, e.pool, g, p, replSep, &edited, "999", 2)
			if n := rawCount(t, e.pool, g, p, "deleted_at IS NULL AND id IN ('"+orig.String()+"','"+taken.String()+"')"); n != 0 {
				t.Fatalf("%d of the original/displaced rows were restored by a refused undo", n)
			}
		})

		t.Run("refuses when the original's month was filled since", func(t *testing.T) {
			p := g.newPosition(t, e.alice)
			taken := g.create(t, e.alice, p, replSep, rd("50"))
			orig := g.create(t, e.alice, p, replOct, rd("100"))
			live, displaced, err := e.ops.move(e.alice, orig, replSep)
			if err != nil {
				t.Fatalf("move: %v", err)
			}
			refill := g.create(t, e.alice, p, replOct, rd("333"))

			if err := e.ops.undoMove(e.alice, orig, live, displaced); !errors.Is(err, repo.ErrUndoSlotTaken) {
				t.Fatalf("err = %v, want ErrUndoSlotTaken", err)
			}
			// All or nothing: the moved row is still live (the archive was rolled
			// back with the failed restore) and the displaced value is still archived.
			requireOneLive(t, e.pool, g, p, replSep, &live, "100", 1)
			requireOneLive(t, e.pool, g, p, replOct, &refill, "333", 1)
			if d := deletedAtOf(t, e.pool, g.table, taken); d == nil {
				t.Fatalf("displaced row was restored by a refused undo")
			}
		})

		t.Run("refuses rows that do not form a move", func(t *testing.T) {
			p := g.newPosition(t, e.alice)
			sepRow := g.create(t, e.alice, p, replSep, rd("50"))
			orig := g.create(t, e.alice, p, replOct, rd("100"))
			live, displaced, err := e.ops.move(e.alice, orig, replSep)
			if err != nil || displaced == nil || *displaced != sepRow {
				t.Fatalf("move: live=%s displaced=%v err=%v", live, displaced, err)
			}
			nov := g.create(t, e.alice, p, replNov, rd("7"))
			e.exec(t, "UPDATE "+g.table+" SET deleted_at = now() WHERE id = $1", nov)

			for name, c := range map[string]struct {
				restore, moved uuid.UUID
				displaced      *uuid.UUID
			}{
				"the same row twice":                         {live, live, nil},
				"an original from the moved row's own month": {sepRow, live, nil},
				"a displaced row from another month":         {orig, live, &nov},
				"the original named as displaced":            {orig, live, &orig},
				"the moved row named as displaced":           {orig, live, &live},
			} {
				if err := e.ops.undoMove(e.alice, c.restore, c.moved, c.displaced); !errors.Is(err, repo.ErrUndoReplacementChanged) {
					t.Errorf("%s: err = %v, want ErrUndoReplacementChanged", name, err)
				}
			}
			// Every refusal changed nothing: the move still stands.
			requireOneLive(t, e.pool, g, p, replSep, &live, "100", 1)
			if d := deletedAtOf(t, e.pool, g.table, orig); d == nil {
				t.Fatalf("original was restored by a refused undo")
			}
			if d := deletedAtOf(t, e.pool, g.table, sepRow); d == nil {
				t.Fatalf("displaced row was restored by a refused undo")
			}
		})

		t.Run("refuses older archived rows of the same slots, which no write of this move archived", func(t *testing.T) {
			p := g.newPosition(t, e.alice)
			// Sep's history: s1 (replaced by s2). Oct's: o0 (replaced by o1).
			s1 := g.create(t, e.alice, p, replSep, rd("50"))
			s2, _, err := g.edit(e.alice, s1, rd("60"), "IDR", nil)
			if err != nil {
				t.Fatalf("edit Sep: %v", err)
			}
			o0 := g.create(t, e.alice, p, replOct, rd("10"))
			o1, _, err := g.edit(e.alice, o0, rd("100"), "IDR", nil)
			if err != nil {
				t.Fatalf("edit Oct: %v", err)
			}
			live, displaced, err := e.ops.move(e.alice, o1, replSep)
			if err != nil || displaced == nil || *displaced != s2 {
				t.Fatalf("move: live=%s displaced=%v err=%v", live, displaced, err)
			}

			if err := e.ops.undoMove(e.alice, o0, live, displaced); !errors.Is(err, repo.ErrUndoReplacementChanged) {
				t.Fatalf("older Oct row as the original: err = %v, want ErrUndoReplacementChanged", err)
			}
			if err := e.ops.undoMove(e.alice, o1, live, &s1); !errors.Is(err, repo.ErrUndoReplacementChanged) {
				t.Fatalf("older Sep row as the displaced one: err = %v, want ErrUndoReplacementChanged", err)
			}
			// Nothing changed: the move still stands and no old row came back.
			requireOneLive(t, e.pool, g, p, replSep, &live, "100", 2)
			for _, id := range []uuid.UUID{o0, o1, s1, s2} {
				if deletedAtOf(t, e.pool, g.table, id) == nil {
					t.Fatalf("row %s was restored by a refused undo", id)
				}
			}

			// The genuine trio still undoes.
			if err := e.ops.undoMove(e.alice, o1, live, displaced); err != nil {
				t.Fatalf("genuine undo: %v", err)
			}
			requireOneLive(t, e.pool, g, p, replOct, &o1, "100", 1)
			requireOneLive(t, e.pool, g, p, replSep, &s2, "60", 2)
		})

		t.Run("a second undo is refused", func(t *testing.T) {
			p := g.newPosition(t, e.alice)
			orig := g.create(t, e.alice, p, replOct, rd("100"))
			live, displaced, err := e.ops.move(e.alice, orig, replSep)
			if err != nil {
				t.Fatalf("move: %v", err)
			}
			if err := e.ops.undoMove(e.alice, orig, live, displaced); err != nil {
				t.Fatalf("undo: %v", err)
			}
			if err := e.ops.undoMove(e.alice, orig, live, displaced); !errors.Is(err, repo.ErrUndoReplacementChanged) {
				t.Fatalf("second undo err = %v, want ErrUndoReplacementChanged", err)
			}
			requireOneLive(t, e.pool, g, p, replOct, &orig, "100", 0)
		})

		t.Run("another household cannot undo it", func(t *testing.T) {
			p := g.newPosition(t, e.alice)
			taken := g.create(t, e.alice, p, replSep, rd("50"))
			orig := g.create(t, e.alice, p, replOct, rd("100"))
			live, displaced, err := e.ops.move(e.alice, orig, replSep)
			if err != nil {
				t.Fatalf("move: %v", err)
			}
			if err := e.ops.undoMove(e.bob, orig, live, displaced); !errors.Is(err, repo.ErrNotFound) {
				t.Fatalf("bob undo err = %v, want ErrNotFound", err)
			}
			requireOneLive(t, e.pool, g, p, replSep, &live, "100", 1)
			if d := deletedAtOf(t, e.pool, g.table, orig); d == nil {
				t.Fatalf("original restored by another household")
			}
			if d := deletedAtOf(t, e.pool, g.table, taken); d == nil {
				t.Fatalf("displaced row restored by another household")
			}
		})
	})
}

// A move and its undo both move the report watermark, so each affected month's
// report is rebuilt on the next read: the source month loses the value, the
// target gains it, and the undo puts both back (INV-SOFT-DELETE-11 for moves).
//
// covers: INV-SNAPSHOTS-15, INV-SOFT-DELETE-13
func TestSnapshotMove_ReportsBothMonths(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)
	user := testutil.CreateHouseholdWithUser(t, q, "Alice")
	ctx := identity.WithUser(context.Background(), user)
	ar := repo.NewAssetRepo(tdb.Pool)
	mr := repo.NewMonthlyReportRepo(tdb.Pool)

	nw := func(t *testing.T, month time.Time, want string) {
		t.Helper()
		rep, err := mr.GetReport(ctx, month)
		if err != nil {
			t.Fatalf("GetReport %s: %v", month.Format("2006-01"), err)
		}
		if !rep.NwTotal.Equal(rd(want)) {
			t.Fatalf("%s nw_total = %s, want %s", month.Format("2006-01"), rep.NwTotal, want)
		}
	}

	// A second account anchors the report range at August, so September still
	// has a report once the mover's September reading has left it.
	anchor := newBankAccount(ctx, t, ar, "Anchor")
	if _, err := ar.CreateAssetSnapshot(ctx, repo.CreateAssetSnapshotParams{AssetID: anchor.Asset.ID, YearMonth: time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC), Amount: rd("5"), Currency: "IDR"}); err != nil {
		t.Fatalf("create anchor: %v", err)
	}
	acct := newBankAccount(ctx, t, ar, "Mover")
	sepSnap, err := ar.CreateAssetSnapshot(ctx, repo.CreateAssetSnapshotParams{AssetID: acct.Asset.ID, YearMonth: replSep, Amount: rd("100"), Currency: "IDR"})
	if err != nil {
		t.Fatalf("create Sep: %v", err)
	}
	if _, err := ar.CreateAssetSnapshot(ctx, repo.CreateAssetSnapshotParams{AssetID: acct.Asset.ID, YearMonth: replOct, Amount: rd("30"), Currency: "IDR"}); err != nil {
		t.Fatalf("create Oct: %v", err)
	}
	nw(t, replSep, "105")
	nw(t, replOct, "35")

	moved, err := ar.MoveAssetSnapshot(ctx, sepSnap.ID, replOct)
	if err != nil || moved.ReplacedID == nil {
		t.Fatalf("move Sep -> Oct: %+v %v", moved, err)
	}
	nw(t, replSep, "5") // the source month no longer has the reading
	nw(t, replOct, "105")

	if err := ar.UndoMoveAssetSnapshot(ctx, sepSnap.ID, moved.ID, moved.ReplacedID); err != nil {
		t.Fatalf("undo move: %v", err)
	}
	nw(t, replSep, "105")
	nw(t, replOct, "35")
}
