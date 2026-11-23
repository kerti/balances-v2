import type { InvestmentTransaction } from "@/api/types";

// A bond has no placement column: its term opens at the date of its earliest live
// Buy and closes at its maturity (#574, INV-BONDS-05). The server owns the rule
// and refuses what breaks it; this is the client's mirror, used only to bound the
// date inputs so the household member is stopped in the form.

// earliestBuyDate is the "YYYY-MM-DD" of the earliest Buy in the ledger, or
// undefined when there is none (a secondary-market bond before its first Buy has
// no term start yet, so its maturity input gets no lower bound).
export function earliestBuyDate(
  transactions: InvestmentTransaction[] | undefined,
): string | undefined {
  let first: string | undefined;
  for (const t of transactions ?? []) {
    if (t.transaction_type !== "buy") continue;
    const day = t.transaction_date.slice(0, 10);
    if (first === undefined || day < first) first = day;
  }
  return first;
}
