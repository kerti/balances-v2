package assets

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/kerti/balances-v2/backend/internal/importcreate"
	"github.com/kerti/balances-v2/backend/internal/repo"
	"github.com/kerti/balances-v2/backend/internal/snapshotimport"
)

// Create-from-file import for the asset-table groups (bank account, property,
// vehicle): the uploaded position workbook's Detail sheet becomes the position,
// its Snapshots sheet seeds the history — atomically on commit. The shared
// transport + preview/commit gate + Detail-cell parsing live in
// internal/importcreate; only the per-group field mapping (the resolve methods
// below) and the repo write differ. See issue #88 (bank account) / #89 (fan-out).

// ----- bank account -------------------------------------------------------

func (h *Handlers) handleImportCreateBankAccount(w http.ResponseWriter, r *http.Request) {
	importcreate.Run(w, r, h.validate, h.resolveBankAccountDetail,
		func(ctx context.Context, p repo.CreateBankAccountParams, tagID *uuid.UUID, rows []repo.ImportSnapshotRow) (uuid.UUID, error) {
			acct, err := h.repo.CreateBankAccountWithSnapshots(ctx, p, tagID, rows)
			if err != nil {
				return uuid.Nil, err
			}
			return acct.Asset.ID, nil
		})
}

func (h *Handlers) resolveBankAccountDetail(ctx context.Context, detail map[string]string) (repo.CreateBankAccountParams, *uuid.UUID, []snapshotimport.FieldError, error) {
	soleOwnerID, emailHandled, fieldErrs, err := importcreate.ResolveSoleOwner(
		ctx, detail["ownership_type"], detail["sole_owner"], h.repo.LookupUserIDByEmail)
	if err != nil {
		return repo.CreateBankAccountParams{}, nil, nil, err
	}

	req := createBankAccountReq{
		DisplayName:     strings.TrimSpace(detail["display_name"]),
		Description:     importcreate.OptionalStr(detail["description"]),
		OwnershipType:   strings.TrimSpace(detail["ownership_type"]),
		SoleOwnerUserID: soleOwnerID,
		NativeCurrency:  strings.TrimSpace(detail["native_currency"]),
		BankName:        strings.TrimSpace(detail["bank_name"]),
		AccountNumber:   strings.TrimSpace(detail["account_number"]),
		AccountType:     strings.TrimSpace(detail["account_type"]),
	}
	fieldErrs = append(fieldErrs, validateStruct(h, &req, emailHandled)...)

	tagID, err := h.repo.LookupTagIDByName(ctx, strings.TrimSpace(detail["tag"]))
	if err != nil {
		return repo.CreateBankAccountParams{}, nil, nil, err
	}

	params := repo.CreateBankAccountParams{
		DisplayName:     req.DisplayName,
		Description:     req.Description,
		OwnershipType:   req.OwnershipType,
		SoleOwnerUserID: req.SoleOwnerUserID,
		NativeCurrency:  req.NativeCurrency,
		BankName:        req.BankName,
		AccountNumber:   req.AccountNumber,
		AccountType:     req.AccountType,
	}
	return params, tagID, fieldErrs, nil
}

// ----- property -----------------------------------------------------------

func (h *Handlers) handleImportCreateProperty(w http.ResponseWriter, r *http.Request) {
	importcreate.Run(w, r, h.validate, h.resolvePropertyDetail,
		func(ctx context.Context, p repo.CreatePropertyParams, tagID *uuid.UUID, rows []repo.ImportSnapshotRow) (uuid.UUID, error) {
			property, err := h.repo.CreatePropertyWithSnapshots(ctx, p, tagID, rows)
			if err != nil {
				return uuid.Nil, err
			}
			return property.Asset.ID, nil
		})
}

func (h *Handlers) resolvePropertyDetail(ctx context.Context, detail map[string]string) (repo.CreatePropertyParams, *uuid.UUID, []snapshotimport.FieldError, error) {
	soleOwnerID, emailHandled, fieldErrs, err := importcreate.ResolveSoleOwner(
		ctx, detail["ownership_type"], detail["sole_owner"], h.repo.LookupUserIDByEmail)
	if err != nil {
		return repo.CreatePropertyParams{}, nil, nil, err
	}

	acquisitionDate := importcreate.Date(detail, "acquisition_date", &fieldErrs)
	acquisitionCost := importcreate.Decimal(detail, "acquisition_cost", &fieldErrs)
	appreciationRate := importcreate.Decimal(detail, "annual_appreciation_rate", &fieldErrs)

	// createPropertyReq validates the required + enum fields (display_name,
	// ownership_type, native_currency, property_type, sole_owner required_if).
	// The optional typed cells are parsed above and carried in params directly.
	req := createPropertyReq{
		DisplayName:     strings.TrimSpace(detail["display_name"]),
		Description:     importcreate.OptionalStr(detail["description"]),
		OwnershipType:   strings.TrimSpace(detail["ownership_type"]),
		SoleOwnerUserID: soleOwnerID,
		NativeCurrency:  strings.TrimSpace(detail["native_currency"]),
		PropertyType:    strings.TrimSpace(detail["property_type"]),
		Address:         importcreate.OptionalStr(detail["address"]),
		AcquisitionCost: acquisitionCost,
	}
	fieldErrs = append(fieldErrs, validateStruct(h, &req, emailHandled)...)

	tagID, err := h.repo.LookupTagIDByName(ctx, strings.TrimSpace(detail["tag"]))
	if err != nil {
		return repo.CreatePropertyParams{}, nil, nil, err
	}

	params := repo.CreatePropertyParams{
		DisplayName:            req.DisplayName,
		Description:            req.Description,
		OwnershipType:          req.OwnershipType,
		SoleOwnerUserID:        req.SoleOwnerUserID,
		NativeCurrency:         req.NativeCurrency,
		PropertyType:           req.PropertyType,
		Address:                req.Address,
		AcquisitionDate:        acquisitionDate,
		AcquisitionCost:        acquisitionCost,
		AnnualAppreciationRate: appreciationRate,
	}
	return params, tagID, fieldErrs, nil
}

// ----- vehicle ------------------------------------------------------------

func (h *Handlers) handleImportCreateVehicle(w http.ResponseWriter, r *http.Request) {
	importcreate.Run(w, r, h.validate, h.resolveVehicleDetail,
		func(ctx context.Context, p repo.CreateVehicleParams, tagID *uuid.UUID, rows []repo.ImportSnapshotRow) (uuid.UUID, error) {
			vehicle, err := h.repo.CreateVehicleWithSnapshots(ctx, p, tagID, rows)
			if err != nil {
				return uuid.Nil, err
			}
			return vehicle.Asset.ID, nil
		})
}

func (h *Handlers) resolveVehicleDetail(ctx context.Context, detail map[string]string) (repo.CreateVehicleParams, *uuid.UUID, []snapshotimport.FieldError, error) {
	soleOwnerID, emailHandled, fieldErrs, err := importcreate.ResolveSoleOwner(
		ctx, detail["ownership_type"], detail["sole_owner"], h.repo.LookupUserIDByEmail)
	if err != nil {
		return repo.CreateVehicleParams{}, nil, nil, err
	}

	year := importcreate.Int32(detail, "year", &fieldErrs)
	depreciationRate := importcreate.Decimal(detail, "annual_depreciation_rate", &fieldErrs)

	req := createVehicleReq{
		DisplayName:     strings.TrimSpace(detail["display_name"]),
		Description:     importcreate.OptionalStr(detail["description"]),
		OwnershipType:   strings.TrimSpace(detail["ownership_type"]),
		SoleOwnerUserID: soleOwnerID,
		NativeCurrency:  strings.TrimSpace(detail["native_currency"]),
		VehicleType:     strings.TrimSpace(detail["vehicle_type"]),
		Make:            importcreate.OptionalStr(detail["make"]),
		Model:           importcreate.OptionalStr(detail["model"]),
		Year:            year,
		PlateNumber:     importcreate.OptionalStr(detail["plate_number"]),
	}
	fieldErrs = append(fieldErrs, validateStruct(h, &req, emailHandled)...)

	tagID, err := h.repo.LookupTagIDByName(ctx, strings.TrimSpace(detail["tag"]))
	if err != nil {
		return repo.CreateVehicleParams{}, nil, nil, err
	}

	params := repo.CreateVehicleParams{
		DisplayName:            req.DisplayName,
		Description:            req.Description,
		OwnershipType:          req.OwnershipType,
		SoleOwnerUserID:        req.SoleOwnerUserID,
		NativeCurrency:         req.NativeCurrency,
		VehicleType:            req.VehicleType,
		Make:                   req.Make,
		Model:                  req.Model,
		Year:                   year,
		PlateNumber:            req.PlateNumber,
		AnnualDepreciationRate: depreciationRate,
	}
	return params, tagID, fieldErrs, nil
}

// validateStruct runs the go-playground validator over a create-request struct
// and maps every failure onto a Detail-sheet FieldError. When emailHandled is
// true the sole_owner field was already resolved (and any error attached) by
// ResolveSoleOwner, so the validator's duplicate required_if is dropped.
func validateStruct(h *Handlers, req any, emailHandled bool) []snapshotimport.FieldError {
	verr := h.validate.Struct(req)
	if verr == nil {
		return nil
	}
	var out []snapshotimport.FieldError
	for _, fe := range importcreate.CollectFieldErrors(verr) {
		if emailHandled && fe.Field == "sole_owner" {
			continue
		}
		out = append(out, fe)
	}
	return out
}
