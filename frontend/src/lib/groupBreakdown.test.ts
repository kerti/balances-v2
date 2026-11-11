// covers: INV-FINANCE-39
import { describe, it, expect } from "vitest";
import {
  breakdownCategorySeries,
  breakdownCategorySlices,
  breakdownRiskSlices,
  breakdownValueSeries,
  categoriesHeld,
  latestMonth,
  slicesHeld,
} from "@/lib/groupBreakdown";
import type { GroupBreakdown } from "@/api/types";

const b: GroupBreakdown = {
  group: "investment",
  reporting_currency: "IDR",
  months: [
    {
      year_month: "2026-01-01T00:00:00Z",
      total: "1000",
      by_subtype: { stock: "1000" },
      by_risk: { low: "0", medium: "0", high: "1000" },
      missing_fx: [],
    },
    {
      year_month: "2026-02-01T00:00:00Z",
      total: "1500.5",
      by_subtype: { stock: "1000", mutual_fund: "500.5" },
      by_risk: { low: "500.5", medium: "0", high: "1000" },
      missing_fx: [],
    },
  ],
};

const KEYS = ["stock", "mutualFund", "bond"] as const;

describe("groupBreakdown", () => {
  it("keys months as bare YYYY-MM and carries the converted total", () => {
    expect(breakdownValueSeries(b)).toEqual([
      { year_month: "2026-01", value: 1000 },
      { year_month: "2026-02", value: 1500.5 },
    ]);
    expect(latestMonth(b)?.total).toBe("1500.5");
    expect(latestMonth(undefined)).toBeNull();
  });

  it("re-keys snake_case subtypes to the hubs' camelCase categories, zero-filled", () => {
    const series = breakdownCategorySeries(b, KEYS);
    expect(series[1].byCategory).toEqual({ stock: 1000, mutualFund: 500.5, bond: 0 });
    expect(series[0].byCategory).toEqual({ stock: 1000, mutualFund: 0, bond: 0 });
  });

  it("takes the pies from the latest month only", () => {
    expect(breakdownCategorySlices(b, KEYS)).toEqual([
      { category: "stock", value: 1000 },
      { category: "mutualFund", value: 500.5 },
      { category: "bond", value: 0 },
    ]);
    expect(breakdownRiskSlices(b, ["low", "medium", "high"] as const)).toEqual([
      { profile: "low", value: 500.5 },
      { profile: "medium", value: 0 },
      { profile: "high", value: 1000 },
    ]);
  });

  it("counts a category as held only when it has value, so one-category charts can be left out", () => {
    const one = breakdownCategorySeries({ ...b, months: [b.months[0]] }, KEYS);
    expect(categoriesHeld(one)).toBe(1);
    expect(categoriesHeld(breakdownCategorySeries(b, KEYS))).toBe(2);
    expect(slicesHeld(breakdownCategorySlices(b, KEYS))).toBe(2);
    expect(slicesHeld([{ value: 0 }, { value: 0 }])).toBe(0);
  });
});
