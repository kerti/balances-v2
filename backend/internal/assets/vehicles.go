package assets

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/kerti/balances-v2/backend/internal/httperr"
	"github.com/kerti/balances-v2/backend/internal/repo"
	"github.com/kerti/balances-v2/backend/internal/snapshotimport"
)

// ----- requests -----------------------------------------------------------

type createVehicleReq struct {
	DisplayName            string           `json:"display_name"               validate:"required"`
	Description            *string          `json:"description"`
	OwnershipType          string           `json:"ownership_type"             validate:"required,oneof=sole joint"`
	SoleOwnerUserID        *uuid.UUID       `json:"sole_owner_user_id"         validate:"required_if=OwnershipType sole"`
	NativeCurrency         string           `json:"native_currency"            validate:"required,iso4217"`
	VehicleType            string           `json:"vehicle_type"               validate:"required,oneof=car motorcycle other"`
	Make                   *string          `json:"make"`
	Model                  *string          `json:"model"`
	Year                   *int32           `json:"year"`
	PlateNumber            *string          `json:"plate_number"`
	AnnualDepreciationRate *decimal.Decimal `json:"annual_depreciation_rate"`
}

type updateVehicleReq struct {
	DisplayName            string           `json:"display_name"             validate:"required"`
	Description            *string          `json:"description"`
	OwnershipType          string           `json:"ownership_type"           validate:"required,oneof=sole joint"`
	SoleOwnerUserID        *uuid.UUID       `json:"sole_owner_user_id"       validate:"required_if=OwnershipType sole"`
	VehicleType            string           `json:"vehicle_type"             validate:"required,oneof=car motorcycle other"`
	Make                   *string          `json:"make"`
	Model                  *string          `json:"model"`
	Year                   *int32           `json:"year"`
	PlateNumber            *string          `json:"plate_number"`
	AnnualDepreciationRate *decimal.Decimal `json:"annual_depreciation_rate"`
}

// ----- handlers -----------------------------------------------------------

func (h *Handlers) handleCreateVehicle(w http.ResponseWriter, r *http.Request) {
	var req createVehicleReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httperr.Write(w, http.StatusBadRequest, httperr.CodeInvalidJSONBody, nil)
		return
	}
	if err := h.validate.Struct(&req); err != nil {
		httperr.WriteValidation(w, err)
		return
	}
	vehicle, err := h.repo.CreateVehicle(r.Context(), repo.CreateVehicleParams{
		DisplayName:            req.DisplayName,
		Description:            req.Description,
		OwnershipType:          req.OwnershipType,
		SoleOwnerUserID:        req.SoleOwnerUserID,
		NativeCurrency:         req.NativeCurrency,
		VehicleType:            req.VehicleType,
		Make:                   req.Make,
		Model:                  req.Model,
		Year:                   req.Year,
		PlateNumber:            req.PlateNumber,
		AnnualDepreciationRate: req.AnnualDepreciationRate,
	})
	if err != nil {
		httperr.WriteRepo(w, "create vehicle", err)
		return
	}
	writeJSON(w, http.StatusCreated, vehicle)
}

func (h *Handlers) handleListVehicles(w http.ResponseWriter, r *http.Request) {
	list, err := h.repo.ListVehicles(r.Context())
	if err != nil {
		httperr.WriteRepo(w, "list vehicles", err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (h *Handlers) handleGetVehicle(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		writeInvalidID(w, "id")
		return
	}
	vehicle, err := h.repo.GetVehicle(r.Context(), id)
	if err != nil {
		httperr.WriteRepo(w, "get vehicle", err)
		return
	}
	writeJSON(w, http.StatusOK, vehicle)
}

func (h *Handlers) handleUpdateVehicle(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		writeInvalidID(w, "id")
		return
	}
	var req updateVehicleReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httperr.Write(w, http.StatusBadRequest, httperr.CodeInvalidJSONBody, nil)
		return
	}
	if err := h.validate.Struct(&req); err != nil {
		httperr.WriteValidation(w, err)
		return
	}

	vehicle, err := h.repo.UpdateVehicle(r.Context(), id, repo.UpdateVehicleParams{
		DisplayName:            req.DisplayName,
		Description:            req.Description,
		OwnershipType:          req.OwnershipType,
		SoleOwnerUserID:        req.SoleOwnerUserID,
		VehicleType:            req.VehicleType,
		Make:                   req.Make,
		Model:                  req.Model,
		Year:                   req.Year,
		PlateNumber:            req.PlateNumber,
		AnnualDepreciationRate: req.AnnualDepreciationRate,
	})
	if err != nil {
		httperr.WriteRepo(w, "update vehicle", err)
		return
	}
	writeJSON(w, http.StatusOK, vehicle)
}

// handleExportVehicle streams a fully-populated position workbook for one
// vehicle — a "Detail" sheet (its fields) + a "Snapshots" sheet (its history)
// — in the importer's format, so the file round-trips back in through the
// unchanged snapshot-import flow on the detail page.
func (h *Handlers) handleExportVehicle(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		writeInvalidID(w, "id")
		return
	}

	data, err := h.repo.ExportVehicle(r.Context(), id)
	if err != nil {
		httperr.WriteRepo(w, "export vehicle", err)
		return
	}

	asset := data.Vehicle.Asset
	xlsx, err := snapshotimport.BuildWorkbook(snapshotimport.TemplateMeta{
		PositionName:    asset.DisplayName,
		DefaultCurrency: asset.NativeCurrency,
		Detail:          vehicleDetailFields(data),
	}, assetSnapshotsToExport(data.Snapshots))
	if err != nil {
		httperr.WriteRepo(w, "export vehicle: build", err)
		return
	}

	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.xlsx"`, snapshotimport.ExportFilename(asset.DisplayName, "vehicle-export")))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(xlsx)
}

// vehicleDetailFields maps a vehicle onto the Detail sheet's field/value/notes
// rows. Field order mirrors the create-request; the two id-typed fields follow
// the repo-wide conventions — ownership_type + a sole_owner email (blank for
// joint), and tag as the Tag's name.
func vehicleDetailFields(data *repo.VehicleExport) []snapshotimport.DetailField {
	asset := data.Vehicle.Asset
	details := data.Vehicle.Details
	return []snapshotimport.DetailField{
		{Key: "display_name", Value: asset.DisplayName},
		{Key: "description", Value: derefStr(asset.Description)},
		{Key: "ownership_type", Value: asset.OwnershipType, Note: "sole | joint"},
		{Key: "sole_owner", Value: data.OwnerEmail, Note: "owner's email; blank when joint"},
		{Key: "native_currency", Value: asset.NativeCurrency, Note: "3-letter ISO code (e.g. IDR)"},
		{Key: "tag", Value: data.TagName, Note: "tag name; blank when untagged"},
		{Key: "vehicle_type", Value: details.VehicleType, Note: "car | motorcycle | other"},
		{Key: "make", Value: derefStr(details.Make)},
		{Key: "model", Value: derefStr(details.Model)},
		{Key: "year", Value: int32Str(details.Year), Note: "4-digit model year"},
		{Key: "plate_number", Value: derefStr(details.PlateNumber)},
		{Key: "annual_depreciation_rate", Value: decStr(details.AnnualDepreciationRate), Note: "percent per year (e.g. 10)"},
	}
}

func (h *Handlers) handleDeleteVehicle(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		writeInvalidID(w, "id")
		return
	}
	if err := h.repo.DeleteVehicle(r.Context(), id); err != nil {
		httperr.WriteRepo(w, "delete vehicle", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
