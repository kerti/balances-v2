// Position lifecycle (ADR-0009). Status enums differ per group; the backend
// (repo validatePositionLifecycle + DB CHECK) is the source of truth, this
// mirror drives the terminate dialog's dropdown and the status badge label.
//
// Labels are resolved through i18next (ADR-0026): callers re-render via a
// useTranslation hook when the locale changes, so the helpers below stay
// pure functions but pick up the live translation each render.
import i18n from "@/i18n";

export type LifecycleGroup = "assets" | "liabilities" | "receivables" | "investments";

export type StatusOption = { value: string; label: string };

// Per-group ordered status values. Source of truth for the dropdown order
// in TerminatePositionDialog and the value-set for statusLabel lookup.
export const STATUS_VALUES: Record<LifecycleGroup, string[]> = {
  assets: ["active", "closed", "sold", "disposed"],
  liabilities: ["active", "paid_off", "forgiven", "written_off"],
  receivables: ["active", "collected", "written_off"],
  investments: ["active", "sold", "matured"],
};

export function statusLabel(group: LifecycleGroup, status: string): string {
  return i18n.t(`common:lifecycle.${group}.${status}`, {
    defaultValue: status,
  });
}

export type InvestmentSubtype = "stock" | "mutual_fund" | "gold" | "bond" | "time_deposit";

// Which terminal statuses each Investment subtype can actually settle, derived
// from the backend's own subtype→transaction matrix
// (validateInvestmentTransactionType): a Sell settles the equity-shaped
// subtypes, a Maturity the deposit-shaped ones, and a Bond takes either.
//
// The group-level enum above offers `sold` and `matured` to all five, which
// leaves combinations no Transaction can express — a matured Stock, a sold
// TimeDeposit. Narrowing the dropdown to this table is what makes the terminate
// dialog's settlement capture total: every status it offers has a Transaction
// that can record where the money went (ADR-0052 §6).
const INVESTMENT_TERMINAL_STATUSES: Record<InvestmentSubtype, string[]> = {
  stock: ["sold"],
  mutual_fund: ["sold"],
  gold: ["sold"],
  bond: ["sold", "matured"],
  time_deposit: ["matured"],
};

// The transaction shape that settles a termination, or null when the pair has
// none. Drives which fields the terminate dialog's settlement block renders.
export function settlementKind(
  subtype: InvestmentSubtype,
  status: string,
): "sell" | "maturity" | null {
  if (!INVESTMENT_TERMINAL_STATUSES[subtype].includes(status)) return null;
  return status === "matured" ? "maturity" : "sell";
}

// `subtype` narrows an investment's terminal statuses per the table above.
// `currentStatus` is always kept in the list even when the narrowing drops it —
// a position may already sit on a status its subtype no longer offers (recorded
// before the narrowing, or written by import/restore), and a dropdown that
// silently blanks out its own current value would read as data loss.
export function statusOptions(
  group: LifecycleGroup,
  subtype?: InvestmentSubtype,
  currentStatus?: string,
): StatusOption[] {
  let values = STATUS_VALUES[group];
  if (group === "investments" && subtype) {
    const settleable = INVESTMENT_TERMINAL_STATUSES[subtype];
    values = values.filter((v) => v === "active" || settleable.includes(v));
  }
  if (currentStatus && !values.includes(currentStatus)) {
    values = [...values, currentStatus];
  }
  return values.map((value) => ({
    value,
    label: statusLabel(group, value),
  }));
}

export function isActiveStatus(status: string): boolean {
  return status === "active";
}
