package assets

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/google/uuid"

	"github.com/kerti/balances-v2/backend/internal/httperr"
	"github.com/kerti/balances-v2/backend/internal/repo"
	"github.com/kerti/balances-v2/backend/internal/snapshotimport"
)

// ----- requests -----------------------------------------------------------

type createBankAccountReq struct {
	DisplayName     string     `json:"display_name"      validate:"required"`
	Description     *string    `json:"description"`
	OwnershipType   string     `json:"ownership_type"    validate:"required,oneof=sole joint"`
	SoleOwnerUserID *uuid.UUID `json:"sole_owner_user_id" validate:"required_if=OwnershipType sole"`
	NativeCurrency  string     `json:"native_currency"   validate:"required,iso4217"`
	BankName        string     `json:"bank_name"         validate:"required"`
	AccountNumber   string     `json:"account_number"    validate:"required"`
	AccountType     string     `json:"account_type"      validate:"required,oneof=savings current other"`
}

type updateBankAccountReq struct {
	DisplayName     string     `json:"display_name"       validate:"required"`
	Description     *string    `json:"description"`
	OwnershipType   string     `json:"ownership_type"     validate:"required,oneof=sole joint"`
	SoleOwnerUserID *uuid.UUID `json:"sole_owner_user_id" validate:"required_if=OwnershipType sole"`
	BankName        string     `json:"bank_name"          validate:"required"`
	AccountNumber   string     `json:"account_number"     validate:"required"`
	AccountType     string     `json:"account_type"       validate:"required,oneof=savings current other"`
}

// ----- handlers -----------------------------------------------------------

func (h *Handlers) handleCreateBankAccount(w http.ResponseWriter, r *http.Request) {
	var req createBankAccountReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httperr.Write(w, http.StatusBadRequest, httperr.CodeInvalidJSONBody, nil)
		return
	}
	if err := h.validate.Struct(&req); err != nil {
		httperr.WriteValidation(w, err)
		return
	}

	account, err := h.repo.CreateBankAccount(r.Context(), repo.CreateBankAccountParams{
		DisplayName:     req.DisplayName,
		Description:     req.Description,
		OwnershipType:   req.OwnershipType,
		SoleOwnerUserID: req.SoleOwnerUserID,
		NativeCurrency:  req.NativeCurrency,
		BankName:        req.BankName,
		AccountNumber:   req.AccountNumber,
		AccountType:     req.AccountType,
	})
	if err != nil {
		httperr.WriteRepo(w, "create bank account", err)
		return
	}
	writeJSON(w, http.StatusCreated, account)
}

func (h *Handlers) handleListBankAccounts(w http.ResponseWriter, r *http.Request) {
	list, err := h.repo.ListBankAccounts(r.Context())
	if err != nil {
		httperr.WriteRepo(w, "list bank accounts", err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (h *Handlers) handleGetBankAccount(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		writeInvalidID(w, "id")
		return
	}
	account, err := h.repo.GetBankAccount(r.Context(), id)
	if err != nil {
		httperr.WriteRepo(w, "get bank account", err)
		return
	}
	writeJSON(w, http.StatusOK, account)
}

func (h *Handlers) handleUpdateBankAccount(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		writeInvalidID(w, "id")
		return
	}
	var req updateBankAccountReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httperr.Write(w, http.StatusBadRequest, httperr.CodeInvalidJSONBody, nil)
		return
	}
	if err := h.validate.Struct(&req); err != nil {
		httperr.WriteValidation(w, err)
		return
	}

	account, err := h.repo.UpdateBankAccount(r.Context(), id, repo.UpdateBankAccountParams{
		DisplayName:     req.DisplayName,
		Description:     req.Description,
		OwnershipType:   req.OwnershipType,
		SoleOwnerUserID: req.SoleOwnerUserID,
		BankName:        req.BankName,
		AccountNumber:   req.AccountNumber,
		AccountType:     req.AccountType,
	})
	if err != nil {
		httperr.WriteRepo(w, "update bank account", err)
		return
	}
	writeJSON(w, http.StatusOK, account)
}

// handleExportBankAccount streams a fully-populated position workbook for one
// bank account — a "Detail" sheet (its fields) + a "Snapshots" sheet (its
// history) — in the exact format the importer reads, so the file round-trips
// back in through the unchanged snapshot-import flow.
func (h *Handlers) handleExportBankAccount(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		writeInvalidID(w, "id")
		return
	}

	data, err := h.repo.ExportBankAccount(r.Context(), id)
	if err != nil {
		httperr.WriteRepo(w, "export bank account", err)
		return
	}

	asset := data.Account.Asset
	snaps := make([]snapshotimport.ExportSnapshot, len(data.Snapshots))
	for i, s := range data.Snapshots {
		snaps[i] = snapshotimport.ExportSnapshot{
			YearMonth:   s.YearMonth,
			AsOfDate:    s.AsOfDate,
			Amount:      s.Amount,
			Currency:    s.Currency,
			Description: s.Description,
		}
	}

	xlsx, err := snapshotimport.BuildWorkbook(snapshotimport.TemplateMeta{
		PositionName:    asset.DisplayName,
		DefaultCurrency: asset.NativeCurrency,
		Detail:          bankAccountDetailFields(data),
	}, snaps)
	if err != nil {
		httperr.WriteRepo(w, "export bank account: build", err)
		return
	}

	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.xlsx"`, exportFilename(asset.DisplayName)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(xlsx)
}

// bankAccountDetailFields maps a bank account onto the Detail sheet's
// field/value/notes rows. Field order mirrors the create-request; the two
// id-typed fields follow the repo-wide conventions — ownership_type + a
// sole_owner email (blank for joint), and tag as the Tag's name.
func bankAccountDetailFields(data *repo.BankAccountExport) []snapshotimport.DetailField {
	asset := data.Account.Asset
	details := data.Account.Details
	desc := ""
	if asset.Description != nil {
		desc = *asset.Description
	}
	return []snapshotimport.DetailField{
		{Key: "display_name", Value: asset.DisplayName},
		{Key: "description", Value: desc},
		{Key: "ownership_type", Value: asset.OwnershipType, Note: "sole | joint"},
		{Key: "sole_owner", Value: data.OwnerEmail, Note: "owner's email; blank when joint"},
		{Key: "native_currency", Value: asset.NativeCurrency, Note: "3-letter ISO code (e.g. IDR)"},
		{Key: "tag", Value: data.TagName, Note: "tag name; blank when untagged"},
		{Key: "bank_name", Value: details.BankName},
		{Key: "account_number", Value: details.AccountNumber},
		{Key: "account_type", Value: details.AccountType, Note: "savings | current | other"},
	}
}

// nonFilenameChars collapses anything outside a safe filename set to a single
// dash, so a display name can't smuggle quotes/newlines into Content-Disposition.
var nonFilenameChars = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

func exportFilename(displayName string) string {
	slug := strings.Trim(nonFilenameChars.ReplaceAllString(displayName, "-"), "-")
	if slug == "" {
		return "bank-account-export"
	}
	return slug + "-export"
}

func (h *Handlers) handleDeleteBankAccount(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		writeInvalidID(w, "id")
		return
	}
	if err := h.repo.DeleteBankAccount(r.Context(), id); err != nil {
		httperr.WriteRepo(w, "delete bank account", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
