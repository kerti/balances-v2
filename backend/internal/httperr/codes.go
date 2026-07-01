package httperr

// Code is the wire-stable identifier for an HTTP error. The frontend looks it
// up in the react-i18next `errors:code.<CODE>` catalog and interpolates the
// envelope's Args; the catalog is the single source of human copy, so we ship
// no message field on the envelope. See ADR-0027 for the full rationale and
// the recipe for adding new codes.
type Code string

const (
	// CodeInternal is the catch-all for 500s — both the "we don't know what
	// went wrong" case and the deliberate fall-through for unexpected repo
	// sentinels (e.g. ErrUnauthenticated, which is unreachable through HTTP
	// because RequireAuth gates every route).
	CodeInternal Code = "INTERNAL"

	// CodeNotFound maps repo.ErrNotFound -> 404. The frontend renders a
	// generic "Not found." line; the URL and op context stay server-side.
	CodeNotFound Code = "NOT_FOUND"

	// CodeValidation is the generic struct-validation 400. Args carry the
	// failing field name (json tag) + the validator rule that fired
	// (`required`, `oneof`, `min`, ...). One catalog template covers every
	// rule via i18next interpolation. See WriteValidation.
	CodeValidation Code = "VALIDATION"

	// CodeInvalidID is a 400 for an unparseable UUID path param. The
	// param name (e.g. "id" vs "snapshot_id") rides in Args.
	CodeInvalidID Code = "INVALID_ID"

	// CodeInvalidJSONBody is a 400 for a request body that fails JSON
	// decode before validator ever runs. No args — the underlying decode
	// error is not leaked.
	CodeInvalidJSONBody Code = "INVALID_JSON_BODY"

	// CodeInvalidYearMonth is a 400 for a year_month query/body field that
	// doesn't parse as YYYY-MM or YYYY-MM-DD.
	CodeInvalidYearMonth Code = "INVALID_YEAR_MONTH"

	// CodeInvalidDate is a 400 for a date field that doesn't parse as
	// YYYY-MM-DD. Args.field carries the field name.
	CodeInvalidDate Code = "INVALID_DATE"

	// CodeFutureYearMonth is a 400 for a year_month value that resolves to
	// a month strictly later than the current UTC month.
	CodeFutureYearMonth Code = "FUTURE_YEAR_MONTH"

	// CodeSnapshotFutureDate is a 400 for a snapshot as_of_date strictly
	// later than today UTC.
	CodeSnapshotFutureDate Code = "SNAPSHOT_FUTURE_DATE"

	// CodeSnapshotDateOutsideMonth maps repo.ErrSnapshotDateOutsideMonth -> 400
	// — a snapshot's as_of_date falls outside its year_month's calendar month.
	CodeSnapshotDateOutsideMonth Code = "SNAPSHOT_DATE_OUTSIDE_MONTH"

	// CodeTransactionFutureDate is a 400 for an investment-transaction
	// transaction_date strictly later than today UTC.
	CodeTransactionFutureDate Code = "TRANSACTION_FUTURE_DATE"

	// CodeInvalidSnapshotShape maps repo.ErrInvalidSnapshotShape -> 400 —
	// the value-column combo doesn't match the parent investment subtype.
	CodeInvalidSnapshotShape Code = "INVALID_SNAPSHOT_SHAPE"

	// CodeInvalidTransactionType maps repo.ErrInvalidTransactionType -> 400 —
	// the transaction_type isn't valid for the parent investment subtype
	// (e.g. Coupon on a Stock).
	CodeInvalidTransactionType Code = "INVALID_TRANSACTION_TYPE"

	// CodeInvalidTransactionShape maps repo.ErrInvalidTransactionShape -> 400 —
	// the value-column combo doesn't match the declared transaction_type
	// (e.g. Buy without quantity).
	CodeInvalidTransactionShape Code = "INVALID_TRANSACTION_SHAPE"

	// CodeInvalidLifecycle maps repo.ErrInvalidLifecycle -> 400 — the
	// requested status / terminated_at combo violates ADR-0009.
	CodeInvalidLifecycle Code = "INVALID_LIFECYCLE"

	// CodeFxRateExists maps repo.ErrFxRateExists -> 409 — uniqueness collision
	// on (household, year_month, currency).
	CodeFxRateExists Code = "FX_RATE_EXISTS"

	// CodeForeignPositionsExist maps repo.ErrForeignPositionsExist -> 409 —
	// turning multi-currency off while non-reporting-currency positions
	// remain.
	CodeForeignPositionsExist Code = "FOREIGN_POSITIONS_EXIST"

	// CodePositionNotActive maps repo.ErrPositionNotActive -> 409 — a
	// transaction against an Investment whose status is no longer active
	// (e.g. after a Maturity row flipped it).
	CodePositionNotActive Code = "POSITION_NOT_ACTIVE"

	// CodeInvalidRate is a 400 for an FX-rate input <= 0.
	CodeInvalidRate Code = "INVALID_RATE"

	// CodeInvalidImportMode is a 400 for a snapshot-import mode query param
	// that isn't "preview" or "commit".
	CodeInvalidImportMode Code = "INVALID_IMPORT_MODE"

	// CodeInvalidFileUpload is a 400 for a missing or oversized multipart
	// file upload (the "file" form field).
	CodeInvalidFileUpload Code = "INVALID_FILE_UPLOAD"

	// CodeInvalidSpreadsheet is a 400 for a spreadsheet that the importer
	// can't read at all (corrupt, wrong format). Per-row validation errors
	// stay on the 422 ImportResult body — see ADR-0027 "Out of scope".
	CodeInvalidSpreadsheet Code = "INVALID_SPREADSHEET"

	// CodeCannotInviteSelf is a 400 for the auth-invitations self-invite
	// rejection.
	CodeCannotInviteSelf Code = "CANNOT_INVITE_SELF"

	// CodeInvitationNoLongerValid is a 409 at the onboarding gate when the
	// chosen invitation is no longer pending for the verified email — used or
	// expired between the gate's read and the commit (ADR-0038 TOCTOU
	// re-validation). The SPA refreshes the gate rather than treating it as a
	// hard error.
	CodeInvitationNoLongerValid Code = "INVITATION_NO_LONGER_VALID"

	// CodeTagNameExists maps repo.ErrTagNameExists -> 409 — a Tag with that
	// name (case-insensitive) already exists in the household.
	CodeTagNameExists Code = "TAG_NAME_EXISTS"

	// CodeInvalidRolloverLink maps repo.ErrInvalidRolloverLink -> 409 — a manual
	// rollover link (issue #65) that would form an illegal chain (self-link,
	// already-linked successor, source with an existing successor, or a cycle).
	CodeInvalidRolloverLink Code = "INVALID_ROLLOVER_LINK"

	// CodeInvalidDepositTerm maps repo.ErrInvalidDepositTerm -> 400 — a time
	// deposit whose maturity_date is not strictly after its placement_date
	// (issue #62).
	CodeInvalidDepositTerm Code = "INVALID_DEPOSIT_TERM"

	// CodeOutsideDepositTerm maps repo.ErrOutsideDepositTerm -> 400 — a time
	// deposit snapshot or transaction (including the Maturity event), or a term
	// edit, that falls outside the deposit's [placement_date, maturity_date]
	// window (issue #62).
	CodeOutsideDepositTerm Code = "OUTSIDE_DEPOSIT_TERM"

	// CodeUnauthorized is the 401 emitted by the session middleware when the
	// request has no valid session cookie. Distinct from the repo's
	// unreachable ErrUnauthenticated — this one is the real, client-facing
	// gate before the repo ever runs.
	CodeUnauthorized Code = "UNAUTHORIZED"

	// CodeInvalidBackupFile is a 400 for an upload that is not a recognizable
	// whole-Household backup — unparseable JSON, or a format_version below 1
	// (backup.ErrInvalidBackupFile, ADR-0036).
	CodeInvalidBackupFile Code = "INVALID_BACKUP_FILE"

	// CodeCorruptBackup is a 400 for a backup whose gzip stream is damaged or
	// truncated (CRC), or whose declared per-section count doesn't match the
	// payload (backup.ErrCorruptBackup, ADR-0036).
	CodeCorruptBackup Code = "CORRUPT_BACKUP"

	// CodeBackupFormatTooNew is a 422 for a backup whose format_version is newer
	// than this build speaks — it is refused rather than guessed
	// (backup.ErrFormatTooNew, ADR-0036).
	CodeBackupFormatTooNew Code = "BACKUP_FORMAT_TOO_NEW"

	// CodeNotMemberOfBackup is a 403 when the caller is not a member of the
	// backup's Household — you may only restore a Household you belong to
	// (backup.ErrNotMemberOfBackup, ADR-0017/ADR-0036).
	CodeNotMemberOfBackup Code = "NOT_A_MEMBER_OF_BACKUP"

	// CodeBackupValidationFailed is a 422 for a backup whose object graph is
	// internally inconsistent (a dangling foreign key, a row in the wrong
	// Household) — backup.ErrValidationFailed (ADR-0036).
	CodeBackupValidationFailed Code = "BACKUP_VALIDATION_FAILED"

	// CodeEmailTaken is a 409 when a local registration uses an email already
	// belonging to a live user (ADR-0039). Registration is the founder/self-serve
	// path, so revealing "this email is in use" here is acceptable (unlike login,
	// which must never enumerate).
	CodeEmailTaken Code = "EMAIL_TAKEN"

	// CodeWeakPassword is a 400 when a local password fails the policy floor
	// (ADR-0039): below the minimum length, or among commonly-breached passwords.
	// Args carry a `reason` (`min` | `breached`) for a specific FE message.
	CodeWeakPassword Code = "WEAK_PASSWORD"

	// CodeInvalidCredentials is the single, deliberately-generic 401 for every
	// local login failure — unknown email, no credential (dormant/Google-only
	// user), or wrong password all return it identically, so login never reveals
	// whether an email exists (no user enumeration, ADR-0039).
	CodeInvalidCredentials Code = "INVALID_CREDENTIALS"

	// CodeTooManyAttempts is a 429 when login is in rate-limit backoff (ADR-0039).
	// Soft, never a hard lock: a Retry-After header tells the client when to retry.
	CodeTooManyAttempts Code = "TOO_MANY_ATTEMPTS"

	// CodeResetLinkNoLongerValid is a 409 when an emailed password-reset link is
	// unknown, already used, or expired — one generic answer for every invalid
	// state so the reset-set screen shows a single "this link is no longer valid"
	// message (ADR-0039, #282). Mirrors CodeInvitationNoLongerValid.
	CodeResetLinkNoLongerValid Code = "RESET_LINK_NO_LONGER_VALID"
)
