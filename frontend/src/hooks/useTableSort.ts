import { useMemo, useState } from "react";
import type { Comparator, SortDir } from "@/lib/sort";

export type ColumnSort<T> = {
  // Direction applied on the first click of this column; clicking the active
  // column again toggles.
  dir: SortDir;
  cmp: Comparator<T>;
};

type Options<T> = {
  defaultKey: string;
  // Applied when the active column compares equal (e.g. fall back to name).
  // Direction-independent so the tiebreak order stays stable.
  tiebreak?: (a: T, b: T) => number;
};

// Single-column client-side sort for the list screens. `columns` and `tiebreak`
// must be referentially stable (wrap them in useMemo / define outside render),
// since the sorted result memoizes on their identity.
export function useTableSort<K extends string, T>(
  rows: T[],
  columns: Record<K, ColumnSort<T>>,
  { defaultKey, tiebreak }: Options<T>,
) {
  const [sort, setSort] = useState<{ key: K; dir: SortDir }>(() => ({
    key: defaultKey as K,
    dir: columns[defaultKey as K].dir,
  }));

  function toggle(key: K) {
    setSort((s) =>
      s.key === key
        ? { key, dir: s.dir === "asc" ? "desc" : "asc" }
        : { key, dir: columns[key].dir },
    );
  }

  const sorted = useMemo(() => {
    const { cmp } = columns[sort.key];
    return [...rows].sort((a, b) => {
      const primary = cmp(a, b, sort.dir);
      if (primary !== 0) return primary;
      return tiebreak ? tiebreak(a, b) : 0;
    });
  }, [rows, sort, columns, tiebreak]);

  return { sorted, sortKey: sort.key, sortDir: sort.dir, toggle };
}
