package backup

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/shopspring/decimal"

	"github.com/kerti/balances-v2/backend/internal/db"
	"github.com/kerti/balances-v2/backend/internal/identity"
	"github.com/kerti/balances-v2/backend/internal/testutil"
)

// This file holds the format-version transform-chain proof (#177, ADR-0036).
// Shipped product stays at FormatVersion 1 with an empty transform chain, so the
// "older file migrates into a newer importer" path has no production exercise
// yet. These tests stand in for it: a *synthetic* v1→v2 transform driven through
// the injectable parseWith seam proves the chain runs and the result still
// validates, and a frozen golden v1 fixture proves a real v1 backup keeps
// parsing — so a future format change can never silently break old backups.
//
// PROCESS COMMITMENT (ADR-0036): every future format change ships its N→N+1
// transform in `transforms` *and* a frozen golden vN fixture under
// testdata/golden/. To mint a new golden, run:
//
//	MINT_GOLDEN=1 go test ./internal/backup/ -run TestMintGoldenFixture
//
// then commit the written file. The minted file is frozen — never regenerate an
// existing golden to "fix" a format change; that defeats the guard. Add a new
// one alongside it.

const goldenDir = "testdata/golden"

// goldenSub is the membership subject baked into the minted golden fixture (the
// seeded "Alice" user's google_sub, see testutil.CreateHouseholdWithUser). Frozen
// alongside the fixture so the harness can prove the membership guard end-to-end.
const goldenSub = "test-sub-Alice"

// covers: INV-BACKUP-06
//
// The genuine "v1 file into a v2 system" proof. A synthetic v1→v2 transform is
// registered in a test-only chain and the importer target is bumped to 2; a real
// frozen v1 fixture is then parsed through that seam. The chain must run (version
// lands at 2, the transform's observable mutation is present) and the migrated
// graph must still validate — exactly what a real format upgrade has to deliver.
func TestSyntheticV1ToV2Transform(t *testing.T) {
	raw := readAnyGolden(t)

	const marker = " [migrated-v2]"
	chain := map[int]transformFunc{
		1: func(env *Envelope) error {
			// A representative in-place edit: a real transform would reshape the
			// payload to the v2 schema. Mutating the display name is observable yet
			// keeps the object graph valid, which is the property under test.
			env.Household.Household.DisplayName += marker
			return nil
		},
	}

	env, err := parseWith(bytes.NewReader(raw), 2, chain, maxDecompressedBackup)
	if err != nil {
		t.Fatalf("parseWith(target=2): %v", err)
	}
	if env.FormatVersion != 2 {
		t.Errorf("format_version after migrate = %d, want 2", env.FormatVersion)
	}
	if !strings.HasSuffix(env.Household.Household.DisplayName, marker) {
		t.Errorf("v1→v2 transform did not run; display name = %q", env.Household.Household.DisplayName)
	}
	// The migrated graph is still internally consistent and the baked-in member
	// still validates — a transform that corrupted references would fail here.
	if _, err := Validate(env, googleCaller(goldenSub)); err != nil {
		t.Fatalf("migrated graph failed validation: %v", err)
	}
}

// covers: INV-BACKUP-07, INV-BACKUP-11
//
// Fixture-locked backwards-compat harness: every historical golden fixture must
// still parse (decode + migrate to the current version + count-integrity) and
// validate against the live code. This is the regression net the process
// commitment buys — the day a format change breaks an old backup, a golden here
// goes red instead of a user's restore failing silently.
func TestGoldenFixturesStillParse(t *testing.T) {
	files := goldenFiles(t)
	if len(files) == 0 {
		t.Fatalf("no golden fixtures in %s — mint one with MINT_GOLDEN=1 (see TestMintGoldenFixture)", goldenDir)
	}
	for _, f := range files {
		t.Run(filepath.Base(f), func(t *testing.T) {
			raw, err := os.ReadFile(f)
			if err != nil {
				t.Fatalf("read golden: %v", err)
			}
			env, err := Parse(bytes.NewReader(raw))
			if err != nil {
				t.Fatalf("golden %s no longer parses: %v", f, err)
			}
			if err := validateGraph(env); err != nil {
				t.Fatalf("golden %s graph no longer valid: %v", f, err)
			}
		})
	}
}

// covers: INV-BONDS-04
//
// The first *real* format transform (v1→v2, #66): a v1 backup predates the
// bond_details.coupon_disposition column, so each bond entry decodes with an
// empty disposition. transforms[1] must backfill the column DEFAULT ('pays_out')
// — otherwise the empty value would restore as NULL into a NOT NULL column — and
// must leave an already-set disposition untouched.
func TestV1ToV2BackfillsCouponDisposition(t *testing.T) {
	env := &Envelope{
		Household: HouseholdData{
			Bonds: []db.BondDetail{
				{CouponDisposition: ""},        // a v1 entry: key absent → decodes to ""
				{CouponDisposition: "accrues"}, // a value the operator set: must survive
			},
		},
	}
	if err := transforms[1](env); err != nil {
		t.Fatalf("transforms[1]: %v", err)
	}
	if got := env.Household.Bonds[0].CouponDisposition; got != "pays_out" {
		t.Errorf("empty disposition backfilled to %q, want pays_out", got)
	}
	if got := env.Household.Bonds[1].CouponDisposition; got != "accrues" {
		t.Errorf("set disposition mutated to %q, want accrues", got)
	}
}

// The second real format transform (v2→v3, #412): a v2 backup predates the
// households.assumed_annual_inflation column, so the field decodes to the decimal
// zero value. transforms[2] must backfill the column DEFAULT (3.5) — otherwise it
// would restore as 0% (wrong) or trip the NOT NULL column — while leaving a value
// that was actually set on a v3 file untouched. (A v2 file never carries a real
// value here, so "is zero" unambiguously means "absent".)
func TestV2ToV3BackfillsAssumedInflation(t *testing.T) {
	absent := &Envelope{Household: HouseholdData{Household: db.Household{}}} // zero → absent
	if err := transforms[2](absent); err != nil {
		t.Fatalf("transforms[2]: %v", err)
	}
	if got := absent.Household.Household.AssumedAnnualInflation; !got.Equal(decimal.RequireFromString("3.5")) {
		t.Errorf("absent assumed_annual_inflation backfilled to %s, want 3.5", got)
	}

	set := &Envelope{Household: HouseholdData{
		Household: db.Household{AssumedAnnualInflation: decimal.RequireFromString("2")},
	}}
	if err := transforms[2](set); err != nil {
		t.Fatalf("transforms[2]: %v", err)
	}
	if got := set.Household.Household.AssumedAnnualInflation; !got.Equal(decimal.RequireFromString("2")) {
		t.Errorf("set assumed_annual_inflation mutated to %s, want 2", got)
	}
}

// covers: INV-FINANCE-36, INV-BACKUP-06
//
// The third real format transform (v3→v4, #594): a v3 backup predates the
// entry_type column on all four position tables, so every position decodes with
// an empty entry type. transforms[3] must backfill the column DEFAULT
// ('acquired') — otherwise it would restore as NULL into a NOT NULL column, and
// silently defaulting the other way would reintroduce the wrong month for every
// restored Position (ADR-0053). A declaration the household actually made must
// survive the restore untouched, which is the half that makes backup/restore a
// round-trip rather than a reset.
func TestV3ToV4BackfillsEntryType(t *testing.T) {
	env := &Envelope{
		Household: HouseholdData{
			Assets:      []db.Asset{{EntryType: ""}, {EntryType: "newly_tracked"}},
			Liabilities: []db.Liability{{EntryType: ""}, {EntryType: "newly_tracked"}},
			Receivables: []db.Receivable{{EntryType: ""}, {EntryType: "newly_tracked"}},
			Investments: []db.Investment{{EntryType: ""}, {EntryType: "newly_tracked"}},
		},
	}
	if err := transforms[3](env); err != nil {
		t.Fatalf("transforms[3]: %v", err)
	}
	got := [][2]string{
		{env.Household.Assets[0].EntryType, env.Household.Assets[1].EntryType},
		{env.Household.Liabilities[0].EntryType, env.Household.Liabilities[1].EntryType},
		{env.Household.Receivables[0].EntryType, env.Household.Receivables[1].EntryType},
		{env.Household.Investments[0].EntryType, env.Household.Investments[1].EntryType},
	}
	for i, pair := range got {
		if pair[0] != "acquired" {
			t.Errorf("group %d: absent entry_type backfilled to %q, want acquired", i, pair[0])
		}
		if pair[1] != "newly_tracked" {
			t.Errorf("group %d: declared entry_type mutated to %q, want newly_tracked", i, pair[1])
		}
	}
}

// covers: INV-BACKUP-06
//
// The restore preview must report the file's *on-disk* format version alongside
// the migrated one (#258), so a v1 fixture parsed on this (v2) build surfaces
// source=1, current=2 — the signal the UI turns into "made by an older version,
// updated automatically". migrate() rewrites FormatVersion in place, so this
// guards that the source is captured before that happens.
func TestPreviewReportsSourceFormatVersion(t *testing.T) {
	raw := readAnyGolden(t) // a frozen v1 fixture

	env, err := Parse(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	sum, err := Validate(env, googleCaller(goldenSub))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if sum.SourceFormatVersion != 1 {
		t.Errorf("source_format_version = %d, want 1 (the on-disk version)", sum.SourceFormatVersion)
	}
	if sum.FormatVersion != FormatVersion {
		t.Errorf("format_version = %d, want %d (migrated to current)", sum.FormatVersion, FormatVersion)
	}
	if sum.SourceFormatVersion >= sum.FormatVersion {
		t.Errorf("a v1 file on a v%d build must read older: source %d < current %d",
			FormatVersion, sum.SourceFormatVersion, sum.FormatVersion)
	}
}

// goldenFiles lists the frozen fixtures, skipping any minting artifacts.
func goldenFiles(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(goldenDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("read golden dir: %v", err)
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() || !strings.Contains(e.Name(), ".json") {
			continue
		}
		out = append(out, filepath.Join(goldenDir, e.Name()))
	}
	return out
}

// readAnyGolden returns the bytes of the v1 golden fixture (the first golden), so
// the transform proof runs against a real frozen file rather than a synthetic one.
func readAnyGolden(t *testing.T) []byte {
	t.Helper()
	files := goldenFiles(t)
	if len(files) == 0 {
		t.Skipf("no golden fixtures in %s — mint one with MINT_GOLDEN=1 (see TestMintGoldenFixture)", goldenDir)
	}
	raw, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	return raw
}

// TestMintGoldenFixture mints the frozen v1 golden from the live export encoder.
// It is gated behind MINT_GOLDEN so it never runs (or needs a DB) in CI — it is
// tooling, run by hand when intentionally adding a golden for a new format
// version. It seeds a realistic household, exports it full-fidelity, and writes
// the gzip artifact under testdata/golden/ for committing.
func TestMintGoldenFixture(t *testing.T) {
	if os.Getenv("MINT_GOLDEN") == "" {
		t.Skip("set MINT_GOLDEN=1 to (re)mint a golden fixture")
	}
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)
	alice := testutil.CreateHouseholdWithUser(t, q, "Alice")
	if derefStr(alice.GoogleSub) != goldenSub {
		t.Fatalf("seeded sub %q != goldenSub %q — update goldenSub", derefStr(alice.GoogleSub), goldenSub)
	}
	ctx := identity.WithUser(context.Background(), alice)
	seedHousehold(ctx, t, tdb.Pool, alice)
	h := New(tdb.Pool, "http://golden.local", &stubIssuer{}, &stubNotifier{}, false, DemoConfig{})

	gzipped := exportBytes(ctx, t, h)
	if err := os.MkdirAll(goldenDir, 0o755); err != nil {
		t.Fatalf("mkdir golden: %v", err)
	}
	path := filepath.Join(goldenDir, "v1_household.json.gz")
	if err := os.WriteFile(path, gzipped, 0o644); err != nil {
		t.Fatalf("write golden: %v", err)
	}
	t.Logf("minted golden v1 fixture: %s (%d bytes) — commit this file", path, len(gzipped))
}

// covers: INV-BACKUP-06, INV-BACKUP-16
//
// A v4 file predates `supersedes` (#602): its close rows and the rows they
// displaced are all there, but nothing says which pairs with which. The
// transform rebuilds that from the rule the column replaced — the displaced
// row's deleted_at equals the close row's created_at, both being the same
// transaction timestamp — so a backup taken before the fix still restores with
// its terminations undoable.
//
// The negative cases carry the weight: a row the *user* deleted (timestamps
// unrelated) must not be adopted as a fallback, and a zero-amount archived row
// is an earlier cycle's close row, which would restore a reactivated Position
// to 0 — the thing INV-LIFECYCLE-04 forbids.
func TestV4ToV5LinksDisplacedSnapshots(t *testing.T) {
	ts := func(s string) pgtype.Timestamptz {
		v, err := time.Parse(time.RFC3339Nano, s)
		if err != nil {
			t.Fatalf("parse %q: %v", s, err)
		}
		return pgtype.Timestamptz{Time: v, Valid: true}
	}
	var (
		asset     = uuid.New()
		feb       = time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
		mar       = time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
		flip      = "2026-02-20T10:00:00.123456Z"
		displace  = uuid.New()
		closeRow  = uuid.New()
		handGone  = uuid.New()
		marClose  = uuid.New()
		staleZero = uuid.New()
	)
	env := &Envelope{
		Household: HouseholdData{
			AssetSnapshots: []db.AssetSnapshot{
				// The pair: a 25 archived at the flip, and the 0 that replaced it.
				{ID: displace, AssetID: asset, YearMonth: feb, Amount: decimal.RequireFromString("25"),
					CreatedAt: ts("2026-02-01T00:00:00Z"), DeletedAt: ts(flip)},
				{ID: closeRow, AssetID: asset, YearMonth: feb, Amount: decimal.Zero, CreatedAt: ts(flip)},
				// A snapshot the user deleted by hand, months from any flip.
				{ID: handGone, AssetID: asset, YearMonth: mar, Amount: decimal.RequireFromString("30"),
					CreatedAt: ts("2026-03-01T00:00:00Z"), DeletedAt: ts("2026-06-09T08:00:00Z")},
				// A close row at that month whose created_at matches nothing.
				{ID: marClose, AssetID: asset, YearMonth: mar, Amount: decimal.Zero,
					CreatedAt: ts("2026-07-01T00:00:00Z")},
				// An earlier cycle's close row, archived at the same instant as
				// marClose was written: a timestamp match, but a 0 to restore.
				{ID: staleZero, AssetID: asset, YearMonth: mar, Amount: decimal.Zero,
					CreatedAt: ts("2026-05-01T00:00:00Z"), DeletedAt: ts("2026-07-01T00:00:00Z")},
			},
		},
	}
	if err := transforms[4](env); err != nil {
		t.Fatalf("transforms[4]: %v", err)
	}

	byID := map[uuid.UUID]db.AssetSnapshot{}
	for _, s := range env.Household.AssetSnapshots {
		byID[s.ID] = s
	}
	if got := byID[closeRow].Supersedes; got == nil || *got != displace {
		t.Errorf("close row supersedes = %v, want %s", got, displace)
	}
	if got := byID[marClose].Supersedes; got != nil {
		t.Errorf("a close row that displaced nothing linked to %s", *got)
	}
	if got := byID[displace].Supersedes; got != nil {
		t.Errorf("the displaced row itself gained a link to %s", *got)
	}
	if got := byID[handGone].Supersedes; got != nil {
		t.Errorf("a hand-deleted snapshot gained a link to %s", *got)
	}
}
