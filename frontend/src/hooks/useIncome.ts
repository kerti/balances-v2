import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "@/api/client";
import { toastBulkSaved, toastSaved } from "@/lib/writeToast";
import i18n from "@/i18n";
import { useUndoableDelete } from "@/hooks/useUndoableDelete";
import type { Income, IncomeCategory, Regularity } from "@/api/types";

export type IncomePayload = {
  date: string;
  amount: string;
  currency: string;
  category: IncomeCategory;
  description: string | null;
  ownership_type: "sole" | "joint";
  sole_owner_user_id: string | null;
  regularity: Regularity;
};

export function useIncome() {
  return useQuery({
    queryKey: ["income"],
    queryFn: () => api<Income[]>("/api/income"),
    staleTime: 10_000,
  });
}

export function useCreateIncome() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (payload: IncomePayload) =>
      api<Income>("/api/income", {
        method: "POST",
        body: JSON.stringify(payload),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["income"] });
      toastSaved("added");
    },
  });
}

// The copy-forward save (#846): every reviewed row in one request, so one toast.
export function useCreateIncomeBulk() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (rows: IncomePayload[]) =>
      api<Income[]>("/api/income/bulk", {
        method: "POST",
        body: JSON.stringify({ rows }),
      }),
    onSuccess: (written) => {
      qc.invalidateQueries({ queryKey: ["income"] });
      toastBulkSaved(written.length, 0);
    },
  });
}

export function useUpdateIncome(id: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (payload: IncomePayload) =>
      api<Income>(`/api/income/${id}`, {
        method: "PATCH",
        body: JSON.stringify(payload),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["income"] });
      toastSaved();
    },
  });
}

export function useDeleteIncome() {
  return useUndoableDelete({
    path: (id) => `/api/income/${id}`,
    message: () => i18n.t("common:toast.deleted.income"),
    invalidate: (qc) => {
      qc.invalidateQueries({ queryKey: ["income"] });
    },
  });
}
