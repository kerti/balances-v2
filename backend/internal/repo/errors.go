// Package repo wraps the generated db.Queries with tenancy-aware methods.
// Every method reads the authenticated user (and therefore household_id)
// from the request context via auth.UserFromContext, then delegates to the
// generated query — which carries household_id in its WHERE clause for
// belt + suspenders enforcement (per ADR-0005 + sqlc query design).
package repo

import "github.com/kerti/balances-v2/backend/internal/errs"

// The sentinel errors are declared in [internal/errs] so the wire-error
// package (internal/httperr) can map them to codes without depending on
// repo (which imports internal/auth and would create an import cycle —
// auth → httperr → repo → auth). The aliases below preserve the legacy
// `repo.ErrFoo` spelling at call sites; new code can also reference
// `errs.ErrFoo` directly. See ADR-0027.
var (
	ErrUnauthenticated          = errs.ErrUnauthenticated
	ErrNotFound                 = errs.ErrNotFound
	ErrInvalidSnapshotShape     = errs.ErrInvalidSnapshotShape
	ErrInvalidTransactionType   = errs.ErrInvalidTransactionType
	ErrInvalidTransactionShape  = errs.ErrInvalidTransactionShape
	ErrInvalidLifecycle         = errs.ErrInvalidLifecycle
	ErrFxRateExists             = errs.ErrFxRateExists
	ErrForeignPositionsExist    = errs.ErrForeignPositionsExist
	ErrPositionNotActive        = errs.ErrPositionNotActive
	ErrTagNameExists            = errs.ErrTagNameExists
	ErrInvalidRolloverLink      = errs.ErrInvalidRolloverLink
	ErrSnapshotDateOutsideMonth = errs.ErrSnapshotDateOutsideMonth
	ErrInvalidDepositTerm       = errs.ErrInvalidDepositTerm
	ErrOutsideDepositTerm       = errs.ErrOutsideDepositTerm
)
