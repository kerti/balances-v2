import type { UseMutationResult } from "@tanstack/react-query";
import { TableCell, TableRow } from "@/components/ui/table";
import { SnapshotRowMenu } from "@/components/common/SnapshotRowMenu";
import {
  useAccruedInterestSnapshotRow,
  type AccruedInterestSnapshotLike,
} from "@/components/common/useAccruedInterestSnapshotRow";
import type { UpdateAccruedInterestSnapshotMutationVariables } from "@/components/dialogs/EditAccruedInterestSnapshotDialog";

type Props<TUpdate, TDelete> = {
  snapshot: AccruedInterestSnapshotLike;
  updateMutation: UseMutationResult<
    TUpdate,
    unknown,
    UpdateAccruedInterestSnapshotMutationVariables
  >;
  deleteMutation: UseMutationResult<TDelete, unknown, string>;
};

// The desktop table row for an accrued snapshot. Shares its state, dialogs and
// ⋮ menu with the mobile `AccruedInterestSnapshotCard` via
// `useAccruedInterestSnapshotRow`, and the same `snapshot-*` testids, so the two
// renderers stay in lockstep (ADR-0051 Phase B).
export function AccruedInterestSnapshotRow<TUpdate, TDelete>({
  snapshot,
  updateMutation,
  deleteMutation,
}: Props<TUpdate, TDelete>) {
  const {
    monthText,
    amountText,
    principalText,
    accruedText,
    statementText,
    description,
    onEdit,
    onDelete,
    dialogs,
  } = useAccruedInterestSnapshotRow(snapshot, updateMutation, deleteMutation);

  return (
    <>
      <TableRow data-testid="snapshot-row">
        <TableCell>
          <div className="font-medium">{monthText}</div>
          {statementText && <div className="text-xs text-muted-foreground">{statementText}</div>}
        </TableCell>
        <TableCell className="text-right tabular-nums">{principalText ?? "—"}</TableCell>
        <TableCell className="text-right tabular-nums">{accruedText ?? "—"}</TableCell>
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
