import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import {
  carryoverSeed,
  carryoverSeedDate,
  thisYearMonth,
  todayDate,
  monthStartDate,
  monthEndDateCapped,
} from './dateLimits'
import type { CarryoverDateMode } from './dateLimits'

// covers: INV-PRESENTATION-02
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

  describe('carryoverSeed', () => {
    // Clock is pinned to 2026-05-30 (May) by the outer beforeEach.

    it('derives yearMonth from the seed date (today mode = current month)', () => {
      expect(carryoverSeed('today', '2026-04')).toEqual({
        yearMonth: '2026-05',
        asOfDate: '2026-05-30',
      })
    })

    it('back-dates yearMonth for end_of_last_month', () => {
      expect(carryoverSeed('end_of_last_month')).toEqual({
        yearMonth: '2026-04',
        asOfDate: '2026-04-30',
      })
    })

    it('back-dates yearMonth for end_of_month_after_last_snapshot', () => {
      expect(carryoverSeed('end_of_month_after_last_snapshot', '2026-03')).toEqual({
        yearMonth: '2026-04',
        asOfDate: '2026-04-30',
      })
    })

    it('keeps the pair consistent when the seed is clamped to today', () => {
      // Snapshot in May → end of June (future) clamps to today; yearMonth must
      // follow the clamped date, not the unclamped June.
      expect(carryoverSeed('end_of_month_after_last_snapshot', '2026-05')).toEqual({
        yearMonth: '2026-05',
        asOfDate: '2026-05-30',
      })
    })

    // The whole point of #119: whatever the mode, the seeded { yearMonth,
    // asOfDate } pair must satisfy the date input's own min/max, so the
    // pre-filled form is submittable without the user editing the month.
    it('produces a pair within the month bounds for every mode', () => {
      const modes: CarryoverDateMode[] = [
        'today',
        'end_of_last_month',
        'end_of_month_after_last_snapshot',
      ]
      for (const mode of modes) {
        const { yearMonth, asOfDate } = carryoverSeed(mode, '2026-03')
        expect(asOfDate.slice(0, 7)).toBe(yearMonth)
        expect(asOfDate >= monthStartDate(yearMonth)).toBe(true)
        expect(asOfDate <= monthEndDateCapped(yearMonth)).toBe(true)
      }
    })
  })

  it('monthStartDate returns the first of the month', () => {
    expect(monthStartDate('2026-03')).toBe('2026-03-01')
    expect(monthStartDate('2026-12')).toBe('2026-12-01')
  })

  it('monthEndDateCapped returns the last day for a past month', () => {
    // System time pinned to 2026-05-30; these months are fully in the past.
    expect(monthEndDateCapped('2026-02')).toBe('2026-02-28') // non-leap
    expect(monthEndDateCapped('2026-04')).toBe('2026-04-30') // 30-day month
    expect(monthEndDateCapped('2024-02')).toBe('2024-02-29') // leap year
  })

  it('monthEndDateCapped caps the current month at today', () => {
    // May has 31 days, but today is the 30th — the cap wins.
    expect(monthEndDateCapped('2026-05')).toBe('2026-05-30')
  })
})
