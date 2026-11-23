// X-axis tick selection for the monthly time-series charts. Recharts' own
// thinning anchors on the last tick, so a series starting Sep 2024 could open
// its axis on "Oct 24" (#839). Here index 0 is always a tick and the rest follow
// at one fixed calendar step, so labels are evenly spaced and never closer than
// `minGapPx`. The last month is not pinned: it is a tick only when the step
// happens to land on it.

// The step is a calendar-friendly month count, so every chart on a page lands
// on the same cadence (monthly, bi-monthly, quarterly, half-yearly, yearly...)
// rather than an arbitrary "every 5 months" (#950).
// Past 60 months the step rounds up to a whole number of five-year blocks.
export const TICK_STEPS = [1, 2, 3, 6, 12, 24, 36, 60] as const;

// "Oct 24" at 12px is ~40px wide; leave breathing room around it.
export const MIN_TICK_GAP_PX = 56;

export function pickTickIndices(
  count: number,
  plotWidth: number,
  minGapPx: number = MIN_TICK_GAP_PX,
): number[] {
  if (count <= 0) return [];
  const maxLabels = Math.max(1, Math.floor(plotWidth / minGapPx));
  const minStep = Math.max(1, Math.ceil(count / maxLabels));
  const step = TICK_STEPS.find((s) => s >= minStep) ?? Math.ceil(minStep / 60) * 60;
  const out: number[] = [];
  for (let i = 0; i < count; i += step) out.push(i);
  return out;
}
