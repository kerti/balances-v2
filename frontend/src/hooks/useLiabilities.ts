import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "@/api/client";
import { postCreateImport, type CreateImportArgs } from "@/hooks/snapshotImport";
import type { Liability, LiabilityListItem } from "@/api/types";

export type CreateLiabilityPayload = {
  display_name: string;
  description: string | null;
  subtype: "personal" | "institutional";
  ownership_type: "sole" | "joint";
  sole_owner_user_id: string | null;
  native_currency: string;
  counterparty_name: string;
  principal: string | null;
  interest_rate: string | null;
  term_months: number | null;
  start_date: string | null;
  maturity_date: string | null;
};

export type UpdateLiabilityPayload = {
  display_name: string;
  description: string | null;
  ownership_type: "sole" | "joint";
  sole_owner_user_id: string | null;
  counterparty_name: string;
  principal: string | null;
  interest_rate: string | null;
  term_months: number | null;
  start_date: string | null;
  maturity_date: string | null;
};

export function useLiabilities(subtype?: "personal" | "institutional") {
  return useQuery({
    queryKey: ["liabilities", subtype ?? "all"],
    queryFn: () => {
      const qs = subtype ? `?subtype=${subtype}` : "";
      return api<LiabilityListItem[]>(`/api/liabilities${qs}`);
    },
    staleTime: 10_000,
  });
}

export function useLiability(id: string | null) {
  return useQuery({
    queryKey: ["liabilities", id],
    queryFn: () => api<Liability>(`/api/liabilities/${id}`),
    enabled: !!id,
  });
}

export function useCreateLiability() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (payload: CreateLiabilityPayload) =>
      api<Liability>("/api/liabilities", {
        method: "POST",
        body: JSON.stringify(payload),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["liabilities"] });
    },
  });
}

export function useUpdateLiability(id: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (payload: UpdateLiabilityPayload) =>
      api<Liability>(`/api/liabilities/${id}`, {
        method: "PATCH",
        body: JSON.stringify(payload),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["liabilities"] });
      qc.invalidateQueries({ queryKey: ["liabilities", id] });
    },
  });
}

// useImportCreateLiability drives the create-from-file dialog on the list
// screen: a preview is a server-side dry-run; a committed create writes a new
// liability + its snapshots in one transaction, so only that refreshes the
// list (every subtype view, via the 'liabilities' key prefix).
export function useImportCreateLiability() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (args: CreateImportArgs) =>
      postCreateImport("/api/liabilities", args.file, args.mode),
    onSuccess: (result) => {
      if (result.committed) {
        qc.invalidateQueries({ queryKey: ["liabilities"] });
      }
    },
  });
}

export function useDeleteLiability() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api(`/api/liabilities/${id}`, { method: "DELETE" }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["liabilities"] });
    },
  });
}
