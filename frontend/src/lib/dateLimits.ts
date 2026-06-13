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
  const d = new Date()
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}`
}

// todayDate returns the current local date as "YYYY-MM-DD", suitable for
// <input type="date" max=…>.
export function todayDate(): string {
  const d = new Date()
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
}

// SUPPORTED_CARRYOVER_DATE_MODES mirrors the users.carryover_date_mode CHECK
// (migration 00026) and the handler's supportedCarryoverDateModes map. It is
// the per-user preference (issue #105) for the as-of date the carryover dialog
// pre-fills. Add a mode by extending all three.
export const SUPPORTED_CARRYOVER_DATE_MODES = [
  'today',
  'end_of_last_month',
  'end_of_month_after_last_snapshot',
] as const
export type CarryoverDateMode = (typeof SUPPORTED_CARRYOVER_DATE_MODES)[number]

export function isSupportedCarryoverDateMode(
  value: string,
): value is CarryoverDateMode {
  return (SUPPORTED_CARRYOVER_DATE_MODES as readonly string[]).includes(value)
}

// localDate formats a local Date as "YYYY-MM-DD". new Date(y, m, d) normalises
// out-of-range month/day, so callers can pass e.g. day 0 (= last day of the
// previous month) and month -1 (= December of the previous year).
function localDate(d: Date): string {
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
}

// carryoverSeedDate computes the as-of date the carryover dialog pre-fills, per
// the user's preference (issue #105):
//   today                            → today's local date (the historical default)
//   end_of_last_month                → last day of the month before this one
//   end_of_month_after_last_snapshot → last day of the month after the latest
//                                       snapshot's period (lastSnapshotMonth)
//
// lastSnapshotMonth is the latest snapshot's year_month ("YYYY-MM" or
// "YYYY-MM-DD"); only the third mode reads it, falling back to today when it is
// missing or unparseable. The result is clamped to today because as-of dates
// may not be in the future (the backend rejects them and the date input caps at
// today) — a clamped seed stays valid and the user can still edit it down.
export function carryoverSeedDate(
  mode: CarryoverDateMode,
  lastSnapshotMonth?: string | null,
): string {
  const today = todayDate()
  let seed: string
  switch (mode) {
    case 'end_of_last_month': {
      const now = new Date()
      // Day 0 of the current month = last day of the previous month.
      seed = localDate(new Date(now.getFullYear(), now.getMonth(), 0))
      break
    }
    case 'end_of_month_after_last_snapshot': {
      const [year, month] = (lastSnapshotMonth ?? '').split('-').map(Number)
      if (!year || !month) return today
      // month is 1-based; day 0 of (month + 1) = last day of the month *after*
      // the snapshot's month (e.g. snapshot 2026-05 → 2026-06-30).
      seed = localDate(new Date(year, month + 1, 0))
      break
    }
    case 'today':
    default:
      seed = today
  }
  return seed > today ? today : seed
}
