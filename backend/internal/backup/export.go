package backup

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kerti/balances-v2/backend/internal/auth"
	"github.com/kerti/balances-v2/backend/internal/db"
	"github.com/kerti/balances-v2/backend/internal/httperr"
)

// Handlers serves the backup endpoints. Export reads across every table, so it
// talks to db.Queries directly rather than threading 22 pass-through methods
// through a repo — household scoping is enforced by passing the caller's
// HouseholdID into every query (the same tenancy boundary the repos use).
type Handlers struct {
	pool     *pgxpool.Pool
	q        *db.Queries
	instance string        // this instance's public URL, stamped into the envelope
	sessions SessionIssuer // re-issues the caller's session after a restore
}

// SessionIssuer mints a fresh session + cookie for a user. The restore flow uses
// it to keep the caller signed in across the session-wiping commit; satisfied by
// *auth.Handlers.
type SessionIssuer interface {
	IssueSession(ctx context.Context, w http.ResponseWriter, userID uuid.UUID, userAgent string) error
}

func New(pool *pgxpool.Pool, instanceURL string, sessions SessionIssuer) *Handlers {
	return &Handlers{pool: pool, q: db.New(pool), instance: instanceURL, sessions: sessions}
}

func (h *Handlers) Mount(r chi.Router) {
	r.Route("/backup", func(r chi.Router) {
		r.Use(auth.RequireAuth)
		// Any member may export (read-only, household-scoped) — equal-access
		// model, ADR-0036/ADR-0004.
		r.Get("/export", h.handleExport)
		// Restore is a two-step, stateless re-upload: preview validates the file
		// and returns the stakes summary; commit re-validates and performs the
		// destructive wipe+load (ADR-0036). Equal-access, but the membership guard
		// inside Validate means a member can only restore their own Household.
		r.Post("/restore/preview", h.handleRestorePreview)
		r.Post("/restore/commit", h.handleRestoreCommit)
	})
}

// handleExport streams the caller's entire Household as a gzipped JSON backup
// (ADR-0036). The ?fidelity= query selects full (default — carries soft-deleted
// rows) or compacted (live rows only).
func (h *Handlers) handleExport(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		httperr.Write(w, http.StatusUnauthorized, httperr.CodeUnauthorized, nil)
		return
	}

	fidelity := ParseFidelity(r.URL.Query().Get("fidelity"))
	env, err := h.buildEnvelope(r.Context(), user.HouseholdID, fidelity)
	if err != nil {
		httperr.WriteRepo(w, "backup export", err)
		return
	}

	filename := fmt.Sprintf("household-backup-%s.json.gz", time.Now().UTC().Format("2006-01-02"))
	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename=%q`, filename))

	// Header is written on the first body write; once streaming starts we can no
	// longer change the status, so any encode error past this point can only be
	// logged — the gather already succeeded, so this is unlikely.
	gz := gzip.NewWriter(w)
	// The response is already committed once we start writing, so a write/flush
	// error here can't change the status — a truncated gzip trailer just makes
	// the corruption detectable on import (CRC), which is the intended guard.
	defer func() { _ = gz.Close() }()
	enc := json.NewEncoder(gz)
	_ = enc.Encode(env)
}

// buildEnvelope gathers the whole Household in parents-before-children order and
// wraps it with the format header. See the memory note in format.go on why this
// assembles before encoding rather than cursor-streaming per table.
func (h *Handlers) buildEnvelope(ctx context.Context, hid uuid.UUID, fidelity Fidelity) (*Envelope, error) {
	data, err := h.gather(ctx, hid, fidelity)
	if err != nil {
		return nil, err
	}
	return &Envelope{
		FormatVersion: FormatVersion,
		ExportedAt:    time.Now().UTC(),
		Instance:      h.instance,
		Fidelity:      fidelity,
		Counts:        data.SectionCounts(),
		Household:     *data,
	}, nil
}

func (h *Handlers) gather(ctx context.Context, hid uuid.UUID, fidelity Fidelity) (*HouseholdData, error) {
	incl := fidelity == FidelityFull

	household, err := h.q.GetHouseholdForExport(ctx, hid)
	if err != nil {
		return nil, fmt.Errorf("household: %w", err)
	}

	d := &HouseholdData{Household: household}

	// Each gatherer is a section read scoped to (hid, incl). Errors are wrapped
	// with the section name so a failure points at the offending table.
	type step struct {
		name string
		run  func() error
	}
	steps := []step{
		{"users", func() error {
			v, e := h.q.ListUsersForExport(ctx, db.ListUsersForExportParams{HouseholdID: hid, IncludeDeleted: incl})
			d.Users = v
			return e
		}},
		{"tags", func() error {
			v, e := h.q.ListTagsForExport(ctx, db.ListTagsForExportParams{HouseholdID: hid, IncludeDeleted: incl})
			d.Tags = v
			return e
		}},
		{"assets", func() error {
			v, e := h.q.ListAssetsForExport(ctx, db.ListAssetsForExportParams{HouseholdID: hid, IncludeDeleted: incl})
			d.Assets = v
			return e
		}},
		{"bank_accounts", func() error {
			v, e := h.q.ListBankAccountsForExport(ctx, db.ListBankAccountsForExportParams{HouseholdID: hid, IncludeDeleted: incl})
			d.BankAccounts = v
			return e
		}},
		{"properties", func() error {
			v, e := h.q.ListPropertiesForExport(ctx, db.ListPropertiesForExportParams{HouseholdID: hid, IncludeDeleted: incl})
			d.Properties = v
			return e
		}},
		{"vehicles", func() error {
			v, e := h.q.ListVehiclesForExport(ctx, db.ListVehiclesForExportParams{HouseholdID: hid, IncludeDeleted: incl})
			d.Vehicles = v
			return e
		}},
		{"liabilities", func() error {
			v, e := h.q.ListLiabilitiesForExport(ctx, db.ListLiabilitiesForExportParams{HouseholdID: hid, IncludeDeleted: incl})
			d.Liabilities = v
			return e
		}},
		{"receivables", func() error {
			v, e := h.q.ListReceivablesForExport(ctx, db.ListReceivablesForExportParams{HouseholdID: hid, IncludeDeleted: incl})
			d.Receivables = v
			return e
		}},
		{"investments", func() error {
			v, e := h.q.ListInvestmentsForExport(ctx, db.ListInvestmentsForExportParams{HouseholdID: hid, IncludeDeleted: incl})
			d.Investments = v
			return e
		}},
		{"stocks", func() error {
			v, e := h.q.ListStocksForExport(ctx, db.ListStocksForExportParams{HouseholdID: hid, IncludeDeleted: incl})
			d.Stocks = v
			return e
		}},
		{"mutual_funds", func() error {
			v, e := h.q.ListMutualFundsForExport(ctx, db.ListMutualFundsForExportParams{HouseholdID: hid, IncludeDeleted: incl})
			d.MutualFunds = v
			return e
		}},
		{"bonds", func() error {
			v, e := h.q.ListBondsForExport(ctx, db.ListBondsForExportParams{HouseholdID: hid, IncludeDeleted: incl})
			d.Bonds = v
			return e
		}},
		{"golds", func() error {
			v, e := h.q.ListGoldsForExport(ctx, db.ListGoldsForExportParams{HouseholdID: hid, IncludeDeleted: incl})
			d.Golds = v
			return e
		}},
		{"time_deposits", func() error {
			v, e := h.q.ListTimeDepositsForExport(ctx, db.ListTimeDepositsForExportParams{HouseholdID: hid, IncludeDeleted: incl})
			d.TimeDeposits = v
			return e
		}},
		{"asset_snapshots", func() error {
			v, e := h.q.ListAssetSnapshotsForExport(ctx, db.ListAssetSnapshotsForExportParams{HouseholdID: hid, IncludeDeleted: incl})
			d.AssetSnapshots = v
			return e
		}},
		{"liability_snapshots", func() error {
			v, e := h.q.ListLiabilitySnapshotsForExport(ctx, db.ListLiabilitySnapshotsForExportParams{HouseholdID: hid, IncludeDeleted: incl})
			d.LiabilitySnapshots = v
			return e
		}},
		{"receivable_snapshots", func() error {
			v, e := h.q.ListReceivableSnapshotsForExport(ctx, db.ListReceivableSnapshotsForExportParams{HouseholdID: hid, IncludeDeleted: incl})
			d.ReceivableSnapshots = v
			return e
		}},
		{"investment_snapshots", func() error {
			v, e := h.q.ListInvestmentSnapshotsForExport(ctx, db.ListInvestmentSnapshotsForExportParams{HouseholdID: hid, IncludeDeleted: incl})
			d.InvestmentSnapshots = v
			return e
		}},
		{"investment_transactions", func() error {
			v, e := h.q.ListInvestmentTransactionsForExport(ctx, db.ListInvestmentTransactionsForExportParams{HouseholdID: hid, IncludeDeleted: incl})
			d.InvestmentTransactions = v
			return e
		}},
		{"income", func() error {
			v, e := h.q.ListIncomeForExport(ctx, db.ListIncomeForExportParams{HouseholdID: hid, IncludeDeleted: incl})
			d.Income = v
			return e
		}},
		{"fx_rates", func() error {
			v, e := h.q.ListFxRatesForExport(ctx, db.ListFxRatesForExportParams{HouseholdID: hid, IncludeDeleted: incl})
			d.FxRates = v
			return e
		}},
	}

	for _, s := range steps {
		if err := s.run(); err != nil {
			return nil, fmt.Errorf("%s: %w", s.name, err)
		}
	}
	return d, nil
}
