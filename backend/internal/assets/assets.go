// Package assets exposes HTTP handlers for the Asset position group.
// The handler layer is thin — it decodes JSON, validates the request via
// go-playground/validator, dispatches to repo.AssetRepo, and encodes the
// response. Tenancy enforcement lives in the repo + SQL layers (per
// ADR-0005 + M3.2 / M3.3); these handlers just thread the request context
// through.
package assets

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
	repo     *repo.AssetRepo
	validate *validator.Validate
	now      func() time.Time
}

// Option mutates a Handlers during construction. Used by tests to inject a
// fake clock for the future-date validation; production wiring takes the
// zero-option path and gets the real time.Now.
type Option func(*Handlers)

// WithNow overrides the clock used for future-date validation on snapshots.
func WithNow(fn func() time.Time) Option {
	return func(h *Handlers) { h.now = fn }
}

func New(r *repo.AssetRepo, opts ...Option) *Handlers {
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

// Mount registers all asset routes. Caller is expected to mount this under
// `/api` and apply SessionMiddleware at a higher level; RequireAuth is
// applied inside Mount so all routes are protected.
//
// Snapshots live under /assets/{id}/snapshots rather than under each
// subtype's path because the snapshot shape and storage table
// (asset_snapshots, per ADR-0022) are shared across all asset subtypes.
func (h *Handlers) Mount(r chi.Router) {
	r.Route("/bank-accounts", func(r chi.Router) {
		r.Use(auth.RequireAuth)
		r.Post("/", h.handleCreateBankAccount)
		r.Get("/", h.handleListBankAccounts)
		r.Route("/{id}", func(r chi.Router) {
			r.Get("/", h.handleGetBankAccount)
			r.Patch("/", h.handleUpdateBankAccount)
			r.Delete("/", h.handleDeleteBankAccount)
			// Export the full position workbook (Detail + Snapshots) in the
			// importer's format, so it round-trips back through the snapshot
			// import on the detail page.
			r.Get("/export", h.handleExportBankAccount)
		})
	})

	r.Route("/properties", func(r chi.Router) {
		r.Use(auth.RequireAuth)
		r.Post("/", h.handleCreateProperty)
		r.Get("/", h.handleListProperties)
		r.Route("/{id}", func(r chi.Router) {
			r.Get("/", h.handleGetProperty)
			r.Patch("/", h.handleUpdateProperty)
			r.Delete("/", h.handleDeleteProperty)
		})
	})

	r.Route("/vehicles", func(r chi.Router) {
		r.Use(auth.RequireAuth)
		r.Post("/", h.handleCreateVehicle)
		r.Get("/", h.handleListVehicles)
		r.Route("/{id}", func(r chi.Router) {
			r.Get("/", h.handleGetVehicle)
			r.Patch("/", h.handleUpdateVehicle)
			r.Delete("/", h.handleDeleteVehicle)
		})
	})

	r.Route("/assets/{id}/snapshots", func(r chi.Router) {
		r.Use(auth.RequireAuth)
		r.Post("/", h.handleCreateSnapshot)
		r.Get("/", h.handleListSnapshots)
		r.Patch("/{snapshotID}", h.handleUpdateSnapshot)
		r.Delete("/{snapshotID}", h.handleDeleteSnapshot)
		// Bulk import (M6 side item): download a scoped .xlsx template, then
		// upload a filled one. ?mode=preview (default) validates + counts;
		// ?mode=commit upserts all-or-nothing. Static segments, so no clash
		// with the /{snapshotID} routes above.
		r.Get("/import-template", h.handleImportTemplate)
		r.Post("/import", h.handleImportSnapshots)
	})

	// Lifecycle (status/terminated_at/termination_note) lives at the parent
	// /assets level — like snapshots — because it operates on the shared
	// `assets` table, not on subtype-specific fields (ADR-0009).
	r.Route("/assets/{id}/lifecycle", func(r chi.Router) {
		r.Use(auth.RequireAuth)
		r.Patch("/", h.handleUpdateLifecycle)
	})
}

// ----- helpers shared across handlers -------------------------------------

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if body != nil {
		_ = json.NewEncoder(w).Encode(body)
	}
}

// writeInvalidID is a small shim around httperr.Write so the "invalid UUID
// path param" call sites read at one level of abstraction. The field arg
// is the JSON-style name the FE will interpolate ("id", "snapshot_id").
func writeInvalidID(w http.ResponseWriter, field string) {
	httperr.Write(w, http.StatusBadRequest, httperr.CodeInvalidID, map[string]any{"field": field})
}

// writeInvalidDate is the equivalent shim for a YYYY-MM-DD parse failure
// on a date-typed body field.
func writeInvalidDate(w http.ResponseWriter, field string) {
	httperr.Write(w, http.StatusBadRequest, httperr.CodeInvalidDate, map[string]any{"field": field})
}

func parseIDParam(r *http.Request, name string) (uuid.UUID, error) {
	raw := chi.URLParam(r, name)
	return uuid.Parse(raw)
}
