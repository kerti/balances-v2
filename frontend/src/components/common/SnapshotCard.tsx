import type { UseMutationResult } from "@tanstack/react-query";
import { Card, CardContent } from "@/components/ui/card";
import { useTranslation } from "react-i18next";
import { CardField, CardFields } from "@/components/common/CardFields";
import { SnapshotRowMenu } from "@/components/common/SnapshotRowMenu";
import { useSnapshotRow, type SnapshotLike } from "@/components/common/useSnapshotRow";
import type { UpdateSnapshotMutationVariables } from "@/components/dialogs/EditSnapshotDialog";

type Props<TUpdate, TDelete> = {
  snapshot: SnapshotLike;
  // The amount column's name on the desktop table (Balance, Value, Owed…).
  amountLabel: string;
  updateMutation: UseMutationResult<TUpdate, unknown, UpdateSnapshotMutationVariables>;
  deleteMutation: UseMutationResult<TDelete, unknown, string>;
};

// Mobile leaf renderer for an amount-only snapshot (ADR-0051 Phase B, "wide
// table → stacked cards"): labelled key/value rows in the desktop column order
// (month first, then the amount, then the note), the card grammar of #849. Shares
// state, dialogs and the ⋮ menu with the desktop `SnapshotRow` via
// `useSnapshotRow`, and the same `snapshot-*` testids, so the two renderers
// stay in lockstep. The ⋮ trigger sizes to the 44px tap floor
// (INV-PRESENTATION-08).
export function SnapshotCard<TUpdate, TDelete>({
  snapshot,
  amountLabel,
  updateMutation,
  deleteMutation,
}: Props<TUpdate, TDelete>) {
  const { t } = useTranslation("common");
  const { monthText, amountText, statementText, description, onEdit, onDelete, dialogs } =
    useSnapshotRow(snapshot, updateMutation, deleteMutation);

  return (
    <>
      <Card data-testid="snapshot-row" className="py-0">
        <CardContent className="space-y-2 p-4">
          <div className="flex items-start gap-2">
            <CardFields className="flex-1">
              <CardField label={t("tableHeaders.month")}>
                <div className="font-medium">{monthText}</div>
                {statementText && (
                  <div className="text-xs text-muted-foreground">{statementText}</div>
                )}
              </CardField>
              <CardField label={amountLabel} valueTestId="snapshot-amount">
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
              <CardField label={t("tableHeaders.notes")}>{description}</CardField>
            </CardFields>
          )}
        </CardContent>
      </Card>

      {dialogs}
    </>
  );
}
