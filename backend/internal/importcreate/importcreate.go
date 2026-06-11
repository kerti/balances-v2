// Package importcreate holds the create-from-file import flow shared by every
// position group: it parses a position workbook's Detail sheet into a create
// request, validates + resolves the owner/tag conventions, parses the Snapshots
// sheet, and — atomically on commit — creates the position with its seeded
// history. Only the per-group field mapping (the ResolveFunc) and the repo
// write (the CommitFunc) differ between groups; everything else lives here so
// the five groups don't each re-implement the transport, the preview/commit
// gate, and the Detail-cell parsing (issue #88 established this for bank
// accounts; #89 fans it out).
package importcreate

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/kerti/balances-v2/backend/internal/httperr"
	"github.com/kerti/balances-v2/backend/internal/repo"
	"github.com/kerti/balances-v2/backend/internal/snapshotimport"
)

// maxUpload caps the uploaded workbook size — a decade of monthly rows plus a
// Detail sheet is a few KB, so 5 MB is generous headroom against an accident.
const maxUpload = 5 << 20 // 5 MB

// Response is the create-from-file import result. It parallels the snapshot-
// import response but adds the Detail-sheet half: would_create (a position
// would be / was created), the field-level errors, and — on a committed write —
// the new position's id. ToInsert counts the seeded snapshots (a brand-new
// position has no existing months, so there is no to_update counterpart).
type Response struct {
	Mode        string                      `json:"mode"`      // "preview" | "commit"
	Committed   bool                        `json:"committed"` // true only when a position was written
	WouldCreate bool                        `json:"would_create"`
	PositionID  *uuid.UUID                  `json:"position_id,omitempty"`
	ToInsert    int                         `json:"to_insert"`
	FieldErrors []snapshotimport.FieldError `json:"field_errors"`
	Errors      []snapshotimport.RowError   `json:"errors"`
}

// ResolveFunc maps a parsed Detail sheet onto a group's create params,
// resolving the two id-typed conventions (sole_owner email -> user id, tag name
// -> tag id) and collecting every per-field problem. A returned error is a DB
// failure during resolution (mapped to a 5xx); a bad email / missing field is a
// FieldError, not an error.
type ResolveFunc[T any] func(ctx context.Context, detail map[string]string) (params T, tagID *uuid.UUID, fieldErrs []snapshotimport.FieldError, err error)

// CommitFunc creates the position + seeds its snapshots in one transaction and
// returns the new position id. It runs only on mode=commit with zero
// field/row errors.
type CommitFunc[T any] func(ctx context.Context, params T, tagID *uuid.UUID, rows []repo.ImportSnapshotRow) (uuid.UUID, error)

// Run executes the shared create-from-file flow for one position group. The
// uploaded workbook's Detail sheet becomes the position (via resolve), the
// Snapshots sheet seeds its history — atomically. mode=preview (default)
// validates the Detail fields + resolves the owner/tag + validates the snapshot
// rows and writes nothing; mode=commit creates the position + snapshots, but
// only if zero field/row errors (all-or-nothing) — otherwise 422 with the
// errors.
func Run[T any](
	w http.ResponseWriter,
	r *http.Request,
	validate *validator.Validate,
	resolve ResolveFunc[T],
	commit CommitFunc[T],
) {
	mode := r.URL.Query().Get("mode")
	if mode == "" {
		mode = "preview"
	}
	if mode != "preview" && mode != "commit" {
		httperr.Write(w, http.StatusBadRequest, httperr.CodeInvalidImportMode, nil)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxUpload)
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

	params, tagID, fieldErrs, err := resolve(r.Context(), detail)
	if err != nil {
		httperr.WriteRepo(w, "import create: resolve detail", err)
		return
	}

	parsed, rowErrs, err := snapshotimport.Parse(bytes.NewReader(data), snapshotimport.Options{
		DefaultCurrency: strings.TrimSpace(detail["native_currency"]),
		ValidCurrency:   func(c string) bool { return validate.Var(c, "iso4217") == nil },
	})
	if err != nil {
		httperr.Write(w, http.StatusBadRequest, httperr.CodeInvalidSpreadsheet, nil)
		return
	}

	resp := Response{
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

	id, err := commit(r.Context(), params, tagID, rows)
	if err != nil {
		httperr.WriteRepo(w, "import create: commit", err)
		return
	}
	resp.Committed = true
	resp.PositionID = &id
	writeJSON(w, http.StatusOK, resp)
}

// ResolveSoleOwner resolves the sole_owner email convention shared by every
// group. It resolves the owner only when ownership is "sole" and an email was
// given; a sole position with a blank email is left to the struct validator's
// required_if, and a joint position ignores any stray email (export writes it
// blank). emailHandled reports whether this function already owns the
// sole_owner field, so the caller can skip the validator's duplicate
// required_if. An unknown email is a FieldError, not an error; a non-nil error
// is a DB failure.
func ResolveSoleOwner(
	ctx context.Context,
	ownership, email string,
	lookup func(context.Context, string) (uuid.UUID, bool, error),
) (soleOwnerID *uuid.UUID, emailHandled bool, fieldErrs []snapshotimport.FieldError, err error) {
	ownership = strings.TrimSpace(ownership)
	email = strings.TrimSpace(email)
	if ownership != "sole" || email == "" {
		return nil, false, nil, nil
	}
	id, found, err := lookup(ctx, email)
	if err != nil {
		return nil, true, nil, err
	}
	if !found {
		return nil, true, []snapshotimport.FieldError{{
			Field:   "sole_owner",
			Message: "no household member has the email " + email,
		}}, nil
	}
	return &id, true, nil, nil
}

// CollectFieldErrors maps every validator.FieldError onto a Detail-sheet
// FieldError. The struct's sole_owner_user_id field is reported as "sole_owner"
// — the Detail-sheet key the user actually filled (an email), not the resolved
// id field.
func CollectFieldErrors(err error) []snapshotimport.FieldError {
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
		out = append(out, snapshotimport.FieldError{Field: field, Message: RuleMessage(fe)})
	}
	return out
}

// RuleMessage renders a short English reason for a failed validator tag. The
// import flow ships English copy inline (unlike the i18n error envelopes), so
// the message is human-readable.
func RuleMessage(fe validator.FieldError) string {
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

// OptionalStr maps a trimmed Detail cell to an optional string field: nil when
// blank, a pointer otherwise.
func OptionalStr(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}

// Decimal parses an optional decimal Detail cell: blank yields nil; an
// unparseable value appends a FieldError naming the key and yields nil.
func Decimal(detail map[string]string, key string, fieldErrs *[]snapshotimport.FieldError) *decimal.Decimal {
	raw := strings.TrimSpace(detail[key])
	if raw == "" {
		return nil
	}
	d, err := decimal.NewFromString(raw)
	if err != nil {
		*fieldErrs = append(*fieldErrs, snapshotimport.FieldError{Field: key, Message: "must be a number"})
		return nil
	}
	return &d
}

// Date parses an optional YYYY-MM-DD Detail cell: blank yields nil; an
// unparseable value appends a FieldError naming the key and yields nil.
func Date(detail map[string]string, key string, fieldErrs *[]snapshotimport.FieldError) *time.Time {
	raw := strings.TrimSpace(detail[key])
	if raw == "" {
		return nil
	}
	t, err := time.Parse("2006-01-02", raw)
	if err != nil {
		*fieldErrs = append(*fieldErrs, snapshotimport.FieldError{Field: key, Message: "must be a date (YYYY-MM-DD)"})
		return nil
	}
	return &t
}

// Int32 parses an optional integer Detail cell: blank yields nil; a non-integer
// or out-of-range value appends a FieldError naming the key and yields nil.
func Int32(detail map[string]string, key string, fieldErrs *[]snapshotimport.FieldError) *int32 {
	raw := strings.TrimSpace(detail[key])
	if raw == "" {
		return nil
	}
	n, err := strconv.ParseInt(raw, 10, 32)
	if err != nil {
		*fieldErrs = append(*fieldErrs, snapshotimport.FieldError{Field: key, Message: "must be a whole number"})
		return nil
	}
	v := int32(n)
	return &v
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if body != nil {
		_ = json.NewEncoder(w).Encode(body)
	}
}
