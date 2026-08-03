package investments

import (
	"encoding/json"
	"net/http"

	"github.com/shopspring/decimal"

	"github.com/kerti/balances-v2/backend/internal/httperr"
	"github.com/kerti/balances-v2/backend/internal/repo"
)

// updateLifecycleReq is the body for PATCH /investments/{id}/lifecycle. See the
// assets package twin for the validation rationale. Note: a Bond/TimeDeposit
// reaches 'matured' automatically via a Maturity transaction (the repo flips
// it); this endpoint covers manual terminal states (e.g. a Stock sold off).
type updateLifecycleReq struct {
	Status          string  `json:"status"           validate:"required"`
	TerminatedAt    *string `json:"terminated_at"    validate:"required_unless=Status active"`
	TerminationNote *string `json:"termination_note"`
	// Settlement is the ADR-0052 §6 capture-at-source payload, unique to this
	// group: the terminal Sell/Maturity the terminate dialog books atomically
	// with the flip. Omitted by every other caller (and by an un-terminate), in
	// which case the ledger is left alone and the report engine's
	// unsettled-termination advisory does its job instead.
	Settlement *settlementReq `json:"settlement"`
}

// settlementReq is subtype-shaped, mirroring createTransactionReq: the pair the
// resolved transaction type needs is supplied and the other pair left null. The
// repo resolves which pair that is from the subtype and the terminal status, and
// rejects a mismatch as ErrInvalidTransactionShape (→ 400).
type settlementReq struct {
	Quantity        *decimal.Decimal `json:"quantity"`
	PricePerUnit    *decimal.Decimal `json:"price_per_unit"`
	PrincipalAmount *decimal.Decimal `json:"principal_amount"`
	InterestAmount  *decimal.Decimal `json:"interest_amount"`
}

func (h *Handlers) handleUpdateLifecycle(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		writeInvalidID(w, "id")
		return
	}
	var req updateLifecycleReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httperr.Write(w, http.StatusBadRequest, httperr.CodeInvalidJSONBody, nil)
		return
	}
	if err := h.validate.Struct(&req); err != nil {
		httperr.WriteValidation(w, err)
		return
	}
	terminatedAt, ok := parseOptionalDate(req.TerminatedAt)
	if !ok {
		writeInvalidDate(w, "terminated_at")
		return
	}

	var settle *repo.InvestmentSettlement
	if req.Settlement != nil {
		settle = &repo.InvestmentSettlement{
			Quantity:        req.Settlement.Quantity,
			PricePerUnit:    req.Settlement.PricePerUnit,
			PrincipalAmount: req.Settlement.PrincipalAmount,
			InterestAmount:  req.Settlement.InterestAmount,
		}
	}

	investment, err := h.repo.UpdateInvestmentLifecycle(r.Context(), id, repo.LifecycleParams{
		Status:          req.Status,
		TerminatedAt:    terminatedAt,
		TerminationNote: req.TerminationNote,
	}, settle)
	if err != nil {
		httperr.WriteRepo(w, "update investment lifecycle", err)
		return
	}
	writeJSON(w, http.StatusOK, investment)
}
