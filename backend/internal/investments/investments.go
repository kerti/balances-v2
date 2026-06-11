// Package investments exposes HTTP handlers for the Investment position
// group. The handler layer is thin — it decodes JSON, validates the request,
// dispatches to repo.InvestmentRepo, and encodes the response. Subtype
// routes (stocks, mutual-funds, golds) sit alongside a shared
// /investments/{id}/snapshots route since investment_snapshots is one
// table per ADR-0022; the repo enforces the subtype→shape XOR mapping
// that the DB CHECK can't see.
package investments

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"

	"github.com/kerti/balances-v2/backend/internal/auth"
	"github.com/kerti/balances-v2/backend/internal/httperr"
	"github.com/kerti/balances-v2/backend/internal/repo"
)

type Handlers struct {
	repo     *repo.InvestmentRepo
	validate *validator.Validate
	now      func() time.Time
}

// Option mutates a Handlers during construction. Used by tests to inject a
// fake clock for the future-date validation; production wiring takes the
// zero-option path and gets the real time.Now.
type Option func(*Handlers)

// WithNow overrides the clock used for future-date validation on snapshots
// and transactions.
func WithNow(fn func() time.Time) Option {
	return func(h *Handlers) { h.now = fn }
}

func New(r *repo.InvestmentRepo, opts ...Option) *Handlers {
	h := &Handlers{
		repo:     r,
		validate: httperr.NewValidator(),
		now:      time.Now,
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// Mount registers all investment routes under /investments. Caller mounts
// this under /api and applies SessionMiddleware higher up; RequireAuth is
// applied per-route here.
//
// Snapshots live under /investments/{id}/snapshots because the snapshot
// table is shared across all subtypes (ADR-0022). The repo validates
// shape (quantity+price for stock/mutual_fund/gold; accrued_interest
// for bond/time_deposit) based on the parent investment's subtype.
func (h *Handlers) Mount(r chi.Router) {
	r.Route("/investments", func(r chi.Router) {
		r.Use(auth.RequireAuth)

		r.Route("/stocks", func(r chi.Router) {
			r.Post("/", h.handleCreateStock)
			r.Get("/", h.handleListStocks)
			r.Route("/{id}", func(r chi.Router) {
				r.Get("/", h.handleGetStock)
				r.Patch("/", h.handleUpdateStock)
				r.Delete("/", h.handleDeleteStock)
				// Export the full position workbook (Detail + Snapshots +
				// Transactions) in the importer's format; round-trips back in
				// through the snapshot-import flow on the detail page.
				r.Get("/export", h.handleExportStock)
			})
		})

		r.Route("/mutual-funds", func(r chi.Router) {
			r.Post("/", h.handleCreateMutualFund)
			r.Get("/", h.handleListMutualFunds)
			r.Route("/{id}", func(r chi.Router) {
				r.Get("/", h.handleGetMutualFund)
				r.Patch("/", h.handleUpdateMutualFund)
				r.Delete("/", h.handleDeleteMutualFund)
				r.Get("/export", h.handleExportMutualFund)
			})
		})

		r.Route("/golds", func(r chi.Router) {
			r.Post("/", h.handleCreateGold)
			r.Get("/", h.handleListGolds)
			r.Route("/{id}", func(r chi.Router) {
				r.Get("/", h.handleGetGold)
				r.Patch("/", h.handleUpdateGold)
				r.Delete("/", h.handleDeleteGold)
				r.Get("/export", h.handleExportGold)
			})
		})

		r.Route("/bonds", func(r chi.Router) {
			r.Post("/", h.handleCreateBond)
			r.Get("/", h.handleListBonds)
			r.Route("/{id}", func(r chi.Router) {
				r.Get("/", h.handleGetBond)
				r.Patch("/", h.handleUpdateBond)
				r.Delete("/", h.handleDeleteBond)
				r.Get("/export", h.handleExportBond)
			})
		})

		r.Route("/time-deposits", func(r chi.Router) {
			r.Post("/", h.handleCreateTimeDeposit)
			r.Get("/", h.handleListTimeDeposits)
			r.Route("/{id}", func(r chi.Router) {
				r.Get("/", h.handleGetTimeDeposit)
				r.Patch("/", h.handleUpdateTimeDeposit)
				r.Delete("/", h.handleDeleteTimeDeposit)
				r.Get("/export", h.handleExportTimeDeposit)
			})
		})

		// Per-position monthly value + cost series for the list/home time
		// graphs (issue #22). Static single segment, so no clash with the
		// two-segment /{id}/… routes below.
		r.Get("/time-series", h.handleInvestmentTimeSeries)

		r.Route("/{id}/snapshots", func(r chi.Router) {
			r.Post("/", h.handleCreateSnapshot)
			r.Get("/", h.handleListSnapshots)
			r.Patch("/{snapshotID}", h.handleUpdateSnapshot)
			r.Delete("/{snapshotID}", h.handleDeleteSnapshot)
			// Bulk import (M6 side item): download a subtype-shaped .xlsx
			// template, then upload a filled one. Static segments, so no
			// clash with the /{snapshotID} routes above.
			r.Get("/import-template", h.handleImportTemplate)
			r.Post("/import", h.handleImportSnapshots)
		})

		r.Route("/{id}/transactions", func(r chi.Router) {
			r.Post("/", h.handleCreateTransaction)
			r.Get("/", h.handleListTransactions)
			r.Patch("/{transactionID}", h.handleUpdateTransaction)
			r.Delete("/{transactionID}", h.handleDeleteTransaction)
		})

		// Lifecycle operates on the shared `investments` table (ADR-0009), so
		// it sits at the parent /{id} level alongside snapshots/transactions
		// rather than under each subtype.
		r.Route("/{id}/lifecycle", func(r chi.Router) {
			r.Patch("/", h.handleUpdateLifecycle)
		})
	})
}

// handleInvestmentTimeSeries returns the per-position monthly value + cost
// series for every investment in the household (issue #22), feeding the
// list/home time graphs without a per-position fan-out.
func (h *Handlers) handleInvestmentTimeSeries(w http.ResponseWriter, r *http.Request) {
	series, err := h.repo.InvestmentTimeSeries(r.Context())
	if err != nil {
		httperr.WriteRepo(w, "investment time series", err)
		return
	}
	writeJSON(w, http.StatusOK, series)
}

// ----- helpers shared across handlers -------------------------------------

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if body != nil {
		_ = json.NewEncoder(w).Encode(body)
	}
}

// writeInvalidID / writeInvalidDate are small shims around httperr.Write so
// the "invalid path-param" and "unparseable date body field" call sites
// stay at one level of abstraction.
func writeInvalidID(w http.ResponseWriter, field string) {
	httperr.Write(w, http.StatusBadRequest, httperr.CodeInvalidID, map[string]any{"field": field})
}

func writeInvalidDate(w http.ResponseWriter, field string) {
	httperr.Write(w, http.StatusBadRequest, httperr.CodeInvalidDate, map[string]any{"field": field})
}

func parseIDParam(r *http.Request, name string) (uuid.UUID, error) {
	return uuid.Parse(chi.URLParam(r, name))
}
