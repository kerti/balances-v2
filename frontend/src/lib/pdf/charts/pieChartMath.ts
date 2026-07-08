// Pure donut-slice geometry — the pie-chart twin of lineChartMath.ts. Each
// slice is an SVG path for an annular wedge (outer arc forward, inner arc
// backward), computed from a value's fraction of the total.

export type PieSliceGeometry = { key: string; value: number; path: string };

export type PieChartSlice = { key: string; value: number };

// On-brand qualitative palette — the same indigo/slate family as the wordmark
// (Wordmark.tsx) and the app's Tailwind theme, not a generic chart palette.
// Lives here (not PieChart.tsx) so PieChart.tsx only exports components —
// react-refresh/only-export-components forbids mixing the two.
export const PIE_CHART_PALETTE = [
  "#6366F1",
  "#0F172A",
  "#94A3B8",
  "#818CF8",
  "#334155",
  "#A5B4FC",
  "#CBD5E1",
  "#4F46E5",
];

const TAU = Math.PI * 2;
// A full-circle sweep degenerates an SVG arc's start/end points to the same
// point (invisible path) — cap just short of it so a single 100% slice still
// renders as a ring.
const MAX_SWEEP = TAU - 1e-6;

function polar(cx: number, cy: number, r: number, angle: number): { x: number; y: number } {
  return { x: cx + r * Math.sin(angle), y: cy - r * Math.cos(angle) };
}

function donutWedgePath(
  cx: number,
  cy: number,
  rOuter: number,
  rInner: number,
  startAngle: number,
  endAngle: number,
): string {
  const large = endAngle - startAngle > Math.PI ? 1 : 0;
  const p0 = polar(cx, cy, rOuter, startAngle);
  const p1 = polar(cx, cy, rOuter, endAngle);
  const p2 = polar(cx, cy, rInner, endAngle);
  const p3 = polar(cx, cy, rInner, startAngle);
  return [
    `M ${p0.x} ${p0.y}`,
    `A ${rOuter} ${rOuter} 0 ${large} 1 ${p1.x} ${p1.y}`,
    `L ${p2.x} ${p2.y}`,
    `A ${rInner} ${rInner} 0 ${large} 0 ${p3.x} ${p3.y}`,
    "Z",
  ].join(" ");
}

// computeDonutSlices turns values into wedge paths, in input order, starting
// at 12 o'clock and sweeping clockwise. Zero/negative-value entries and an
// all-zero/empty input produce no slices (nothing to draw).
export function computeDonutSlices(
  data: { key: string; value: number }[],
  opts: { cx: number; cy: number; rOuter: number; rInner: number },
): PieSliceGeometry[] {
  const total = data.reduce((sum, d) => sum + Math.max(d.value, 0), 0);
  if (total <= 0) return [];

  let angle = 0;
  const out: PieSliceGeometry[] = [];
  for (const d of data) {
    if (d.value <= 0) continue;
    const sweep = Math.min((d.value / total) * TAU, MAX_SWEEP);
    out.push({
      key: d.key,
      value: d.value,
      path: donutWedgePath(opts.cx, opts.cy, opts.rOuter, opts.rInner, angle, angle + sweep),
    });
    angle += sweep;
  }
  return out;
}
