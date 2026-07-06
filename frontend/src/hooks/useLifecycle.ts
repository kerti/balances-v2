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

// Query keys to invalidate after a lifecycle change. Always the list + the
// single-row cache; for investments also the snapshot list, because an
// investment terminal flip (Sell / manual terminate) upserts a truthful 0-value
// close snapshot server-side (repo/lifecycle.go, INV-LIFECYCLE-03) — the same
// close the Maturity path writes. Without this refresh the new close snapshot
// only shows after a manual reload (issue #56). The other three groups carry no
// close snapshot, so they don't touch the snapshot list.
export function lifecycleInvalidationKeys(
  group: LifecycleGroup,
  id: string,
  listKey: string,
): unknown[][] {
  const keys: unknown[][] = [[listKey], [listKey, id]];
  if (group === "investments") {
    keys.push(["investment-snapshots", id]);
  }
  return keys;
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
