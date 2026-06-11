package assets

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"

	"github.com/kerti/balances-v2/backend/internal/httperr"
	"github.com/kerti/balances-v2/backend/internal/repo"
	"github.com/kerti/balances-v2/backend/internal/snapshotimport"
)

// importCreateResponse is the create-from-file import result. It parallels the
// snapshot-import importResponse but adds the Detail-sheet half: would_create
// (a position would be / was created), the field-level errors, and — on a
// committed write — the new position's id. ToInsert counts the seeded
// snapshots (a brand-new position has no existing months, so there is no
// to_update counterpart).
type importCreateResponse struct {
	Mode        string                      `json:"mode"`      // "preview" | "commit"
	Committed   bool                        `json:"committed"` // true only when a position was written
	WouldCreate bool                        `json:"would_create"`
	PositionID  *uuid.UUID                  `json:"position_id,omitempty"`
	ToInsert    int                         `json:"to_insert"`
	FieldErrors []snapshotimport.FieldError `json:"field_errors"`
	Errors      []snapshotimport.RowError   `json:"errors"`
}

// handleImportCreateBankAccount creates a brand-new bank account from an
// uploaded position workbook: the Detail sheet becomes the position, the
// Snapshots sheet seeds its history — atomically. With mode=preview (default)
// it validates the Detail fields + resolves the sole_owner email / tag name +
// validates the snapshot rows and writes nothing; with mode=commit it creates
// the position + snapshots in one transaction, but only if zero field/row
// errors (all-or-nothing) — otherwise 422 with the errors.
func (h *Handlers) handleImportCreateBankAccount(w http.ResponseWriter, r *http.Request) {
	mode := r.URL.Query().Get("mode")
	if mode == "" {
		mode = "preview"
	}
	if mode != "preview" && mode != "commit" {
		httperr.Write(w, http.StatusBadRequest, httperr.CodeInvalidImportMode, nil)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxImportUpload)
	file, _, err := r.FormFile("file")
	if err != nil {
		httperr.Write(w, http.StatusBadRequest, httperr.CodeInvalidFileUpload, nil)
		return
	}
	defer func() { _ = file.Close() }()

	// Read the upload once: both the Detail and the Snapshots sheet are parsed
	// from the same workbook, each opening its own reader over these bytes.
	data, err := io.ReadAll(file)
	if err != nil {
		httperr.Write(w, http.StatusBadRequest, httperr.CodeInvalidFileUpload, nil)
		return
	}

	detail, err := snapshotimport.ParseDetail(bytes.NewReader(data))
	if err != nil {
		// Unreadable file, or no Detail sheet — there is nothing to create from.
		httperr.Write(w, http.StatusBadRequest, httperr.CodeInvalidSpreadsheet, nil)
		return
	}

	params, tagID, fieldErrs, err := h.resolveBankAccountDetail(r.Context(), detail)
	if err != nil {
		httperr.WriteRepo(w, "import create: resolve detail", err)
		return
	}

	parsed, rowErrs, err := snapshotimport.Parse(bytes.NewReader(data), snapshotimport.Options{
		DefaultCurrency: strings.TrimSpace(detail["native_currency"]),
		ValidCurrency:   func(c string) bool { return h.validate.Var(c, "iso4217") == nil },
	})
	if err != nil {
		httperr.Write(w, http.StatusBadRequest, httperr.CodeInvalidSpreadsheet, nil)
		return
	}

	resp := importCreateResponse{
		Mode:        mode,
		FieldErrors: fieldErrs,
		Errors:      rowErrs,
		ToInsert:    len(parsed),
	}
	if resp.FieldErrors == nil {
		resp.FieldErrors = []snapshotimport.FieldError{}
	}
	if resp.Errors == nil {
		resp.Errors = []snapshotimport.RowError{}
	}
	hasErrors := len(fieldErrs) > 0 || len(rowErrs) > 0
	resp.WouldCreate = !hasErrors

	// commit refuses a workbook with any bad field or row — fix and re-upload.
	if mode == "commit" && hasErrors {
		writeJSON(w, http.StatusUnprocessableEntity, resp)
		return
	}

	if mode == "preview" {
		writeJSON(w, http.StatusOK, resp)
		return
	}

	rows := make([]repo.ImportSnapshotRow, len(parsed))
	for i, p := range parsed {
		rows[i] = repo.ImportSnapshotRow{
			YearMonth:   p.YearMonth,
			Amount:      p.Amount,
			Currency:    p.Currency,
			AsOfDate:    p.AsOfDate,
			Description: p.Description,
		}
	}

	account, err := h.repo.CreateBankAccountWithSnapshots(r.Context(), params, tagID, rows)
	if err != nil {
		httperr.WriteRepo(w, "import create bank account", err)
		return
	}
	resp.Committed = true
	resp.PositionID = &account.Asset.ID
	writeJSON(w, http.StatusOK, resp)
}

// resolveBankAccountDetail turns the parsed Detail sheet into create params,
// resolving the two id-typed conventions (sole_owner email -> user id, tag name
// -> tag id) and collecting every per-field problem. A returned error is a DB
// failure during resolution (mapped to a 5xx); a bad email / missing field is a
// FieldError, not an error. An unmatched tag is left unassigned (tagID nil), per
// the create-import contract.
func (h *Handlers) resolveBankAccountDetail(ctx context.Context, detail map[string]string) (repo.CreateBankAccountParams, *uuid.UUID, []snapshotimport.FieldError, error) {
	var fieldErrs []snapshotimport.FieldError

	ownership := strings.TrimSpace(detail["ownership_type"])
	email := strings.TrimSpace(detail["sole_owner"])

	// Resolve the sole owner only when ownership is "sole" and an email was
	// given. A sole position with a blank email is left to the struct
	// validator's required_if (reported as the "sole_owner" field below); a
	// joint position ignores any stray email (export writes it blank).
	var soleOwnerID *uuid.UUID
	emailHandled := false
	if ownership == "sole" && email != "" {
		id, found, err := h.repo.LookupUserIDByEmail(ctx, email)
		if err != nil {
			return repo.CreateBankAccountParams{}, nil, nil, err
		}
		if !found {
			fieldErrs = append(fieldErrs, snapshotimport.FieldError{
				Field:   "sole_owner",
				Message: "no household member has the email " + email,
			})
		} else {
			soleOwnerID = &id
		}
		emailHandled = true
	}

	req := createBankAccountReq{
		DisplayName:     strings.TrimSpace(detail["display_name"]),
		Description:     optionalStr(detail["description"]),
		OwnershipType:   ownership,
		SoleOwnerUserID: soleOwnerID,
		NativeCurrency:  strings.TrimSpace(detail["native_currency"]),
		BankName:        strings.TrimSpace(detail["bank_name"]),
		AccountNumber:   strings.TrimSpace(detail["account_number"]),
		AccountType:     strings.TrimSpace(detail["account_type"]),
	}

	if verr := h.validate.Struct(&req); verr != nil {
		for _, fe := range collectFieldErrors(verr) {
			// Don't double-report sole_owner: if we already attached a
			// resolution error for it, skip the validator's required_if.
			if emailHandled && fe.Field == "sole_owner" {
				continue
			}
			fieldErrs = append(fieldErrs, fe)
		}
	}

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

// collectFieldErrors maps every validator.FieldError onto a Detail-sheet
// FieldError. The struct's sole_owner_user_id field is reported as "sole_owner"
// — the Detail-sheet key the user actually filled (an email), not the resolved
// id field.
func collectFieldErrors(err error) []snapshotimport.FieldError {
	var verrs validator.ValidationErrors
	if !errors.As(err, &verrs) {
		return nil
	}
	out := make([]snapshotimport.FieldError, 0, len(verrs))
	for _, fe := range verrs {
		field := fe.Field()
		if field == "sole_owner_user_id" {
			field = "sole_owner"
		}
		out = append(out, snapshotimport.FieldError{Field: field, Message: ruleMessage(fe)})
	}
	return out
}

// ruleMessage renders a short English reason for a failed validator tag, mirror
// of the rules on createBankAccountReq. The import flow ships English copy
// inline (unlike the i18n error envelopes), so the message is human-readable.
func ruleMessage(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required", "required_if":
		return "is required"
	case "oneof":
		return "must be one of: " + strings.ReplaceAll(fe.Param(), " ", ", ")
	case "iso4217":
		return "must be a 3-letter ISO currency code"
	default:
		return "is invalid"
	}
}

// optionalStr maps a trimmed Detail cell to an optional string field: nil when
// blank, a pointer otherwise.
func optionalStr(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}
