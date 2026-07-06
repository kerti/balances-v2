// Comparator builders for client-side table sorting (the list screens). Each
// builder returns a direction-aware comparator so a single function handles
// both asc and desc; null handling that must stay direction-independent (e.g.
// rows with no value sinking to the bottom either way) is baked into the
// builder rather than left to the caller.

export type SortDir = "asc" | "desc";

export type Comparator<T> = (a: T, b: T, dir: SortDir) => number;

// Locale-aware text sort.
export function byText<T>(get: (t: T) => string): Comparator<T> {
  return (a, b, dir) => {
    const r = get(a).localeCompare(get(b));
    return dir === "asc" ? r : -r;
  };
}

// Numeric sort with nulls always last, regardless of direction — a position
// with no snapshot has no balance to rank, so it sinks to the bottom whether
// you sort ascending or descending.
export function byNumberNullsLast<T>(get: (t: T) => number | null): Comparator<T> {
  return (a, b, dir) => {
    const av = get(a);
    const bv = get(b);
    if (av === null || bv === null) {
      if (av === null && bv === null) return 0;
      return av === null ? 1 : -1;
    }
    const r = av - bv;
    return dir === "asc" ? r : -r;
  };
}
