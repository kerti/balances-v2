// Aggregator for the investment list-screen headline + time graph (issue
// #14, slice 14c). Pure: takes per-position {value, cost, snapshots,
// costSeries} and emits per-currency totals + per-currency monthly time
// series with carry-forward.
//
// **No FX.** Currencies stay separate, matching the no-FX convention on
// existing list-screen headlines (`lib/totals.ts`). Mixed-currency
// households get one card per currency.
//
// **Headline + count are active-only** (`activeCurrencyTotals` parity —
// closed positions have no current value/cost). **Time series includes
// terminated positions historically** (issue #21): each position
// contributes carry-forward up to but *excluding* its `terminated_at`
// month, then drops out. Without this, a sold/matured position
// vanishes from the chart entirely, hiding the portfolio's past
// shape.

export type Position = {
  id: string;
  currency: string;
  // `'active'` | `'sold'` | `'matured'` | …  Only `'active'` rows count
  // toward the headline; closed rows still appear in the time series up
  // to their `terminated_at` month.
  status: string;
  // ISO timestamp for closed positions, null for active. Caps the
  // position's contribution to the time series at that YYYY-MM.
  terminated_at: string | null;
  latestValue: number | null;
  // "As of now" cost basis. Caller computes via lib/costBasis per the
  // subtype's quirk (ledger replay for Stock/MF/Gold/Bond; flat principal
  // for TD — bonds always carry a Buy at placement now, issue #27).
  // **Optional**: the non-investment groups (assets / liabilities /
  // receivables) have no cost basis (epic #204), so they omit it and the
  // aggregator treats cost as 0 — a value-only series.
  cost?: number;
  snapshots: Array<{ year_month: string; amount: string }>;
  // Aligned with snapshots by year_month — caller computes via
  // costBasisSeries (ledger) or flatCostSeries (constant). Omitted by the
  // value-only (non-investment) groups.
  costSeries?: Array<{ year_month: string; cost: number }>;
};

export type CurrencyAggregate = {
  currency: string;
  value: number;
  cost: number;
  pl: number;
};

export type TimePoint = {
  year_month: string; // bare "YYYY-MM"
  value: number;
  cost: number;
};

export type ListAggregates = {
  byCurrency: CurrencyAggregate[]; // value desc; currency code breaks ties
  timeSeriesByCurrency: Map<string, TimePoint[]>;
  // Active positions that contributed a balance — for the "N stocks"
  // count under the headline.
  count: number;
};

import { monthRange } from "@/lib/months";

const monthOf = (s: string) => s.slice(0, 7);

export function aggregateListPositions(positions: Position[]): ListAggregates {
  const active = positions.filter((p) => p.status === "active");

  // Per-currency current totals.
  const currencyMap = new Map<string, { value: number; cost: number; count: number }>();
  for (const p of active) {
    const entry = currencyMap.get(p.currency) ?? {
      value: 0,
      cost: 0,
      count: 0,
    };
    if (p.latestValue !== null) {
      entry.value += p.latestValue;
      entry.count++;
    }
    // Cost always contributes — a position with no snapshot still has a
    // cost basis (e.g. a freshly-placed bond whose Buy is recorded).
    // Value-only groups omit cost entirely → treated as 0.
    entry.cost += p.cost ?? 0;
    currencyMap.set(p.currency, entry);
  }

  const byCurrency: CurrencyAggregate[] = [...currencyMap.entries()]
    .map(([currency, { value, cost }]) => ({
      currency,
      value,
      cost,
      pl: value - cost,
    }))
    .sort((a, b) => b.value - a.value || a.currency.localeCompare(b.currency));

  // Per-currency monthly series (carry-forward). Includes closed
  // positions historically (issue #21) — `aggregateMonthly` caps each
  // position at its `terminated_at` month.
  const byCurrencyPositions = new Map<string, Position[]>();
  for (const p of positions) {
    if (!byCurrencyPositions.has(p.currency)) {
      byCurrencyPositions.set(p.currency, []);
    }
    byCurrencyPositions.get(p.currency)!.push(p);
  }

  const timeSeriesByCurrency = new Map<string, TimePoint[]>();
  for (const [currency, positionsInCurrency] of byCurrencyPositions) {
    const series = aggregateMonthly(positionsInCurrency);
    if (series.length > 0) {
      timeSeriesByCurrency.set(currency, series);
    }
  }

  // Active positions with a value — for the headline subline count.
  const count = active.filter((p) => p.latestValue !== null).length;

  return { byCurrency, timeSeriesByCurrency, count };
}

// Walks the union of snapshot months across the positions in one
// currency. For each month M and each position p, picks p's latest
// snapshot at-or-before M (carry-forward) and its cost-basis at M, then
// sums. Cursors are per-position so the inner loop is O(1) amortized
// across the sorted months.
//
// Closed positions (terminated_at set) contribute carry-forward up to but
// EXCLUDING their termination month, then drop out. A position paid out /
// matured by month-end isn't held at that month-end, and the
// termination-month snapshot is the synthetic 0-close (#25/#27) anyway — so
// excluding it loses nothing real. This (a) keeps a same-month rollover from
// double-counting (predecessor carried + successor real, one spike month) and
// (b) matches the per-position detail chart, which ends a closed position at
// its last real snapshot + a Sold/Matured marker, never plotting the 0-close.
function aggregateMonthly(positions: Position[]): TimePoint[] {
  type Sorted = {
    months: string[];
    values: number[];
    costs: number[];
    termMonth: string | null;
  };
  const sorted: Sorted[] = positions.map((p) => {
    const termMonth = p.terminated_at ? monthOf(p.terminated_at) : null;
    // A closed position is held only *through the month before* terminated_at.
    // Drop its termination month (and any later snapshot) here so it neither
    // extends the timeline into a 0-close/payout month — which would crater a
    // lone closed position to 0 at the end — nor reaches the sum. The walk
    // cap below stops the carried value leaking past termMonth when *another*
    // position extends the range. The termination-month snapshot is the
    // synthetic 0-close (#25/#27) anyway, so excluding it loses nothing real.
    const live = (m: string) => termMonth === null || m < termMonth;
    // Build {month → (value, cost)} so snapshot + cost lookups merge by
    // month even if the input arrays are in different orders.
    const byMonth = new Map<string, { value: number; cost: number }>();
    for (const s of p.snapshots) {
      const m = monthOf(s.year_month);
      if (!live(m)) continue;
      byMonth.set(m, { value: Number(s.amount), cost: 0 });
    }
    for (const c of p.costSeries ?? []) {
      const m = monthOf(c.year_month);
      if (!live(m)) continue;
      const entry = byMonth.get(m) ?? { value: 0, cost: 0 };
      entry.cost = c.cost;
      byMonth.set(m, entry);
    }
    const months = [...byMonth.keys()].sort();
    return {
      months,
      values: months.map((m) => byMonth.get(m)!.value),
      costs: months.map((m) => byMonth.get(m)!.cost),
      termMonth,
    };
  });

  const present = [...new Set(sorted.flatMap((s) => s.months))].sort();
  if (present.length === 0) return [];
  // Walk the full continuous range, not just months that carry a snapshot,
  // so the categorical X axis renders a proportional timeline (#24). Gap
  // months take carry-forward values from the cursors below.
  const allMonths = monthRange(present[0], present[present.length - 1]);

  const out: TimePoint[] = [];
  const cursors = sorted.map(() => -1);
  for (const month of allMonths) {
    let value = 0;
    let cost = 0;
    for (let i = 0; i < sorted.length; i++) {
      if (sorted[i].termMonth !== null && month >= sorted[i].termMonth!) {
        continue;
      }
      while (
        cursors[i] + 1 < sorted[i].months.length &&
        sorted[i].months[cursors[i] + 1] <= month
      ) {
        cursors[i]++;
      }
      if (cursors[i] >= 0) {
        value += sorted[i].values[cursors[i]];
        cost += sorted[i].costs[cursors[i]];
      }
    }
    out.push({ year_month: month, value, cost });
  }
  return out;
}
