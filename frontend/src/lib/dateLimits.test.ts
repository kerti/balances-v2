import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { carryoverSeedDate, thisYearMonth, todayDate } from './dateLimits'

describe('dateLimits', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    // Pin local time to 2026-05-30 09:15:00. Asserting against fixed strings
    // would be timezone-sensitive otherwise.
    vi.setSystemTime(new Date(2026, 4, 30, 9, 15, 0))
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('thisYearMonth returns YYYY-MM in local time', () => {
    expect(thisYearMonth()).toBe('2026-05')
  })

  it('todayDate returns YYYY-MM-DD in local time', () => {
    expect(todayDate()).toBe('2026-05-30')
  })

  it('zero-pads single-digit months and days', () => {
    vi.setSystemTime(new Date(2027, 0, 3, 12, 0, 0))
    expect(thisYearMonth()).toBe('2027-01')
    expect(todayDate()).toBe('2027-01-03')
  })

  describe('carryoverSeedDate', () => {
    // Clock is pinned to 2026-05-30 (May) by the outer beforeEach.

    it("today mode returns today's local date", () => {
      expect(carryoverSeedDate('today', '2026-04')).toBe('2026-05-30')
    })

    it('end_of_last_month returns the last day of the previous month', () => {
      expect(carryoverSeedDate('end_of_last_month')).toBe('2026-04-30')
    })

    it('end_of_last_month crosses the year boundary in January', () => {
      vi.setSystemTime(new Date(2026, 0, 10, 9, 0, 0)) // 2026-01-10
      expect(carryoverSeedDate('end_of_last_month')).toBe('2025-12-31')
    })

    it('end_of_month_after_last_snapshot returns the end of the next month', () => {
      // Snapshot in March → end of April; April is past, so not clamped.
      expect(carryoverSeedDate('end_of_month_after_last_snapshot', '2026-03')).toBe(
        '2026-04-30',
      )
    })

    it('accepts a full YYYY-MM-DD snapshot date', () => {
      expect(
        carryoverSeedDate('end_of_month_after_last_snapshot', '2026-02-14'),
      ).toBe('2026-03-31')
    })

    it('clamps a future seed to today', () => {
      // Snapshot in May → end of June (future); clamps to today (2026-05-30).
      expect(carryoverSeedDate('end_of_month_after_last_snapshot', '2026-05')).toBe(
        '2026-05-30',
      )
    })

    it('falls back to today when the snapshot month is missing', () => {
      expect(carryoverSeedDate('end_of_month_after_last_snapshot')).toBe(
        '2026-05-30',
      )
      expect(carryoverSeedDate('end_of_month_after_last_snapshot', null)).toBe(
        '2026-05-30',
      )
    })
  })
})
