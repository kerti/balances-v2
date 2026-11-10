import { useTranslation } from "react-i18next";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { MonthPickerPopover } from "@/components/common/MonthPickerPopover";
import { monthEndDateCapped, monthStartDate, yearMonthsUpToNow } from "@/lib/dateLimits";
import { formatYearMonth } from "@/lib/format";
import type { FormMode } from "./mode";

type Props = {
  mode: FormMode;
  /** Prefix for the field ids, e.g. "inv" gives inv_year_month / inv_as_of_date. */
  idPrefix: string;
  yearMonth: string;
  onYearMonthChange: (next: string) => void;
  asOfDate: string;
  onAsOfDateChange: (next: string) => void;
};

// The month + statement-date row every snapshot form opens with. Create picks
// the month; Edit shows it locked in the same slot (a snapshot cannot move
// months — it would collide with the (position, month) unique key — so the
// hint points at delete + re-record).
export function SnapshotMonthRow({
  mode,
  idPrefix,
  yearMonth,
  onYearMonthChange,
  asOfDate,
  onAsOfDateChange,
}: Props) {
  const { t } = useTranslation("common");
  const monthId = `${idPrefix}year_month`;
  return (
    <>
      <div className="grid grid-cols-2 gap-3 [&>*]:content-end">
        <div className="grid gap-2">
          <Label htmlFor={monthId}>{t("fields.month")}</Label>
          {mode === "create" ? (
            <MonthPickerPopover
              id={monthId}
              testId={monthId}
              months={yearMonthsUpToNow()}
              selected={yearMonth}
              onSelect={onYearMonthChange}
            />
          ) : (
            <Input
              id={monthId}
              data-testid="snapshot-month-locked"
              disabled
              value={formatYearMonth(yearMonth)}
            />
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
      </div>
      {mode === "edit" && (
        <p className="text-xs text-muted-foreground">{t("snapshot.wrongMonthHint")}</p>
      )}
    </>
  );
}
