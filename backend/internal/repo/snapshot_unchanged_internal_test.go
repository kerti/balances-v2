package repo

import (
	"testing"

	"github.com/shopspring/decimal"

	"github.com/kerti/balances-v2/backend/internal/db"
)

func decPtr(s string) *decimal.Decimal { d := dec(s); return &d }

// A quantity × price amount derived by the client carries up to 12 places;
// the column stores 4. Re-saving the same quantity and price must read as
// unchanged against the stored, rounded row, or every unchanged replace
// archives a duplicate (ADR-0058 §1, #788).
// covers: INV-SNAPSHOTS-11
func TestInvestmentSnapshotUnchanged_RoundsToColumnScale(t *testing.T) {
	stored := db.InvestmentSnapshot{
		Amount:       dec("15751.2963"),
		Currency:     "IDR",
		Quantity:     decPtr("1500.12345678"),
		PricePerUnit: decPtr("10.5000"),
	}
	resend := db.CreateInvestmentSnapshotParams{
		Amount:       dec("15751.29629619"),
		Currency:     "IDR",
		Quantity:     decPtr("1500.12345678"),
		PricePerUnit: decPtr("10.5"),
	}
	if !investmentSnapshotUnchanged(stored, resend) {
		t.Fatal("same quantity and price re-sent with an unrounded amount read as a change")
	}

	resend.PricePerUnit = decPtr("10.6")
	resend.Amount = dec("15901.30864187")
	if investmentSnapshotUnchanged(stored, resend) {
		t.Fatal("a changed price read as unchanged")
	}
}
