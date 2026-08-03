import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "@/api/client";
import type { LifecycleGroup } from "@/lib/lifecycle";

// PATCH /api/{group}/{id}/lifecycle. The backend operates on the parent table
// (4 groups, not the 10 subtypes), so every subtype detail page funnels through
// the same endpoint — the caller passes its own list query-key so we can
// invalidate both the list and the single-row cache after a status change.
// `settlement` is Investment-only (ADR-0052 §6): the terminal Sell/Maturity the
// dialog books in the SAME database transaction as the flip, so the position can
// never be left holding nothing with no record of where its value went. Its
// shape follows the subtype — quantity × price_per_unit for a Sell, principal +
// interest for a Maturity — and the unused pair stays null. Omitted entirely by
// the other three groups, by an un-terminate, and when the termination month
// already carries a sale the user recorded by hand.
export type LifecycleSettlement = {
  quantity: string | null;
  price_per_unit: string | null;
  principal_amount: string | null;
  interest_amount: string | null;
};

export type LifecyclePayload = {
  status: string;
  terminated_at: string | null;
  termination_note: string | null;
  settlement?: LifecycleSettlement;
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
  const keys: unknown[][] = [[listKey], [listKey, id], [SNAPSHOT_QUERY_KEY[group], id]];
  // Terminating an Investment can now write its settling Sell/Maturity in the
  // same request (ADR-0052 §6), so the ledger the detail page renders is stale
  // the moment the flip lands. Only investments have one.
  if (group === "investments") {
    keys.push(["investment-transactions", id]);
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
