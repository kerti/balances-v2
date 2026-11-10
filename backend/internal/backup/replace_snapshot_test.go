package backup

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/kerti/balances-v2/backend/internal/db"
	"github.com/kerti/balances-v2/backend/internal/identity"
	"github.com/kerti/balances-v2/backend/internal/repo"
	"github.com/kerti/balances-v2/backend/internal/testutil"
)

// A replace archives the displaced snapshot instead of overwriting it
// (ADR-0058 §1), and leaves `supersedes` null so that archived row is nothing a
// live row points at. A compacted backup therefore leaves it behind, like any
// other row the user would find in the Recycle Bin; a full backup keeps it.
// Had the replace stored the link, the compacted file would carry every
// replaced month, and after a second edit would carry a row whose own target is
// missing.
//
// covers: INV-SNAPSHOTS-11
func TestCompactedBackupDropsReplacedSnapshots(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)
	alice := testutil.CreateHouseholdWithUser(t, q, "Alice")
	ctx := identity.WithUser(context.Background(), alice)
	assets := repo.NewAssetRepo(tdb.Pool)
	h := New(tdb.Pool, "http://test.local", &stubIssuer{}, &stubNotifier{}, false, DemoConfig{})

	oct := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)

	acct, err := assets.CreateBankAccount(ctx, repo.CreateBankAccountParams{
		DisplayName: "Main checking", OwnershipType: "sole", SoleOwnerUserID: &alice.ID,
		NativeCurrency: "IDR", BankName: "TestBank", AccountNumber: "1234567890", AccountType: "savings",
	})
	if err != nil {
		t.Fatalf("CreateBankAccount: %v", err)
	}
	assetID := acct.Asset.ID

	first, err := assets.CreateAssetSnapshot(ctx, repo.CreateAssetSnapshotParams{
		AssetID: assetID, YearMonth: oct, Amount: decimal.RequireFromString("100"), Currency: "IDR",
	})
	if err != nil {
		t.Fatalf("CreateAssetSnapshot: %v", err)
	}
	// Two replaces in a row: the case a stored link would have broken.
	second, err := assets.UpdateAssetSnapshot(ctx, repo.UpdateAssetSnapshotParams{
		SnapshotID: first.ID, Amount: decimal.RequireFromString("150"), Currency: "IDR",
	})
	if err != nil {
		t.Fatalf("first replace: %v", err)
	}
	third, err := assets.UpdateAssetSnapshot(ctx, repo.UpdateAssetSnapshotParams{
		SnapshotID: second.ID, Amount: decimal.RequireFromString("175"), Currency: "IDR",
	})
	if err != nil {
		t.Fatalf("second replace: %v", err)
	}

	t.Run("compacted carries only the live row", func(t *testing.T) {
		env, err := Parse(bytes.NewReader(compactedExportBytes(ctx, t, h)))
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}
		snaps := env.Household.AssetSnapshots
		if len(snaps) != 1 {
			t.Fatalf("asset_snapshots carried = %d, want 1 (the live row only)", len(snaps))
		}
		got := snaps[0]
		if got.ID != third.ID || !got.Amount.Equal(decimal.RequireFromString("175")) || got.DeletedAt.Valid {
			t.Errorf("carried row = %s amount %s deleted=%v, want live %s at 175", got.ID, got.Amount, got.DeletedAt.Valid, third.ID)
		}
		if got.Supersedes != nil {
			t.Errorf("carried row supersedes %s, want null: a replace must not store the link", got.Supersedes)
		}
		if _, err := Validate(env, callerFrom(alice)); err != nil {
			t.Fatalf("Validate rejects the compacted file: %v", err)
		}
	})

	t.Run("full keeps both displaced rows", func(t *testing.T) {
		env, err := Parse(bytes.NewReader(exportBytes(ctx, t, h)))
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}
		archived := map[string]string{}
		for _, s := range env.Household.AssetSnapshots {
			if s.DeletedAt.Valid {
				archived[s.ID.String()] = s.Amount.String()
			}
		}
		if len(archived) != 2 || archived[first.ID.String()] != "100" || archived[second.ID.String()] != "150" {
			t.Errorf("archived rows in the full file = %v, want 100 and 150", archived)
		}
	})
}
