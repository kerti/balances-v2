package backup

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/kerti/balances-v2/backend/internal/auth"
	"github.com/kerti/balances-v2/backend/internal/httperr"
)

// maxRestoreUpload caps the uploaded backup size. Backups are gzipped JSON and
// realistic Households are KB-to-MB scale (ADR-0036), so 50 MB is a generous
// ceiling that still bounds a malicious upload.
const maxRestoreUpload = 50 << 20 // 50 MB

// handleRestorePreview validates an uploaded backup without touching the
// database and returns the stakes Summary (household name, fidelity, per-section
// counts) that drives the confirmation screen (ADR-0036). It runs the full
// parse + integrity + graph + membership checks, so a file that previews cleanly
// is one that commit will accept.
func (h *Handlers) handleRestorePreview(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		httperr.Write(w, http.StatusUnauthorized, httperr.CodeUnauthorized, nil)
		return
	}

	raw, ok := readBackupUpload(w, r)
	if !ok {
		return
	}

	env, err := Parse(bytes.NewReader(raw))
	if err != nil {
		writeRestoreErr(w, err)
		return
	}
	summary, err := Validate(env, user.GoogleSub)
	if err != nil {
		writeRestoreErr(w, err)
		return
	}

	// The stakes the UI scales its confirmation to: what the caller's current
	// Household will lose. An all-zero map means an empty Household (a fresh
	// self-host import), which the UI confirms with a checkbox rather than the
	// type-to-erase ceremony.
	current, err := h.gather(r.Context(), user.HouseholdID, FidelityFull)
	if err != nil {
		httperr.WriteRepo(w, "backup restore preview", err)
		return
	}
	writeJSON(w, http.StatusOK, RestorePreview{Backup: summary, Current: current.SectionCounts()})
}

// RestorePreview is the non-destructive preview returned before a commit: what
// the backup will load (Backup) and what the caller's current Household will
// lose (Current). The UI scales its confirmation to the stakes — a checkbox when
// Current is empty, a type-to-erase ceremony otherwise (ADR-0036).
type RestorePreview struct {
	Backup  *Summary       `json:"backup"`
	Current map[string]int `json:"current"`
}

// handleRestoreCommit performs the destructive restore. It re-parses and
// re-validates the re-uploaded file from scratch — preview is never trusted, and
// the membership guard re-runs here so a member can only ever overwrite their
// own Household — then wipes and loads in one transaction (ADR-0036). The wipe
// deletes every session, so afterwards it re-issues one for the caller (re-linked
// by google_sub) to keep them signed in across the restore (ADR-0017).
func (h *Handlers) handleRestoreCommit(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		httperr.Write(w, http.StatusUnauthorized, httperr.CodeUnauthorized, nil)
		return
	}

	raw, ok := readBackupUpload(w, r)
	if !ok {
		return
	}

	env, err := Parse(bytes.NewReader(raw))
	if err != nil {
		writeRestoreErr(w, err)
		return
	}
	summary, err := Validate(env, user.GoogleSub)
	if err != nil {
		writeRestoreErr(w, err)
		return
	}

	if err := Commit(r.Context(), h.pool, env, user.HouseholdID); err != nil {
		// Validate already passed, so a failure here is infrastructural (DB error
		// or an FK the graph check doesn't police). The transaction rolled back, so
		// the caller's data is intact — report a generic 500.
		httperr.WriteRepo(w, "backup restore commit", err)
		return
	}

	// The wipe deleted every session, so re-issue one for the restored caller —
	// re-linked by google_sub (already proven a member) — to spare a non-technical
	// user a sudden, alarming logout. Best-effort: the destructive work already
	// committed, so on any failure here we just let them sign in again rather than
	// fail a successful restore. Must precede writeJSON (it sets a cookie header).
	if restored, err := h.q.GetUserByGoogleSub(r.Context(), user.GoogleSub); err != nil {
		slog.Warn("restore: lookup restored caller for re-login", "err", err)
	} else if err := h.sessions.IssueSession(r.Context(), w, restored.ID, r.UserAgent()); err != nil {
		slog.Warn("restore: re-issue caller session", "err", err)
	}

	writeJSON(w, http.StatusOK, restoreResult{Restored: true, Summary: summary})
}

// restoreResult is the commit response. Restored is always true on a 2xx (a
// failure takes the error path); the Summary echoes what was loaded so the UI
// can show "restored N positions" before reloading into the restored data.
type restoreResult struct {
	Restored bool     `json:"restored"`
	Summary  *Summary `json:"summary"`
}

// readBackupUpload reads the size-capped multipart "file" field shared by both
// restore steps. On any failure it writes the error response and returns
// ok=false so the caller returns immediately.
func readBackupUpload(w http.ResponseWriter, r *http.Request) (data []byte, ok bool) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRestoreUpload)
	file, _, err := r.FormFile("file")
	if err != nil {
		httperr.Write(w, http.StatusBadRequest, httperr.CodeInvalidFileUpload, nil)
		return nil, false
	}
	defer func() { _ = file.Close() }()

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(file); err != nil {
		httperr.Write(w, http.StatusBadRequest, httperr.CodeInvalidFileUpload, nil)
		return nil, false
	}
	return buf.Bytes(), true
}

// writeJSON encodes v as a JSON body with the given status. Used for the restore
// success responses (the error path goes through httperr).
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeRestoreErr maps a restore sentinel (restore.go) to its ADR-0027 envelope
// + status. A malformed/corrupt file is a 400 (the client sent bad bytes); a
// well-formed file we can't process — too-new format, inconsistent graph — is a
// 422; a non-member is a 403. Anything unexpected falls through to 500.
func writeRestoreErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrInvalidBackupFile):
		httperr.Write(w, http.StatusBadRequest, httperr.CodeInvalidBackupFile, nil)
	case errors.Is(err, ErrCorruptBackup):
		httperr.Write(w, http.StatusBadRequest, httperr.CodeCorruptBackup, nil)
	case errors.Is(err, ErrFormatTooNew):
		httperr.Write(w, http.StatusUnprocessableEntity, httperr.CodeBackupFormatTooNew, nil)
	case errors.Is(err, ErrNotMemberOfBackup):
		httperr.Write(w, http.StatusForbidden, httperr.CodeNotMemberOfBackup, nil)
	case errors.Is(err, ErrValidationFailed):
		httperr.Write(w, http.StatusUnprocessableEntity, httperr.CodeBackupValidationFailed, nil)
	default:
		httperr.WriteRepo(w, "backup restore", err)
	}
}
