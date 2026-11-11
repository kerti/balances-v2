// Y-axis domain fitting for value-over-time charts (#852).
//
// A line chart of a position growing a few percent a year, drawn from a zero
// baseline, looks flat: the whole climb is squeezed into the top sliver. So a
// line chart fits its domain to the visible data with a small pad, snapped
// outward to round tick values so the lowest tick is a real, readable number.
// A stacked chart only reads from zero and keeps it (`includeZero`). Any
// series that crosses zero spans it, and 0 is always one of the ticks.

export type FittedDomain = {
  domain: [number, number];
  ticks: number[];
  // True when the domain spans zero, so the chart can draw an explicit 0 line.
  crossesZero: boolean;
};

type Options = {
  // Stacked area / bar: the axis must start at 0.
  includeZero?: boolean;
  // Approximate number of ticks wanted; the 1-2-5 step snapping may yield a few
  // more or fewer.
  tickCount?: number;
  // Fraction of the data span added above and below before snapping.
  padding?: number;
};

// A round step (1, 2, 2.5, 5 x 10^n) close to raw.
function niceStep(raw: number): number {
  const exp = Math.floor(Math.log10(raw));
  const base = 10 ** exp;
  const f = raw / base;
  // Thresholds sit a little above 1/2/5 so a raw step just over one does not
  // jump to double it; a coarse step would snap the lowest tick far below the
  // data and bring back the flat-chart problem.
  const nice = f <= 1.2 ? 1 : f <= 2.4 ? 2 : f <= 3 ? 2.5 : f <= 6 ? 5 : 10;
  return nice * base;
}

// Strip float noise (0.1 * 3 -> 0.30000000000000004) from a multiple of step.
function snap(n: number, step: number): number {
  const decimals = Math.max(0, -Math.floor(Math.log10(step)) + 1);
  return Number(n.toFixed(decimals));
}

export function fitYDomain(values: number[], opts: Options = {}): FittedDomain {
  const { includeZero = false, tickCount = 6, padding = 0.02 } = opts;
  const finite = values.filter((v) => Number.isFinite(v));
  if (finite.length === 0) return { domain: [0, 1], ticks: [0, 1], crossesZero: false };

  let min = Math.min(...finite);
  let max = Math.max(...finite);
  if (includeZero) {
    min = Math.min(min, 0);
    max = Math.max(max, 0);
  }

  // A flat series has no span to pad; borrow a slice of its magnitude.
  const span = max - min || Math.abs(max) * 0.1 || 1;
  const pad = span * padding;
  let lo = includeZero && min >= 0 ? 0 : min - pad;
  let hi = includeZero && max <= 0 ? 0 : max + pad;
  // Padding must not drag a never-negative series below zero.
  if (min >= 0) lo = Math.max(lo, 0);
  if (max <= 0) hi = Math.min(hi, 0);

  const step = niceStep((hi - lo) / Math.max(1, tickCount - 1));
  lo = snap(Math.floor(lo / step) * step, step);
  hi = snap(Math.ceil(hi / step) * step, step);

  const ticks: number[] = [];
  for (let v = lo; v <= hi + step / 1000; v += step) ticks.push(snap(v, step));
  return { domain: [lo, hi], ticks, crossesZero: lo < 0 && hi > 0 };
}
