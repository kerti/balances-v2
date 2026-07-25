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

  // Principal and accrued each get their own line (label left, value right) —
  // either leg alone still reads, and neither renders nothing.
  const legs = [
    principalText && {
      label: t("investments:snapshotsCard.principalHeader"),
      value: principalText,
    },
    accruedText && { label: t("investments:snapshotsCard.accruedHeader"), value: accruedText },
  ].filter((leg): leg is { label: string; value: string } => Boolean(leg));

  return (
    <>
      <Card data-testid="snapshot-row">
        <CardContent className="space-y-2 p-4">
          <div className="flex items-start gap-2">
            <div className="flex-1 space-y-0.5">
              <div className="text-lg font-semibold tabular-nums" data-testid="snapshot-amount">
                {amountText}
              </div>
              {legs.map((leg) => (
                <div
                  key={leg.label}
                  className="flex justify-between gap-2 text-sm text-muted-foreground"
                >
                  <span>{leg.label}</span>
                  <span className="tabular-nums">{leg.value}</span>
                </div>
              ))}
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
