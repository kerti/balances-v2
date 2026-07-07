import type { FxRate, MonthlyReport } from "@/api/types";
import { convert, resolveDisplayRate } from "@/lib/fx";

export type IncomeStatementData = {
  earned: number;
  investmentReturn: number;
  assetValueChange: number;
  livingExpenses: number;
  netWorthChange: number;
};

export type ReportPdfData = {
  yearMonth: string;
  currency: string;
  headline: {
    total: string;
    secondary: { currency: string; amount: number; rateMonth: string } | null;
  };
  incomeStatement: IncomeStatementData | null;
  byPerson: { key: string; nw: string }[];
  fxRatesUsed: { currency: string; rate: string }[];
  groupBreakdown: { labelKey: string; value: number; negative: boolean }[];
  series: { year_month: string; amount: string }[];
};

export function buildReportPdfData(params: {
  reports: MonthlyReport[];
  selected: MonthlyReport;
  currency: string;
  secondaryCurrency: string;
  rates: FxRate[];
}): ReportPdfData {
  const { reports, selected, currency, secondaryCurrency, rates } = params;
  let secondary: ReportPdfData["headline"]["secondary"] = null;
  if (secondaryCurrency) {
    const resolved = resolveDisplayRate(rates, secondaryCurrency, selected.year_month);
    if (resolved) {
      secondary = {
        currency: secondaryCurrency,
        amount: convert(selected.nw_total, resolved.rate),
        rateMonth: resolved.rateMonth,
      };
    }
  }
  let incomeStatement: ReportPdfData["incomeStatement"] = null;
  if (selected.derived_living_expenses !== null) {
    const earned = Number(selected.earned_income_total ?? "0");
    const investmentReturn = Number(selected.investment_return_total ?? "0");
    const assetValueChange = Number(selected.asset_value_change ?? "0");
    const livingExpenses = Number(selected.derived_living_expenses);
    incomeStatement = {
      earned,
      investmentReturn,
      assetValueChange,
      livingExpenses,
      netWorthChange: earned + investmentReturn + assetValueChange - livingExpenses,
    };
  }

  const byPerson = Object.entries(selected.user_breakdowns)
    .map(([key, bucket]) => ({ key, nw: bucket.nw }))
    .sort((a, b) => Number(b.nw) - Number(a.nw));

  const fxRatesUsed = Object.entries(selected.fx_rates_used)
    .map(([currency, rate]) => ({ currency, rate }))
    .sort((a, b) => a.currency.localeCompare(b.currency));

  const groupBreakdown: ReportPdfData["groupBreakdown"] = [
    { labelKey: "assets", value: Number(selected.nw_assets), negative: false },
    { labelKey: "investments", value: Number(selected.nw_investments), negative: false },
    { labelKey: "receivables", value: Number(selected.nw_receivables), negative: false },
    { labelKey: "liabilities", value: Number(selected.nw_liabilities), negative: true },
  ];

  const series = reports.map((r) => ({ year_month: r.year_month, amount: r.nw_total }));

  return {
    yearMonth: selected.year_month,
    currency,
    headline: {
      total: selected.nw_total,
      secondary,
    },
    incomeStatement,
    byPerson,
    fxRatesUsed,
    groupBreakdown,
    series,
  };
}
