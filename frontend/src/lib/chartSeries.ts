// One display order for a chart's series, shared by its legend and its tooltip.
//
// Recharts orders them differently on its own: the legend sorts by dataKey
// ("amount", "assets", "investments", "liabilities"), while shadcn's
// ChartTooltipContent renders its payload in render order and ignores
// `itemSorter`. So the dashboard legend read Net worth / Assets / Investments /
// Liabilities and its tooltip Net worth / Assets / Liabilities / Investments.
// Both now sort through seriesRank against the one order seriesOrder returns.

// The primary area leads, then the cost baseline (detail screens), then the
// composition lines in the order the caller passed them.
export function seriesOrder(extraKeys: readonly string[], hasCost: boolean): string[] {
  return ["amount", ...(hasCost ? ["cost"] : []), ...extraKeys];
}

// A series' position in `order`; a key the order doesn't name sorts last.
export function seriesRank(order: readonly string[], key: unknown): number {
  const i = order.indexOf(String(key));
  return i === -1 ? order.length : i;
}

// A stable sort of `items` into `order`, by whichever key `keyOf` reads off each.
export function sortBySeriesOrder<T>(
  items: readonly T[],
  order: readonly string[],
  keyOf: (item: T) => unknown,
): T[] {
  return [...items].sort((a, b) => seriesRank(order, keyOf(a)) - seriesRank(order, keyOf(b)));
}
