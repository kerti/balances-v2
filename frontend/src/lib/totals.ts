// Per-currency totals for a list screen's headline.
//
// Sums each position's latest-snapshot balance, grouped by currency, over
// ACTIVE positions only — terminated (closed/sold/etc.) positions are excluded
// so the headline reflects current holdings, and positions with no snapshot
// contribute nothing. Currencies are kept separate (no FX conversion): a
// mixed-currency household sees subtotals, a single-currency one sees a single
// figure. FX-converted net worth lives on the dashboard (ADR-0002).

export type CurrencyTotal = { currency: string; amount: number };

export type TotalsResult = {
  totals: CurrencyTotal[]; // largest first; currency code breaks ties
  count: number; // active positions that contributed a balance
};

type TotalsInput = {
  status: string;
  snapshot: { amount: string; currency: string } | null;
};

export function activeCurrencyTotals(items: TotalsInput[]): TotalsResult {
  const byCurrency = new Map<string, number>();
  let count = 0;
  for (const it of items) {
    if (it.status !== "active" || !it.snapshot) continue;
    count++;
    const prev = byCurrency.get(it.snapshot.currency) ?? 0;
    byCurrency.set(it.snapshot.currency, prev + Number(it.snapshot.amount));
  }
  const totals = [...byCurrency.entries()]
    .map(([currency, amount]) => ({ currency, amount }))
    .sort((a, b) => b.amount - a.amount || a.currency.localeCompare(b.currency));
  return { totals, count };
}
