package investments

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/kerti/balances-v2/backend/internal/importcreate"
	"github.com/kerti/balances-v2/backend/internal/repo"
	"github.com/kerti/balances-v2/backend/internal/snapshotimport"
)

// Create-from-list import for the five investment subtypes (issue #90) — the
// heaviest slice. The uploaded workbook's Detail sheet becomes a new investment,
// its Snapshots sheet seeds the subtype-shaped history, and its Transactions
// sheet seeds the ledger — atomically on commit (importcreate.RunWithLedger).
// Only the per-subtype Detail mapping (the resolve methods) and the repo write
// differ; the transport, preview/commit gate, snapshot/ledger parsing, and the
// ADR-0023 ledger validation live in internal/importcreate.
//
// Maturity follows decision (b) from #90: a Maturity row is applied last and
// legitimately matures the position (close snapshot included). Only Bond and
// TimeDeposit accept Maturity (the subtype→type matrix), enforced per-row by the
// shared validateLedger.

// ----- stock --------------------------------------------------------------

func (h *Handlers) handleImportCreateStock(w http.ResponseWriter, r *http.Request) {
	importcreate.RunWithLedger(w, r, h.validate, "stock", shapeForSubtype("stock"), h.resolveStockDetail,
		func(ctx context.Context, p repo.CreateStockParams, tagID *uuid.UUID, snaps []repo.ImportInvestmentSnapshotRow, ledger []repo.ImportTransactionRow) (uuid.UUID, error) {
			s, err := h.repo.CreateStockWithSnapshotsAndLedger(ctx, p, tagID, snaps, ledger)
			if err != nil {
				return uuid.Nil, err
			}
			return s.Investment.ID, nil
		})
}

func (h *Handlers) resolveStockDetail(ctx context.Context, detail map[string]string) (repo.CreateStockParams, *uuid.UUID, []snapshotimport.FieldError, error) {
	soleOwnerID, emailHandled, fieldErrs, err := importcreate.ResolveSoleOwner(
		ctx, detail["ownership_type"], detail["sole_owner"], h.repo.LookupUserIDByEmail)
	if err != nil {
		return repo.CreateStockParams{}, nil, nil, err
	}

	req := createStockReq{
		DisplayName:     strings.TrimSpace(detail["display_name"]),
		Description:     importcreate.OptionalStr(detail["description"]),
		OwnershipType:   strings.TrimSpace(detail["ownership_type"]),
		SoleOwnerUserID: soleOwnerID,
		NativeCurrency:  strings.TrimSpace(detail["native_currency"]),
		RiskProfile:     strings.TrimSpace(detail["risk_profile"]),
		Ticker:          strings.TrimSpace(detail["ticker"]),
		Exchange:        strings.TrimSpace(detail["exchange"]),
	}
	fieldErrs = append(fieldErrs, validateImportStruct(h, &req, emailHandled)...)

	tagID, err := h.repo.LookupTagIDByName(ctx, strings.TrimSpace(detail["tag"]))
	if err != nil {
		return repo.CreateStockParams{}, nil, nil, err
	}

	return repo.CreateStockParams{
		DisplayName:     req.DisplayName,
		Description:     req.Description,
		OwnershipType:   req.OwnershipType,
		SoleOwnerUserID: req.SoleOwnerUserID,
		NativeCurrency:  req.NativeCurrency,
		RiskProfile:     req.RiskProfile,
		Ticker:          req.Ticker,
		Exchange:        req.Exchange,
	}, tagID, fieldErrs, nil
}

// ----- mutual fund --------------------------------------------------------

func (h *Handlers) handleImportCreateMutualFund(w http.ResponseWriter, r *http.Request) {
	importcreate.RunWithLedger(w, r, h.validate, "mutual_fund", shapeForSubtype("mutual_fund"), h.resolveMutualFundDetail,
		func(ctx context.Context, p repo.CreateMutualFundParams, tagID *uuid.UUID, snaps []repo.ImportInvestmentSnapshotRow, ledger []repo.ImportTransactionRow) (uuid.UUID, error) {
			mf, err := h.repo.CreateMutualFundWithSnapshotsAndLedger(ctx, p, tagID, snaps, ledger)
			if err != nil {
				return uuid.Nil, err
			}
			return mf.Investment.ID, nil
		})
}

func (h *Handlers) resolveMutualFundDetail(ctx context.Context, detail map[string]string) (repo.CreateMutualFundParams, *uuid.UUID, []snapshotimport.FieldError, error) {
	soleOwnerID, emailHandled, fieldErrs, err := importcreate.ResolveSoleOwner(
		ctx, detail["ownership_type"], detail["sole_owner"], h.repo.LookupUserIDByEmail)
	if err != nil {
		return repo.CreateMutualFundParams{}, nil, nil, err
	}

	req := createMutualFundReq{
		DisplayName:     strings.TrimSpace(detail["display_name"]),
		Description:     importcreate.OptionalStr(detail["description"]),
		OwnershipType:   strings.TrimSpace(detail["ownership_type"]),
		SoleOwnerUserID: soleOwnerID,
		NativeCurrency:  strings.TrimSpace(detail["native_currency"]),
		RiskProfile:     strings.TrimSpace(detail["risk_profile"]),
		FundCode:        strings.TrimSpace(detail["fund_code"]),
		FundManager:     importcreate.OptionalStr(detail["fund_manager"]),
		FundType:        strings.TrimSpace(detail["fund_type"]),
	}
	fieldErrs = append(fieldErrs, validateImportStruct(h, &req, emailHandled)...)

	tagID, err := h.repo.LookupTagIDByName(ctx, strings.TrimSpace(detail["tag"]))
	if err != nil {
		return repo.CreateMutualFundParams{}, nil, nil, err
	}

	return repo.CreateMutualFundParams{
		DisplayName:     req.DisplayName,
		Description:     req.Description,
		OwnershipType:   req.OwnershipType,
		SoleOwnerUserID: req.SoleOwnerUserID,
		NativeCurrency:  req.NativeCurrency,
		RiskProfile:     req.RiskProfile,
		FundCode:        req.FundCode,
		FundManager:     req.FundManager,
		FundType:        req.FundType,
	}, tagID, fieldErrs, nil
}

// ----- gold ---------------------------------------------------------------

func (h *Handlers) handleImportCreateGold(w http.ResponseWriter, r *http.Request) {
	importcreate.RunWithLedger(w, r, h.validate, "gold", shapeForSubtype("gold"), h.resolveGoldDetail,
		func(ctx context.Context, p repo.CreateGoldParams, tagID *uuid.UUID, snaps []repo.ImportInvestmentSnapshotRow, ledger []repo.ImportTransactionRow) (uuid.UUID, error) {
			g, err := h.repo.CreateGoldWithSnapshotsAndLedger(ctx, p, tagID, snaps, ledger)
			if err != nil {
				return uuid.Nil, err
			}
			return g.Investment.ID, nil
		})
}

func (h *Handlers) resolveGoldDetail(ctx context.Context, detail map[string]string) (repo.CreateGoldParams, *uuid.UUID, []snapshotimport.FieldError, error) {
	soleOwnerID, emailHandled, fieldErrs, err := importcreate.ResolveSoleOwner(
		ctx, detail["ownership_type"], detail["sole_owner"], h.repo.LookupUserIDByEmail)
	if err != nil {
		return repo.CreateGoldParams{}, nil, nil, err
	}

	purity := importcreate.Decimal(detail, "purity", &fieldErrs)
	req := createGoldReq{
		DisplayName:     strings.TrimSpace(detail["display_name"]),
		Description:     importcreate.OptionalStr(detail["description"]),
		OwnershipType:   strings.TrimSpace(detail["ownership_type"]),
		SoleOwnerUserID: soleOwnerID,
		NativeCurrency:  strings.TrimSpace(detail["native_currency"]),
		RiskProfile:     strings.TrimSpace(detail["risk_profile"]),
		Form:            strings.TrimSpace(detail["form"]),
		Purity:          purity,
	}
	fieldErrs = append(fieldErrs, validateImportStruct(h, &req, emailHandled)...)

	tagID, err := h.repo.LookupTagIDByName(ctx, strings.TrimSpace(detail["tag"]))
	if err != nil {
		return repo.CreateGoldParams{}, nil, nil, err
	}

	return repo.CreateGoldParams{
		DisplayName:     req.DisplayName,
		Description:     req.Description,
		OwnershipType:   req.OwnershipType,
		SoleOwnerUserID: req.SoleOwnerUserID,
		NativeCurrency:  req.NativeCurrency,
		RiskProfile:     req.RiskProfile,
		Form:            req.Form,
		Purity:          decVal(purity),
	}, tagID, fieldErrs, nil
}

// ----- bond ---------------------------------------------------------------

func (h *Handlers) handleImportCreateBond(w http.ResponseWriter, r *http.Request) {
	importcreate.RunWithLedger(w, r, h.validate, "bond", shapeForSubtype("bond"), h.resolveBondDetail,
		func(ctx context.Context, p repo.CreateBondParams, tagID *uuid.UUID, snaps []repo.ImportInvestmentSnapshotRow, ledger []repo.ImportTransactionRow) (uuid.UUID, error) {
			b, err := h.repo.CreateBondWithSnapshotsAndLedger(ctx, p, tagID, snaps, ledger)
			if err != nil {
				return uuid.Nil, err
			}
			return b.Investment.ID, nil
		})
}

func (h *Handlers) resolveBondDetail(ctx context.Context, detail map[string]string) (repo.CreateBondParams, *uuid.UUID, []snapshotimport.FieldError, error) {
	soleOwnerID, emailHandled, fieldErrs, err := importcreate.ResolveSoleOwner(
		ctx, detail["ownership_type"], detail["sole_owner"], h.repo.LookupUserIDByEmail)
	if err != nil {
		return repo.CreateBondParams{}, nil, nil, err
	}

	couponRate := importcreate.Decimal(detail, "coupon_rate", &fieldErrs)
	maturityDate := importcreate.Date(detail, "maturity_date", &fieldErrs)
	req := createBondReq{
		DisplayName:     strings.TrimSpace(detail["display_name"]),
		Description:     importcreate.OptionalStr(detail["description"]),
		OwnershipType:   strings.TrimSpace(detail["ownership_type"]),
		SoleOwnerUserID: soleOwnerID,
		NativeCurrency:  strings.TrimSpace(detail["native_currency"]),
		RiskProfile:     strings.TrimSpace(detail["risk_profile"]),
		BondType:        strings.TrimSpace(detail["bond_type"]),
		SeriesCode:      importcreate.OptionalStr(detail["series_code"]),
		Issuer:          strings.TrimSpace(detail["issuer"]),
		CouponRate:      couponRate,
		CouponFrequency: strings.TrimSpace(detail["coupon_frequency"]),
		MaturityDate:    strings.TrimSpace(detail["maturity_date"]),
	}
	// The placement Buy lives on the Transactions sheet, so the bond export omits
	// face_value/placement_date from Detail and the seed import never re-derives
	// the buy (CreateBondWithSnapshotsAndLedger does not auto-seed). Drop their
	// required_if errors — they are not import fields.
	fieldErrs = append(fieldErrs, validateImportStruct(h, &req, emailHandled, "face_value", "placement_date")...)

	tagID, err := h.repo.LookupTagIDByName(ctx, strings.TrimSpace(detail["tag"]))
	if err != nil {
		return repo.CreateBondParams{}, nil, nil, err
	}

	return repo.CreateBondParams{
		DisplayName:     req.DisplayName,
		Description:     req.Description,
		OwnershipType:   req.OwnershipType,
		SoleOwnerUserID: req.SoleOwnerUserID,
		NativeCurrency:  req.NativeCurrency,
		RiskProfile:     req.RiskProfile,
		BondType:        req.BondType,
		SeriesCode:      req.SeriesCode,
		Issuer:          req.Issuer,
		CouponRate:      decVal(couponRate),
		CouponFrequency: req.CouponFrequency,
		MaturityDate:    timeVal(maturityDate),
	}, tagID, fieldErrs, nil
}

// ----- time deposit -------------------------------------------------------

func (h *Handlers) handleImportCreateTimeDeposit(w http.ResponseWriter, r *http.Request) {
	importcreate.RunWithLedger(w, r, h.validate, "time_deposit", shapeForSubtype("time_deposit"), h.resolveTimeDepositDetail,
		func(ctx context.Context, p repo.CreateTimeDepositParams, tagID *uuid.UUID, snaps []repo.ImportInvestmentSnapshotRow, ledger []repo.ImportTransactionRow) (uuid.UUID, error) {
			td, err := h.repo.CreateTimeDepositWithSnapshotsAndLedger(ctx, p, tagID, snaps, ledger)
			if err != nil {
				return uuid.Nil, err
			}
			return td.Investment.ID, nil
		})
}

func (h *Handlers) resolveTimeDepositDetail(ctx context.Context, detail map[string]string) (repo.CreateTimeDepositParams, *uuid.UUID, []snapshotimport.FieldError, error) {
	soleOwnerID, emailHandled, fieldErrs, err := importcreate.ResolveSoleOwner(
		ctx, detail["ownership_type"], detail["sole_owner"], h.repo.LookupUserIDByEmail)
	if err != nil {
		return repo.CreateTimeDepositParams{}, nil, nil, err
	}

	principal := importcreate.Decimal(detail, "principal", &fieldErrs)
	interestRate := importcreate.Decimal(detail, "interest_rate", &fieldErrs)
	termMonths := importcreate.Int32(detail, "term_months", &fieldErrs)
	placementDate := importcreate.Date(detail, "placement_date", &fieldErrs)
	maturityDate := importcreate.Date(detail, "maturity_date", &fieldErrs)
	req := createTimeDepositReq{
		DisplayName:     strings.TrimSpace(detail["display_name"]),
		Description:     importcreate.OptionalStr(detail["description"]),
		OwnershipType:   strings.TrimSpace(detail["ownership_type"]),
		SoleOwnerUserID: soleOwnerID,
		NativeCurrency:  strings.TrimSpace(detail["native_currency"]),
		RiskProfile:     strings.TrimSpace(detail["risk_profile"]),
		BankName:        strings.TrimSpace(detail["bank_name"]),
		Principal:       principal,
		InterestRate:    interestRate,
		TermMonths:      int32Val(termMonths),
		PlacementDate:   strings.TrimSpace(detail["placement_date"]),
		MaturityDate:    strings.TrimSpace(detail["maturity_date"]),
		RolloverPolicy:  strings.TrimSpace(detail["rollover_policy"]),
	}
	fieldErrs = append(fieldErrs, validateImportStruct(h, &req, emailHandled)...)

	tagID, err := h.repo.LookupTagIDByName(ctx, strings.TrimSpace(detail["tag"]))
	if err != nil {
		return repo.CreateTimeDepositParams{}, nil, nil, err
	}

	return repo.CreateTimeDepositParams{
		DisplayName:     req.DisplayName,
		Description:     req.Description,
		OwnershipType:   req.OwnershipType,
		SoleOwnerUserID: req.SoleOwnerUserID,
		NativeCurrency:  req.NativeCurrency,
		RiskProfile:     req.RiskProfile,
		BankName:        req.BankName,
		Principal:       decVal(principal),
		InterestRate:    decVal(interestRate),
		TermMonths:      int32Val(termMonths),
		PlacementDate:   timeVal(placementDate),
		MaturityDate:    timeVal(maturityDate),
		RolloverPolicy:  req.RolloverPolicy,
	}, tagID, fieldErrs, nil
}

// ----- helpers ------------------------------------------------------------

// validateImportStruct runs the create-request validator over a resolved Detail
// struct and maps each failure onto a Detail-sheet FieldError. The sole_owner
// field (when already resolved by ResolveSoleOwner) and any keys in drop are
// suppressed — the latter for create-request fields that are not import Detail
// columns (e.g. bond face_value/placement_date, carried by the ledger instead).
func validateImportStruct(h *Handlers, req any, emailHandled bool, drop ...string) []snapshotimport.FieldError {
	verr := h.validate.Struct(req)
	if verr == nil {
		return nil
	}
	dropped := make(map[string]struct{}, len(drop))
	for _, d := range drop {
		dropped[d] = struct{}{}
	}
	var out []snapshotimport.FieldError
	for _, fe := range importcreate.CollectFieldErrors(verr) {
		if emailHandled && fe.Field == "sole_owner" {
			continue
		}
		if _, skip := dropped[fe.Field]; skip {
			continue
		}
		out = append(out, fe)
	}
	return out
}

func decVal(p *decimal.Decimal) decimal.Decimal {
	if p == nil {
		return decimal.Zero
	}
	return *p
}

func timeVal(p *time.Time) time.Time {
	if p == nil {
		return time.Time{}
	}
	return *p
}

func int32Val(p *int32) int32 {
	if p == nil {
		return 0
	}
	return *p
}
