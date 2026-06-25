// Continuous "YYYY-MM" month sequencing for value-over-time charts.
//
// Every chart renders on a *categorical* X axis (one tick per data point),
// so a series that skips months — gap months where no snapshot was taken,
// or no position reported — collapses the timeline and spaces unequal time
// gaps equally (issue #24). Enumerating the full inclusive range lets the
// carry-forward fill downstream keep the timeline proportional.

// Absolute month index (year*12 + month-1) for a "YYYY-MM(-…)" string.
// Inputs may be bare "YYYY-MM" or the API's "YYYY-MM-DDT…" shape; only the
// leading year+month are read. Returns null on anything malformed.
function monthIndex(s: string): number | null {
  const m = /^(\d{4})-(\d{2})/.exec(s);
  if (!m) return null;
  const mo = Number(m[2]);
  if (mo < 1 || mo > 12) return null;
  return Number(m[1]) * 12 + (mo - 1);
}

// Inclusive list of bare "YYYY-MM" strings from `first` to `last`. Returns
// [] if either bound is malformed or `last` precedes `first`, and [first]
// when the two coincide.
export function monthRange(first: string, last: string): string[] {
  const a = monthIndex(first);
  const b = monthIndex(last);
  if (a === null || b === null || b < a) return [];
  const out: string[] = [];
  for (let i = a; i <= b; i++) {
    out.push(`${Math.floor(i / 12)}-${String((i % 12) + 1).padStart(2, "0")}`);
  }
  return out;
}
