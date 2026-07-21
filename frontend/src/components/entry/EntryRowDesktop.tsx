import { RotateCcw } from "lucide-react";
import { useTranslation } from "react-i18next";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { EntryRowMeta, type EntryRowRendererProps } from "@/components/entry/entryRow";
import type { EntryShape } from "@/components/entry/shapes";

// Amount-only rows show the currency in a fixed-width column left of the single
// input (they have no computed total to carry it). Multi-field shapes drop this
// column — the currency already prints on their derived total (below), so a
// second copy next to the quantity was redundant and misleading.
const CURRENCY_COL = "w-10 shrink-0 text-right";
// The derived-total column: its own column, set off from the inputs (ml-3), with
// the whole "symbol number" unit right-aligned (justify-end) so large totals
// don't clip and the numbers line up on the right. Wide enough for household IDR
// figures.
const DERIVED_COL = "w-40 shrink-0 ml-3";

// EntryRowDesktopHeader labels the input columns once per group (ADR-0046
// Presentation / UX): with the shape's placeholders hidden the moment a value
// carries forward, a qty×price / accrued row otherwise gives no clue which field
// is units and which is price. It renders as the right-hand cluster of the group
// title line (sharing that vertical space) and mirrors the row's right cluster —
// one right-aligned label per field at its own widthClass, then a value-column
// spacer and an action spacer — so the labels sit over their inputs. Amount-only
// shapes (a single unlabelled field) render nothing. Decorative (aria-hidden):
// the inputs keep their own aria-labels.
export function EntryRowDesktopHeader({ shape }: { shape: EntryShape }) {
  const { t } = useTranslation("common");
  if (!shape.fields.some((f) => f.labelKey)) return null;
  return (
    <div className="flex items-center gap-3 text-xs" aria-hidden>
      <div className="flex items-center gap-2">
        {shape.fields.map((f) => (
          <span key={f.key} className={`${f.widthClass} truncate text-right`}>
            {f.labelKey ? t(f.labelKey) : ""}
          </span>
        ))}
      </div>
      {shape.derived && <span className={DERIVED_COL} />}
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
  const { dirty, positionId, derived } = view;
  return (
    <li className="flex items-center gap-3 py-2" data-testid={`${tid}-entry-row-${positionId}`}>
      <div className="min-w-0 flex-1">
        <EntryRowMeta view={view} tid={tid} copy={copy} />
      </div>
      {/* Currency only where there's no total to carry it (amount-only). */}
      {!shape.derived && (
        <span className={`${CURRENCY_COL} text-xs text-muted-foreground`}>{view.currency}</span>
      )}
      <div className="flex items-center gap-2">
        {shape.fields.map((f) => (
          <Input
            key={f.key}
            className={`${f.widthClass} text-right${dirty ? " border-amber-500 ring-1 ring-amber-500" : ""}`}
            inputMode="decimal"
            aria-label={f.labelKey ? t(f.labelKey) : undefined}
            placeholder={f.labelKey ? t(f.labelKey) : undefined}
            value={view.fieldValues[f.key] ?? ""}
            onChange={(e) => onFieldChange(f.key, e.target.value)}
            data-testid={`${tid}-entry-${f.testidSuffix}-${positionId}`}
          />
        ))}
      </div>
      {derived !== null && (
        <span
          className={`${DERIVED_COL} flex items-baseline justify-end gap-1.5 text-sm tabular-nums text-muted-foreground`}
          data-testid={`${tid}-entry-value-${positionId}`}
        >
          {derived.value !== null ? (
            <>
              <span>{derived.symbol}</span>
              <span>{derived.value}</span>
            </>
          ) : (
            "—"
          )}
        </span>
      )}
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
