package backup

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kerti/balances-v2/backend/internal/auth"
	"github.com/kerti/balances-v2/backend/internal/db"
	"github.com/kerti/balances-v2/backend/internal/testutil"
)

// createLocalHouseholdWithUser mints a household with a single local-only member
// (no google_sub) plus a password credential — the shape a local-only self-host
// household has (ADR-0039). Returns the user row and the stored hash.
func createLocalHouseholdWithUser(t *testing.T, q *db.Queries, displayName, email, passwordHash string) db.User {
	t.Helper()
	ctx := context.Background()
	hh, err := q.CreateHousehold(ctx, db.CreateHouseholdParams{
		DisplayName:       displayName + "'s Household",
		ReportingCurrency: "IDR",
	})
	if err != nil {
		t.Fatalf("CreateHousehold(%s): %v", displayName, err)
	}
	u, err := q.CreateLocalUser(ctx, db.CreateLocalUserParams{
		HouseholdID: hh.ID,
		DisplayName: displayName,
		Email:       email,
		Locale:      "id-ID",
		TimeZone:    "Asia/Jakarta",
	})
	if err != nil {
		t.Fatalf("CreateLocalUser(%s): %v", displayName, err)
	}
	if _, err := q.UpsertLocalCredential(ctx, db.UpsertLocalCredentialParams{
		UserID:       u.ID,
		PasswordHash: passwordHash,
	}); err != nil {
		t.Fatalf("UpsertLocalCredential(%s): %v", displayName, err)
	}
	return u
}

// wipeInTx removes a household outright, simulating the old box being gone before
// a restore onto a fresh instance (the backup's original UUID must be free to
// land verbatim). Reuses the package wipe so it stays in lockstep with the schema.
func wipeInTx(t *testing.T, pool *pgxpool.Pool, hid uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin wipe tx: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := wipeHousehold(ctx, tx, hid); err != nil {
		t.Fatalf("wipeHousehold: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit wipe: %v", err)
	}
}

// covers: INV-AUTH-22
//
// The restore membership guard's null-sub branch (ADR-0039, #285). A local caller
// (no google_sub) is matched by email — including onto a Google-origin backup row
// (the blessed Google→local migration). A Google caller is still matched by
// google_sub only: email never gates a Google account, so a matching email with a
// different sub is refused.
func TestMatchCallerEmailBranch(t *testing.T) {
	gid, lid := uuid.New(), uuid.New()
	env := &Envelope{Household: HouseholdData{Users: []db.User{
		{ID: gid, Email: "google@example.com", GoogleSub: strptr("sub-google")},
		{ID: lid, Email: "local@example.com", GoogleSub: nil},
	}}}

	cases := []struct {
		name    string
		caller  Caller
		wantID  uuid.UUID
		wantsOK bool
	}{
		{"local caller matches a local row by email", Caller{Email: "local@example.com"}, lid, true},
		{"local caller matches a Google row by email (migration)", Caller{Email: "google@example.com"}, gid, true},
		{"local caller with unknown email is not a member", Caller{Email: "nobody@example.com"}, uuid.Nil, false},
		{"local caller with empty email is not a member", Caller{Email: ""}, uuid.Nil, false},
		{"google caller matches by sub", Caller{GoogleSub: strptr("sub-google")}, gid, true},
		{"google caller with a matching email but different sub is refused", Caller{GoogleSub: strptr("sub-other"), Email: "google@example.com"}, uuid.Nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := matchCaller(env, tc.caller)
			if ok != tc.wantsOK || got != tc.wantID {
				t.Errorf("matchCaller = (%v, %v), want (%v, %v)", got, ok, tc.wantID, tc.wantsOK)
			}
		})
	}
}

// covers: INV-AUTH-24
//
// checkStranding refuses a backup with local-only members (no google_sub) onto an
// instance with local auth disabled — those members could never sign in again
// (ADR-0039). When local auth is enabled, or the backup has no local members,
// there is nothing to strand.
func TestCheckStranding(t *testing.T) {
	withLocal := &Envelope{Household: HouseholdData{Users: []db.User{
		{ID: uuid.New(), GoogleSub: strptr("sub-a")},
		{ID: uuid.New(), GoogleSub: nil},
	}}}
	allGoogle := &Envelope{Household: HouseholdData{Users: []db.User{
		{ID: uuid.New(), GoogleSub: strptr("sub-a")},
	}}}

	if err := checkStranding(withLocal, false); !errors.Is(err, ErrStrandsLocalMembers) {
		t.Errorf("local members + local auth off: err = %v, want ErrStrandsLocalMembers", err)
	}
	if err := checkStranding(withLocal, true); err != nil {
		t.Errorf("local members + local auth on: err = %v, want nil", err)
	}
	if err := checkStranding(allGoogle, false); err != nil {
		t.Errorf("all-Google backup: err = %v, want nil", err)
	}
}

// covers: INV-AUTH-23
//
// The local-only round-trip (ADR-0039, #285): a backup from a local household
// restores onto a fresh instance whose bootstrap account shares the member's
// email. The bootstrap UUID is discarded and the backup's original UUID wins; the
// caller is re-bound to it; and the restorer's credential is carried DB-row→DB-row
// across the wipe (the file holds no hash) so a future login still resolves.
func TestRestoreLocalRoundTripCarriesCredential(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)

	const email = "restorer@example.com"
	const oldBoxHash = "argon2id$OLD-BOX-HASH"
	const newBoxHash = "argon2id$NEW-BOX-HASH"

	// The old box: a local household owned by the restorer, exported to a backup.
	orig := createLocalHouseholdWithUser(t, q, "Restorer", email, oldBoxHash)
	origCtx := auth.WithUser(context.Background(), orig)
	seedHousehold(origCtx, t, tdb.Pool, orig)
	h := New(tdb.Pool, "http://test.local", &stubIssuer{}, &stubNotifier{}, true, DemoConfig{})
	gzipped := exportBytes(origCtx, t, h)

	env, err := Parse(bytes.NewReader(gzipped))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	// The old box is gone: free its UUIDs so the backup can land verbatim.
	wipeInTx(t, tdb.Pool, orig.HouseholdID)

	// The fresh box: the operator bootstraps a new local account with the same
	// email and a new password — a different UUID from the backup's original.
	boot := createLocalHouseholdWithUser(t, q, "Restorer", email, newBoxHash)
	if boot.ID == orig.ID {
		t.Fatal("bootstrap UUID collided with the backup's — test cannot prove discard")
	}

	restoredID, err := Commit(context.Background(), tdb.Pool, env, callerFrom(boot))
	if err != nil {
		t.Fatalf("Commit: %v", err)
	}

	// The backup's original UUID won; the fresh bootstrap UUID was discarded.
	if restoredID != orig.ID {
		t.Errorf("restoredID = %v, want the backup's original %v (bootstrap UUID must be discarded)", restoredID, orig.ID)
	}

	// The credential was carried DB-row→DB-row against the restored UUID: it holds
	// the fresh-box hash (the bootstrap row's), never anything from the file.
	cred, err := q.GetLocalCredentialByUserID(context.Background(), orig.ID)
	if err != nil {
		t.Fatalf("restored credential missing: %v", err)
	}
	if cred.PasswordHash != newBoxHash {
		t.Errorf("carried hash = %q, want the bootstrap %q", cred.PasswordHash, newBoxHash)
	}

	// The bootstrap household is gone (wiped before the load).
	if _, err := q.GetLocalCredentialByUserID(context.Background(), boot.ID); err == nil {
		t.Error("bootstrap credential survived; the fresh bootstrap row should be wiped")
	}

	// The restored data is present and owned by the restored (original) user.
	after, err := h.buildEnvelope(context.Background(), orig.HouseholdID, FidelityFull)
	if err != nil {
		t.Fatalf("re-export: %v", err)
	}
	if len(after.Household.Users) != 1 || after.Household.Users[0].ID != orig.ID {
		t.Errorf("restored users = %+v, want the single original %v", after.Household.Users, orig.ID)
	}
	if after.Household.Users[0].GoogleSub != nil {
		t.Error("restored local user gained a google_sub — identity should stay local")
	}
}

// covers: INV-AUTH-25
//
// The backup file carries no credential secret (ADR-0039, #285): local_credentials
// is excluded exactly like sessions, so the export is unchanged and a hash never
// leaves the box in the file.
func TestBackupExcludesLocalCredential(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)

	const secret = "argon2id$THIS-MUST-NOT-LEAK"
	u := createLocalHouseholdWithUser(t, q, "Solo", "solo@example.com", secret)
	ctx := auth.WithUser(context.Background(), u)
	seedHousehold(ctx, t, tdb.Pool, u)
	h := New(tdb.Pool, "http://test.local", &stubIssuer{}, &stubNotifier{}, true, DemoConfig{})

	raw := gunzip(t, exportBytes(ctx, t, h))
	for _, needle := range []string{secret, "password_hash", "local_credential"} {
		if strings.Contains(string(raw), needle) {
			t.Errorf("backup file leaks %q", needle)
		}
	}

	// The identity (non-secret) still round-trips: the local user is in the file
	// with a null google_sub, so the format is unchanged.
	env, err := Parse(bytes.NewReader(exportBytes(ctx, t, h)))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	var found bool
	for _, user := range env.Household.Users {
		if user.ID == u.ID {
			found = true
			if user.GoogleSub != nil {
				t.Error("exported local user has a google_sub, want nil")
			}
		}
	}
	if !found {
		t.Error("local user missing from the backup")
	}
}

// covers: INV-AUTH-24
//
// The stranding guard fires through the HTTP endpoints, ahead of the membership
// guard: a local-member backup previewed/committed onto a Google-only instance is
// refused 422 RESTORE_STRANDS_LOCAL_MEMBERS (ADR-0039), so the operator is told to
// enable local auth rather than seeing a generic not-a-member error.
func TestRestoreStrandingEndpoint(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)

	// A local-member backup, built on a local-enabled instance.
	local := createLocalHouseholdWithUser(t, q, "Local", "local@example.com", "argon2id$H")
	localCtx := auth.WithUser(context.Background(), local)
	seedHousehold(localCtx, t, tdb.Pool, local)
	exporter := New(tdb.Pool, "http://test.local", &stubIssuer{}, &stubNotifier{}, true, DemoConfig{})
	gzipped := exportBytes(localCtx, t, exporter)

	// A Google-only instance (local auth disabled) refuses it. The caller is a
	// Google member of a different household — stranding is checked before
	// membership, so any authenticated caller trips it.
	bob := testutil.CreateHouseholdWithUser(t, q, "Bob")
	bobCtx := auth.WithUser(context.Background(), bob)
	googleOnly := New(tdb.Pool, "http://test.local", &stubIssuer{}, &stubNotifier{}, false, DemoConfig{})

	for _, ep := range []struct {
		name string
		fn   http.HandlerFunc
		path string
	}{
		{"preview", googleOnly.handleRestorePreview, "/api/backup/restore/preview"},
		{"commit", googleOnly.handleRestoreCommit, "/api/backup/restore/commit"},
	} {
		t.Run(ep.name, func(t *testing.T) {
			rec := postBackup(bobCtx, t, ep.fn, ep.path, gzipped)
			if rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status = %d, want 422; body=%s", rec.Code, rec.Body.String())
			}
			if !bytes.Contains(rec.Body.Bytes(), []byte("RESTORE_STRANDS_LOCAL_MEMBERS")) {
				t.Errorf("body = %s, want RESTORE_STRANDS_LOCAL_MEMBERS", rec.Body.String())
			}
		})
	}
}
