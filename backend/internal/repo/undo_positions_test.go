package repo_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kerti/balances-v2/backend/internal/repo"
)

// Undoing a position delete (ADR-0058 §2). The delete cascades its tombstone to
// the children that were live (INV-SOFT-DELETE-05) and stamps them with the
// parent's own deleted_at; the undo restores exactly those children, so a
// snapshot or transaction deleted earlier by hand stays archived.

// undoPositionKind is one of the ten position routes: how to create a bare
// position, delete it, undo it, and a sibling subtype's undo that must refuse
// the id (nil where the table holds a single subtype).
type undoPositionKind struct {
	name, table string
	create      func(t *testing.T, ctx context.Context, pool *pgxpool.Pool) uuid.UUID
	del         func(ctx context.Context, pool *pgxpool.Pool, id uuid.UUID) error
	undo        func(ctx context.Context, pool *pgxpool.Pool, id uuid.UUID) error
	wrongUndo   func(ctx context.Context, pool *pgxpool.Pool, id uuid.UUID) error
}

func undoPositionKinds() []undoPositionKind {
	ar := func(p *pgxpool.Pool) *repo.AssetRepo { return repo.NewAssetRepo(p) }
	ir := func(p *pgxpool.Pool) *repo.InvestmentRepo { return repo.NewInvestmentRepo(p) }
	must := func(t *testing.T, name string, err error) {
		t.Helper()
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	return []undoPositionKind{
		{
			name: "bank account", table: "assets",
			create: func(t *testing.T, ctx context.Context, p *pgxpool.Pool) uuid.UUID {
				return newBankAccount(ctx, t, ar(p), "Checking").Asset.ID
			},
			del: func(ctx context.Context, p *pgxpool.Pool, id uuid.UUID) error {
				return ar(p).DeleteBankAccount(ctx, id)
			},
			undo: func(ctx context.Context, p *pgxpool.Pool, id uuid.UUID) error {
				return ar(p).UndoDeleteBankAccount(ctx, id)
			},
			wrongUndo: func(ctx context.Context, p *pgxpool.Pool, id uuid.UUID) error {
				return ar(p).UndoDeleteVehicle(ctx, id)
			},
		},
		{
			name: "property", table: "assets",
			create: func(t *testing.T, ctx context.Context, p *pgxpool.Pool) uuid.UUID {
				v, err := ar(p).CreateProperty(ctx, repo.CreatePropertyParams{DisplayName: "House", OwnershipType: "joint", NativeCurrency: "IDR", PropertyType: "house"})
				must(t, "CreateProperty", err)
				return v.Asset.ID
			},
			del: func(ctx context.Context, p *pgxpool.Pool, id uuid.UUID) error { return ar(p).DeleteProperty(ctx, id) },
			undo: func(ctx context.Context, p *pgxpool.Pool, id uuid.UUID) error {
				return ar(p).UndoDeleteProperty(ctx, id)
			},
			wrongUndo: func(ctx context.Context, p *pgxpool.Pool, id uuid.UUID) error {
				return ar(p).UndoDeleteBankAccount(ctx, id)
			},
		},
		{
			name: "vehicle", table: "assets",
			create: func(t *testing.T, ctx context.Context, p *pgxpool.Pool) uuid.UUID {
				v, err := ar(p).CreateVehicle(ctx, repo.CreateVehicleParams{DisplayName: "Car", OwnershipType: "joint", NativeCurrency: "IDR", VehicleType: "car"})
				must(t, "CreateVehicle", err)
				return v.Asset.ID
			},
			del: func(ctx context.Context, p *pgxpool.Pool, id uuid.UUID) error { return ar(p).DeleteVehicle(ctx, id) },
			undo: func(ctx context.Context, p *pgxpool.Pool, id uuid.UUID) error {
				return ar(p).UndoDeleteVehicle(ctx, id)
			},
			wrongUndo: func(ctx context.Context, p *pgxpool.Pool, id uuid.UUID) error {
				return ar(p).UndoDeleteProperty(ctx, id)
			},
		},
		{
			name: "liability", table: "liabilities",
			create: func(t *testing.T, ctx context.Context, p *pgxpool.Pool) uuid.UUID {
				l, err := repo.NewLiabilityRepo(p).CreateLiability(ctx, repo.CreateLiabilityParams{
					DisplayName: "Mortgage", Subtype: "institutional", OwnershipType: "joint", NativeCurrency: "IDR", CounterpartyName: "Bank",
				})
				must(t, "CreateLiability", err)
				return l.ID
			},
			del: func(ctx context.Context, p *pgxpool.Pool, id uuid.UUID) error {
				return repo.NewLiabilityRepo(p).DeleteLiability(ctx, id)
			},
			undo: func(ctx context.Context, p *pgxpool.Pool, id uuid.UUID) error {
				return repo.NewLiabilityRepo(p).UndoDeleteLiability(ctx, id)
			},
		},
		{
			name: "receivable", table: "receivables",
			create: func(t *testing.T, ctx context.Context, p *pgxpool.Pool) uuid.UUID {
				r, err := repo.NewReceivableRepo(p).CreateReceivable(ctx, repo.CreateReceivableParams{
					DisplayName: "Loan", OwnershipType: "joint", NativeCurrency: "IDR", CounterpartyName: "Bob",
				})
				must(t, "CreateReceivable", err)
				return r.ID
			},
			del: func(ctx context.Context, p *pgxpool.Pool, id uuid.UUID) error {
				return repo.NewReceivableRepo(p).DeleteReceivable(ctx, id)
			},
			undo: func(ctx context.Context, p *pgxpool.Pool, id uuid.UUID) error {
				return repo.NewReceivableRepo(p).UndoDeleteReceivable(ctx, id)
			},
		},
		{
			name: "stock", table: "investments",
			create: func(t *testing.T, ctx context.Context, p *pgxpool.Pool) uuid.UUID {
				s, err := ir(p).CreateStock(ctx, repo.CreateStockParams{DisplayName: "BBCA", OwnershipType: "joint", NativeCurrency: "IDR", RiskProfile: "medium", Ticker: "BBCA", Exchange: "IDX"})
				must(t, "CreateStock", err)
				return s.Investment.ID
			},
			del:       func(ctx context.Context, p *pgxpool.Pool, id uuid.UUID) error { return ir(p).DeleteStock(ctx, id) },
			undo:      func(ctx context.Context, p *pgxpool.Pool, id uuid.UUID) error { return ir(p).UndoDeleteStock(ctx, id) },
			wrongUndo: func(ctx context.Context, p *pgxpool.Pool, id uuid.UUID) error { return ir(p).UndoDeleteGold(ctx, id) },
		},
		{
			name: "mutual fund", table: "investments",
			create: func(t *testing.T, ctx context.Context, p *pgxpool.Pool) uuid.UUID {
				m, err := ir(p).CreateMutualFund(ctx, repo.CreateMutualFundParams{DisplayName: "MF", OwnershipType: "joint", NativeCurrency: "IDR", RiskProfile: "medium", FundCode: "BNI-AM", FundType: "money_market"})
				must(t, "CreateMutualFund", err)
				return m.Investment.ID
			},
			del: func(ctx context.Context, p *pgxpool.Pool, id uuid.UUID) error { return ir(p).DeleteMutualFund(ctx, id) },
			undo: func(ctx context.Context, p *pgxpool.Pool, id uuid.UUID) error {
				return ir(p).UndoDeleteMutualFund(ctx, id)
			},
			wrongUndo: func(ctx context.Context, p *pgxpool.Pool, id uuid.UUID) error { return ir(p).UndoDeleteStock(ctx, id) },
		},
		{
			name: "gold", table: "investments",
			create: func(t *testing.T, ctx context.Context, p *pgxpool.Pool) uuid.UUID {
				g, err := ir(p).CreateGold(ctx, repo.CreateGoldParams{DisplayName: "Gold", OwnershipType: "joint", NativeCurrency: "IDR", RiskProfile: "low", Form: "bar", Purity: rd("0.9999")})
				must(t, "CreateGold", err)
				return g.Investment.ID
			},
			del:  func(ctx context.Context, p *pgxpool.Pool, id uuid.UUID) error { return ir(p).DeleteGold(ctx, id) },
			undo: func(ctx context.Context, p *pgxpool.Pool, id uuid.UUID) error { return ir(p).UndoDeleteGold(ctx, id) },
			wrongUndo: func(ctx context.Context, p *pgxpool.Pool, id uuid.UUID) error {
				return ir(p).UndoDeleteMutualFund(ctx, id)
			},
		},
		{
			name: "bond", table: "investments",
			create: func(t *testing.T, ctx context.Context, p *pgxpool.Pool) uuid.UUID {
				b, err := ir(p).CreateBond(ctx, repo.CreateBondParams{
					DisplayName: "ORI024", OwnershipType: "joint", NativeCurrency: "IDR", RiskProfile: "low", BondType: "secondary_market",
					Issuer: "Govt", CouponRate: rd("6.25"), CouponFrequency: "monthly", CouponDisposition: "accrues",
					MaturityDate: time.Date(2030, time.January, 1, 0, 0, 0, 0, time.UTC),
				})
				must(t, "CreateBond", err)
				return b.Investment.ID
			},
			del:  func(ctx context.Context, p *pgxpool.Pool, id uuid.UUID) error { return ir(p).DeleteBond(ctx, id) },
			undo: func(ctx context.Context, p *pgxpool.Pool, id uuid.UUID) error { return ir(p).UndoDeleteBond(ctx, id) },
			wrongUndo: func(ctx context.Context, p *pgxpool.Pool, id uuid.UUID) error {
				return ir(p).UndoDeleteTimeDeposit(ctx, id)
			},
		},
		{
			name: "time deposit", table: "investments",
			create: func(t *testing.T, ctx context.Context, p *pgxpool.Pool) uuid.UUID {
				d, err := ir(p).CreateTimeDeposit(ctx, repo.CreateTimeDepositParams{
					DisplayName: "BCA TD", OwnershipType: "joint", NativeCurrency: "IDR", RiskProfile: "medium", BankName: "BCA",
					Principal: rd("50000000"), InterestRate: rd("5.5"), TermMonths: 12,
					PlacementDate: time.Date(2026, time.January, 15, 0, 0, 0, 0, time.UTC), MaturityDate: time.Date(2027, time.January, 15, 0, 0, 0, 0, time.UTC),
					RolloverPolicy: "auto_renew_principal",
				})
				must(t, "CreateTimeDeposit", err)
				return d.Investment.ID
			},
			del: func(ctx context.Context, p *pgxpool.Pool, id uuid.UUID) error {
				return ir(p).DeleteTimeDeposit(ctx, id)
			},
			undo: func(ctx context.Context, p *pgxpool.Pool, id uuid.UUID) error {
				return ir(p).UndoDeleteTimeDeposit(ctx, id)
			},
			wrongUndo: func(ctx context.Context, p *pgxpool.Pool, id uuid.UUID) error { return ir(p).UndoDeleteBond(ctx, id) },
		},
	}
}

// TestUndoDeletePosition_AllSubtypes: for each of the ten position routes the
// undo restores the position, refuses another household's id, refuses a sibling
// subtype's route (the restore twin of the Delete{Subtype} guard), and a second
// undo of the now-live row is not found.
//
// covers: INV-SOFT-DELETE-06, INV-SOFT-DELETE-08, INV-SOFT-DELETE-12
func TestUndoDeletePosition_AllSubtypes(t *testing.T) {
	p := newUndoPair(t)
	for _, k := range undoPositionKinds() {
		t.Run(k.name, func(t *testing.T) {
			id := k.create(t, p.alice, p.pool)
			if err := k.del(p.alice, p.pool, id); err != nil {
				t.Fatalf("delete: %v", err)
			}

			if err := k.undo(p.bob, p.pool, id); !errors.Is(err, repo.ErrNotFound) {
				t.Errorf("bob undo: want ErrNotFound, got %v", err)
			}
			if k.wrongUndo != nil {
				if err := k.wrongUndo(p.alice, p.pool, id); !errors.Is(err, repo.ErrNotFound) {
					t.Errorf("sibling-subtype undo: want ErrNotFound, got %v", err)
				}
			}
			if !p.archived(t, k.table, id) {
				t.Fatal("a refused undo restored the position")
			}

			if err := k.undo(p.alice, p.pool, id); err != nil {
				t.Fatalf("undo: %v", err)
			}
			if p.archived(t, k.table, id) {
				t.Error("position still archived after its undo")
			}
			if err := k.undo(p.alice, p.pool, id); !errors.Is(err, repo.ErrNotFound) {
				t.Errorf("second undo: want ErrNotFound, got %v", err)
			}
		})
	}
}

// TestUndoDeletePosition_RestoresCascadedChildren: the children the delete
// cascaded come back with the parent, in every table that holds them, while a
// child that was already archived before the position was deleted stays
// archived.
//
// covers: INV-SOFT-DELETE-12
func TestUndoDeletePosition_RestoresCascadedChildren(t *testing.T) {
	p := newUndoPair(t)
	undoFor := map[string]func(ctx context.Context, id uuid.UUID) error{
		"asset":      repo.NewAssetRepo(p.pool).UndoDeleteBankAccount,
		"liability":  repo.NewLiabilityRepo(p.pool).UndoDeleteLiability,
		"receivable": repo.NewReceivableRepo(p.pool).UndoDeleteReceivable,
		"investment": repo.NewInvestmentRepo(p.pool).UndoDeleteStock,
	}

	for _, g := range cascadeGroups() {
		t.Run(g.name+" all children", func(t *testing.T) {
			id, del := g.build(p.alice, t, p.pool, "All "+g.name)
			if err := del(p.alice); err != nil {
				t.Fatalf("delete: %v", err)
			}
			for _, c := range g.children {
				if got := countLiveChildren(t, p.pool, c.table, c.fkCol, id); got != 0 {
					t.Fatalf("%s: %d live rows after the delete, want 0", c.table, got)
				}
			}
			if err := undoFor[g.name](p.alice, id); err != nil {
				t.Fatalf("undo: %v", err)
			}
			if p.archived(t, g.parent, id) {
				t.Error("parent still archived")
			}
			for _, c := range g.children {
				if got := countLiveChildren(t, p.pool, c.table, c.fkCol, id); got != 1 {
					t.Errorf("%s: %d live rows after the undo, want 1", c.table, got)
				}
			}
		})

		for i, early := range g.children {
			t.Run(g.name+" earlier-archived "+early.table, func(t *testing.T) {
				id, del := g.build(p.alice, t, p.pool, "Early "+g.name+early.table)
				if _, err := p.pool.Exec(context.Background(),
					"UPDATE "+early.table+" SET deleted_at = now() - interval '1 day' WHERE "+early.fkCol+" = $1", id); err != nil {
					t.Fatalf("pre-archive %s: %v", early.table, err)
				}
				if err := del(p.alice); err != nil {
					t.Fatalf("delete: %v", err)
				}
				if err := undoFor[g.name](p.alice, id); err != nil {
					t.Fatalf("undo: %v", err)
				}
				for j, c := range g.children {
					want := 1
					if j == i {
						want = 0
					}
					if got := countLiveChildren(t, p.pool, c.table, c.fkCol, id); got != want {
						t.Errorf("%s: %d live rows after the undo, want %d", c.table, got, want)
					}
				}
			})
		}
	}
}

// TestUndoDeletePosition_RestoresReportEffect: the position's value returns to
// net worth on the next report read.
//
// covers: INV-SOFT-DELETE-11
func TestUndoDeletePosition_RestoresReportEffect(t *testing.T) {
	p := newUndoPair(t)
	ar := repo.NewAssetRepo(p.pool)
	keep := newBankAccount(p.alice, t, ar, "Keep")
	gone := newBankAccount(p.alice, t, ar, "Gone")
	for _, a := range []uuid.UUID{keep.Asset.ID, gone.Asset.ID} {
		if _, err := ar.CreateAssetSnapshot(p.alice, repo.CreateAssetSnapshotParams{AssetID: a, YearMonth: replSep, Amount: rd("100"), Currency: "IDR"}); err != nil {
			t.Fatalf("snapshot: %v", err)
		}
	}
	nw := func() string { return p.reportAt(t, replSep).NwTotal.String() }

	if got := nw(); got != "200" {
		t.Fatalf("before = %s, want 200", got)
	}
	if err := ar.DeleteBankAccount(p.alice, gone.Asset.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if got := nw(); got != "100" {
		t.Fatalf("after delete = %s, want 100", got)
	}
	if err := ar.UndoDeleteBankAccount(p.alice, gone.Asset.ID); err != nil {
		t.Fatalf("undo: %v", err)
	}
	if got := nw(); got != "200" {
		t.Errorf("after undo = %s, want 200", got)
	}
}
