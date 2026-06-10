// Package httperr writes typed JSON error responses for the HTTP layer.
// Every 4xx / 5xx response from internal/* handlers ships an Envelope —
// `{"code": "<CODE>", "args": {...}}` — that the frontend looks up in the
// react-i18next `errors:code.<CODE>` catalog. The catalog is the single
// source of human copy; the wire ships no `message` field (see ADR-0027).
//
// The OAuth callback flow (which redirects on failure) and the dev-only
// mock OIDC subcommand are the explicit exceptions and keep their plain
// `http.Error` bodies.
package httperr

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"

	"github.com/kerti/balances-v2/backend/internal/errs"
)

// Envelope is the wire JSON shape for an HTTP error response. Code is the
// wire-stable identifier; Args is an optional flat map of interpolation
// values whose keys match `{{placeholder}}` slots in the catalog string.
// Nested objects are not supported in Args.
type Envelope struct {
	Code Code           `json:"code"`
	Args map[string]any `json:"args,omitempty"`
}

// Write encodes a single Envelope and ends the response with status. The
// Content-Type is set explicitly because callers reach this helper through
// shared handlers and shouldn't have to remember.
func Write(w http.ResponseWriter, status int, code Code, args map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(Envelope{Code: code, Args: args})
}

// WriteRepo maps a repo sentinel to its envelope + status, logging unknown
// errors with op before falling through to INTERNAL. ErrUnauthenticated is
// deliberately unmapped: every route is gated by RequireAuth, so the repo
// seeing no user is a server bug, not a client error (see ADR-0027 and the
// HANDOFF convention).
func WriteRepo(w http.ResponseWriter, op string, err error) {
	switch {
	case errors.Is(err, errs.ErrNotFound):
		Write(w, http.StatusNotFound, CodeNotFound, nil)
	case errors.Is(err, errs.ErrInvalidLifecycle):
		Write(w, http.StatusBadRequest, CodeInvalidLifecycle, nil)
	case errors.Is(err, errs.ErrInvalidSnapshotShape):
		Write(w, http.StatusBadRequest, CodeInvalidSnapshotShape, nil)
	case errors.Is(err, errs.ErrInvalidTransactionType):
		Write(w, http.StatusBadRequest, CodeInvalidTransactionType, nil)
	case errors.Is(err, errs.ErrInvalidTransactionShape):
		Write(w, http.StatusBadRequest, CodeInvalidTransactionShape, nil)
	case errors.Is(err, errs.ErrFxRateExists):
		Write(w, http.StatusConflict, CodeFxRateExists, nil)
	case errors.Is(err, errs.ErrForeignPositionsExist):
		Write(w, http.StatusConflict, CodeForeignPositionsExist, nil)
	case errors.Is(err, errs.ErrPositionNotActive):
		Write(w, http.StatusConflict, CodePositionNotActive, nil)
	case errors.Is(err, errs.ErrTagNameExists):
		Write(w, http.StatusConflict, CodeTagNameExists, nil)
	case errors.Is(err, errs.ErrInvalidRolloverLink):
		Write(w, http.StatusConflict, CodeInvalidRolloverLink, nil)
	case errors.Is(err, errs.ErrSnapshotDateOutsideMonth):
		Write(w, http.StatusBadRequest, CodeSnapshotDateOutsideMonth, nil)
	default:
		slog.Error(op, "err", err)
		Write(w, http.StatusInternalServerError, CodeInternal, nil)
	}
}

// WriteValidation maps a validator.ValidationErrors to a 400 VALIDATION
// envelope using the first field error. Args.field carries the JSON tag
// (because we register a tag-name func on NewValidator) and Args.rule
// carries the validator tag that fired (`required`, `oneof`, ...). Any
// other error type is logged and mapped to INTERNAL — callers should only
// reach for this helper after validator.Struct.
func WriteValidation(w http.ResponseWriter, err error) {
	var verrs validator.ValidationErrors
	if errors.As(err, &verrs) && len(verrs) > 0 {
		first := verrs[0]
		Write(w, http.StatusBadRequest, CodeValidation, map[string]any{
			"field": first.Field(),
			"rule":  first.Tag(),
		})
		return
	}
	slog.Error("validation", "err", err)
	Write(w, http.StatusInternalServerError, CodeInternal, nil)
}

// NewValidator returns a *validator.Validate that reports a struct field by
// its JSON tag (text before the first comma) rather than its Go name, so
// the `field` arg in a VALIDATION envelope reads like the on-wire field
// the client sent — `amount`, not `Amount`. The frontend catalog can then
// rely on stable lowercase snake_case field names. Fields without a json
// tag (and `json:"-"` fields) fall through to the validator's default and
// report by Go name; the handler structs in internal/* tag every wire
// field, so this fall-through is a safety net, not a path we ride.
func NewValidator() *validator.Validate {
	v := validator.New(validator.WithRequiredStructEnabled())
	v.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "-" {
			return ""
		}
		return name
	})
	return v
}
