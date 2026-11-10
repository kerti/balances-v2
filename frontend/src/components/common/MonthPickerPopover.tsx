import { useState } from "react";
import { useTranslation } from "react-i18next";
import { ChevronDown, ChevronLeft, ChevronRight } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { FieldError, describedBy, useFieldError, useFormErrors } from "@/components/ui/field-error";
import { cn } from "@/lib/utils";
import { formatShortMonth, formatYearMonth } from "@/lib/format";

// MonthPickerPopover replaces a flat 120+-option <select> for a month picker
// (dashboard net-worth report, income filter). Trigger shows the current
// month; popover shows a year nav (clamped to the [min, max] year span of
// `months`) plus a 4×3 month grid with cells disabled for months without an
// entry. Selecting a cell fires onSelect with the exact string it was handed
// in `months` — the caller keeps whatever key shape it uses (full ISO
// `year_month` for the dashboard, `"YYYY-MM"` for income and for form fields
// fed by `yearMonthsUpToNow`).

const MONTH_INDEXES = Array.from({ length: 12 }, (_, i) => i);

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
  id,
  testId = "month-picker-trigger",
  placeholder,
  fullWidth = false,
}: {
  months: string[];
  selected: string;
  onSelect: (yearMonth: string) => void;
  /** Lets a form <Label htmlFor> name the trigger. */
  id?: string;
  /** Overrides the trigger's data-testid so several pickers can share a page. */
  testId?: string;
  /** Trigger text while `selected` is empty (a form field not yet filled). */
  placeholder?: string;
  /**
   * Fill the container: the group takes the full row, the trigger grows and the
   * arrows keep their size. Forms use it (#865); toolbars keep the default
   * fixed-width group.
   */
  fullWidth?: boolean;
}) {
  const { t } = useTranslation(["dashboard", "common"]);
  const { message, invalid } = useFieldError(id);
  const formErrors = useFormErrors();
  // Key-by-month lookup so the cell click fires with the exact string the
  // caller handed in, not a re-synthesised one. Safer if the backend ever
  // changes the day/time component.
  const isoByKey = new Map(months.map((m) => [ymKey(m), m]));
  const years = Array.from(new Set(months.map(yearOf))).sort((a, b) => a - b);
  const minYear = years[0];
  const maxYear = years[years.length - 1];

  const hasSelection = selected !== "";
  const selectedYear = hasSelection ? yearOf(selected) : maxYear;
  const selectedMonthIdx = hasSelection ? monthIdxOf(selected) : -1;

  // Prev/next walk the sorted *available* months, not calendar months — a
  // multi-year gap in `months` collapses to a single click since the arrows
  // jump to the nearest neighbour in this list, never into an empty month.
  const sortedKeys = Array.from(isoByKey.keys()).sort();
  const selectedPos = sortedKeys.indexOf(ymKey(selected));
  const prevIso = selectedPos > 0 ? isoByKey.get(sortedKeys[selectedPos - 1]) : undefined;
  const nextIso =
    selectedPos >= 0 && selectedPos < sortedKeys.length - 1
      ? isoByKey.get(sortedKeys[selectedPos + 1])
      : undefined;

  // Format the trigger label from the UTC parts via a canonical noon-UTC ISO
  // so a local-timezone rollover never shifts the displayed month (the raw
  // `selected` may be a bare "YYYY-MM" or a midnight-Z ISO).
  const selectedLabel = hasSelection
    ? formatYearMonth(
        `${selectedYear}-${String(selectedMonthIdx + 1).padStart(2, "0")}-01T12:00:00Z`,
      )
    : (placeholder ?? "");

  const [open, setOpen] = useState(false);
  const [viewYear, setViewYear] = useState(selectedYear);

  // Reset the year nav to the currently-selected year each time the popover
  // opens — otherwise re-opening shows whatever year the user last browsed,
  // which is confusing when the selection didn't change.
  function handleOpenChange(next: boolean) {
    if (next) setViewYear(selectedYear);
    setOpen(next);
  }

  // A pick is a field change the form can't see as a DOM event: it retires a
  // server error that was about this field (e.g. SNAPSHOT_MONTH_EXISTS).
  function select(iso: string) {
    formErrors?.dismissServerError();
    onSelect(iso);
  }

  return (
    <>
      <div className={cn("items-center -space-x-px", fullWidth ? "flex w-full" : "inline-flex")}>
        <Button
          variant="ghost"
          size="icon-lg"
          type="button"
          data-testid="month-picker-prev"
          disabled={!prevIso}
          onClick={() => prevIso && select(prevIso)}
          aria-label={t("monthPicker.prevMonth")}
          className="relative size-11 shrink-0 rounded-r-none border-border focus-visible:z-10 dark:border-input md:size-9"
        >
          <ChevronLeft className="size-4" />
        </Button>
        <Popover open={open} onOpenChange={handleOpenChange}>
          <PopoverTrigger asChild>
            <Button
              id={id}
              variant="outline"
              size="lg"
              data-testid={testId}
              aria-invalid={invalid || undefined}
              aria-describedby={describedBy(undefined, id, invalid)}
              className={cn(
                "relative h-11 justify-between gap-2 rounded-none focus-visible:z-10 md:h-9",
                fullWidth ? "min-w-0 flex-1" : "min-w-40",
              )}
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
              <span className="text-sm font-medium" data-testid="month-picker-year-label">
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
              {MONTH_INDEXES.map((idx) => {
                const key = `${viewYear}-${String(idx + 1).padStart(2, "0")}`;
                const iso = isoByKey.get(key);
                const disabled = !iso;
                const isSelected = viewYear === selectedYear && idx === selectedMonthIdx;
                return (
                  <Button
                    key={key}
                    variant={isSelected ? "default" : "ghost"}
                    size="sm"
                    data-testid={`month-picker-cell-${key}`}
                    disabled={disabled}
                    onClick={() => {
                      if (!iso) return;
                      select(iso);
                      setOpen(false);
                    }}
                    className={cn("h-8", disabled && "opacity-40")}
                  >
                    {formatShortMonth(new Date(viewYear, idx, 1))}
                  </Button>
                );
              })}
            </div>
          </PopoverContent>
        </Popover>
        <Button
          variant="ghost"
          size="icon-lg"
          type="button"
          data-testid="month-picker-next"
          disabled={!nextIso}
          onClick={() => nextIso && select(nextIso)}
          aria-label={t("monthPicker.nextMonth")}
          className="relative size-11 shrink-0 rounded-l-none border-border focus-visible:z-10 dark:border-input md:size-9"
        >
          <ChevronRight className="size-4" />
        </Button>
      </div>
      {message && id && <FieldError id={id}>{message}</FieldError>}
    </>
  );
}
