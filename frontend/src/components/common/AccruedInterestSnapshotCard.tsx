import type { UseMutationResult } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { CompactSnapshotRow } from "@/components/common/CompactSnapshotRow";
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

// Mobile leaf renderer for an accrued snapshot: a compact `month · total · ⋯`
// row (#952, the explicit exception to #849's labelled cards) that expands on tap
// to principal and accrued. Shares state, dialogs and the ⋮ menu with the desktop
// `AccruedInterestSnapshotRow` via `useAccruedInterestSnapshotRow`. The ⋮ trigger
// sizes to the 44px tap floor (INV-PRESENTATION-08).
export function AccruedInterestSnapshotCard<TUpdate, TDelete>({
  snapshot,
  updateMutation,
  deleteMutation,
  principal,
}: Props<TUpdate, TDelete>) {
  const { t } = useTranslation("investments");
  const {
    monthText,
    amountText,
    statementText,
    principalText,
    accruedText,
    description,
    onEdit,
    onDelete,
    dialogs,
  } = useAccruedInterestSnapshotRow(snapshot, updateMutation, deleteMutation, principal);

  const breakdown = [
    ...(principalText ? [{ label: t("snapshotsCard.principalHeader"), value: principalText }] : []),
    ...(accruedText ? [{ label: t("snapshotsCard.accruedHeader"), value: accruedText }] : []),
  ];

  return (
    <>
      <CompactSnapshotRow
        monthText={monthText}
        amountText={amountText}
        statement={statementText}
        note={description}
        breakdown={breakdown}
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
