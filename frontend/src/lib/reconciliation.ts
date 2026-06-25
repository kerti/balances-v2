import type { InvestmentSnapshot, InvestmentTransaction } from "@/api/types";

// Reconciliation per CONTEXT.md / ADR-0003:
//   snapshot.quantity should equal Σ(Buy.qty) − Σ(Sell.qty) − Σ(Fee.qty_deducted)
// Only applies to subtypes that record quantity-based snapshots
// (Stock, MutualFund, Gold). Bond / TimeDeposit use accrued-interest
// snapshots — no quantity to reconcile against.
//
// Warning-only — bank/broker statements remain the source of truth, so a
// mismatch is a data-entry flag not a write block.

export type ReconciliationResult = {
  expected: number;
  actual: number;
  // Allow tiny floating-point drift; treat anything within 1e-6 of expected
  // as a match. (Backend stores DECIMAL(20,8) so legitimate sub-unit deltas
  // round to zero at our display scale.)
  matches: boolean;
};

export function reconcileQuantity(
  latestSnapshot: InvestmentSnapshot | null | undefined,
  transactions: InvestmentTransaction[] | undefined,
): ReconciliationResult | null {
  if (!latestSnapshot || latestSnapshot.quantity === null) return null;
  if (!transactions || transactions.length === 0) return null;

  const expected = transactions.reduce((acc, t) => {
    if (t.transaction_type === "buy" && t.quantity) {
      return acc + Number(t.quantity);
    }
    if (t.transaction_type === "sell" && t.quantity) {
      return acc - Number(t.quantity);
    }
    if (t.transaction_type === "fee" && t.quantity) {
      return acc - Number(t.quantity);
    }
    return acc;
  }, 0);

  const actual = Number(latestSnapshot.quantity);
  const matches = Math.abs(expected - actual) < 1e-6;
  return { expected, actual, matches };
}
