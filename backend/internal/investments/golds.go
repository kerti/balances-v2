package investments

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/kerti/balances-v2/backend/internal/httperr"
	"github.com/kerti/balances-v2/backend/internal/repo"
)

type createGoldReq struct {
	DisplayName     string           `json:"display_name"       validate:"required"`
	Description     *string          `json:"description"`
	OwnershipType   string           `json:"ownership_type"     validate:"required,oneof=sole joint"`
	SoleOwnerUserID *uuid.UUID       `json:"sole_owner_user_id" validate:"required_if=OwnershipType sole"`
	NativeCurrency  string           `json:"native_currency"    validate:"required,iso4217"`
	RiskProfile     string           `json:"risk_profile"       validate:"required,oneof=low medium high"`
	Form            string           `json:"form"               validate:"required,oneof=bar coin digital jewelry"`
	Purity          *decimal.Decimal `json:"purity"             validate:"required"`
	// EntryType declares where the Position came from (ADR-0053 §3). Optional on
	// the wire: absent means `acquired`, the column DEFAULT and the pre-ADR-0053
	// behaviour, so a client that never learned the field still creates genuine
	// acquisitions.
	EntryType string `json:"entry_type" validate:"omitempty,oneof=acquired newly_tracked"`
}

type updateGoldReq struct {
	DisplayName     string           `json:"display_name"       validate:"required"`
	Description     *string          `json:"description"`
	OwnershipType   string           `json:"ownership_type"     validate:"required,oneof=sole joint"`
	SoleOwnerUserID *uuid.UUID       `json:"sole_owner_user_id" validate:"required_if=OwnershipType sole"`
	RiskProfile     string           `json:"risk_profile"       validate:"required,oneof=low medium high"`
	Form            string           `json:"form"               validate:"required,oneof=bar coin digital jewelry"`
	Purity          *decimal.Decimal `json:"purity"             validate:"required"`
	// EntryType re-declares where the Position came from. Absent (null) leaves the
	// existing declaration alone rather than resetting it to `acquired` — silently
	// undoing a `newly_tracked` declaration is the residual ADR-0053 warns about.
	EntryType *string `json:"entry_type" validate:"omitempty,oneof=acquired newly_tracked"`
}

func (h *Handlers) handleCreateGold(w http.ResponseWriter, r *http.Request) {
	var req createGoldReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httperr.Write(w, http.StatusBadRequest, httperr.CodeInvalidJSONBody, nil)
		return
	}
	if err := h.validate.Struct(&req); err != nil {
		httperr.WriteValidation(w, err)
		return
	}

	g, err := h.repo.CreateGold(r.Context(), repo.CreateGoldParams{
		DisplayName:     req.DisplayName,
		Description:     req.Description,
		OwnershipType:   req.OwnershipType,
		SoleOwnerUserID: req.SoleOwnerUserID,
		NativeCurrency:  req.NativeCurrency,
		RiskProfile:     req.RiskProfile,
		Form:            req.Form,
		Purity:          *req.Purity,
		EntryType:       req.EntryType,
	})
	if err != nil {
		httperr.WriteRepo(w, "create gold", err)
		return
	}
	writeJSON(w, http.StatusCreated, g)
}

func (h *Handlers) handleListGolds(w http.ResponseWriter, r *http.Request) {
	list, err := h.repo.ListGolds(r.Context())
	if err != nil {
		httperr.WriteRepo(w, "list golds", err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (h *Handlers) handleGetGold(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		writeInvalidID(w, "id")
		return
	}
	g, err := h.repo.GetGold(r.Context(), id)
	if err != nil {
		httperr.WriteRepo(w, "get gold", err)
		return
	}
	writeJSON(w, http.StatusOK, g)
}

func (h *Handlers) handleUpdateGold(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		writeInvalidID(w, "id")
		return
	}
	var req updateGoldReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httperr.Write(w, http.StatusBadRequest, httperr.CodeInvalidJSONBody, nil)
		return
	}
	if err := h.validate.Struct(&req); err != nil {
		httperr.WriteValidation(w, err)
		return
	}

	g, err := h.repo.UpdateGold(r.Context(), id, repo.UpdateGoldParams{
		DisplayName:     req.DisplayName,
		Description:     req.Description,
		OwnershipType:   req.OwnershipType,
		SoleOwnerUserID: req.SoleOwnerUserID,
		RiskProfile:     req.RiskProfile,
		Form:            req.Form,
		Purity:          *req.Purity,
		EntryType:       req.EntryType,
	})
	if err != nil {
		httperr.WriteRepo(w, "update gold", err)
		return
	}
	writeJSON(w, http.StatusOK, g)
}

func (h *Handlers) handleDeleteGold(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		writeInvalidID(w, "id")
		return
	}
	if err := h.repo.DeleteGold(r.Context(), id); err != nil {
		httperr.WriteRepo(w, "delete gold", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
