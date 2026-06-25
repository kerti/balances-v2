// One household-scoped fetch of every position's monthly value + cost series
// for the list/home time graphs (issue #22) — replaces the per-position
// `useInvestmentBatch*` fan-out (N parallel snapshot + transaction fetches).
//
// The backend (`repo/investment_time_series.go`) samples cost at each
// position's snapshot months, exactly mirroring the old client-side
// `costBasisSeries` / `flatCostSeries`, so the series drop straight into
// `lib/listAggregates.ts#aggregateListPositions` with no client replay.
//
// The headline already reads `item.cost_basis` + `item.latest_snapshot` off
// the list payload (#18), so this hook only feeds the time graph.

import { useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import { api } from "@/api/client";

export type InvestmentTimeSeriesItem = {
  investment_id: string;
  value_series: Array<{ year_month: string; amount: string }>;
  cost_series: Array<{ year_month: string; cost: string }>;
};

// Shape consumed by `Position` in lib/listAggregates: `snapshots` (value by
// month) + `costSeries` (cost by month, cost as a number).
export type PositionSeries = {
  snapshots: Array<{ year_month: string; amount: string }>;
  costSeries: Array<{ year_month: string; cost: number }>;
};

export function useInvestmentTimeSeries(): {
  byId: Map<string, PositionSeries>;
  isLoading: boolean;
  hasError: boolean;
} {
  const query = useQuery({
    queryKey: ["investment-time-series"],
    queryFn: () =>
      api<InvestmentTimeSeriesItem[]>("/api/investments/time-series"),
  });

  const byId = useMemo(() => {
    const m = new Map<string, PositionSeries>();
    for (const it of query.data ?? []) {
      m.set(it.investment_id, {
        snapshots: it.value_series,
        costSeries: it.cost_series.map((c) => ({
          year_month: c.year_month,
          cost: Number(c.cost),
        })),
      });
    }
    return m;
  }, [query.data]);

  return { byId, isLoading: query.isPending, hasError: query.isError };
}
