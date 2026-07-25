import type { UseMutationResult } from "@tanstack/react-query";
import { Card, CardContent } from "@/components/ui/card";
import { SnapshotRowMenu } from "@/components/common/SnapshotRowMenu";
import { useSnapshotRow, type SnapshotLike } from "@/components/common/useSnapshotRow";
import type { UpdateSnapshotMutationVariables } from "@/components/dialogs/EditSnapshotDialog";

type Props<TUpdate, TDelete> = {
  snapshot: SnapshotLike;
  updateMutation: UseMutationResult<TUpdate, unknown, UpdateSnapshotMutationVariables>;
  deleteMutation: UseMutationResult<TDelete, unknown, string>;
};

// Mobile leaf renderer for an amount-only snapshot (ADR-0051 Phase B, "wide
// table → stacked cards"): the amount is promoted to the card headline so the
// value the household member came for reads with no horizontal scroll, with the
// month + statement date below it and the note as a full-width line. Shares
// state, dialogs and the ⋮ menu with the desktop `SnapshotRow` via
// `useSnapshotRow`, and the same `snapshot-*` testids, so the two renderers
// stay in lockstep. The ⋮ trigger sizes to the 44px tap floor
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
      <Card data-testid="snapshot-row">
        <CardContent className="space-y-2 p-4">
          <div className="flex items-start gap-2">
            <div className="flex-1 space-y-0.5">
              <div className="text-lg font-semibold tabular-nums" data-testid="snapshot-amount">
                {amountText}
              </div>
              <div className="text-sm text-muted-foreground">{monthText}</div>
              {statementText && (
                <div className="text-xs text-muted-foreground">{statementText}</div>
              )}
            </div>
            <SnapshotRowMenu
              onEdit={onEdit}
              onDelete={onDelete}
              variant="outline"
              triggerClassName="size-11 shrink-0"
            />
          </div>
          {description && <p className="text-sm">{description}</p>}
        </CardContent>
      </Card>

      {dialogs}
    </>
  );
}
