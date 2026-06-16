package backup

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/kerti/balances-v2/backend/internal/auth"
	"github.com/kerti/balances-v2/backend/internal/db"
	"github.com/kerti/balances-v2/backend/internal/testutil"
)

func exportBytes(t *testing.T, h *Handlers, ctx context.Context) []byte {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/backup/export?fidelity=full", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	h.handleExport(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("export status = %d", rec.Code)
	}
	return rec.Body.Bytes()
}

func gunzip(t *testing.T, b []byte) []byte {
	t.Helper()
	gz, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		t.Fatalf("gzip reader: %v", err)
	}
	raw, err := io.ReadAll(gz)
	if err != nil {
		t.Fatalf("gunzip: %v", err)
	}
	return raw
}

// covers: INV-BACKUP-06, INV-BACKUP-07, INV-BACKUP-08
func TestRestoreParseValidate(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)
	alice := testutil.CreateHouseholdWithUser(t, q, "Alice")
	aliceCtx := auth.WithUser(context.Background(), alice)
	seedHousehold(aliceCtx, t, tdb.Pool, alice)
	h := New(tdb.Pool, "http://test.local")

	gzipped := exportBytes(t, h, aliceCtx)

	t.Run("gzip round-trip parses and a member validates", func(t *testing.T) {
		env, err := Parse(bytes.NewReader(gzipped))
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}
		if env.FormatVersion != FormatVersion {
			t.Errorf("format_version = %d", env.FormatVersion)
		}
		sum, err := Validate(env, alice.GoogleSub, alice.Email)
		if err != nil {
			t.Fatalf("Validate: %v", err)
		}
		if sum.Counts["asset_snapshots"] != 2 {
			t.Errorf("summary asset_snapshots = %d, want 2", sum.Counts["asset_snapshots"])
		}
		if sum.HouseholdName == "" {
			t.Error("summary household name empty")
		}
	})

	t.Run("plain JSON (no gzip) also parses", func(t *testing.T) {
		if _, err := Parse(bytes.NewReader(gunzip(t, gzipped))); err != nil {
			t.Fatalf("Parse plain: %v", err)
		}
	})

	t.Run("truncated gzip is corrupt", func(t *testing.T) {
		_, err := Parse(bytes.NewReader(gzipped[:len(gzipped)-5]))
		if !errors.Is(err, ErrCorruptBackup) {
			t.Errorf("err = %v, want ErrCorruptBackup", err)
		}
	})

	t.Run("non-member is refused", func(t *testing.T) {
		env, _ := Parse(bytes.NewReader(gzipped))
		_, err := Validate(env, "stranger-sub", "stranger@example.com")
		if !errors.Is(err, ErrNotMemberOfBackup) {
			t.Errorf("err = %v, want ErrNotMemberOfBackup", err)
		}
	})

	t.Run("email fallback matches membership", func(t *testing.T) {
		env, _ := Parse(bytes.NewReader(gzipped))
		if _, err := Validate(env, "different-sub", alice.Email); err != nil {
			t.Errorf("email fallback should match: %v", err)
		}
	})

	t.Run("dangling snapshot FK fails validation", func(t *testing.T) {
		env, _ := Parse(bytes.NewReader(gzipped))
		env.Household.AssetSnapshots = append(env.Household.AssetSnapshots, db.AssetSnapshot{
			ID:        uuid.New(),
			AssetID:   uuid.New(), // points at no asset in the payload
			YearMonth: time.Now(),
		})
		_, err := Validate(env, alice.GoogleSub, alice.Email)
		if !errors.Is(err, ErrValidationFailed) {
			t.Errorf("err = %v, want ErrValidationFailed", err)
		}
	})
}

// covers: INV-BACKUP-06
func TestMigrateGuards(t *testing.T) {
	t.Run("newer version refused", func(t *testing.T) {
		err := migrate(&Envelope{FormatVersion: FormatVersion + 1})
		if !errors.Is(err, ErrFormatTooNew) {
			t.Errorf("err = %v, want ErrFormatTooNew", err)
		}
	})
	t.Run("sub-1 version invalid", func(t *testing.T) {
		err := migrate(&Envelope{FormatVersion: 0})
		if !errors.Is(err, ErrInvalidBackupFile) {
			t.Errorf("err = %v, want ErrInvalidBackupFile", err)
		}
	})
	t.Run("current version passes", func(t *testing.T) {
		if err := migrate(&Envelope{FormatVersion: FormatVersion}); err != nil {
			t.Errorf("migrate current: %v", err)
		}
	})
}

// covers: INV-BACKUP-07
func TestAssertCountsDetectsTamper(t *testing.T) {
	env := &Envelope{
		FormatVersion: FormatVersion,
		Counts:        map[string]int{"assets": 5},
		Household:     HouseholdData{Assets: nil}, // actual 0, declared 5
	}
	if err := assertCounts(env); !errors.Is(err, ErrCorruptBackup) {
		t.Errorf("err = %v, want ErrCorruptBackup", err)
	}
}
