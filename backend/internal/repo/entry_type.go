package repo

// A Position's entry type declares what its birth was (ADR-0053 §3). It cannot
// be inferred: an acquisition funded from wealth already tracked here and a
// Position the Household already owned present to the report engine as the same
// thing — a first Snapshot with no prior value — so only the Household can say
// which happened.
const (
	// EntryTypeAcquired: funded from tracked wealth, so the other leg of the
	// movement is already in the books and the residual is right without help.
	// The column DEFAULT, and the value that reproduces pre-ADR-0053 behaviour.
	EntryTypeAcquired = "acquired"
	// EntryTypeNewlyTracked: already owned, or arrived with the Household. Its
	// first Snapshot enters net worth with no counterpart flow, so the Tracking
	// Change term absorbs it and Living Expenses is left alone.
	EntryTypeNewlyTracked = "newly_tracked"
)

// entryTypeOrDefault normalises an unset entry type to `acquired`, mirroring the
// column DEFAULT at the Go boundary. Create paths that predate ADR-0053 (the
// demo seeder, fixtures, an API client that never learned the field) therefore
// keep producing genuine acquisitions, which is what they always meant.
func entryTypeOrDefault(s string) string {
	if s == "" {
		return EntryTypeAcquired
	}
	return s
}
