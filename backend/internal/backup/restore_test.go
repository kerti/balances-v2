package backup

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/kerti/balances-v2/backend/internal/auth"
	"github.com/kerti/balances-v2/backend/internal/db"
	"github.com/kerti/balances-v2/backend/internal/repo"
	"github.com/kerti/balances-v2/backend/internal/testutil"
)

// stubIssuer is a test double for SessionIssuer that records the user it was
// asked to re-sign-in and writes a marker cookie, so a test can assert the
// post-restore re-login happened. It also records whether the cookie was
// cleared instead — the erasure path (ADR-0040), which has no household left
// to re-issue a session against.
type stubIssuer struct {
	lastUserID uuid.UUID
	cleared    bool
}

func (s *stubIssuer) IssueSession(_ context.Context, w http.ResponseWriter, userID uuid.UUID, _ string) error {
	s.lastUserID = userID
	http.SetCookie(w, &http.Cookie{Name: "session", Value: "reissued", Path: "/"})
	return nil
}

func (s *stubIssuer) ClearSessionCookie(w http.ResponseWriter) {
	s.cleared = true
	http.SetCookie(w, &http.Cookie{Name: "session", Value: "", Path: "/", MaxAge: -1})
}

// stubNotifier is a no-op RestoreNotifier for tests that don't exercise the
// post-restore emails (the notifier's own behaviour is covered in the auth
// package). It records the household/restorer it was handed so a test can assert
// the commit handler fired it after a successful restore. It also records the
// erasure call (ADR-0040) — members are captured by the caller before the wipe,
// so the stub just echoes back whatever it was handed.
type stubNotifier struct {
	householdID uuid.UUID
	restorerID  uuid.UUID
	itemCount   int
	calls       int

	eraseMembers       []db.User
	eraseFounderID     uuid.UUID
	eraseHouseholdName string
	eraseCalls         int
}

func (s *stubNotifier) NotifyRestore(_ context.Context, householdID, restorerID uuid.UUID, itemCount int) {
	s.householdID, s.restorerID, s.itemCount, s.calls = householdID, restorerID, itemCount, s.calls+1
}

func (s *stubNotifier) NotifyErasure(_ context.Context, members []db.User, founderID uuid.UUID, householdName string) {
	s.eraseMembers, s.eraseFounderID, s.eraseHouseholdName = members, founderID, householdName
	s.eraseCalls++
}

func exportBytes(ctx context.Context, t *testing.T, h *Handlers) []byte {
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

// gzipBytes gzip-compresses b, for tests that need to hand-build a gzip stream
// smaller than a real backup export.
func gzipBytes(t *testing.T, b []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	if _, err := gw.Write(b); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := gw.Close(); err != nil {
		t.Fatalf("close gzip writer: %v", err)
	}
	return buf.Bytes()
}

// covers: INV-BACKUP-06, INV-BACKUP-07, INV-BACKUP-08
func TestRestoreParseValidate(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)
	alice := testutil.CreateHouseholdWithUser(t, q, "Alice")
	aliceCtx := auth.WithUser(context.Background(), alice)
	seedHousehold(aliceCtx, t, tdb.Pool, alice)
	h := New(tdb.Pool, "http://test.local", &stubIssuer{}, &stubNotifier{}, false, DemoConfig{})

	gzipped := exportBytes(aliceCtx, t, h)

	t.Run("gzip round-trip parses and a member validates", func(t *testing.T) {
		env, err := Parse(bytes.NewReader(gzipped))
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}
		if env.FormatVersion != FormatVersion {
			t.Errorf("format_version = %d", env.FormatVersion)
		}
		sum, err := Validate(env, callerFrom(alice))
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

	// covers: INV-BACKUP-07
	t.Run("gzip bomb past the decompressed ceiling is corrupt, not OOM", func(t *testing.T) {
		// A real 500 MB ceiling isn't worth exercising literally per test run —
		// inject a tiny one via the same seam parseWith already uses for
		// target/chain, and prove a payload one byte over it is refused.
		payload := bytes.Repeat([]byte("x"), 2048)
		bomb := gzipBytes(t, payload)
		_, err := parseWith(bytes.NewReader(bomb), FormatVersion, transforms, 1024)
		if !errors.Is(err, ErrCorruptBackup) {
			t.Errorf("err = %v, want ErrCorruptBackup", err)
		}
	})

	t.Run("non-member is refused", func(t *testing.T) {
		env, _ := Parse(bytes.NewReader(gzipped))
		_, err := Validate(env, googleCaller("stranger-sub"))
		if !errors.Is(err, ErrNotMemberOfBackup) {
			t.Errorf("err = %v, want ErrNotMemberOfBackup", err)
		}
	})

	t.Run("a matching email is not enough for a Google caller — membership is google_sub only", func(t *testing.T) {
		env, _ := Parse(bytes.NewReader(gzipped))
		// A Google caller (sub set) with a different subject but Alice's email must
		// be refused: for a Google account, email is mutable/reassignable and can't
		// gate a destructive restore (the email branch is scoped to null-sub callers).
		caller := Caller{GoogleSub: strptr("different-sub"), Email: alice.Email}
		if _, err := Validate(env, caller); !errors.Is(err, ErrNotMemberOfBackup) {
			t.Errorf("err = %v, want ErrNotMemberOfBackup (email must not match for a Google caller)", err)
		}
	})

	t.Run("dangling snapshot FK fails validation", func(t *testing.T) {
		env, _ := Parse(bytes.NewReader(gzipped))
		env.Household.AssetSnapshots = append(env.Household.AssetSnapshots, db.AssetSnapshot{
			ID:        uuid.New(),
			AssetID:   uuid.New(), // points at no asset in the payload
			YearMonth: time.Now(),
		})
		_, err := Validate(env, callerFrom(alice))
		if !errors.Is(err, ErrValidationFailed) {
			t.Errorf("err = %v, want ErrValidationFailed", err)
		}
	})
}

// covers: INV-BACKUP-09, INV-BACKUP-10
func TestRestoreCommit(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)
	alice := testutil.CreateHouseholdWithUser(t, q, "Alice")
	bob := testutil.CreateHouseholdWithUser(t, q, "Bob")
	aliceCtx := auth.WithUser(context.Background(), alice)
	bobCtx := auth.WithUser(context.Background(), bob)
	seedHousehold(aliceCtx, t, tdb.Pool, alice)
	seedHousehold(bobCtx, t, tdb.Pool, bob) // cross-tenant noise — must survive untouched
	h := New(tdb.Pool, "http://test.local", &stubIssuer{}, &stubNotifier{}, false, DemoConfig{})

	// The golden backup, captured before any mutation.
	before, err := h.buildEnvelope(context.Background(), alice.HouseholdID, FidelityFull)
	if err != nil {
		t.Fatalf("export before: %v", err)
	}
	bobBefore, err := h.buildEnvelope(context.Background(), bob.HouseholdID, FidelityFull)
	if err != nil {
		t.Fatalf("export bob: %v", err)
	}
	gzipped := exportBytes(aliceCtx, t, h)

	t.Run("export -> commit -> re-export is verbatim", func(t *testing.T) {
		// Mutate Alice's live data so the restore has something to overwrite.
		assets := repo.NewAssetRepo(tdb.Pool)
		if _, err := assets.CreateBankAccount(aliceCtx, repo.CreateBankAccountParams{
			DisplayName:     "Scratch",
			OwnershipType:   "sole",
			SoleOwnerUserID: &alice.ID,
			NativeCurrency:  "IDR",
			BankName:        "X",
			AccountNumber:   "9",
			AccountType:     "savings",
		}); err != nil {
			t.Fatalf("mutate: %v", err)
		}

		env, err := Parse(bytes.NewReader(gzipped))
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}
		if _, err := Validate(env, callerFrom(alice)); err != nil {
			t.Fatalf("Validate: %v", err)
		}
		if _, err := Commit(context.Background(), tdb.Pool, env, callerFrom(alice)); err != nil {
			t.Fatalf("Commit: %v", err)
		}

		after, err := h.buildEnvelope(context.Background(), alice.HouseholdID, FidelityFull)
		if err != nil {
			t.Fatalf("export after: %v", err)
		}
		// Every section count matches the golden — the scratch account is gone and
		// the soft-deleted snapshot came back (full-fidelity verbatim round-trip).
		for name, want := range before.Counts {
			if got := after.Counts[name]; got != want {
				t.Errorf("section %q count = %d after restore, want %d", name, got, want)
			}
		}
		if after.Household.Household.DisplayName != before.Household.Household.DisplayName {
			t.Errorf("household name = %q, want %q",
				after.Household.Household.DisplayName, before.Household.Household.DisplayName)
		}
		if got := after.Counts["asset_snapshots"]; got != 2 {
			t.Errorf("asset_snapshots = %d, want 2 (live + soft-deleted restored)", got)
		}
	})

	t.Run("a different household is untouched", func(t *testing.T) {
		bobAfter, err := h.buildEnvelope(context.Background(), bob.HouseholdID, FidelityFull)
		if err != nil {
			t.Fatalf("export bob after: %v", err)
		}
		for name, want := range bobBefore.Counts {
			if got := bobAfter.Counts[name]; got != want {
				t.Errorf("bob section %q count = %d, want %d (cross-tenant bleed)", name, got, want)
			}
		}
	})

	t.Run("a failed load rolls back, leaving the caller's data intact", func(t *testing.T) {
		env, err := Parse(bytes.NewReader(gzipped))
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}
		// Poison a row in a way validateGraph does not police (created_by is not
		// checked): the FK insert fails mid-load, after the wipe has already run.
		bad := uuid.New()
		env.Household.AssetSnapshots[0].CreatedBy = &bad
		if _, err := Commit(context.Background(), tdb.Pool, env, callerFrom(alice)); err == nil {
			t.Fatal("Commit succeeded, want a foreign-key failure")
		}

		after, err := h.buildEnvelope(context.Background(), alice.HouseholdID, FidelityFull)
		if err != nil {
			t.Fatalf("export after rollback: %v", err)
		}
		if got := len(after.Household.Assets); got != 1 {
			t.Errorf("assets after rollback = %d, want 1 (wipe must have rolled back)", got)
		}
		if got := after.Counts["asset_snapshots"]; got != 2 {
			t.Errorf("asset_snapshots after rollback = %d, want 2", got)
		}
	})
}

// googleCaller builds a Caller for a Google-authenticated user with the given
// subject — the identity the membership guard matches by google_sub.
func googleCaller(sub string) Caller {
	return Caller{GoogleSub: strptr(sub)}
}

// strptr returns a pointer to s — for building a nullable google_sub in a Caller.
func strptr(s string) *string { return &s }

// derefStr returns the pointed-to string, or "" when nil — a test convenience
// for comparing a nullable google_sub (ADR-0039).
func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// multipartBackup wraps raw backup bytes as a multipart body with a "file"
// field, returning the body and its Content-Type header.
func multipartBackup(t *testing.T, raw []byte) (*bytes.Buffer, string) {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	part, err := mw.CreateFormFile("file", "household-backup.json.gz")
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	if _, err := part.Write(raw); err != nil {
		t.Fatalf("write part: %v", err)
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	return &body, mw.FormDataContentType()
}

// postBackup builds and serves a multipart restore request to the given handler.
func postBackup(ctx context.Context, t *testing.T, fn http.HandlerFunc, path string, raw []byte) *httptest.ResponseRecorder {
	t.Helper()
	body, ct := multipartBackup(t, raw)
	req := httptest.NewRequest(http.MethodPost, path, body).WithContext(ctx)
	req.Header.Set("Content-Type", ct)
	rec := httptest.NewRecorder()
	fn(rec, req)
	return rec
}

// covers: INV-BACKUP-08, INV-BACKUP-09, INV-BACKUP-10
func TestRestoreEndpoints(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)
	alice := testutil.CreateHouseholdWithUser(t, q, "Alice")
	bob := testutil.CreateHouseholdWithUser(t, q, "Bob")
	aliceCtx := auth.WithUser(context.Background(), alice)
	bobCtx := auth.WithUser(context.Background(), bob)
	seedHousehold(aliceCtx, t, tdb.Pool, alice)
	notifier := &stubNotifier{}
	h := New(tdb.Pool, "http://test.local", &stubIssuer{}, notifier, false, DemoConfig{})
	gzipped := exportBytes(aliceCtx, t, h)

	t.Run("preview returns the stakes summary", func(t *testing.T) {
		rec := postBackup(aliceCtx, t, h.handleRestorePreview, "/api/backup/restore/preview", gzipped)
		if rec.Code != http.StatusOK {
			t.Fatalf("preview status = %d, body=%s", rec.Code, rec.Body.String())
		}
		var prev RestorePreview
		if err := json.Unmarshal(rec.Body.Bytes(), &prev); err != nil {
			t.Fatalf("decode preview: %v", err)
		}
		if prev.Backup.Counts["asset_snapshots"] != 2 {
			t.Errorf("backup asset_snapshots = %d, want 2", prev.Backup.Counts["asset_snapshots"])
		}
		if prev.Backup.HouseholdName == "" {
			t.Error("backup household name empty")
		}
		// Current stakes: Alice's seeded household is non-empty (one asset).
		if prev.Current["assets"] != 1 {
			t.Errorf("current assets = %d, want 1 (stakes)", prev.Current["assets"])
		}
	})

	t.Run("preview refuses a non-member with 403", func(t *testing.T) {
		rec := postBackup(bobCtx, t, h.handleRestorePreview, "/api/backup/restore/preview", gzipped)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403", rec.Code)
		}
		if !bytes.Contains(rec.Body.Bytes(), []byte("NOT_A_MEMBER_OF_BACKUP")) {
			t.Errorf("body = %s, want NOT_A_MEMBER_OF_BACKUP", rec.Body.String())
		}
	})

	t.Run("preview rejects a corrupt file with 400", func(t *testing.T) {
		rec := postBackup(aliceCtx, t, h.handleRestorePreview, "/api/backup/restore/preview", gzipped[:len(gzipped)-5])
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", rec.Code)
		}
		if !bytes.Contains(rec.Body.Bytes(), []byte("CORRUPT_BACKUP")) {
			t.Errorf("body = %s, want CORRUPT_BACKUP", rec.Body.String())
		}
	})

	t.Run("commit restores and a non-member cannot", func(t *testing.T) {
		// Non-member commit is refused before any wipe.
		rec := postBackup(bobCtx, t, h.handleRestoreCommit, "/api/backup/restore/commit", gzipped)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("non-member commit status = %d, want 403", rec.Code)
		}

		// Member commit performs the restore.
		rec = postBackup(aliceCtx, t, h.handleRestoreCommit, "/api/backup/restore/commit", gzipped)
		if rec.Code != http.StatusOK {
			t.Fatalf("commit status = %d, body=%s", rec.Code, rec.Body.String())
		}
		var res restoreResult
		if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
			t.Fatalf("decode result: %v", err)
		}
		if !res.Restored {
			t.Error("restored = false")
		}
		// The caller is kept signed in: a fresh session cookie rides the response
		// rather than dumping them on the sign-in screen.
		var reissued bool
		for _, c := range rec.Result().Cookies() {
			if c.Name == "session" && c.Value != "" {
				reissued = true
			}
		}
		if !reissued {
			t.Error("expected a re-issued session cookie after restore")
		}
		after, err := h.buildEnvelope(context.Background(), alice.HouseholdID, FidelityFull)
		if err != nil {
			t.Fatalf("export after: %v", err)
		}
		if got := after.Counts["asset_snapshots"]; got != 2 {
			t.Errorf("asset_snapshots after commit = %d, want 2", got)
		}
		// The successful commit fired the post-restore notifier exactly once, for
		// the restored caller's adopted household. The refused non-member commit
		// above must not have fired it.
		if notifier.calls != 1 {
			t.Errorf("NotifyRestore calls = %d, want 1 (success only)", notifier.calls)
		}
		if notifier.householdID != alice.HouseholdID {
			t.Errorf("NotifyRestore household = %s, want restored household %s", notifier.householdID, alice.HouseholdID)
		}
		if notifier.itemCount <= 0 {
			t.Errorf("NotifyRestore itemCount = %d, want > 0", notifier.itemCount)
		}
	})
}

func TestSummaryItemCount(t *testing.T) {
	if got := summaryItemCount(nil); got != 0 {
		t.Errorf("nil summary = %d, want 0", got)
	}
	s := &Summary{Counts: map[string]int{"assets": 3, "investments": 2, "income": 5}}
	if got := summaryItemCount(s); got != 10 {
		t.Errorf("count = %d, want 10", got)
	}
}

// covers: INV-BACKUP-06
func TestMigrateGuards(t *testing.T) {
	t.Run("newer version refused", func(t *testing.T) {
		err := migrate(&Envelope{FormatVersion: FormatVersion + 1}, FormatVersion, transforms)
		if !errors.Is(err, ErrFormatTooNew) {
			t.Errorf("err = %v, want ErrFormatTooNew", err)
		}
	})
	t.Run("sub-1 version invalid", func(t *testing.T) {
		err := migrate(&Envelope{FormatVersion: 0}, FormatVersion, transforms)
		if !errors.Is(err, ErrInvalidBackupFile) {
			t.Errorf("err = %v, want ErrInvalidBackupFile", err)
		}
	})
	t.Run("current version passes", func(t *testing.T) {
		if err := migrate(&Envelope{FormatVersion: FormatVersion}, FormatVersion, transforms); err != nil {
			t.Errorf("migrate current: %v", err)
		}
	})
	t.Run("a gap in the chain is refused, not silently skipped", func(t *testing.T) {
		// Targeting v3 with only a v1→v2 transform registered must fail at the
		// missing v2→v3 hop rather than load a half-migrated file.
		chain := map[int]transformFunc{1: func(*Envelope) error { return nil }}
		err := migrate(&Envelope{FormatVersion: 1}, 3, chain)
		if !errors.Is(err, ErrValidationFailed) {
			t.Errorf("err = %v, want ErrValidationFailed (missing v2→v3)", err)
		}
	})
	t.Run("a transform that errors aborts the migration", func(t *testing.T) {
		boom := errors.New("boom")
		chain := map[int]transformFunc{1: func(*Envelope) error { return boom }}
		err := migrate(&Envelope{FormatVersion: 1}, 2, chain)
		if !errors.Is(err, ErrValidationFailed) {
			t.Errorf("err = %v, want ErrValidationFailed wrapping the transform error", err)
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
