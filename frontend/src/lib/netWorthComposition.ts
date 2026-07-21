import type { MonthlyReport } from "@/api/types";

// A single point on a composition line — the chart's SnapshotLike shape
// (year_month + string amount), so these arrays feed SnapshotChart directly.
export type SeriesPoint = { year_month: string; amount: string };

export type Composition = {
  assets: SeriesPoint[];
  liabilities: SeriesPoint[];
  investments: SeriesPoint[];
};

// netWorthComposition derives the dashboard net-worth chart's three secondary
// lines from the monthly reports — the presentation of net-worth composition
// per ADR-0001's Presentation / UX section:
//
//   assets       — the asset group with RECEIVABLES FOLDED IN (receivables is
//                  small/often-empty; a 4th line would clutter without signal)
//   liabilities  — a POSITIVE MAGNITUDE, exactly as the engine stores it (a
//                  below-zero line is more faithful but less readable for the
//                  non-technical audience)
//   investments  — the investment group at total closing value, passed through
//
// The net-worth primary line is built separately from nw_total. By construction
// `assets + investments − liabilities === nw_total` for every month — the fold
// and the positive-magnitude choice are presentation-only and do not change
// that identity (INV-PRESENTATION-07). Arithmetic uses Number(), the codebase's
// convention for chart/display math (cf. lib/costBasis).
export function netWorthComposition(reports: MonthlyReport[]): Composition {
  return {
    assets: reports.map((r) => ({
      year_month: r.year_month,
      amount: String(Number(r.nw_assets) + Number(r.nw_receivables)),
    })),
    liabilities: reports.map((r) => ({
      year_month: r.year_month,
      amount: r.nw_liabilities,
    })),
    investments: reports.map((r) => ({
      year_month: r.year_month,
      amount: r.nw_investments,
    })),
  };
}
