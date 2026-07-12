import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "@/api/client";
import type { InflationRate } from "@/api/types";

// Manual monthly inflation rates (ADR-0048). Each rate is an annualized (YoY)
// percentage feeding the Fund Resilience projection. No ['reports'] refresh is
// needed — inflation only affects the (deferred) statistics panel, not the
// dashboard's converted totals.
export function useInflationRates() {
  return useQuery({
    queryKey: ["inflation-rates"],
    queryFn: () => api<InflationRate[]>("/api/inflation-rates"),
    staleTime: 30_000,
  });
}

function invalidate(qc: ReturnType<typeof useQueryClient>) {
  qc.invalidateQueries({ queryKey: ["inflation-rates"] });
}

export function useCreateInflationRate() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (p: { year_month: string; rate: string }) =>
      api<InflationRate>("/api/inflation-rates", {
        method: "POST",
        body: JSON.stringify(p),
      }),
    onSuccess: () => invalidate(qc),
  });
}

export function useUpdateInflationRate() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, rate }: { id: string; rate: string }) =>
      api<InflationRate>(`/api/inflation-rates/${id}`, {
        method: "PATCH",
        body: JSON.stringify({ rate }),
      }),
    onSuccess: () => invalidate(qc),
  });
}

export function useDeleteInflationRate() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api(`/api/inflation-rates/${id}`, { method: "DELETE" }),
    onSuccess: () => invalidate(qc),
  });
}
