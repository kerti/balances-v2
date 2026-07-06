import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "@/api/client";
import type { AssetSnapshot } from "@/api/types";
import {
  postSnapshotImport,
  snapshotImportTemplateUrl,
  type ImportArgs,
  type ImportResult,
  type ImportRowError,
} from "./snapshotImport";

// Re-exported so existing importers of these types from useAssetSnapshots keep
// working after the shared-core extraction.
export type { ImportResult, ImportRowError };

// Asset snapshots live under /api/assets/{id}/snapshots — shared across
// every asset subtype (bank_account, property, vehicle) since the snapshot
// shape and storage table are the same per ADR-0022.
//
// Mutations invalidate all three asset-type list queries because each
// list shows the latest snapshot inline; we don't know which subtype the
// affected asset belongs to without an extra lookup, and invalidating
// three small queries at household scale is cheaper than tracking that.

const ASSET_LIST_KEYS = [["bank-accounts"], ["properties"], ["vehicles"]] as const;

function invalidateAssetLists(qc: ReturnType<typeof useQueryClient>) {
  ASSET_LIST_KEYS.forEach((key) =>
    qc.invalidateQueries({ queryKey: key as unknown as readonly unknown[] }),
  );
}

export type CreateSnapshotPayload = {
  year_month: string; // "YYYY-MM" or "YYYY-MM-DD"
  amount: string;
  currency: string;
  as_of_date: string | null;
  description: string | null;
};

export type UpdateSnapshotPayload = {
  amount: string;
  currency: string;
  as_of_date: string | null;
  description: string | null;
};

export function useSnapshots(assetId: string | null) {
  return useQuery({
    queryKey: ["snapshots", assetId],
    queryFn: () => api<AssetSnapshot[]>(`/api/assets/${assetId}/snapshots`),
    enabled: !!assetId,
  });
}

export function useCreateSnapshot(assetId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (payload: CreateSnapshotPayload) =>
      api<AssetSnapshot>(`/api/assets/${assetId}/snapshots`, {
        method: "POST",
        body: JSON.stringify(payload),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["snapshots", assetId] });
      invalidateAssetLists(qc);
    },
  });
}

export function useUpdateSnapshot(assetId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (args: { snapshotId: string; payload: UpdateSnapshotPayload }) =>
      api<AssetSnapshot>(`/api/assets/${assetId}/snapshots/${args.snapshotId}`, {
        method: "PATCH",
        body: JSON.stringify(args.payload),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["snapshots", assetId] });
      invalidateAssetLists(qc);
    },
  });
}

export function useDeleteSnapshot(assetId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (snapshotId: string) =>
      api(`/api/assets/${assetId}/snapshots/${snapshotId}`, {
        method: "DELETE",
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["snapshots", assetId] });
      invalidateAssetLists(qc);
    },
  });
}

// ----- bulk snapshot import (xlsx template) -------------------------------

// importTemplateUrl scopes the shared download URL to one asset.
export function importTemplateUrl(assetId: string): string {
  return snapshotImportTemplateUrl(`/api/assets/${assetId}/snapshots`);
}

// bankAccountExportUrl is the plain-GET download for a bank account's full
// position workbook (Detail + Snapshots). Like the template link, the session
// cookie rides along same-origin, so a bare anchor is enough.
export function bankAccountExportUrl(assetId: string): string {
  return `/api/bank-accounts/${assetId}/export`;
}

// propertyExportUrl / vehicleExportUrl are the same plain-GET workbook download
// for the property and vehicle groups.
export function propertyExportUrl(assetId: string): string {
  return `/api/properties/${assetId}/export`;
}

export function vehicleExportUrl(assetId: string): string {
  return `/api/vehicles/${assetId}/export`;
}

export function useImportSnapshots(assetId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (args: ImportArgs) =>
      postSnapshotImport(`/api/assets/${assetId}/snapshots`, args.file, args.mode),
    onSuccess: (result) => {
      // Only a real write should refresh the snapshot/list caches; a preview
      // changed nothing. (The global MutationCache still pokes ['reports'],
      // which is a harmless no-op refetch when nothing was committed.)
      if (result.committed) {
        qc.invalidateQueries({ queryKey: ["snapshots", assetId] });
        invalidateAssetLists(qc);
      }
    },
  });
}
