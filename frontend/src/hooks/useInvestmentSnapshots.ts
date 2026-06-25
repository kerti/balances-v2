import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "@/api/client";
import type { InvestmentSnapshot } from "@/api/types";
import {
  postSnapshotImport,
  snapshotImportTemplateUrl,
  type ImportArgs,
} from "./snapshotImport";

// Investment snapshots live under /api/investments/{id}/snapshots — one
// shared table per ADR-0022. quantity + price_per_unit carry the value shape
// for stock/mutual_fund/gold (the backend's validateInvestmentSnapshotShape
// enforces the subtype→shape mapping; accrued_interest is for the M4.3b
// bond/time_deposit subtypes). Each subtype that uses these hooks should
// pre-fill the fields it cares about and leave the rest null.
//
// Mutations invalidate both the snapshot list query and the parent subtype
// list (which inlines the latest snapshot per row).

export type CreateInvestmentSnapshotPayload = {
  year_month: string;
  amount: string;
  currency: string;
  quantity: string | null;
  price_per_unit: string | null;
  accrued_interest: string | null;
  as_of_date: string | null;
  description: string | null;
};

export type UpdateInvestmentSnapshotPayload = {
  amount: string;
  currency: string;
  quantity: string | null;
  price_per_unit: string | null;
  accrued_interest: string | null;
  as_of_date: string | null;
  description: string | null;
};

export type InvestmentListKey =
  | "stocks"
  | "mutual-funds"
  | "golds"
  | "bonds"
  | "time-deposits";

export function useInvestmentSnapshots(investmentId: string | null) {
  return useQuery({
    queryKey: ["investment-snapshots", investmentId],
    queryFn: () =>
      api<InvestmentSnapshot[]>(`/api/investments/${investmentId}/snapshots`),
    enabled: !!investmentId,
  });
}

export function useCreateInvestmentSnapshot(
  investmentId: string,
  listKey: InvestmentListKey,
) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (payload: CreateInvestmentSnapshotPayload) =>
      api<InvestmentSnapshot>(`/api/investments/${investmentId}/snapshots`, {
        method: "POST",
        body: JSON.stringify(payload),
      }),
    onSuccess: () => {
      qc.invalidateQueries({
        queryKey: ["investment-snapshots", investmentId],
      });
      qc.invalidateQueries({ queryKey: [listKey] });
    },
  });
}

export function useUpdateInvestmentSnapshot(
  investmentId: string,
  listKey: InvestmentListKey,
) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (args: {
      snapshotId: string;
      payload: UpdateInvestmentSnapshotPayload;
    }) =>
      api<InvestmentSnapshot>(
        `/api/investments/${investmentId}/snapshots/${args.snapshotId}`,
        {
          method: "PATCH",
          body: JSON.stringify(args.payload),
        },
      ),
    onSuccess: () => {
      qc.invalidateQueries({
        queryKey: ["investment-snapshots", investmentId],
      });
      qc.invalidateQueries({ queryKey: [listKey] });
    },
  });
}

export function useDeleteInvestmentSnapshot(
  investmentId: string,
  listKey: InvestmentListKey,
) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (snapshotId: string) =>
      api(`/api/investments/${investmentId}/snapshots/${snapshotId}`, {
        method: "DELETE",
      }),
    onSuccess: () => {
      qc.invalidateQueries({
        queryKey: ["investment-snapshots", investmentId],
      });
      qc.invalidateQueries({ queryKey: [listKey] });
    },
  });
}

// ----- bulk snapshot import (xlsx template) -------------------------------
// The backend picks the template column shape (quantity-price vs
// accrued-interest) from the investment's subtype, so the frontend stays
// shape-agnostic — same dialog, same hook, for all five subtypes.

export function investmentImportTemplateUrl(investmentId: string): string {
  return snapshotImportTemplateUrl(
    `/api/investments/${investmentId}/snapshots`,
  );
}

// Per-subtype export URLs for the full position workbook (Detail + Snapshots +
// Transactions). Plain-GET download — the session cookie rides along
// same-origin, like the import-template link. The route is subtype-specific
// because the backend resolves the subtype-shaped Detail sheet.
export function stockExportUrl(investmentId: string): string {
  return `/api/investments/stocks/${investmentId}/export`;
}

export function mutualFundExportUrl(investmentId: string): string {
  return `/api/investments/mutual-funds/${investmentId}/export`;
}

export function bondExportUrl(investmentId: string): string {
  return `/api/investments/bonds/${investmentId}/export`;
}

export function goldExportUrl(investmentId: string): string {
  return `/api/investments/golds/${investmentId}/export`;
}

export function timeDepositExportUrl(investmentId: string): string {
  return `/api/investments/time-deposits/${investmentId}/export`;
}

export function useImportInvestmentSnapshots(
  investmentId: string,
  listKey: InvestmentListKey,
) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (args: ImportArgs) =>
      postSnapshotImport(
        `/api/investments/${investmentId}/snapshots`,
        args.file,
        args.mode,
      ),
    onSuccess: (result) => {
      // Only a real write should refresh the caches; a preview changed nothing.
      if (result.committed) {
        qc.invalidateQueries({
          queryKey: ["investment-snapshots", investmentId],
        });
        qc.invalidateQueries({ queryKey: [listKey] });
      }
    },
  });
}
