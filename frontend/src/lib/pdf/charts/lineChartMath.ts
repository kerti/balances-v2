import { monthRange } from "@/lib/months";

export type ChartPoint = { x: number; y: number };

export type LineChartGeometry = {
  path: string;
  points: ChartPoint[];
  minY: number;
  maxY: number;
};

// Fills gap months by carrying the last known amount forward, mirroring
// SnapshotChartImpl's toChartData — a skipped month keeps the timeline
// proportional instead of collapsing the gap (issue #24).
function fillGaps(series: { year_month: string; amount: string }[]): number[] {
  const byMonth = new Map<string, number>();
  for (const s of series) byMonth.set(s.year_month.slice(0, 7), Number(s.amount));
  const months = [...byMonth.keys()].sort();
  let last = 0;
  return monthRange(months[0], months[months.length - 1]).map((ym) => {
    if (byMonth.has(ym)) last = byMonth.get(ym)!;
    return last;
  });
}

export function computeLineChartGeometry(
  series: { year_month: string; amount: string }[],
  opts: { width: number; height: number },
): LineChartGeometry {
  if (series.length === 0) {
    return { path: "", points: [], minY: 0, maxY: 0 };
  }

  const amounts = fillGaps(series);
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

  return { path, points, minY, maxY };
}
