import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { thisYearMonth, todayDate, monthStartDate, monthEndDateCapped } from "./dateLimits";

// covers: INV-PRESENTATION-02
describe("dateLimits", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    // Pin local time to 2026-05-30 09:15:00. Asserting against fixed strings
    // would be timezone-sensitive otherwise.
    vi.setSystemTime(new Date(2026, 4, 30, 9, 15, 0));
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("thisYearMonth returns YYYY-MM in local time", () => {
    expect(thisYearMonth()).toBe("2026-05");
  });

  it("todayDate returns YYYY-MM-DD in local time", () => {
    expect(todayDate()).toBe("2026-05-30");
  });

  it("zero-pads single-digit months and days", () => {
    vi.setSystemTime(new Date(2027, 0, 3, 12, 0, 0));
    expect(thisYearMonth()).toBe("2027-01");
    expect(todayDate()).toBe("2027-01-03");
  });

  it("monthStartDate returns the first of the month", () => {
    expect(monthStartDate("2026-03")).toBe("2026-03-01");
    expect(monthStartDate("2026-12")).toBe("2026-12-01");
  });

  it("monthEndDateCapped returns the last day for a past month", () => {
    // System time pinned to 2026-05-30; these months are fully in the past.
    expect(monthEndDateCapped("2026-02")).toBe("2026-02-28"); // non-leap
    expect(monthEndDateCapped("2026-04")).toBe("2026-04-30"); // 30-day month
    expect(monthEndDateCapped("2024-02")).toBe("2024-02-29"); // leap year
  });

  it("monthEndDateCapped caps the current month at today", () => {
    // May has 31 days, but today is the 30th — the cap wins.
    expect(monthEndDateCapped("2026-05")).toBe("2026-05-30");
  });
});
