package investments

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/shopspring/decimal"

	"github.com/kerti/balances-v2/backend/internal/dateguard"
	"github.com/kerti/balances-v2/backend/internal/httperr"
	"github.com/kerti/balances-v2/backend/internal/repo"
)

// createTransactionReq carries all value-shape columns; the repo validates
// which combination is required based on the declared transaction_type and
// the parent investment's subtype. Wrong combos return
// ErrInvalidTransactionType or ErrInvalidTransactionShape, both mapped to
// 400 in repoErrorStatus.
type createTransactionReq struct {
	TransactionType      string           `json:"transaction_type" validate:"required,oneof=buy sell coupon dividend distribution fee maturity"`
	TransactionDate      string           `json:"transaction_date" validate:"required"`
	Currency             string           `json:"currency"         validate:"required,iso4217"`
	Description          *string          `json:"description"`
	Amount               *decimal.Decimal `json:"amount"`
	Quantity             *decimal.Decimal `json:"quantity"`
	PricePerUnit         *decimal.Decimal `json:"price_per_unit"`
	PrincipalAmount      *decimal.Decimal `json:"principal_amount"`
	InterestAmount       *decimal.Decimal `json:"interest_amount"`
	PrincipalDisposition *string          `json:"principal_disposition" validate:"omitempty,oneof=rolled_to_new cash_out"`
	InterestDisposition  *string          `json:"interest_disposition"  validate:"omitempty,oneof=rolled_to_new cash_out"`
}

// updateTransactionReq does not include transaction_type — it's immutable
// on an existing row (changing the type would invalidate the shape).
type updateTransactionReq struct {
	TransactionDate      string           `json:"transaction_date" validate:"required"`
	Currency             string           `json:"currency"         validate:"required,iso4217"`
	Description          *string          `json:"description"`
	Amount               *decimal.Decimal `json:"amount"`
	Quantity             *decimal.Decimal `json:"quantity"`
	PricePerUnit         *decimal.Decimal `json:"price_per_unit"`
	PrincipalAmount      *decimal.Decimal `json:"principal_amount"`
	InterestAmount       *decimal.Decimal `json:"interest_amount"`
	PrincipalDisposition *string          `json:"principal_disposition" validate:"omitempty,oneof=rolled_to_new cash_out"`
	InterestDisposition  *string          `json:"interest_disposition"  validate:"omitempty,oneof=rolled_to_new cash_out"`
}

func (h *Handlers) handleCreateTransaction(w http.ResponseWriter, r *http.Request) {
	investmentID, err := parseIDParam(r, "id")
	if err != nil {
		writeInvalidID(w, "id")
		return
	}

	var req createTransactionReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httperr.Write(w, http.StatusBadRequest, httperr.CodeInvalidJSONBody, nil)
		return
	}
	if err := h.validate.Struct(&req); err != nil {
		httperr.WriteValidation(w, err)
		return
	}

	txnDate, err := time.Parse("2006-01-02", req.TransactionDate)
	if err != nil {
		writeInvalidDate(w, "transaction_date")
		return
	}
	if dateguard.IsFuture(txnDate, h.now()) {
		httperr.Write(w, http.StatusBadRequest, httperr.CodeTransactionFutureDate, nil)
		return
	}

	txn, err := h.repo.CreateInvestmentTransaction(r.Context(), repo.CreateInvestmentTransactionParams{
		InvestmentID:         investmentID,
		TransactionType:      req.TransactionType,
		TransactionDate:      txnDate,
		Currency:             req.Currency,
		Description:          req.Description,
		Amount:               req.Amount,
		Quantity:             req.Quantity,
		PricePerUnit:         req.PricePerUnit,
		PrincipalAmount:      req.PrincipalAmount,
		InterestAmount:       req.InterestAmount,
		PrincipalDisposition: req.PrincipalDisposition,
		InterestDisposition:  req.InterestDisposition,
	})
	if err != nil {
		httperr.WriteRepo(w, "create investment transaction", err)
		return
	}
	writeJSON(w, http.StatusCreated, txn)
}

func (h *Handlers) handleListTransactions(w http.ResponseWriter, r *http.Request) {
	investmentID, err := parseIDParam(r, "id")
	if err != nil {
		writeInvalidID(w, "id")
		return
	}
	txns, err := h.repo.ListInvestmentTransactions(r.Context(), investmentID)
	if err != nil {
		httperr.WriteRepo(w, "list investment transactions", err)
		return
	}
	writeJSON(w, http.StatusOK, txns)
}

func (h *Handlers) handleUpdateTransaction(w http.ResponseWriter, r *http.Request) {
	transactionID, err := parseIDParam(r, "transactionID")
	if err != nil {
		writeInvalidID(w, "transaction_id")
		return
	}

	var req updateTransactionReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httperr.Write(w, http.StatusBadRequest, httperr.CodeInvalidJSONBody, nil)
		return
	}
	if err := h.validate.Struct(&req); err != nil {
		httperr.WriteValidation(w, err)
		return
	}

	txnDate, err := time.Parse("2006-01-02", req.TransactionDate)
	if err != nil {
		writeInvalidDate(w, "transaction_date")
		return
	}
	if dateguard.IsFuture(txnDate, h.now()) {
		httperr.Write(w, http.StatusBadRequest, httperr.CodeTransactionFutureDate, nil)
		return
	}

	txn, err := h.repo.UpdateInvestmentTransaction(r.Context(), repo.UpdateInvestmentTransactionParams{
		TransactionID:        transactionID,
		TransactionDate:      txnDate,
		Currency:             req.Currency,
		Description:          req.Description,
		Amount:               req.Amount,
		Quantity:             req.Quantity,
		PricePerUnit:         req.PricePerUnit,
		PrincipalAmount:      req.PrincipalAmount,
		InterestAmount:       req.InterestAmount,
		PrincipalDisposition: req.PrincipalDisposition,
		InterestDisposition:  req.InterestDisposition,
	})
	if err != nil {
		httperr.WriteRepo(w, "update investment transaction", err)
		return
	}
	writeJSON(w, http.StatusOK, txn)
}

func (h *Handlers) handleDeleteTransaction(w http.ResponseWriter, r *http.Request) {
	transactionID, err := parseIDParam(r, "transactionID")
	if err != nil {
		writeInvalidID(w, "transaction_id")
		return
	}
	if err := h.repo.DeleteInvestmentTransaction(r.Context(), transactionID); err != nil {
		httperr.WriteRepo(w, "delete investment transaction", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
