// Package backup implements whole-Household backup (export) and, in later
// slices, restore (import) — ADR-0036. A backup is a single portable artifact
// holding a Household's entire data, used for disaster recovery and
// SaaS<->self-host portability. It is a *logical* JSON export decoupled from
// the physical schema, carrying its own standalone format_version.
//
// This slice (#174) implements export only. The envelope embeds the generated
// db.* row structs directly: they already carry json tags, marshal decimals as
// strings (ADR-0011, shopspring default) and UUIDs/timestamps as strings, so
// the wire shape is full-fidelity and exact by construction.
//
// Memory note: this first implementation assembles the Household into the typed
// Envelope and encodes it in one pass. The load-bearing contract — the
// parents-before-children section order (Section field order below) — is
// honored in the output, so a future importer can stream it. Constant-memory
// per-table cursor batching is a documented refinement (ADR-0036, parking lot),
// deferred until a Household is large enough to need it; realistic alpha data
// is KB-scale.
package backup

import (
	"time"

	"github.com/kerti/balances-v2/backend/internal/db"
)

// FormatVersion is the standalone backup-format version (ADR-0036), decoupled
// from the goose migration number and the app SemVer. It bumps only when the
// backup *shape* changes. There is exactly one version today, so there is
// nothing to migrate yet; the transform chain + its fixtures arrive with the
// restore slices (#175/#177).
const FormatVersion = 1

// Fidelity selects what a backup carries (ADR-0036).
type Fidelity string

const (
	// FidelityFull carries soft-deleted ("Recycle Bin") rows with deleted_at
	// intact — an exact round-trip of the source Household.
	FidelityFull Fidelity = "full"
	// FidelityCompacted carries live rows only — a clean snapshot of current
	// truth.
	FidelityCompacted Fidelity = "compacted"
)

// ParseFidelity maps a query/string value to a Fidelity, defaulting to full
// (the safe, lossless choice) for an empty or unknown value.
func ParseFidelity(s string) Fidelity {
	if s == string(FidelityCompacted) {
		return FidelityCompacted
	}
	return FidelityFull
}

// Envelope is the top-level backup artifact. Field order is the serialized
// order: header first, then the Household payload.
type Envelope struct {
	FormatVersion int            `json:"format_version"`
	ExportedAt    time.Time      `json:"exported_at"`
	Instance      string         `json:"instance"`  // the producing instance's public URL
	Fidelity      Fidelity       `json:"fidelity"`  // "full" | "compacted"
	Counts        map[string]int `json:"counts"`    // per-section row counts (import integrity)
	Household     HouseholdData  `json:"household"` // the payload
}

// HouseholdData is the Household payload in **parents-before-children** order
// (ADR-0036): a child section never precedes its parent. This order is a frozen
// part of the format_version 1 contract — it is what lets a future importer
// stream the file in a single pass, resolving each child's foreign key against
// already-seen parents. Do not reorder without a format bump.
type HouseholdData struct {
	Household db.Household `json:"household"`
	Users     []db.User    `json:"users"`
	Tags      []db.Tag     `json:"tags"`

	// Positions: parent row then its subtype/extension detail.
	Assets       []db.Asset             `json:"assets"`
	BankAccounts []db.BankAccountDetail `json:"bank_accounts"`
	Properties   []db.PropertyDetail    `json:"properties"`
	Vehicles     []db.VehicleDetail     `json:"vehicles"`
	Liabilities  []db.Liability         `json:"liabilities"`
	Receivables  []db.Receivable        `json:"receivables"`
	Investments  []db.Investment        `json:"investments"`
	Stocks       []db.StockDetail       `json:"stocks"`
	MutualFunds  []db.MutualFundDetail  `json:"mutual_funds"`
	Bonds        []db.BondDetail        `json:"bonds"`
	Golds        []db.GoldDetail        `json:"golds"`
	TimeDeposits []db.TimeDepositDetail `json:"time_deposits"`

	// History: snapshots and the investment ledger, after their positions.
	AssetSnapshots         []db.AssetSnapshot         `json:"asset_snapshots"`
	LiabilitySnapshots     []db.LiabilitySnapshot     `json:"liability_snapshots"`
	ReceivableSnapshots    []db.ReceivableSnapshot    `json:"receivable_snapshots"`
	InvestmentSnapshots    []db.InvestmentSnapshot    `json:"investment_snapshots"`
	InvestmentTransactions []db.InvestmentTransaction `json:"investment_transactions"`

	// Standalone flows.
	Income  []db.Income `json:"income"`
	FxRates []db.FxRate `json:"fx_rates"`
}
