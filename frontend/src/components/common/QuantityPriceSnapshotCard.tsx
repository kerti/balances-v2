import type { UseMutationResult } from "@tanstack/react-query";
import { Card, CardContent } from "@/components/ui/card";
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
// stacked cards"): the position value is promoted to the card headline so the
// figure the household member came for reads with no horizontal scroll, with the
// quantity × unit-price as a secondary line, the month + statement date below,
// and the note as a full-width line. Shares state, dialogs and the ⋮ menu with
// the desktop `QuantityPriceSnapshotRow` via `useQuantityPriceSnapshotRow`, and
// the same `snapshot-*` testids, so the two renderers stay in lockstep. The ⋮
// trigger sizes to the 44px tap floor (INV-PRESENTATION-08).
export function QuantityPriceSnapshotCard<TUpdate, TDelete>({
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

  // "100 sh × $8,500" when both legs are present; a single leg alone still reads
  // (a snapshot may carry only one), and neither renders nothing.
  const perUnitText =
    quantityText && priceText
      ? `${quantityText} × ${priceText}`
      : (quantityText ?? priceText ?? null);

  return (
    <>
      <Card data-testid="snapshot-row">
        <CardContent className="space-y-2 p-4">
          <div className="flex items-start gap-2">
            <div className="flex-1 space-y-0.5">
              <div className="text-lg font-semibold tabular-nums" data-testid="snapshot-amount">
                {amountText}
              </div>
              {perUnitText && (
                <div className="text-sm tabular-nums text-muted-foreground">{perUnitText}</div>
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
