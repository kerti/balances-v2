import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Callout } from "@/components/common/Callout";
import { MonthPickerPopover } from "@/components/common/MonthPickerPopover";
import { monthEndDateCapped, monthStartDate, yearMonthsUpToNow } from "@/lib/dateLimits";
import { formatYearMonth } from "@/lib/format";
import { formatRecordMonth } from "@/lib/recordSnapshot";
import type { FormMode } from "./mode";
import { LockedFieldHint } from "./LockedFieldHint";

type Props = {
  mode: FormMode;
  /** Prefix for the field ids, e.g. "inv" gives inv_year_month / inv_as_of_date. */
  idPrefix: string;
  yearMonth: string;
  onYearMonthChange: (next: string) => void;
  asOfDate: string;
  onAsOfDateChange: (next: string) => void;
  /** Create only: the existing snapshot's total, formatted, when the picked
   *  month already has one. Shows the replace notice under the picker. */
  replacing?: string;
  /** Edit only: opens the Move dialog (#791). Omitted when the row cannot be moved. */
  onMove?: () => void;
};

// The month + statement-date row every snapshot form opens with. Create picks
// the month; Edit shows it locked in the same slot — `year_month` is immutable
// (#76) — and offers Move… beside it, one atomic act (#791).
export function SnapshotMonthRow({
  mode,
  idPrefix,
  yearMonth,
  onYearMonthChange,
  asOfDate,
  onAsOfDateChange,
  replacing,
  onMove,
}: Props) {
  const { t } = useTranslation("common");
  const monthId = `${idPrefix}year_month`;
  return (
    <>
      {/* Always stacked, never side by side (#865): the picker group is
          wider than half a dialog at every width and ran over the date. */}
      <div className="grid gap-2">
        <Label htmlFor={monthId}>{t("fields.month")}</Label>
        {mode === "create" ? (
          <MonthPickerPopover
            fullWidth
            id={monthId}
            testId={monthId}
            months={yearMonthsUpToNow()}
            selected={yearMonth}
            onSelect={onYearMonthChange}
          />
        ) : (
          <div className="flex items-center gap-2">
            <Input
              id={monthId}
              data-testid="snapshot-month-locked"
              disabled
              value={formatYearMonth(yearMonth)}
            />
            {onMove && (
              <Button
                type="button"
                variant="outline"
                className="h-11 shrink-0 md:h-9"
                data-testid="snapshot-move-open"
                onClick={onMove}
              >
                {t("snapshot.moveTrigger")}
              </Button>
            )}
          </div>
        )}
        <LockedFieldHint mode={mode} id="snapshot-month" />
        {mode === "create" && replacing !== undefined && (
          <Callout data-testid="snapshot-replace-notice">
            {t("snapshot.replaceNotice", { month: formatRecordMonth(yearMonth), value: replacing })}
          </Callout>
        )}
      </div>
      <div className="grid gap-2">
        <Label htmlFor={`${idPrefix}as_of_date`}>{t("fields.statementDate")}</Label>
        <Input
          id={`${idPrefix}as_of_date`}
          type="date"
          min={monthStartDate(yearMonth)}
          max={monthEndDateCapped(yearMonth)}
          value={asOfDate}
          onChange={(e) => onAsOfDateChange(e.target.value)}
        />
      </div>
    </>
  );
}
