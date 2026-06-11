package investments

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/kerti/balances-v2/backend/internal/db"
	"github.com/kerti/balances-v2/backend/internal/httperr"
	"github.com/kerti/balances-v2/backend/internal/repo"
	"github.com/kerti/balances-v2/backend/internal/snapshotimport"
)

// The export handlers stream a fully-populated position workbook for one
// investment — a "Detail" sheet (its create-request fields), a "Snapshots"
// sheet (its history, in the subtype's column shape), and a "Transactions"
// sheet (its full ledger, ADR-0023 column union). The file round-trips back in
// through the unchanged snapshot-import flow on the detail page: import reads
// only the Snapshots sheet, so Detail + Transactions are ignored there.

func (h *Handlers) handleExportStock(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		writeInvalidID(w, "id")
		return
	}
	data, err := h.repo.ExportStock(r.Context(), id)
	if err != nil {
		httperr.WriteRepo(w, "export stock", err)
		return
	}
	inv := data.Stock.Investment
	d := data.Stock.Details
	writeInvestmentWorkbook(w, inv, data.InvestmentExportCommon, "stock-export", []snapshotimport.DetailField{
		{Key: "risk_profile", Value: inv.RiskProfile, Note: "low | medium | high"},
		{Key: "ticker", Value: d.Ticker},
		{Key: "exchange", Value: d.Exchange},
	})
}

func (h *Handlers) handleExportMutualFund(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		writeInvalidID(w, "id")
		return
	}
	data, err := h.repo.ExportMutualFund(r.Context(), id)
	if err != nil {
		httperr.WriteRepo(w, "export mutual fund", err)
		return
	}
	inv := data.MutualFund.Investment
	d := data.MutualFund.Details
	writeInvestmentWorkbook(w, inv, data.InvestmentExportCommon, "mutual-fund-export", []snapshotimport.DetailField{
		{Key: "risk_profile", Value: inv.RiskProfile, Note: "low | medium | high"},
		{Key: "fund_code", Value: d.FundCode},
		{Key: "fund_manager", Value: derefStr(d.FundManager)},
		{Key: "fund_type", Value: d.FundType, Note: "money_market | fixed_income | equity | mixed"},
	})
}

func (h *Handlers) handleExportBond(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		writeInvalidID(w, "id")
		return
	}
	data, err := h.repo.ExportBond(r.Context(), id)
	if err != nil {
		httperr.WriteRepo(w, "export bond", err)
		return
	}
	inv := data.Bond.Investment
	d := data.Bond.Details
	writeInvestmentWorkbook(w, inv, data.InvestmentExportCommon, "bond-export", []snapshotimport.DetailField{
		{Key: "risk_profile", Value: inv.RiskProfile, Note: "low | medium | high"},
		{Key: "bond_type", Value: d.BondType, Note: "govt_primary | secondary_market"},
		{Key: "series_code", Value: derefStr(d.SeriesCode)},
		{Key: "issuer", Value: d.Issuer},
		{Key: "coupon_rate", Value: d.CouponRate.String(), Note: "percent per year (e.g. 6.5)"},
		{Key: "coupon_frequency", Value: d.CouponFrequency, Note: "monthly | quarterly | semi_annual | annual"},
		{Key: "maturity_date", Value: d.MaturityDate.Format("2006-01-02"), Note: "YYYY-MM-DD"},
	})
}

func (h *Handlers) handleExportGold(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		writeInvalidID(w, "id")
		return
	}
	data, err := h.repo.ExportGold(r.Context(), id)
	if err != nil {
		httperr.WriteRepo(w, "export gold", err)
		return
	}
	inv := data.Gold.Investment
	d := data.Gold.Details
	writeInvestmentWorkbook(w, inv, data.InvestmentExportCommon, "gold-export", []snapshotimport.DetailField{
		{Key: "risk_profile", Value: inv.RiskProfile, Note: "low | medium | high"},
		{Key: "form", Value: d.Form, Note: "bar | coin | digital | jewelry"},
		{Key: "purity", Value: d.Purity.String(), Note: "fineness, e.g. 0.999"},
	})
}

func (h *Handlers) handleExportTimeDeposit(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		writeInvalidID(w, "id")
		return
	}
	data, err := h.repo.ExportTimeDeposit(r.Context(), id)
	if err != nil {
		httperr.WriteRepo(w, "export time deposit", err)
		return
	}
	inv := data.TimeDeposit.Investment
	d := data.TimeDeposit.Details
	writeInvestmentWorkbook(w, inv, data.InvestmentExportCommon, "time-deposit-export", []snapshotimport.DetailField{
		{Key: "risk_profile", Value: inv.RiskProfile, Note: "low | medium | high"},
		{Key: "bank_name", Value: d.BankName},
		{Key: "principal", Value: d.Principal.String(), Note: "digits only, no thousands separators"},
		{Key: "interest_rate", Value: d.InterestRate.String(), Note: "percent per year (e.g. 6)"},
		{Key: "term_months", Value: strconv.FormatInt(int64(d.TermMonths), 10), Note: "deposit term in months"},
		{Key: "placement_date", Value: d.PlacementDate.Format("2006-01-02"), Note: "YYYY-MM-DD"},
		{Key: "maturity_date", Value: d.MaturityDate.Format("2006-01-02"), Note: "YYYY-MM-DD"},
		{Key: "rollover_policy", Value: d.RolloverPolicy, Note: "auto_renew_principal | auto_renew_with_interest | no_rollover"},
	})
}

// writeInvestmentWorkbook builds and streams the workbook for one investment.
// commonDetail are the shared identity fields every subtype carries; subtypeFields
// are the subtype-specific ones, appended after. Shape + Transactions are derived
// from the investment so the Snapshots column layout matches its subtype and the
// full ledger ships on its own sheet.
func writeInvestmentWorkbook(
	w http.ResponseWriter,
	inv db.Investment,
	common repo.InvestmentExportCommon,
	fallbackName string,
	subtypeFields []snapshotimport.DetailField,
) {
	detail := append(commonDetailFields(inv, common.OwnerEmail, common.TagName), subtypeFields...)

	xlsx, err := snapshotimport.BuildWorkbook(snapshotimport.TemplateMeta{
		PositionName:    inv.DisplayName,
		DefaultCurrency: inv.NativeCurrency,
		Shape:           shapeForSubtype(inv.Subtype),
		Detail:          detail,
		Transactions:    transactionsToExport(common.Transactions),
	}, investmentSnapshotsToExport(common.Snapshots))
	if err != nil {
		httperr.WriteRepo(w, "export investment: build", err)
		return
	}

	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.xlsx"`, snapshotimport.ExportFilename(inv.DisplayName, fallbackName)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(xlsx)
}

// commonDetailFields are the identity fields shared by every investment subtype,
// in create-request order. The two id-typed fields follow the repo-wide
// conventions — ownership_type + a sole_owner email (blank for joint), and tag
// as the Tag's name.
func commonDetailFields(inv db.Investment, ownerEmail, tagName string) []snapshotimport.DetailField {
	return []snapshotimport.DetailField{
		{Key: "display_name", Value: inv.DisplayName},
		{Key: "description", Value: derefStr(inv.Description)},
		{Key: "ownership_type", Value: inv.OwnershipType, Note: "sole | joint"},
		{Key: "sole_owner", Value: ownerEmail, Note: "owner's email; blank when joint"},
		{Key: "native_currency", Value: inv.NativeCurrency, Note: "3-letter ISO code (e.g. IDR)"},
		{Key: "tag", Value: tagName, Note: "tag name; blank when untagged"},
	}
}

// investmentSnapshotsToExport maps investment_snapshots rows onto the importer's
// ExportSnapshot. All shape columns ride along; BuildWorkbook writes only the
// ones the meta's Shape uses.
func investmentSnapshotsToExport(snaps []db.InvestmentSnapshot) []snapshotimport.ExportSnapshot {
	out := make([]snapshotimport.ExportSnapshot, len(snaps))
	for i, s := range snaps {
		out[i] = snapshotimport.ExportSnapshot{
			YearMonth:       s.YearMonth,
			AsOfDate:        s.AsOfDate,
			Amount:          s.Amount,
			Currency:        s.Currency,
			Description:     s.Description,
			Quantity:        s.Quantity,
			PricePerUnit:    s.PricePerUnit,
			AccruedInterest: s.AccruedInterest,
		}
	}
	return out
}

// transactionsToExport maps investment_transactions rows onto the importer's
// ExportTransaction (the Transactions ledger sheet). Always returns a non-nil
// slice so the sheet is emitted even for an investment with no transactions.
func transactionsToExport(txns []db.InvestmentTransaction) []snapshotimport.ExportTransaction {
	out := make([]snapshotimport.ExportTransaction, len(txns))
	for i, t := range txns {
		out[i] = snapshotimport.ExportTransaction{
			TransactionType:      t.TransactionType,
			TransactionDate:      t.TransactionDate,
			Currency:             t.Currency,
			Amount:               t.Amount,
			Quantity:             t.Quantity,
			PricePerUnit:         t.PricePerUnit,
			PrincipalAmount:      t.PrincipalAmount,
			InterestAmount:       t.InterestAmount,
			PrincipalDisposition: t.PrincipalDisposition,
			InterestDisposition:  t.InterestDisposition,
			Description:          t.Description,
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
