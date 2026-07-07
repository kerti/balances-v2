import { describe, it, expect } from "vitest";
import { buildReportPdfData } from "@/lib/pdf/reportPdfData";
import type { FxRate, MonthlyReport } from "@/api/types";

// Fixtures carry only the fields buildReportPdfData actually reads — cast
// past the full wire type, mirroring lib/fx.test.ts's fixture style.
const rate = (currency: string, yearMonth: string, r: string): FxRate =>
  ({ currency, year_month: `${yearMonth}-01T00:00:00Z`, rate: r }) as FxRate;

const report = (overrides: Partial<MonthlyReport> = {}): MonthlyReport =>
  ({
    year_month: "2026-06-01T00:00:00Z",
    generated_at: "2026-06-30T00:00:00Z",
    reporting_currency: "IDR",
    nw_total: "100000000",
    nw_assets: "60000000",
    nw_liabilities: "10000000",
    nw_receivables: "5000000",
    nw_investments: "45000000",
    earned_income_total: "8000000",
    investment_return_total: "500000",
    asset_value_change: "0",
    derived_living_expenses: "3000000",
    user_breakdowns: {},
    stale_positions: [],
    fx_rates_used: {},
    missing_fx: [],
    ...overrides,
  }) as MonthlyReport;

describe("buildReportPdfData", () => {
  it("omits the secondary currency when none is selected", () => {
    const selected = report();
    const data = buildReportPdfData({
      reports: [selected],
      selected,
      currency: "IDR",
      secondaryCurrency: "",
      rates: [],
    });
    expect(data.headline.secondary).toBeNull();
  });

  it("projects the headline total into a resolvable secondary currency", () => {
    const selected = report({ year_month: "2026-06-01T00:00:00Z", nw_total: "160000000" });
    const data = buildReportPdfData({
      reports: [selected],
      selected,
      currency: "IDR",
      secondaryCurrency: "USD",
      rates: [rate("USD", "2026-06", "16000")],
    });
    expect(data.headline.secondary).toEqual({
      currency: "USD",
      amount: 10000,
      rateMonth: "2026-06-01T00:00:00Z",
    });
  });

  it("omits the secondary currency without throwing when its rate is unresolvable", () => {
    const selected = report({ year_month: "2026-06-01T00:00:00Z" });
    expect(() =>
      buildReportPdfData({
        reports: [selected],
        selected,
        currency: "IDR",
        secondaryCurrency: "EUR", // no EUR rate at all — stale/deleted selection
        rates: [rate("USD", "2026-06", "16000")],
      }),
    ).not.toThrow();
    const data = buildReportPdfData({
      reports: [selected],
      selected,
      currency: "IDR",
      secondaryCurrency: "EUR",
      rates: [rate("USD", "2026-06", "16000")],
    });
    expect(data.headline.secondary).toBeNull();
  });
});

describe("buildReportPdfData — income statement", () => {
  it("suppresses the income statement on the first-month baseline", () => {
    const selected = report({ derived_living_expenses: null });
    const data = buildReportPdfData({
      reports: [selected],
      selected,
      currency: "IDR",
      secondaryCurrency: "",
      rates: [],
    });
    expect(data.incomeStatement).toBeNull();
  });

  it("computes the income statement lines on a non-baseline month", () => {
    const selected = report({
      earned_income_total: "8000000",
      investment_return_total: "500000",
      asset_value_change: "-200000",
      derived_living_expenses: "3000000",
    });
    const data = buildReportPdfData({
      reports: [selected],
      selected,
      currency: "IDR",
      secondaryCurrency: "",
      rates: [],
    });
    expect(data.incomeStatement).toEqual({
      earned: 8000000,
      investmentReturn: 500000,
      assetValueChange: -200000,
      livingExpenses: 3000000,
      netWorthChange: 5300000, // 8000000 + 500000 + (-200000) - 3000000
    });
  });

  it("treats independently-null earned/investment/asset-value fields as 0 on a non-baseline month", () => {
    // The wire type allows these to be null independently of
    // derived_living_expenses, even though ADR-0006 ties them together in
    // practice (only the baseline month suppresses them as a group).
    const selected = report({
      earned_income_total: null,
      investment_return_total: null,
      asset_value_change: null,
      derived_living_expenses: "3000000",
    });
    const data = buildReportPdfData({
      reports: [selected],
      selected,
      currency: "IDR",
      secondaryCurrency: "",
      rates: [],
    });
    expect(data.incomeStatement).toEqual({
      earned: 0,
      investmentReturn: 0,
      assetValueChange: 0,
      livingExpenses: 3000000,
      netWorthChange: -3000000,
    });
  });
});

describe("buildReportPdfData — byPerson", () => {
  it("sorts entries descending by net worth", () => {
    const selected = report({
      user_breakdowns: {
        "user-a": { nw: "10000000", earned_income: "0", investment_return: "0" },
        joint: { nw: "50000000", earned_income: "0", investment_return: "0" },
        "user-b": { nw: "25000000", earned_income: "0", investment_return: "0" },
      },
    });
    const data = buildReportPdfData({
      reports: [selected],
      selected,
      currency: "IDR",
      secondaryCurrency: "",
      rates: [],
    });
    expect(data.byPerson.map((p) => p.key)).toEqual(["joint", "user-b", "user-a"]);
  });
});

describe("buildReportPdfData — fxRatesUsed", () => {
  it("sorts fx rates used alphabetically by currency", () => {
    const selected = report({ fx_rates_used: { USD: "16000", SGD: "12000" } });
    const data = buildReportPdfData({
      reports: [selected],
      selected,
      currency: "IDR",
      secondaryCurrency: "",
      rates: [],
    });
    expect(data.fxRatesUsed).toEqual([
      { currency: "SGD", rate: "12000" },
      { currency: "USD", rate: "16000" },
    ]);
  });

  it("is empty when no fx rates were used this month", () => {
    const selected = report({ fx_rates_used: {} });
    const data = buildReportPdfData({
      reports: [selected],
      selected,
      currency: "IDR",
      secondaryCurrency: "",
      rates: [],
    });
    expect(data.fxRatesUsed).toEqual([]);
  });
});

describe("buildReportPdfData — groupBreakdown", () => {
  it("always returns 4 fixed rows in fixed order, liabilities flagged negative", () => {
    const selected = report({
      nw_assets: "60000000",
      nw_investments: "45000000",
      nw_receivables: "5000000",
      nw_liabilities: "10000000",
    });
    const data = buildReportPdfData({
      reports: [selected],
      selected,
      currency: "IDR",
      secondaryCurrency: "",
      rates: [],
    });
    expect(data.groupBreakdown).toEqual([
      { labelKey: "assets", value: 60000000, negative: false },
      { labelKey: "investments", value: 45000000, negative: false },
      { labelKey: "receivables", value: 5000000, negative: false },
      { labelKey: "liabilities", value: 10000000, negative: true },
    ]);
  });
});

describe("buildReportPdfData — series", () => {
  it("maps reports to year_month/amount pairs, preserving order", () => {
    const r1 = report({ year_month: "2026-04-01T00:00:00Z", nw_total: "90000000" });
    const r2 = report({ year_month: "2026-05-01T00:00:00Z", nw_total: "95000000" });
    const r3 = report({ year_month: "2026-06-01T00:00:00Z", nw_total: "100000000" });
    const data = buildReportPdfData({
      reports: [r1, r2, r3],
      selected: r3,
      currency: "IDR",
      secondaryCurrency: "",
      rates: [],
    });
    expect(data.series).toEqual([
      { year_month: "2026-04-01T00:00:00Z", amount: "90000000" },
      { year_month: "2026-05-01T00:00:00Z", amount: "95000000" },
      { year_month: "2026-06-01T00:00:00Z", amount: "100000000" },
    ]);
  });
});

describe("buildReportPdfData — top-level identity fields", () => {
  it("carries the selected month and reporting currency through for the render layer", () => {
    const selected = report({ year_month: "2026-06-01T00:00:00Z" });
    const data = buildReportPdfData({
      reports: [selected],
      selected,
      currency: "IDR",
      secondaryCurrency: "",
      rates: [],
    });
    expect(data.yearMonth).toBe("2026-06-01T00:00:00Z");
    expect(data.currency).toBe("IDR");
  });
});
