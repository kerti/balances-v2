package dateguard_test

import (
	"testing"
	"time"

	"github.com/kerti/balances-v2/backend/internal/dateguard"
)

// utc is a small helper for a UTC calendar date at midnight.
func utc(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// covers: INV-SNAPSHOTS-05
func TestIsFuture(t *testing.T) {
	// now is fixed at 2030-01-15 00:00 UTC. A civil as_of/txn date is a
	// bare calendar day with no timezone; the guard must tolerate the max
	// forward timezone offset (UTC+14 => at most one civil day ahead) so a
	// household member in a UTC+ zone entering their local "today" — which
	// can be UTC-tomorrow — is not bounced with a future-date 400.
	now := time.Date(2030, 1, 15, 0, 0, 0, 0, time.UTC)

	cases := []struct {
		name string
		t    time.Time
		want bool
	}{
		{"past", utc(2030, 1, 10), false},
		{"utc today", utc(2030, 1, 15), false},
		{"one day ahead (UTC+ local today)", utc(2030, 1, 16), false},
		{"two days ahead (genuinely future)", utc(2030, 1, 17), true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := dateguard.IsFuture(c.t, now); got != c.want {
				t.Errorf("IsFuture(%s) = %v, want %v", c.t.Format("2006-01-02"), got, c.want)
			}
		})
	}
}

// covers: INV-SNAPSHOTS-05
func TestIsFutureMonth(t *testing.T) {
	cases := []struct {
		name string
		now  time.Time
		ym   time.Time
		want bool
	}{
		{"current month", utc(2030, 6, 15), utc(2030, 6, 1), false},
		{"past month", utc(2030, 6, 15), utc(2030, 5, 1), false},
		{"next month, mid-month now (nonsense)", utc(2030, 6, 15), utc(2030, 7, 1), true},
		// UTC is last day of June; a UTC+ member's local calendar has already
		// rolled to July, so a July year_month must be accepted.
		{"next month, UTC on last day (tz rollover)", time.Date(2030, 6, 30, 23, 0, 0, 0, time.UTC), utc(2030, 7, 1), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := dateguard.IsFutureMonth(c.ym, c.now); got != c.want {
				t.Errorf("IsFutureMonth(ym=%s, now=%s) = %v, want %v",
					c.ym.Format("2006-01"), c.now.Format("2006-01-02"), got, c.want)
			}
		})
	}
}
