// Wherever several currencies are listed together (#937): the reporting
// currency first, then the rest A-Z. An empty or absent reporting currency
// (session still loading) degrades to plain A-Z.
export function compareCurrencies(reporting: string): (a: string, b: string) => number {
  return (a, b) => {
    if (a === b) return 0;
    if (a === reporting) return -1;
    if (b === reporting) return 1;
    return a < b ? -1 : 1;
  };
}

export function orderCurrencies(currencies: Iterable<string>, reporting: string): string[] {
  return [...currencies].sort(compareCurrencies(reporting));
}

export function orderByCurrency<T extends { currency: string }>(
  items: Iterable<T>,
  reporting: string,
): T[] {
  const cmp = compareCurrencies(reporting);
  return [...items].sort((a, b) => cmp(a.currency, b.currency));
}

export function orderMapByCurrency<V>(map: Map<string, V>, reporting: string): Map<string, V> {
  const cmp = compareCurrencies(reporting);
  return new Map([...map.entries()].sort(([a], [b]) => cmp(a, b)));
}
