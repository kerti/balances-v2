// Package dateguard holds the server-side "not in the future" checks for
// snapshot/transaction civil dates (bare YYYY-MM-DD and YYYY-MM month buckets).
// A snapshot is by definition a past observation and a transaction records
// something that already happened, so a future date/month is nonsense and is
// rejected 400 (INV-SNAPSHOTS-05).
//
// A civil date carries no timezone. The client seeds it from the household
// member's local calendar day (INV-PRESENTATION-02); in a UTC+ zone that local
// "today" can already be UTC-tomorrow. Comparing such a value against a plain
// UTC "today" rejected valid same-day saves (#426). The guard therefore
// tolerates the maximum forward timezone offset — UTC+14, i.e. at most one
// civil day ahead of UTC — before calling a date "future". The extra slack is
// immaterial for what is a soft sanity check (it keeps year-2099 typos out),
// and it never rejects a genuine same-day save anywhere on Earth.
package dateguard

import "time"

// maxTZSlack is the largest amount a household member's local calendar day can
// lead UTC by. The furthest-forward real timezone is UTC+14 (Line Islands); a
// full civil day covers it with margin and keeps the arithmetic in whole days.
const maxTZSlack = 24 * time.Hour

// IsFuture reports whether t (a civil calendar date parsed as UTC midnight) is
// strictly after "today", where today is UTC now shifted forward by maxTZSlack
// so a UTC+ member's local-today is accepted.
func IsFuture(t, now time.Time) bool {
	n := now.Add(maxTZSlack).UTC()
	today := time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, time.UTC)
	return t.After(today)
}

// IsFutureMonth reports whether ym (first-of-month UTC) is strictly later than
// the current month, derived from UTC now shifted forward by maxTZSlack so a
// month that has already begun on the member's local calendar is accepted.
func IsFutureMonth(ym, now time.Time) bool {
	n := now.Add(maxTZSlack).UTC()
	currentMonth := time.Date(n.Year(), n.Month(), 1, 0, 0, 0, 0, time.UTC)
	return ym.After(currentMonth)
}
