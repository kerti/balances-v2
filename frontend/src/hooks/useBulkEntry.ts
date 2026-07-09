import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, ApiError, isEnvelope, type ErrorEnvelope } from "@/api/client";

// One row of the bulk monthly-entry list (ADR-0046): an eligible asset with its
// carry-forward prefill. prefill_amount / carried_from are null for an asset
// with no snapshot at or before the target month. Shapes mirror the Go handler
// (internal/assets/bulk_snapshots.go) — hand-written DTOs, not model types.
export type AssetEntryRow = {
  asset_id: string;
  display_name: string;
  currency: string;
  prefill_amount: string | null;
  carried_from: string | null;
};

export type AssetEntryList = {
  year_month: string;
  rows: AssetEntryRow[];
};

// One dirty row the user changed — the client sends only these.
export type BulkSnapshotRow = {
  asset_id: string;
  amount: string;
  currency: string;
};

export type BulkSaveArgs = {
  year_month: string;
  as_of_date: string;
  rows: BulkSnapshotRow[];
};

// The 422 per-row error body (ADR-0046) — a documented non-envelope shape, like
// the snapshot importer's row errors, so it is read off the response directly
// rather than through the envelope-filtering `api()` wrapper.
export type BulkRowError = { asset_id: string; code: string };

// A save either commits (ok) or is rejected with per-row errors (a 422 that is
// data, not a thrown error — mirrors postCreateImport's 422 handling).
export type BulkSaveResult = { ok: true; written: number } | { ok: false; errors: BulkRowError[] };

// useAssetEntryList fetches the eligible-assets-with-prefill list for a target
// month. Disabled until a month is chosen.
export function useAssetEntryList(yearMonth: string | null) {
  return useQuery({
    queryKey: ["assets", "entry", yearMonth],
    queryFn: () => api<AssetEntryList>(`/api/assets/snapshots/entry?year_month=${yearMonth}`),
    enabled: !!yearMonth,
    staleTime: 0,
  });
}

// useBulkSaveAssetSnapshots posts a whole batch. A 422 (per-row rejections) is
// returned as data — `{ ok: false }` — not thrown, since the envelope-filtering
// `api()` wrapper would drop the non-envelope body; every other non-2xx throws
// ApiError as usual. On a committed save it invalidates the asset lists +
// reports so carried-forward net worth refreshes.
export function useBulkSaveAssetSnapshots() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (args: BulkSaveArgs): Promise<BulkSaveResult> => {
      const res = await fetch("/api/assets/snapshots/bulk", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(args),
      });
      if (res.status === 422) {
        const body = (await res.json()) as { errors: BulkRowError[] };
        return { ok: false, errors: body.errors };
      }
      if (!res.ok) {
        let errBody: ErrorEnvelope | undefined;
        try {
          const parsed = await res.json();
          errBody = isEnvelope(parsed) ? parsed : undefined;
        } catch {
          errBody = undefined;
        }
        throw new ApiError(
          res.status,
          res.statusText || `bulk save failed (${res.status})`,
          errBody,
        );
      }
      const body = (await res.json()) as { written: number };
      return { ok: true, written: body.written };
    },
    onSuccess: (result) => {
      if (!result.ok) return;
      qc.invalidateQueries({ queryKey: ["assets"] });
      qc.invalidateQueries({ queryKey: ["bank-accounts"] });
      qc.invalidateQueries({ queryKey: ["reports"] });
    },
  });
}
