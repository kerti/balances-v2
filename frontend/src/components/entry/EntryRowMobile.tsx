import { RotateCcw } from "lucide-react";
import { useTranslation } from "react-i18next";
import { NumericInput } from "@/components/common/NumericInput";
import { Label } from "@/components/ui/label";
import { Button } from "@/components/ui/button";
import {
  EntryRowMeta,
  RowSameCheckbox,
  type EntryRowRendererProps,
} from "@/components/entry/entryRow";

// Mobile bulk-entry row (ADR-0050 "cramped horizontal input row → stacked
// fields"): the name/ownership/when block on top, then each tab-stop on its own
// full-width line, then a footer with the computed value and the row actions.
// Inputs and the reset control are ≥44px tall (the a11y floor: tap targets
// ≥44px, no horizontal scroll to read the value, focus order = visual order).
// Same testids as EntryRowDesktop so the entry specs assert either renderer.
export function EntryRowMobile({
  view,
  shape,
  tid,
  copy,
  onFieldChange,
  onReset,
  onUnchanged,
}: EntryRowRendererProps) {
  const { t } = useTranslation("common");
  const { dirty, positionId } = view;
  const touched = dirty || view.unchanged;
  // Two inputs (quantity | price, principal | accrued) share one line; the
  // row's undo / Same controls then move up beside the currency (#849).
  const sideBySide = shape.fields.length === 2;
  const rowActions = (
    <>
      {touched && (
        <Button
          type="button"
          variant="ghost"
          size="icon"
          className="size-11 shrink-0"
          onClick={onReset}
          aria-label={t("bulkEntry.undo")}
          title={t("bulkEntry.undo")}
          data-testid={`${tid}-entry-undo-${positionId}`}
        >
          <RotateCcw className="size-4" />
        </Button>
      )}
      {view.when.kind === "carried" && !view.dirty && (
        <label className="flex h-11 min-w-11 shrink-0 cursor-pointer flex-col items-center justify-center gap-0.5 px-1 text-[10px] leading-none text-muted-foreground">
          <RowSameCheckbox view={view} tid={tid} onUnchanged={onUnchanged} />
          {t("bulkEntry.same")}
        </label>
      )}
    </>
  );
  return (
    <li className="space-y-2 py-3" data-testid={`${tid}-entry-row-${positionId}`}>
      <div className="flex items-start justify-between gap-2">
        <div className="min-w-0">
          <EntryRowMeta view={view} tid={tid} copy={copy} />
        </div>
        <div className="flex shrink-0 items-start gap-1">
          <span className="text-xs text-muted-foreground">{view.currency}</span>
          {sideBySide && rowActions}
        </div>
      </div>

      <div className={sideBySide ? "grid grid-cols-2 gap-2" : "space-y-2"}>
        {shape.fields.map((f, i) => {
          const inputId = `${tid}-entry-${f.testidSuffix}-${positionId}`;
          return (
            <div key={f.key} className="min-w-0 space-y-1">
              {f.labelKey && (
                <Label htmlFor={inputId} className="text-xs text-muted-foreground">
                  {t(f.labelKey)}
                </Label>
              )}
              <div className="flex items-center gap-2">
                <NumericInput
                  id={inputId}
                  className={`h-11 min-w-0 flex-1 text-right text-base${dirty ? " border-primary ring-1 ring-primary" : ""}`}
                  decimals={f.decimals(view.currency)}
                  maxIntegerDigits={f.maxIntegerDigits}
                  aria-label={f.labelKey ? t(f.labelKey) : undefined}
                  value={view.fieldValues[f.key] ?? ""}
                  onChange={(v) => onFieldChange(f.key, v)}
                  data-testid={inputId}
                />
                {!sideBySide && i === shape.fields.length - 1 && rowActions}
              </div>
            </div>
          );
        })}
      </div>

      {/*
       * Desktop labels the derived-total column in its group header; mobile
       * has no such header, so the footer carries the column's name inline —
       * the mobile twin of that header — so the figure reads as a named,
       * computed column ("Value 15.000" for qty×price, "Principal 3.000.000"
       * for accrued) rather than a stray number. The figure carries its currency
       * symbol like every other money total (INC-32); a bare "—" after the
       * label until the inputs complete.
       */}
      {view.derived !== null && (
        <span
          className="text-sm text-muted-foreground"
          data-testid={`${tid}-entry-value-${positionId}`}
        >
          {shape.derivedLabelKey && <span className="mr-1.5">{t(shape.derivedLabelKey)}</span>}
          <span>
            {view.derived.value === null ? "—" : `${view.derived.symbol} ${view.derived.value}`}
          </span>
        </span>
      )}
    </li>
  );
}
