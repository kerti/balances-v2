import type { UseMutationResult } from "@tanstack/react-query";
import { Card, CardContent } from "@/components/ui/card";
import { useTranslation } from "react-i18next";
import { CardField, CardFields } from "@/components/common/CardFields";
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

// Mobile leaf renderer for a qty×price snapshot (ADR-0051 Phase B, "wide table →
// stacked cards"): labelled key/value rows in the desktop column order (month,
// quantity, price, total value, note), the card grammar of #849. Shares state, dialogs and the ⋮ menu with
// the desktop `QuantityPriceSnapshotRow` via `useQuantityPriceSnapshotRow`, and
// the same `snapshot-*` testids, so the two renderers stay in lockstep. The ⋮
// trigger sizes to the 44px tap floor (INV-PRESENTATION-08).
export function QuantityPriceSnapshotCard<TUpdate, TDelete>({
  snapshot,
  quantityUnit,
  updateMutation,
  deleteMutation,
}: Props<TUpdate, TDelete>) {
  const { t } = useTranslation(["investments", "common"]);
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
              {quantityText && (
                <CardField label={t("investments:stock.quantityHeader")}>{quantityText}</CardField>
              )}
              {priceText && (
                <CardField label={t("investments:stock.priceHeader")}>{priceText}</CardField>
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
