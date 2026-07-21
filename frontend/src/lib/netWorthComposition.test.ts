import { describe, expect, it } from "vitest";
import type { MonthlyReport } from "@/api/types";
import { netWorthComposition } from "./netWorthComposition";

// covers: INV-PRESENTATION-07

// Minimal MonthlyReport factory — only the nw_* fields the composition reads.
// Values are whole numbers so the Number-based fold adds exactly (no float
// noise) and the reconciliation identity can assert on ===.
function report(
  year_month: string,
  nw: { assets: number; receivables: number; investments: number; liabilities: number },
): MonthlyReport {
  const total = nw.assets + nw.receivables + nw.investments - nw.liabilities;
  return {
    year_month,
    nw_total: String(total),
    nw_assets: String(nw.assets),
    nw_receivables: String(nw.receivables),
    nw_investments: String(nw.investments),
    nw_liabilities: String(nw.liabilities),
  } as MonthlyReport;
}

const reports: MonthlyReport[] = [
  report("2026-01", { assets: 1000, receivables: 50, investments: 400, liabilities: 200 }),
  report("2026-02", { assets: 1100, receivables: 0, investments: 450, liabilities: 180 }),
];

describe("netWorthComposition", () => {
  it("folds receivables into the assets line", () => {
    const { assets } = netWorthComposition(reports);
    expect(assets).toEqual([
      { year_month: "2026-01", amount: "1050" }, // 1000 + 50
      { year_month: "2026-02", amount: "1100" }, // 1100 + 0
    ]);
  });

  it("passes liabilities through as a positive magnitude (as stored)", () => {
    const { liabilities } = netWorthComposition(reports);
    expect(liabilities).toEqual([
      { year_month: "2026-01", amount: "200" },
      { year_month: "2026-02", amount: "180" },
    ]);
  });

  it("passes investments through at total closing value", () => {
    const { investments } = netWorthComposition(reports);
    expect(investments).toEqual([
      { year_month: "2026-01", amount: "400" },
      { year_month: "2026-02", amount: "450" },
    ]);
  });

  // INV-PRESENTATION-07: the charted composition reconciles to the net-worth
  // line for every month — assets(+receivables folded) + investments −
  // liabilities === nw_total. The fold and positive-magnitude are
  // presentation-only and must not break this identity.
  it("reconciles to the net-worth total every month", () => {
    const { assets, liabilities, investments } = netWorthComposition(reports);
    reports.forEach((r, i) => {
      const reconciled =
        Number(assets[i].amount) + Number(investments[i].amount) - Number(liabilities[i].amount);
      expect(String(reconciled)).toBe(r.nw_total);
    });
  });

  it("returns empty series for empty input", () => {
    expect(netWorthComposition([])).toEqual({ assets: [], liabilities: [], investments: [] });
  });
});
