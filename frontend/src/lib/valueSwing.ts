import { toScaled } from "@/lib/decimal";
import { formatCurrency } from "@/lib/format";
import { formatRecordMonth } from "@/lib/recordSnapshot";

// "up" when a newly entered value is 10 times the previous one or more, "down"
// when it is a tenth or less; otherwise null (#933). A dropped or extra digit is
// the commonest way to mis-key a month's value, and it moves the figure by
// exactly this much, so a single-entry Record snapshot asks before saving one.
export type Swing = "up" | "down" | null;

// Compared on the wire's string decimals through BigInt (lib/decimal), never a
// float. Both ends are inclusive: a figure that is exactly 10x or exactly a tenth
// is the typo this exists for.
//
// No swing is reported without a basis to compare against: a missing,
// unreadable or zero previous value; a zero new value (closing a loan or
// emptying an account is a real, deliberate entry, not a slipped digit); or a
// change of sign (a different mistake, not a decimal shift). Signed values
// compare by magnitude.
export function valueSwing(prior: string | null | undefined, next: string | null): Swing {
  if (prior == null || next === null) return null;
  const p = toScaled(prior);
  const n = toScaled(next);
  if (p === null || n === null || p === 0n || n === 0n) return null;
  if (p < 0n !== n < 0n) return null;
  const a = p < 0n ? -p : p;
  const b = n < 0n ? -n : n;
  if (b >= a * 10n) return "up";
  if (b * 10n <= a) return "down";
  return null;
}

export type SwingAsk = {
  swing: Exclude<Swing, null>;
  // Already formatted for display: the new value, the last recorded value, and
  // the month that value belongs to.
  value: string;
  prior: string;
  month: string;
};

// The question a single-entry Record snapshot asks before saving `amount`, or
// null when the figure is not a swing from the previous recorded month.
export function swingAsk(
  prior: { amount: string; year_month: string } | undefined,
  amount: string | null,
  currency: string,
): SwingAsk | null {
  const swing = valueSwing(prior?.amount, amount);
  if (swing === null || prior === undefined || amount === null) return null;
  return {
    swing,
    value: formatCurrency(amount, currency),
    prior: formatCurrency(prior.amount, currency),
    month: formatRecordMonth(prior.year_month),
  };
}
