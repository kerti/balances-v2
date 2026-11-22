// Fee cash→quantity helper (Q12). A unit-settled fee removes units from the
// position at a conversion price, so the three values are dependent:
//
//     cash_amount = quantity_deducted × price_per_unit
//
// The user always knows the cash amount (it's what the manager charged) and the
// conversion price (the spot/buyback price the units were valued at). This
// derives the third — the units deducted — so the non-technical owner never
// does the division by hand. The division is exact (`lib/decimal`), so the
// suggestion carries no float artefact; it is still only a suggestion the user can
// override — the backend stores whatever the form submits.
//
// quantity is DECIMAL(20,8) on the wire, so the result rounds half away from zero
// to 8dp with trailing zeros trimmed (so "0.05000000" shows as "0.05").

import { divideDecimals, parseExact } from "@/lib/decimal";

export function deriveFeeQuantity(amount: string, pricePerUnit: string): string | null {
  const a = parseExact(amount);
  const p = parseExact(pricePerUnit);
  if (a === null || p === null || a.n <= 0n || p.n <= 0n) return null;
  return divideDecimals(amount, pricePerUnit);
}
