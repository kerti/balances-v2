// One household-scoped fetch of every liability's monthly value series for the
// Liabilities Home time graphs (epic #204) — the value-only twin of
// useAssetTimeSeries. Liabilities carry no cost basis, so the backend
// (`repo/liability_time_series.go`) returns value series only.

import { useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import { api } from "@/api/client";

export type LiabilityTimeSeriesItem = {
  liability_id: string;
  value_series: Array<{ year_month: string; amount: string }>;
};

export type LiabilityPositionSeries = {
  snapshots: Array<{ year_month: string; amount: string }>;
};

export function useLiabilityTimeSeries(): {
  byId: Map<string, LiabilityPositionSeries>;
  isLoading: boolean;
  hasError: boolean;
} {
  const query = useQuery({
    queryKey: ["liability-time-series"],
    queryFn: () =>
      api<LiabilityTimeSeriesItem[]>("/api/liabilities/time-series"),
  });

  const byId = useMemo(() => {
    const m = new Map<string, LiabilityPositionSeries>();
    for (const it of query.data ?? []) {
      m.set(it.liability_id, { snapshots: it.value_series });
    }
    return m;
  }, [query.data]);

  return { byId, isLoading: query.isPending, hasError: query.isError };
}
