import { useMutation, useQuery, useQueryClient, type QueryClient } from "@tanstack/react-query";
import { api } from "@/api/client";
import { snapshotGroupOf, type SnapshotGroup } from "@/lib/snapshotMove";
import { formatRecordMonth } from "@/lib/recordSnapshot";
import { toastSnapshotMove } from "@/lib/writeToast";
import type { SnapshotWrite } from "@/api/types";

// A live snapshot of the same position, in any group: the fields the Move
// dialog needs to spot a taken target month and name its value.
export type MovableSnapshot = {
  id: string;
  year_month: string;
  amount: string;
  currency: string;
};

function invalidate(qc: QueryClient, group: SnapshotGroup) {
  qc.invalidateQueries({ queryKey: group.listKey });
  group.positionKeys.forEach((key) => qc.invalidateQueries({ queryKey: key }));
}

// Move an Edit-dialog snapshot to another month (ADR-0046 amendment, #791). The
// group's routes and query keys come from the row itself (lib/snapshotMove), so
// the three snapshot shapes share this one hook. `group` is null for a row with
// no parent id, in which case the dialog does not offer Move.
export function useMoveSnapshot(snapshot: MovableSnapshot) {
  const qc = useQueryClient();
  const group = snapshotGroupOf(snapshot);

  // The same query the detail page runs, so it is normally already cached; the
  // dialog reads it to say what a taken month holds.
  const list = useQuery({
    queryKey: group?.listKey ?? ["snapshot-move-none"],
    queryFn: () => api<MovableSnapshot[]>(group!.basePath),
    enabled: !!group,
  });

  const mutation = useMutation({
    mutationFn: (yearMonth: string) =>
      api<SnapshotWrite<MovableSnapshot>>(`${group!.basePath}/${snapshot.id}/move`, {
        method: "POST",
        body: JSON.stringify({ year_month: yearMonth }),
      }),
    onSuccess: (moved) => {
      invalidate(qc, group!);
      toastSnapshotMove(
        qc,
        group!.basePath,
        snapshot.id,
        moved,
        formatRecordMonth(moved.year_month),
        () => invalidate(qc, group!),
      );
    },
  });

  return { available: !!group, snapshots: list.data, mutation };
}
