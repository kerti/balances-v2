package backup

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/google/uuid"

	"github.com/kerti/balances-v2/backend/internal/db"
)

// Restore-side sentinel errors. The HTTP layer (a later increment) maps each to
// an ADR-0027 error code; the UI copy lives in the frontend catalog (ADR-0036
// Presentation): INVALID_BACKUP_FILE, CORRUPT_BACKUP, BACKUP_FORMAT_TOO_NEW,
// NOT_A_MEMBER_OF_BACKUP, BACKUP_VALIDATION_FAILED.
var (
	// ErrInvalidBackupFile — not a recognizable backup (unparseable JSON, or a
	// version below 1).
	ErrInvalidBackupFile = errors.New("invalid backup file")
	// ErrCorruptBackup — the gzip stream is damaged/truncated (CRC) or a declared
	// section count doesn't match the payload.
	ErrCorruptBackup = errors.New("corrupt backup")
	// ErrFormatTooNew — the file's format_version is newer than this build speaks;
	// refuse rather than guess (ADR-0036).
	ErrFormatTooNew = errors.New("backup format too new")
	// ErrNotMemberOfBackup — the caller is not a member of the backup's household.
	ErrNotMemberOfBackup = errors.New("not a member of the backup household")
	// ErrValidationFailed — the object graph is internally inconsistent (a dangling
	// foreign key, a row in the wrong household).
	ErrValidationFailed = errors.New("backup validation failed")
	// ErrStrandsLocalMembers — the backup carries local-only members (no
	// google_sub) but this instance has local auth disabled, so restoring would
	// leave them permanently unable to sign in (no Google, no way to set a
	// password). Refuse rather than silently strand them; the operator is told to
	// enable AUTH_LOCAL_ENABLED (ADR-0039).
	ErrStrandsLocalMembers = errors.New("restore would strand local members")
)

// transformFunc migrates an envelope from format_version N to N+1, in place.
type transformFunc func(*Envelope) error

// transforms is the format-version migration chain (ADR-0036): key N migrates
// N→N+1.
//
// transforms[1] (v1→v2, #66) backfills bond_details.coupon_disposition: a v1 file
// predates the column, so each bond entry is missing the key (decodes to "") and
// would otherwise restore as NULL into a NOT NULL column. Defaulting to
// 'pays_out' reproduces the column DEFAULT and pre-#66 behaviour.
var transforms = map[int]transformFunc{
	1: func(e *Envelope) error {
		for i := range e.Household.Bonds {
			if e.Household.Bonds[i].CouponDisposition == "" {
				e.Household.Bonds[i].CouponDisposition = "pays_out"
			}
		}
		return nil
	},
}

// Parse reads a backup artifact (gzip or plain JSON), decodes the envelope,
// migrates it to the current format_version, and verifies integrity. It does not
// touch the database — preview and commit both start here.
func Parse(r io.Reader) (*Envelope, error) {
	return parseWith(r, FormatVersion, transforms)
}

// parseWith is Parse with the target format version and transform chain injected
// rather than read from the package globals. Product code always parses against
// the build's FormatVersion + the real (empty-at-v1) transforms via Parse; the
// seam exists so the test suite can prove an older file migrates into a *newer*
// importer — the genuine "v1 file into a v2 system" proof (#177) — without
// shipping a synthetic v2 in product code.
func parseWith(r io.Reader, target int, chain map[int]transformFunc) (*Envelope, error) {
	br := bufio.NewReader(r)
	gzipped := false
	if magic, _ := br.Peek(2); len(magic) == 2 && magic[0] == 0x1f && magic[1] == 0x8b {
		gzipped = true
	}

	var raw []byte
	var err error
	if gzipped {
		gz, e := gzip.NewReader(br)
		if e != nil {
			return nil, fmt.Errorf("%w: %v", ErrCorruptBackup, e)
		}
		defer func() { _ = gz.Close() }()
		// ReadAll forces the gzip trailer (CRC + length) to be verified, so a
		// truncated/corrupt file surfaces here rather than as silent short data.
		if raw, err = io.ReadAll(gz); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrCorruptBackup, err)
		}
	} else {
		if raw, err = io.ReadAll(br); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidBackupFile, err)
		}
	}

	var env Envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidBackupFile, err)
	}
	// Remember the on-disk version before migrate() rewrites FormatVersion to the
	// current one, so the preview can tell the operator their file was upgraded (#258).
	env.sourceFormatVersion = env.FormatVersion
	if err := migrate(&env, target, chain); err != nil {
		return nil, err
	}
	if err := assertCounts(&env); err != nil {
		return nil, err
	}
	return &env, nil
}

// migrate runs the transform chain from the file's format_version up to target,
// refusing a newer (> target) or sub-1 version (ADR-0036). target is the
// importer's format version (FormatVersion in production) and chain is the
// registry of N→N+1 transforms; both are passed in so the test suite can drive
// a synthetic target/chain without touching the product globals.
func migrate(env *Envelope, target int, chain map[int]transformFunc) error {
	if env.FormatVersion < 1 {
		return fmt.Errorf("%w: format_version %d", ErrInvalidBackupFile, env.FormatVersion)
	}
	if env.FormatVersion > target {
		return fmt.Errorf("%w: file is v%d, this app speaks v%d", ErrFormatTooNew, env.FormatVersion, target)
	}
	for v := env.FormatVersion; v < target; v++ {
		fn, ok := chain[v]
		if !ok {
			return fmt.Errorf("%w: no transform for v%d→v%d", ErrValidationFailed, v, v+1)
		}
		if err := fn(env); err != nil {
			return fmt.Errorf("%w: v%d→v%d: %v", ErrValidationFailed, v, v+1, err)
		}
		env.FormatVersion = v + 1
	}
	return nil
}

// assertCounts checks every declared per-section count against the actual
// payload (ADR-0036 integrity guard). A nil/absent Counts map is permitted (a
// hand-made or pre-counts file) and simply skips the check.
func assertCounts(env *Envelope) error {
	actual := env.Household.SectionCounts()
	for name, want := range env.Counts {
		if got := actual[name]; got != want {
			return fmt.Errorf("%w: section %q declared %d, found %d", ErrCorruptBackup, name, want, got)
		}
	}
	return nil
}

// Summary is the non-destructive preview returned before a restore commits — the
// counts erased/loaded and the household name drive the confirmation screen.
type Summary struct {
	HouseholdName string `json:"household_name"`
	// FormatVersion is the version after migration (the version this build now
	// holds the data at); SourceFormatVersion is the file's on-disk version. When
	// Source < FormatVersion the file was made by an older build and upgraded on
	// the way in (#258) — the preview surfaces a reassurance note.
	FormatVersion       int            `json:"format_version"`
	SourceFormatVersion int            `json:"source_format_version"`
	Fidelity            Fidelity       `json:"fidelity"`
	Counts              map[string]int `json:"counts"`
}

// Caller is the live-authenticated user attempting a restore, reduced to the
// identity the membership guard and post-restore continuity need. GoogleSub is
// nil for a local (password) account (ADR-0039); Email is the match key for such
// a caller. HouseholdID is the caller's current household — the one the wipe
// empties — and UserID is their current (bootstrap) row, the source of the
// carried local credential.
type Caller struct {
	HouseholdID uuid.UUID
	UserID      uuid.UUID
	GoogleSub   *string
	Email       string
}

// Validate runs the full pre-commit checks: the object graph is internally
// consistent, and the caller is a member of the backup's household (the security
// guard — you can only restore a household you belong to). Returns the preview
// Summary on success.
func Validate(env *Envelope, caller Caller) (*Summary, error) {
	if err := validateGraph(env); err != nil {
		return nil, err
	}
	if _, ok := matchCaller(env, caller); !ok {
		return nil, ErrNotMemberOfBackup
	}
	return &Summary{
		HouseholdName:       env.Household.Household.DisplayName,
		FormatVersion:       env.FormatVersion,
		SourceFormatVersion: env.sourceFormatVersion,
		Fidelity:            env.Fidelity,
		Counts:              env.Household.SectionCounts(),
	}, nil
}

// matchCaller finds the backup User row that IS the caller and returns its
// backup UUID (the restored user's original id) — the membership guard
// (ADR-0036/ADR-0039). A Google caller (GoogleSub set) matches by google_sub
// only: it is Google's immutable subject id, whereas email is mutable and can be
// reassigned, so trusting it for a Google account would be a (narrow)
// impersonation hole. A local caller (GoogleSub nil, ADR-0039) has no sub, so
// matches by email — the null-sub branch ADR-0036 foresaw; a Google-origin row
// is a valid target (the blessed Google→local migration path). Confidentiality
// still rests on possession of the backup file, not on the email match.
func matchCaller(env *Envelope, caller Caller) (uuid.UUID, bool) {
	for _, u := range env.Household.Users {
		if caller.GoogleSub != nil {
			if u.GoogleSub != nil && *u.GoogleSub == *caller.GoogleSub {
				return u.ID, true
			}
			continue
		}
		if caller.Email != "" && u.Email == caller.Email {
			return u.ID, true
		}
	}
	return uuid.Nil, false
}

// checkStranding refuses a backup that would leave local-only members unable to
// ever sign in (ADR-0039). A member with no google_sub can only authenticate via
// a local password; on an instance with local auth disabled they have no Google
// identity and no way to set one, so a restore would strand them permanently. We
// guard rather than silently proceed — the operator is directed to enable local
// auth. When local auth is enabled (or the backup has no local members), there
// is nothing to strand.
func checkStranding(env *Envelope, authLocalEnabled bool) error {
	if authLocalEnabled {
		return nil
	}
	for _, u := range env.Household.Users {
		if u.GoogleSub == nil {
			return ErrStrandsLocalMembers
		}
	}
	return nil
}

// validateGraph checks referential integrity across the payload: every position
// belongs to the backup's household, every owner/tag reference resolves to a
// user/tag in the payload, and every detail/snapshot/ledger row points at a
// position present in the payload. A future streaming importer would do the same
// checks against resident parent-id sets (ADR-0036).
func validateGraph(env *Envelope) error {
	h := &env.Household
	hid := h.Household.ID

	users := idSet(h.Users, func(u db.User) uuid.UUID { return u.ID })
	tags := idSet(h.Tags, func(t db.Tag) uuid.UUID { return t.ID })

	for _, u := range h.Users {
		if u.HouseholdID != hid {
			return wrongHousehold("user", u.ID)
		}
	}

	assets := idSet(h.Assets, func(a db.Asset) uuid.UUID { return a.ID })
	investments := idSet(h.Investments, func(i db.Investment) uuid.UUID { return i.ID })
	liabilities := idSet(h.Liabilities, func(l db.Liability) uuid.UUID { return l.ID })
	receivables := idSet(h.Receivables, func(r db.Receivable) uuid.UUID { return r.ID })

	// Positions: in-household, owner+tag resolvable.
	for _, a := range h.Assets {
		if a.HouseholdID != hid {
			return wrongHousehold("asset", a.ID)
		}
		if !optIn(users, a.SoleOwnerUserID) {
			return danglingRef("asset", a.ID, "sole_owner_user_id")
		}
		if !optIn(tags, a.TagID) {
			return danglingRef("asset", a.ID, "tag_id")
		}
	}
	for _, i := range h.Investments {
		if i.HouseholdID != hid {
			return wrongHousehold("investment", i.ID)
		}
		if !optIn(users, i.SoleOwnerUserID) {
			return danglingRef("investment", i.ID, "sole_owner_user_id")
		}
		if !optIn(tags, i.TagID) {
			return danglingRef("investment", i.ID, "tag_id")
		}
		if !optIn(investments, i.RolledFromInvestmentID) {
			return danglingRef("investment", i.ID, "rolled_from_investment_id")
		}
	}
	for _, l := range h.Liabilities {
		if l.HouseholdID != hid {
			return wrongHousehold("liability", l.ID)
		}
		if !optIn(users, l.SoleOwnerUserID) {
			return danglingRef("liability", l.ID, "sole_owner_user_id")
		}
		if !optIn(tags, l.TagID) {
			return danglingRef("liability", l.ID, "tag_id")
		}
	}
	for _, r := range h.Receivables {
		if r.HouseholdID != hid {
			return wrongHousehold("receivable", r.ID)
		}
		if !optIn(users, r.SoleOwnerUserID) {
			return danglingRef("receivable", r.ID, "sole_owner_user_id")
		}
		if !optIn(tags, r.TagID) {
			return danglingRef("receivable", r.ID, "tag_id")
		}
	}

	for _, t := range h.Tags {
		if t.HouseholdID != hid {
			return wrongHousehold("tag", t.ID)
		}
	}

	// Detail tables point at their position.
	for _, d := range h.BankAccounts {
		if !assets[d.AssetID] {
			return danglingRef("bank_account", d.AssetID, "asset_id")
		}
	}
	for _, d := range h.Properties {
		if !assets[d.AssetID] {
			return danglingRef("property", d.AssetID, "asset_id")
		}
	}
	for _, d := range h.Vehicles {
		if !assets[d.AssetID] {
			return danglingRef("vehicle", d.AssetID, "asset_id")
		}
	}
	for _, d := range h.Stocks {
		if !investments[d.InvestmentID] {
			return danglingRef("stock", d.InvestmentID, "investment_id")
		}
	}
	for _, d := range h.MutualFunds {
		if !investments[d.InvestmentID] {
			return danglingRef("mutual_fund", d.InvestmentID, "investment_id")
		}
	}
	for _, d := range h.Bonds {
		if !investments[d.InvestmentID] {
			return danglingRef("bond", d.InvestmentID, "investment_id")
		}
	}
	for _, d := range h.Golds {
		if !investments[d.InvestmentID] {
			return danglingRef("gold", d.InvestmentID, "investment_id")
		}
	}
	for _, d := range h.TimeDeposits {
		if !investments[d.InvestmentID] {
			return danglingRef("time_deposit", d.InvestmentID, "investment_id")
		}
	}

	// Snapshots + ledger point at their position.
	for _, s := range h.AssetSnapshots {
		if !assets[s.AssetID] {
			return danglingRef("asset_snapshot", s.ID, "asset_id")
		}
	}
	for _, s := range h.LiabilitySnapshots {
		if !liabilities[s.LiabilityID] {
			return danglingRef("liability_snapshot", s.ID, "liability_id")
		}
	}
	for _, s := range h.ReceivableSnapshots {
		if !receivables[s.ReceivableID] {
			return danglingRef("receivable_snapshot", s.ID, "receivable_id")
		}
	}
	for _, s := range h.InvestmentSnapshots {
		if !investments[s.InvestmentID] {
			return danglingRef("investment_snapshot", s.ID, "investment_id")
		}
	}
	for _, tx := range h.InvestmentTransactions {
		if !investments[tx.InvestmentID] {
			return danglingRef("investment_transaction", tx.ID, "investment_id")
		}
	}

	// Standalone flows.
	for _, in := range h.Income {
		if in.HouseholdID != hid {
			return wrongHousehold("income", in.ID)
		}
		if !optIn(users, in.SoleOwnerUserID) {
			return danglingRef("income", in.ID, "sole_owner_user_id")
		}
	}
	for _, fx := range h.FxRates {
		if fx.HouseholdID != hid {
			return wrongHousehold("fx_rate", fx.ID)
		}
	}
	return nil
}

func idSet[T any](xs []T, key func(T) uuid.UUID) map[uuid.UUID]bool {
	m := make(map[uuid.UUID]bool, len(xs))
	for _, x := range xs {
		m[key(x)] = true
	}
	return m
}

// optIn reports whether an optional foreign key is either unset or resolves to a
// member of the set.
func optIn(set map[uuid.UUID]bool, id *uuid.UUID) bool {
	return id == nil || set[*id]
}

func wrongHousehold(kind string, id uuid.UUID) error {
	return fmt.Errorf("%w: %s %s belongs to a different household", ErrValidationFailed, kind, id)
}

func danglingRef(kind string, id uuid.UUID, field string) error {
	return fmt.Errorf("%w: %s %s has an unresolved %s", ErrValidationFailed, kind, id, field)
}
