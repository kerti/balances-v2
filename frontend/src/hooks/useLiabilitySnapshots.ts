import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "@/api/client";
import type { LiabilitySnapshot } from "@/api/types";
import {
  postSnapshotImport,
  snapshotImportTemplateUrl,
  type ImportArgs,
} from "./snapshotImport";

// Liability snapshots live under /api/liabilities/{id}/snapshots — per-group
// per ADR-0022. Mutations invalidate the liability list query because each
// row in the list shows the latest snapshot inline.

export type CreateLiabilitySnapshotPayload = {
  year_month: string;
  amount: string;
  currency: string;
  as_of_date: string | null;
  description: string | null;
};

export type UpdateLiabilitySnapshotPayload = {
  amount: string;
  currency: string;
  as_of_date: string | null;
  description: string | null;
};

export function useLiabilitySnapshots(liabilityId: string | null) {
  return useQuery({
    queryKey: ["liability-snapshots", liabilityId],
    queryFn: () =>
      api<LiabilitySnapshot[]>(`/api/liabilities/${liabilityId}/snapshots`),
    enabled: !!liabilityId,
  });
}

export function useCreateLiabilitySnapshot(liabilityId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (payload: CreateLiabilitySnapshotPayload) =>
      api<LiabilitySnapshot>(`/api/liabilities/${liabilityId}/snapshots`, {
        method: "POST",
        body: JSON.stringify(payload),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["liability-snapshots", liabilityId] });
      qc.invalidateQueries({ queryKey: ["liabilities"] });
    },
  });
}

export function useUpdateLiabilitySnapshot(liabilityId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (args: {
      snapshotId: string;
      payload: UpdateLiabilitySnapshotPayload;
    }) =>
      api<LiabilitySnapshot>(
        `/api/liabilities/${liabilityId}/snapshots/${args.snapshotId}`,
        {
          method: "PATCH",
          body: JSON.stringify(args.payload),
        },
      ),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["liability-snapshots", liabilityId] });
      qc.invalidateQueries({ queryKey: ["liabilities"] });
    },
  });
}

export function useDeleteLiabilitySnapshot(liabilityId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (snapshotId: string) =>
      api(`/api/liabilities/${liabilityId}/snapshots/${snapshotId}`, {
        method: "DELETE",
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["liability-snapshots", liabilityId] });
      qc.invalidateQueries({ queryKey: ["liabilities"] });
    },
  });
}

// ----- bulk snapshot import (xlsx template) -------------------------------

export function liabilityImportTemplateUrl(liabilityId: string): string {
  return snapshotImportTemplateUrl(`/api/liabilities/${liabilityId}/snapshots`);
}

// liabilityExportUrl is the plain-GET download for a liability's full position
// workbook (Detail + Snapshots). The session cookie rides along same-origin, so
// a bare anchor is enough.
export function liabilityExportUrl(liabilityId: string): string {
  return `/api/liabilities/${liabilityId}/export`;
}

export function useImportLiabilitySnapshots(liabilityId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (args: ImportArgs) =>
      postSnapshotImport(
        `/api/liabilities/${liabilityId}/snapshots`,
        args.file,
        args.mode,
      ),
    onSuccess: (result) => {
      // Only a real write should refresh the caches; a preview changed nothing.
      if (result.committed) {
        qc.invalidateQueries({
          queryKey: ["liability-snapshots", liabilityId],
        });
        qc.invalidateQueries({ queryKey: ["liabilities"] });
      }
    },
  });
}
