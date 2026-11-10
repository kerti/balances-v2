import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";

export type LastRecorded = {
  // The previous snapshot's total, already formatted ("IDR 12,500,000").
  value: string;
  // The previous snapshot's month, already formatted ("Oct 2026").
  month: string;
  // Fills the value fields (never the statement date) from that snapshot.
  onApply: () => void;
};

// The muted "Last recorded: IDR … (Oct 2026)" line and its "Same as last month"
// button, shown under the value fields of an empty month. The fields start
// empty (ADR-0046 amendment): nothing is prefilled until the user asks.
export function LastRecordedHint({ value, month, onApply }: LastRecorded) {
  const { t } = useTranslation("common");
  return (
    <div
      className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground"
      data-testid="snapshot-last-recorded"
    >
      <span>{t("snapshot.lastRecorded", { value, month })}</span>
      <Button
        type="button"
        variant="outline"
        size="sm"
        onClick={onApply}
        data-testid="snapshot-same-as-last"
      >
        {t("snapshot.sameAsLast")}
      </Button>
    </div>
  );
}
