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

type createBondReq struct {
	DisplayName     string     `json:"display_name"       validate:"required"`
	Description     *string    `json:"description"`
	OwnershipType   string     `json:"ownership_type"     validate:"required,oneof=sole joint"`
	SoleOwnerUserID *uuid.UUID `json:"sole_owner_user_id" validate:"required_if=OwnershipType sole"`
	NativeCurrency  string     `json:"native_currency"    validate:"required,iso4217"`
	RiskProfile     string     `json:"risk_profile"       validate:"required,oneof=low medium high"`
	BondType        string     `json:"bond_type"          validate:"required,oneof=govt_primary secondary_market"`
	SeriesCode      *string    `json:"series_code"`
	Issuer          string     `json:"issuer"             validate:"required"`
	// FaceValue + PlacementDate seed the placement Buy for a govt_primary bond
	// (issue #27); secondary_market bonds omit them and record the real Buy
	// themselves. Required only for govt_primary.
	FaceValue         *decimal.Decimal `json:"face_value"         validate:"required_if=BondType govt_primary"`
	PlacementDate     string           `json:"placement_date"     validate:"required_if=BondType govt_primary"`
	CouponRate        *decimal.Decimal `json:"coupon_rate"         validate:"required"`
	CouponFrequency   string           `json:"coupon_frequency"    validate:"required,oneof=monthly quarterly semi_annual annual"`
	CouponDisposition string           `json:"coupon_disposition"  validate:"omitempty,oneof=pays_out accrues"`
	MaturityDate      string           `json:"maturity_date"       validate:"required"`
}

type updateBondReq struct {
	DisplayName       string           `json:"display_name"       validate:"required"`
	Description       *string          `json:"description"`
	OwnershipType     string           `json:"ownership_type"     validate:"required,oneof=sole joint"`
	SoleOwnerUserID   *uuid.UUID       `json:"sole_owner_user_id" validate:"required_if=OwnershipType sole"`
	RiskProfile       string           `json:"risk_profile"       validate:"required,oneof=low medium high"`
	BondType          string           `json:"bond_type"          validate:"required,oneof=govt_primary secondary_market"`
	SeriesCode        *string          `json:"series_code"`
	Issuer            string           `json:"issuer"             validate:"required"`
	CouponRate        *decimal.Decimal `json:"coupon_rate"         validate:"required"`
	CouponFrequency   string           `json:"coupon_frequency"    validate:"required,oneof=monthly quarterly semi_annual annual"`
	CouponDisposition string           `json:"coupon_disposition"  validate:"omitempty,oneof=pays_out accrues"`
	MaturityDate      string           `json:"maturity_date"       validate:"required"`
}

func (h *Handlers) handleCreateBond(w http.ResponseWriter, r *http.Request) {
	var req createBondReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httperr.Write(w, http.StatusBadRequest, httperr.CodeInvalidJSONBody, nil)
		return
	}
	if err := h.validate.Struct(&req); err != nil {
		httperr.WriteValidation(w, err)
		return
	}
	maturity, err := time.Parse("2006-01-02", req.MaturityDate)
	if err != nil {
		writeInvalidDate(w, "maturity_date")
		return
	}
	// PlacementDate is required only for govt_primary (validated above); parse it
	// when present so it can seed the placement Buy (issue #27).
	var placement *time.Time
	if req.PlacementDate != "" {
		p, err := time.Parse("2006-01-02", req.PlacementDate)
		if err != nil {
			writeInvalidDate(w, "placement_date")
			return
		}
		placement = &p
	}

	b, err := h.repo.CreateBond(r.Context(), repo.CreateBondParams{
		DisplayName:       req.DisplayName,
		Description:       req.Description,
		OwnershipType:     req.OwnershipType,
		SoleOwnerUserID:   req.SoleOwnerUserID,
		NativeCurrency:    req.NativeCurrency,
		RiskProfile:       req.RiskProfile,
		BondType:          req.BondType,
		SeriesCode:        req.SeriesCode,
		Issuer:            req.Issuer,
		FaceValue:         req.FaceValue,
		PlacementDate:     placement,
		CouponRate:        *req.CouponRate,
		CouponFrequency:   req.CouponFrequency,
		CouponDisposition: req.CouponDisposition,
		MaturityDate:      maturity,
	})
	if err != nil {
		httperr.WriteRepo(w, "create bond", err)
		return
	}
	writeJSON(w, http.StatusCreated, b)
}

func (h *Handlers) handleListBonds(w http.ResponseWriter, r *http.Request) {
	list, err := h.repo.ListBonds(r.Context())
	if err != nil {
		httperr.WriteRepo(w, "list bonds", err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (h *Handlers) handleGetBond(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		writeInvalidID(w, "id")
		return
	}
	b, err := h.repo.GetBond(r.Context(), id)
	if err != nil {
		httperr.WriteRepo(w, "get bond", err)
		return
	}
	writeJSON(w, http.StatusOK, b)
}

func (h *Handlers) handleUpdateBond(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		writeInvalidID(w, "id")
		return
	}
	var req updateBondReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httperr.Write(w, http.StatusBadRequest, httperr.CodeInvalidJSONBody, nil)
		return
	}
	if err := h.validate.Struct(&req); err != nil {
		httperr.WriteValidation(w, err)
		return
	}
	maturity, err := time.Parse("2006-01-02", req.MaturityDate)
	if err != nil {
		writeInvalidDate(w, "maturity_date")
		return
	}

	b, err := h.repo.UpdateBond(r.Context(), id, repo.UpdateBondParams{
		DisplayName:       req.DisplayName,
		Description:       req.Description,
		OwnershipType:     req.OwnershipType,
		SoleOwnerUserID:   req.SoleOwnerUserID,
		RiskProfile:       req.RiskProfile,
		BondType:          req.BondType,
		SeriesCode:        req.SeriesCode,
		Issuer:            req.Issuer,
		CouponRate:        *req.CouponRate,
		CouponFrequency:   req.CouponFrequency,
		CouponDisposition: req.CouponDisposition,
		MaturityDate:      maturity,
	})
	if err != nil {
		httperr.WriteRepo(w, "update bond", err)
		return
	}
	writeJSON(w, http.StatusOK, b)
}

func (h *Handlers) handleDeleteBond(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		writeInvalidID(w, "id")
		return
	}
	if err := h.repo.DeleteBond(r.Context(), id); err != nil {
		httperr.WriteRepo(w, "delete bond", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
