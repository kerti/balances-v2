package repo_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/kerti/balances-v2/backend/internal/db"
	"github.com/kerti/balances-v2/backend/internal/identity"
	"github.com/kerti/balances-v2/backend/internal/repo"
	"github.com/kerti/balances-v2/backend/internal/testutil"
)

// tagFamily is one position subtype's create/update pair, reduced to what the
// tag_id contract needs: the new row's id and returned tag_id, plus the table
// and filter to read the stored column and raw-count rows.
type tagFamily struct {
	name   string
	table  string // parent table holding tag_id
	filter string // SQL predicate selecting this subtype's rows
	create func(ctx context.Context, name string, tag *uuid.UUID) (uuid.UUID, *uuid.UUID, error)
	update func(ctx context.Context, id uuid.UUID, name string, c repo.TagChange) (*uuid.UUID, error)
}

func positionTagFamilies(ar *repo.AssetRepo, ir *repo.InvestmentRepo, lr *repo.LiabilityRepo, rr *repo.ReceivableRepo) []tagFamily {
	rate := decimal.RequireFromString("0.055")
	maturity := time.Date(2029, time.October, 15, 0, 0, 0, 0, time.UTC)
	placement := time.Date(2026, time.January, 15, 0, 0, 0, 0, time.UTC)
	tdMaturity := time.Date(2027, time.January, 15, 0, 0, 0, 0, time.UTC)

	return []tagFamily{
		{
			name: "bank_account", table: "assets", filter: "subtype = 'bank_account'",
			create: func(ctx context.Context, name string, tag *uuid.UUID) (uuid.UUID, *uuid.UUID, error) {
				x, err := ar.CreateBankAccount(ctx, repo.CreateBankAccountParams{DisplayName: name, OwnershipType: "joint", NativeCurrency: "IDR", BankName: "BCA", AccountType: "savings", TagID: tag})
				if err != nil {
					return uuid.Nil, nil, err
				}
				return x.Asset.ID, x.Asset.TagID, nil
			},
			update: func(ctx context.Context, id uuid.UUID, name string, c repo.TagChange) (*uuid.UUID, error) {
				x, err := ar.UpdateBankAccount(ctx, id, repo.UpdateBankAccountParams{DisplayName: name, OwnershipType: "joint", BankName: "BCA", AccountType: "savings", Tag: c})
				if err != nil {
					return nil, err
				}
				return x.Asset.TagID, nil
			},
		},
		{
			name: "property", table: "assets", filter: "subtype = 'property'",
			create: func(ctx context.Context, name string, tag *uuid.UUID) (uuid.UUID, *uuid.UUID, error) {
				x, err := ar.CreateProperty(ctx, repo.CreatePropertyParams{DisplayName: name, OwnershipType: "joint", NativeCurrency: "IDR", PropertyType: "house", TagID: tag})
				if err != nil {
					return uuid.Nil, nil, err
				}
				return x.Asset.ID, x.Asset.TagID, nil
			},
			update: func(ctx context.Context, id uuid.UUID, name string, c repo.TagChange) (*uuid.UUID, error) {
				x, err := ar.UpdateProperty(ctx, id, repo.UpdatePropertyParams{DisplayName: name, OwnershipType: "joint", PropertyType: "house", Tag: c})
				if err != nil {
					return nil, err
				}
				return x.Asset.TagID, nil
			},
		},
		{
			name: "vehicle", table: "assets", filter: "subtype = 'vehicle'",
			create: func(ctx context.Context, name string, tag *uuid.UUID) (uuid.UUID, *uuid.UUID, error) {
				x, err := ar.CreateVehicle(ctx, repo.CreateVehicleParams{DisplayName: name, OwnershipType: "joint", NativeCurrency: "IDR", VehicleType: "car", TagID: tag})
				if err != nil {
					return uuid.Nil, nil, err
				}
				return x.Asset.ID, x.Asset.TagID, nil
			},
			update: func(ctx context.Context, id uuid.UUID, name string, c repo.TagChange) (*uuid.UUID, error) {
				x, err := ar.UpdateVehicle(ctx, id, repo.UpdateVehicleParams{DisplayName: name, OwnershipType: "joint", VehicleType: "car", Tag: c})
				if err != nil {
					return nil, err
				}
				return x.Asset.TagID, nil
			},
		},
		{
			name: "liability", table: "liabilities", filter: "true",
			create: func(ctx context.Context, name string, tag *uuid.UUID) (uuid.UUID, *uuid.UUID, error) {
				x, err := lr.CreateLiability(ctx, repo.CreateLiabilityParams{DisplayName: name, Subtype: "personal", OwnershipType: "joint", NativeCurrency: "IDR", CounterpartyName: "Uncle", TagID: tag})
				if err != nil {
					return uuid.Nil, nil, err
				}
				return x.ID, x.TagID, nil
			},
			update: func(ctx context.Context, id uuid.UUID, name string, c repo.TagChange) (*uuid.UUID, error) {
				x, err := lr.UpdateLiability(ctx, id, repo.UpdateLiabilityParams{DisplayName: name, OwnershipType: "joint", CounterpartyName: "Uncle", Tag: c})
				if err != nil {
					return nil, err
				}
				return x.TagID, nil
			},
		},
		{
			name: "receivable", table: "receivables", filter: "true",
			create: func(ctx context.Context, name string, tag *uuid.UUID) (uuid.UUID, *uuid.UUID, error) {
				x, err := rr.CreateReceivable(ctx, repo.CreateReceivableParams{DisplayName: name, OwnershipType: "joint", NativeCurrency: "IDR", CounterpartyName: "Brother", TagID: tag})
				if err != nil {
					return uuid.Nil, nil, err
				}
				return x.ID, x.TagID, nil
			},
			update: func(ctx context.Context, id uuid.UUID, name string, c repo.TagChange) (*uuid.UUID, error) {
				x, err := rr.UpdateReceivable(ctx, id, repo.UpdateReceivableParams{DisplayName: name, OwnershipType: "joint", CounterpartyName: "Brother", Tag: c})
				if err != nil {
					return nil, err
				}
				return x.TagID, nil
			},
		},
		{
			name: "stock", table: "investments", filter: "subtype = 'stock'",
			create: func(ctx context.Context, name string, tag *uuid.UUID) (uuid.UUID, *uuid.UUID, error) {
				x, err := ir.CreateStock(ctx, repo.CreateStockParams{DisplayName: name, OwnershipType: "joint", NativeCurrency: "IDR", RiskProfile: "medium", Ticker: "BBCA", Exchange: "IDX", TagID: tag})
				if err != nil {
					return uuid.Nil, nil, err
				}
				return x.Investment.ID, x.Investment.TagID, nil
			},
			update: func(ctx context.Context, id uuid.UUID, name string, c repo.TagChange) (*uuid.UUID, error) {
				x, err := ir.UpdateStock(ctx, id, repo.UpdateStockParams{DisplayName: name, OwnershipType: "joint", RiskProfile: "medium", Ticker: "BBCA", Exchange: "IDX", Tag: c})
				if err != nil {
					return nil, err
				}
				return x.Investment.TagID, nil
			},
		},
		{
			name: "mutual_fund", table: "investments", filter: "subtype = 'mutual_fund'",
			create: func(ctx context.Context, name string, tag *uuid.UUID) (uuid.UUID, *uuid.UUID, error) {
				x, err := ir.CreateMutualFund(ctx, repo.CreateMutualFundParams{DisplayName: name, OwnershipType: "joint", NativeCurrency: "IDR", RiskProfile: "medium", FundCode: "SCMUS", FundType: "money_market", TagID: tag})
				if err != nil {
					return uuid.Nil, nil, err
				}
				return x.Investment.ID, x.Investment.TagID, nil
			},
			update: func(ctx context.Context, id uuid.UUID, name string, c repo.TagChange) (*uuid.UUID, error) {
				x, err := ir.UpdateMutualFund(ctx, id, repo.UpdateMutualFundParams{DisplayName: name, OwnershipType: "joint", RiskProfile: "medium", FundCode: "SCMUS", FundType: "money_market", Tag: c})
				if err != nil {
					return nil, err
				}
				return x.Investment.TagID, nil
			},
		},
		{
			name: "gold", table: "investments", filter: "subtype = 'gold'",
			create: func(ctx context.Context, name string, tag *uuid.UUID) (uuid.UUID, *uuid.UUID, error) {
				x, err := ir.CreateGold(ctx, repo.CreateGoldParams{DisplayName: name, OwnershipType: "joint", NativeCurrency: "IDR", RiskProfile: "medium", Form: "bar", Purity: decimal.RequireFromString("0.9999"), TagID: tag})
				if err != nil {
					return uuid.Nil, nil, err
				}
				return x.Investment.ID, x.Investment.TagID, nil
			},
			update: func(ctx context.Context, id uuid.UUID, name string, c repo.TagChange) (*uuid.UUID, error) {
				x, err := ir.UpdateGold(ctx, id, repo.UpdateGoldParams{DisplayName: name, OwnershipType: "joint", RiskProfile: "medium", Form: "bar", Purity: decimal.RequireFromString("0.9999"), Tag: c})
				if err != nil {
					return nil, err
				}
				return x.Investment.TagID, nil
			},
		},
		{
			name: "bond", table: "investments", filter: "subtype = 'bond'",
			create: func(ctx context.Context, name string, tag *uuid.UUID) (uuid.UUID, *uuid.UUID, error) {
				x, err := ir.CreateBond(ctx, repo.CreateBondParams{DisplayName: name, OwnershipType: "joint", NativeCurrency: "IDR", RiskProfile: "medium", BondType: "secondary_market", Issuer: "Republik Indonesia", CouponRate: rate, CouponFrequency: "monthly", MaturityDate: maturity, TagID: tag})
				if err != nil {
					return uuid.Nil, nil, err
				}
				return x.Investment.ID, x.Investment.TagID, nil
			},
			update: func(ctx context.Context, id uuid.UUID, name string, c repo.TagChange) (*uuid.UUID, error) {
				x, err := ir.UpdateBond(ctx, id, repo.UpdateBondParams{DisplayName: name, OwnershipType: "joint", RiskProfile: "medium", BondType: "secondary_market", Issuer: "Republik Indonesia", CouponRate: rate, CouponFrequency: "monthly", CouponDisposition: repo.CouponDispositionPaysOut, MaturityDate: maturity, Tag: c})
				if err != nil {
					return nil, err
				}
				return x.Investment.TagID, nil
			},
		},
		{
			name: "time_deposit", table: "investments", filter: "subtype = 'time_deposit'",
			create: func(ctx context.Context, name string, tag *uuid.UUID) (uuid.UUID, *uuid.UUID, error) {
				x, err := ir.CreateTimeDeposit(ctx, repo.CreateTimeDepositParams{DisplayName: name, OwnershipType: "joint", NativeCurrency: "IDR", RiskProfile: "medium", BankName: "BCA", Principal: decimal.NewFromInt(50_000_000), InterestRate: rate, TermMonths: 12, PlacementDate: placement, MaturityDate: tdMaturity, RolloverPolicy: "auto_renew_principal", TagID: tag})
				if err != nil {
					return uuid.Nil, nil, err
				}
				return x.Investment.ID, x.Investment.TagID, nil
			},
			update: func(ctx context.Context, id uuid.UUID, name string, c repo.TagChange) (*uuid.UUID, error) {
				x, err := ir.UpdateTimeDeposit(ctx, id, repo.UpdateTimeDepositParams{DisplayName: name, OwnershipType: "joint", RiskProfile: "medium", BankName: "BCA", Principal: decimal.NewFromInt(50_000_000), InterestRate: rate, TermMonths: 12, PlacementDate: placement, MaturityDate: tdMaturity, RolloverPolicy: "auto_renew_principal", Tag: c})
				if err != nil {
					return nil, err
				}
				return x.Investment.TagID, nil
			},
		},
	}
}

// TestPositionWrites_TagID covers issue #839: every position subtype's create
// and update accepts a tag_id that is written in the same transaction as the
// position. Set, change, clear, leave-alone-when-absent, a Tag from another
// household, a deleted Tag, and "an invalid Tag leaves no position behind" are
// asserted per subtype against raw SQL, so a getter that hides a row cannot fake
// a pass.
// covers: INV-TAGS-07, INV-TENANCY-12
func TestPositionWrites_TagID(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)

	alice := testutil.CreateHouseholdWithUser(t, q, "Alice")
	bob := testutil.CreateHouseholdWithUser(t, q, "Bob")
	if alice.HouseholdID == bob.HouseholdID {
		t.Fatalf("fixture: alice and bob ended up in the same household")
	}
	aliceCtx := identity.WithUser(context.Background(), alice)
	bobCtx := identity.WithUser(context.Background(), bob)

	tr := repo.NewTagRepo(tdb.Pool)
	tagA, err := tr.CreateTag(aliceCtx, "Goals", "#3b82f6")
	if err != nil {
		t.Fatalf("CreateTag A: %v", err)
	}
	tagB, err := tr.CreateTag(aliceCtx, "Retirement", "#10b981")
	if err != nil {
		t.Fatalf("CreateTag B: %v", err)
	}
	tagDead, err := tr.CreateTag(aliceCtx, "Old", "#ef4444")
	if err != nil {
		t.Fatalf("CreateTag Old: %v", err)
	}
	if err := tr.DeleteTag(aliceCtx, tagDead.ID); err != nil {
		t.Fatalf("DeleteTag Old: %v", err)
	}
	bobTag, err := tr.CreateTag(bobCtx, "Bob's", "#f59e0b")
	if err != nil {
		t.Fatalf("CreateTag Bob: %v", err)
	}

	ctx := context.Background()
	storedTag := func(t *testing.T, f tagFamily, id uuid.UUID) *uuid.UUID {
		t.Helper()
		var got *uuid.UUID
		if err := tdb.Pool.QueryRow(ctx, fmt.Sprintf("SELECT tag_id FROM %s WHERE id = $1", f.table), id).Scan(&got); err != nil {
			t.Fatalf("read stored tag_id: %v", err)
		}
		return got
	}
	storedName := func(t *testing.T, f tagFamily, id uuid.UUID) string {
		t.Helper()
		var got string
		if err := tdb.Pool.QueryRow(ctx, fmt.Sprintf("SELECT display_name FROM %s WHERE id = $1", f.table), id).Scan(&got); err != nil {
			t.Fatalf("read stored display_name: %v", err)
		}
		return got
	}
	count := func(t *testing.T, f tagFamily) int {
		t.Helper()
		var n int
		if err := tdb.Pool.QueryRow(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s", f.table, f.filter)).Scan(&n); err != nil {
			t.Fatalf("count: %v", err)
		}
		return n
	}
	same := func(a, b *uuid.UUID) bool {
		if a == nil || b == nil {
			return a == nil && b == nil
		}
		return *a == *b
	}

	fams := positionTagFamilies(repo.NewAssetRepo(tdb.Pool), repo.NewInvestmentRepo(tdb.Pool), repo.NewLiabilityRepo(tdb.Pool), repo.NewReceivableRepo(tdb.Pool))
	for _, f := range fams {
		t.Run(f.name, func(t *testing.T) {
			// Create without a tag: untagged.
			plainID, plainTag, err := f.create(aliceCtx, "plain", nil)
			if err != nil {
				t.Fatalf("create untagged: %v", err)
			}
			if plainTag != nil || storedTag(t, f, plainID) != nil {
				t.Errorf("untagged create: returned %v stored %v, want nil", plainTag, storedTag(t, f, plainID))
			}

			// Create with a tag: returned aggregate and the stored column agree.
			id, gotTag, err := f.create(aliceCtx, "tagged", &tagA.ID)
			if err != nil {
				t.Fatalf("create tagged: %v", err)
			}
			if !same(gotTag, &tagA.ID) || !same(storedTag(t, f, id), &tagA.ID) {
				t.Errorf("tagged create: returned %v stored %v, want %v", gotTag, storedTag(t, f, id), tagA.ID)
			}

			// Update: change A -> B.
			gotTag, err = f.update(aliceCtx, id, "tagged-1", repo.TagChange{Set: true, ID: &tagB.ID})
			if err != nil {
				t.Fatalf("update change: %v", err)
			}
			if !same(gotTag, &tagB.ID) || !same(storedTag(t, f, id), &tagB.ID) {
				t.Errorf("update change: returned %v stored %v, want %v", gotTag, storedTag(t, f, id), tagB.ID)
			}

			// Update with the key absent leaves the tag alone, yet the rest of the write lands.
			gotTag, err = f.update(aliceCtx, id, "tagged-2", repo.TagChange{})
			if err != nil {
				t.Fatalf("update absent: %v", err)
			}
			if !same(gotTag, &tagB.ID) || !same(storedTag(t, f, id), &tagB.ID) {
				t.Errorf("update absent: returned %v stored %v, want unchanged %v", gotTag, storedTag(t, f, id), tagB.ID)
			}
			if got := storedName(t, f, id); got != "tagged-2" {
				t.Errorf("update absent: display_name = %q, want the rest of the update applied", got)
			}

			// Update with an explicit null clears it.
			gotTag, err = f.update(aliceCtx, id, "tagged-3", repo.TagChange{Set: true})
			if err != nil {
				t.Fatalf("update clear: %v", err)
			}
			if gotTag != nil || storedTag(t, f, id) != nil {
				t.Errorf("update clear: returned %v stored %v, want nil", gotTag, storedTag(t, f, id))
			}

			// Another household's Tag: ErrNotFound and nothing written, on create...
			before := count(t, f)
			if _, _, err := f.create(aliceCtx, "foreign-tag", &bobTag.ID); !errors.Is(err, repo.ErrNotFound) {
				t.Errorf("create with bob's tag: got %v, want ErrNotFound", err)
			}
			if after := count(t, f); after != before {
				t.Errorf("create with bob's tag left %d row(s) behind (count %d -> %d)", after-before, before, after)
			}
			// ...and on update, where the position's own edit must roll back too.
			if _, err := f.update(aliceCtx, id, "foreign-tag", repo.TagChange{Set: true, ID: &bobTag.ID}); !errors.Is(err, repo.ErrNotFound) {
				t.Errorf("update with bob's tag: got %v, want ErrNotFound", err)
			}
			if storedTag(t, f, id) != nil {
				t.Errorf("update with bob's tag: stored tag_id = %v, want nil", storedTag(t, f, id))
			}
			if got := storedName(t, f, id); got != "tagged-3" {
				t.Errorf("update with bob's tag: display_name = %q, want the edit rolled back to tagged-3", got)
			}

			// A soft-deleted Tag is as good as missing.
			if _, _, err := f.create(aliceCtx, "dead-tag", &tagDead.ID); !errors.Is(err, repo.ErrNotFound) {
				t.Errorf("create with deleted tag: got %v, want ErrNotFound", err)
			}
			if _, err := f.update(aliceCtx, id, "dead-tag", repo.TagChange{Set: true, ID: &tagDead.ID}); !errors.Is(err, repo.ErrNotFound) {
				t.Errorf("update with deleted tag: got %v, want ErrNotFound", err)
			}

			// Bob cannot reach alice's position by carrying his own Tag.
			if _, err := f.update(bobCtx, id, "hijack", repo.TagChange{Set: true, ID: &bobTag.ID}); !errors.Is(err, repo.ErrNotFound) {
				t.Errorf("bob update of alice's position: got %v, want ErrNotFound", err)
			}
			if got := storedName(t, f, id); got != "tagged-3" {
				t.Errorf("bob update: display_name = %q, want unchanged", got)
			}
		})
	}
}
