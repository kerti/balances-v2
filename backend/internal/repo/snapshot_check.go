package repo

import (
	"errors"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
)

// asOfMonthViolation reports whether err is the CHECK constraint added in
// migration 00003 (`<table>_as_of_in_month`) that pins a snapshot's as_of_date
// to its year_month's month. All four position-group snapshot tables carry a
// constraint with that suffix, raised on both INSERT and UPDATE, so matching on
// the suffix covers every snapshot path with one helper. Callers map a true
// result to ErrSnapshotDateOutsideMonth (400).
func asOfMonthViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23514" && // check_violation
		strings.HasSuffix(pgErr.ConstraintName, "_as_of_in_month")
}
