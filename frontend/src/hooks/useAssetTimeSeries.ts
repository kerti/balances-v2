// One household-scoped fetch of every asset's monthly value series for the
// Assets Home time graphs (epic #204) — the value-only twin of
// `useInvestmentTimeSeries`. Assets carry no cost basis (ADR-0022 shared
// snapshot table, no ledger), so the backend
// (`repo/asset_time_series.go`) returns value series only.

import { useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import { api } from "@/api/client";

export type AssetTimeSeriesItem = {
  asset_id: string;
  value_series: Array<{ year_month: string; amount: string }>;
};

// Shape consumed by `GroupPosition.snapshots` in lib/groupHomeAggregates.
export type AssetPositionSeries = {
  snapshots: Array<{ year_month: string; amount: string }>;
};

export function useAssetTimeSeries(): {
  byId: Map<string, AssetPositionSeries>;
  isLoading: boolean;
  hasError: boolean;
} {
  const query = useQuery({
    queryKey: ["asset-time-series"],
    queryFn: () => api<AssetTimeSeriesItem[]>("/api/assets/time-series"),
  });

  const byId = useMemo(() => {
    const m = new Map<string, AssetPositionSeries>();
    for (const it of query.data ?? []) {
      m.set(it.asset_id, { snapshots: it.value_series });
    }
    return m;
  }, [query.data]);

  return { byId, isLoading: query.isPending, hasError: query.isError };
}
