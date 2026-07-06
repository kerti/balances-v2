// Generic value-only cross-subtype aggregator for the **group Home** screens
// (epic #204) — the shared helper behind AssetsHome and LiabilitiesHome.
//
// Sibling to `lib/homeAggregates.ts` (the investments Home). That one carries
// cost basis + a risk pie; the asset/liability/receivable groups have **no
// cost basis** (ADR-0022 shared snapshot table, no ledger), so this is the
// value-only path: it reuses `aggregateListPositions` for the per-currency
// headline + total-over-time series (cost omitted → 0), and adds a generic
// category stack + composition pie keyed by a caller-supplied category list.
//
// No FX. Same 14c convention as the list screens: one card-set per currency.
//
// Current-state output (the pie) is active-only; the time + stack series
// include terminated positions historically, capped at `terminated_at` —
// inherited from `aggregateListPositions` / mirrored in the stack walk.

import {
  aggregateListPositions,
  type CurrencyAggregate,
  type Position,
  type TimePoint,
} from "@/lib/listAggregates";
import { monthRange } from "@/lib/months";

// A group position is a value-only `Position` (no cost / costSeries) tagged
// with its subtype category — a free-form string so each group supplies its
// own set (bankAccount/property/vehicle, personal/institutional, …).
export type GroupPosition = Omit<Position, "cost" | "costSeries"> & {
  category: string;
};

export type GroupCategoryTimePoint = {
  year_month: string;
  byCategory: Record<string, number>;
};

export type GroupCategorySlice = {
  category: string;
  value: number;
};

export type GroupHomeAggregates = {
  byCurrency: CurrencyAggregate[];
  timeSeriesByCurrency: Map<string, TimePoint[]>;
  categorySeriesByCurrency: Map<string, GroupCategoryTimePoint[]>;
  categoryPieByCurrency: Map<string, GroupCategorySlice[]>;
  count: number;
};

const monthOf = (s: string) => s.slice(0, 7);

// aggregateGroupHome merges value-only positions into per-currency cards.
// `categories` fixes the key order for the stack + pie (so a legend renders
// stably even when a household holds only one subtype).
export function aggregateGroupHome(
  positions: GroupPosition[],
  categories: string[],
): GroupHomeAggregates {
  // Headline + total-over-time come straight from the list aggregator. Cost
  // is omitted, so every TimePoint.cost is 0 — callers render value only.
  const base = aggregateListPositions(
    positions.map((p) => ({
      id: p.id,
      currency: p.currency,
      status: p.status,
      terminated_at: p.terminated_at,
      latestValue: p.latestValue,
      snapshots: p.snapshots,
    })),
  );

  const byCurrencyAll = new Map<string, GroupPosition[]>();
  const byCurrencyActive = new Map<string, GroupPosition[]>();
  for (const p of positions) {
    if (!byCurrencyAll.has(p.currency)) byCurrencyAll.set(p.currency, []);
    byCurrencyAll.get(p.currency)!.push(p);
    if (p.status === "active") {
      if (!byCurrencyActive.has(p.currency)) byCurrencyActive.set(p.currency, []);
      byCurrencyActive.get(p.currency)!.push(p);
    }
  }

  const categorySeriesByCurrency = new Map<string, GroupCategoryTimePoint[]>();
  const categoryPieByCurrency = new Map<string, GroupCategorySlice[]>();

  for (const [currency, ps] of byCurrencyAll) {
    const series = aggregateMonthlyByCategory(ps, categories);
    if (series.length > 0) categorySeriesByCurrency.set(currency, series);
  }
  for (const [currency, ps] of byCurrencyActive) {
    categoryPieByCurrency.set(currency, currentCategoryPie(ps, categories));
  }

  return {
    byCurrency: base.byCurrency,
    timeSeriesByCurrency: base.timeSeriesByCurrency,
    categorySeriesByCurrency,
    categoryPieByCurrency,
    count: base.count,
  };
}

// Carry-forward monthly walk per category — the generic twin of
// homeAggregates' aggregateMonthlyByCategory. Closed positions contribute up
// to but EXCLUDING their terminated_at month (issue #21 cap), same as the
// value series in listAggregates' aggregateMonthly.
function aggregateMonthlyByCategory(
  positions: GroupPosition[],
  categories: string[],
): GroupCategoryTimePoint[] {
  const emptyByCategory = (): Record<string, number> =>
    Object.fromEntries(categories.map((c) => [c, 0]));

  type Sorted = {
    category: string;
    months: string[];
    values: number[];
    termMonth: string | null;
  };
  const sorted: Sorted[] = positions.map((p) => {
    const termMonth = p.terminated_at ? monthOf(p.terminated_at) : null;
    const live = (m: string) => termMonth === null || m < termMonth;
    const byMonth = new Map<string, number>();
    for (const s of p.snapshots) {
      const m = monthOf(s.year_month);
      if (!live(m)) continue;
      byMonth.set(m, Number(s.amount));
    }
    const months = [...byMonth.keys()].sort();
    return {
      category: p.category,
      months,
      values: months.map((m) => byMonth.get(m)!),
      termMonth,
    };
  });

  const present = [...new Set(sorted.flatMap((s) => s.months))].sort();
  if (present.length === 0) return [];
  const allMonths = monthRange(present[0], present[present.length - 1]);

  const out: GroupCategoryTimePoint[] = [];
  const cursors = sorted.map(() => -1);
  for (const month of allMonths) {
    const byCategory = emptyByCategory();
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
        byCategory[sorted[i].category] += sorted[i].values[cursors[i]];
      }
    }
    out.push({ year_month: month, byCategory });
  }
  return out;
}

function currentCategoryPie(
  positions: GroupPosition[],
  categories: string[],
): GroupCategorySlice[] {
  const totals: Record<string, number> = Object.fromEntries(categories.map((c) => [c, 0]));
  for (const p of positions) {
    if (p.latestValue === null) continue;
    totals[p.category] += p.latestValue;
  }
  // Emit every key (even at zero) so the legend order is stable.
  return categories.map((category) => ({ category, value: totals[category] }));
}
