// Shapes the backend's per-group breakdown (GET /api/reports/groups/{group},
// #845) into what the hub charts take. Pure: it only re-keys and re-types; every
// conversion already happened server-side in the dashboard's engine pass.

import type { GroupBreakdown, GroupBreakdownMonth } from "@/api/types";

// The wire names subtypes snake_case; the hubs' category keys (and their i18n
// labels) are camelCase.
const CATEGORY_KEY: Record<string, string> = {
  bank_account: "bankAccount",
  mutual_fund: "mutualFund",
  time_deposit: "timeDeposit",
};
const categoryKey = (subtype: string) => CATEGORY_KEY[subtype] ?? subtype;

const monthOf = (iso: string) => iso.slice(0, 7);

export function latestMonth(b: GroupBreakdown | undefined): GroupBreakdownMonth | null {
  return b && b.months.length > 0 ? b.months[b.months.length - 1] : null;
}

// Total value over time, in the shape SnapshotChart takes.
export function breakdownValueSeries(b: GroupBreakdown): { year_month: string; value: number }[] {
  return b.months.map((m) => ({ year_month: monthOf(m.year_month), value: Number(m.total) }));
}

function byCategory<K extends string>(
  values: Record<string, string> | undefined,
  keys: readonly K[],
  keyOf: (wire: string) => string,
): Record<K, number> {
  const out = Object.fromEntries(keys.map((k) => [k, 0])) as Record<K, number>;
  for (const [wire, v] of Object.entries(values ?? {})) {
    const k = keyOf(wire) as K;
    if (k in out) out[k] += Number(v);
  }
  return out;
}

// Monthly value per category, for the 100%-stacked share chart. Every key in
// `keys` is present on every point (zero when unheld) so the legend is stable.
export function breakdownCategorySeries<K extends string>(
  b: GroupBreakdown,
  keys: readonly K[],
): { year_month: string; byCategory: Record<K, number> }[] {
  return b.months.map((m) => ({
    year_month: monthOf(m.year_month),
    byCategory: byCategory(m.by_subtype, keys, categoryKey),
  }));
}

// The latest month's value per category / risk profile, for the pies.
export function breakdownCategorySlices<K extends string>(
  b: GroupBreakdown,
  keys: readonly K[],
): { category: K; value: number }[] {
  const latest = byCategory(latestMonth(b)?.by_subtype, keys, categoryKey);
  return keys.map((category) => ({ category, value: latest[category] }));
}

export function breakdownRiskSlices<K extends string>(
  b: GroupBreakdown,
  keys: readonly K[],
): { profile: K; value: number }[] {
  const latest = byCategory(latestMonth(b)?.by_risk, keys, (k) => k);
  return keys.map((profile) => ({ profile, value: latest[profile] }));
}

// A composition chart only says something when two or more categories are held.
// With one, the share area is a flat 100% band and the pie a full circle — both
// are left out rather than drawn (ADR-0022 amendment, #845).
export function categoriesHeld(series: { byCategory: Record<string, number> }[]): number {
  const held = new Set<string>();
  for (const p of series) {
    for (const [k, v] of Object.entries(p.byCategory)) if (v > 0) held.add(k);
  }
  return held.size;
}

export function slicesHeld(slices: { value: number }[]): number {
  return slices.filter((s) => s.value > 0).length;
}
