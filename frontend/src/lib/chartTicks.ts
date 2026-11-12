// X-axis tick selection for the monthly time-series charts. Recharts' own
// thinning anchors on the last tick, so a series starting Sep 2024 could open
// its axis on "Oct 24" (#839). Here index 0 is always a tick and the rest follow
// at one fixed step, so labels are evenly spaced and never closer than
// `minGapPx`. The last month is not pinned: it is a tick only when the step
// happens to land on it.

// "Oct 24" at 12px is ~40px wide; leave breathing room around it.
export const MIN_TICK_GAP_PX = 56;

export function pickTickIndices(
  count: number,
  plotWidth: number,
  minGapPx: number = MIN_TICK_GAP_PX,
): number[] {
  if (count <= 0) return [];
  const maxLabels = Math.max(1, Math.floor(plotWidth / minGapPx));
  const step = Math.max(1, Math.ceil(count / maxLabels));
  const out: number[] = [];
  for (let i = 0; i < count; i += step) out.push(i);
  return out;
}
