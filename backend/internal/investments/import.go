package investments

import (
	"fmt"
	"net/http"

	"github.com/kerti/balances-v2/backend/internal/httperr"
	"github.com/kerti/balances-v2/backend/internal/repo"
	"github.com/kerti/balances-v2/backend/internal/snapshotimport"
)

// maxImportUpload caps the uploaded spreadsheet size. A decade of monthly rows
// is a few KB; 5 MB is generous headroom against a hostile/confused upload.
const maxImportUpload = 5 << 20 // 5 MB

// shapeForSubtype maps an investment subtype to its snapshot template shape:
// stock/mutual_fund/gold carry quantity+price_per_unit; bond/time_deposit carry
// the total value + accrued_interest (mirrors validateInvestmentSnapshotShape).
func shapeForSubtype(subtype string) snapshotimport.Shape {
	switch subtype {
	case "bond", "time_deposit":
		return snapshotimport.ShapeAccruedInterest
	default: // stock, mutual_fund, gold
		return snapshotimport.ShapeQuantityPrice
	}
}

// handleImportTemplate streams an .xlsx snapshot-import template scoped to the
// investment — its name + native currency + subtype-driven column shape baked
// into the example + instructions.
func (h *Handlers) handleImportTemplate(w http.ResponseWriter, r *http.Request) {
	investmentID, err := parseIDParam(r, "id")
	if err != nil {
		writeInvalidID(w, "id")
		return
	}

	name, currency, subtype, err := h.repo.InvestmentImportMeta(r.Context(), investmentID)
	if err != nil {
		httperr.WriteRepo(w, "import template: investment meta", err)
		return
	}

	xlsx, err := snapshotimport.BuildTemplate(snapshotimport.TemplateMeta{
		PositionName:    name,
		DefaultCurrency: currency,
		Shape:           shapeForSubtype(subtype),
	})
	if err != nil {
		httperr.WriteRepo(w, "import template: build", err)
		return
	}

	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", `attachment; filename="snapshot-import-template.xlsx"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(xlsx)
}

type importResponse struct {
	Mode      string                    `json:"mode"`      // "preview" | "commit"
	Committed bool                      `json:"committed"` // true only when rows were written
	ToInsert  int                       `json:"to_insert"`
	ToUpdate  int                       `json:"to_update"` // existing month, different values: the earlier row is archived
	Unchanged int                       `json:"unchanged"` // existing month, identical values: nothing written
	Errors    []snapshotimport.RowError `json:"errors"`
}

// handleImportSnapshots accepts a filled-in template (multipart "file"). The
// column shape parsed is selected by the investment's subtype. With mode=preview
// (default) it parses + validates + classifies and writes nothing; with
// mode=commit it additionally writes (replacing changed months, ADR-0058), but only if zero rows errored
// (all-or-nothing) — otherwise it returns 422 with the row errors.
func (h *Handlers) handleImportSnapshots(w http.ResponseWriter, r *http.Request) {
	investmentID, err := parseIDParam(r, "id")
	if err != nil {
		writeInvalidID(w, "id")
		return
	}

	mode := r.URL.Query().Get("mode")
	if mode == "" {
		mode = "preview"
	}
	if mode != "preview" && mode != "commit" {
		httperr.Write(w, http.StatusBadRequest, httperr.CodeInvalidImportMode, nil)
		return
	}

	// Ownership check + default currency for blank cells + subtype-driven shape
	// (404 if not owned).
	_, currency, subtype, err := h.repo.InvestmentImportMeta(r.Context(), investmentID)
	if err != nil {
		httperr.WriteRepo(w, "import: investment meta", err)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxImportUpload)
	file, _, err := r.FormFile("file")
	if err != nil {
		httperr.Write(w, http.StatusBadRequest, httperr.CodeInvalidFileUpload, nil)
		return
	}
	defer func() { _ = file.Close() }()

	parsed, rowErrs, err := snapshotimport.Parse(file, snapshotimport.Options{
		DefaultCurrency: currency,
		ValidCurrency:   func(c string) bool { return h.validate.Var(c, "iso4217") == nil },
		Shape:           shapeForSubtype(subtype),
	})
	if err != nil {
		httperr.Write(w, http.StatusBadRequest, httperr.CodeInvalidSpreadsheet, nil)
		return
	}

	resp := importResponse{Mode: mode, Errors: rowErrs}
	if resp.Errors == nil {
		resp.Errors = []snapshotimport.RowError{}
	}

	// commit refuses to write a file with any bad row — fix and re-upload.
	if mode == "commit" && len(rowErrs) > 0 {
		writeJSON(w, http.StatusUnprocessableEntity, resp)
		return
	}

	rows := make([]repo.ImportInvestmentSnapshotRow, len(parsed))
	for i, p := range parsed {
		rows[i] = repo.ImportInvestmentSnapshotRow{
			YearMonth:       p.YearMonth,
			Amount:          p.Amount,
			Currency:        p.Currency,
			Quantity:        p.Quantity,
			PricePerUnit:    p.PricePerUnit,
			AccruedInterest: p.AccruedInterest,
			AsOfDate:        p.AsOfDate,
			Description:     p.Description,
		}
	}

	dryRun := mode == "preview"
	res, err := h.repo.ImportInvestmentSnapshots(r.Context(), investmentID, rows, dryRun)
	if err != nil {
		httperr.WriteRepo(w, fmt.Sprintf("import snapshots (%s)", mode), err)
		return
	}

	resp.ToInsert = res.ToInsert
	resp.ToUpdate = res.ToUpdate
	resp.Unchanged = res.Unchanged
	resp.Committed = mode == "commit"
	writeJSON(w, http.StatusOK, resp)
}
