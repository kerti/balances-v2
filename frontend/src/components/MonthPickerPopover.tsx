import { useState } from "react";
import { useTranslation } from "react-i18next";
import { ChevronDown, ChevronLeft, ChevronRight } from "lucide-react";

import { Button } from "@/components/ui/button";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import { cn } from "@/lib/utils";
import { formatYearMonth } from "@/lib/format";

// MonthPickerPopover replaces a flat 120+-option <select> for a month picker
// (dashboard net-worth report, income filter). Trigger shows the current
// month; popover shows a year nav (clamped to the [min, max] year span of
// `months`) plus a 4×3 month grid with cells disabled for months without an
// entry. Selecting a cell fires onSelect with the exact string it was handed
// in `months` — the caller keeps whatever key shape it uses (full ISO
// `year_month` for the dashboard, `"YYYY-MM"` for income).

// Month-label keys index into common.months.{jan…dec}. Order is fixed Jan→Dec
// (calendar order), independent of locale.
const MONTH_KEYS = [
  "jan",
  "feb",
  "mar",
  "apr",
  "may",
  "jun",
  "jul",
  "aug",
  "sep",
  "oct",
  "nov",
  "dec",
] as const;

// year_month is stored as "YYYY-MM-01T00:00:00Z" — UTC midnight. Use UTC
// getters so local-timezone rollover never shifts the displayed month.
function ymKey(iso: string): string {
  const d = new Date(iso);
  return `${d.getUTCFullYear()}-${String(d.getUTCMonth() + 1).padStart(2, "0")}`;
}

function yearOf(iso: string): number {
  return new Date(iso).getUTCFullYear();
}

function monthIdxOf(iso: string): number {
  return new Date(iso).getUTCMonth();
}

export function MonthPickerPopover({
  months,
  selected,
  onSelect,
}: {
  months: string[];
  selected: string;
  onSelect: (yearMonth: string) => void;
}) {
  const { t } = useTranslation(["dashboard", "common"]);
  // Key-by-month lookup so the cell click fires with the exact string the
  // caller handed in, not a re-synthesised one. Safer if the backend ever
  // changes the day/time component.
  const isoByKey = new Map(months.map((m) => [ymKey(m), m]));
  const years = Array.from(new Set(months.map(yearOf))).sort((a, b) => a - b);
  const minYear = years[0];
  const maxYear = years[years.length - 1];

  const selectedYear = yearOf(selected);
  const selectedMonthIdx = monthIdxOf(selected);

  // Format the trigger label from the UTC parts via a canonical noon-UTC ISO
  // so a local-timezone rollover never shifts the displayed month (the raw
  // `selected` may be a bare "YYYY-MM" or a midnight-Z ISO).
  const selectedLabel = formatYearMonth(
    `${selectedYear}-${String(selectedMonthIdx + 1).padStart(2, "0")}-01T12:00:00Z`,
  );

  const [open, setOpen] = useState(false);
  const [viewYear, setViewYear] = useState(selectedYear);

  // Reset the year nav to the currently-selected year each time the popover
  // opens — otherwise re-opening shows whatever year the user last browsed,
  // which is confusing when the selection didn't change.
  function handleOpenChange(next: boolean) {
    if (next) setViewYear(selectedYear);
    setOpen(next);
  }

  return (
    <Popover open={open} onOpenChange={handleOpenChange}>
      <PopoverTrigger asChild>
        <Button
          variant="outline"
          size="lg"
          data-testid="month-picker-trigger"
          className="justify-between gap-2"
        >
          {selectedLabel}
          <ChevronDown className="size-4 opacity-60" />
        </Button>
      </PopoverTrigger>
      <PopoverContent className="w-64" data-testid="month-picker-content">
        <div className="mb-2 flex items-center justify-between">
          <Button
            variant="ghost"
            size="icon-sm"
            data-testid="month-picker-year-prev"
            disabled={viewYear <= minYear}
            onClick={() => setViewYear((y) => Math.max(minYear, y - 1))}
            aria-label={t("monthPicker.prevYear")}
          >
            <ChevronLeft className="size-4" />
          </Button>
          <span
            className="text-sm font-medium"
            data-testid="month-picker-year-label"
          >
            {viewYear}
          </span>
          <Button
            variant="ghost"
            size="icon-sm"
            data-testid="month-picker-year-next"
            disabled={viewYear >= maxYear}
            onClick={() => setViewYear((y) => Math.min(maxYear, y + 1))}
            aria-label={t("monthPicker.nextYear")}
          >
            <ChevronRight className="size-4" />
          </Button>
        </div>
        <div className="grid grid-cols-4 gap-1">
          {MONTH_KEYS.map((monthKey, idx) => {
            const key = `${viewYear}-${String(idx + 1).padStart(2, "0")}`;
            const iso = isoByKey.get(key);
            const disabled = !iso;
            const isSelected =
              viewYear === selectedYear && idx === selectedMonthIdx;
            return (
              <Button
                key={key}
                variant={isSelected ? "default" : "ghost"}
                size="sm"
                data-testid={`month-picker-cell-${key}`}
                disabled={disabled}
                onClick={() => {
                  if (!iso) return;
                  onSelect(iso);
                  setOpen(false);
                }}
                className={cn("h-8", disabled && "opacity-40")}
              >
                {t(`common:months.${monthKey}`)}
              </Button>
            );
          })}
        </div>
      </PopoverContent>
    </Popover>
  );
}
