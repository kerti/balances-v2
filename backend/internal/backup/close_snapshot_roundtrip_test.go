package backup

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/kerti/balances-v2/backend/internal/db"
	"github.com/kerti/balances-v2/backend/internal/identity"
	"github.com/kerti/balances-v2/backend/internal/repo"
	"github.com/kerti/balances-v2/backend/internal/testutil"
)

// compactedExportBytes is exportBytes' compacted twin — the fidelity whose whole
// point is to leave the user's Recycle Bin behind.
func compactedExportBytes(ctx context.Context, t *testing.T, h *Handlers) []byte {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/backup/export?fidelity=compacted", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	h.handleExport(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("compacted export status = %d", rec.Code)
	}
	return rec.Body.Bytes()
}

// TestCompactedBackupPreservesTerminationUndo is the whole of #602 in one pass.
//
// Terminating a Position displaces the snapshot the user recorded in the
// termination month: that row is soft-deleted and a truthful 0-value close row
// takes its place, so un-terminate can hand the recorded value back
// (INV-LIFECYCLE-04). The displaced row therefore carries a `deleted_at` while
// being nothing the user deleted — and a compacted backup, which exists to drop
// the Recycle Bin, used to drop it too, keeping the live close row that pointed
// at it. A household restored from such a file read a carried-forward value from
// an earlier month on the next undo, silently and with no error anywhere.
//
// The two months matter: February is the one displaced, January holds a
// different value, so a lost fallback shows up as January's 20 rather than as a
// hole. Asserting "not zero" or "no error" would pass on the broken behaviour.
//
// covers: INV-BACKUP-16, INV-LIFECYCLE-04
func TestCompactedBackupPreservesTerminationUndo(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)
	alice := testutil.CreateHouseholdWithUser(t, q, "Alice")
	ctx := identity.WithUser(context.Background(), alice)
	assets := repo.NewAssetRepo(tdb.Pool)
	h := New(tdb.Pool, "http://test.local", &stubIssuer{}, &stubNotifier{}, false, DemoConfig{})

	var (
		jan    = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		feb    = time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
		closed = time.Date(2026, 2, 14, 0, 0, 0, 0, time.UTC)
		janAmt = decimal.RequireFromString("20000000")
		febAmt = decimal.RequireFromString("25000000")
	)

	bankAcc, err := assets.CreateBankAccount(ctx, repo.CreateBankAccountParams{
		DisplayName:     "Main checking",
		OwnershipType:   "sole",
		SoleOwnerUserID: &alice.ID,
		NativeCurrency:  "IDR",
		BankName:        "TestBank",
		AccountNumber:   "1234567890",
		AccountType:     "savings",
	})
	if err != nil {
		t.Fatalf("CreateBankAccount: %v", err)
	}
	assetID := bankAcc.Asset.ID

	for _, s := range []struct {
		month  time.Time
		amount decimal.Decimal
	}{{jan, janAmt}, {feb, febAmt}} {
		if _, err := assets.CreateAssetSnapshot(ctx, repo.CreateAssetSnapshotParams{
			AssetID:   assetID,
			YearMonth: s.month,
			Amount:    s.amount,
			Currency:  "IDR",
		}); err != nil {
			t.Fatalf("CreateAssetSnapshot %s: %v", s.month.Format("2006-01"), err)
		}
	}

	// Close the account mid-February: the 25M is displaced by a 0-value close row.
	if _, err := assets.UpdateAssetLifecycle(ctx, assetID, repo.LifecycleParams{
		Status: "closed", TerminatedAt: &closed,
	}); err != nil {
		t.Fatalf("terminate: %v", err)
	}

	live, err := q.GetAssetSnapshotAtMonth(context.Background(), db.GetAssetSnapshotAtMonthParams{
		AssetID: assetID, YearMonth: feb, HouseholdID: alice.HouseholdID,
	})
	if err != nil {
		t.Fatalf("read close snapshot: %v", err)
	}
	if !live.Amount.IsZero() {
		t.Fatalf("February snapshot after termination = %s, want the 0-value close row", live.Amount)
	}
	if live.Supersedes == nil {
		t.Fatal("close row does not name the snapshot it displaced")
	}
	displacedID := *live.Supersedes

	gzipped := compactedExportBytes(ctx, t, h)
	env, err := Parse(bytes.NewReader(gzipped))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if env.Fidelity != FidelityCompacted {
		t.Fatalf("fidelity = %q, want compacted", env.Fidelity)
	}

	t.Run("the compacted file carries the displaced row, and nothing else deleted", func(t *testing.T) {
		var carried, deleted int
		found := false
		for _, s := range env.Household.AssetSnapshots {
			if s.DeletedAt.Valid {
				deleted++
			}
			if s.ID == displacedID {
				found = true
				if !s.Amount.Equal(febAmt) {
					t.Errorf("displaced row amount = %s, want %s", s.Amount, febAmt)
				}
			}
			carried++
		}
		if !found {
			t.Errorf("displaced snapshot %s missing from the compacted file", displacedID)
		}
		// January + the February close row + the row it displaced. Nothing else:
		// "compacted" still means the user's own deletions are left behind.
		if carried != 3 {
			t.Errorf("asset_snapshots carried = %d, want 3", carried)
		}
		if deleted != 1 {
			t.Errorf("soft-deleted rows carried = %d, want 1 (only the displaced one)", deleted)
		}
	})

	t.Run("undo after restoring the compacted file returns the recorded value", func(t *testing.T) {
		if _, err := Validate(env, callerFrom(alice)); err != nil {
			t.Fatalf("Validate: %v", err)
		}
		if _, err := Commit(context.Background(), tdb.Pool, env, callerFrom(alice)); err != nil {
			t.Fatalf("Commit: %v", err)
		}

		// Reactivate: the close row goes, the displaced 25M comes back.
		if _, err := assets.UpdateAssetLifecycle(ctx, assetID, repo.LifecycleParams{
			Status: repo.StatusActive,
		}); err != nil {
			t.Fatalf("un-terminate: %v", err)
		}

		got, err := q.GetAssetSnapshotAtMonth(context.Background(), db.GetAssetSnapshotAtMonthParams{
			AssetID: assetID, YearMonth: feb, HouseholdID: alice.HouseholdID,
		})
		if err != nil {
			t.Fatalf("read February after undo: %v", err)
		}
		if !got.Amount.Equal(febAmt) {
			t.Errorf("February after undo = %s, want %s (%s would be January carried forward)",
				got.Amount, febAmt, janAmt)
		}
		if got.ID != displacedID {
			t.Errorf("February row id = %s, want the displaced row %s", got.ID, displacedID)
		}
	})
}
