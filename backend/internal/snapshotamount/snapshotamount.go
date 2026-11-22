// Package snapshotamount derives the stored amount of a quantity × price
// snapshot (stock, mutual fund, gold). The server owns this figure on every
// write path (single create and update, bulk entry, workbook import), so a
// client-sent amount can never disagree with the quantity and price beside it.
package snapshotamount

import "github.com/shopspring/decimal"

// scale is the amount column's scale, DECIMAL(20,4) (ADR-0011).
const scale = 4

// QuantityPrice is quantity × price_per_unit, rounded half away from zero to 4
// places once, here. The exact product has up to 12 places (8 + 4); rounding it
// explicitly keeps Postgres from doing it silently on insert, and the frontend's
// lib/decimal multiply mirrors this so the dialog previews what is stored.
func QuantityPrice(quantity, pricePerUnit decimal.Decimal) decimal.Decimal {
	return quantity.Mul(pricePerUnit).Round(scale)
}
