import type { UseMutationResult } from "@tanstack/react-query";
import { CompactSnapshotRow } from "@/components/common/CompactSnapshotRow";
import { SnapshotRowMenu } from "@/components/common/SnapshotRowMenu";
import { useSnapshotRow, type SnapshotLike } from "@/components/common/useSnapshotRow";
import type { UpdateSnapshotMutationVariables } from "@/components/dialogs/EditSnapshotDialog";

type Props<TUpdate, TDelete> = {
  snapshot: SnapshotLike;
  updateMutation: UseMutationResult<TUpdate, unknown, UpdateSnapshotMutationVariables>;
  deleteMutation: UseMutationResult<TDelete, unknown, string>;
};

// Mobile leaf renderer for an amount-only snapshot: a compact `month · value · ⋯`
// row (#952, the explicit exception to #849's labelled cards). Shares state,
// dialogs and the ⋮ menu with the desktop `SnapshotRow` via `useSnapshotRow`, and
// the same `snapshot-*` testids. The ⋮ trigger sizes to the 44px tap floor
// (INV-PRESENTATION-08).
export function SnapshotCard<TUpdate, TDelete>({
  snapshot,
  updateMutation,
  deleteMutation,
}: Props<TUpdate, TDelete>) {
  const { monthText, amountText, statementText, description, onEdit, onDelete, dialogs } =
    useSnapshotRow(snapshot, updateMutation, deleteMutation);

  return (
    <>
      <CompactSnapshotRow
        monthText={monthText}
        amountText={amountText}
        statement={statementText}
        note={description}
        menu={
          <SnapshotRowMenu
            onEdit={onEdit}
            onDelete={onDelete}
            variant="ghost"
            triggerClassName="size-11 shrink-0"
          />
        }
      />
      {dialogs}
    </>
  );
}
