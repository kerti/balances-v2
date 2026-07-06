import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "@/api/client";
import type { FxRate } from "@/api/types";

// Manual monthly FX rates (ADR-0002). ['reports'] refresh is handled globally
// by the MutationCache in main.tsx (a rate change re-converts the dashboard).
export function useFxRates() {
  return useQuery({
    queryKey: ["fx-rates"],
    queryFn: () => api<FxRate[]>("/api/fx-rates"),
    staleTime: 30_000,
  });
}

function invalidate(qc: ReturnType<typeof useQueryClient>) {
  qc.invalidateQueries({ queryKey: ["fx-rates"] });
}

export function useCreateFxRate() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (p: { year_month: string; currency: string; rate: string }) =>
      api<FxRate>("/api/fx-rates", { method: "POST", body: JSON.stringify(p) }),
    onSuccess: () => invalidate(qc),
  });
}

export function useUpdateFxRate() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, rate }: { id: string; rate: string }) =>
      api<FxRate>(`/api/fx-rates/${id}`, {
        method: "PATCH",
        body: JSON.stringify({ rate }),
      }),
    onSuccess: () => invalidate(qc),
  });
}

export function useDeleteFxRate() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api(`/api/fx-rates/${id}`, { method: "DELETE" }),
    onSuccess: () => invalidate(qc),
  });
}
