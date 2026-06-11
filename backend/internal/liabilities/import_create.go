package liabilities

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/kerti/balances-v2/backend/internal/importcreate"
	"github.com/kerti/balances-v2/backend/internal/repo"
	"github.com/kerti/balances-v2/backend/internal/snapshotimport"
)

// handleImportCreate creates a brand-new liability from an uploaded position
// workbook: the Detail sheet becomes the liability, the Snapshots sheet seeds
// its history — atomically on commit. The shared transport + preview/commit
// gate + Detail-cell parsing live in internal/importcreate; only the per-group
// field mapping (resolveDetail) and the repo write differ. See issue #88 (bank
// account) / #89 (fan-out).
func (h *Handlers) handleImportCreate(w http.ResponseWriter, r *http.Request) {
	importcreate.Run(w, r, h.validate, h.resolveDetail,
		func(ctx context.Context, p repo.CreateLiabilityParams, tagID *uuid.UUID, rows []repo.ImportSnapshotRow) (uuid.UUID, error) {
			row, err := h.repo.CreateLiabilityWithSnapshots(ctx, p, tagID, rows)
			if err != nil {
				return uuid.Nil, err
			}
			return row.ID, nil
		})
}

// resolveDetail turns the parsed Detail sheet into create params, resolving the
// sole_owner email + tag name conventions and collecting every per-field
// problem. The required + enum fields are validated via createReq; the optional
// typed cells (principal/interest_rate decimals, term_months int, start/
// maturity dates) are parsed directly and carried in params.
func (h *Handlers) resolveDetail(ctx context.Context, detail map[string]string) (repo.CreateLiabilityParams, *uuid.UUID, []snapshotimport.FieldError, error) {
	soleOwnerID, emailHandled, fieldErrs, err := importcreate.ResolveSoleOwner(
		ctx, detail["ownership_type"], detail["sole_owner"], h.repo.LookupUserIDByEmail)
	if err != nil {
		return repo.CreateLiabilityParams{}, nil, nil, err
	}

	principal := importcreate.Decimal(detail, "principal", &fieldErrs)
	interestRate := importcreate.Decimal(detail, "interest_rate", &fieldErrs)
	termMonths := importcreate.Int32(detail, "term_months", &fieldErrs)
	startDate := importcreate.Date(detail, "start_date", &fieldErrs)
	maturityDate := importcreate.Date(detail, "maturity_date", &fieldErrs)

	req := createReq{
		DisplayName:      strings.TrimSpace(detail["display_name"]),
		Description:      importcreate.OptionalStr(detail["description"]),
		Subtype:          strings.TrimSpace(detail["subtype"]),
		OwnershipType:    strings.TrimSpace(detail["ownership_type"]),
		SoleOwnerUserID:  soleOwnerID,
		NativeCurrency:   strings.TrimSpace(detail["native_currency"]),
		CounterpartyName: strings.TrimSpace(detail["counterparty_name"]),
		Principal:        principal,
		InterestRate:     interestRate,
		TermMonths:       termMonths,
	}
	if verr := h.validate.Struct(&req); verr != nil {
		for _, fe := range importcreate.CollectFieldErrors(verr) {
			if emailHandled && fe.Field == "sole_owner" {
				continue
			}
			fieldErrs = append(fieldErrs, fe)
		}
	}

	tagID, err := h.repo.LookupTagIDByName(ctx, strings.TrimSpace(detail["tag"]))
	if err != nil {
		return repo.CreateLiabilityParams{}, nil, nil, err
	}

	params := repo.CreateLiabilityParams{
		DisplayName:      req.DisplayName,
		Description:      req.Description,
		Subtype:          req.Subtype,
		OwnershipType:    req.OwnershipType,
		SoleOwnerUserID:  req.SoleOwnerUserID,
		NativeCurrency:   req.NativeCurrency,
		CounterpartyName: req.CounterpartyName,
		Principal:        principal,
		InterestRate:     interestRate,
		TermMonths:       termMonths,
		StartDate:        startDate,
		MaturityDate:     maturityDate,
	}
	return params, tagID, fieldErrs, nil
}
