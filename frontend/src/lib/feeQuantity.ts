// Fee cash→quantity helper (Q12). A unit-settled fee removes units from the
// position at a conversion price, so the three values are dependent:
//
//     cash_amount = quantity_deducted × price_per_unit
//
// The user always knows the cash amount (it's what the manager charged) and the
// conversion price (the spot/buyback price the units were valued at). This
// derives the third — the units deducted — so the non-technical owner never
// does the division by hand. Pure Number arithmetic, matching `lib/revaluation`
// and `lib/costBasis`; the result is a display suggestion the user can override,
// not authoritative — the backend stores whatever the form submits.
//
// quantity is DECIMAL(20,8) on the wire, so the result rounds to 8dp with
// trailing zeros trimmed (so "0.05000000" shows as "0.05").

export function deriveFeeQuantity(
  amount: string,
  pricePerUnit: string,
): string | null {
  if (!amount || !pricePerUnit) return null;
  const a = Number(amount);
  const p = Number(pricePerUnit);
  if (!Number.isFinite(a) || !Number.isFinite(p)) return null;
  if (a <= 0 || p <= 0) return null;
  const q = a / p;
  // Round to 8dp (DECIMAL(20,8)); `toString()` drops the trailing zeros the
  // round introduces.
  return (Math.round(q * 1e8) / 1e8).toString();
}
