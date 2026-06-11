package liabilities

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/shopspring/decimal"

	"github.com/kerti/balances-v2/backend/internal/db"
	"github.com/kerti/balances-v2/backend/internal/httperr"
	"github.com/kerti/balances-v2/backend/internal/repo"
	"github.com/kerti/balances-v2/backend/internal/snapshotimport"
)

// handleExport streams a fully-populated position workbook for one liability —
// a "Detail" sheet (its fields) + a "Snapshots" sheet (its history) — in the
// importer's format, so the file round-trips back in through the unchanged
// snapshot-import flow on the detail page. Liabilities carry no transaction
// ledger, so there is no Transactions sheet.
func (h *Handlers) handleExport(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		writeInvalidID(w, "id")
		return
	}

	data, err := h.repo.ExportLiability(r.Context(), id)
	if err != nil {
		httperr.WriteRepo(w, "export liability", err)
		return
	}

	l := data.Liability
	xlsx, err := snapshotimport.BuildWorkbook(snapshotimport.TemplateMeta{
		PositionName:    l.DisplayName,
		DefaultCurrency: l.NativeCurrency,
		Detail:          liabilityDetailFields(data),
	}, liabilitySnapshotsToExport(data.Snapshots))
	if err != nil {
		httperr.WriteRepo(w, "export liability: build", err)
		return
	}

	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.xlsx"`, snapshotimport.ExportFilename(l.DisplayName, "liability-export")))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(xlsx)
}

// liabilityDetailFields maps a liability onto the Detail sheet's
// field/value/notes rows. Field order mirrors the create-request; the two
// id-typed fields follow the repo-wide conventions — ownership_type + a
// sole_owner email (blank for joint), and tag as the Tag's name.
func liabilityDetailFields(data *repo.LiabilityExport) []snapshotimport.DetailField {
	l := data.Liability
	return []snapshotimport.DetailField{
		{Key: "display_name", Value: l.DisplayName},
		{Key: "description", Value: derefStr(l.Description)},
		{Key: "subtype", Value: l.Subtype, Note: "personal | institutional"},
		{Key: "ownership_type", Value: l.OwnershipType, Note: "sole | joint"},
		{Key: "sole_owner", Value: data.OwnerEmail, Note: "owner's email; blank when joint"},
		{Key: "native_currency", Value: l.NativeCurrency, Note: "3-letter ISO code (e.g. IDR)"},
		{Key: "tag", Value: data.TagName, Note: "tag name; blank when untagged"},
		{Key: "counterparty_name", Value: l.CounterpartyName},
		{Key: "principal", Value: decStr(l.Principal), Note: "digits only, no thousands separators"},
		{Key: "interest_rate", Value: decStr(l.InterestRate), Note: "percent per year (e.g. 7.5)"},
		{Key: "term_months", Value: int32Str(l.TermMonths), Note: "loan term in months"},
		{Key: "start_date", Value: dateStr(l.StartDate), Note: "YYYY-MM-DD"},
		{Key: "maturity_date", Value: dateStr(l.MaturityDate), Note: "YYYY-MM-DD"},
	}
}

// liabilitySnapshotsToExport maps liability_snapshots rows (flat amount shape)
// onto the importer's ExportSnapshot — the Snapshots half of the workbook.
func liabilitySnapshotsToExport(snaps []db.LiabilitySnapshot) []snapshotimport.ExportSnapshot {
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

func decStr(p *decimal.Decimal) string {
	if p == nil {
		return ""
	}
	return p.String()
}

func int32Str(p *int32) string {
	if p == nil {
		return ""
	}
	return strconv.FormatInt(int64(*p), 10)
}

func dateStr(p *time.Time) string {
	if p == nil {
		return ""
	}
	return p.Format("2006-01-02")
}
