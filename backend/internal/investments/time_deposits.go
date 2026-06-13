package investments

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/kerti/balances-v2/backend/internal/httperr"
	"github.com/kerti/balances-v2/backend/internal/repo"
)

type createTimeDepositReq struct {
	DisplayName     string           `json:"display_name"       validate:"required"`
	Description     *string          `json:"description"`
	OwnershipType   string           `json:"ownership_type"     validate:"required,oneof=sole joint"`
	SoleOwnerUserID *uuid.UUID       `json:"sole_owner_user_id" validate:"required_if=OwnershipType sole"`
	NativeCurrency  string           `json:"native_currency"    validate:"required,iso4217"`
	RiskProfile     string           `json:"risk_profile"       validate:"required,oneof=low medium high"`
	BankName        string           `json:"bank_name"          validate:"required"`
	Principal       *decimal.Decimal `json:"principal"          validate:"required"`
	InterestRate    *decimal.Decimal `json:"interest_rate"      validate:"required"`
	TermMonths      int32            `json:"term_months"        validate:"required,gt=0"`
	PlacementDate   string           `json:"placement_date"     validate:"required"`
	MaturityDate    string           `json:"maturity_date"      validate:"required"`
	RolloverPolicy  string           `json:"rollover_policy"    validate:"required,oneof=auto_renew_principal auto_renew_with_interest no_rollover"`
	// RolledFromInvestmentID links a rollover successor to its matured source
	// (issue #29). Set by the frontend rollover helper; absent on fresh deposits.
	RolledFromInvestmentID *uuid.UUID `json:"rolled_from_investment_id"`
}

type updateTimeDepositReq struct {
	DisplayName     string           `json:"display_name"       validate:"required"`
	Description     *string          `json:"description"`
	OwnershipType   string           `json:"ownership_type"     validate:"required,oneof=sole joint"`
	SoleOwnerUserID *uuid.UUID       `json:"sole_owner_user_id" validate:"required_if=OwnershipType sole"`
	RiskProfile     string           `json:"risk_profile"       validate:"required,oneof=low medium high"`
	BankName        string           `json:"bank_name"          validate:"required"`
	Principal       *decimal.Decimal `json:"principal"          validate:"required"`
	InterestRate    *decimal.Decimal `json:"interest_rate"      validate:"required"`
	TermMonths      int32            `json:"term_months"        validate:"required,gt=0"`
	PlacementDate   string           `json:"placement_date"     validate:"required"`
	MaturityDate    string           `json:"maturity_date"      validate:"required"`
	RolloverPolicy  string           `json:"rollover_policy"    validate:"required,oneof=auto_renew_principal auto_renew_with_interest no_rollover"`
}

func (h *Handlers) handleCreateTimeDeposit(w http.ResponseWriter, r *http.Request) {
	var req createTimeDepositReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httperr.Write(w, http.StatusBadRequest, httperr.CodeInvalidJSONBody, nil)
		return
	}
	if err := h.validate.Struct(&req); err != nil {
		httperr.WriteValidation(w, err)
		return
	}
	placement, err := time.Parse("2006-01-02", req.PlacementDate)
	if err != nil {
		writeInvalidDate(w, "placement_date")
		return
	}
	maturity, err := time.Parse("2006-01-02", req.MaturityDate)
	if err != nil {
		writeInvalidDate(w, "maturity_date")
		return
	}

	t, err := h.repo.CreateTimeDeposit(r.Context(), repo.CreateTimeDepositParams{
		DisplayName:            req.DisplayName,
		Description:            req.Description,
		OwnershipType:          req.OwnershipType,
		SoleOwnerUserID:        req.SoleOwnerUserID,
		NativeCurrency:         req.NativeCurrency,
		RiskProfile:            req.RiskProfile,
		BankName:               req.BankName,
		Principal:              *req.Principal,
		InterestRate:           *req.InterestRate,
		TermMonths:             req.TermMonths,
		PlacementDate:          placement,
		MaturityDate:           maturity,
		RolloverPolicy:         req.RolloverPolicy,
		RolledFromInvestmentID: req.RolledFromInvestmentID,
	})
	if err != nil {
		httperr.WriteRepo(w, "create time deposit", err)
		return
	}
	writeJSON(w, http.StatusCreated, t)
}

func (h *Handlers) handleListTimeDeposits(w http.ResponseWriter, r *http.Request) {
	list, err := h.repo.ListTimeDeposits(r.Context())
	if err != nil {
		httperr.WriteRepo(w, "list time deposits", err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (h *Handlers) handleGetTimeDeposit(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		writeInvalidID(w, "id")
		return
	}
	t, err := h.repo.GetTimeDeposit(r.Context(), id)
	if err != nil {
		httperr.WriteRepo(w, "get time deposit", err)
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (h *Handlers) handleUpdateTimeDeposit(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		writeInvalidID(w, "id")
		return
	}
	var req updateTimeDepositReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httperr.Write(w, http.StatusBadRequest, httperr.CodeInvalidJSONBody, nil)
		return
	}
	if err := h.validate.Struct(&req); err != nil {
		httperr.WriteValidation(w, err)
		return
	}
	placement, err := time.Parse("2006-01-02", req.PlacementDate)
	if err != nil {
		writeInvalidDate(w, "placement_date")
		return
	}
	maturity, err := time.Parse("2006-01-02", req.MaturityDate)
	if err != nil {
		writeInvalidDate(w, "maturity_date")
		return
	}

	td, err := h.repo.UpdateTimeDeposit(r.Context(), id, repo.UpdateTimeDepositParams{
		DisplayName:     req.DisplayName,
		Description:     req.Description,
		OwnershipType:   req.OwnershipType,
		SoleOwnerUserID: req.SoleOwnerUserID,
		RiskProfile:     req.RiskProfile,
		BankName:        req.BankName,
		Principal:       *req.Principal,
		InterestRate:    *req.InterestRate,
		TermMonths:      req.TermMonths,
		PlacementDate:   placement,
		MaturityDate:    maturity,
		RolloverPolicy:  req.RolloverPolicy,
	})
	if err != nil {
		httperr.WriteRepo(w, "update time deposit", err)
		return
	}
	writeJSON(w, http.StatusOK, td)
}

type linkRolloverSuccessorReq struct {
	// SuccessorID is the existing deposit that redeployed this (matured) one's
	// funds (issue #65). Stamping its rolled_from_investment_id clears this
	// position's rollover callout.
	SuccessorID uuid.UUID `json:"successor_id" validate:"required"`
}

func (h *Handlers) handleLinkRolloverSuccessor(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		writeInvalidID(w, "id")
		return
	}
	var req linkRolloverSuccessorReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httperr.Write(w, http.StatusBadRequest, httperr.CodeInvalidJSONBody, nil)
		return
	}
	if err := h.validate.Struct(&req); err != nil {
		httperr.WriteValidation(w, err)
		return
	}
	td, err := h.repo.LinkRolloverSuccessor(r.Context(), id, req.SuccessorID)
	if err != nil {
		httperr.WriteRepo(w, "link rollover successor", err)
		return
	}
	writeJSON(w, http.StatusOK, td)
}

func (h *Handlers) handleDeleteTimeDeposit(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		writeInvalidID(w, "id")
		return
	}
	if err := h.repo.DeleteTimeDeposit(r.Context(), id); err != nil {
		httperr.WriteRepo(w, "delete time deposit", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
