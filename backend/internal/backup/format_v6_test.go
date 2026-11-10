package backup

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/kerti/balances-v2/backend/internal/db"
	"github.com/kerti/balances-v2/backend/internal/identity"
	"github.com/kerti/balances-v2/backend/internal/testutil"
)

// covers: INV-BACKUP-06
//
// v5 → v6 (#790): users.carryover_date_mode was dropped. A current export must
// no longer write the key, and a v5 file whose users still carry it must keep
// restoring — the loader ignores the unknown key rather than failing on it.
// The v5 file is built by taking a real export and putting the key back, so the
// rest of the payload is exactly what this build emits.
func TestV5BackupWithCarryoverDateModeStillRestores(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)
	alice := testutil.CreateHouseholdWithUser(t, q, "Alice")
	ctx := identity.WithUser(context.Background(), alice)
	seedHousehold(ctx, t, tdb.Pool, alice)
	h := New(tdb.Pool, "http://test.local", &stubIssuer{}, &stubNotifier{}, false, DemoConfig{})

	var doc map[string]any
	dec := json.NewDecoder(bytes.NewReader(gunzip(t, exportBytes(ctx, t, h))))
	dec.UseNumber() // counts must round-trip as integers, not float64
	if err := dec.Decode(&doc); err != nil {
		t.Fatalf("decode export: %v", err)
	}
	if got, _ := doc["format_version"].(json.Number); got.String() != "6" {
		t.Fatalf("export format_version = %v, want 6", doc["format_version"])
	}
	users := doc["household"].(map[string]any)["users"].([]any)
	if len(users) == 0 {
		t.Fatal("export has no users")
	}
	for _, u := range users {
		row := u.(map[string]any)
		if _, has := row["carryover_date_mode"]; has {
			t.Fatal("a v6 export still writes users.carryover_date_mode")
		}
		row["carryover_date_mode"] = "end_of_last_month" // what a v5 build wrote
	}
	doc["format_version"] = 5
	v5, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal v5 file: %v", err)
	}

	env, err := Parse(bytes.NewReader(gzipBytes(t, v5)))
	if err != nil {
		t.Fatalf("Parse v5 file: %v", err)
	}
	if env.sourceFormatVersion != 5 || env.FormatVersion != FormatVersion {
		t.Errorf("versions = source %d / current %d, want 5 / %d",
			env.sourceFormatVersion, env.FormatVersion, FormatVersion)
	}
	if _, err := Validate(env, callerFrom(alice)); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if _, err := Commit(context.Background(), tdb.Pool, env, callerFrom(alice)); err != nil {
		t.Fatalf("Commit v5 file: %v", err)
	}

	var n int
	if err := tdb.Pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM users WHERE household_id = $1`, alice.HouseholdID).Scan(&n); err != nil {
		t.Fatalf("count users: %v", err)
	}
	if n != len(users) {
		t.Errorf("users after restore = %d, want %d", n, len(users))
	}
}

// A v6 file must be refused by a build that still speaks v5 — that is what the
// bump buys: the older build would otherwise restore the missing key as NULL into
// its NOT NULL column and fail halfway through.
func TestV6FileRefusedByV5Importer(t *testing.T) {
	err := migrate(&Envelope{FormatVersion: 6}, 5, transforms)
	if err == nil {
		t.Fatal("a v6 file was accepted by a v5 importer")
	}
}
