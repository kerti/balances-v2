// Package receivables exposes HTTP handlers for the Receivable position
// group. Receivables have no subtype and no extension table — all metadata
// is inline on the core row. Snapshots share the per-group
// receivable_snapshots table and are exposed under
// /api/receivables/{id}/snapshots.
package receivables

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/kerti/balances-v2/backend/internal/auth"
	"github.com/kerti/balances-v2/backend/internal/httperr"
	"github.com/kerti/balances-v2/backend/internal/repo"
)

type Handlers struct {
	repo     *repo.ReceivableRepo
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

func New(r *repo.ReceivableRepo, opts ...Option) *Handlers {
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

func (h *Handlers) Mount(r chi.Router) {
	r.Route("/receivables", func(r chi.Router) {
		r.Use(auth.RequireAuth)
		r.Post("/", h.handleCreate)
		r.Get("/", h.handleList)
		// Create-from-file import: upload a position workbook from the list
		// screen and create a brand-new receivable (Detail sheet) + seed its
		// snapshots (Snapshots sheet) atomically. Static segment, so no clash
		// with POST "/" or the /{id} routes.
		r.Post("/import", h.handleImportCreate)
		// Per-receivable monthly value series for the list total-over-time
		// chart (epic #204). Value-only — receivables carry no cost basis.
		// Static single segment, so no clash with the /{id}/… routes below.
		r.Get("/time-series", h.handleReceivableTimeSeries)
		r.Route("/{id}", func(r chi.Router) {
			r.Get("/", h.handleGet)
			r.Patch("/", h.handleUpdate)
			r.Delete("/", h.handleDelete)
			r.Patch("/lifecycle", h.handleUpdateLifecycle)
			// Export the full position workbook (Detail + Snapshots) in the
			// importer's format, so it round-trips back through the snapshot
			// import on the detail page.
			r.Get("/export", h.handleExport)
			r.Route("/snapshots", func(r chi.Router) {
				r.Post("/", h.handleCreateSnapshot)
				r.Get("/", h.handleListSnapshots)
				r.Patch("/{snapshotID}", h.handleUpdateSnapshot)
				r.Delete("/{snapshotID}", h.handleDeleteSnapshot)
				// Bulk import (M6 side item): download a scoped .xlsx
				// template, then upload a filled one. Static segments, so no
				// clash with the /{snapshotID} routes above.
				r.Get("/import-template", h.handleImportTemplate)
				r.Post("/import", h.handleImportSnapshots)
			})
		})
	})
}

// ----- requests -----------------------------------------------------------

type createReq struct {
	DisplayName      string     `json:"display_name"            validate:"required"`
	Description      *string    `json:"description"`
	OwnershipType    string     `json:"ownership_type"          validate:"required,oneof=sole joint"`
	SoleOwnerUserID  *uuid.UUID `json:"sole_owner_user_id"      validate:"required_if=OwnershipType sole"`
	NativeCurrency   string     `json:"native_currency"         validate:"required,iso4217"`
	CounterpartyName string     `json:"counterparty_name"       validate:"required"`
	DueDate          *string    `json:"due_date"`
}

type updateReq struct {
	DisplayName      string     `json:"display_name"            validate:"required"`
	Description      *string    `json:"description"`
	OwnershipType    string     `json:"ownership_type"          validate:"required,oneof=sole joint"`
	SoleOwnerUserID  *uuid.UUID `json:"sole_owner_user_id"      validate:"required_if=OwnershipType sole"`
	CounterpartyName string     `json:"counterparty_name"       validate:"required"`
	DueDate          *string    `json:"due_date"`
}

// ----- core CRUD ----------------------------------------------------------

func (h *Handlers) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req createReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httperr.Write(w, http.StatusBadRequest, httperr.CodeInvalidJSONBody, nil)
		return
	}
	if err := h.validate.Struct(&req); err != nil {
		httperr.WriteValidation(w, err)
		return
	}
	dueDate, ok := parseOptionalDate(req.DueDate)
	if !ok {
		writeInvalidDate(w, "due_date")
		return
	}

	row, err := h.repo.CreateReceivable(r.Context(), repo.CreateReceivableParams{
		DisplayName:      req.DisplayName,
		Description:      req.Description,
		OwnershipType:    req.OwnershipType,
		SoleOwnerUserID:  req.SoleOwnerUserID,
		NativeCurrency:   req.NativeCurrency,
		CounterpartyName: req.CounterpartyName,
		DueDate:          dueDate,
	})
	if err != nil {
		httperr.WriteRepo(w, "create receivable", err)
		return
	}
	writeJSON(w, http.StatusCreated, row)
}

func (h *Handlers) handleList(w http.ResponseWriter, r *http.Request) {
	list, err := h.repo.ListReceivables(r.Context())
	if err != nil {
		httperr.WriteRepo(w, "list receivables", err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

// handleReceivableTimeSeries returns the per-receivable monthly value series
// for every receivable in the household (epic #204), feeding the list
// total-over-time chart without a per-receivable fan-out. Value-only.
func (h *Handlers) handleReceivableTimeSeries(w http.ResponseWriter, r *http.Request) {
	series, err := h.repo.ReceivableTimeSeries(r.Context())
	if err != nil {
		httperr.WriteRepo(w, "receivable time series", err)
		return
	}
	writeJSON(w, http.StatusOK, series)
}

func (h *Handlers) handleGet(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		writeInvalidID(w, "id")
		return
	}
	row, err := h.repo.GetReceivable(r.Context(), id)
	if err != nil {
		httperr.WriteRepo(w, "get receivable", err)
		return
	}
	writeJSON(w, http.StatusOK, row)
}

func (h *Handlers) handleUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		writeInvalidID(w, "id")
		return
	}
	var req updateReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httperr.Write(w, http.StatusBadRequest, httperr.CodeInvalidJSONBody, nil)
		return
	}
	if err := h.validate.Struct(&req); err != nil {
		httperr.WriteValidation(w, err)
		return
	}
	dueDate, ok := parseOptionalDate(req.DueDate)
	if !ok {
		writeInvalidDate(w, "due_date")
		return
	}

	row, err := h.repo.UpdateReceivable(r.Context(), id, repo.UpdateReceivableParams{
		DisplayName:      req.DisplayName,
		Description:      req.Description,
		OwnershipType:    req.OwnershipType,
		SoleOwnerUserID:  req.SoleOwnerUserID,
		CounterpartyName: req.CounterpartyName,
		DueDate:          dueDate,
	})
	if err != nil {
		httperr.WriteRepo(w, "update receivable", err)
		return
	}
	writeJSON(w, http.StatusOK, row)
}

func (h *Handlers) handleDelete(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		writeInvalidID(w, "id")
		return
	}
	if err := h.repo.DeleteReceivable(r.Context(), id); err != nil {
		httperr.WriteRepo(w, "delete receivable", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ----- snapshots ----------------------------------------------------------

type createSnapshotReq struct {
	YearMonth   string           `json:"year_month"  validate:"required"`
	Amount      *decimal.Decimal `json:"amount"      validate:"required"`
	Currency    string           `json:"currency"    validate:"required,iso4217"`
	AsOfDate    *string          `json:"as_of_date"`
	Description *string          `json:"description"`
}

type updateSnapshotReq struct {
	Amount      *decimal.Decimal `json:"amount"      validate:"required"`
	Currency    string           `json:"currency"    validate:"required,iso4217"`
	AsOfDate    *string          `json:"as_of_date"`
	Description *string          `json:"description"`
}

func (h *Handlers) handleCreateSnapshot(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		writeInvalidID(w, "id")
		return
	}
	var req createSnapshotReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httperr.Write(w, http.StatusBadRequest, httperr.CodeInvalidJSONBody, nil)
		return
	}
	if err := h.validate.Struct(&req); err != nil {
		httperr.WriteValidation(w, err)
		return
	}
	ym, err := parseYearMonth(req.YearMonth)
	if err != nil {
		httperr.Write(w, http.StatusBadRequest, httperr.CodeInvalidYearMonth, nil)
		return
	}
	if isFutureYearMonth(ym, h.now()) {
		httperr.Write(w, http.StatusBadRequest, httperr.CodeFutureYearMonth, nil)
		return
	}
	asOf, ok := parseOptionalDate(req.AsOfDate)
	if !ok {
		writeInvalidDate(w, "as_of_date")
		return
	}
	if asOf != nil && isFutureDate(*asOf, h.now()) {
		httperr.Write(w, http.StatusBadRequest, httperr.CodeSnapshotFutureDate, nil)
		return
	}

	snap, err := h.repo.CreateReceivableSnapshot(r.Context(), repo.CreateReceivableSnapshotParams{
		ReceivableID: id,
		YearMonth:    ym,
		Amount:       *req.Amount,
		Currency:     req.Currency,
		AsOfDate:     asOf,
		Description:  req.Description,
	})
	if err != nil {
		httperr.WriteRepo(w, "create receivable snapshot", err)
		return
	}
	writeJSON(w, http.StatusCreated, snap)
}

func (h *Handlers) handleListSnapshots(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		writeInvalidID(w, "id")
		return
	}
	snaps, err := h.repo.ListReceivableSnapshots(r.Context(), id)
	if err != nil {
		httperr.WriteRepo(w, "list receivable snapshots", err)
		return
	}
	writeJSON(w, http.StatusOK, snaps)
}

func (h *Handlers) handleUpdateSnapshot(w http.ResponseWriter, r *http.Request) {
	snapshotID, err := parseIDParam(r, "snapshotID")
	if err != nil {
		writeInvalidID(w, "snapshot_id")
		return
	}
	var req updateSnapshotReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httperr.Write(w, http.StatusBadRequest, httperr.CodeInvalidJSONBody, nil)
		return
	}
	if err := h.validate.Struct(&req); err != nil {
		httperr.WriteValidation(w, err)
		return
	}
	asOf, ok := parseOptionalDate(req.AsOfDate)
	if !ok {
		writeInvalidDate(w, "as_of_date")
		return
	}
	if asOf != nil && isFutureDate(*asOf, h.now()) {
		httperr.Write(w, http.StatusBadRequest, httperr.CodeSnapshotFutureDate, nil)
		return
	}

	snap, err := h.repo.UpdateReceivableSnapshot(r.Context(), repo.UpdateReceivableSnapshotParams{
		SnapshotID:  snapshotID,
		Amount:      *req.Amount,
		Currency:    req.Currency,
		AsOfDate:    asOf,
		Description: req.Description,
	})
	if err != nil {
		httperr.WriteRepo(w, "update receivable snapshot", err)
		return
	}
	writeJSON(w, http.StatusOK, snap)
}

func (h *Handlers) handleDeleteSnapshot(w http.ResponseWriter, r *http.Request) {
	snapshotID, err := parseIDParam(r, "snapshotID")
	if err != nil {
		writeInvalidID(w, "snapshot_id")
		return
	}
	if err := h.repo.DeleteReceivableSnapshot(r.Context(), snapshotID); err != nil {
		httperr.WriteRepo(w, "delete receivable snapshot", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ----- helpers ------------------------------------------------------------

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

// writeInvalidDate is the equivalent shim for a YYYY-MM-DD parse failure on
// an optional body field. parseOptionalDate is field-agnostic by design;
// the caller names the field at the write site.
func writeInvalidDate(w http.ResponseWriter, field string) {
	httperr.Write(w, http.StatusBadRequest, httperr.CodeInvalidDate, map[string]any{"field": field})
}

func parseIDParam(r *http.Request, name string) (uuid.UUID, error) {
	return uuid.Parse(chi.URLParam(r, name))
}

// parseOptionalDate parses a YYYY-MM-DD pointer-string into a UTC time. Empty
// or nil input is a success with a nil time; an unparseable string returns
// (nil, false) so the caller can emit INVALID_DATE with its known field
// name rather than threading the field through here.
func parseOptionalDate(s *string) (*time.Time, bool) {
	if s == nil || *s == "" {
		return nil, true
	}
	t, err := time.Parse("2006-01-02", *s)
	if err != nil {
		return nil, false
	}
	return &t, true
}

func parseYearMonth(s string) (time.Time, error) {
	if len(s) == 7 {
		return time.Parse("2006-01", s)
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return time.Time{}, err
	}
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC), nil
}

// isFutureYearMonth reports whether ym (first-of-month UTC) is strictly later
// than the current month derived from now.
func isFutureYearMonth(ym, now time.Time) bool {
	n := now.UTC()
	currentMonth := time.Date(n.Year(), n.Month(), 1, 0, 0, 0, 0, time.UTC)
	return ym.After(currentMonth)
}

// isFutureDate reports whether t (a calendar date parsed as UTC midnight) is
// strictly after today UTC.
func isFutureDate(t, now time.Time) bool {
	n := now.UTC()
	today := time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, time.UTC)
	return t.After(today)
}
