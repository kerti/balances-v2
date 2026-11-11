import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Callout } from "@/components/common/Callout";
import { holdingMonthLabel } from "@/lib/recordSnapshot";
import { useRecordSnapshotRequest } from "@/lib/recordSnapshotRequest";

// The "latest snapshot quantity doesn't match the ledger total" warning above a
// qty x price position's transaction ledger. It carries the same
// "Update {Mon} holding →" action as a trade's toast (#830): it only opens Record
// snapshot prefilled with the ledger quantity; the snapshot stays the source of
// truth until the user saves it. `active` hides the action for a closed position,
// whose Record snapshot button is hidden too.
export function ReconcileBanner({
  message,
  snapshots,
  active,
}: {
  message: string;
  snapshots: Array<{ year_month: string }> | undefined;
  active: boolean;
}) {
  const { t } = useTranslation(["common"]);
  const { request, available } = useRecordSnapshotRequest();
  return (
    <Callout variant="warning" className="mx-6 mb-3" data-testid="reconcile-banner">
      <div className="flex flex-wrap items-center justify-between gap-x-4 gap-y-2">
        <span>{message}</span>
        {active && available && (
          <Button
            type="button"
            size="sm"
            variant="outline"
            className="h-11 md:h-8"
            data-testid="reconcile-update-holding"
            onClick={request}
          >
            {t("common:toast.updateHolding", { month: holdingMonthLabel(snapshots) })}
          </Button>
        )}
      </div>
    </Callout>
  );
}
