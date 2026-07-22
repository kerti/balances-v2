import { RotateCcw } from "lucide-react";
import { useTranslation } from "react-i18next";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Button } from "@/components/ui/button";
import { EntryRowMeta, type EntryRowRendererProps } from "@/components/entry/entryRow";

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
}: EntryRowRendererProps) {
  const { t } = useTranslation("common");
  const { dirty, positionId } = view;
  return (
    <li className="space-y-2 py-3" data-testid={`${tid}-entry-row-${positionId}`}>
      <div className="flex items-start justify-between gap-2">
        <div className="min-w-0">
          <EntryRowMeta view={view} tid={tid} copy={copy} />
        </div>
        <span className="shrink-0 text-xs text-muted-foreground">{view.currency}</span>
      </div>

      {shape.fields.map((f) => {
        const inputId = `${tid}-entry-${f.testidSuffix}-${positionId}`;
        return (
          <div key={f.key} className="space-y-1">
            {f.labelKey && (
              <Label htmlFor={inputId} className="text-xs text-muted-foreground">
                {t(f.labelKey)}
              </Label>
            )}
            <Input
              id={inputId}
              className={`h-11 w-full text-right text-base${dirty ? " border-amber-500 ring-1 ring-amber-500" : ""}`}
              inputMode="decimal"
              aria-label={f.labelKey ? t(f.labelKey) : undefined}
              value={view.fieldValues[f.key] ?? ""}
              onChange={(e) => onFieldChange(f.key, e.target.value)}
              data-testid={inputId}
            />
          </div>
        );
      })}

      <div className="flex items-center justify-between gap-2">
        {view.derived !== null ? (
          // Desktop labels the derived-total column in its group header; mobile
          // has no such header, so the footer carries the column's name inline —
          // the mobile twin of that header — so the figure reads as a named,
          // computed column ("Value 15.000" for qty×price, "Principal 3.000.000"
          // for accrued) rather than a stray number. Currency sits at the card's
          // top-right; a bare "—" after the label until the inputs complete.
          <span
            className="text-sm text-muted-foreground"
            data-testid={`${tid}-entry-value-${positionId}`}
          >
            {shape.derivedLabelKey && <span className="mr-1.5">{t(shape.derivedLabelKey)}</span>}
            <span className="tabular-nums">{view.derived.value ?? "—"}</span>
          </span>
        ) : (
          <span />
        )}
        <Button
          type="button"
          variant="ghost"
          size="icon"
          className={`size-11 shrink-0${dirty ? "" : " invisible"}`}
          onClick={onReset}
          aria-label={t("bulkEntry.undo")}
          title={t("bulkEntry.undo")}
          disabled={!dirty}
          data-testid={`${tid}-entry-undo-${positionId}`}
        >
          <RotateCcw className="size-4" />
        </Button>
      </div>
    </li>
  );
}
