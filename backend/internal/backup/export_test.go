package backup

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/kerti/balances-v2/backend/internal/auth"
	"github.com/kerti/balances-v2/backend/internal/db"
	"github.com/kerti/balances-v2/backend/internal/repo"
	"github.com/kerti/balances-v2/backend/internal/testutil"
)

// seedHousehold creates, for the authenticated ctx, a bank account with a tag,
// two snapshots (one then soft-deleted), and one income event. Returns the live
// snapshot amount as a string for wire-format assertions.
func seedHousehold(ctx context.Context, t *testing.T, pool *pgxpool.Pool, owner db.User) string {
	t.Helper()
	assets := repo.NewAssetRepo(pool)
	tags := repo.NewTagRepo(pool)
	income := repo.NewIncomeRepo(pool)

	acct, err := assets.CreateBankAccount(ctx, repo.CreateBankAccountParams{
		DisplayName:     "Main checking",
		OwnershipType:   "sole",
		SoleOwnerUserID: &owner.ID,
		NativeCurrency:  "IDR",
		BankName:        "TestBank",
		AccountNumber:   "1234567890",
		AccountType:     "savings",
	})
	if err != nil {
		t.Fatalf("CreateBankAccount: %v", err)
	}

	tag, err := tags.CreateTag(ctx, "Emergency fund", "#22c55e")
	if err != nil {
		t.Fatalf("CreateTag: %v", err)
	}
	if err := tags.AssignTag(ctx, repo.TagGroupAsset, acct.Asset.ID, &tag.ID); err != nil {
		t.Fatalf("AssignTag: %v", err)
	}

	const liveAmount = "10000000"
	if _, err := assets.CreateAssetSnapshot(ctx, repo.CreateAssetSnapshotParams{
		AssetID:   acct.Asset.ID,
		YearMonth: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Amount:    decimal.RequireFromString(liveAmount),
		Currency:  "IDR",
	}); err != nil {
		t.Fatalf("CreateAssetSnapshot live: %v", err)
	}
	gone, err := assets.CreateAssetSnapshot(ctx, repo.CreateAssetSnapshotParams{
		AssetID:   acct.Asset.ID,
		YearMonth: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
		Amount:    decimal.RequireFromString("9000000"),
		Currency:  "IDR",
	})
	if err != nil {
		t.Fatalf("CreateAssetSnapshot deleted: %v", err)
	}
	if err := assets.DeleteAssetSnapshot(ctx, gone.ID); err != nil {
		t.Fatalf("DeleteAssetSnapshot: %v", err)
	}

	if _, err := income.CreateIncome(ctx, repo.CreateIncomeParams{
		Date:            time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC),
		Amount:          decimal.RequireFromString("5000000"),
		Currency:        "IDR",
		Category:        "salary",
		OwnershipType:   "sole",
		SoleOwnerUserID: &owner.ID,
		Regularity:      "routine",
	}); err != nil {
		t.Fatalf("CreateIncome: %v", err)
	}
	return liveAmount
}

// covers: INV-BACKUP-01, INV-BACKUP-02, INV-BACKUP-03, INV-BACKUP-04
func TestBackupExport(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)

	alice := testutil.CreateHouseholdWithUser(t, q, "Alice")
	bob := testutil.CreateHouseholdWithUser(t, q, "Bob")
	aliceCtx := auth.WithUser(context.Background(), alice)
	bobCtx := auth.WithUser(context.Background(), bob)

	liveAmount := seedHousehold(aliceCtx, t, tdb.Pool, alice)
	seedHousehold(bobCtx, t, tdb.Pool, bob) // cross-tenant noise

	h := New(tdb.Pool, "http://test.local", &stubIssuer{}, &stubNotifier{})

	t.Run("full fidelity carries soft-deleted rows", func(t *testing.T) {
		env, err := h.buildEnvelope(context.Background(), alice.HouseholdID, FidelityFull)
		if err != nil {
			t.Fatalf("buildEnvelope: %v", err)
		}
		if env.FormatVersion != FormatVersion {
			t.Errorf("format_version = %d, want %d", env.FormatVersion, FormatVersion)
		}
		if env.Fidelity != FidelityFull {
			t.Errorf("fidelity = %q, want full", env.Fidelity)
		}
		if env.Instance != "http://test.local" {
			t.Errorf("instance = %q", env.Instance)
		}
		if got := env.Counts["asset_snapshots"]; got != 2 {
			t.Errorf("asset_snapshots count = %d, want 2 (live + soft-deleted)", got)
		}
		if got := len(env.Household.Assets); got != 1 {
			t.Errorf("assets = %d, want 1", got)
		}
		if got := env.Counts["income"]; got != 1 {
			t.Errorf("income count = %d, want 1", got)
		}
	})

	t.Run("compacted drops soft-deleted rows", func(t *testing.T) {
		env, err := h.buildEnvelope(context.Background(), alice.HouseholdID, FidelityCompacted)
		if err != nil {
			t.Fatalf("buildEnvelope: %v", err)
		}
		if got := env.Counts["asset_snapshots"]; got != 1 {
			t.Errorf("asset_snapshots count = %d, want 1 (live only)", got)
		}
	})

	t.Run("tenancy: only the caller's household", func(t *testing.T) {
		env, err := h.buildEnvelope(context.Background(), alice.HouseholdID, FidelityFull)
		if err != nil {
			t.Fatalf("buildEnvelope: %v", err)
		}
		if env.Household.Household.ID != alice.HouseholdID {
			t.Errorf("household id mismatch")
		}
		for _, a := range env.Household.Assets {
			if a.HouseholdID != alice.HouseholdID {
				t.Errorf("leaked asset from household %s", a.HouseholdID)
			}
		}
		if got := len(env.Household.Users); got != 1 {
			t.Errorf("users = %d, want 1 (Bob excluded)", got)
		}
	})

	t.Run("HTTP: gzip stream, parents-before-children, decimals-as-strings", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/backup/export?fidelity=full", nil)
		req = req.WithContext(aliceCtx)
		rec := httptest.NewRecorder()
		h.handleExport(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d", rec.Code)
		}
		if cd := rec.Header().Get("Content-Disposition"); !strings.Contains(cd, ".json.gz") {
			t.Errorf("Content-Disposition = %q", cd)
		}

		gz, err := gzip.NewReader(bytes.NewReader(rec.Body.Bytes()))
		if err != nil {
			t.Fatalf("gzip reader: %v", err)
		}
		raw, err := io.ReadAll(gz)
		if err != nil {
			t.Fatalf("gunzip: %v", err)
		}
		body := string(raw)

		// Decimals must be JSON strings, never bare numbers (ADR-0011).
		if !strings.Contains(body, `"amount":"`+liveAmount+`"`) {
			t.Errorf("expected quoted decimal amount %q in payload", liveAmount)
		}
		// Parents-before-children section order (ADR-0036 contract). Match the
		// array form ("key":[) so this targets the payload sections, not the
		// alphabetically-sorted counts map (where "asset_snapshots":N sorts first).
		iAssets, iSnaps := strings.Index(body, `"assets":[`), strings.Index(body, `"asset_snapshots":[`)
		if iAssets < 0 || iSnaps < 0 || iAssets > iSnaps {
			t.Errorf("assets (%d) must precede asset_snapshots (%d)", iAssets, iSnaps)
		}

		var env Envelope
		if err := json.Unmarshal(raw, &env); err != nil {
			t.Fatalf("unmarshal round-trip: %v", err)
		}
		if env.Counts["asset_snapshots"] != 2 {
			t.Errorf("round-trip asset_snapshots = %d, want 2", env.Counts["asset_snapshots"])
		}
	})
}
