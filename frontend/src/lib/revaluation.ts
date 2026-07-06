// Revaluation helper for property + vehicle snapshot entry (ADR-0008, Q8a).
// Suggests a new snapshot value from the latest prior snapshot revalued by the
// position's signed annual rate: positive rate grows (property appreciation),
// negative rate declines (depreciation, leasehold amortization). Pure JS
// arithmetic: this is a display suggestion the user can override, not
// authoritative valuation — backend computes nothing from it.
//
// Vehicle's annual_depreciation_rate is stored unsigned (always meaning "loss
// %/yr"), so its callsite negates before passing in.
//
// See `suggestRevalued` for the entry point.

export type RevaluationSuggestion = {
  // 4dp decimal string, sized to drop straight into the snapshot amount input
  // (which accepts any precision the DECIMAL(20,4) column will store).
  amount: string;
  // The snapshot the suggestion derives from — surfaced in the rationale text.
  anchorAmount: string;
  anchorYearMonth: string; // "YYYY-MM"
  monthsElapsed: number;
  annualRatePct: number; // signed: positive = appreciate, negative = decline
};

type SuggestArgs = {
  newYearMonth: string; // "YYYY-MM"
  annualRatePct: string | null;
  // Snapshots in any order; the helper picks the latest one strictly before
  // newYearMonth. year_month here can be either "YYYY-MM" or the API's
  // "YYYY-MM-DDT00:00:00Z" — only the year + month leading digits are read.
  snapshots: { year_month: string; amount: string }[] | undefined;
};

// suggestRevalued returns null whenever no useful suggestion applies: no rate,
// zero rate (no drift to project), no prior snapshot, picked month not strictly
// after the anchor, or any input that doesn't parse. Callers render nothing on
// null.
export function suggestRevalued(args: SuggestArgs): RevaluationSuggestion | null {
  if (!args.annualRatePct) return null;
  const rate = Number(args.annualRatePct);
  if (!Number.isFinite(rate) || rate === 0) return null;

  const target = monthIndex(args.newYearMonth);
  if (target === null) return null;

  // Pick the latest snapshot strictly before the target month. Tracking the
  // running max (rather than sorting + binary-searching) is fine at household
  // scale — a position has tens of snapshots, not thousands.
  let anchor: { ym: string; m: number; amount: string } | null = null;
  for (const s of args.snapshots ?? []) {
    const m = monthIndex(s.year_month);
    if (m === null || m >= target) continue;
    if (anchor === null || m > anchor.m) {
      anchor = { ym: s.year_month.slice(0, 7), m, amount: s.amount };
    }
  }
  if (!anchor) return null;

  const prev = Number(anchor.amount);
  if (!Number.isFinite(prev) || prev <= 0) return null;

  const months = target - anchor.m;
  // Signed-rate compound: (1 + r/100)^(t years). r > 0 grows, r < 0 declines.
  const factor = Math.pow(1 + rate / 100, months / 12);
  const raw = prev * factor;
  // Round to 4dp matching DECIMAL(20,4). `toString()` then drops trailing zeros
  // produced by the round, so the input shows "19493588.6896" not "…6896000…".
  const amount = (Math.round(raw * 10000) / 10000).toString();

  return {
    amount,
    anchorAmount: anchor.amount,
    anchorYearMonth: anchor.ym,
    monthsElapsed: months,
    annualRatePct: rate,
  };
}

// monthIndex turns a "YYYY-MM(-…)" string into an absolute month count from
// year 0 so two months can be subtracted to get the elapsed-month delta.
// Returns null on anything that doesn't start with the expected pattern.
function monthIndex(s: string): number | null {
  const m = /^(\d{4})-(\d{2})/.exec(s);
  if (!m) return null;
  const y = Number(m[1]);
  const mo = Number(m[2]);
  if (mo < 1 || mo > 12) return null;
  return y * 12 + (mo - 1);
}
