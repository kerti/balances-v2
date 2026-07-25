import type { UseMutationResult } from "@tanstack/react-query";
import { TableCell, TableRow } from "@/components/ui/table";
import { SnapshotRowMenu } from "@/components/common/SnapshotRowMenu";
import { useSnapshotRow, type SnapshotLike } from "@/components/common/useSnapshotRow";
import type { UpdateSnapshotMutationVariables } from "@/components/dialogs/EditSnapshotDialog";

type Props<TUpdate, TDelete> = {
  snapshot: SnapshotLike;
  updateMutation: UseMutationResult<TUpdate, unknown, UpdateSnapshotMutationVariables>;
  deleteMutation: UseMutationResult<TDelete, unknown, string>;
};

// The desktop table row for an amount-only snapshot. Shares its state, dialogs
// and ⋮ menu with the mobile `SnapshotCard` via `useSnapshotRow`, and the same
// `snapshot-*` testids, so the two renderers stay in lockstep (ADR-0051 Phase B).
export function SnapshotRow<TUpdate, TDelete>({
  snapshot,
  updateMutation,
  deleteMutation,
}: Props<TUpdate, TDelete>) {
  const { monthText, amountText, statementText, description, onEdit, onDelete, dialogs } =
    useSnapshotRow(snapshot, updateMutation, deleteMutation);

  return (
    <>
      <TableRow data-testid="snapshot-row">
        <TableCell>
          <div className="font-medium">{monthText}</div>
          {statementText && <div className="text-xs text-muted-foreground">{statementText}</div>}
        </TableCell>
        <TableCell className="text-right tabular-nums" data-testid="snapshot-amount">
          {amountText}
        </TableCell>
        <TableCell className="text-muted-foreground">{description ?? "—"}</TableCell>
        <TableCell className="text-right">
          <SnapshotRowMenu onEdit={onEdit} onDelete={onDelete} />
        </TableCell>
      </TableRow>

      {dialogs}
    </>
  );
}
