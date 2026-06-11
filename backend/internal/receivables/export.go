package receivables

import (
	"fmt"
	"net/http"
	"time"

	"github.com/kerti/balances-v2/backend/internal/db"
	"github.com/kerti/balances-v2/backend/internal/httperr"
	"github.com/kerti/balances-v2/backend/internal/repo"
	"github.com/kerti/balances-v2/backend/internal/snapshotimport"
)

// handleExport streams a fully-populated position workbook for one receivable —
// a "Detail" sheet (its fields) + a "Snapshots" sheet (its history) — in the
// importer's format, so the file round-trips back in through the unchanged
// snapshot-import flow on the detail page. Receivables carry no transaction
// ledger, so there is no Transactions sheet.
func (h *Handlers) handleExport(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		writeInvalidID(w, "id")
		return
	}

	data, err := h.repo.ExportReceivable(r.Context(), id)
	if err != nil {
		httperr.WriteRepo(w, "export receivable", err)
		return
	}

	rv := data.Receivable
	xlsx, err := snapshotimport.BuildWorkbook(snapshotimport.TemplateMeta{
		PositionName:    rv.DisplayName,
		DefaultCurrency: rv.NativeCurrency,
		Detail:          receivableDetailFields(data),
	}, receivableSnapshotsToExport(data.Snapshots))
	if err != nil {
		httperr.WriteRepo(w, "export receivable: build", err)
		return
	}

	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.xlsx"`, snapshotimport.ExportFilename(rv.DisplayName, "receivable-export")))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(xlsx)
}

// receivableDetailFields maps a receivable onto the Detail sheet's
// field/value/notes rows. Field order mirrors the create-request; the two
// id-typed fields follow the repo-wide conventions — ownership_type + a
// sole_owner email (blank for joint), and tag as the Tag's name.
func receivableDetailFields(data *repo.ReceivableExport) []snapshotimport.DetailField {
	rv := data.Receivable
	return []snapshotimport.DetailField{
		{Key: "display_name", Value: rv.DisplayName},
		{Key: "description", Value: derefStr(rv.Description)},
		{Key: "ownership_type", Value: rv.OwnershipType, Note: "sole | joint"},
		{Key: "sole_owner", Value: data.OwnerEmail, Note: "owner's email; blank when joint"},
		{Key: "native_currency", Value: rv.NativeCurrency, Note: "3-letter ISO code (e.g. IDR)"},
		{Key: "tag", Value: data.TagName, Note: "tag name; blank when untagged"},
		{Key: "counterparty_name", Value: rv.CounterpartyName},
		{Key: "due_date", Value: dateStr(rv.DueDate), Note: "YYYY-MM-DD"},
	}
}

// receivableSnapshotsToExport maps receivable_snapshots rows (flat amount shape)
// onto the importer's ExportSnapshot — the Snapshots half of the workbook.
func receivableSnapshotsToExport(snaps []db.ReceivableSnapshot) []snapshotimport.ExportSnapshot {
	out := make([]snapshotimport.ExportSnapshot, len(snaps))
	for i, s := range snaps {
		out[i] = snapshotimport.ExportSnapshot{
			YearMonth:   s.YearMonth,
			AsOfDate:    s.AsOfDate,
			Amount:      s.Amount,
			Currency:    s.Currency,
			Description: s.Description,
		}
	}
	return out
}

// ----- Detail-cell formatters for optional fields -------------------------

func derefStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func dateStr(p *time.Time) string {
	if p == nil {
		return ""
	}
	return p.Format("2006-01-02")
}
