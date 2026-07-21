import { RotateCcw } from "lucide-react";
import { useTranslation } from "react-i18next";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { EntryRowMeta, type EntryRowRendererProps } from "@/components/entry/entryRow";
import type { EntryShape } from "@/components/entry/shapes";

// Currency sits in a fixed-width column so multi-field rows line their inputs up
// under EntryRowDesktopHeader (a variable-width code would drift the columns).
const CURRENCY_COL = "w-10 shrink-0 text-right";

// EntryRowDesktopHeader labels the input columns once per group (ADR-0046
// Presentation / UX): with the shape's placeholders hidden the moment a value
// carries forward, a qty×price / accrued row otherwise gives no clue which field
// is units and which is price. It mirrors the row's right-hand cluster —
// name-spacer, currency-spacer, one label per field at its own widthClass, a
// value-spacer and an action-spacer — so the labels sit over their inputs.
// Amount-only shapes (a single unlabelled field) render no header. Decorative
// (aria-hidden): the inputs keep their own aria-labels.
export function EntryRowDesktopHeader({ shape }: { shape: EntryShape }) {
  const { t } = useTranslation("common");
  if (!shape.fields.some((f) => f.labelKey)) return null;
  return (
    <div
      className="flex items-center gap-3 pb-1 text-xs font-medium text-muted-foreground"
      aria-hidden
    >
      <div className="min-w-0 flex-1" />
      <span className={CURRENCY_COL} />
      <div className="flex items-center gap-2">
        {shape.fields.map((f) => (
          <span key={f.key} className={`${f.widthClass} truncate`}>
            {f.labelKey ? t(f.labelKey) : ""}
          </span>
        ))}
        {shape.derived && <span className="w-28 shrink-0" />}
      </div>
      <span className="size-8 shrink-0" />
    </div>
  );
}

// Desktop bulk-entry row (ADR-0046 original layout, ADR-0050 desktop renderer):
// name block on the left, the shape's field inputs + computed value in a cramped
// horizontal group on the right, the reset action trailing. Picked by
// EntryScreen when useIsMobile() is false; EntryRowMobile is its ≥stacked twin.
export function EntryRowDesktop({
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
    <li className="flex items-center gap-3 py-2" data-testid={`${tid}-entry-row-${positionId}`}>
      <div className="min-w-0 flex-1">
        <EntryRowMeta view={view} tid={tid} copy={copy} />
      </div>
      <span className={`${CURRENCY_COL} text-xs text-muted-foreground`}>{view.currency}</span>
      <div className="flex items-center gap-2">
        {shape.fields.map((f) => (
          <Input
            key={f.key}
            className={`${f.widthClass}${dirty ? " border-amber-500 ring-1 ring-amber-500" : ""}`}
            inputMode="decimal"
            aria-label={f.labelKey ? t(f.labelKey) : undefined}
            placeholder={f.labelKey ? t(f.labelKey) : undefined}
            value={view.fieldValues[f.key] ?? ""}
            onChange={(e) => onFieldChange(f.key, e.target.value)}
            data-testid={`${tid}-entry-${f.testidSuffix}-${positionId}`}
          />
        ))}
        {view.derived !== null && (
          <span
            className="w-28 shrink-0 text-right text-sm tabular-nums text-muted-foreground"
            data-testid={`${tid}-entry-value-${positionId}`}
          >
            {view.derived}
          </span>
        )}
      </div>
      <Button
        type="button"
        variant="ghost"
        size="icon"
        className={`size-8 shrink-0${dirty ? "" : " invisible"}`}
        onClick={onReset}
        aria-label={t("bulkEntry.undo")}
        title={t("bulkEntry.undo")}
        disabled={!dirty}
        data-testid={`${tid}-entry-undo-${positionId}`}
      >
        <RotateCcw className="size-4" />
      </Button>
    </li>
  );
}
