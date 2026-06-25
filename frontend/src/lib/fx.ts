import type { FxRate } from "@/api/types";

// Q15c side-by-side display (ADR-0010): the monthly report stores every figure
// in the household reporting currency. To show a figure in a second currency we
// project it at that month's FX rate — the most recent rate with
// year_month <= M (carry-forward, mirroring snapshot + rate resolution in
// ADR-0002/0006). `rate` is reporting-currency units per 1 unit of `currency`,
// so foreign = reporting / rate. Pure display arithmetic at household scale —
// the backend owns the authoritative converted totals (see lib/format.ts).

export type DisplayRate = {
  rate: number;
  rateMonth: string; // year_month the rate is actually from (< M when carried forward)
};

// availableDisplayCurrencies lists currencies the user can project into: any
// currency with at least one rate, minus the reporting currency (trivially 1:1,
// stores no rate row). Sorted alphabetically for a stable selector.
export function availableDisplayCurrencies(
  rates: FxRate[],
  reportingCurrency: string,
): string[] {
  const set = new Set<string>();
  for (const r of rates) {
    if (r.currency !== reportingCurrency) set.add(r.currency);
  }
  return [...set].sort();
}

// resolveDisplayRate finds the carry-forward rate for `currency` in month
// `yearMonth`: the most recent rate with year_month <= yearMonth. Returns null
// when no rate exists on or before that month (the figure can't be projected)
// or the stored rate is non-positive/garbage.
export function resolveDisplayRate(
  rates: FxRate[],
  currency: string,
  yearMonth: string,
): DisplayRate | null {
  const m = yearMonth.slice(0, 7); // YYYY-MM
  let best: FxRate | null = null;
  for (const r of rates) {
    if (r.currency !== currency) continue;
    if (r.year_month.slice(0, 7) > m) continue;
    if (!best || r.year_month.slice(0, 7) > best.year_month.slice(0, 7))
      best = r;
  }
  if (!best) return null;
  const rate = Number(best.rate);
  if (!Number.isFinite(rate) || rate <= 0) return null;
  return { rate, rateMonth: best.year_month };
}

// convert projects a reporting-currency amount (string on the wire for
// precision) into the display currency using a resolved rate.
export function convert(reportingAmount: string, rate: number): number {
  return Number(reportingAmount) / rate;
}
