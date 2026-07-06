// Cost-basis computation from the investment-transaction ledger.
//
// Convention: avg-cost FIFO-ish — buys add to cost + quantity; sells reduce
// both proportionally (cost / qty × sellQty subtracted). Fee transactions
// capitalize into cost (treating them as part of the all-in money put in).
// Coupon / Dividend / Distribution are income, not cost adjustments —
// ignored. Maturity is terminal — ignored (position lifecycle handles the
// "this is over now" semantics).
//
// Subtype quirks the caller handles, not this helper:
//   - TimeDeposit: ledger has only Maturity transactions; cost = principal
//     from time_deposit_details. Use `flatCostSeries`.
//   - Stock / MutualFund / Gold / Bond: pure ledger replay via
//     `costBasisSeries`. Bonds always carry a Buy at placement now (govt_primary
//     seeded at par, secondary recorded by the user — issue #27).

import type { InvestmentTransaction } from "@/api/types";

export type CostBasis = { cost: number; heldQty: number };

// Snapshots come in any order from callers; the chart sorts them itself
// before plotting. This helper sorts internally for the cumulative walk
// and returns one entry per input snapshot (in the input order) so the
// caller can zip it back with `snapshots` by index.
type SnapshotLike = { year_month: string };

export function computeCostBasis(transactions: InvestmentTransaction[]): CostBasis {
  let cost = 0;
  let qty = 0;
  for (const tx of orderedByDate(transactions)) {
    applyTxn(
      tx,
      (dCost, dQty) => {
        cost += dCost;
        qty += dQty;
      },
      () => ({ cost, qty }),
    );
  }
  return { cost, heldQty: qty };
}

// Parallel cost-basis series at each snapshot month. Cost at month M is
// the cumulative replay of all transactions dated in months ≤ M (yyyy-mm
// prefix comparison — mid-month txns roll into that month's snapshot).
export function costBasisSeries(
  snapshots: SnapshotLike[],
  transactions: InvestmentTransaction[],
): Array<{ year_month: string; cost: number }> {
  const monthOf = (s: string) => s.slice(0, 7);
  const ordered = orderedByDate(transactions);
  const monthsAsc = [...new Set(snapshots.map((s) => monthOf(s.year_month)))].sort();

  let cost = 0;
  let qty = 0;
  let i = 0;
  const costByMonth = new Map<string, number>();
  for (const ym of monthsAsc) {
    while (i < ordered.length && monthOf(ordered[i].transaction_date) <= ym) {
      applyTxn(
        ordered[i],
        (dCost, dQty) => {
          cost += dCost;
          qty += dQty;
        },
        () => ({ cost, qty }),
      );
      i++;
    }
    costByMonth.set(ym, cost);
  }
  return snapshots.map((s) => ({
    year_month: s.year_month,
    cost: costByMonth.get(monthOf(s.year_month)) ?? 0,
  }));
}

// Flat constant cost — used by TimeDeposit (principal) and Bond
// govt-primary (face value when no Buy txn exists). Returns one entry
// per input snapshot, all with the same `cost`.
export function flatCostSeries(
  snapshots: SnapshotLike[],
  cost: number,
): Array<{ year_month: string; cost: number }> {
  return snapshots.map((s) => ({ year_month: s.year_month, cost }));
}

// ---- internals --------------------------------------------------------

function orderedByDate(transactions: InvestmentTransaction[]): InvestmentTransaction[] {
  return [...transactions].sort((a, b) => a.transaction_date.localeCompare(b.transaction_date));
}

// Side-effecting reducer step. Reads current (cost, qty) via `read` so
// sell-side avg-cost calculation sees the running state, then emits the
// delta to apply via `apply`. Skips transactions with null shape fields
// (defensive — backend enforces shape per migration 00010 CHECK).
function applyTxn(
  tx: InvestmentTransaction,
  apply: (dCost: number, dQty: number) => void,
  read: () => { cost: number; qty: number },
) {
  const amt = tx.amount ? Number(tx.amount) : NaN;
  const q = tx.quantity ? Number(tx.quantity) : NaN;
  switch (tx.transaction_type) {
    case "buy":
      if (Number.isFinite(amt) && Number.isFinite(q)) apply(amt, q);
      break;
    case "sell": {
      if (!Number.isFinite(q)) return;
      const { cost, qty } = read();
      if (qty <= 0) return;
      const sellQty = Math.min(q, qty);
      apply(-((cost / qty) * sellQty), -sellQty);
      break;
    }
    case "fee":
      if (Number.isFinite(amt)) apply(amt, 0);
      break;
    // coupon, dividend, distribution: income — ignored.
    // maturity: terminal — ignored.
  }
}
