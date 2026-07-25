import type { UseMutationResult } from "@tanstack/react-query";
import { TableCell, TableRow } from "@/components/ui/table";
import { SnapshotRowMenu } from "@/components/common/SnapshotRowMenu";
import {
  useQuantityPriceSnapshotRow,
  type QuantityPriceSnapshotLike,
} from "@/components/common/useQuantityPriceSnapshotRow";
import type { UpdateQuantityPriceSnapshotMutationVariables } from "@/components/dialogs/EditQuantityPriceSnapshotDialog";

type Props<TUpdate, TDelete> = {
  snapshot: QuantityPriceSnapshotLike;
  // Unit label is subtype-specific ("sh" for stocks, "units" for mutual
  // funds, "g" for gold). Passed in so this row stays subtype-agnostic.
  quantityUnit: string;
  updateMutation: UseMutationResult<TUpdate, unknown, UpdateQuantityPriceSnapshotMutationVariables>;
  deleteMutation: UseMutationResult<TDelete, unknown, string>;
};

// The desktop table row for a qty×price snapshot. Shares its state, dialogs and
// ⋮ menu with the mobile `QuantityPriceSnapshotCard` via
// `useQuantityPriceSnapshotRow`, and the same `snapshot-*` testids, so the two
// renderers stay in lockstep (ADR-0051 Phase B).
export function QuantityPriceSnapshotRow<TUpdate, TDelete>({
  snapshot,
  quantityUnit,
  updateMutation,
  deleteMutation,
}: Props<TUpdate, TDelete>) {
  const {
    monthText,
    amountText,
    quantityText,
    priceText,
    statementText,
    description,
    onEdit,
    onDelete,
    dialogs,
  } = useQuantityPriceSnapshotRow(snapshot, quantityUnit, updateMutation, deleteMutation);

  return (
    <>
      <TableRow data-testid="snapshot-row">
        <TableCell>
          <div className="font-medium">{monthText}</div>
          {statementText && <div className="text-xs text-muted-foreground">{statementText}</div>}
        </TableCell>
        <TableCell className="text-right tabular-nums">{quantityText ?? "—"}</TableCell>
        <TableCell className="text-right tabular-nums">{priceText ?? "—"}</TableCell>
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
