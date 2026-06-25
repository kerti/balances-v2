import { describe, expect, it } from "vitest";
import { suggestRevalued } from "./revaluation";

describe("suggestRevalued", () => {
  it("grows the value for a positive (appreciation) rate", () => {
    // 20,000,000 × 1.05^(6/12) ≈ 20,493,901.5319…
    const got = suggestRevalued({
      newYearMonth: "2026-02",
      annualRatePct: "5",
      snapshots: [{ year_month: "2025-08", amount: "20000000" }],
    });
    expect(got).not.toBeNull();
    expect(got!.amount).toBe("20493901.5319");
    expect(got!.anchorAmount).toBe("20000000");
    expect(got!.anchorYearMonth).toBe("2025-08");
    expect(got!.monthsElapsed).toBe(6);
    expect(got!.annualRatePct).toBe(5);
  });

  it("declines the value for a negative rate (vehicle, HGB lease)", () => {
    // 20,000,000 × 0.95^(6/12) ≈ 19,493,588.6896…
    const got = suggestRevalued({
      newYearMonth: "2026-02",
      annualRatePct: "-5",
      snapshots: [{ year_month: "2025-08", amount: "20000000" }],
    });
    expect(got!.amount).toBe("19493588.6896");
    expect(got!.annualRatePct).toBe(-5);
  });

  it("handles a full year of appreciation exactly", () => {
    const got = suggestRevalued({
      newYearMonth: "2026-08",
      annualRatePct: "10",
      snapshots: [{ year_month: "2025-08", amount: "1000" }],
    });
    expect(got!.amount).toBe("1100");
    expect(got!.monthsElapsed).toBe(12);
  });

  it("compounds appreciation across multiple years", () => {
    // 1000 × 1.1^2 = 1210
    const got = suggestRevalued({
      newYearMonth: "2027-08",
      annualRatePct: "10",
      snapshots: [{ year_month: "2025-08", amount: "1000" }],
    });
    expect(got!.amount).toBe("1210");
    expect(got!.monthsElapsed).toBe(24);
  });

  it("picks the latest snapshot strictly before the target month", () => {
    const got = suggestRevalued({
      newYearMonth: "2026-06",
      annualRatePct: "10",
      snapshots: [
        { year_month: "2025-01", amount: "1000" },
        { year_month: "2026-01", amount: "900" },
        { year_month: "2025-08", amount: "950" },
      ],
    });
    expect(got!.anchorYearMonth).toBe("2026-01");
    expect(got!.monthsElapsed).toBe(5);
  });

  it("accepts the API ISO-datetime year_month shape", () => {
    const got = suggestRevalued({
      newYearMonth: "2026-02",
      annualRatePct: "5",
      snapshots: [{ year_month: "2025-08-01T00:00:00Z", amount: "20000000" }],
    });
    expect(got!.anchorYearMonth).toBe("2025-08");
    expect(got!.monthsElapsed).toBe(6);
  });

  it("returns null when the target month equals the latest anchor", () => {
    // Same-month re-entry: no drift to apply, no suggestion.
    expect(
      suggestRevalued({
        newYearMonth: "2025-08",
        annualRatePct: "5",
        snapshots: [{ year_month: "2025-08", amount: "20000000" }],
      }),
    ).toBeNull();
  });

  it("returns null when the target month is before every snapshot", () => {
    expect(
      suggestRevalued({
        newYearMonth: "2025-01",
        annualRatePct: "5",
        snapshots: [{ year_month: "2025-08", amount: "20000000" }],
      }),
    ).toBeNull();
  });

  it("returns null when no rate is set on the position", () => {
    expect(
      suggestRevalued({
        newYearMonth: "2026-02",
        annualRatePct: null,
        snapshots: [{ year_month: "2025-08", amount: "20000000" }],
      }),
    ).toBeNull();
  });

  it("returns null when rate is exactly zero (flat, no drift)", () => {
    expect(
      suggestRevalued({
        newYearMonth: "2026-02",
        annualRatePct: "0",
        snapshots: [{ year_month: "2025-08", amount: "20000000" }],
      }),
    ).toBeNull();
  });

  it("returns null when the snapshot list is empty or undefined", () => {
    expect(
      suggestRevalued({
        newYearMonth: "2026-02",
        annualRatePct: "5",
        snapshots: [],
      }),
    ).toBeNull();
    expect(
      suggestRevalued({
        newYearMonth: "2026-02",
        annualRatePct: "5",
        snapshots: undefined,
      }),
    ).toBeNull();
  });

  it("returns null on malformed month strings", () => {
    expect(
      suggestRevalued({
        newYearMonth: "not-a-month",
        annualRatePct: "5",
        snapshots: [{ year_month: "2025-08", amount: "20000000" }],
      }),
    ).toBeNull();
  });

  it("returns null on a well-formed but out-of-range month (mo > 12)", () => {
    expect(
      suggestRevalued({
        newYearMonth: "2025-13",
        annualRatePct: "5",
        snapshots: [{ year_month: "2025-08", amount: "20000000" }],
      }),
    ).toBeNull();
  });

  it("returns null when the anchor snapshot value is zero or negative", () => {
    expect(
      suggestRevalued({
        newYearMonth: "2026-02",
        annualRatePct: "5",
        snapshots: [{ year_month: "2025-08", amount: "0" }],
      }),
    ).toBeNull();
  });
});
