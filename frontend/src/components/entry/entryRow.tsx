import { useTranslation } from "react-i18next";
import type { EntryFieldValues, EntryShape } from "@/components/entry/shapes";

// Split-renderer contract for one bulk-entry row (ADR-0050). EntryScreen owns
// all data / dirty-tracking / per-row field state / batch validation / atomic
// Save, and delegates only the row to EntryRowDesktop (cramped horizontal row)
// or EntryRowMobile (stacked fields). Both consume this one presentation-neutral
// projection plus the same onFieldChange/onReset callbacks — no logic forks into
// a renderer, and both expose the *same* data-testids so one spec asserts either.

// The carry-forward surfacing state for a row's "when" line.
export type EntryWhen =
  | { kind: "overwrite" } // a snapshot already exists for the chosen month
  | { kind: "carried"; month: string } // value carried from an earlier month (formatted)
  | { kind: "none" }; // no history yet

export type EntryRowView = {
  positionId: string;
  displayName: string;
  currency: string;
  dirty: boolean;
  ownership: string;
  when: EntryWhen;
  error: boolean;
  // Shown value per field key (the user's edit, else the carry-forward prefill).
  fieldValues: EntryFieldValues;
  // Formatted derived value (qty×price / accrued), "—" when the inputs are
  // incomplete, or null when the shape has no derived column (amount-only).
  derived: string | null;
};

export type EntryRowRendererProps = {
  view: EntryRowView;
  shape: EntryShape;
  tid: string;
  // Copy prefix ("bulkEntry" or a group override) for the per-row error string.
  copy: string;
  onFieldChange: (fieldKey: string, value: string) => void;
  onReset: () => void;
};

// The name / ownership / "when" / per-row-error block. Identical in both
// renderers (same testids), so shared here; only its placement diverges — a
// left column on desktop, the top of the stack on mobile.
export function EntryRowMeta({
  view,
  tid,
  copy,
}: {
  view: EntryRowView;
  tid: string;
  copy: string;
}) {
  const { t } = useTranslation("common");
  return (
    <>
      <div className="flex items-center gap-1.5">
        {view.dirty && (
          <span
            className="size-1.5 shrink-0 rounded-full bg-amber-500"
            data-testid={`${tid}-entry-dirty-${view.positionId}`}
            aria-hidden
          />
        )}
        <span className="truncate font-medium">{view.displayName}</span>
      </div>
      <div className="text-xs text-muted-foreground">
        {view.ownership}
        {" · "}
        {view.when.kind === "overwrite" ? (
          <span
            className="text-amber-600"
            data-testid={`${tid}-entry-overwrite-${view.positionId}`}
          >
            {t("bulkEntry.overwritesThisMonth")}
          </span>
        ) : view.when.kind === "carried" ? (
          t("bulkEntry.carriedFrom", { month: view.when.month })
        ) : (
          t("bulkEntry.noHistory")
        )}
      </div>
      {view.error && (
        <div
          className="text-xs text-destructive"
          data-testid={`${tid}-entry-error-${view.positionId}`}
        >
          {t(`${copy}.rowError`)}
        </div>
      )}
    </>
  );
}
