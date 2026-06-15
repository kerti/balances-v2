package repo

import (
	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/kerti/balances-v2/backend/internal/db"
)

// costBasisFromLedger replays an investment's transaction ledger to the
// current "money still in" cost basis. It mirrors the frontend
// lib/costBasis.ts convention exactly so list-screen and detail-screen
// figures agree:
//
//   - buy:  cost += amount, qty += quantity
//   - sell: proportional avg-cost reduction — cost -= cost*sellQty/qty,
//     qty -= sellQty (sellQty clamped to qty held)
//   - fee:  cost += amount (capitalised into the all-in money put in)
//   - coupon / dividend / distribution: income, not a cost adjustment — ignored
//   - maturity: terminal — ignored (position lifecycle handles "this is over")
//
// txns MUST be ordered by transaction_date ascending; the batch query
// ListInvestmentTransactionsByInvestmentIDs already orders that way.
// Transactions with the null shape fields for their type are skipped
// defensively (the DB CHECK in migration 00010 enforces the shape).
func costBasisFromLedger(txns []db.InvestmentTransaction) decimal.Decimal {
	cost := decimal.Zero
	qty := decimal.Zero
	for _, tx := range txns {
		applyLedgerTxn(&cost, &qty, tx)
	}
	return cost
}

// applyLedgerTxn advances the running (cost, qty) by one transaction per the
// avg-cost rules above. Shared by costBasisFromLedger (terminal figure) and
// costSeriesAtMonths (per-month series) so the two never drift.
func applyLedgerTxn(cost, qty *decimal.Decimal, tx db.InvestmentTransaction) {
	switch tx.TransactionType {
	case "buy":
		if tx.Amount != nil && tx.Quantity != nil {
			*cost = cost.Add(*tx.Amount)
			*qty = qty.Add(*tx.Quantity)
		}
	case "sell":
		if tx.Quantity == nil || !qty.IsPositive() {
			return
		}
		sellQty := *tx.Quantity
		if sellQty.GreaterThan(*qty) {
			sellQty = *qty
		}
		// reduce cost proportionally: cost*sellQty/qty
		*cost = cost.Sub(cost.Mul(sellQty).Div(*qty))
		*qty = qty.Sub(sellQty)
	case "fee":
		if tx.Amount != nil {
			*cost = cost.Add(*tx.Amount)
		}
	}
}

// bondFaceUnit is the IDR nominal carried by one bond quantity unit (issue #27).
// Indonesian primary retail bonds (SBR/ST/ORI/SR) trade in IDR 1,000,000 units,
// so a bond's quantity is its nominal / 1,000,000 and price_per_unit is
// 1,000,000 at par; discount/premium is expressed via price_per_unit.
var bondFaceUnit = decimal.NewFromInt(1_000_000)

// outstandingFaceFromLedger derives a bond's held nominal from its transaction
// ledger (issue #27): (Σ buy_qty − Σ sell_qty) × 1,000,000. It replaces the
// dropped bond_details.face_value scalar — a hand-maintained total would be a
// duplicated, drift-prone source of truth (ADR-0003). Bond list/detail responses
// surface it as OutstandingFace; it stays correct across multi-tranche top-ups
// and partial sells by construction (the running Σ over the ledger).
func outstandingFaceFromLedger(txns []db.InvestmentTransaction) decimal.Decimal {
	qty := decimal.Zero
	for _, tx := range txns {
		if tx.Quantity == nil {
			continue
		}
		switch tx.TransactionType {
		case "buy":
			qty = qty.Add(*tx.Quantity)
		case "sell":
			qty = qty.Sub(*tx.Quantity)
		}
	}
	return qty.Mul(bondFaceUnit)
}

// transactionAggregates summarises a position's ledger for list rows (issue
// #67): the total transaction count and the most-recent transaction date as a
// YYYY-MM-DD string (nil when the ledger is empty). txns MUST be ordered by
// transaction_date ascending — ListInvestmentTransactionsByInvestmentIDs is —
// so the last element is the latest. The date is formatted plain (no time
// component) so list rows render a clean day without the storage timestamp.
func transactionAggregates(txns []db.InvestmentTransaction) (int, *string) {
	if len(txns) == 0 {
		return 0, nil
	}
	last := txns[len(txns)-1].TransactionDate.Format("2006-01-02")
	return len(txns), &last
}

// groupTransactionsByInvestment buckets a flat batch result by investment_id,
// preserving the query's ascending date order within each bucket.
func groupTransactionsByInvestment(txns []db.InvestmentTransaction) map[uuid.UUID][]db.InvestmentTransaction {
	byID := make(map[uuid.UUID][]db.InvestmentTransaction)
	for _, tx := range txns {
		byID[tx.InvestmentID] = append(byID[tx.InvestmentID], tx)
	}
	return byID
}
