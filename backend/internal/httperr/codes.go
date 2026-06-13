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
)
