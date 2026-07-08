import type { FxRate, MonthlyReport, PositionDetail } from "@/api/types";
import { convert, resolveDisplayRate } from "@/lib/fx";

export type IncomeStatementData = {
  earned: number;
  investmentReturn: number;
  assetValueChange: number;
  livingExpenses: number;
  netWorthChange: number;
};

// ItemizedPosition is one position drilled down under its group/subtype
// heading (ADR-0045) — the itemized breakdown the v1 export deliberately
// omitted. amount is already in the reporting currency (the /positions
// endpoint converts server-side); no further FX math needed here.
export type ItemizedPosition = {
  id: string;
  name: string;
  subtype: string;
  amount: number;
  stale: boolean;
};

export type ItemizedPositions = {
  asset: ItemizedPosition[];
  liability: ItemizedPosition[];
  receivable: ItemizedPosition[];
  investment: ItemizedPosition[];
};

// CompositionSlice is one wedge of a composition donut — key is a subtype or
// income/return category, resolved to a translated label by the render layer
// (ReportDocument), same separation as groupBreakdown's labelKey.
export type CompositionSlice = { key: string; value: number };

export type ReportPdfComposition = {
  assets: CompositionSlice[];
  investments: CompositionSlice[];
  liabilities: CompositionSlice[];
  earnedIncome: CompositionSlice[];
  investmentReturn: CompositionSlice[];
};

// TrendPoint extends the existing net-worth series with the two lines the
// reference template's "expense vs passive income" chart wants. Passive
// income = investment return (as opposed to earned/active income) — the
// household's own framing, matching the template's "Pendapatan Pasif" label.
export type TrendPoint = { year_month: string; livingExpenses: number; investmentReturn: number };

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
  itemizedPositions: ItemizedPositions;
  composition: ReportPdfComposition;
  trend: TrendPoint[];
};

// EARNED_INCOME_CATEGORIES / INVESTMENT_RETURN_SUBTYPES fix the composition
// pie's key order (stable legend) and name the MonthlyReport per-category
// fields to read — mirrors ADR-0012's category set.
const EARNED_INCOME_CATEGORIES: { key: string; field: keyof MonthlyReport }[] = [
  { key: "salary", field: "earned_income_salary" },
  { key: "business", field: "earned_income_business" },
  { key: "rental", field: "earned_income_rental" },
  { key: "gift", field: "earned_income_gift" },
  { key: "tax_refund", field: "earned_income_tax_refund" },
  { key: "insurance", field: "earned_income_insurance" },
  { key: "other", field: "earned_income_other" },
];

const INVESTMENT_RETURN_SUBTYPES: { key: string; field: keyof MonthlyReport }[] = [
  { key: "stock", field: "investment_return_stock" },
  { key: "mutual_fund", field: "investment_return_mutual_fund" },
  { key: "bond", field: "investment_return_bond" },
  { key: "gold", field: "investment_return_gold" },
  { key: "time_deposit", field: "investment_return_time_deposit" },
];

function groupItemized(positions: PositionDetail[]): ItemizedPositions {
  const out: ItemizedPositions = { asset: [], liability: [], receivable: [], investment: [] };
  for (const p of positions) {
    out[p.group].push({
      id: p.position_id,
      name: p.name,
      subtype: p.subtype,
      amount: Number(p.amount),
      stale: p.stale,
    });
  }
  for (const group of Object.values(out)) {
    group.sort((a, b) => b.amount - a.amount);
  }
  return out;
}

function subtypeComposition(items: ItemizedPosition[]): CompositionSlice[] {
  const totals = new Map<string, number>();
  for (const item of items) {
    totals.set(item.subtype, (totals.get(item.subtype) ?? 0) + item.amount);
  }
  return [...totals.entries()]
    .map(([key, value]) => ({ key, value }))
    .sort((a, b) => b.value - a.value);
}

// categoryComposition reads a fixed set of per-category MonthlyReport fields
// (null on the baseline month, same as the totals they sum to) and drops
// zero/absent categories so the legend doesn't pad out with empty wedges.
function categoryComposition(
  report: MonthlyReport,
  categories: { key: string; field: keyof MonthlyReport }[],
): CompositionSlice[] {
  return categories
    .map(({ key, field }) => ({ key, value: Number(report[field] ?? "0") }))
    .filter((slice) => slice.value !== 0);
}

// last12MonthsTrend takes the most recent 12 months up to and including the
// selected month — not the full series buildReportPdfData's `series` uses for
// the net-worth chart, which spans the household's whole history.
function last12MonthsTrend(reports: MonthlyReport[], uptoYearMonth: string): TrendPoint[] {
  return reports
    .filter((r) => r.year_month <= uptoYearMonth)
    .sort((a, b) => a.year_month.localeCompare(b.year_month))
    .slice(-12)
    .map((r) => ({
      year_month: r.year_month,
      livingExpenses: Number(r.derived_living_expenses ?? "0"),
      investmentReturn: Number(r.investment_return_total ?? "0"),
    }));
}

export function buildReportPdfData(params: {
  reports: MonthlyReport[];
  selected: MonthlyReport;
  currency: string;
  secondaryCurrency: string;
  rates: FxRate[];
  positions: PositionDetail[];
}): ReportPdfData {
  const { reports, selected, currency, secondaryCurrency, rates, positions } = params;
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

  const itemizedPositions = groupItemized(positions);
  const composition: ReportPdfComposition = {
    assets: subtypeComposition(itemizedPositions.asset),
    investments: subtypeComposition(itemizedPositions.investment),
    liabilities: subtypeComposition(itemizedPositions.liability),
    earnedIncome: incomeStatement ? categoryComposition(selected, EARNED_INCOME_CATEGORIES) : [],
    investmentReturn: incomeStatement
      ? categoryComposition(selected, INVESTMENT_RETURN_SUBTYPES)
      : [],
  };
  const trend = last12MonthsTrend(reports, selected.year_month);

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
    itemizedPositions,
    composition,
    trend,
  };
}
