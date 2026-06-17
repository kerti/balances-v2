package backup

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// covers: INV-BACKUP-06, INV-BACKUP-07, INV-BACKUP-08
func TestWriteRestoreErr(t *testing.T) {
	// Each restore sentinel maps to a fixed status + ADR-0027 envelope code. A
	// 400 means "you sent bad bytes", a 422 means "well-formed but we can't load
	// it", a 403 is the membership guard, and anything unexpected is a 500.
	cases := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{"invalid file", ErrInvalidBackupFile, http.StatusBadRequest, "INVALID_BACKUP_FILE"},
		{"corrupt", ErrCorruptBackup, http.StatusBadRequest, "CORRUPT_BACKUP"},
		{"too new", ErrFormatTooNew, http.StatusUnprocessableEntity, "BACKUP_FORMAT_TOO_NEW"},
		{"not a member", ErrNotMemberOfBackup, http.StatusForbidden, "NOT_A_MEMBER_OF_BACKUP"},
		{"graph invalid", ErrValidationFailed, http.StatusUnprocessableEntity, "BACKUP_VALIDATION_FAILED"},
		{"unexpected -> 500", errors.New("disk on fire"), http.StatusInternalServerError, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			writeRestoreErr(rec, tc.err)
			if rec.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			if tc.wantCode != "" && !strings.Contains(rec.Body.String(), tc.wantCode) {
				t.Errorf("body = %s, want code %s", rec.Body.String(), tc.wantCode)
			}
		})
	}
}

func TestReadBackupUploadRejectsMissingFile(t *testing.T) {
	// A request with no multipart "file" part is a bad upload, not a server
	// error: readBackupUpload must write a 400 and report ok=false so the caller
	// returns without parsing.
	req := httptest.NewRequest(http.MethodPost, "/api/backup/restore/preview", strings.NewReader("not multipart"))
	rec := httptest.NewRecorder()
	if _, ok := readBackupUpload(rec, req); ok {
		t.Fatal("readBackupUpload ok = true, want false for a missing file")
	}
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "INVALID_FILE_UPLOAD") {
		t.Errorf("body = %s, want INVALID_FILE_UPLOAD", rec.Body.String())
	}
}

// covers: INV-BACKUP-06
func TestParseRejectsMalformedJSON(t *testing.T) {
	// A plain (non-gzip) body that gunzips fine but isn't valid JSON is an
	// invalid backup file, not a corrupt gzip stream.
	_, err := Parse(bytes.NewReader([]byte("{not json")))
	if !errors.Is(err, ErrInvalidBackupFile) {
		t.Errorf("err = %v, want ErrInvalidBackupFile", err)
	}
}

func TestHandleExportRequiresUser(t *testing.T) {
	// No authenticated user in context (the middleware is bypassed in this direct
	// call) → 401 before any household is touched. nil pool is safe: the guard
	// returns first.
	h := New(nil, "http://test.local", &stubIssuer{}, &stubNotifier{})
	req := httptest.NewRequest(http.MethodGet, "/api/backup/export", nil).WithContext(context.Background())
	rec := httptest.NewRecorder()
	h.handleExport(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestParseFidelity(t *testing.T) {
	// Unknown/empty defaults to full — the safe, lossless choice (ADR-0036).
	cases := map[string]Fidelity{
		"compacted": FidelityCompacted,
		"full":      FidelityFull,
		"":          FidelityFull,
		"garbage":   FidelityFull,
	}
	for in, want := range cases {
		if got := ParseFidelity(in); got != want {
			t.Errorf("ParseFidelity(%q) = %q, want %q", in, got, want)
		}
	}
}
