import type { UseMutationResult } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { CompactSnapshotRow } from "@/components/common/CompactSnapshotRow";
import { SnapshotRowMenu } from "@/components/common/SnapshotRowMenu";
import {
  useQuantityPriceSnapshotRow,
  type QuantityPriceSnapshotLike,
} from "@/components/common/useQuantityPriceSnapshotRow";
import type { UpdateQuantityPriceSnapshotMutationVariables } from "@/components/dialogs/EditQuantityPriceSnapshotDialog";

type Props<TUpdate, TDelete> = {
  snapshot: QuantityPriceSnapshotLike;
  // Unit label is subtype-specific ("sh" for stocks, "units" for mutual funds,
  // "g" for gold). Passed in so this card stays subtype-agnostic.
  quantityUnit: string;
  updateMutation: UseMutationResult<TUpdate, unknown, UpdateQuantityPriceSnapshotMutationVariables>;
  deleteMutation: UseMutationResult<TDelete, unknown, string>;
};

// Mobile leaf renderer for a qty×price snapshot: a compact `month · total · ⋯`
// row (#952, the explicit exception to #849's labelled cards) that expands on tap
// to the quantity and price. Shares state, dialogs and the ⋮ menu with the
// desktop `QuantityPriceSnapshotRow` via `useQuantityPriceSnapshotRow`. The ⋮
// trigger sizes to the 44px tap floor (INV-PRESENTATION-08).
export function QuantityPriceSnapshotCard<TUpdate, TDelete>({
  snapshot,
  quantityUnit,
  updateMutation,
  deleteMutation,
}: Props<TUpdate, TDelete>) {
  const { t } = useTranslation("investments");
  const {
    monthText,
    amountText,
    statementText,
    quantityText,
    priceText,
    description,
    onEdit,
    onDelete,
    dialogs,
  } = useQuantityPriceSnapshotRow(snapshot, quantityUnit, updateMutation, deleteMutation);

  const breakdown = [
    ...(quantityText ? [{ label: t("stock.quantityHeader"), value: quantityText }] : []),
    ...(priceText ? [{ label: t("stock.priceHeader"), value: priceText }] : []),
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
