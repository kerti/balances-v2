import type { UseMutationResult } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { Card, CardContent } from "@/components/ui/card";
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

// Mobile leaf renderer for an accrued snapshot (ADR-0051 Phase B, "wide table →
// stacked cards"): the total value is promoted to the card headline so the
// figure the household member came for reads with no horizontal scroll, with the
// principal and accrued-interest split as a secondary line, the month + statement
// date below, and the note as a full-width line. Shares state, dialogs and the ⋮
// menu with the desktop `AccruedInterestSnapshotRow` via
// `useAccruedInterestSnapshotRow`, and the same `snapshot-*` testids, so the two
// renderers stay in lockstep. The ⋮ trigger sizes to the 44px tap floor
// (INV-PRESENTATION-08).
export function AccruedInterestSnapshotCard<TUpdate, TDelete>({
  snapshot,
  updateMutation,
  deleteMutation,
}: Props<TUpdate, TDelete>) {
  const { t } = useTranslation(["investments"]);
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

  // "Principal $9,500 · Accrued $500" when both legs are present; either leg
  // alone still reads, and neither renders nothing.
  const legs = [
    principalText && `${t("investments:snapshotsCard.principalHeader")} ${principalText}`,
    accruedText && `${t("investments:snapshotsCard.accruedHeader")} ${accruedText}`,
  ].filter(Boolean);
  const splitText = legs.length > 0 ? legs.join(" · ") : null;

  return (
    <>
      <Card data-testid="snapshot-row">
        <CardContent className="space-y-2 p-4">
          <div className="flex items-start gap-2">
            <div className="flex-1 space-y-0.5">
              <div className="text-lg font-semibold tabular-nums" data-testid="snapshot-amount">
                {amountText}
              </div>
              {splitText && (
                <div className="text-sm tabular-nums text-muted-foreground">{splitText}</div>
              )}
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
