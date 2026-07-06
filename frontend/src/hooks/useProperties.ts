import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "@/api/client";
import { postCreateImport, type CreateImportArgs } from "@/hooks/snapshotImport";
import type { Property, PropertyListItem } from "@/api/types";

export type CreatePropertyPayload = {
  display_name: string;
  description: string | null;
  ownership_type: "sole" | "joint";
  sole_owner_user_id: string | null;
  native_currency: string;
  property_type: "house" | "apartment" | "land" | "commercial";
  address: string | null;
  acquisition_date: string | null;
  acquisition_cost: string | null;
  annual_appreciation_rate: string | null;
};

export type UpdatePropertyPayload = {
  display_name: string;
  description: string | null;
  ownership_type: "sole" | "joint";
  sole_owner_user_id: string | null;
  property_type: "house" | "apartment" | "land" | "commercial";
  address: string | null;
  acquisition_date: string | null;
  acquisition_cost: string | null;
  annual_appreciation_rate: string | null;
};

export function useProperties() {
  return useQuery({
    queryKey: ["properties"],
    queryFn: () => api<PropertyListItem[]>("/api/properties"),
    staleTime: 10_000,
  });
}

export function useProperty(id: string | null) {
  return useQuery({
    queryKey: ["properties", id],
    queryFn: () => api<Property>(`/api/properties/${id}`),
    enabled: !!id,
  });
}

export function useCreateProperty() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (payload: CreatePropertyPayload) =>
      api<Property>("/api/properties", {
        method: "POST",
        body: JSON.stringify(payload),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["properties"] });
    },
  });
}

export function useUpdateProperty(id: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (payload: UpdatePropertyPayload) =>
      api<Property>(`/api/properties/${id}`, {
        method: "PATCH",
        body: JSON.stringify(payload),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["properties"] });
      qc.invalidateQueries({ queryKey: ["properties", id] });
    },
  });
}

// useImportCreateProperty drives the create-from-file dialog on the list
// screen: a preview is a server-side dry-run; a committed create writes a new
// property + its snapshots in one transaction, so only that refreshes the list.
export function useImportCreateProperty() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (args: CreateImportArgs) =>
      postCreateImport("/api/properties", args.file, args.mode),
    onSuccess: (result) => {
      if (result.committed) {
        qc.invalidateQueries({ queryKey: ["properties"] });
      }
    },
  });
}

export function useDeleteProperty() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api(`/api/properties/${id}`, { method: "DELETE" }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["properties"] });
    },
  });
}
