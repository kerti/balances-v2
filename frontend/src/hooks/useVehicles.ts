import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "@/api/client";
import { postCreateImport, type CreateImportArgs } from "@/hooks/snapshotImport";
import type { Vehicle, VehicleListItem } from "@/api/types";

export type CreateVehiclePayload = {
  display_name: string;
  description: string | null;
  ownership_type: "sole" | "joint";
  sole_owner_user_id: string | null;
  native_currency: string;
  vehicle_type: "car" | "motorcycle" | "other";
  make: string | null;
  model: string | null;
  year: number | null;
  plate_number: string | null;
  annual_depreciation_rate: string | null;
};

export type UpdateVehiclePayload = {
  display_name: string;
  description: string | null;
  ownership_type: "sole" | "joint";
  sole_owner_user_id: string | null;
  vehicle_type: "car" | "motorcycle" | "other";
  make: string | null;
  model: string | null;
  year: number | null;
  plate_number: string | null;
  annual_depreciation_rate: string | null;
};

export function useVehicles() {
  return useQuery({
    queryKey: ["vehicles"],
    queryFn: () => api<VehicleListItem[]>("/api/vehicles"),
    staleTime: 10_000,
  });
}

export function useVehicle(id: string | null) {
  return useQuery({
    queryKey: ["vehicles", id],
    queryFn: () => api<Vehicle>(`/api/vehicles/${id}`),
    enabled: !!id,
  });
}

export function useCreateVehicle() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (payload: CreateVehiclePayload) =>
      api<Vehicle>("/api/vehicles", {
        method: "POST",
        body: JSON.stringify(payload),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["vehicles"] });
    },
  });
}

export function useUpdateVehicle(id: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (payload: UpdateVehiclePayload) =>
      api<Vehicle>(`/api/vehicles/${id}`, {
        method: "PATCH",
        body: JSON.stringify(payload),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["vehicles"] });
      qc.invalidateQueries({ queryKey: ["vehicles", id] });
    },
  });
}

// useImportCreateVehicle drives the create-from-file dialog on the list screen:
// a preview is a server-side dry-run; a committed create writes a new vehicle +
// its snapshots in one transaction, so only that refreshes the list.
export function useImportCreateVehicle() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (args: CreateImportArgs) => postCreateImport("/api/vehicles", args.file, args.mode),
    onSuccess: (result) => {
      if (result.committed) {
        qc.invalidateQueries({ queryKey: ["vehicles"] });
      }
    },
  });
}

export function useDeleteVehicle() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api(`/api/vehicles/${id}`, { method: "DELETE" }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["vehicles"] });
    },
  });
}
