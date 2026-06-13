// Package errs holds the cross-package sentinel error variables. It is a leaf
// dependency: no other internal/* package depends on anything from here, so
// any package can import it without creating an import cycle. The position-
// group repos in internal/repo re-export these for compatibility (so call
// sites continue to write `repo.ErrNotFound`), and internal/httperr imports
// them directly to map sentinels to wire codes per ADR-0027.
//
// New sentinels go here. Add a re-export line in internal/repo/errors.go if
// you want the legacy `repo.ErrFoo` spelling to keep working at the call
// sites; otherwise consumers can also import this package directly.
package errs

import "errors"

var (
	// ErrUnauthenticated is returned when a repository method runs without a
	// user attached to the request context. Handlers are gated by RequireAuth,
	// so this is unreachable through the HTTP path — HTTP packages intentionally
	// don't map it and let it fall through to 500 (a misconfigured route is a
	// server bug, not a client error). Kept here for defense in depth in case
	// the repo is ever called from a non-HTTP entrypoint.
	ErrUnauthenticated = errors.New("repo: no user in request context")

	// ErrNotFound is returned when a query that expected a single row found
	// none — either the row genuinely doesn't exist or it belongs to a
	// different Household (the SQL filter makes the two cases indistinguishable
	// from the caller's perspective, which is intentional).
	ErrNotFound = errors.New("repo: not found")

	// ErrInvalidSnapshotShape is returned when an Investment snapshot mutation
	// supplies a value-column combination that violates the subtype's expected
	// shape (per ADR-0022). Stock/MutualFund/Gold require quantity+price_per_unit
	// and reject accrued_interest; Bond/TimeDeposit require accrued_interest
	// and reject quantity+price_per_unit. The DB's CHECK constraint catches
	// rows that satisfy no shape; this error catches rows that pick the wrong
	// shape for their parent's subtype (which the DB can't see).
	ErrInvalidSnapshotShape = errors.New("repo: invalid investment snapshot shape for subtype")

	// ErrInvalidTransactionType is returned when an Investment transaction
	// mutation asks for a transaction_type the parent's subtype doesn't
	// support (e.g., Coupon on a Stock, Buy on a TimeDeposit). The DB-level
	// CHECK enforces shape-vs-type consistency; this error enforces the
	// subtype-vs-type compatibility matrix (which the DB can't see).
	ErrInvalidTransactionType = errors.New("repo: invalid transaction type for subtype")

	// ErrInvalidTransactionShape is returned when an Investment transaction
	// mutation supplies a value-column combination that doesn't match its
	// declared transaction_type (e.g., Buy without quantity, Maturity
	// without principal_disposition). Caught at the repo layer with a
	// human-readable message; the DB CHECK would also reject these but
	// later in the call.
	ErrInvalidTransactionShape = errors.New("repo: invalid transaction shape for type")

	// ErrInvalidLifecycle is returned when a Position lifecycle mutation
	// supplies a status the group doesn't define, or violates the
	// status/terminated_at biconditional (per ADR-0009: a non-active Position
	// must carry a terminated_at, an active one must not). The DB CHECK
	// (migration 00012) backs the date half; this error catches both halves
	// at the repo layer with a human-readable message and a clean 400.
	ErrInvalidLifecycle = errors.New("repo: invalid position lifecycle")

	// ErrFxRateExists is returned when creating an FX rate that collides with an
	// existing (household, year_month, currency) row — the identity is unique,
	// so the caller should edit the existing rate instead. Mapped to 409.
	ErrFxRateExists = errors.New("repo: fx rate already exists for that month and currency")

	// ErrForeignPositionsExist is returned when turning multi-currency off while
	// positions denominated in a non-reporting currency still exist (their
	// values would silently be treated as reporting currency). Mapped to 409.
	ErrForeignPositionsExist = errors.New("repo: foreign-currency positions exist")

	// ErrPositionNotActive is returned when a transaction is created against an
	// Investment whose status is no longer 'active' — most notably after a
	// Maturity transaction flips it to 'matured' (ADR-0009: Maturity is
	// terminal). Mapped to 409 Conflict: the request is well-formed but the
	// position's state forbids it.
	ErrPositionNotActive = errors.New("repo: position is not active")

	// ErrTagNameExists is returned when creating or renaming a Tag to a name
	// that already exists (case-insensitive) among the household's living
	// Tags — the (household, lower(name)) partial unique index. Mapped to 409.
	ErrTagNameExists = errors.New("repo: tag name already exists")

	// ErrInvalidRolloverLink is returned when manually linking a matured deposit
	// to its rollover successor (issue #65) would form an illegal chain: the two
	// are the same position, the successor is already rolled over from somewhere,
	// the source already has a successor, or the link would close a direct cycle.
	// Mapped to 409 Conflict — the request is well-formed but the positions'
	// current rollover state forbids it.
	ErrInvalidRolloverLink = errors.New("repo: invalid rollover link")
)
