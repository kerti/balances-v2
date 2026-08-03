import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "@/api/client";
import type { LifecycleGroup } from "@/lib/lifecycle";

// PATCH /api/{group}/{id}/lifecycle. The backend operates on the parent table
// (4 groups, not the 10 subtypes), so every subtype detail page funnels through
// the same endpoint — the caller passes its own list query-key so we can
// invalidate both the list and the single-row cache after a status change.
export type LifecyclePayload = {
  status: string;
  terminated_at: string | null;
  termination_note: string | null;
};

// Snapshot-list query key per group. The keys are not uniformly named — assets
// claimed the bare "snapshots" first — so they are spelled out rather than
// derived from the group.
const SNAPSHOT_QUERY_KEY: Record<LifecycleGroup, string> = {
  assets: "snapshots",
  liabilities: "liability-snapshots",
  receivables: "receivable-snapshots",
  investments: "investment-snapshots",
};

// Query keys to invalidate after a lifecycle change: the list, the single-row
// cache, and the snapshot list. Every terminal flip writes a truthful 0-value
// close snapshot at the termination month server-side (repo/lifecycle.go,
// INV-LIFECYCLE-03), and un-terminating puts back the snapshot that close
// displaced — so the snapshot list changes underneath the detail page on every
// group. Without this refresh the change only shows after a manual reload (issue
// #56, originally investment-only; ADR-0052 generalised the write to all four).
export function lifecycleInvalidationKeys(
  group: LifecycleGroup,
  id: string,
  listKey: string,
): unknown[][] {
  return [[listKey], [listKey, id], [SNAPSHOT_QUERY_KEY[group], id]];
}

export function useUpdateLifecycle(group: LifecycleGroup, id: string, listKey: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (payload: LifecyclePayload) =>
      api(`/api/${group}/${id}/lifecycle`, {
        method: "PATCH",
        body: JSON.stringify(payload),
      }),
    onSuccess: () => {
      for (const queryKey of lifecycleInvalidationKeys(group, id, listKey)) {
        qc.invalidateQueries({ queryKey });
      }
    },
  });
}
