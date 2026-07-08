import { monthRange } from "@/lib/months";

export type ChartPoint = { x: number; y: number };

export type LineChartGeometry = {
  path: string;
  points: ChartPoint[];
  months: string[];
  minY: number;
  maxY: number;
};

// "YYYY-MM" -> a real Date, for feeding month labels through formatChartMonth.
export function monthToDate(yearMonth: string): Date {
  const [y, m] = yearMonth.split("-").map(Number);
  return new Date(y, m - 1, 1);
}

// Fills gap months by carrying the last known amount forward, mirroring
// SnapshotChartImpl's toChartData — a skipped month keeps the timeline
// proportional instead of collapsing the gap (issue #24). Returns the
// enumerated months alongside the amounts so callers can label the x axis.
function fillGaps(series: { year_month: string; amount: string }[]): {
  months: string[];
  amounts: number[];
} {
  const byMonth = new Map<string, number>();
  for (const s of series) byMonth.set(s.year_month.slice(0, 7), Number(s.amount));
  const sortedMonths = [...byMonth.keys()].sort();
  const months = monthRange(sortedMonths[0], sortedMonths[sortedMonths.length - 1]);
  let last = 0;
  const amounts = months.map((ym) => {
    if (byMonth.has(ym)) last = byMonth.get(ym)!;
    return last;
  });
  return { months, amounts };
}

// Same indigo/slate family TrendChart.tsx draws with — living expenses in
// slate, passive income (investment return) in indigo, matching the app's
// "spend vs. grow" framing elsewhere. Lives here (not TrendChart.tsx) so that
// file only exports a component — react-refresh/only-export-components
// forbids mixing the two.
export const TREND_LIVING_EXPENSES_COLOR = "#334155";
export const TREND_INVESTMENT_RETURN_COLOR = "#6366F1";

export type TwoLineChartGeometry = {
  pathA: string;
  pathB: string;
  pointsA: ChartPoint[];
  pointsB: ChartPoint[];
  minY: number;
  maxY: number;
};

// computeTwoLineGeometry draws two pre-aligned numeric series (no gap-filling
// — the trend chart's caller already sources both from the same contiguous
// monthly report rows, unlike the sparse/possibly-gapped series
// computeLineChartGeometry handles) on one shared y-scale, so they're
// visually comparable on a single chart.
export function computeTwoLineGeometry(
  a: number[],
  b: number[],
  opts: { width: number; height: number },
): TwoLineChartGeometry {
  if (a.length === 0 || b.length === 0) {
    return { pathA: "", pathB: "", pointsA: [], pointsB: [], minY: 0, maxY: 0 };
  }
  const all = [...a, ...b];
  const minY = Math.min(...all);
  const maxY = Math.max(...all);
  const range = maxY - minY;
  const scaleY = (v: number): number =>
    range === 0 ? opts.height / 2 : opts.height - ((v - minY) / range) * opts.height;

  const toPoints = (values: number[]): ChartPoint[] =>
    values.map((v, i) => ({
      x: values.length === 1 ? 0 : (i / (values.length - 1)) * opts.width,
      y: scaleY(v),
    }));
  const toPath = (points: ChartPoint[]): string =>
    points.map((p, i) => `${i === 0 ? "M" : "L"} ${p.x} ${p.y}`).join(" ");

  const pointsA = toPoints(a);
  const pointsB = toPoints(b);
  return { pathA: toPath(pointsA), pathB: toPath(pointsB), pointsA, pointsB, minY, maxY };
}

export function computeLineChartGeometry(
  series: { year_month: string; amount: string }[],
  opts: { width: number; height: number },
): LineChartGeometry {
  if (series.length === 0) {
    return { path: "", points: [], months: [], minY: 0, maxY: 0 };
  }

  const { months, amounts } = fillGaps(series);
  const minY = Math.min(...amounts);
  const maxY = Math.max(...amounts);
  const range = maxY - minY;

  const scaleY = (v: number): number =>
    range === 0 ? opts.height / 2 : opts.height - ((v - minY) / range) * opts.height;

  const points: ChartPoint[] = amounts.map((v, i) => ({
    x: amounts.length === 1 ? 0 : (i / (amounts.length - 1)) * opts.width,
    y: scaleY(v),
  }));

  const path = points.map((p, i) => `${i === 0 ? "M" : "L"} ${p.x} ${p.y}`).join(" ");

  return { path, points, months, minY, maxY };
}
