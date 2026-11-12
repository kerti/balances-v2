import type { UseMutationResult } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { Card, CardContent } from "@/components/ui/card";
import { CardField, CardFields } from "@/components/common/CardFields";
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
  // A time deposit's principal in force (#844); bonds pass nothing.
  principal?: string;
};

// Mobile leaf renderer for an accrued snapshot (ADR-0051 Phase B, "wide table →
// stacked cards"): labelled key/value rows in the desktop column order (month,
// principal, accrued, total value, note), the card grammar of #849. Shares state, dialogs and the ⋮
// menu with the desktop `AccruedInterestSnapshotRow` via
// `useAccruedInterestSnapshotRow`, and the same `snapshot-*` testids, so the two
// renderers stay in lockstep. The ⋮ trigger sizes to the 44px tap floor
// (INV-PRESENTATION-08).
export function AccruedInterestSnapshotCard<TUpdate, TDelete>({
  snapshot,
  updateMutation,
  deleteMutation,
  principal,
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
  } = useAccruedInterestSnapshotRow(snapshot, updateMutation, deleteMutation, principal);

  return (
    <>
      <Card data-testid="snapshot-row" className="py-0">
        <CardContent className="space-y-2 p-4">
          <div className="flex items-start gap-2">
            <CardFields className="flex-1">
              <CardField label={t("investments:snapshotsCard.monthHeader")}>
                <div className="font-medium">{monthText}</div>
                {statementText && (
                  <div className="text-xs text-muted-foreground">{statementText}</div>
                )}
              </CardField>
              {principalText && (
                <CardField label={t("investments:snapshotsCard.principalHeader")}>
                  {principalText}
                </CardField>
              )}
              {accruedText && (
                <CardField label={t("investments:snapshotsCard.accruedHeader")}>
                  {accruedText}
                </CardField>
              )}
              <CardField
                label={t("investments:snapshotsCard.totalValueHeader")}
                valueTestId="snapshot-amount"
              >
                <span className="font-semibold">{amountText}</span>
              </CardField>
            </CardFields>
            <SnapshotRowMenu
              onEdit={onEdit}
              onDelete={onDelete}
              variant="outline"
              triggerClassName="size-11 shrink-0"
            />
          </div>
          {description && (
            <CardFields>
              <CardField label={t("investments:snapshotsCard.notesHeader")}>
                {description}
              </CardField>
            </CardFields>
          )}
        </CardContent>
      </Card>

      {dialogs}
    </>
  );
}
