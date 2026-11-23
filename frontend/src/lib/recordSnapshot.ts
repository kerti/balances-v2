// Pure helpers behind the "Record snapshot" dialogs (ADR-0046 amendment, #788).
// A snapshot's `year_month` arrives as "YYYY-MM-01T00:00:00Z" from the API and
// as bare "YYYY-MM" from the month picker, so every comparison goes through
// `ymOf`, which keeps just the leading year and month.
import { thisYearMonth } from "@/lib/dateLimits";
import { formatShortMonth, formatShortYearMonth } from "@/lib/format";

type Dated = { year_month: string };

export function ymOf(yearMonth: string): string {
  return yearMonth.slice(0, 7);
}

function nextMonth(ym: string): string {
  const [y, m] = ym.split("-").map(Number);
  return m === 12 ? `${y + 1}-01` : `${y}-${String(m + 1).padStart(2, "0")}`;
}

function prevMonth(ym: string): string {
  const [y, m] = ym.split("-").map(Number);
  return m === 1 ? `${y - 1}-12` : `${y}-${String(m - 1).padStart(2, "0")}`;
}

// The month after the latest snapshot, never later than the current month (a
// snapshot is a past observation); the current month with no snapshots yet.
// This is the month a trade's "Update {Mon} holding" shortcut targets (#830):
// the trade is happening now, so it never scans back for an older gap.
export function holdingMonth(snapshots: Dated[] | undefined, now = thisYearMonth()): string {
  let latest = "";
  for (const s of snapshots ?? []) {
    const ym = ymOf(s.year_month);
    if (ym > latest) latest = ym;
  }
  if (!latest) return now;
  const next = nextMonth(latest);
  return next > now ? now : next;
}

// The month the header's Record snapshot opens on (#953 S3): the first month
// with no value. That is the month after the latest snapshot while it is still
// in the past; once the current month is recorded, the most recent earlier gap
// back to the first snapshot, so the dialog does not open on a replace by
// default. Only a history with no gap at all falls back to the current month,
// which then opens in replace mode.
export function defaultRecordMonth(snapshots: Dated[] | undefined, now = thisYearMonth()): string {
  const next = holdingMonth(snapshots, now);
  if (!snapshotForMonth(snapshots, next)) return next;
  let earliest = next;
  for (const s of snapshots ?? []) {
    const ym = ymOf(s.year_month);
    if (ym < earliest) earliest = ym;
  }
  for (let ym = prevMonth(next); ym > earliest; ym = prevMonth(ym)) {
    if (!snapshotForMonth(snapshots, ym)) return ym;
  }
  return next;
}

// The live snapshot for a month, if any.
export function snapshotForMonth<S extends Dated>(
  snapshots: S[] | undefined,
  yearMonth: string,
): S | undefined {
  const ym = ymOf(yearMonth);
  return snapshots?.find((s) => ymOf(s.year_month) === ym);
}

// The latest snapshot strictly before a month: what "Same as last month" copies.
export function lastRecordedBefore<S extends Dated>(
  snapshots: S[] | undefined,
  yearMonth: string,
): S | undefined {
  const ym = ymOf(yearMonth);
  let best: S | undefined;
  for (const s of snapshots ?? []) {
    const sm = ymOf(s.year_month);
    if (sm < ym && (!best || sm > ymOf(best.year_month))) best = s;
  }
  return best;
}

// "Nov 2026" for the replace notice and the "Last recorded" line. Anchored at
// noon UTC so a local-timezone offset can never roll the first of the month
// into a neighbouring one.
export function formatRecordMonth(yearMonth: string): string {
  return formatShortYearMonth(new Date(`${ymOf(yearMonth)}-01T12:00:00Z`));
}

// A statement date from the API ("YYYY-MM-DDT…") as the date input's value.
export function asOfInputValue(asOf: string | null): string {
  return asOf ? asOf.slice(0, 10) : "";
}

// ---- "Update {Mon} holding" (#830) --------------------------------------------

// The short month name a trade's toast / the mismatch banner offers to update:
// the month Record snapshot will open on ("Nov"). Anchored at noon UTC like
// `formatRecordMonth`.
export function holdingMonthLabel(snapshots: Dated[] | undefined): string {
  const ym = holdingMonth(snapshots);
  return formatShortMonth(new Date(`${ym}-01T12:00:00Z`));
}

type PricedTxn = {
  transaction_type: string;
  transaction_date: string;
  price_per_unit: string | null;
};

// The price the shortcut prefills into Record snapshot for a quantity x price
// position. "Latest price" is the price per unit of the snapshot already in force
// for the month being opened (replace mode) or else the most recent snapshot
// before it: a snapshot's price is a month-end valuation, which a trade's
// execution price is not (gold buys at the dealer's ask but is valued at buyback,
// issue #19). Only a position with no priced snapshot yet falls back to its most
// recent Buy/Sell price. Null means "leave the field empty".
export function latestHoldingPrice(
  snapshots: Array<Dated & { price_per_unit: string | null }> | undefined,
  transactions: PricedTxn[] | undefined,
  yearMonth: string,
): string | null {
  const fromSnapshot =
    snapshotForMonth(snapshots, yearMonth)?.price_per_unit ??
    lastRecordedBefore(snapshots, yearMonth)?.price_per_unit;
  if (fromSnapshot) return fromSnapshot;
  let latest: PricedTxn | undefined;
  for (const tx of transactions ?? []) {
    if (tx.transaction_type !== "buy" && tx.transaction_type !== "sell") continue;
    if (!tx.price_per_unit) continue;
    if (!latest || tx.transaction_date >= latest.transaction_date) latest = tx;
  }
  return latest?.price_per_unit ?? null;
}

// Radix `onOpenAutoFocus` for the Record snapshot dialogs: land on the form's
// first value input (marked `data-first-value`) instead of the month picker's
// previous-month arrow, which is the first tabbable element. The picker stays
// reachable with Shift+Tab.
export function focusFirstValue(event: Event) {
  const first = (event.target as HTMLElement).querySelector<HTMLElement>("[data-first-value]");
  if (!first) return;
  event.preventDefault();
  first.focus();
}
