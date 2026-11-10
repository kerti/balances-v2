import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "@/api/client";
import { toastSaved } from "@/lib/writeToast";
import i18n from "@/i18n";
import { useUndoableDelete } from "@/hooks/useUndoableDelete";
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

// Foreign currencies the household's items are recorded in (reporting currency
// excluded): the options of the New-rate currency select. Always refetched on
// open so an item added a moment ago is offered.
export function useFxCurrencies() {
  return useQuery({
    queryKey: ["fx-rates", "currencies"],
    queryFn: () => api<string[]>("/api/fx-rates/currencies"),
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
    onSuccess: () => {
      invalidate(qc);
      toastSaved("added");
    },
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
    onSuccess: () => {
      invalidate(qc);
      toastSaved();
    },
  });
}

export function useDeleteFxRate() {
  return useUndoableDelete({
    path: (id) => `/api/fx-rates/${id}`,
    message: () => i18n.t("common:toast.deleted.fxRate"),
    invalidate: invalidate,
  });
}
