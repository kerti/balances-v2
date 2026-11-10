// Pure helpers behind the "Record snapshot" dialogs (ADR-0046 amendment, #788).
// A snapshot's `year_month` arrives as "YYYY-MM-01T00:00:00Z" from the API and
// as bare "YYYY-MM" from the month picker, so every comparison goes through
// `ymOf`, which keeps just the leading year and month.
import { thisYearMonth } from "@/lib/dateLimits";
import { formatShortYearMonth } from "@/lib/format";

type Dated = { year_month: string };

export function ymOf(yearMonth: string): string {
  return yearMonth.slice(0, 7);
}

function nextMonth(ym: string): string {
  const [y, m] = ym.split("-").map(Number);
  return m === 12 ? `${y + 1}-01` : `${y}-${String(m + 1).padStart(2, "0")}`;
}

// The month Record snapshot opens on: the month after the latest snapshot,
// never later than the current month (a snapshot is a past observation). With
// no snapshots yet, the current month. When the latest snapshot is already in
// the current month this is the current month, which then opens in replace mode.
export function defaultRecordMonth(snapshots: Dated[] | undefined, now = thisYearMonth()): string {
  let latest = "";
  for (const s of snapshots ?? []) {
    const ym = ymOf(s.year_month);
    if (ym > latest) latest = ym;
  }
  if (!latest) return now;
  const next = nextMonth(latest);
  return next > now ? now : next;
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
