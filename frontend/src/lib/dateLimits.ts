// Helpers for the `max` attribute on snapshot/transaction date and month
// inputs. A snapshot is by definition a past observation; a transaction
// records something that already happened. Pairing the input `max` with the
// backend's future-date validation (5 snapshot + 1 transaction create/update
// route each) keeps obvious nonsense (year 2099 typos) out of the form
// before it reaches the API.

// thisYearMonth returns the current local month as "YYYY-MM", suitable for
// <input type="month" max=…>. Local time matches what the user sees in the
// picker; the backend allows the corresponding UTC month, which is at most
// one calendar day off — close enough that the picker constraint never
// surprises the user.
export function thisYearMonth(): string {
  const d = new Date();
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}`;
}

// yearMonthsUpToNow lists every "YYYY-MM" from `fromYear`-01 through the current
// month — the `months` a MonthPickerPopover needs to act as a form field with
// the same ceiling the native `max={thisYearMonth()}` gave.
export function yearMonthsUpToNow(fromYear = 1990): string[] {
  const [endYear, endMonth] = thisYearMonth().split("-").map(Number);
  const out: string[] = [];
  for (let y = fromYear; y <= endYear; y++) {
    const last = y === endYear ? endMonth : 12;
    for (let m = 1; m <= last; m++) out.push(`${y}-${String(m).padStart(2, "0")}`);
  }
  return out;
}

// pastYearMonthOrNull returns `value` when it is a well-formed "YYYY-MM" that is
// not after the current month, else null — for seeding a month input from a URL
// param without trusting it.
export function pastYearMonthOrNull(value: string | null): string | null {
  if (value === null || !/^\d{4}-(0[1-9]|1[0-2])$/.test(value)) return null;
  return value <= thisYearMonth() ? value : null;
}

// todayDate returns the current local date as "YYYY-MM-DD", suitable for
// <input type="date" max=…>.
export function todayDate(): string {
  const d = new Date();
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
}

// SUPPORTED_CARRYOVER_DATE_MODES mirrors the users.carryover_date_mode CHECK
// (migration 00002) and the handler's supportedCarryoverDateModes map. It is
// the per-user preference (issue #105) that Copy carryover used to seed its
// as-of date from. Record snapshot (#788) never pre-fills a statement date, so
// nothing reads the preference now; it stays only until ADR-0046's
// `carryover_date_mode` drop removes the column, the API field and the Settings
// row together. Add a mode by extending all three.
export const SUPPORTED_CARRYOVER_DATE_MODES = [
  "today",
  "end_of_last_month",
  "end_of_month_after_last_snapshot",
] as const;
export type CarryoverDateMode = (typeof SUPPORTED_CARRYOVER_DATE_MODES)[number];

export function isSupportedCarryoverDateMode(value: string): value is CarryoverDateMode {
  return (SUPPORTED_CARRYOVER_DATE_MODES as readonly string[]).includes(value);
}

// monthStartDate returns the first day of a "YYYY-MM" month as "YYYY-MM-DD",
// suitable for the `min` of a snapshot's statement-date input — the statement
// date must fall within the snapshot's month (backend CHECK
// <table>_as_of_in_month, migration 00003).
export function monthStartDate(yearMonth: string): string {
  return `${yearMonth}-01`;
}

// monthEndDateCapped returns the last day of a "YYYY-MM" month as "YYYY-MM-DD",
// but never later than today — for the current month the cap is today, since a
// snapshot is a past observation. Suitable for the `max` of the statement-date
// input. Both halves are YYYY-MM-DD, so the lexicographic min is also the
// chronological min.
export function monthEndDateCapped(yearMonth: string): string {
  const [year, month] = yearMonth.split("-").map(Number);
  // Day 0 of the next 1-based month index is the last day of `month`.
  const lastDay = new Date(year, month, 0).getDate();
  const endOfMonth = `${yearMonth}-${String(lastDay).padStart(2, "0")}`;
  const today = todayDate();
  return endOfMonth < today ? endOfMonth : today;
}
