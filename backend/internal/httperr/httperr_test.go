package httperr

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kerti/balances-v2/backend/internal/errs"
)

func TestWrite_envelope(t *testing.T) {
	rec := httptest.NewRecorder()
	Write(rec, http.StatusBadRequest, CodeInvalidID, map[string]any{"field": "snapshot_id"})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: want 400, got %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("content-type: want application/json, got %q", got)
	}
	var env Envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode: %v (body=%q)", err, rec.Body.String())
	}
	if env.Code != CodeInvalidID {
		t.Errorf("code: want %s, got %s", CodeInvalidID, env.Code)
	}
	if env.Args["field"] != "snapshot_id" {
		t.Errorf("args.field: want snapshot_id, got %v", env.Args["field"])
	}
}

func TestWrite_argsOmittedWhenNil(t *testing.T) {
	rec := httptest.NewRecorder()
	Write(rec, http.StatusNotFound, CodeNotFound, nil)

	// args should be absent from the wire JSON, not present-as-null.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, ok := raw["args"]; ok {
		t.Errorf("args should be omitted when nil; got body %q", rec.Body.String())
	}
}

func TestWriteRepo_sentinelMapping(t *testing.T) {
	cases := []struct {
		name   string
		err    error
		status int
		code   Code
	}{
		{"NotFound", errs.ErrNotFound, http.StatusNotFound, CodeNotFound},
		{"InvalidLifecycle", errs.ErrInvalidLifecycle, http.StatusBadRequest, CodeInvalidLifecycle},
		{"InvalidSnapshotShape", errs.ErrInvalidSnapshotShape, http.StatusBadRequest, CodeInvalidSnapshotShape},
		{"InvalidTransactionType", errs.ErrInvalidTransactionType, http.StatusBadRequest, CodeInvalidTransactionType},
		{"InvalidTransactionShape", errs.ErrInvalidTransactionShape, http.StatusBadRequest, CodeInvalidTransactionShape},
		{"FxRateExists", errs.ErrFxRateExists, http.StatusConflict, CodeFxRateExists},
		{"ForeignPositionsExist", errs.ErrForeignPositionsExist, http.StatusConflict, CodeForeignPositionsExist},
		{"PositionNotActive", errs.ErrPositionNotActive, http.StatusConflict, CodePositionNotActive},
		{"TagNameExists", errs.ErrTagNameExists, http.StatusConflict, CodeTagNameExists},
		{"InvalidRolloverLink", errs.ErrInvalidRolloverLink, http.StatusConflict, CodeInvalidRolloverLink},
		{"SnapshotDateOutsideMonth", errs.ErrSnapshotDateOutsideMonth, http.StatusBadRequest, CodeSnapshotDateOutsideMonth},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			WriteRepo(rec, "op", tc.err)
			if rec.Code != tc.status {
				t.Fatalf("status: want %d, got %d", tc.status, rec.Code)
			}
			var env Envelope
			if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if env.Code != tc.code {
				t.Errorf("code: want %s, got %s", tc.code, env.Code)
			}
		})
	}
}

func TestWriteRepo_wrappedSentinel(t *testing.T) {
	// errors.Is must traverse wrapping — the repo layer often returns
	// fmt.Errorf("...: %w", errs.ErrNotFound) and the mapping has to keep
	// working through the wrap.
	wrapped := fmt.Errorf("get receivable %s: %w", "id-here", errs.ErrNotFound)

	rec := httptest.NewRecorder()
	WriteRepo(rec, "get receivable", wrapped)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status: want 404, got %d", rec.Code)
	}
	var env Envelope
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	if env.Code != CodeNotFound {
		t.Errorf("code: want %s, got %s", CodeNotFound, env.Code)
	}
}

func TestWriteRepo_unknownErrorMapsToInternal(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteRepo(rec, "op", errors.New("boom"))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status: want 500, got %d", rec.Code)
	}
	var env Envelope
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	if env.Code != CodeInternal {
		t.Errorf("code: want %s, got %s", CodeInternal, env.Code)
	}
	// Args must be absent — we deliberately don't leak the underlying
	// error message to the wire.
	if env.Args != nil {
		t.Errorf("args should be nil for INTERNAL; got %v", env.Args)
	}
}

func TestWriteRepo_unauthenticatedFallsThrough(t *testing.T) {
	// ErrUnauthenticated is deliberately not mapped — RequireAuth gates
	// every HTTP route, so a repo seeing no user is a server bug. It
	// must surface as 500 INTERNAL, not 401, so we don't paper over the
	// misconfiguration.
	rec := httptest.NewRecorder()
	WriteRepo(rec, "op", errs.ErrUnauthenticated)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status: want 500, got %d", rec.Code)
	}
	var env Envelope
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	if env.Code != CodeInternal {
		t.Errorf("code: want %s, got %s", CodeInternal, env.Code)
	}
}

type sample struct {
	Amount   string `json:"amount"        validate:"required"`
	Kind     string `json:"kind"          validate:"required,oneof=a b c"`
	Ignored  string `json:"-"             validate:"required"`
	Untagged string `validate:"required"`
}

func TestWriteValidation_envelopeFromFirstFieldError(t *testing.T) {
	v := NewValidator()
	err := v.Struct(sample{Kind: "a", Ignored: "x", Untagged: "x"}) // Amount empty -> required fires
	if err == nil {
		t.Fatal("expected validation error")
	}

	rec := httptest.NewRecorder()
	WriteValidation(rec, err)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: want 400, got %d", rec.Code)
	}
	var env Envelope
	if jerr := json.Unmarshal(rec.Body.Bytes(), &env); jerr != nil {
		t.Fatalf("decode: %v", jerr)
	}
	if env.Code != CodeValidation {
		t.Errorf("code: want %s, got %s", CodeValidation, env.Code)
	}
	// The JSON-tag-aware validator must report "amount", not "Amount".
	if env.Args["field"] != "amount" {
		t.Errorf("args.field: want amount, got %v", env.Args["field"])
	}
	if env.Args["rule"] != "required" {
		t.Errorf("args.rule: want required, got %v", env.Args["rule"])
	}
}

func TestWriteValidation_oneofTagSurfaces(t *testing.T) {
	v := NewValidator()
	err := v.Struct(sample{Amount: "1", Kind: "nope", Ignored: "x", Untagged: "x"})

	rec := httptest.NewRecorder()
	WriteValidation(rec, err)

	var env Envelope
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	if env.Args["field"] != "kind" {
		t.Errorf("args.field: want kind, got %v", env.Args["field"])
	}
	if env.Args["rule"] != "oneof" {
		t.Errorf("args.rule: want oneof, got %v", env.Args["rule"])
	}
}

func TestWriteValidation_nonValidatorErrorMapsToInternal(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteValidation(rec, errors.New("not a validator error"))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status: want 500, got %d", rec.Code)
	}
	var env Envelope
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	if env.Code != CodeInternal {
		t.Errorf("code: want %s, got %s", CodeInternal, env.Code)
	}
}

func TestNewValidator_untaggedFieldFallsBackToGoName(t *testing.T) {
	// Fields without a json tag fall back to the validator's default —
	// the Go field name. The internal/* request structs tag every wire
	// field, so this path is a safety net, not a production code path;
	// this test locks the behaviour so a contributor doesn't add an
	// untagged field expecting empty-string and silently leak the Go
	// name to the FE catalog lookup.
	v := NewValidator()
	err := v.Struct(sample{Amount: "1", Kind: "a", Ignored: "x"}) // Untagged empty
	if err == nil {
		t.Fatal("expected validation error")
	}

	rec := httptest.NewRecorder()
	WriteValidation(rec, err)

	var env Envelope
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	if env.Args["field"] != "Untagged" {
		t.Errorf("args.field: want Untagged, got %v", env.Args["field"])
	}
}
