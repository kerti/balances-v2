package snapshotamount_test

import (
	"testing"

	"github.com/shopspring/decimal"

	"github.com/kerti/balances-v2/backend/internal/snapshotamount"
)

// Rows are mirrored in frontend/src/lib/decimal.test.ts ("multiplyDecimals"):
// the two implementations must print the same string for every row.
//
// covers: INV-SNAPSHOTS-20
func TestQuantityPrice(t *testing.T) {
	cases := []struct {
		name, qty, price, want string
	}{
		{"whole numbers", "40", "110", "4400"},
		{"fractional quantity", "0.5", "9.99", "4.995"},
		{"float artefact avoided", "0.1", "3", "0.3"},
		{"eight-place quantity, four-place price", "1.12345678", "2.1234", "2.3855"},
		{"fifth decimal 5 rounds up", "0.00005", "1", "0.0001"},
		{"fifth decimal below 5 rounds down", "0.00004", "1", "0"},
		{"just above the half rounds up", "1.00005001", "1", "1.0001"},
		{"just below the half rounds down", "1.00004999", "1", "1"},
		{"twelve-place product rounds once", "0.12345678", "0.1234", "0.0152"},
		{"trailing zeros trimmed", "2.50000000", "4.0000", "10"},
		{"20-digit figures", "12345678901234.5", "1000000", "12345678901234500000"},
		{"product wider than the column", "99999999999999", "99999999999999", "9999999999999800000000000001"},
		{"zero quantity", "0", "123.4567", "0"},
		{"zero price", "7.5", "0", "0"},
		{"negative rounds away from zero", "-0.00005", "1", "-0.0001"},
		{"negative below the half", "-0.00004", "1", "0"},
		{"negative quantity", "-2", "1.5", "-3"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := snapshotamount.QuantityPrice(decimal.RequireFromString(c.qty), decimal.RequireFromString(c.price))
			if got.String() != c.want {
				t.Errorf("QuantityPrice(%s, %s) = %s, want %s", c.qty, c.price, got.String(), c.want)
			}
		})
	}
}
